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
	msg := inputMsg(ec)

	// msg.payload or msg.prompt override node settings.
	prompt := resolveFromMsg(msg, ec.Node.Data, "prompt")
	if prompt == "" {
		// If payload is a string, use it as the prompt directly.
		if s, ok := MsgPayload(msg).(string); ok && s != "" {
			prompt = s
		}
	}
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
	return MsgSet(msg, result), nil
}

func executeAIRunnerNode(ctx context.Context, ec *ExecContext) (any, error) {
	msg := inputMsg(ec)
	provider := resolveFromMsg(msg, ec.Node.Data, "provider")
	model := resolveFromMsg(msg, ec.Node.Data, "model")
	prompt := resolveFromMsg(msg, ec.Node.Data, "prompt")
	workDir := resolveFromMsg(msg, ec.Node.Data, "work_dir")

	// If payload is a string and no prompt set, use payload as prompt.
	if prompt == "" {
		if s, ok := MsgPayload(msg).(string); ok && s != "" {
			prompt = s
		}
	}

	if prompt == "" {
		return nil, fmt.Errorf("ai-runner node %s: missing prompt", ec.Node.ID)
	}
	if ec.Services.Agents == nil {
		return nil, fmt.Errorf("ai-runner node %s: no agent runner registry configured", ec.Node.ID)
	}

	// Apply timeout_mins from schema (default 30 mins).
	if timeoutMins := getIntOrDefault(ec.Node.Data, "timeout_mins", 30); timeoutMins > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutMins)*time.Minute)
		defer cancel()
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

	out := MsgSet(msg, map[string]any{
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
	})
	return out, nil
}

func executeSlackSendNode(ctx context.Context, ec *ExecContext) (any, error) {
	msg := inputMsg(ec)

	// msg.channel, msg.payload (as message text) override settings.
	channel := resolveFromMsg(msg, ec.Node.Data, "channel")
	message := resolveFromMsg(msg, ec.Node.Data, "message")
	if message == "" {
		if s, ok := MsgPayload(msg).(string); ok {
			message = s
		}
	}
	if channel == "" || message == "" {
		return nil, fmt.Errorf("slack-send node %s: missing channel or message", ec.Node.ID)
	}
	if ec.Services.Slack == nil {
		return nil, fmt.Errorf("slack-send node %s: slack adapter not configured", ec.Node.ID)
	}
	threadTS := resolveFromMsg(msg, ec.Node.Data, "thread_ts")
	result, err := ec.Services.Slack.SendMessage(ctx, channel, message, threadTS)
	if err != nil {
		return nil, err
	}
	return MsgSet(msg, result), nil
}

func executeEmailSendNode(ctx context.Context, ec *ExecContext) (any, error) {
	msg := inputMsg(ec)
	to := resolveFromMsg(msg, ec.Node.Data, "to")
	subject := resolveFromMsg(msg, ec.Node.Data, "subject")
	body := resolveFromMsg(msg, ec.Node.Data, "body")
	if body == "" {
		if s, ok := MsgPayload(msg).(string); ok {
			body = s
		}
	}
	if to == "" || subject == "" {
		return nil, fmt.Errorf("email-send node %s: missing to or subject", ec.Node.ID)
	}
	if ec.Services.Email == nil {
		return nil, fmt.Errorf("email-send node %s: email adapter not configured", ec.Node.ID)
	}
	if err := ec.Services.Email.SendEmail(ctx, to, subject, body); err != nil {
		return nil, fmt.Errorf("email-send node %s: %w", ec.Node.ID, err)
	}
	return MsgSet(msg, map[string]any{"sent": true, "to": to, "subject": subject}), nil
}

func executeWebhookCallNode(ctx context.Context, ec *ExecContext) (any, error) {
	msg := inputMsg(ec)

	url := resolveFromMsg(msg, ec.Node.Data, "url")
	if url == "" {
		return nil, fmt.Errorf("webhook-call node %s: missing url", ec.Node.ID)
	}
	method := resolveFromMsg(msg, ec.Node.Data, "method")
	if method == "" {
		method = "GET"
	}

	// msg.payload becomes the request body for POST/PUT/PATCH.
	var reqBody io.Reader
	payload := MsgPayload(msg)
	if payload != nil && (method == "POST" || method == "PUT" || method == "PATCH") {
		bodyBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("webhook-call node %s: marshal body: %w", ec.Node.ID, err)
		}
		reqBody = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("webhook-call node %s: create request: %w", ec.Node.ID, err)
	}
	if msgHdrs, ok := MsgGet(msg, "headers"); ok {
		if hdrs, ok := msgHdrs.(map[string]any); ok {
			for k, v := range hdrs {
				req.Header.Set(k, fmt.Sprintf("%v", v))
			}
		}
	}
	if req.Header.Get("Content-Type") == "" && reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := sharedHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("webhook-call node %s: %w", ec.Node.ID, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var responsePayload any
	if json.Unmarshal(respBody, &responsePayload) != nil {
		responsePayload = string(respBody)
	}

	out := MsgSet(msg, responsePayload)
	out["statusCode"] = resp.StatusCode
	out["headers"] = headerMap(resp.Header)
	return out, nil
}

func executeToolCall(ctx context.Context, ec *ExecContext) (any, error) {
	if ec.Services.Tools == nil {
		return nil, fmt.Errorf("tool node %s: no tool caller configured", ec.Node.ID)
	}
	toolName := ec.Node.Type[len("tool:"):]
	msg := inputMsg(ec)

	// Build params: node settings (config panel) are defaults.
	// msg-level properties override settings. msg.payload is the primary
	// data — if it's a map, its keys override too (Node-RED pattern).
	params := make(map[string]any)
	for k, v := range ec.Node.Data {
		if k == "on_error" || k == "max_retries" || k == "timeout" {
			continue
		}
		params[k] = v
	}
	// msg-level overrides (e.g. msg.filter, msg.limit).
	if m, ok := msg.(map[string]any); ok {
		for k, v := range m {
			if k == "payload" || k == "_msgid" || k == "_timestamp" || k == "topic" {
				continue
			}
			params[k] = v
		}
	}
	// msg.payload overrides (highest priority).
	if payload, ok := MsgPayload(msg).(map[string]any); ok {
		for k, v := range payload {
			params[k] = v
		}
	}

	result, err := ec.Services.Tools.CallTool(ctx, ec.Run.UserID, toolName, params)
	if err != nil {
		return nil, err
	}
	return MsgSet(msg, result), nil
}

