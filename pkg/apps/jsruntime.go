package apps

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dop251/goja"
)

// HTTPLogEntry records a single outbound HTTP request made during tool execution.
type HTTPLogEntry struct {
	Method         string  `json:"method"`
	URL            string  `json:"url"`
	Status         int     `json:"status"`
	DurationMS     float64 `json:"duration_ms"`
	RedirectedFrom *string `json:"redirected_from"`
}

// LoggingRoundTripper wraps an http.RoundTripper and records every request.
type LoggingRoundTripper struct {
	inner   http.RoundTripper
	mu      sync.Mutex
	entries []HTTPLogEntry
}

// NewLoggingRoundTripper creates a round-tripper that logs all requests.
func NewLoggingRoundTripper(inner http.RoundTripper) *LoggingRoundTripper {
	if inner == nil {
		inner = http.DefaultTransport
	}
	return &LoggingRoundTripper{inner: inner}
}

func (t *LoggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := t.inner.RoundTrip(req)
	dur := time.Since(start).Seconds() * 1000

	entry := HTTPLogEntry{
		Method: req.Method,
		URL:    req.URL.String(),
	}
	if resp != nil {
		entry.Status = resp.StatusCode
	}
	entry.DurationMS = dur

	// If this request came from a redirect, record the referrer.
	if ref := req.Header.Get("Referer"); ref != "" {
		entry.RedirectedFrom = &ref
	}

	t.mu.Lock()
	// Detect redirect chains: if the previous entry was a 3xx to this URL, link them.
	if len(t.entries) > 0 {
		prev := &t.entries[len(t.entries)-1]
		if prev.Status >= 300 && prev.Status < 400 && entry.RedirectedFrom == nil {
			prevURL := prev.URL
			entry.RedirectedFrom = &prevURL
		}
	}
	t.entries = append(t.entries, entry)
	t.mu.Unlock()

	return resp, err
}

// Entries returns the captured log entries.
func (t *LoggingRoundTripper) Entries() []HTTPLogEntry {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]HTTPLogEntry, len(t.entries))
	copy(out, t.entries)
	return out
}

// ToolCaller lets a JS tool call other tools in the same app.
// Implemented by a closure in mcp_jstools.go / tools.go that scopes
// calls to the current app only.
type ToolCaller interface {
	CallTool(toolName string, params map[string]any) (map[string]any, error)
}

// JSRuntime executes JavaScript tools in a sandboxed Goja VM.
type JSRuntime struct {
	allowedHosts []string
	secrets      map[string]string
	config       map[string]string
	appDir       string
	appName      string
	timeout      time.Duration
	transport    http.RoundTripper    // optional custom transport for HTTP tracing
	pluginQuery  PluginQuerySource    // optional plugin discovery API for JS tools
	toolCaller   ToolCaller           // optional — call other tools in the same app
	envAllowlist []string             // env var prefixes allowed to read
}

// DefaultEnvAllowlist is the set of environment variable prefixes that JS tools can read.
var DefaultEnvAllowlist = []string{"BIZZY_", "NUBE_", "OLLAMA_", "GITHUB_", "OPENAI_API_", "ANTHROPIC_API_"}

// NewJSRuntime creates a runtime for executing JS tools within an app.
func NewJSRuntime(app *App, secrets, config map[string]string, timeout time.Duration) *JSRuntime {
	return &JSRuntime{
		allowedHosts: app.Permissions.AllowedHosts,
		secrets:      secrets,
		config:       config,
		appDir:       app.Dir,
		appName:      app.Name,
		timeout:      timeout,
		envAllowlist: DefaultEnvAllowlist,
	}
}

// NewTestJSRuntime creates a standalone runtime for the test-tool endpoint.
// It does not require an App on disk — everything is provided inline.
func NewTestJSRuntime(allowedHosts []string, secrets, settings map[string]string, timeout time.Duration, transport http.RoundTripper) *JSRuntime {
	return &JSRuntime{
		allowedHosts: allowedHosts,
		secrets:      secrets,
		config:       settings,
		timeout:      timeout,
		transport:    transport,
	}
}

// SetPluginQuery wires the plugin discovery API into this runtime.
// When set, JS tools can use plugins.exists(), plugins.info(), plugins.list(),
// and plugins.call() to discover and invoke plugin tools.
func (r *JSRuntime) SetPluginQuery(pq PluginQuerySource) {
	r.pluginQuery = pq
}

// SetToolCaller wires same-app tool calling into this runtime.
// When set, JS tools can use tools.call("other_tool", {params}) to call
// other tools in the same app.
func (r *JSRuntime) SetToolCaller(tc ToolCaller) {
	r.toolCaller = tc
}

