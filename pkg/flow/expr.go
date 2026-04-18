package flow

import (
	"fmt"

	"github.com/expr-lang/expr"
)

// evalExpr evaluates an expression string against the given environment.
// Uses expr-lang/expr for sandboxed, type-safe evaluation.
func evalExpr(expression string, env map[string]any) (any, error) {
	if expression == "" {
		return nil, fmt.Errorf("empty expression")
	}
	program, err := expr.Compile(expression, expr.Env(env))
	if err != nil {
		return nil, fmt.Errorf("compile expression %q: %w", expression, err)
	}
	result, err := expr.Run(program, env)
	if err != nil {
		return nil, fmt.Errorf("eval expression %q: %w", expression, err)
	}
	return result, nil
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

// buildExprEnv creates the sandboxed environment for expression evaluation.
// Only input port values and flow variables are accessible.
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

// compileExpr validates that an expression compiles without error.
// Used at flow-save time to catch syntax errors early.
func compileExpr(expression string) error {
	if expression == "" {
		return nil
	}
	// Compile with a permissive environment — we just want syntax checking.
	_, err := expr.Compile(expression, expr.AllowUndefinedVariables())
	if err != nil {
		return fmt.Errorf("expression %q: %w", expression, err)
	}
	return nil
}
