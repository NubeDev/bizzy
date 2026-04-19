package flow

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/NubeDev/bizzy/pkg/apps"
)

// executeFunction implements the Node-RED function node pattern.
// The user writes JavaScript that receives `msg` and returns the modified msg.
//
// Available in the JS scope:
//
//	msg           — the incoming message (read/write payload, topic, custom props)
//	node.log(s)   — log a message
//	node.warn(s)  — log a warning
//	node.error(s) — log an error
//	flow.get(key) — read flow-level persistent state
//	flow.set(k,v) — write flow-level persistent state
//	tools.call(name, params) — call an app tool (sync, returns result)
//
// Plus all platform APIs from JSRuntime: http, base64, crypto, env, secrets, etc.
//
// Example user code:
//
//	msg.payload = msg.payload * 2;
//	return msg;
//
//	// Or call a tool:
//	var nodes = tools.call("rubix.query_nodes", {filter: "type=sensor"});
//	msg.payload = nodes;
//	return msg;
//
//	// Or drop the message (returns null):
//	if (msg.payload.skip) return null;
//	return msg;
func executeFunction(ctx context.Context, ec *ExecContext) (any, error) {
	code, ok := ec.Node.Data["code"].(string)
	if !ok || code == "" {
		return inputMsg(ec), nil // no code = passthrough
	}

	msg := inputMsg(ec)

	// Get a user-scoped JS runtime (with secrets, plugins, etc) or a bare one.
	rt := ec.JSRuntime()
	if rt == nil {
		rt = apps.NewFlowRuntime(10 * time.Second)
	}

	// Build the context objects that get passed into the JS function.
	params := map[string]any{
		"msg":  msg,
		"node": buildNodeAPI(ec),
		"flow": buildFlowAPI(ec),
	}
	if ec.Services != nil && ec.Services.Tools != nil {
		params["tools"] = buildToolsAPI(ctx, ec)
	}

	// Wrap the user's code in a handle(params) function so EvalExpression
	// can call it. Destructure params into the variables the user expects.
	wrapped := fmt.Sprintf(`function handle(params) {
var msg = params.msg;
var node = params.node;
var flow = params.flow;
var tools = params.tools || {};
%s
}`, code)

	result, err := rt.EvalExpression(wrapped, params)
	if err != nil {
		return nil, fmt.Errorf("function node %s: %w", ec.Node.ID, err)
	}

	// null/undefined return = drop the message (node output is nil).
	if result == nil {
		return nil, nil
	}

	// If the user returned a non-msg value, wrap it.
	if !IsMsg(result) {
		return MsgSet(msg, result), nil
	}
	return result, nil
}

// buildNodeAPI creates the node.log/warn/error helpers (like Node-RED).
func buildNodeAPI(ec *ExecContext) map[string]any {
	prefix := fmt.Sprintf("[flow %s] node %s", ec.Run.FlowName, ec.Node.ID)
	return map[string]any{
		"id":   ec.Node.ID,
		"name": ec.Node.Label,
		"log": func(args ...any) {
			log.Printf("%s: %v", prefix, fmt.Sprint(args...))
		},
		"warn": func(args ...any) {
			log.Printf("%s [WARN]: %v", prefix, fmt.Sprint(args...))
		},
		"error": func(args ...any) {
			log.Printf("%s [ERROR]: %v", prefix, fmt.Sprint(args...))
		},
	}
}

// buildFlowAPI creates the flow.get/set persistent state helpers.
func buildFlowAPI(ec *ExecContext) map[string]any {
	return map[string]any{
		"get": func(key string) any {
			if ec.Engine == nil {
				return nil
			}
			v, _ := ec.Engine.GetFlowState(ec.Run.FlowID, key)
			return v
		},
		"set": func(key string, value any) {
			if ec.Engine == nil {
				return
			}
			ec.Engine.SetFlowState(ec.Run.FlowID, key, value)
		},
	}
}

// buildToolsAPI creates the tools.call helper for calling app tools from JS.
func buildToolsAPI(ctx context.Context, ec *ExecContext) map[string]any {
	return map[string]any{
		"call": func(toolName string, params map[string]any) any {
			if ec.Services.Tools == nil {
				return map[string]any{"error": "no tool service configured"}
			}
			result, err := ec.Services.Tools.CallTool(ctx, ec.Run.UserID, toolName, params)
			if err != nil {
				return map[string]any{"error": err.Error()}
			}
			return result
		},
	}
}
