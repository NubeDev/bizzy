package flow

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/NubeDev/bizzy/pkg/airunner"
	"github.com/NubeDev/bizzy/pkg/services"
)

// Engine executes flow DAGs. It follows a single-writer model: one main goroutine
// owns all run state while worker goroutines execute nodes and report results
// back via a channel.
type Engine struct {
	store     *Store
	registry  *NodeTypeRegistry
	executors *ExecutorRegistry
	services  *Services
	events    eventEmitter
	*runtime  // deploy/undeploy lifecycle

	// Resource limits.
	MaxParallelNodes int
	MaxRunsPerUser   int

	// Active run tracking.
	mu      sync.RWMutex
	cancels map[string]context.CancelFunc // runID -> cancel
}

// NewEngine creates a flow execution engine with all built-in executors registered.
func NewEngine(store *Store, registry *NodeTypeRegistry) *Engine {
	executors := NewExecutorRegistry()
	RegisterBuiltinExecutors(executors)
	RegisterIntegrationExecutors(executors)

	e := &Engine{
		store:            store,
		registry:         registry,
		executors:        executors,
		services:         &Services{},
		runtime:          newRuntime(),
		MaxParallelNodes: 50,
		MaxRunsPerUser:   10,
		cancels:          make(map[string]context.CancelFunc),
	}
	RegisterBuiltinTriggers(e)
	return e
}

// SetServices merges non-nil fields from s into the engine's services.
// Fields that are nil in s are left unchanged.
func (e *Engine) SetServices(s *Services) {
	if s.Tools != nil {
		e.services.Tools = s.Tools
	}
	if s.Prompts != nil {
		e.services.Prompts = s.Prompts
	}
	if s.Agents != nil {
		e.services.Agents = s.Agents
	}
	if s.Slack != nil {
		e.services.Slack = s.Slack
	}
	if s.Email != nil {
		e.services.Email = s.Email
	}
	if s.Bus != nil {
		e.services.Bus = s.Bus
		e.events = eventEmitter{bus: s.Bus}
	}
}

// SetBus attaches an event publisher.
func (e *Engine) SetBus(bus EventPublisher) {
	e.events = eventEmitter{bus: bus}
	e.services.Bus = bus
}

// SetTools sets the tool caller for tool: nodes.
func (e *Engine) SetTools(tools services.ToolCaller) {
	e.services.Tools = tools
}

// SetPrompts sets the prompt runner for ai-prompt nodes.
func (e *Engine) SetPrompts(prompts services.PromptRunner) {
	e.services.Prompts = prompts
}

// SetAgents sets the AI runner registry for ai-runner nodes.
func (e *Engine) SetAgents(agents *airunner.Registry) {
	e.services.Agents = agents
}

// SetSlack sets the Slack sender for slack-send nodes.
func (e *Engine) SetSlack(slack SlackSender) {
	e.services.Slack = slack
}

// SetEmail sets the email sender for email-send nodes.
func (e *Engine) SetEmail(email EmailSender) {
	e.services.Email = email
}

// Executors returns the executor registry for registering custom node types.
func (e *Engine) Executors() *ExecutorRegistry {
	return e.executors
}

// Registry returns the node type registry.
func (e *Engine) Registry() *NodeTypeRegistry {
	return e.registry
}

// Store returns the flow store.
func (e *Engine) Store() *Store {
	return e.store
}

// --- Flow execution ---

// StartRun creates and begins executing a flow run.
func (e *Engine) StartRun(ctx context.Context, flowID, userID string, inputs map[string]any, replyTo *ReplyInfo) (*FlowRun, error) {
	def, err := e.store.GetFlow(flowID)
	if err != nil {
		return nil, fmt.Errorf("flow %s not found: %w", flowID, err)
	}

	// Check concurrent run limit.
	if e.MaxRunsPerUser > 0 {
		count, _ := e.store.CountActiveRuns(userID)
		if int(count) >= e.MaxRunsPerUser {
			return nil, fmt.Errorf("concurrent run limit reached (%d)", e.MaxRunsPerUser)
		}
	}

	// Validate the flow.
	if verr := Validate(def, e.registry); verr != nil {
		return nil, verr
	}

	// Initialize node states.
	nodeStates := make(map[string]NodeState, len(def.Nodes))
	for _, n := range def.Nodes {
		nodeStates[n.ID] = NodeState{Status: NodePending}
	}

	run := &FlowRun{
		FlowID:      def.ID,
		FlowVersion: def.Version,
		FlowName:    def.Name,
		Status:      FlowRunRunning,
		Inputs:      inputs,
		NodeStates:  nodeStates,
		Variables:   make(map[string]any),
		UserID:      userID,
		ReplyTo:     replyTo,
	}

	if err := e.store.CreateRun(run); err != nil {
		return nil, fmt.Errorf("create run: %w", err)
	}

	e.events.flowStarted(run)

	// Execute asynchronously — detach from the caller's context so the flow
	// outlives the HTTP request that triggered it.
	go e.execute(context.Background(), run, def)

	return run, nil
}

