package workflow

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/NubeDev/bizzy/pkg/services"
	"gorm.io/gorm"
)

// EventPublisher publishes lifecycle events to the event bus.
// Optional — if nil, events are silently skipped.
type EventPublisher interface {
	Publish(topic string, data any) error
}

// Runner executes workflow runs. It manages active runs in memory and persists
// state to the database.
type Runner struct {
	mu         sync.RWMutex
	active     map[string]*activeRun // workflow_id -> active run state
	db         *gorm.DB
	workflows  *Store
	tools      services.ToolCaller
	prompts    services.PromptRunner
	bus        EventPublisher // optional event bus
}

// activeRun holds the in-memory state for a running workflow, including the
// cancel function and approval channel.
type activeRun struct {
	cancel   context.CancelFunc
	approval chan approvalAction
}

type approvalAction struct {
	Action   string // "approve", "reject", "cancel"
	Feedback string
}

// NewRunner creates a workflow runner.
func NewRunner(
	db *gorm.DB,
	workflows *Store,
	tools services.ToolCaller,
	prompts services.PromptRunner,
) *Runner {
	return &Runner{
		active:    make(map[string]*activeRun),
		db:        db,
		workflows: workflows,
		tools:     tools,
		prompts:   prompts,
	}
}

// SetBus attaches an event publisher for lifecycle events.
func (r *Runner) SetBus(bus EventPublisher) {
	r.bus = bus
}

// publish sends an event if a bus is configured.
func (r *Runner) publish(topic string, data any) {
	if r.bus != nil {
		r.bus.Publish(topic, data)
	}
}

// Start begins executing a workflow. The workflow_id in the run is treated as
// an idempotency key — if a run with that ID already exists, it is returned.
func (r *Runner) Start(run models.WorkflowRun) (*models.WorkflowRun, error) {
	// Idempotency check.
	var existing models.WorkflowRun
	if err := r.db.First(&existing, "id = ?", run.ID).Error; err == nil {
		return &existing, nil
	}

	def, ok := r.workflows.Get(run.AppName, run.Workflow)
	if !ok {
		return nil, fmt.Errorf("workflow not found: %s/%s", run.AppName, run.Workflow)
	}

	if err := Validate(def); err != nil {
		return nil, fmt.Errorf("invalid workflow: %w", err)
	}

	// Validate required inputs.
	for _, input := range def.Inputs {
		if input.Required {
			if _, ok := run.Inputs[input.Name]; !ok {
				return nil, fmt.Errorf("missing required input: %s", input.Name)
			}
		}
	}

	// Apply input defaults.
	for _, input := range def.Inputs {
		if _, ok := run.Inputs[input.Name]; !ok && input.Default != "" {
			run.Inputs[input.Name] = input.Default
		}
	}

	// Initialize stage results.
	now := time.Now().UTC()
	run.Status = models.WorkflowRunning
	run.Version = 1
	run.CurrentIdx = 0
	run.CreatedAt = now
	run.Stages = make([]models.StageResult, len(def.Stages))
	for i, s := range def.Stages {
		run.Stages[i] = models.StageResult{
			Name:   s.Name,
			Status: models.StagePending,
		}
	}

	if err := r.db.Create(&run).Error; err != nil {
		return nil, fmt.Errorf("persist workflow run: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	ar := &activeRun{
		cancel:   cancel,
		approval: make(chan approvalAction, 1),
	}

	r.mu.Lock()
	r.active[run.ID] = ar
	r.mu.Unlock()

	go r.execute(ctx, run.ID, def, ar)

	r.publish(TopicStarted, RunEvent{
		RunID:    run.ID,
		AppName:  run.AppName,
		Workflow: run.Workflow,
		UserID:   run.UserID,
		Status:   "running",
	})

	return &run, nil
}

// Get returns the current state of a workflow run.
func (r *Runner) Get(id string) (*models.WorkflowRun, bool) {
	var run models.WorkflowRun
	if err := r.db.First(&run, "id = ?", id).Error; err != nil {
		return nil, false
	}
	return &run, true
}

// List returns workflow runs, optionally filtered.
func (r *Runner) List(userID, appName string, status models.WorkflowStatus) []models.WorkflowRun {
	query := r.db.Model(&models.WorkflowRun{})
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if appName != "" {
		query = query.Where("app_name = ?", appName)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var runs []models.WorkflowRun
	query.Order("created_at DESC").Find(&runs)
	return runs
}

// Approve sends an approval or rejection to a waiting workflow.
func (r *Runner) Approve(id string, action, feedback string) error {
	r.mu.RLock()
	ar, ok := r.active[id]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("workflow %s is not active", id)
	}

	var run models.WorkflowRun
	if err := r.db.First(&run, "id = ?", id).Error; err != nil {
		return fmt.Errorf("workflow %s not found", id)
	}
	if run.Status != models.WorkflowWaitingApproval {
		return fmt.Errorf("workflow %s is not waiting for approval (status: %s)", id, run.Status)
	}

	select {
	case ar.approval <- approvalAction{Action: action, Feedback: feedback}:
		return nil
	default:
		return fmt.Errorf("approval already pending for workflow %s", id)
	}
}

// Cancel stops a running workflow.
func (r *Runner) Cancel(id string) error {
	r.mu.RLock()
	ar, ok := r.active[id]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("workflow %s is not active", id)
	}

	ar.cancel()

	var run models.WorkflowRun
	r.db.First(&run, "id = ?", id)
	now := time.Now().UTC()
	run.Status = models.WorkflowCancelled
	run.Version++
	run.FinishedAt = &now
	r.db.Save(&run)

	r.mu.Lock()
	delete(r.active, id)
	r.mu.Unlock()

	r.publish(TopicCancelled, RunEvent{
		RunID:    run.ID,
		AppName:  run.AppName,
		Workflow: run.Workflow,
		UserID:   run.UserID,
		Status:   "cancelled",
	})

	return nil
}

