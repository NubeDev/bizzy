package flow

import (
	"fmt"
	"sync"

	"github.com/NubeDev/bizzy/pkg/flow/settings"
)

// NodeTypeDef describes a node type that can be placed on the canvas.
type NodeTypeDef struct {
	Type        string   `json:"type"`
	Label       string   `json:"label"`
	Description string   `json:"description,omitempty"`
	Category    string   `json:"category"` // "flow-control", "tool", "integration", "data"
	Icon        string   `json:"icon,omitempty"`
	Source      string   `json:"source"`  // "builtin", "app", "plugin"
	Ports       PortsDef `json:"ports"`
	Settings    any      `json:"settings,omitempty"` // JSON Schema for node config panel
}

// NodeTypeRegistry holds all available node types.
type NodeTypeRegistry struct {
	mu    sync.RWMutex
	types map[string]NodeTypeDef
}

// NewRegistry creates a registry pre-populated with built-in node types.
func NewRegistry() *NodeTypeRegistry {
	r := &NodeTypeRegistry{types: make(map[string]NodeTypeDef)}
	r.registerBuiltins()
	return r
}

// Register adds or replaces a node type.
func (r *NodeTypeRegistry) Register(def NodeTypeDef) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.types[def.Type] = def
}

// Get returns a node type definition.
func (r *NodeTypeRegistry) Get(typ string) (NodeTypeDef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.types[typ]
	return d, ok
}

// All returns all registered node types.
func (r *NodeTypeRegistry) All() []NodeTypeDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]NodeTypeDef, 0, len(r.types))
	for _, d := range r.types {
		out = append(out, d)
	}
	return out
}

// Has checks if a node type is registered.
func (r *NodeTypeRegistry) Has(typ string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.types[typ]
	return ok
}

// Remove removes a node type.
func (r *NodeTypeRegistry) Remove(typ string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.types, typ)
}

// RegisterTool registers a tool as a placeable node type.
// Called when apps are installed or plugins register tools.
func (r *NodeTypeRegistry) RegisterTool(toolType, label, description, category, source string, inputPorts, outputPorts []PortDef) {
	r.Register(NodeTypeDef{
		Type:        toolType,
		Label:       label,
		Description: description,
		Category:    category,
		Source:      source,
		Ports: PortsDef{
			Inputs:  inputPorts,
			Outputs: outputPorts,
		},
	})
}

