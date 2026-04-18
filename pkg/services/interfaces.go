package services

import "context"

// ToolCaller executes a named tool and returns the result.
// This is the canonical interface — used by flow engine, workflow runner,
// and command bus. ToolService implements it directly.
type ToolCaller interface {
	CallTool(ctx context.Context, userID, toolName string, params map[string]any) (any, error)
}

// PromptRunner sends a prompt to an AI provider and returns the text response.
// This is the canonical interface — used by flow engine and workflow runner.
type PromptRunner interface {
	RunPrompt(ctx context.Context, userID, prompt string) (string, error)
}
