package command

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/NubeDev/bizzy/pkg/bus"
	"github.com/NubeDev/bizzy/pkg/models"
)

// WorkflowStarter abstracts the workflow.Runner for the command router.
type WorkflowStarter interface {
	Start(run models.WorkflowRun) (*models.WorkflowRun, error)
	Get(id string) (*models.WorkflowRun, bool)
	List(userID, appName string, status models.WorkflowStatus) []models.WorkflowRun
	Approve(id string, action, feedback string) error
	Cancel(id string) error
}

// ToolExecutor abstracts the services.ToolService for the command router.
type ToolExecutor interface {
	CallTool(ctx context.Context, userID, toolName string, params map[string]any) (any, error)
}

// AgentExecutor abstracts the services.AgentService for async AI jobs.
type AgentExecutor interface {
	// RunJob starts an async AI job and returns the job ID.
	RunJob(userID, prompt, provider, model string) string
}

// ToolLister lists available tools and prompts for a user.
type ToolLister interface {
	ListTools(userID string) []ToolInfo
	ListPrompts(userID string) []PromptInfo
}

// ToolInfo is a simplified tool descriptor for command responses.
type ToolInfo struct {
	Name    string `json:"name"`
	AppName string `json:"appName"`
	Desc    string `json:"description"`
}

// PromptInfo is a simplified prompt descriptor for command responses.
type PromptInfo struct {
	Name    string `json:"name"`
	AppName string `json:"appName"`
	Desc    string `json:"description"`
}

// RouterConfig holds all dependencies for the command router.
type RouterConfig struct {
	Parser    *Parser
	Workflows WorkflowStarter
	Tools     ToolExecutor
	Agents    AgentExecutor
	Lister    ToolLister // optional — enables list tools/prompts
	Bus       *bus.Bus
	Adapters  AdapterRegistry
}

// Router validates, deduplicates, rate-limits, and dispatches commands
// to the right executor, then publishes results to the event bus.
type Router struct {
	parser    *Parser
	workflows WorkflowStarter
	tools     ToolExecutor
	agents    AgentExecutor
	lister    ToolLister
	bus       *bus.Bus
	adapters  AdapterRegistry
	dedup     *dedupCache
	limiter   *rateLimiter
}

// NewRouter creates a command router.
func NewRouter(cfg RouterConfig) *Router {
	return &Router{
		parser:    cfg.Parser,
		workflows: cfg.Workflows,
		tools:     cfg.Tools,
		agents:    cfg.Agents,
		lister:    cfg.Lister,
		bus:       cfg.Bus,
		adapters:  cfg.Adapters,
		dedup:     newDedupCache(5 * time.Minute),
		limiter:   newRateLimiter(10, 30), // 10 req/s burst, 30 tokens
	}
}

// Parser returns the command parser (used by adapters).
func (r *Router) Parser() *Parser {
	return r.parser
}

// Bus returns the event bus (used by adapters to subscribe).
func (r *Router) Bus() *bus.Bus {
	return r.bus
}

// Execute processes a command through the full pipeline.
func (r *Router) Execute(ctx context.Context, cmd Command) {
	// 1. Deduplication.
	if r.dedup.hasSeen(cmd.ID) {
		return
	}
	r.dedup.mark(cmd.ID)

	// 2. Rate limit.
	if !r.limiter.allow(cmd.UserID) {
		r.reply(ctx, cmd, ReplyMessage{Text: "Rate limited — try again shortly."})
		return
	}

	// 3. Publish command received event.
	r.bus.Publish(bus.TopicCommandReceived, CommandEvent{Command: cmd})

	// 4. Dispatch based on verb.
	var result Result
	var err error

	switch cmd.Verb {
	case VerbRun:
		result, err = r.executeRun(ctx, cmd)
	case VerbAsk:
		result, err = r.executeAsk(ctx, cmd)
	case VerbStatus:
		result, err = r.executeStatus(ctx, cmd)
	case VerbCancel:
		result, err = r.executeCancel(ctx, cmd)
	case VerbList:
		result, err = r.executeList(ctx, cmd)
	case VerbApprove, VerbReject:
		result, err = r.executeApproval(ctx, cmd)
	case VerbHelp:
		result, err = r.executeHelp(ctx, cmd)
	default:
		err = fmt.Errorf("unknown verb: %s", cmd.Verb)
	}

	// 5. Publish result.
	if err != nil {
		r.bus.Publish(bus.TopicCommandFailed, CommandResultEvent{
			Command: cmd,
			Error:   err.Error(),
		})
	} else {
		topic := bus.TopicCommandCompleted
		if result.Async {
			topic = bus.TopicCommandAccepted
		}
		r.bus.Publish(topic, CommandResultEvent{
			Command: cmd,
			Result:  result,
		})
	}
}

