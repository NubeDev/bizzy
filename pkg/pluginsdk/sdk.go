// Package pluginsdk is the Go client library for building bizzy plugins.
//
// A plugin is a separate process that connects to bizzy's embedded NATS
// server and provides one or more services: tools, prompts, workflows,
// adapters, or event handlers. This package handles the full protocol
// lifecycle so you can focus on your business logic.
//
// This package has no dependencies on the bizzy server — only nats.go.
// External plugin projects can import it without pulling in the server.
//
// # Quick start (tools plugin)
//
//	p := pluginsdk.NewPlugin("weather", "1.0.0", "Weather tools")
//
//	p.AddTool(pluginsdk.Tool{
//	    Name:        "get_forecast",
//	    Description: "Get weather forecast for a location",
//	    Parameters:  pluginsdk.Params("location", "string", "City name", true),
//	    Handler: func(params map[string]any) (any, error) {
//	        city, _ := params["location"].(string)
//	        return map[string]any{"city": city, "temp": 22}, nil
//	    },
//	})
//
//	log.Fatal(p.Run())
//
// # Quick start (prompts plugin)
//
//	p := pluginsdk.NewPlugin("templates", "1.0.0", "Prompt templates")
//
//	p.AddPrompt(pluginsdk.Prompt{
//	    Name:        "code_review",
//	    Description: "Review code for issues",
//	    Template:    "Review this {{language}} code:\n\n{{code}}",
//	    Arguments: []pluginsdk.PromptArg{
//	        {Name: "language", Description: "Programming language", Required: true},
//	        {Name: "code", Description: "Code to review", Required: true},
//	    },
//	})
//
//	log.Fatal(p.Run())
package pluginsdk

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
)

// ---------------------------------------------------------------------------
// Public types — what plugin developers work with
// ---------------------------------------------------------------------------

// ToolHandler handles a tool call. It receives the params map from the AI
// and returns a result (any JSON-serializable value) or an error.
type ToolHandler func(params map[string]any) (any, error)

// Tool describes a tool the plugin provides.
type Tool struct {
	Name        string
	Description string
	Parameters  map[string]any // JSON Schema object, or nil
	Handler     ToolHandler
}

// Prompt describes a prompt template the plugin provides.
type Prompt struct {
	Name        string
	Description string
	Template    string
	Arguments   []PromptArg
}

// PromptArg is a substitution variable in a prompt template.
type PromptArg struct {
	Name        string
	Description string
	Required    bool
}

// Workflow describes a workflow the plugin provides.
type Workflow struct {
	Name        string
	Description string
	Stages      []WorkflowStage
}

// WorkflowStage is a single stage in a workflow.
type WorkflowStage struct {
	Name   string
	Tool   string // tool to call (optional)
	Type   string // "approval", "prompt", etc. (optional)
	Prompt string // prompt text (optional)
}

// Adapter configures the plugin as a command bus channel.
type Adapter struct {
	Channel     string
	ParseConfig map[string]any
}

// HandlerSubscription subscribes to a NATS subject for direct event handling.
type HandlerSubscription struct {
	Subject string
	Queue   string // optional queue group
	Handler func(subject string, data []byte)
}

// ---------------------------------------------------------------------------
// Plugin builder
// ---------------------------------------------------------------------------

// Plugin is the client handle. Create with NewPlugin, add services, call Run.
type Plugin struct {
	name        string
	ver         string
	description string
	preamble    string

	tools     map[string]Tool
	prompts   []Prompt
	workflows []Workflow
	adapter   *Adapter
	handlers  []HandlerSubscription

	natsURL      string
	heartbeatSec int
	logger       *log.Logger
}

// NewPlugin creates a new plugin. Name must be lowercase alphanumeric
// (matching ^[a-z][a-z0-9_-]{0,62}$).
func NewPlugin(name, ver, description string) *Plugin {
	return &Plugin{
		name:         name,
		ver:          ver,
		description:  description,
		tools:        make(map[string]Tool),
		natsURL:      envOrDefault("NATS_URL", "nats://127.0.0.1:4222"),
		heartbeatSec: 10,
		logger:       log.New(os.Stderr, fmt.Sprintf("[%s] ", name), log.LstdFlags),
	}
}

// AddTool registers a tool the AI can call.
func (p *Plugin) AddTool(t Tool) *Plugin {
	p.tools[t.Name] = t
	return p
}

// AddPrompt registers a prompt template.
func (p *Plugin) AddPrompt(pr Prompt) *Plugin {
	p.prompts = append(p.prompts, pr)
	return p
}

// AddWorkflow registers a workflow definition.
func (p *Plugin) AddWorkflow(w Workflow) *Plugin {
	p.workflows = append(p.workflows, w)
	return p
}

