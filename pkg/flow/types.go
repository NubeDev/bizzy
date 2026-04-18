// Package flow implements a DAG-based visual workflow engine.
// It provides a graph execution runtime where nodes represent operations
// (tools, AI prompts, integrations) and edges define data flow between them.
package flow

import (
	"errors"
	"time"
)

// --- Flow Definition (saved to DB) ---

// FlowDef is the persisted definition of a flow — nodes, edges, and metadata.
type FlowDef struct {
	ID          string            `json:"id" gorm:"primaryKey"`
	Name        string            `json:"name" gorm:"uniqueIndex"`
	Description string            `json:"description"`
	Version     int               `json:"version"`
	Nodes       []FlowNodeDef     `json:"nodes" gorm:"serializer:json"`
	Edges       []FlowEdgeDef     `json:"edges" gorm:"serializer:json"`
	Inputs      []FlowInputDef    `json:"inputs,omitempty" gorm:"serializer:json"`
	Trigger     *TriggerDef       `json:"trigger,omitempty" gorm:"serializer:json"`
	Settings    map[string]any    `json:"settings,omitempty" gorm:"serializer:json"`
	UserID      string            `json:"user_id" gorm:"index"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// GetNode returns the node definition with the given ID, or nil.
func (f *FlowDef) GetNode(id string) *FlowNodeDef {
	for i := range f.Nodes {
		if f.Nodes[i].ID == id {
			return &f.Nodes[i]
		}
	}
	return nil
}

// EdgesFrom returns all edges originating from the given node.
func (f *FlowDef) EdgesFrom(nodeID string) []FlowEdgeDef {
	var out []FlowEdgeDef
	for _, e := range f.Edges {
		if e.Source == nodeID {
			out = append(out, e)
		}
	}
	return out
}

// EdgesTo returns all edges targeting the given node.
func (f *FlowDef) EdgesTo(nodeID string) []FlowEdgeDef {
	var out []FlowEdgeDef
	for _, e := range f.Edges {
		if e.Target == nodeID {
			out = append(out, e)
		}
	}
	return out
}

// FlowNodeDef is a node on the canvas.
type FlowNodeDef struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Label    string         `json:"label,omitempty"`
	Position Position       `json:"position"`
	Data     map[string]any `json:"data,omitempty"`
	Ports    *PortsDef      `json:"ports,omitempty"`
}

// Position holds x/y coordinates for React Flow rendering.
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// PortsDef allows nodes to declare custom ports beyond the defaults.
type PortsDef struct {
	Inputs  []PortDef `json:"inputs,omitempty"`
	Outputs []PortDef `json:"outputs,omitempty"`
}

// PortDef describes a single input or output port on a node.
type PortDef struct {
	Handle   string `json:"handle"`
	Label    string `json:"label,omitempty"`
	Type     string `json:"type,omitempty"` // "any", "string", "number", "bool", "object"
	Required bool   `json:"required,omitempty"`
}

// FlowEdgeDef is a connection between two nodes (source port → target port).
type FlowEdgeDef struct {
	ID           string `json:"id"`
	Source       string `json:"source"`
	SourceHandle string `json:"sourceHandle"`
	Target       string `json:"target"`
	TargetHandle string `json:"targetHandle"`
	Condition    string `json:"condition,omitempty"`
	Label        string `json:"label,omitempty"`
}

// FlowInputDef describes an input parameter the flow accepts.
type FlowInputDef struct {
	Name        string `json:"name"`
	Type        string `json:"type,omitempty"`
	Description string `json:"description,omitempty"`
	Default     any    `json:"default,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// TriggerDef defines what starts a flow automatically.
type TriggerDef struct {
	Type     string         `json:"type"`               // "manual", "cron", "webhook", "event"
	Schedule string         `json:"schedule,omitempty"`  // cron expression
	Event    string         `json:"event,omitempty"`     // NATS topic pattern
	Filter   map[string]any `json:"filter,omitempty"`
}

// TriggerConfig returns the trigger node's Data map, which holds the trigger
// configuration (type, schedule, etc). Returns nil if no trigger node exists
// or if the trigger is manual.
func (f *FlowDef) TriggerConfig() map[string]any {
	for _, n := range f.Nodes {
		if n.Type == "trigger" {
			return n.Data
		}
	}
	return nil
}

// --- Flow Run (execution state) ---

// FlowRun tracks a single execution of a flow.
type FlowRun struct {
	ID          string               `json:"id" gorm:"primaryKey"`
	FlowID      string               `json:"flow_id" gorm:"index"`
	FlowVersion int                  `json:"flow_version"`
	FlowName    string               `json:"flow_name"`
	Status      FlowRunStatus        `json:"status" gorm:"index"`
	Inputs      map[string]any       `json:"inputs,omitempty" gorm:"serializer:json"`
	Output      map[string]any       `json:"output,omitempty" gorm:"serializer:json"`
	NodeStates  map[string]NodeState `json:"node_states" gorm:"serializer:json"`
	Variables   map[string]any       `json:"variables,omitempty" gorm:"serializer:json"`
	Error       string               `json:"error,omitempty"`
	UserID      string               `json:"user_id" gorm:"index"`
	ReplyTo     *ReplyInfo           `json:"reply_to,omitempty" gorm:"serializer:json"`
	CreatedAt   time.Time            `json:"created_at"`
	FinishedAt  *time.Time           `json:"finished_at,omitempty"`
}

// FlowRunStatus represents the state of a flow run.
type FlowRunStatus string

const (
	FlowRunPending         FlowRunStatus = "pending"
	FlowRunRunning         FlowRunStatus = "running"
	FlowRunWaitingApproval FlowRunStatus = "waiting_approval"
	FlowRunCompleted       FlowRunStatus = "completed"
	FlowRunFailed          FlowRunStatus = "failed"
	FlowRunCancelled       FlowRunStatus = "cancelled"
)

// NodeState tracks per-node execution within a run.
type NodeState struct {
	Status     NodeStatus `json:"status"`
	Input      any        `json:"input,omitempty"`
	Output     any        `json:"output,omitempty"`
	Error      string     `json:"error,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	DurationMS int        `json:"duration_ms,omitempty"`
	Retries    int        `json:"retries,omitempty"`
}

// NodeStatus represents the state of a node within a run.
type NodeStatus string

const (
	NodePending   NodeStatus = "pending"
	NodeReady     NodeStatus = "ready"
	NodeRunning   NodeStatus = "running"
	NodeCompleted NodeStatus = "completed"
	NodeFailed    NodeStatus = "failed"
	NodeSkipped   NodeStatus = "skipped"
	NodeWaiting   NodeStatus = "waiting" // approval gate
)

// ReplyInfo describes where to send the flow result (e.g. Slack thread).
type ReplyInfo struct {
	Adapter   string `json:"adapter"`
	Channel   string `json:"channel,omitempty"`
	ThreadTS  string `json:"thread_ts,omitempty"`
	MessageID string `json:"message_id,omitempty"`
}

// PortOutput is returned by nodes with multiple output ports to indicate
// which port the output should be routed through.
type PortOutput struct {
	Port  string `json:"port"`
	Value any    `json:"value"`
}

// RaceOutput extends PortOutput with the winning port name.
type RaceOutput struct {
	Port   string `json:"port"`
	Value  any    `json:"value"`
	Winner string `json:"winner"`
}

// nodeResult is sent by worker goroutines back to the engine's main loop.
type nodeResult struct {
	NodeID     string
	Output     any
	Error      error
	StartedAt  time.Time
	FinishedAt time.Time
}

// ErrApprovalRequired is a sentinel error returned by approval nodes
// to signal the main loop to pause the run.
var ErrApprovalRequired = errors.New("approval required")
