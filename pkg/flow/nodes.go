package flow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"text/template"
	"time"
)

// sharedHTTPClient is a package-level client with connection pooling.
// Used by executeHTTPRequest and executeWebhookCallNode instead of creating
// a new client per invocation.
var sharedHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	},
}

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
	r.RegisterFunc("debug", executeDebug)
	r.RegisterFunc("function", executeFunction)
	r.RegisterFunc("value", executeValue)
	r.RegisterFunc("template", executeTemplate)
	r.RegisterFunc("http-request", executeHTTPRequest)
	r.RegisterFunc("transform", executeTransform)
	r.RegisterFunc("set-variable", executeSetVariable)
	r.RegisterFunc("log", executeLog)
	r.RegisterFunc("counter", executeCounter)
}

// --- Flow control executors ---

// inputMsg returns the msg arriving on the "input" port.
func inputMsg(ec *ExecContext) any {
	return ec.Inputs["input"]
}

func executeTrigger(_ context.Context, ec *ExecContext) (any, error) {
	return NewMsg(ec.Run.Inputs), nil
}

func executeApproval(_ context.Context, _ *ExecContext) (any, error) {
	return nil, ErrApprovalRequired
}

func executeCondition(_ context.Context, ec *ExecContext) (any, error) {
	exprStr, ok := ec.Node.Data["expression"].(string)
	if !ok {
		return nil, fmt.Errorf("condition node %s: missing 'expression' in data", ec.Node.ID)
	}

	msg := inputMsg(ec)
	env := buildExprEnv(ec.Inputs, nil)
	result, err := evalExpr(exprStr, env, ec.JSRuntime())
	if err != nil {
		return nil, fmt.Errorf("condition node %s: %w", ec.Node.ID, err)
	}

	if boolVal, ok := result.(bool); ok && boolVal {
		return PortOutput{Port: "true", Value: msg}, nil
	}
	return PortOutput{Port: "false", Value: msg}, nil
}

func executeSwitch(_ context.Context, ec *ExecContext) (any, error) {
	exprStr, ok := ec.Node.Data["expression"].(string)
	if !ok {
		return nil, fmt.Errorf("switch node %s: missing 'expression' in data", ec.Node.ID)
	}

	msg := inputMsg(ec)
	env := buildExprEnv(ec.Inputs, nil)
	result, err := evalExpr(exprStr, env, ec.JSRuntime())
	if err != nil {
		return nil, fmt.Errorf("switch node %s: %w", ec.Node.ID, err)
	}

	value := fmt.Sprintf("%v", result)
	cases, _ := ec.Node.Data["cases"].(map[string]any)
	for caseName := range cases {
		if caseName == value {
			return PortOutput{Port: "case_" + caseName, Value: msg}, nil
		}
	}
	return PortOutput{Port: "default", Value: msg}, nil
}

func executeMerge(_ context.Context, ec *ExecContext) (any, error) {
	// Merge collects payloads from all inputs into a single msg.
	merged := make(map[string]any)
	var baseMsg any
	for port, val := range ec.Inputs {
		merged[port] = MsgPayload(val)
		if baseMsg == nil {
			baseMsg = val
		}
	}
	return MsgSet(baseMsg, merged), nil
}

func executeRace(_ context.Context, ec *ExecContext) (any, error) {
	var winnerPort string
	var winnerMsg any
	for port, val := range ec.Inputs {
		if val != nil {
			winnerPort = port
			winnerMsg = val
			break
		}
	}
	out := MsgSet(winnerMsg, MsgPayload(winnerMsg))
	out["_winner"] = winnerPort
	return RaceOutput{
		Port:   "output",
		Value:  out,
		Winner: winnerPort,
	}, nil
}

func executeForeach(ctx context.Context, ec *ExecContext) (any, error) {
	// Items come from the "items" port. If it's a msg, extract the payload.
	rawItems := ec.Inputs["items"]
	itemsPayload := MsgPayload(rawItems)
	items, ok := itemsPayload.([]any)
	if !ok {
		return nil, fmt.Errorf("foreach node %s: 'items' input must be an array, got %T", ec.Node.ID, itemsPayload)
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
		return PortOutput{Port: "done", Value: NewMsg(items)}, nil
	}

	concurrency := getIntOrDefault(ec.Node.Data, "concurrency", 10)
	values := make([]any, len(items))
	iterResults := make([]*subgraphResult, len(items))
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
			res, err := ec.Engine.executeSubgraph(ctx, ec.Run, ec.Def, subgraphNodeIDs, iterPrefix, val)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("foreach node %s iteration %d: %w", ec.Node.ID, idx, err)
				}
				mu.Unlock()
				return
			}
			mu.Lock()
			values[idx] = res.Value
			iterResults[idx] = res
			mu.Unlock()
		}(i, item)
	}
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	// Merge all iteration node states into the run under the run mutex.
	for _, res := range iterResults {
		if res != nil {
			ec.Run.MergeNodeStates(res.States)
		}
	}

	return PortOutput{Port: "done", Value: NewMsg(values)}, nil
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
		return inputMsg(ec), nil // pass msg through
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func executeOutput(_ context.Context, ec *ExecContext) (any, error) {
	// Extract payload from msg for the flow output.
	msg := inputMsg(ec)
	return PortOutput{Port: "output", Value: MsgPayload(msg)}, nil
}

