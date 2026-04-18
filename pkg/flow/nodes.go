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

// RegisterBuiltinExecutors registers all built-in node executors.
func RegisterBuiltinExecutors(r *ExecutorRegistry) {
	// Flow control.
	r.RegisterFunc("trigger", executeTrigger)
	r.RegisterFunc("approval", executeApproval)
	r.RegisterFunc("condition", executeCondition)
	r.RegisterFunc("switch", executeSwitch)
	r.RegisterFunc("merge", executeMerge)
	r.RegisterFunc("race", executeRace)
	r.RegisterFunc("foreach", executeForeach)
	r.RegisterFunc("delay", executeDelay)
	r.RegisterFunc("output", executeOutput)
	r.RegisterFunc("error", executeError)

	// Data nodes.
	r.RegisterFunc("value", executeValue)
	r.RegisterFunc("template", executeTemplate)
	r.RegisterFunc("http-request", executeHTTPRequest)
	r.RegisterFunc("transform", executeTransform)
	r.RegisterFunc("set-variable", executeSetVariable)
	r.RegisterFunc("log", executeLog)
	r.RegisterFunc("counter", executeCounter)
}

// --- Flow control executors ---

func executeTrigger(_ context.Context, ec *ExecContext) (any, error) {
	return ec.Run.Inputs, nil
}

func executeApproval(_ context.Context, _ *ExecContext) (any, error) {
	return nil, ErrApprovalRequired
}

func executeCondition(_ context.Context, ec *ExecContext) (any, error) {
	exprStr, ok := ec.Node.Data["expression"].(string)
	if !ok {
		return nil, fmt.Errorf("condition node %s: missing 'expression' in data", ec.Node.ID)
	}

	env := buildExprEnv(ec.Inputs, nil)
	result, err := evalExpr(exprStr, env)
	if err != nil {
		return nil, fmt.Errorf("condition node %s: %w", ec.Node.ID, err)
	}

	if boolVal, ok := result.(bool); ok && boolVal {
		return PortOutput{Port: "true", Value: ec.Inputs["input"]}, nil
	}
	return PortOutput{Port: "false", Value: ec.Inputs["input"]}, nil
}

func executeSwitch(_ context.Context, ec *ExecContext) (any, error) {
	exprStr, ok := ec.Node.Data["expression"].(string)
	if !ok {
		return nil, fmt.Errorf("switch node %s: missing 'expression' in data", ec.Node.ID)
	}

	env := buildExprEnv(ec.Inputs, nil)
	result, err := evalExpr(exprStr, env)
	if err != nil {
		return nil, fmt.Errorf("switch node %s: %w", ec.Node.ID, err)
	}

	value := fmt.Sprintf("%v", result)
	cases, _ := ec.Node.Data["cases"].(map[string]any)
	for caseName := range cases {
		if caseName == value {
			return PortOutput{Port: "case_" + caseName, Value: ec.Inputs["input"]}, nil
		}
	}
	return PortOutput{Port: "default", Value: ec.Inputs["input"]}, nil
}

func executeMerge(_ context.Context, ec *ExecContext) (any, error) {
	return ec.Inputs, nil
}

