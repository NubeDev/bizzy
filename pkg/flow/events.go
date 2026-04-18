package flow

import "time"

// NATS topic constants for flow events.
const (
	TopicFlowStarted         = "flow.started"
	TopicFlowNodeStarted     = "flow.node.started"
	TopicFlowNodeCompleted   = "flow.node.completed"
	TopicFlowNodeFailed      = "flow.node.failed"
	TopicFlowNodeSkipped     = "flow.node.skipped"
	TopicFlowNodeProgress    = "flow.node.progress"
	TopicFlowWaitingApproval = "flow.waiting_approval"
	TopicFlowApproved        = "flow.approved"
	TopicFlowRejected        = "flow.rejected"
	TopicFlowCompleted       = "flow.completed"
	TopicFlowFailed          = "flow.failed"
	TopicFlowCancelled       = "flow.cancelled"
)

// FlowEvent is the payload published to the NATS event bus.
type FlowEvent struct {
	RunID      string    `json:"run_id"`
	FlowID     string    `json:"flow_id"`
	FlowName   string    `json:"flow_name"`
	NodeID     string    `json:"node_id,omitempty"`
	NodeType   string    `json:"node_type,omitempty"`
	UserID     string    `json:"user_id"`
	Status     string    `json:"status"`
	Output     any       `json:"output,omitempty"`
	Error      string    `json:"error,omitempty"`
	DurationMS int       `json:"duration_ms,omitempty"`
	ReplyTo    *ReplyInfo `json:"reply_to,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

// EventPublisher publishes events to the bus.
type EventPublisher interface {
	Publish(topic string, data any) error
}

// eventEmitter wraps the publisher with convenience methods.
type eventEmitter struct {
	bus EventPublisher
}

func (e *eventEmitter) publish(topic string, ev FlowEvent) {
	if e.bus == nil {
		return
	}
	ev.Timestamp = time.Now()
	e.bus.Publish(topic, ev)
}

func (e *eventEmitter) flowStarted(run *FlowRun) {
	e.publish(TopicFlowStarted, FlowEvent{
		RunID:    run.ID,
		FlowID:   run.FlowID,
		FlowName: run.FlowName,
		UserID:   run.UserID,
		Status:   string(run.Status),
		ReplyTo:  run.ReplyTo,
	})
}

func (e *eventEmitter) nodeStarted(run *FlowRun, node *FlowNodeDef) {
	e.publish(TopicFlowNodeStarted, FlowEvent{
		RunID:    run.ID,
		FlowID:   run.FlowID,
		FlowName: run.FlowName,
		NodeID:   node.ID,
		NodeType: node.Type,
		UserID:   run.UserID,
		Status:   string(NodeRunning),
	})
}

func (e *eventEmitter) nodeCompleted(run *FlowRun, node *FlowNodeDef, durationMS int) {
	e.publish(TopicFlowNodeCompleted, FlowEvent{
		RunID:      run.ID,
		FlowID:     run.FlowID,
		FlowName:   run.FlowName,
		NodeID:     node.ID,
		NodeType:   node.Type,
		UserID:     run.UserID,
		Status:     string(NodeCompleted),
		DurationMS: durationMS,
	})
}

func (e *eventEmitter) nodeFailed(run *FlowRun, node *FlowNodeDef, errMsg string) {
	e.publish(TopicFlowNodeFailed, FlowEvent{
		RunID:    run.ID,
		FlowID:   run.FlowID,
		FlowName: run.FlowName,
		NodeID:   node.ID,
		NodeType: node.Type,
		UserID:   run.UserID,
		Status:   string(NodeFailed),
		Error:    errMsg,
	})
}

func (e *eventEmitter) flowCompleted(run *FlowRun) {
	e.publish(TopicFlowCompleted, FlowEvent{
		RunID:    run.ID,
		FlowID:   run.FlowID,
		FlowName: run.FlowName,
		UserID:   run.UserID,
		Status:   string(FlowRunCompleted),
	})
}

func (e *eventEmitter) flowFailed(run *FlowRun, errMsg string) {
	e.publish(TopicFlowFailed, FlowEvent{
		RunID:    run.ID,
		FlowID:   run.FlowID,
		FlowName: run.FlowName,
		UserID:   run.UserID,
		Status:   string(FlowRunFailed),
		Error:    errMsg,
	})
}

func (e *eventEmitter) flowCancelled(run *FlowRun) {
	e.publish(TopicFlowCancelled, FlowEvent{
		RunID:    run.ID,
		FlowID:   run.FlowID,
		FlowName: run.FlowName,
		UserID:   run.UserID,
		Status:   string(FlowRunCancelled),
	})
}

func (e *eventEmitter) waitingApproval(run *FlowRun, nodeID string, input any) {
	e.publish(TopicFlowWaitingApproval, FlowEvent{
		RunID:    run.ID,
		FlowID:   run.FlowID,
		FlowName: run.FlowName,
		NodeID:   nodeID,
		UserID:   run.UserID,
		Status:   string(FlowRunWaitingApproval),
		Output:   input,
		ReplyTo:  run.ReplyTo,
	})
}