func (r *Router) executeRun(ctx context.Context, cmd Command) (Result, error) {
	// Resolve target kind if missing.
	if cmd.Target.Kind == "" {
		cmd.Target.Kind = "workflow" // default: try workflow first
	}

	switch cmd.Target.Kind {
	case "workflow":
		return r.runWorkflow(ctx, cmd)
	case "tool":
		return r.runTool(ctx, cmd)
	case "ai":
		return r.runAI(ctx, cmd)
	default:
		return Result{}, fmt.Errorf("unknown target kind: %s", cmd.Target.Kind)
	}
}

func (r *Router) runWorkflow(ctx context.Context, cmd Command) (Result, error) {
	// Parse "appName/workflowName" from target name.
	appName, wfName := splitAppWorkflow(cmd.Target.Name)

	run := models.WorkflowRun{
		ID:       models.GenerateID("wf-"),
		AppName:  appName,
		Workflow: wfName,
		Inputs:   cmd.Params,
		UserID:   cmd.UserID,
	}

	started, err := r.workflows.Start(run)
	if err != nil {
		return Result{}, fmt.Errorf("start workflow: %w", err)
	}

	// Publish workflow started event with reply info.
	r.bus.Publish(bus.TopicWorkflowStarted, bus.EventData{
		CommandID:  cmd.ID,
		UserID:     cmd.UserID,
		TargetKind: "workflow",
		TargetName: cmd.Target.Name,
		TargetID:   started.ID,
		ReplyTo:    mustMarshal(cmd.ReplyTo),
		Status:     "running",
	})

	return Result{
		ID:      started.ID,
		Message: "Workflow started",
		Async:   true,
	}, nil
}

func (r *Router) runTool(ctx context.Context, cmd Command) (Result, error) {
	output, err := r.tools.CallTool(ctx, cmd.UserID, cmd.Target.Name, cmd.Params)
	if err != nil {
		return Result{}, fmt.Errorf("call tool: %w", err)
	}
	return Result{Output: output}, nil
}

func (r *Router) runAI(ctx context.Context, cmd Command) (Result, error) {
	prompt, _ := cmd.Params["prompt"].(string)
	if prompt == "" {
		return Result{}, fmt.Errorf("ask requires a prompt")
	}

	provider, _ := cmd.Params["provider"].(string)
	model, _ := cmd.Params["model"].(string)

	if r.agents == nil {
		return Result{}, fmt.Errorf("agent service not configured")
	}

	jobID := r.agents.RunJob(cmd.UserID, prompt, provider, model)
	return Result{
		ID:      jobID,
		Message: "Job started",
		Async:   true,
	}, nil
}

func (r *Router) executeAsk(ctx context.Context, cmd Command) (Result, error) {
	cmd.Target = Target{Kind: "ai"}
	return r.runAI(ctx, cmd)
}

func (r *Router) executeStatus(ctx context.Context, cmd Command) (Result, error) {
	switch cmd.Target.Kind {
	case "workflow", "":
		run, ok := r.workflows.Get(cmd.Target.Name)
		if !ok {
			return Result{}, fmt.Errorf("workflow %s not found", cmd.Target.Name)
		}
		return Result{Output: run}, nil
	default:
		return Result{}, fmt.Errorf("status not supported for: %s", cmd.Target.Kind)
	}
}

func (r *Router) executeCancel(ctx context.Context, cmd Command) (Result, error) {
	switch cmd.Target.Kind {
	case "workflow", "":
		if err := r.workflows.Cancel(cmd.Target.Name); err != nil {
			return Result{}, err
		}
		return Result{Message: fmt.Sprintf("Cancelled %s", cmd.Target.Name)}, nil
	default:
		return Result{}, fmt.Errorf("cancel not supported for: %s", cmd.Target.Kind)
	}
}

