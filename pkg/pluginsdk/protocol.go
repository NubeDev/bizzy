package pluginsdk

// Protocol constants and message types for the bizzy plugin protocol.
// These are intentionally duplicated from pkg/plugin/types.go so that
// the SDK has zero dependencies on the server codebase. External plugin
// projects only need this package + nats.go.

// ProtocolVersion is the plugin protocol version this SDK implements.
// Must match pkg/version.PluginProtocol on the server side.
const ProtocolVersion = "1.0.0"

// NATS subjects.
const (
	subjectRegister   = "extension.register"
	subjectDeregister = "extension.deregister"
	subjectHealthPfx  = "extension.health." // + plugin name
	subjectToolCallPfx = "tool.call."       // + plugin.tool
)

// Service types.
const (
	svcTools     = "tools"
	svcPrompts   = "prompts"
	svcWorkflows = "workflows"
	svcAdapter   = "adapter"
	svcHandler   = "handler"
)

// --- Wire types (JSON over NATS) ---

type registerRequest struct {
	APIVersion  string           `json:"api_version"`
	Name        string           `json:"name"`
	Version     string           `json:"version"`
	Description string           `json:"description,omitempty"`
	Services    []string         `json:"services"`
	Tools       []toolSpec       `json:"tools,omitempty"`
	Prompts     []promptSpec     `json:"prompts,omitempty"`
	Workflows   []workflowSpec   `json:"workflows,omitempty"`
	Adapter     *adapterSpec     `json:"adapter,omitempty"`
	Preamble    string           `json:"preamble,omitempty"`
}

type registerResponse struct {
	APIVersion      string `json:"api_version"`
	Status          string `json:"status"` // "ok" or "error"
	Error           string `json:"error,omitempty"`
	ToolsRegistered int    `json:"tools_registered"`
	Reloaded        bool   `json:"reloaded"`
}

type deregisterRequest struct {
	APIVersion string `json:"api_version"`
	Name       string `json:"name"`
}

type healthMessage struct {
	APIVersion string `json:"api_version"`
	Status     string `json:"status"`
}

type toolCallRequest struct {
	APIVersion string         `json:"api_version"`
	Params     map[string]any `json:"params"`
	Context    toolCallCtx    `json:"context"`
}

type toolCallCtx struct {
	UserID    string `json:"user_id,omitempty"`
	CommandID string `json:"command_id,omitempty"`
	TimeoutMS int    `json:"timeout_ms"`
}

type toolCallResponse struct {
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

// --- Manifest sub-types ---

type toolSpec struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type promptSpec struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Template    string      `json:"template"`
	Arguments   []promptArg `json:"arguments,omitempty"`
}

type promptArg struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

type workflowSpec struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Stages      []stageSpec `json:"stages"`
}

type stageSpec struct {
	Name   string `json:"name"`
	Tool   string `json:"tool,omitempty"`
	Type   string `json:"type,omitempty"`
	Prompt string `json:"prompt,omitempty"`
}

type adapterSpec struct {
	Channel     string         `json:"channel"`
	ParseConfig map[string]any `json:"parse_config,omitempty"`
}