// execute is the main execution loop (single-writer).
func (e *Engine) execute(ctx context.Context, run *FlowRun, def *FlowDef) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Register cancel for external cancellation.
	e.mu.Lock()
	e.cancels[run.ID] = cancel
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		delete(e.cancels, run.ID)
		e.mu.Unlock()
	}()

	// Apply flow-level timeout.
	if timeoutSec := getIntOrDefault(def.Settings, "timeout", 3600); timeoutSec > 0 {
		var timeoutCancel context.CancelFunc
		ctx, timeoutCancel = context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
		defer timeoutCancel()
	}

	// Initialize input delivery map.
	deliveredInputs := make(map[string]map[string]any) // nodeID -> portHandle -> value

	results := make(chan nodeResult, len(def.Nodes))
	inflight := 0

	// Seed the trigger node.
	triggerID := findTriggerNode(def)
	if triggerID == "" {
		run.Status = FlowRunFailed
		run.Error = "no trigger node found"
		e.persistRun(run)
		return
	}
	markReady(run, triggerID)
	inflight += e.fireReadyNodes(ctx, run, def, results, deliveredInputs)

	for inflight > 0 {
		select {
		case <-ctx.Done():
			run.Status = FlowRunCancelled
			e.persistRun(run)
			e.events.flowCancelled(run)
			return

		case res := <-results:
			inflight--

			if res.Error != nil {
				cont := e.handleNodeError(run, def, res, deliveredInputs)
				if !cont {
					cancel()
					run.Status = FlowRunFailed
					e.persistRun(run)
					e.events.flowFailed(run, run.Error)
					return
				}
			} else {
				state := run.NodeStates[res.NodeID]
				state.Status = NodeCompleted
				state.Output = res.Output
				state.StartedAt = &res.StartedAt
				state.FinishedAt = &res.FinishedAt
				state.DurationMS = int(res.FinishedAt.Sub(res.StartedAt).Milliseconds())
				run.NodeStates[res.NodeID] = state

				nodeDef := def.GetNode(res.NodeID)
				e.events.nodeCompleted(run, nodeDef, state.DurationMS)

				// Handle output node — store as flow output.
				if nodeDef.Type == "output" {
					if po, ok := res.Output.(PortOutput); ok {
						run.Output = wrapOutput(po.Value)
					} else {
						run.Output = wrapOutput(res.Output)
					}
				}

				e.propagateOutputs(run, def, res.NodeID, res.Output, deliveredInputs)
			}

			e.persistRun(run)

			// Check for flow-level completion.
			if allTerminalsDone(run, def) {
				run.Status = FlowRunCompleted
				now := time.Now()
				run.FinishedAt = &now
				e.persistRun(run)
				e.events.flowCompleted(run)
				return
			}

			// Check for pause (approval gate).
			if run.Status == FlowRunWaitingApproval {
				return
			}

			inflight += e.fireReadyNodes(ctx, run, def, results, deliveredInputs)
		}
	}

	// No in-flight nodes and no terminals done.
	if run.Status == FlowRunRunning {
		run.Status = FlowRunCompleted
		now := time.Now()
		run.FinishedAt = &now
		e.persistRun(run)
		e.events.flowCompleted(run)
	}
}

// dispatch routes node execution through the executor registry.
func (e *Engine) dispatch(ctx context.Context, run *FlowRun, def *FlowDef, node *FlowNodeDef, inputs map[string]any) (any, error) {
	// Apply per-node timeout.
	if timeout := getIntOrDefault(node.Data, "timeout", 0); timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
	}

	return e.executors.Dispatch(ctx, &ExecContext{
		Run:      run,
		Node:     node,
		Def:      def,
		Inputs:   inputs,
		Services: e.services,
		Engine:   e,
	})
}

// --- Helpers ---

// wrapOutput converts any value to map[string]any for GORM-compatible storage.
func wrapOutput(v any) map[string]any {
	if v == nil {
		return nil
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{"value": v}
}

func (e *Engine) persistRun(run *FlowRun) {
	if err := e.store.SaveRun(run); err != nil {
		log.Printf("[flow] failed to persist run %s: %v", run.ID, err)
	}
}