func (r *Router) executeList(ctx context.Context, cmd Command) (Result, error) {
	switch cmd.Target.Kind {
	case "workflow":
		statusFilter := models.WorkflowStatus("")
		if s, ok := cmd.Params["status"].(string); ok {
			statusFilter = models.WorkflowStatus(s)
		}
		runs := r.workflows.List(cmd.UserID, "", statusFilter)
		return Result{Output: runs}, nil
	case "tool":
		if r.lister == nil {
			return Result{}, fmt.Errorf("tool listing not available")
		}
		tools := r.lister.ListTools(cmd.UserID)
		return Result{Output: tools}, nil
	case "prompt":
		if r.lister == nil {
			return Result{}, fmt.Errorf("prompt listing not available")
		}
		prompts := r.lister.ListPrompts(cmd.UserID)
		return Result{Output: prompts}, nil
	case "":
		// No kind specified — list workflows by default.
		runs := r.workflows.List(cmd.UserID, "", "")
		return Result{Output: runs}, nil
	default:
		return Result{}, fmt.Errorf("list not supported for: %s", cmd.Target.Kind)
	}
}

func (r *Router) executeApproval(ctx context.Context, cmd Command) (Result, error) {
	action := "approve"
	if cmd.Verb == VerbReject {
		action = "reject"
	}

	feedback, _ := cmd.Params["feedback"].(string)

	if err := r.workflows.Approve(cmd.Target.Name, action, feedback); err != nil {
		return Result{}, err
	}

	return Result{Message: fmt.Sprintf("Workflow %s %sd", cmd.Target.Name, action)}, nil
}

func (r *Router) executeHelp(ctx context.Context, cmd Command) (Result, error) {
	help := `Available commands:
  run [kind/name] [--param value]   Start a workflow or tool
  ask "prompt"                      Ask AI a question
  status [kind/name]                Check progress
  cancel [kind/name]                Stop something
  list [kind] [--status ...]        List items
  approve [kind/name]               Approve a waiting workflow
  reject [kind/name] [--feedback]   Reject a waiting workflow
  help                              Show this message

Target kinds: workflow, tool, job
Examples:
  run workflow/weekly-report --site Sydney
  ask "which devices are offline?"
  status wf-abc123
  cancel wf-abc123
  list workflows --status running`

	return Result{Output: help, Message: help}, nil
}

func (r *Router) reply(ctx context.Context, cmd Command, msg ReplyMessage) {
	if r.adapters == nil {
		return
	}
	ch, err := r.adapters.BuildReply(cmd.ReplyTo)
	if err != nil {
		log.Printf("[command] reply failed for channel=%s: %v", cmd.ReplyTo.Channel, err)
		return
	}
	ch.Send(ctx, msg)
}

// splitAppWorkflow splits "appName/workflowName" into parts.
// If no slash, appName is empty (router will search).
func splitAppWorkflow(name string) (string, string) {
	for i, ch := range name {
		if ch == '/' {
			return name[:i], name[i+1:]
		}
	}
	return "", name
}

func mustMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// --- dedup cache ---

type dedupCache struct {
	mu      sync.Mutex
	entries map[string]time.Time
	ttl     time.Duration
}

func newDedupCache(ttl time.Duration) *dedupCache {
	d := &dedupCache{
		entries: make(map[string]time.Time),
		ttl:     ttl,
	}
	go d.cleanup()
	return d
}

func (d *dedupCache) hasSeen(id string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if t, ok := d.entries[id]; ok {
		return time.Since(t) < d.ttl
	}
	return false
}

func (d *dedupCache) mark(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.entries[id] = time.Now()
}

func (d *dedupCache) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		d.mu.Lock()
		now := time.Now()
		for id, t := range d.entries {
			if now.Sub(t) > d.ttl {
				delete(d.entries, id)
			}
		}
		d.mu.Unlock()
	}
}

// --- rate limiter (simple token bucket per user) ---

type rateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rate     float64
	capacity int
}

type bucket struct {
	tokens    float64
	lastCheck time.Time
}

func newRateLimiter(rate float64, capacity int) *rateLimiter {
	return &rateLimiter{
		buckets:  make(map[string]*bucket),
		rate:     rate,
		capacity: capacity,
	}
}

func (rl *rateLimiter) allow(userID string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, ok := rl.buckets[userID]
	if !ok {
		b = &bucket{tokens: float64(rl.capacity), lastCheck: time.Now()}
		rl.buckets[userID] = b
	}

	now := time.Now()
	elapsed := now.Sub(b.lastCheck).Seconds()
	b.tokens += elapsed * rl.rate
	if b.tokens > float64(rl.capacity) {
		b.tokens = float64(rl.capacity)
	}
	b.lastCheck = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
