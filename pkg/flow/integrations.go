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

// RegisterIntegrationExecutors registers integration node executors.
func RegisterIntegrationExecutors(r *ExecutorRegistry) {
	r.RegisterFunc("ai-prompt", executeAIPrompt)
	r.RegisterFunc("ai-runner", executeAIRunnerNode)
	r.RegisterFunc("slack-send", executeSlackSendNode)
	r.RegisterFunc("email-send", executeEmailSendNode)
	r.RegisterFunc("webhook-call", executeWebhookCallNode)
	r.RegisterPrefix("tool:", NodeExecutorFunc(executeToolCall))
}

func executeAIPrompt(ctx context.Context, ec *ExecContext) (any, error) {
	prompt := resolveString(ec.Node.Data, ec.Inputs, "prompt")
	if prompt == "" {
		return nil, fmt.Errorf("ai-prompt node %s: missing prompt", ec.Node.ID)
	}
	if ec.Services.Prompts == nil {
		return nil, fmt.Errorf("ai-prompt node %s: no prompt runner configured", ec.Node.ID)
	}
	result, err := ec.Services.Prompts.RunPrompt(ctx, ec.Run.UserID, prompt)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func executeAIRunnerNode(ctx context.Context, ec *ExecContext) (any, error) {
	provider := resolveString(ec.Node.Data, ec.Inputs, "provider")
	model := resolveString(ec.Node.Data, ec.Inputs, "model")
	prompt := resolveString(ec.Node.Data, ec.Inputs, "prompt")
	workDir := resolveString(ec.Node.Data, ec.Inputs, "work_dir")

	if prompt == "" {
		return nil, fmt.Errorf("ai-runner node %s: missing prompt", ec.Node.ID)
	}
	if ec.Services.Agents == nil {
		return nil, fmt.Errorf("ai-runner node %s: no agent runner registry configured", ec.Node.ID)
	}

	runner, err := ec.Services.Agents.Get(airunner.Provider(provider))
	if err != nil {
		return nil, fmt.Errorf("ai-runner node %s: provider %q not available: %w", ec.Node.ID, provider, err)
	}

	sessionID := fmt.Sprintf("flow:%s:%s", ec.Run.ID, ec.Node.ID)
	cfg := airunner.RunConfig{
		Prompt:         prompt,
		Model:          model,
		WorkDir:        workDir,
		AllowedTools:   getStringOrDefault(ec.Node.Data, "allowed_tools", "*"),
		ThinkingBudget: getStringOrDefault(ec.Node.Data, "thinking_budget", "medium"),
	}

	if resumeSession, _ := ec.Node.Data["resume_session"].(bool); resumeSession {
		if prev, ok := ec.Run.NodeStates[ec.Node.ID]; ok && prev.Output != nil {
			if prevResult, ok := prev.Output.(map[string]any); ok {
				if sid, ok := prevResult["session_id"].(string); ok {
					cfg.ResumeID = sid
				}
			}
		}
	}

	result := runner.Run(ctx, cfg, sessionID, func(ev airunner.Event) {
		if ec.Services.Bus != nil {
			ec.Services.Bus.Publish(TopicFlowNodeProgress, FlowEvent{
				RunID:    ec.Run.ID,
				FlowID:   ec.Run.FlowID,
				FlowName: ec.Run.FlowName,
				NodeID:   ec.Node.ID,
				NodeType: "ai-runner",
				UserID:   ec.Run.UserID,
				Output:   ev,
			})
		}
	})

	if result.Text == "" && result.DurationMS == 0 {
		return nil, fmt.Errorf("ai-runner node %s: session returned empty result", ec.Node.ID)
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

func executeSlackSendNode(ctx context.Context, ec *ExecContext) (any, error) {
	channel := resolveString(ec.Node.Data, ec.Inputs, "channel")
	message := resolveString(ec.Node.Data, ec.Inputs, "message")
	if channel == "" || message == "" {
		return nil, fmt.Errorf("slack-send node %s: missing channel or message", ec.Node.ID)
	}
	if ec.Services.Slack == nil {
		return nil, fmt.Errorf("slack-send node %s: slack adapter not configured", ec.Node.ID)
	}
	threadTS := resolveString(ec.Node.Data, ec.Inputs, "thread_ts")
	return ec.Services.Slack.SendMessage(ctx, channel, message, threadTS)
}

func executeEmailSendNode(ctx context.Context, ec *ExecContext) (any, error) {
	to := resolveString(ec.Node.Data, ec.Inputs, "to")
	subject := resolveString(ec.Node.Data, ec.Inputs, "subject")
	body := resolveString(ec.Node.Data, ec.Inputs, "body")
	if to == "" || subject == "" {
		return nil, fmt.Errorf("email-send node %s: missing to or subject", ec.Node.ID)
	}
	if ec.Services.Email == nil {
		return nil, fmt.Errorf("email-send node %s: email adapter not configured", ec.Node.ID)
	}
	if err := ec.Services.Email.SendEmail(ctx, to, subject, body); err != nil {
		return nil, fmt.Errorf("email-send node %s: %w", ec.Node.ID, err)
	}
	return map[string]any{"sent": true, "to": to, "subject": subject}, nil
}

func executeWebhookCallNode(ctx context.Context, ec *ExecContext) (any, error) {
	url := resolveString(ec.Node.Data, ec.Inputs, "url")
	if url == "" {
		return nil, fmt.Errorf("webhook-call node %s: missing url", ec.Node.ID)
	}
	method := resolveString(ec.Node.Data, ec.Inputs, "method")
	if method == "" {
		method = "GET"
	}

	var reqBody io.Reader
	if bodyVal, ok := ec.Inputs["body"]; ok && bodyVal != nil {
		bodyBytes, err := json.Marshal(bodyVal)
		if err != nil {
			return nil, fmt.Errorf("webhook-call node %s: marshal body: %w", ec.Node.ID, err)
		}
		reqBody = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("webhook-call node %s: create request: %w", ec.Node.ID, err)
	}
	if headers, ok := ec.Inputs["headers"].(map[string]any); ok {
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
		return nil, fmt.Errorf("webhook-call node %s: %w", ec.Node.ID, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	result := map[string]any{"status": resp.StatusCode, "status_text": resp.Status}
	var jsonBody any
	if json.Unmarshal(respBody, &jsonBody) == nil {
		result["body"] = jsonBody
	} else {
		result["body"] = string(respBody)
	}
	return result, nil
}

func executeToolCall(ctx context.Context, ec *ExecContext) (any, error) {
	if ec.Services.Tools == nil {
		return nil, fmt.Errorf("tool node %s: no tool caller configured", ec.Node.ID)
	}
	toolName := ec.Node.Type[len("tool:"):]
	return ec.Services.Tools.CallTool(ctx, ec.Run.UserID, toolName, ec.Inputs)
}