func executeRace(_ context.Context, ec *ExecContext) (any, error) {
	var winnerPort string
	var winnerValue any
	for port, val := range ec.Inputs {
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

func executeForeach(ctx context.Context, ec *ExecContext) (any, error) {
	items, ok := ec.Inputs["items"].([]any)
	if !ok {
		return nil, fmt.Errorf("foreach node %s: 'items' input must be an array", ec.Node.ID)
	}

	maxIter := getIntOrDefault(ec.Node.Data, "max_iterations", 1000)
	if maxIter > 10000 {
		maxIter = 10000
	}
	if len(items) > maxIter {
		return nil, fmt.Errorf("foreach node %s: %d items exceeds max_iterations (%d)", ec.Node.ID, len(items), maxIter)
	}

	subgraphRaw, ok := ec.Node.Data["_subgraph_nodes"]
	if !ok {
		return PortOutput{Port: "done", Value: items}, nil
	}

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

	concurrency := getIntOrDefault(ec.Node.Data, "concurrency", 10)
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

			iterPrefix := fmt.Sprintf("%s:iter-%d", ec.Node.ID, idx)
			result, err := ec.Engine.executeSubgraph(ctx, ec.Run, ec.Def, subgraphNodeIDs, iterPrefix, val)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("foreach node %s iteration %d: %w", ec.Node.ID, idx, err)
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

func executeDelay(ctx context.Context, ec *ExecContext) (any, error) {
	durStr, _ := ec.Node.Data["duration"].(string)
	if durStr == "" {
		durStr = "1s"
	}
	dur, err := time.ParseDuration(durStr)
	if err != nil {
		return nil, fmt.Errorf("delay node %s: invalid duration %q: %w", ec.Node.ID, durStr, err)
	}

	select {
	case <-time.After(dur):
		return ec.Inputs["input"], nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func executeOutput(_ context.Context, ec *ExecContext) (any, error) {
	return PortOutput{Port: "output", Value: ec.Inputs["input"]}, nil
}

func executeError(_ context.Context, ec *ExecContext) (any, error) {
	return nil, fmt.Errorf("%v", ec.Inputs["input"])
}

// --- Data node executors ---

func executeValue(_ context.Context, ec *ExecContext) (any, error) {
	if ec.Node.Data == nil {
		return nil, nil
	}
	return ec.Node.Data["value"], nil
}

func executeTemplate(_ context.Context, ec *ExecContext) (any, error) {
	tmplStr, ok := ec.Node.Data["template"].(string)
	if !ok || tmplStr == "" {
		return ec.Inputs["input"], nil
	}

	data := make(map[string]any)
	for k, v := range ec.Inputs {
		data[k] = v
	}
	if ec.Run.Variables != nil {
		for k, v := range ec.Run.Variables {
			data[k] = v
		}
	}

	var buf strings.Builder
	t, err := template.New("node").Parse(tmplStr)
	if err != nil {
		return nil, fmt.Errorf("template node %s: parse error: %w", ec.Node.ID, err)
	}
	if err := t.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("template node %s: execute error: %w", ec.Node.ID, err)
	}
	return buf.String(), nil
}

func executeHTTPRequest(ctx context.Context, ec *ExecContext) (any, error) {
	url := resolveString(ec.Node.Data, ec.Inputs, "url")
	if url == "" {
		return nil, fmt.Errorf("http-request node %s: missing url", ec.Node.ID)
	}

	method := resolveString(ec.Node.Data, ec.Inputs, "method")
	if method == "" {
		method = "GET"
	}

	var reqBody io.Reader
	if bodyVal, ok := ec.Inputs["body"]; ok && bodyVal != nil {
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
		return nil, fmt.Errorf("http-request node %s: %w", ec.Node.ID, err)
	}

	if hdrs, ok := ec.Node.Data["headers"].(map[string]any); ok {
		for k, v := range hdrs {
			req.Header.Set(k, fmt.Sprintf("%v", v))
		}
	}
	if hdrs, ok := ec.Inputs["headers"].(map[string]any); ok {
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
		return nil, fmt.Errorf("http-request node %s: %w", ec.Node.ID, err)
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

func executeTransform(_ context.Context, ec *ExecContext) (any, error) {
	exprStr, ok := ec.Node.Data["expression"].(string)
	if !ok {
		return nil, fmt.Errorf("transform node %s: missing 'expression' in data", ec.Node.ID)
	}

	env := buildExprEnv(ec.Inputs, ec.Run.Variables)
	result, err := evalExpr(exprStr, env)
	if err != nil {
		return nil, fmt.Errorf("transform node %s: %w", ec.Node.ID, err)
	}
	return result, nil
}

func executeSetVariable(_ context.Context, ec *ExecContext) (any, error) {
	varName, _ := ec.Node.Data["variable"].(string)
	if varName == "" {
		return nil, fmt.Errorf("set-variable node %s: missing 'variable' in data", ec.Node.ID)
	}

	if ec.Run.Variables == nil {
		ec.Run.Variables = make(map[string]any)
	}
	ec.Run.Variables[varName] = ec.Inputs["input"]
	return ec.Inputs["input"], nil
}

func executeLog(_ context.Context, ec *ExecContext) (any, error) {
	msg, _ := ec.Node.Data["message"].(string)
	if msg == "" {
		msg = fmt.Sprintf("[flow %s] node %s: %v", ec.Run.FlowName, ec.Node.ID, ec.Inputs["input"])
	}
	fmt.Println(msg)
	return ec.Inputs["input"], nil
}

func executeCounter(_ context.Context, ec *ExecContext) (any, error) {
	varName := getStringOrDefault(ec.Node.Data, "variable", "counter")
	operation := getStringOrDefault(ec.Node.Data, "operation", "increment")
	step := getIntOrDefault(ec.Node.Data, "step", 1)
	initial := getIntOrDefault(ec.Node.Data, "initial", 0)

	if ec.Run.Variables == nil {
		ec.Run.Variables = make(map[string]any)
	}

	// Read current value from flow variables, or use initial.
	current := initial
	if v, ok := ec.Run.Variables[varName]; ok {
		switch n := v.(type) {
		case int:
			current = n
		case float64:
			current = int(n)
		case int64:
			current = int(n)
		}
	}

	switch operation {
	case "increment":
		current += step
	case "decrement":
		current -= step
	case "reset":
		current = initial
	case "set":
		if v, ok := ec.Inputs["input"]; ok {
			switch n := v.(type) {
			case int:
				current = n
			case float64:
				current = int(n)
			case int64:
				current = int(n)
			}
		}
	}

	ec.Run.Variables[varName] = current
	return map[string]any{"value": current, "variable": varName}, nil
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
