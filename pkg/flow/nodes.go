package flow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"text/template"
	"time"
)

// --- Built-in node executors ---

func (e *Engine) executeCondition(node *FlowNodeDef, inputs map[string]any) (any, error) {
	exprStr, ok := node.Data["expression"].(string)
	if !ok {
		return nil, fmt.Errorf("condition node %s: missing 'expression' in data", node.ID)
	}

	env := buildExprEnv(inputs, nil)
	result, err := evalExpr(exprStr, env)
	if err != nil {
		return nil, fmt.Errorf("condition node %s: %w", node.ID, err)
	}

	if boolVal, ok := result.(bool); ok && boolVal {
		return PortOutput{Port: "true", Value: inputs["input"]}, nil
	}
	return PortOutput{Port: "false", Value: inputs["input"]}, nil
}

func (e *Engine) executeSwitch(node *FlowNodeDef, inputs map[string]any) (any, error) {
	exprStr, ok := node.Data["expression"].(string)
	if !ok {
		return nil, fmt.Errorf("switch node %s: missing 'expression' in data", node.ID)
	}

	env := buildExprEnv(inputs, nil)
	result, err := evalExpr(exprStr, env)
	if err != nil {
		return nil, fmt.Errorf("switch node %s: %w", node.ID, err)
	}

	value := fmt.Sprintf("%v", result)
	cases, _ := node.Data["cases"].(map[string]any)
	for caseName := range cases {
		if caseName == value {
			return PortOutput{Port: "case_" + caseName, Value: inputs["input"]}, nil
		}
	}
	return PortOutput{Port: "default", Value: inputs["input"]}, nil
}

func (e *Engine) executeMerge(inputs map[string]any) (any, error) {
	return inputs, nil
}

func (e *Engine) executeRace(ctx context.Context, run *FlowRun, node *FlowNodeDef, inputs map[string]any) (any, error) {
	var winnerPort string
	var winnerValue any
	for port, val := range inputs {
		if val != nil {
			winnerPort = port
			winnerValue = val
			break
		}
	}

	return RaceOutput{
		Port:   "output",
		Value:  winnerValue,
		Winner: winnerPort,
	}, nil
}

func (e *Engine) executeForeach(ctx context.Context, run *FlowRun, node *FlowNodeDef, inputs map[string]any) (any, error) {
	items, ok := inputs["items"].([]any)
	if !ok {
		return nil, fmt.Errorf("foreach node %s: 'items' input must be an array", node.ID)
	}

	maxIter := getIntOrDefault(node.Data, "max_iterations", 1000)
	if maxIter > 10000 {
		maxIter = 10000
	}
	if len(items) > maxIter {
		return nil, fmt.Errorf("foreach node %s: %d items exceeds max_iterations (%d)", node.ID, len(items), maxIter)
	}

	subgraphRaw, ok := node.Data["_subgraph_nodes"]
	if !ok {
		// No subgraph — just collect items.
		return PortOutput{Port: "done", Value: items}, nil
	}

	// Parse subgraph node IDs.
	var subgraphNodeIDs []string
	switch v := subgraphRaw.(type) {
	case []string:
		subgraphNodeIDs = v
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				subgraphNodeIDs = append(subgraphNodeIDs, s)
			}
		}
	}

	if len(subgraphNodeIDs) == 0 {
		return PortOutput{Port: "done", Value: items}, nil
	}

	concurrency := getIntOrDefault(node.Data, "concurrency", 10)
	results := make([]any, len(items))
	sem := make(chan struct{}, concurrency)
	var mu sync.Mutex
	var firstErr error
	var wg sync.WaitGroup

	for i, item := range items {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, val any) {
			defer wg.Done()
			defer func() { <-sem }()

			iterPrefix := fmt.Sprintf("%s:iter-%d", node.ID, idx)
			result, err := e.executeSubgraph(ctx, run, subgraphNodeIDs, iterPrefix, val)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("foreach node %s iteration %d: %w", node.ID, idx, err)
				}
				mu.Unlock()
				return
			}
			results[idx] = result
		}(i, item)
	}
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}
	return PortOutput{Port: "done", Value: results}, nil
}