// registerBuiltins registers all built-in flow control, integration, and data nodes.
func (r *NodeTypeRegistry) registerBuiltins() {
	// Flow control nodes
	r.Register(NodeTypeDef{
		Type: "trigger", Label: "Trigger", Category: "flow-control", Source: "builtin",
		Description: "Entry point of the flow. Emits the flow inputs.",
		Ports: PortsDef{
			Outputs: []PortDef{{Handle: "output", Label: "Output", Type: "any"}},
		},
	})
	r.Register(NodeTypeDef{
		Type: "approval", Label: "Approval", Category: "flow-control", Source: "builtin",
		Description: "Pauses execution and waits for user approval or rejection.",
		Ports: PortsDef{
			Inputs:  []PortDef{{Handle: "input", Label: "Input", Type: "any", Required: true}},
			Outputs: []PortDef{{Handle: "approved", Label: "Approved", Type: "any"}, {Handle: "rejected", Label: "Rejected", Type: "any"}},
		},
	})
	r.Register(NodeTypeDef{
		Type: "condition", Label: "Condition", Category: "flow-control", Source: "builtin",
		Description: "Evaluates an expression and routes to true or false output.",
		Ports: PortsDef{
			Inputs:  []PortDef{{Handle: "input", Label: "Input", Type: "any", Required: true}},
			Outputs: []PortDef{{Handle: "true", Label: "True", Type: "any"}, {Handle: "false", Label: "False", Type: "any"}},
		},
	})
	r.Register(NodeTypeDef{
		Type: "switch", Label: "Switch", Category: "flow-control", Source: "builtin",
		Description: "Multi-way branch. Routes to matching case output port.",
		Ports: PortsDef{
			Inputs:  []PortDef{{Handle: "input", Label: "Input", Type: "any", Required: true}},
			Outputs: []PortDef{{Handle: "default", Label: "Default", Type: "any"}},
		},
	})
	r.Register(NodeTypeDef{
		Type: "merge", Label: "Merge", Category: "flow-control", Source: "builtin",
		Description: "Fan-in join. Waits for all connected inputs, then emits merged result.",
		Ports: PortsDef{
			Inputs:  []PortDef{{Handle: "input_1", Label: "Input 1", Type: "any"}, {Handle: "input_2", Label: "Input 2", Type: "any"}},
			Outputs: []PortDef{{Handle: "output", Label: "Output", Type: "any"}},
		},
	})
	r.Register(NodeTypeDef{
		Type: "race", Label: "Race", Category: "flow-control", Source: "builtin",
		Description: "Fan-in first. Emits the first input that arrives.",
		Ports: PortsDef{
			Inputs:  []PortDef{{Handle: "input_1", Label: "Input 1", Type: "any"}, {Handle: "input_2", Label: "Input 2", Type: "any"}},
			Outputs: []PortDef{{Handle: "output", Label: "Output", Type: "any"}, {Handle: "winner", Label: "Winner", Type: "string"}},
		},
	})
	r.Register(NodeTypeDef{
		Type: "foreach", Label: "ForEach", Category: "flow-control", Source: "builtin",
		Description: "Iterates over array input. Executes downstream subgraph per item.",
		Ports: PortsDef{
			Inputs:  []PortDef{{Handle: "items", Label: "Items", Type: "any", Required: true}},
			Outputs: []PortDef{{Handle: "item", Label: "Item", Type: "any"}, {Handle: "done", Label: "Done", Type: "any"}},
		},
	})
	r.Register(NodeTypeDef{
		Type: "delay", Label: "Delay", Category: "flow-control", Source: "builtin",
		Description: "Waits for configured duration, then passes input through.",
		Ports: PortsDef{
			Inputs:  []PortDef{{Handle: "input", Label: "Input", Type: "any", Required: true}},
			Outputs: []PortDef{{Handle: "output", Label: "Output", Type: "any"}},
		},
	})
	r.Register(NodeTypeDef{
		Type: "output", Label: "Output", Category: "flow-control", Source: "builtin",
		Description: "Terminal node. Marks the final result of the flow.",
		Ports: PortsDef{
			Inputs: []PortDef{{Handle: "input", Label: "Input", Type: "any", Required: true}},
		},
	})
	r.Register(NodeTypeDef{
		Type: "error", Label: "Error", Category: "flow-control", Source: "builtin",
		Description: "Terminal error node. Marks the flow as failed.",
		Ports: PortsDef{
			Inputs: []PortDef{{Handle: "input", Label: "Input", Type: "any", Required: true}},
		},
	})

	// Data nodes
	r.Register(NodeTypeDef{
		Type: "debug", Label: "Debug", Category: "data", Source: "builtin",
		Description: "Passthrough node that captures messages for inspection. Shows in the debug panel without modifying the flow.",
		Ports: PortsDef{
			Inputs:  []PortDef{{Handle: "input", Label: "Input", Type: "any", Required: true}},
			Outputs: []PortDef{{Handle: "output", Label: "Output", Type: "any"}},
		},
	})
	r.Register(NodeTypeDef{
		Type: "function", Label: "Function", Category: "data", Source: "builtin",
		Description: "Write JavaScript to transform msg. Has access to flow state, tools, and platform APIs. Like Node-RED's function node.",
		Ports: PortsDef{
			Inputs:  []PortDef{{Handle: "input", Label: "Input", Type: "any", Required: true}},
			Outputs: []PortDef{{Handle: "output", Label: "Output", Type: "any"}},
		},
	})
	r.Register(NodeTypeDef{
		Type: "value", Label: "Value", Category: "data", Source: "builtin",
		Description: "Emits a static value configured on the node. Great for testing.",
		Ports: PortsDef{
			Outputs: []PortDef{{Handle: "output", Label: "Output", Type: "any"}},
		},
	})
	r.Register(NodeTypeDef{
		Type: "template", Label: "Template", Category: "data", Source: "builtin",
		Description: "Formats a Go template string using input values and flow variables.",
		Ports: PortsDef{
			Inputs:  []PortDef{{Handle: "input", Label: "Input", Type: "any"}},
			Outputs: []PortDef{{Handle: "output", Label: "Output", Type: "string"}},
		},
	})
	r.Register(NodeTypeDef{
		Type: "http-request", Label: "HTTP Request", Category: "data", Source: "builtin",
		Description: "Makes an HTTP request. msg.url, msg.method, msg.headers override settings. msg.payload becomes the body for POST/PUT.",
		Ports: PortsDef{
			Inputs:  []PortDef{{Handle: "input", Label: "Input", Type: "any"}},
			Outputs: []PortDef{{Handle: "output", Label: "Output", Type: "any"}},
		},
	})
	r.Register(NodeTypeDef{
		Type: "transform", Label: "Transform", Category: "data", Source: "builtin",
		Description: "Applies an expression to reshape data between nodes.",
		Ports: PortsDef{
			Inputs:  []PortDef{{Handle: "input", Label: "Input", Type: "any", Required: true}},
			Outputs: []PortDef{{Handle: "output", Label: "Output", Type: "any"}},
		},
	})
	r.Register(NodeTypeDef{
		Type: "set-variable", Label: "Set Variable", Category: "data", Source: "builtin",
		Description: "Stores a value into the flow's variable map.",
		Ports: PortsDef{
			Inputs:  []PortDef{{Handle: "input", Label: "Input", Type: "any", Required: true}},
			Outputs: []PortDef{{Handle: "output", Label: "Output", Type: "any"}},
		},
	})
	r.Register(NodeTypeDef{
		Type: "log", Label: "Log", Category: "data", Source: "builtin",
		Description: "Logs the input value to the flow run record (passthrough).",
		Ports: PortsDef{
			Inputs:  []PortDef{{Handle: "input", Label: "Input", Type: "any", Required: true}},
			Outputs: []PortDef{{Handle: "output", Label: "Output", Type: "any"}},
		},
	})

	r.Register(NodeTypeDef{
		Type: "counter", Label: "Counter", Category: "data", Source: "builtin",
		Description: "Increments, decrements, resets, or sets a counter stored in flow variables.",
		Ports: PortsDef{
			Inputs:  []PortDef{{Handle: "input", Label: "Input", Type: "any"}},
			Outputs: []PortDef{{Handle: "output", Label: "Output", Type: "object"}},
		},
	})

	// Integration nodes
	r.Register(NodeTypeDef{
		Type: "ai-prompt", Label: "AI Prompt", Category: "integration", Source: "builtin",
		Description: "Run an AI prompt. msg.payload or msg.prompt is the prompt text. msg.provider/model override settings.",
		Ports: PortsDef{
			Inputs:  []PortDef{{Handle: "input", Label: "Input", Type: "any", Required: true}},
			Outputs: []PortDef{{Handle: "output", Label: "Output", Type: "any"}},
		},
	})
	r.Register(NodeTypeDef{
		Type: "ai-runner", Label: "AI Runner", Category: "integration", Source: "builtin",
		Description: "Run a full AI coding session. msg.payload or msg.prompt is the prompt. msg.work_dir/provider/model override settings.",
		Ports: PortsDef{
			Inputs:  []PortDef{{Handle: "input", Label: "Input", Type: "any", Required: true}},
			Outputs: []PortDef{{Handle: "output", Label: "Output", Type: "any"}},
		},
	})
	r.Register(NodeTypeDef{
		Type: "slack-send", Label: "Slack Send", Category: "integration", Source: "builtin",
		Description: "Send a Slack message. msg.payload is the message text. msg.channel/thread_ts override settings.",
		Ports: PortsDef{
			Inputs:  []PortDef{{Handle: "input", Label: "Input", Type: "any", Required: true}},
			Outputs: []PortDef{{Handle: "output", Label: "Output", Type: "any"}},
		},
	})
	r.Register(NodeTypeDef{
		Type: "email-send", Label: "Email Send", Category: "integration", Source: "builtin",
		Description: "Send an email. msg.payload is the body. msg.to/subject override settings.",
		Ports: PortsDef{
			Inputs:  []PortDef{{Handle: "input", Label: "Input", Type: "any", Required: true}},
			Outputs: []PortDef{{Handle: "output", Label: "Output", Type: "any"}},
		},
	})
	r.Register(NodeTypeDef{
		Type: "webhook-call", Label: "Webhook Call", Category: "integration", Source: "builtin",
		Description: "HTTP request. msg.url/method/headers override settings. msg.payload is the body.",
		Ports: PortsDef{
			Inputs:  []PortDef{{Handle: "input", Label: "Input", Type: "any"}},
			Outputs: []PortDef{{Handle: "output", Label: "Output", Type: "any"}},
		},
	})

	// Attach JSON Schema settings to each built-in node type.
	for nodeType, schema := range settings.BuiltinSchemas() {
		if def, ok := r.types[nodeType]; ok {
			def.Settings = schema
			r.types[nodeType] = def
		}
	}

	fmt.Printf("[flow] registered %d built-in node types\n", len(r.types))
}