func executeError(_ context.Context, ec *ExecContext) (any, error) {
	msg := inputMsg(ec)
	return nil, fmt.Errorf("%v", MsgPayload(msg))
}

// --- Data node executors ---

func executeValue(_ context.Context, ec *ExecContext) (any, error) {
	if ec.Node.Data == nil {
		return NewMsg(nil), nil
	}
	return NewMsg(ec.Node.Data["value"]), nil
}

func executeTemplate(_ context.Context, ec *ExecContext) (any, error) {
	msg := inputMsg(ec)
	tmplStr, ok := ec.Node.Data["template"].(string)
	if !ok || tmplStr == "" {
		return msg, nil
	}

	payload := MsgPayload(msg)

	// Build template data: payload fields are top-level, plus explicit aliases.
	data := make(map[string]any)
	if m, ok := payload.(map[string]any); ok {
		for k, v := range m {
			data[k] = v
		}
	}
	data["payload"] = payload
	data["input"] = payload // backward compat
	data["msg"] = msg
	data["topic"] = MsgTopic(msg)
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
	return MsgSet(msg, buf.String()), nil
}

func executeHTTPRequest(ctx context.Context, ec *ExecContext) (any, error) {
	msg := inputMsg(ec)

	// Node-RED pattern: msg.url, msg.method, msg.headers override node settings.
	// msg.payload becomes the request body for POST/PUT.
	url := resolveFromMsg(msg, ec.Node.Data, "url")
	if url == "" {
		return nil, fmt.Errorf("http-request node %s: missing url", ec.Node.ID)
	}

	method := resolveFromMsg(msg, ec.Node.Data, "method")
	if method == "" {
		method = "GET"
	}

	var reqBody io.Reader
	payload := MsgPayload(msg)
	if payload != nil && (method == "POST" || method == "PUT" || method == "PATCH") {
		switch b := payload.(type) {
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

	// Headers: node settings first, then msg.headers override.
	if hdrs, ok := ec.Node.Data["headers"].(map[string]any); ok {
		for k, v := range hdrs {
			req.Header.Set(k, fmt.Sprintf("%v", v))
		}
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
		return nil, fmt.Errorf("http-request node %s: %w", ec.Node.ID, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	// Output msg: payload = response body, plus statusCode and headers.
	var responsePayload any
	if json.Unmarshal(respBody, &responsePayload) != nil {
		responsePayload = string(respBody)
	}

	out := MsgSet(msg, responsePayload)
	out["statusCode"] = resp.StatusCode
	out["headers"] = headerMap(resp.Header)
	out["responseUrl"] = url
	return out, nil
}

func headerMap(h http.Header) map[string]string {
	m := make(map[string]string, len(h))
	for k := range h {
		m[strings.ToLower(k)] = h.Get(k)
	}
	return m
}

func executeTransform(_ context.Context, ec *ExecContext) (any, error) {
	exprStr, ok := ec.Node.Data["expression"].(string)
	if !ok {
		return nil, fmt.Errorf("transform node %s: missing 'expression' in data", ec.Node.ID)
	}

	msg := inputMsg(ec)
	env := buildExprEnv(ec.Inputs, ec.Run.Variables)
	result, err := evalExpr(exprStr, env, ec.JSRuntime())
	if err != nil {
		return nil, fmt.Errorf("transform node %s: %w", ec.Node.ID, err)
	}
	return MsgSet(msg, result), nil
}

func executeSetVariable(_ context.Context, ec *ExecContext) (any, error) {
	varName, _ := ec.Node.Data["variable"].(string)
	if varName == "" {
		return nil, fmt.Errorf("set-variable node %s: missing 'variable' in data", ec.Node.ID)
	}

	msg := inputMsg(ec)
	ec.Run.SetVariable(varName, MsgPayload(msg))
	return msg, nil // pass msg through
}

func executeLog(_ context.Context, ec *ExecContext) (any, error) {
	flowMsg := inputMsg(ec)
	logText, _ := ec.Node.Data["message"].(string)
	if logText == "" {
		logText = fmt.Sprintf("[flow %s] node %s: %v", ec.Run.FlowName, ec.Node.ID, MsgPayload(flowMsg))
	}
	fmt.Println(logText)
	return flowMsg, nil // pass msg through
}

func executeCounter(_ context.Context, ec *ExecContext) (any, error) {
	msg := inputMsg(ec)
	varName := getStringOrDefault(ec.Node.Data, "variable", "counter")
	operation := getStringOrDefault(ec.Node.Data, "operation", "increment")
	step := getIntOrDefault(ec.Node.Data, "step", 1)
	initial := getIntOrDefault(ec.Node.Data, "initial", 0)

	// Read current value from flow-level persistent state (survives across runs).
	current := initial
	if ec.Engine != nil {
		if v, ok := ec.Engine.GetFlowState(ec.Run.FlowID, varName); ok {
			current = toInt(v, initial)
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
		current = toInt(MsgPayload(msg), initial)
	}

	// Write back to persistent flow state.
	if ec.Engine != nil {
		ec.Engine.SetFlowState(ec.Run.FlowID, varName, current)
	}

	// Also store in run variables for visibility in the run result.
	ec.Run.SetVariable(varName, current)

	return MsgSet(msg, map[string]any{"value": current, "variable": varName}), nil
}

func toInt(v any, fallback int) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	case int64:
		return int(n)
	}
	return fallback
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
