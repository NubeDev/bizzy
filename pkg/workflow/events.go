package workflow

// Event topics published by the workflow runner.
const (
	TopicStarted          = "workflow.started"
	TopicStageStarted     = "workflow.stage.started"
	TopicStageCompleted   = "workflow.stage.completed"
	TopicStageFailed      = "workflow.stage.failed"
	TopicWaitingApproval  = "workflow.waiting_approval"
	TopicApproved         = "workflow.approved"
	TopicRejected         = "workflow.rejected"
	TopicCompleted        = "workflow.completed"
	TopicFailed           = "workflow.failed"
	TopicCancelled        = "workflow.cancelled"
)

// RunEvent carries workflow lifecycle data for the event bus.
type RunEvent struct {
	RunID      string `json:"run_id"`
	AppName    string `json:"app_name"`
	Workflow   string `json:"workflow"`
	UserID     string `json:"user_id"`
	Stage      string `json:"stage,omitempty"`
	Status     string `json:"status,omitempty"`
	DurationMS int    `json:"duration_ms,omitempty"`
	Error      string `json:"error,omitempty"`
	Output     any    `json:"output,omitempty"`
}
