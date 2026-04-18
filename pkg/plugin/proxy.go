package plugin

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/NubeDev/bizzy/pkg/version"
	"github.com/nats-io/nats.go"
)

// Proxy dispatches tool calls to plugins over NATS request/reply.
// It implements the same conceptual interface as JS tool execution
// and OpenAPI proxy, so callers don't know they're talking to a plugin.
type Proxy struct {
	nc      *nats.Conn
	timeout time.Duration
}

// NewProxy creates a tool call proxy.
func NewProxy(nc *nats.Conn, timeout time.Duration) *Proxy {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &Proxy{nc: nc, timeout: timeout}
}

// Call sends a tool request to a plugin and waits for the reply.
func (p *Proxy) Call(pluginName, toolName string, params map[string]any, ctx ToolCallCtx) (any, error) {
	subject := fmt.Sprintf("%s%s.%s", SubjectToolCallPrefix, pluginName, toolName)

	if ctx.TimeoutMS == 0 {
		ctx.TimeoutMS = int(p.timeout.Milliseconds())
	}

	req := ToolCallRequest{
		APIVersion: version.PluginProtocol,
		Params:     params,
		Context:    ctx,
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal tool call: %w", err)
	}

	timeout := p.timeout
	if ctx.TimeoutMS > 0 {
		timeout = time.Duration(ctx.TimeoutMS) * time.Millisecond
	}

	msg, err := p.nc.Request(subject, payload, timeout)
	if err != nil {
		return nil, fmt.Errorf("plugin %s tool %s: %w", pluginName, toolName, err)
	}

	var resp ToolCallResponse
	if err := json.Unmarshal(msg.Data, &resp); err != nil {
		return nil, fmt.Errorf("plugin %s tool %s: unmarshal response: %w", pluginName, toolName, err)
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("plugin %s tool %s: %s", pluginName, toolName, resp.Error)
	}
	return resp.Result, nil
}
