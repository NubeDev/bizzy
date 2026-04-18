package flow

import (
	"time"

	"github.com/NubeDev/bizzy/pkg/apps"
)

// evalExpr evaluates a JS expression against the given environment using goja.
// The env keys become top-level properties accessible in the expression.
// Supports simple expressions ("input.value > 5") and full functions
// ("function handle(params) { return params.input * 2 }").
func evalExpr(expression string, env map[string]any) (any, error) {
	rt := apps.NewFlowRuntime(5 * time.Second)
	return rt.EvalExpression(expression, env)
}

// evalCondition evaluates an expression and returns whether the condition edge should fire.
func evalCondition(condition string, value any) bool {
	env := map[string]any{"value": value}
	result, err := evalExpr(condition, env)
	if err != nil {
		return false
	}
	b, ok := result.(bool)
	return ok && b
}

// buildExprEnv creates the environment for expression evaluation.
// Input port values are top-level keys, flow variables are under "vars".
func buildExprEnv(inputs map[string]any, vars map[string]any) map[string]any {
	env := make(map[string]any, len(inputs)+1)
	for k, v := range inputs {
		env[k] = v
	}
	if vars != nil {
		env["vars"] = vars
	}
	return env
}