func (e *Engine) executeDelay(ctx context.Context, node *FlowNodeDef, inputs map[string]any) (any, error) {
	durStr, _ := node.Data["duration"].(string)
	if durStr == "" {
		durStr = "1s"
	}
	dur, err := time.ParseDuration(durStr)
	if err != nil {
		return nil, fmt.Errorf("delay node %s: invalid duration %q: %w", node.ID, durStr, err)
	}

	select {
	case <-time.After(dur):
		return inputs["input"], nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (e *Engine) executeTransform(node *FlowNodeDef, inputs map[string]any, vars map[string]any) (any, error) {
	exprStr, ok := node.Data["expression"].(string)
	if !ok {
		return nil, fmt.Errorf("transform node %s: missing 'expression' in data", node.ID)
	}

	env := buildExprEnv(inputs, vars)
	result, err := evalExpr(exprStr, env)
	if err != nil {
		return nil, fmt.Errorf("transform node %s: %w", node.ID, err)
	}
	return result, nil
}

func (e *Engine) executeSetVariable(run *FlowRun, node *FlowNodeDef, inputs map[string]any) (any, error) {
	varName, _ := node.Data["variable"].(string)
	if varName == "" {
		return nil, fmt.Errorf("set-variable node %s: missing 'variable' in data", node.ID)
	}

	if run.Variables == nil {
		run.Variables = make(map[string]any)
	}
	run.Variables[varName] = inputs["input"]
	return inputs["input"], nil
}

func (e *Engine) executeLog(run *FlowRun, node *FlowNodeDef, inputs map[string]any) (any, error) {
	msg, _ := node.Data["message"].(string)
	if msg == "" {
		msg = fmt.Sprintf("[flow %s] node %s: %v", run.FlowName, node.ID, inputs["input"])
	}
	fmt.Println(msg)
	return inputs["input"], nil
}

// --- Simple utility nodes (no external deps, great for testing) ---

// executeValue emits a static value configured on the node.
// Config: data.value (any JSON value — string, number, object, array)
func (e *Engine) executeValue(node *FlowNodeDef, inputs map[string]any) (any, error) {
	if node.Data == nil {
		return nil, nil
	}
	return node.Data["value"], nil
}

// executeTemplate formats a Go template string using the node inputs and flow variables.
// Config: data.template (string with {{.input}}, {{.varName}} placeholders)
func (e *Engine) executeTemplate(run *FlowRun, node *FlowNodeDef, inputs map[string]any) (any, error) {
	tmplStr, ok := node.Data["template"].(string)
	if !ok || tmplStr == "" {
		// Passthrough — just return the input.
		return inputs["input"], nil
	}

	// Build template data from inputs + variables.
	data := make(map[string]any)
	for k, v := range inputs {
		data[k] = v
	}
	if run.Variables != nil {
		for k, v := range run.Variables {
			data[k] = v
		}
	}

	var buf strings.Builder
	t, err := template.New("node").Parse(tmplStr)
	if err != nil {
		return nil, fmt.Errorf("template node %s: parse error: %w", node.ID, err)
	}
	if err := t.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("template node %s: execute error: %w", node.ID, err)
	}
	return buf.String(), nil
}

// executeHTTPRequest makes an HTTP GET/POST and returns the response.
// Config: data.url, data.method (default GET), data.headers
// Inputs can override: inputs["url"], inputs["method"], inputs["body"]
func (e *Engine) executeHTTPRequest(ctx context.Context, node *FlowNodeDef, inputs map[string]any) (any, error) {
	url := resolveString(node.Data, inputs, "url")
	if url == "" {
		return nil, fmt.Errorf("http-request node %s: missing url", node.ID)
	}

	method := resolveString(node.Data, inputs, "method")
	if method == "" {
		method = "GET"
	}

	var reqBody io.Reader
	if bodyVal, ok := inputs["body"]; ok && bodyVal != nil {
		switch b := bodyVal.(type) {
		case string:
			reqBody = strings.NewReader(b)
		default:
			bodyBytes, _ := json.Marshal(b)
			reqBody = bytes.NewReader(bodyBytes)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("http-request node %s: %w", node.ID, err)
	}

	// Headers from config.
	if hdrs, ok := node.Data["headers"].(map[string]any); ok {
		for k, v := range hdrs {
			req.Header.Set(k, fmt.Sprintf("%v", v))
		}
	}
	// Headers from input.
	if hdrs, ok := inputs["headers"].(map[string]any); ok {
		for k, v := range hdrs {
			req.Header.Set(k, fmt.Sprintf("%v", v))
		}
	}
	if req.Header.Get("Content-Type") == "" && reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http-request node %s: %w", node.ID, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	result := map[string]any{
		"status":      resp.StatusCode,
		"status_text": resp.Status,
	}
	var jsonBody any
	if json.Unmarshal(respBody, &jsonBody) == nil {
		result["body"] = jsonBody
	} else {
		result["body"] = string(respBody)
	}
	return result, nil
}

// --- Dispatch ---

func (e *Engine) dispatch(ctx context.Context, run *FlowRun, node *FlowNodeDef, inputs map[string]any) (any, error) {
	// Apply per-node timeout.
	if timeout := getIntOrDefault(node.Data, "timeout", 0); timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
	}

	switch {
	case node.Type == "trigger":
		return run.Inputs, nil

	case node.Type == "approval":
		return nil, ErrApprovalRequired

	case node.Type == "condition":
		return e.executeCondition(node, inputs)

	case node.Type == "switch":
		return e.executeSwitch(node, inputs)

	case node.Type == "merge":
		return e.executeMerge(inputs)

	case node.Type == "race":
		return e.executeRace(ctx, run, node, inputs)

	case node.Type == "foreach":
		return e.executeForeach(ctx, run, node, inputs)

	case node.Type == "delay":
		return e.executeDelay(ctx, node, inputs)

	case node.Type == "transform":
		return e.executeTransform(node, inputs, run.Variables)

	case node.Type == "set-variable":
		return e.executeSetVariable(run, node, inputs)

	case node.Type == "log":
		return e.executeLog(run, node, inputs)

	case node.Type == "value":
		return e.executeValue(node, inputs)

	case node.Type == "template":
		return e.executeTemplate(run, node, inputs)

	case node.Type == "http-request":
		return e.executeHTTPRequest(ctx, node, inputs)

	case node.Type == "ai-prompt":
		return e.executePrompt(ctx, run, node, inputs)

	case node.Type == "ai-runner":
		return e.executeAIRunner(ctx, run, node, inputs)

	case strings.HasPrefix(node.Type, "tool:"):
		toolName := strings.TrimPrefix(node.Type, "tool:")
		return e.tools.CallTool(ctx, run.UserID, toolName, inputs)

	case node.Type == "slack-send":
		return e.executeSlackSend(ctx, node, inputs)

	case node.Type == "email-send":
		return e.executeEmailSend(ctx, node, inputs)

	case node.Type == "webhook-call":
		return e.executeWebhookCall(ctx, node, inputs)

	case node.Type == "output":
		return PortOutput{Port: "output", Value: inputs["input"]}, nil

	case node.Type == "error":
		return nil, fmt.Errorf("%v", inputs["input"])

	default:
		return nil, fmt.Errorf("unknown node type: %s", node.Type)
	}
}

// --- Helpers ---

func getIntOrDefault(data map[string]any, key string, def int) int {
	if data == nil {
		return def
	}
	v, ok := data[key]
	if !ok {
		return def
	}
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	case int64:
		return int(n)
	}
	return def
}

func getStringOrDefault(data map[string]any, key, def string) string {
	if data == nil {
		return def
	}
	s, ok := data[key].(string)
	if !ok || s == "" {
		return def
	}
	return s
}

func resolveString(data map[string]any, inputs map[string]any, key string) string {
	// First check inputs (from edge connections), then data (node config).
	if v, ok := inputs[key].(string); ok && v != "" {
		return v
	}
	if data == nil {
		return ""
	}
	s, _ := data[key].(string)
	return s
}

func getErrorStrategy(node *FlowNodeDef) string {
	if node.Data == nil {
		return "stop"
	}
	s, _ := node.Data["on_error"].(string)
	if s == "" {
		return "stop"
	}
	return s
}

func getMaxRetries(node *FlowNodeDef) int {
	return getIntOrDefault(node.Data, "max_retries", 3)
}
