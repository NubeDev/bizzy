package models

import "time"

// WorkflowRun tracks a single execution of a workflow.
type WorkflowRun struct {
	ID         string         `json:"id" gorm:"primaryKey"`
	AppName    string         `json:"app" gorm:"index"`
	Workflow   string         `json:"workflow"`
	Inputs     map[string]any `json:"inputs" gorm:"serializer:json"`
	Status     WorkflowStatus `json:"status" gorm:"index"`
	Version    int            `json:"version"`
	Stages     []StageResult  `json:"stages" gorm:"serializer:json"`
	CurrentIdx int            `json:"current_idx"`
	FailedAt   string         `json:"failed_at,omitempty"`
	Error      string         `json:"error,omitempty"`
	UserID     string         `json:"user_id" gorm:"index"`
	CreatedAt  time.Time      `json:"created_at"`
	FinishedAt *time.Time     `json:"finished_at,omitempty"`
}

// CurrentStage returns the name of the currently executing stage, or empty if done.
func (w WorkflowRun) CurrentStage() string {
	if w.CurrentIdx >= 0 && w.CurrentIdx < len(w.Stages) {
		return w.Stages[w.CurrentIdx].Name
	}
	return ""
}

// WorkflowStatus represents the state of a workflow run.
type WorkflowStatus string

const (
	WorkflowRunning         WorkflowStatus = "running"
	WorkflowWaitingApproval WorkflowStatus = "waiting_approval"
	WorkflowCompleted       WorkflowStatus = "completed"
	WorkflowFailed          WorkflowStatus = "failed"
	WorkflowCancelled       WorkflowStatus = "cancelled"
)

// StageResult tracks the execution state and output of a single stage.
type StageResult struct {
	Name       string      `json:"name"`
	Status     StageStatus `json:"status"`
	Output     any         `json:"output,omitempty"`
	Error      string      `json:"error,omitempty"`
	DurationMS int         `json:"duration_ms,omitempty"`
	StartedAt  *time.Time  `json:"started_at,omitempty"`
}

// StageStatus represents the state of a stage within a workflow run.
type StageStatus string

const (
	StagePending   StageStatus = "pending"
	StageRunning   StageStatus = "running"
	StageCompleted StageStatus = "completed"
	StageFailed    StageStatus = "failed"
	StageSkipped   StageStatus = "skipped"
	StageWaiting   StageStatus = "waiting" // waiting for approval
)
