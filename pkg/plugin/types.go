// Package plugin implements the separate-process plugin extension system.
// Plugins are independent processes that connect to bizzy's embedded NATS
// server and provide tools, prompts, workflows, adapters, or event handlers.
package plugin

import "time"

// ---------------------------------------------------------------------------
// Service types
// ---------------------------------------------------------------------------

// ServiceType declares what a plugin provides.
type ServiceType string

const (
	ServiceTools     ServiceType = "tools"
	ServicePrompts   ServiceType = "prompts"
	ServiceWorkflows ServiceType = "workflows"
	ServiceAdapter   ServiceType = "adapter"
	ServiceHandler   ServiceType = "handler"
)

// ValidServiceTypes is the set of recognised service types.
var ValidServiceTypes = map[ServiceType]bool{
	ServiceTools:     true,
	ServicePrompts:   true,
	ServiceWorkflows: true,
	ServiceAdapter:   true,
	ServiceHandler:   true,
}

// ---------------------------------------------------------------------------
// Manifest — the plugin's self-description (loaded from YAML or received
// over NATS as JSON). This is the single source of truth for everything
// the plugin provides.
// ---------------------------------------------------------------------------

// Manifest is the complete plugin declaration.
type Manifest struct {
	Name        string        `yaml:"name" json:"name"`
	Version     string        `yaml:"version" json:"version"`
	Description string        `yaml:"description" json:"description,omitempty"`
	Services    []ServiceType `yaml:"services" json:"services"`
	Tools       []ToolSpec    `yaml:"tools" json:"tools,omitempty"`
	Prompts     []PromptSpec  `yaml:"prompts" json:"prompts,omitempty"`
	Workflows   []WorkflowSpec `yaml:"workflows" json:"workflows,omitempty"`
	Adapter     *AdapterSpec  `yaml:"adapter" json:"adapter,omitempty"`
	Preamble    string        `yaml:"preamble" json:"preamble,omitempty"`
}

// HasService reports whether the manifest declares the given service type.
func (m *Manifest) HasService(s ServiceType) bool {
	for _, svc := range m.Services {
		if svc == s {
			return true
		}
	}
	return false
}

// ToolSpec describes a single tool a plugin provides.
type ToolSpec struct {
	Name        string         `yaml:"name" json:"name"`
	Description string         `yaml:"description" json:"description"`
	Parameters  map[string]any `yaml:"parameters" json:"parameters,omitempty"`
}

// PromptSpec describes a prompt template a plugin provides.
type PromptSpec struct {
	Name        string       `yaml:"name" json:"name"`
	Description string       `yaml:"description" json:"description"`
	Template    string       `yaml:"template" json:"template"`
	Arguments   []PromptArg  `yaml:"arguments" json:"arguments,omitempty"`
}

// PromptArg is a single substitution argument in a prompt template.
type PromptArg struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
	Required    bool   `yaml:"required" json:"required"`
}

// WorkflowSpec describes a workflow definition a plugin provides.
type WorkflowSpec struct {
	Name        string      `yaml:"name" json:"name"`
	Description string      `yaml:"description" json:"description"`
	Stages      []StageSpec `yaml:"stages" json:"stages"`
}

// StageSpec describes a single stage in a plugin-provided workflow.
type StageSpec struct {
	Name   string `yaml:"name" json:"name"`
	Tool   string `yaml:"tool" json:"tool,omitempty"`
	Type   string `yaml:"type" json:"type,omitempty"` // "approval", "prompt", etc.
	Prompt string `yaml:"prompt" json:"prompt,omitempty"`
}

// AdapterSpec configures a plugin that acts as a command bus channel.
type AdapterSpec struct {
	Channel     string         `yaml:"channel" json:"channel"`
	ParseConfig map[string]any `yaml:"parse_config" json:"parse_config,omitempty"`
}

// ---------------------------------------------------------------------------
// Runtime state — held in memory by the registry.
// ---------------------------------------------------------------------------

// PluginState is the in-memory runtime state for a registered plugin.
type PluginState struct {
	Manifest       Manifest
	Status         string    // "active", "crashed", "disabled"
	RegisteredAt   time.Time
	LastHeartbeat  time.Time
	HealthFailures int
}

// ---------------------------------------------------------------------------
// NATS message types — every message includes api_version so both sides
// can detect protocol mismatches.
// ---------------------------------------------------------------------------

// RegisterRequest is published by a plugin to extension.register.
// It is identical to the Manifest plus the api_version envelope field.
type RegisterRequest struct {
	APIVersion string `json:"api_version"`
	Manifest
}

// RegisterResponse is the reply sent back to the plugin.
type RegisterResponse struct {
	APIVersion      string `json:"api_version"`
	Status          string `json:"status"` // "ok" or "error"
	Error           string `json:"error,omitempty"`
	ToolsRegistered int    `json:"tools_registered"`
	Reloaded        bool   `json:"reloaded"`
}

// DeregisterRequest is published by a plugin to extension.deregister.
type DeregisterRequest struct {
	APIVersion string `json:"api_version"`
	Name       string `json:"name"`
}

// ToolCallRequest is sent by bizzy to tool.call.<plugin>.<tool>.
type ToolCallRequest struct {
	APIVersion string         `json:"api_version"`
	Params     map[string]any `json:"params"`
	Context    ToolCallCtx    `json:"context"`
}

// ToolCallCtx carries metadata about who initiated the tool call.
type ToolCallCtx struct {
	UserID    string `json:"user_id,omitempty"`
	CommandID string `json:"command_id,omitempty"`
	TimeoutMS int    `json:"timeout_ms"`
}

// ToolCallResponse is the reply from a plugin for a tool call.
type ToolCallResponse struct {
	APIVersion string `json:"api_version,omitempty"`
	Result     any    `json:"result,omitempty"`
	Error      string `json:"error,omitempty"`
	Retryable  bool   `json:"retryable,omitempty"`
}

// HealthMessage is published by a plugin to extension.health.<name>.
type HealthMessage struct {
	APIVersion string `json:"api_version"`
	Status     string `json:"status"` // "ok"
}

// ---------------------------------------------------------------------------
// NATS subject constants
// ---------------------------------------------------------------------------

const (
	SubjectRegister   = "extension.register"
	SubjectDeregister = "extension.deregister"
	SubjectHealthPrefix = "extension.health."   // + plugin name
	SubjectRegisteredPrefix = "extension.registered." // + plugin name
	SubjectToolCallPrefix   = "tool.call."            // + plugin.tool

	SubjectHealthWildcard = "extension.health.*"
)
