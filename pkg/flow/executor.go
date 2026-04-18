package flow

import (
	"context"
	"fmt"
	"strings"

	"github.com/NubeDev/bizzy/pkg/airunner"
	"github.com/NubeDev/bizzy/pkg/apps"
	"github.com/NubeDev/bizzy/pkg/services"
)

// SlackSender is the interface for sending Slack messages from flow nodes.
type SlackSender interface {
	SendMessage(ctx context.Context, channel, message, threadTS string) (any, error)
}

// EmailSender is the interface for sending emails from flow nodes.
type EmailSender interface {
	SendEmail(ctx context.Context, to, subject, body string) error
}

// --- Node executor framework ---

// NodeExecutor executes a single node type within a flow run.
type NodeExecutor interface {
	Execute(ctx context.Context, exec *ExecContext) (any, error)
}

// NodeExecutorFunc adapts a plain function into a NodeExecutor.
type NodeExecutorFunc func(ctx context.Context, exec *ExecContext) (any, error)

func (f NodeExecutorFunc) Execute(ctx context.Context, exec *ExecContext) (any, error) {
	return f(ctx, exec)
}

// ExecContext carries everything a node executor needs.
type ExecContext struct {
	Run      *FlowRun
	Node     *FlowNodeDef
	Def      *FlowDef // the flow definition (for subgraph execution)
	Inputs   map[string]any
	Services *Services
	// Engine reference for nodes that need subgraph execution (foreach).
	Engine *Engine
}

// JSRuntime returns a user-scoped JSRuntime via the factory, or nil if no
// factory is configured (callers fall back to a bare runtime).
func (ec *ExecContext) JSRuntime() *apps.JSRuntime {
	if ec.Services != nil && ec.Services.JSFactory != nil {
		return ec.Services.JSFactory(ec.Run.UserID)
	}
	return nil
}

// JSRuntimeFactory creates a JSRuntime scoped to a user. When wired with
// the app ecosystem, it returns a runtime with secrets, config, plugins, and
// tool calling. When nil, the engine falls back to a bare NewFlowRuntime.
type JSRuntimeFactory func(userID string) *apps.JSRuntime

// Services bundles external dependencies that node executors may use.
// Not every executor needs all of these — each takes what it needs.
type Services struct {
	Tools      services.ToolCaller
	Prompts    services.PromptRunner
	Agents     *airunner.Registry
	Slack      SlackSender
	Email      EmailSender
	Bus        EventPublisher
	JSFactory  JSRuntimeFactory // creates user-scoped JS runtimes for expression eval
}

// ExecutorRegistry maps node type names to their executors.
type ExecutorRegistry struct {
	executors map[string]NodeExecutor
	// prefixExecutors handle dynamic types like "tool:*".
	prefixExecutors map[string]NodeExecutor
}

// NewExecutorRegistry creates an empty executor registry.
func NewExecutorRegistry() *ExecutorRegistry {
	return &ExecutorRegistry{
		executors:       make(map[string]NodeExecutor),
		prefixExecutors: make(map[string]NodeExecutor),
	}
}

// Register adds an executor for a specific node type.
func (r *ExecutorRegistry) Register(nodeType string, exec NodeExecutor) {
	r.executors[nodeType] = exec
}

// RegisterFunc registers a function as an executor for a node type.
func (r *ExecutorRegistry) RegisterFunc(nodeType string, fn func(ctx context.Context, exec *ExecContext) (any, error)) {
	r.executors[nodeType] = NodeExecutorFunc(fn)
}

// RegisterPrefix adds an executor for all node types starting with the given prefix.
func (r *ExecutorRegistry) RegisterPrefix(prefix string, exec NodeExecutor) {
	r.prefixExecutors[prefix] = exec
}

// Get returns the executor for a node type, checking exact matches first, then prefixes.
func (r *ExecutorRegistry) Get(nodeType string) (NodeExecutor, bool) {
	if exec, ok := r.executors[nodeType]; ok {
		return exec, true
	}
	for prefix, exec := range r.prefixExecutors {
		if strings.HasPrefix(nodeType, prefix) {
			return exec, true
		}
	}
	return nil, false
}

// Dispatch finds and executes the right executor for a node.
func (r *ExecutorRegistry) Dispatch(ctx context.Context, exec *ExecContext) (any, error) {
	executor, ok := r.Get(exec.Node.Type)
	if !ok {
		return nil, fmt.Errorf("unknown node type: %s", exec.Node.Type)
	}
	return executor.Execute(ctx, exec)
}