// SetAdapter configures this plugin as a command bus adapter.
func (p *Plugin) SetAdapter(a Adapter) *Plugin {
	p.adapter = &a
	return p
}

// AddHandler subscribes to a NATS subject for direct event handling.
// The plugin self-subscribes — bizzy does no wiring for handler services.
func (p *Plugin) AddHandler(h HandlerSubscription) *Plugin {
	p.handlers = append(p.handlers, h)
	return p
}

// SetPreamble sets an AI preamble injected when the plugin's tools are active.
func (p *Plugin) SetPreamble(s string) *Plugin {
	p.preamble = s
	return p
}

// SetNATSURL overrides the NATS URL (default: $NATS_URL or nats://127.0.0.1:4222).
func (p *Plugin) SetNATSURL(url string) *Plugin {
	p.natsURL = url
	return p
}

// SetHeartbeatInterval overrides the heartbeat interval in seconds (default: 10, range: 5-60).
func (p *Plugin) SetHeartbeatInterval(seconds int) *Plugin {
	if seconds < 5 {
		seconds = 5
	}
	if seconds > 60 {
		seconds = 60
	}
	p.heartbeatSec = seconds
	return p
}

// SetLogger overrides the default stderr logger.
func (p *Plugin) SetLogger(l *log.Logger) *Plugin {
	p.logger = l
	return p
}

// Run connects to NATS, registers the plugin, subscribes to tool calls,
// starts the heartbeat, and blocks until SIGINT/SIGTERM. Returns nil
// on clean shutdown.
func (p *Plugin) Run() error {
	if err := p.validate(); err != nil {
		return err
	}

	nc, err := nats.Connect(p.natsURL,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		return fmt.Errorf("nats connect: %w", err)
	}
	defer nc.Close()
	p.logger.Printf("connected to %s", p.natsURL)

	if err := p.register(nc); err != nil {
		return fmt.Errorf("register: %w", err)
	}

	if err := p.subscribe(nc); err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}

	go p.heartbeat(nc)

	p.logReady()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	p.deregister(nc)
	return nil
}

// ---------------------------------------------------------------------------
// Param helpers — convenience builders for JSON Schema
// ---------------------------------------------------------------------------

// Params builds a JSON Schema object with a single parameter.
// Chain with ParamsAdd for multiple parameters.
func Params(name, typ, description string, required bool) map[string]any {
	s := map[string]any{
		"type": "object",
		"properties": map[string]any{
			name: map[string]any{
				"type":        typ,
				"description": description,
			},
		},
	}
	if required {
		s["required"] = []string{name}
	}
	return s
}

// ParamsAdd adds a parameter to an existing schema built with Params.
func ParamsAdd(schema map[string]any, name, typ, description string, required bool) map[string]any {
	props, _ := schema["properties"].(map[string]any)
	if props == nil {
		props = make(map[string]any)
		schema["properties"] = props
	}
	props[name] = map[string]any{
		"type":        typ,
		"description": description,
	}
	if required {
		req, _ := schema["required"].([]string)
		schema["required"] = append(req, name)
	}
	return schema
}

// ---------------------------------------------------------------------------
// Internal
// ---------------------------------------------------------------------------

func (p *Plugin) validate() error {
	hasService := len(p.tools) > 0 || len(p.prompts) > 0 ||
		len(p.workflows) > 0 || p.adapter != nil || len(p.handlers) > 0
	if !hasService {
		return fmt.Errorf("plugin has no services — add at least one tool, prompt, workflow, adapter, or handler")
	}
	return nil
}

func (p *Plugin) services() []string {
	var svc []string
	if len(p.tools) > 0 {
		svc = append(svc, svcTools)
	}
	if len(p.prompts) > 0 {
		svc = append(svc, svcPrompts)
	}
	if len(p.workflows) > 0 {
		svc = append(svc, svcWorkflows)
	}
	if p.adapter != nil {
		svc = append(svc, svcAdapter)
	}
	if len(p.handlers) > 0 {
		svc = append(svc, svcHandler)
	}
	return svc
}

