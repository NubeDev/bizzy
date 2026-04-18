package flow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/NubeDev/bizzy/pkg/airunner"
)

// executePrompt runs a single-turn AI prompt.
func (e *Engine) executePrompt(ctx context.Context, run *FlowRun, node *FlowNodeDef, inputs map[string]any) (any, error) {
	prompt := resolveString(node.Data, inputs, "prompt")
	if prompt == "" {
		return nil, fmt.Errorf("ai-prompt node %s: missing prompt", node.ID)
	}

	if e.prompts == nil {
		return nil, fmt.Errorf("ai-prompt node %s: no prompt runner configured", node.ID)
	}

	result, err := e.prompts.RunPrompt(ctx, run.UserID, prompt)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// executeAIRunner runs a full AI coding session via the airunner infrastructure.
func (e *Engine) executeAIRunner(ctx context.Context, run *FlowRun, node *FlowNodeDef, inputs map[string]any) (any, error) {
	provider := resolveString(node.Data, inputs, "provider")
	model := resolveString(node.Data, inputs, "model")
	prompt := resolveString(node.Data, inputs, "prompt")
	workDir := resolveString(node.Data, inputs, "work_dir")

	if prompt == "" {
		return nil, fmt.Errorf("ai-runner node %s: missing prompt", node.ID)
	}
	if e.agents == nil {
		return nil, fmt.Errorf("ai-runner node %s: no agent runner registry configured", node.ID)
	}

	runner, err := e.agents.Get(airunner.Provider(provider))
	if err != nil {
		return nil, fmt.Errorf("ai-runner node %s: provider %q not available: %w", node.ID, provider, err)
	}

	sessionID := fmt.Sprintf("flow:%s:%s", run.ID, node.ID)

	cfg := airunner.RunConfig{
		Prompt:         prompt,
		Model:          model,
		WorkDir:        workDir,
		AllowedTools:   getStringOrDefault(node.Data, "allowed_tools", "*"),
		ThinkingBudget: getStringOrDefault(node.Data, "thinking_budget", "medium"),
	}

	// Resume previous session if configured.
	if resumeSession, _ := node.Data["resume_session"].(bool); resumeSession {
		if prev, ok := run.NodeStates[node.ID]; ok && prev.Output != nil {
			if prevResult, ok := prev.Output.(map[string]any); ok {
				if sid, ok := prevResult["session_id"].(string); ok {
					cfg.ResumeID = sid
				}
			}
		}
	}

	result := runner.Run(ctx, cfg, sessionID, func(ev airunner.Event) {
		e.events.publish(TopicFlowNodeProgress, FlowEvent{
			RunID:    run.ID,
			FlowID:   run.FlowID,
			FlowName: run.FlowName,
			NodeID:   node.ID,
			NodeType: "ai-runner",
			UserID:   run.UserID,
			Output:   ev,
		})
	})

	if result.Text == "" && result.DurationMS == 0 {
		return nil, fmt.Errorf("ai-runner node %s: session returned empty result", node.ID)
	}

	return map[string]any{
		"text":          result.Text,
		"provider":      result.Provider,
		"model":         result.Model,
		"session_id":    result.ClaudeSessionID,
		"cost_usd":      result.CostUSD,
		"duration_ms":   result.DurationMS,
		"input_tokens":  result.InputTokens,
		"output_tokens": result.OutputTokens,
		"tool_calls":    result.ToolCalls,
		"tool_call_log": result.ToolCallLog,
	}, nil
}

// executeSlackSend sends a message via the Slack adapter.
func (e *Engine) executeSlackSend(ctx context.Context, node *FlowNodeDef, inputs map[string]any) (any, error) {
	channel := resolveString(node.Data, inputs, "channel")
	message := resolveString(node.Data, inputs, "message")
	if channel == "" || message == "" {
		return nil, fmt.Errorf("slack-send node %s: missing channel or message", node.ID)
	}

	if e.slack == nil {
		return nil, fmt.Errorf("slack-send node %s: slack adapter not configured", node.ID)
	}

	threadTS := resolveString(node.Data, inputs, "thread_ts")
	result, err := e.slack.SendMessage(ctx, channel, message, threadTS)
	if err != nil {
		return nil, fmt.Errorf("slack-send node %s: %w", node.ID, err)
	}
	return result, nil
}

// executeEmailSend sends an email via SMTP.
func (e *Engine) executeEmailSend(ctx context.Context, node *FlowNodeDef, inputs map[string]any) (any, error) {
	to := resolveString(node.Data, inputs, "to")
	subject := resolveString(node.Data, inputs, "subject")
	body := resolveString(node.Data, inputs, "body")
	if to == "" || subject == "" {
		return nil, fmt.Errorf("email-send node %s: missing to or subject", node.ID)
	}

	if e.email == nil {
		return nil, fmt.Errorf("email-send node %s: email adapter not configured", node.ID)
	}

	err := e.email.SendEmail(ctx, to, subject, body)
	if err != nil {
		return nil, fmt.Errorf("email-send node %s: %w", node.ID, err)
	}
	return map[string]any{"sent": true, "to": to, "subject": subject}, nil
}

// executeWebhookCall makes an HTTP request.
func (e *Engine) executeWebhookCall(ctx context.Context, node *FlowNodeDef, inputs map[string]any) (any, error) {
	url := resolveString(node.Data, inputs, "url")
	if url == "" {
		return nil, fmt.Errorf("webhook-call node %s: missing url", node.ID)
	}

	method := resolveString(node.Data, inputs, "method")
	if method == "" {
		method = "GET"
	}

	var reqBody io.Reader
	if bodyVal, ok := inputs["body"]; ok && bodyVal != nil {
		bodyBytes, err := json.Marshal(bodyVal)
		if err != nil {
			return nil, fmt.Errorf("webhook-call node %s: marshal body: %w", node.ID, err)
		}
		reqBody = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("webhook-call node %s: create request: %w", node.ID, err)
	}

	// Set headers.
	if headers, ok := inputs["headers"].(map[string]any); ok {
		for k, v := range headers {
			req.Header.Set(k, fmt.Sprintf("%v", v))
		}
	}
	if req.Header.Get("Content-Type") == "" && reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("webhook-call node %s: %w", node.ID, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("webhook-call node %s: read response: %w", node.ID, err)
	}

	result := map[string]any{
		"status":      resp.StatusCode,
		"status_text": resp.Status,
	}

	// Try to parse JSON response.
	var jsonBody any
	if err := json.Unmarshal(respBody, &jsonBody); err == nil {
		result["body"] = jsonBody
	} else {
		result["body"] = string(respBody)
	}

	return result, nil
}

// --- Integration adapter interfaces ---

// SlackSender is the interface for sending Slack messages from flow nodes.
type SlackSender interface {
	SendMessage(ctx context.Context, channel, message, threadTS string) (any, error)
}

// EmailSender is the interface for sending emails from flow nodes.
type EmailSender interface {
	SendEmail(ctx context.Context, to, subject, body string) error
}