// execute runs the workflow stages sequentially in a goroutine.
func (r *Runner) execute(ctx context.Context, runID string, def *WorkflowDef, ar *activeRun) {
	defer func() {
		r.mu.Lock()
		delete(r.active, runID)
		r.mu.Unlock()
	}()

	var run models.WorkflowRun
	r.db.First(&run, "id = ?", runID)

	// Build the template variable map: starts with inputs.
	vars := map[string]any{
		"inputs": run.Inputs,
	}

	for i, stageDef := range def.Stages {
		if ctx.Err() != nil {
			return
		}

		// Mark stage as running.
		now := time.Now().UTC()
		run.CurrentIdx = i
		run.Stages[i].Status = models.StageRunning
		run.Stages[i].StartedAt = &now
		run.Version++
		r.db.Save(&run)

		r.publish(TopicStageStarted, RunEvent{
			RunID:    run.ID,
			AppName:  run.AppName,
			Workflow: run.Workflow,
			UserID:   run.UserID,
			Stage:    stageDef.Name,
			Status:   "running",
		})

		result, err := r.executeStage(ctx, &run, &stageDef, vars, ar)

		elapsed := time.Since(now)
		run.Stages[i].DurationMS = int(elapsed.Milliseconds())

		if err != nil {
			onFail := stageDef.OnFail
			if onFail == "" {
				onFail = "stop"
			}

			switch onFail {
			case "skip":
				run.Stages[i].Status = models.StageSkipped
				run.Stages[i].Error = err.Error()
				run.Version++
				r.db.Save(&run)
				continue

			case "retry":
				maxRetries := stageDef.MaxRetries
				if maxRetries <= 0 {
					maxRetries = 3
				}
				retried := false
				for attempt := 1; attempt <= maxRetries; attempt++ {
					result, err = r.executeStage(ctx, &run, &stageDef, vars, ar)
					if err == nil {
						retried = true
						break
					}
				}
				if !retried {
					r.failRun(&run, i, err)
					return
				}
				// Fall through to save result below.

			default: // "stop"
				r.failRun(&run, i, err)
				return
			}
		}

		// Save result.
		run.Stages[i].Status = models.StageCompleted
		run.Stages[i].Output = result
		run.Version++

		r.publish(TopicStageCompleted, RunEvent{
			RunID:      run.ID,
			AppName:    run.AppName,
			Workflow:   run.Workflow,
			UserID:     run.UserID,
			Stage:      stageDef.Name,
			Status:     "completed",
			DurationMS: run.Stages[i].DurationMS,
		})

		// Store output as template variable.
		if stageDef.SaveAs != "" {
			vars[stageDef.SaveAs] = result
		}
		// Also set {{previous}} to last completed output.
		vars["previous"] = result

		r.db.Save(&run)
	}

	// All stages complete.
	now := time.Now().UTC()
	run.Status = models.WorkflowCompleted
	run.Version++
	run.FinishedAt = &now
	r.db.Save(&run)

	r.publish(TopicCompleted, RunEvent{
		RunID:    run.ID,
		AppName:  run.AppName,
		Workflow: run.Workflow,
		UserID:   run.UserID,
		Status:   "completed",
	})
}

// executeStage runs a single stage and returns its output.
func (r *Runner) executeStage(
	ctx context.Context,
	run *models.WorkflowRun,
	stage *StageDef,
	vars map[string]any,
	ar *activeRun,
) (any, error) {
	// Apply per-stage timeout if set.
	if stage.TimeoutSec > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(stage.TimeoutSec)*time.Second)
		defer cancel()
	}

	switch stage.StageType() {
	case "tool":
		return r.executeTool(ctx, run, stage, vars)
	case "prompt":
		return r.executePrompt(ctx, run, stage, vars)
	case "approval":
		return r.executeApproval(ctx, run, stage, vars, ar)
	case "output":
		return r.executeOutput(stage, vars)
	case "conditional":
		return r.executeConditional(ctx, run, stage, vars)
	default:
		return nil, fmt.Errorf("unknown stage type: %s", stage.StageType())
	}
}