// ExecuteScript runs inline JS source (with optional helpers) and returns the result.
func (r *JSRuntime) ExecuteScript(script, helpers string, params map[string]any) (map[string]any, error) {
	vm := goja.New()

	r.injectHTTPAPI(vm)
	r.injectSecretsAPI(vm)
	r.injectConfigAPI(vm)
	r.injectLogAPI(vm)
	r.injectPluginsAPI(vm)
	r.injectToolsAPI(vm)
	r.injectBase64API(vm)
	r.injectURLAPI(vm)
	r.injectCryptoAPI(vm)
	r.injectEnvAPI(vm)
	// files API omitted — test-tool has no app directory

	// Load helpers first.
	if helpers != "" {
		if _, err := vm.RunString(helpers); err != nil {
			return nil, fmt.Errorf("helpers error: %w", err)
		}
	}

	if _, err := vm.RunString(script); err != nil {
		return nil, fmt.Errorf("script error: %w", err)
	}

	handleFn, ok := goja.AssertFunction(vm.Get("handle"))
	if !ok {
		return nil, fmt.Errorf("script must define a handle(params) function")
	}

	type result struct {
		val goja.Value
		err error
	}
	done := make(chan result, 1)

	timer := time.AfterFunc(r.timeout, func() {
		vm.Interrupt("timeout: exceeded " + r.timeout.String())
	})

	go func() {
		val, err := handleFn(goja.Undefined(), vm.ToValue(params))
		done <- result{val, err}
	}()

	res := <-done
	timer.Stop()

	if res.err != nil {
		return nil, fmt.Errorf("handle() error: %w", res.err)
	}

	exported := res.val.Export()
	switch v := exported.(type) {
	case map[string]any:
		return v, nil
	default:
		return map[string]any{"result": exported}, nil
	}
}

// Execute runs a JS tool script with the given params and returns the JSON result.
// If a _helpers.js file exists in the same tools/ directory, it is loaded first
// so all tools in the app share common functions (login, resolveNodeId, etc.).
func (r *JSRuntime) Execute(scriptPath string, params map[string]any) (map[string]any, error) {
	script, err := os.ReadFile(scriptPath)
	if err != nil {
		return nil, fmt.Errorf("read script: %w", err)
	}

	vm := goja.New()

	// Inject host APIs.
	r.injectHTTPAPI(vm)
	r.injectSecretsAPI(vm)
	r.injectConfigAPI(vm)
	r.injectLogAPI(vm)
	r.injectFilesAPI(vm)
	r.injectPluginsAPI(vm)
	r.injectToolsAPI(vm)
	r.injectBase64API(vm)
	r.injectURLAPI(vm)
	r.injectCryptoAPI(vm)
	r.injectEnvAPI(vm)

	// Load shared helpers if _helpers.js exists in the same directory.
	helpersPath := filepath.Join(filepath.Dir(scriptPath), "_helpers.js")
	if helpers, err := os.ReadFile(helpersPath); err == nil {
		if _, err := vm.RunString(string(helpers)); err != nil {
			return nil, fmt.Errorf("_helpers.js error: %w", err)
		}
	}

	toolName := filepath.Base(scriptPath)
	log.Printf("[jsruntime] executing %s", toolName)

	// Run the script to define the handle() function.
	if _, err := vm.RunString(string(script)); err != nil {
		return nil, fmt.Errorf("script error: %w", err)
	}

	// Call handle(params).
	handleFn, ok := goja.AssertFunction(vm.Get("handle"))
	if !ok {
		return nil, fmt.Errorf("script must define a handle(params) function")
	}

	// Execute with timeout.
	type result struct {
		val goja.Value
		err error
	}
	done := make(chan result, 1)

	// Set up timeout interrupt.
	timer := time.AfterFunc(r.timeout, func() {
		vm.Interrupt("timeout: exceeded " + r.timeout.String())
	})

	go func() {
		val, err := handleFn(goja.Undefined(), vm.ToValue(params))
		done <- result{val, err}
	}()

	res := <-done
	timer.Stop()

	if res.err != nil {
		return nil, fmt.Errorf("handle() error: %w", res.err)
	}

	// Convert result to map.
	exported := res.val.Export()
	switch v := exported.(type) {
	case map[string]any:
		if errMsg, ok := v["error"]; ok {
			log.Printf("[jsruntime] %s returned error: %v", toolName, errMsg)
		} else {
			log.Printf("[jsruntime] %s completed OK", toolName)
		}
		return v, nil
	default:
		log.Printf("[jsruntime] %s completed OK", toolName)
		return map[string]any{"result": exported}, nil
	}
}
