package flow

import (
	"time"

	"github.com/NubeDev/bizzy/pkg/apps"
)

// evalExpr evaluates a JS expression against the given environment using goja.
// The env keys become top-level properties accessible in the expression.
// Supports simple expressions ("input.value > 5") and full functions
// ("function handle(params) { return params.input * 2 }").
//
// If rt is non-nil it is used directly (user-scoped runtime with secrets,
// config, plugins). Otherwise a bare flow runtime is created.
func evalExpr(expression string, env map[string]any, rt *apps.JSRuntime) (any, error) {
	if rt == nil {
		rt = apps.NewFlowRuntime(5 * time.Second)
	}
	return rt.EvalExpression(expression, env)
}

// evalCondition evaluates an expression and returns whether the condition edge should fire.
// The value may be a msg — extracts payload for the "value" variable.
func evalCondition(condition string, value any) bool {
	env := map[string]any{"value": MsgPayload(value)}
	result, err := evalExpr(condition, env, nil)
	if err != nil {
		return false
	}
	b, ok := result.(bool)
	return ok && b
}

// buildExprEnv creates the environment for expression evaluation.
// Follows the Node-RED msg convention:
//
//	msg     — the full msg object (payload, topic, _msgid, custom props)
//	payload — msg.payload (the main data)
//	input   — alias for payload (backward compat: "input.value > 5" still works)
//	topic   — msg.topic
//	vars    — flow-level variables
func buildExprEnv(inputs map[string]any, vars map[string]any) map[string]any {
	env := make(map[string]any)

	if msg, ok := inputs["input"]; ok {
		env["msg"] = msg
		payload := MsgPayload(msg)
		env["payload"] = payload
		env["input"] = payload // backward compat
		env["topic"] = MsgTopic(msg)
	}

	// Multi-input nodes: expose other ports directly.
	for k, v := range inputs {
		if k != "input" {
			env[k] = v
		}
	}

	if vars != nil {
		env["vars"] = vars
	}
	return env
}