func (r *Runner) executeTool(ctx context.Context, run *models.WorkflowRun, stage *StageDef, vars map[string]any) (any, error) {
	toolName := Resolve(stage.Tool, vars)
	params := ResolveParams(stage.Params, vars)
	return r.tools.CallTool(ctx, run.UserID, toolName, params)
}

func (r *Runner) executePrompt(ctx context.Context, run *models.WorkflowRun, stage *StageDef, vars map[string]any) (any, error) {
	prompt := Resolve(stage.Prompt, vars)
	text, err := r.prompts.RunPrompt(ctx, run.UserID, prompt)
	if err != nil {
		return nil, err
	}
	return text, nil
}

func (r *Runner) executeApproval(
	ctx context.Context,
	run *models.WorkflowRun,
	stage *StageDef,
	vars map[string]any,
	ar *activeRun,
) (any, error) {
	// Resolve the content to show the user.
	content := Resolve(stage.Show, vars)

	// Update run status to waiting.
	stageIdx := run.CurrentIdx
	run.Status = models.WorkflowWaitingApproval
	run.Stages[stageIdx].Status = models.StageWaiting
	run.Stages[stageIdx].Output = content
	run.Version++
	r.db.Save(run)

	r.publish(TopicWaitingApproval, RunEvent{
		RunID:    run.ID,
		AppName:  run.AppName,
		Workflow: run.Workflow,
		UserID:   run.UserID,
		Stage:    stage.Name,
		Status:   "waiting_approval",
		Output:   content,
	})

	// Wait for approval.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case action := <-ar.approval:
		switch action.Action {
		case "approve":
			run.Status = models.WorkflowRunning
			r.publish(TopicApproved, RunEvent{
				RunID: run.ID, AppName: run.AppName, Workflow: run.Workflow,
				UserID: run.UserID, Stage: stage.Name, Status: "approved",
			})
			return content, nil
		case "reject":
			// Inject feedback into vars for retry stages.
			if action.Feedback != "" {
				vars["rejection_feedback"] = action.Feedback
			}
			run.Status = models.WorkflowRunning
			r.publish(TopicRejected, RunEvent{
				RunID: run.ID, AppName: run.AppName, Workflow: run.Workflow,
				UserID: run.UserID, Stage: stage.Name, Status: "rejected",
				Error: action.Feedback,
			})
			return nil, fmt.Errorf("rejected: %s", action.Feedback)
		case "cancel":
			return nil, fmt.Errorf("cancelled by user")
		default:
			return nil, fmt.Errorf("unknown approval action: %s", action.Action)
		}
	}
}

func (r *Runner) executeOutput(stage *StageDef, vars map[string]any) (any, error) {
	if stage.Output == nil {
		return nil, fmt.Errorf("output stage has no output definition")
	}
	out := map[string]any{
		"message": Resolve(stage.Output.Message, vars),
	}
	if stage.Output.File != "" {
		out["file"] = Resolve(stage.Output.File, vars)
	}
	if stage.Output.Content != "" {
		out["content"] = Resolve(stage.Output.Content, vars)
	}
	return out, nil
}

func (r *Runner) executeConditional(ctx context.Context, run *models.WorkflowRun, stage *StageDef, vars map[string]any) (any, error) {
	switchVal := Resolve(stage.Switch, vars)

	caseDef, ok := stage.Cases[switchVal]
	if !ok {
		return nil, fmt.Errorf("conditional stage %q: no case for value %q", stage.Name, switchVal)
	}

	if caseDef.Tool != "" {
		toolName := Resolve(caseDef.Tool, vars)
		params := ResolveParams(caseDef.Params, vars)
		return r.tools.CallTool(ctx, run.UserID, toolName, params)
	}
	if caseDef.Prompt != "" {
		prompt := Resolve(caseDef.Prompt, vars)
		return r.prompts.RunPrompt(ctx, run.UserID, prompt)
	}
	return nil, fmt.Errorf("conditional case %q has no tool or prompt", switchVal)
}

func (r *Runner) failRun(run *models.WorkflowRun, stageIdx int, err error) {
	now := time.Now().UTC()
	run.Stages[stageIdx].Status = models.StageFailed
	run.Stages[stageIdx].Error = err.Error()
	run.Status = models.WorkflowFailed
	run.FailedAt = run.Stages[stageIdx].Name
	run.Error = err.Error()
	run.Version++
	run.FinishedAt = &now
	r.db.Save(run)

	r.publish(TopicStageFailed, RunEvent{
		RunID:   run.ID,
		AppName: run.AppName,
		Workflow: run.Workflow,
		UserID:  run.UserID,
		Stage:   run.Stages[stageIdx].Name,
		Status:  "failed",
		Error:   err.Error(),
	})

	r.publish(TopicFailed, RunEvent{
		RunID:    run.ID,
		AppName:  run.AppName,
		Workflow: run.Workflow,
		UserID:   run.UserID,
		Status:   "failed",
		Error:    err.Error(),
	})
}
