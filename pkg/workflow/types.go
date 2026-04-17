// Package workflow implements the multi-app staged workflow engine.
package workflow

// WorkflowDef is the parsed representation of a workflow YAML file.
type WorkflowDef struct {
	Name        string     `yaml:"name"`
	Description string     `yaml:"description"`
	Depends     []string   `yaml:"depends"`
	Inputs      []InputDef `yaml:"inputs"`
	Stages      []StageDef `yaml:"stages"`
}

// InputDef describes a user-provided input for a workflow.
type InputDef struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Required    bool     `yaml:"required"`
	Options     []string `yaml:"options,omitempty"`
	Default     string   `yaml:"default,omitempty"`
}

// StageDef describes a single stage in a workflow.
type StageDef struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`

	// Stage type — exactly one of these should be set.
	Tool     string         `yaml:"tool,omitempty"`     // MCP tool to call
	Prompt   string         `yaml:"prompt,omitempty"`   // AI prompt to execute
	Approval bool           `yaml:"approval,omitempty"` // pause for user approval
	Output   *OutputDef     `yaml:"output,omitempty"`   // deliver final result
	Type     string         `yaml:"type,omitempty"`     // "conditional" for switch stages
	Switch   string         `yaml:"switch,omitempty"`   // value to switch on
	Cases    map[string]CaseDef `yaml:"cases,omitempty"` // conditional branches

	// Common fields.
	Params   map[string]string `yaml:"params,omitempty"`
	SaveAs   string            `yaml:"save_as,omitempty"`
	OnFail   string            `yaml:"on_fail,omitempty"`   // stop (default), retry, skip, fallback
	OnReject string            `yaml:"on_reject,omitempty"` // for approval stages: retry, stop

	// Approval stage fields.
	Show string `yaml:"show,omitempty"` // template to display for approval

	// Retry settings.
	MaxRetries int `yaml:"max_retries,omitempty"`

	// Timeout in seconds (0 = use default).
	TimeoutSec int `yaml:"timeout_sec,omitempty"`
}

// StageType returns the type of this stage.
func (s StageDef) StageType() string {
	switch {
	case s.Type == "conditional":
		return "conditional"
	case s.Tool != "":
		return "tool"
	case s.Prompt != "":
		return "prompt"
	case s.Approval:
		return "approval"
	case s.Output != nil:
		return "output"
	default:
		return "unknown"
	}
}

// OutputDef describes the final output of a workflow.
type OutputDef struct {
	Message string `yaml:"message"`
	File    string `yaml:"file,omitempty"`
	Content string `yaml:"content,omitempty"`
}

// CaseDef describes a single branch in a conditional stage.
type CaseDef struct {
	Tool   string            `yaml:"tool,omitempty"`
	Prompt string            `yaml:"prompt,omitempty"`
	Params map[string]string `yaml:"params,omitempty"`
}