func (p *Plugin) buildManifest() registerRequest {
	var tools []toolSpec
	for _, t := range p.tools {
		tools = append(tools, toolSpec{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
		})
	}

	var prompts []promptSpec
	for _, pr := range p.prompts {
		var args []promptArg
		for _, a := range pr.Arguments {
			args = append(args, promptArg{
				Name:        a.Name,
				Description: a.Description,
				Required:    a.Required,
			})
		}
		prompts = append(prompts, promptSpec{
			Name:        pr.Name,
			Description: pr.Description,
			Template:    pr.Template,
			Arguments:   args,
		})
	}

	var workflows []workflowSpec
	for _, w := range p.workflows {
		var stages []stageSpec
		for _, s := range w.Stages {
			stages = append(stages, stageSpec{
				Name:   s.Name,
				Tool:   s.Tool,
				Type:   s.Type,
				Prompt: s.Prompt,
			})
		}
		workflows = append(workflows, workflowSpec{
			Name:        w.Name,
			Description: w.Description,
			Stages:      stages,
		})
	}

	var adapter *adapterSpec
	if p.adapter != nil {
		adapter = &adapterSpec{
			Channel:     p.adapter.Channel,
			ParseConfig: p.adapter.ParseConfig,
		}
	}

	return registerRequest{
		APIVersion:  ProtocolVersion,
		Name:        p.name,
		Version:     p.ver,
		Description: p.description,
		Services:    p.services(),
		Tools:       tools,
		Prompts:     prompts,
		Workflows:   workflows,
		Adapter:     adapter,
		Preamble:    p.preamble,
	}
}

func (p *Plugin) register(nc *nats.Conn) error {
	payload, err := json.Marshal(p.buildManifest())
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	msg, err := nc.Request(subjectRegister, payload, 10*time.Second)
	if err != nil {
		return fmt.Errorf("nats request: %w", err)
	}
	var reply registerResponse
	if err := json.Unmarshal(msg.Data, &reply); err != nil {
		return fmt.Errorf("parse reply: %w", err)
	}
	if reply.Status != "ok" {
		return fmt.Errorf("rejected: %s", reply.Error)
	}
	p.logger.Printf("registered (v%s, protocol=%s, tools=%d, reloaded=%v)",
		p.ver, ProtocolVersion, reply.ToolsRegistered, reply.Reloaded)
	return nil
}

func (p *Plugin) subscribe(nc *nats.Conn) error {
	if len(p.tools) > 0 {
		subject := subjectToolCallPfx + p.name + ".*"
		_, err := nc.QueueSubscribe(subject, p.name, func(msg *nats.Msg) {
			go p.dispatchTool(msg)
		})
		if err != nil {
			return fmt.Errorf("tool subscribe: %w", err)
		}
	}

	for _, h := range p.handlers {
		h := h
		var err error
		if h.Queue != "" {
			_, err = nc.QueueSubscribe(h.Subject, h.Queue, func(msg *nats.Msg) {
				h.Handler(msg.Subject, msg.Data)
			})
		} else {
			_, err = nc.Subscribe(h.Subject, func(msg *nats.Msg) {
				h.Handler(msg.Subject, msg.Data)
			})
		}
		if err != nil {
			return fmt.Errorf("handler subscribe %s: %w", h.Subject, err)
		}
	}
	return nil
}

func (p *Plugin) heartbeat(nc *nats.Conn) {
	ticker := time.NewTicker(time.Duration(p.heartbeatSec) * time.Second)
	defer ticker.Stop()
	hb, _ := json.Marshal(healthMessage{
		APIVersion: ProtocolVersion,
		Status:     "ok",
	})
	subject := subjectHealthPfx + p.name
	for range ticker.C {
		nc.Publish(subject, hb)
	}
}

func (p *Plugin) deregister(nc *nats.Conn) {
	data, _ := json.Marshal(deregisterRequest{
		APIVersion: ProtocolVersion,
		Name:       p.name,
	})
	nc.Publish(subjectDeregister, data)
	nc.Flush()
	p.logger.Printf("deregistered, exiting")
}

func (p *Plugin) dispatchTool(msg *nats.Msg) {
	var req toolCallRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		p.respondErr(msg, "invalid request payload")
		return
	}

	prefix := subjectToolCallPfx + p.name + "."
	toolName := msg.Subject[len(prefix):]

	t, ok := p.tools[toolName]
	if !ok {
		p.respondErr(msg, fmt.Sprintf("unknown tool: %s", toolName))
		return
	}

	result, err := t.Handler(req.Params)
	if err != nil {
		p.respondErr(msg, err.Error())
		return
	}

	resp, _ := json.Marshal(toolCallResponse{Result: result})
	msg.Respond(resp)
}

func (p *Plugin) respondErr(msg *nats.Msg, errMsg string) {
	resp, _ := json.Marshal(toolCallResponse{Error: errMsg})
	msg.Respond(resp)
}

func (p *Plugin) logReady() {
	parts := fmt.Sprintf("tools=%d prompts=%d workflows=%d",
		len(p.tools), len(p.prompts), len(p.workflows))
	if p.adapter != nil {
		parts += " adapter=" + p.adapter.Channel
	}
	if len(p.handlers) > 0 {
		parts += fmt.Sprintf(" handlers=%d", len(p.handlers))
	}
	p.logger.Printf("ready (%s)", parts)
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
