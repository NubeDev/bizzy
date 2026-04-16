package apps

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dop251/goja"
)

// JSRuntime executes JavaScript tools in a sandboxed Goja VM.
type JSRuntime struct {
	allowedHosts []string
	secrets      map[string]string
	config       map[string]string
	appDir       string
	timeout      time.Duration
}

// NewJSRuntime creates a runtime for executing JS tools within an app.
func NewJSRuntime(app *App, secrets, config map[string]string, timeout time.Duration) *JSRuntime {
	return &JSRuntime{
		allowedHosts: app.Permissions.AllowedHosts,
		secrets:      secrets,
		config:       config,
		appDir:       app.Dir,
		timeout:      timeout,
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

// --- Host API: http.* ---

func (r *JSRuntime) injectHTTPAPI(vm *goja.Runtime) {
	httpObj := vm.NewObject()

	makeRequest := func(method string) func(goja.FunctionCall) goja.Value {
		return func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) < 1 {
				panic(vm.NewGoError(fmt.Errorf("http.%s: url required", strings.ToLower(method))))
			}
			rawURL := call.Arguments[0].String()

			// Enforce allowedHosts.
			if err := CheckAllowedHost(rawURL, r.allowedHosts); err != nil {
				panic(vm.NewGoError(err))
			}

			var bodyReader io.Reader
			var opts map[string]any

			// For methods with body (POST, PUT, PATCH): second arg is body, third is opts.
			// For methods without body (GET, DELETE): second arg is opts.
			if method == "POST" || method == "PUT" || method == "PATCH" {
				if len(call.Arguments) >= 2 {
					bodyData := call.Arguments[1].Export()
					if bodyData != nil {
						jsonBytes, err := json.Marshal(bodyData)
						if err != nil {
							panic(vm.NewGoError(fmt.Errorf("http.%s: marshal body: %w", strings.ToLower(method), err)))
						}
						bodyReader = bytes.NewReader(jsonBytes)
					}
				}
				if len(call.Arguments) >= 3 {
					if o, ok := call.Arguments[2].Export().(map[string]any); ok {
						opts = o
					}
				}
			} else {
				if len(call.Arguments) >= 2 {
					if o, ok := call.Arguments[1].Export().(map[string]any); ok {
						opts = o
					}
				}
			}

			req, err := http.NewRequest(method, rawURL, bodyReader)
			if err != nil {
				panic(vm.NewGoError(fmt.Errorf("http.%s: create request: %w", strings.ToLower(method), err)))
			}

			if bodyReader != nil {
				req.Header.Set("Content-Type", "application/json")
			}

			// Apply headers from opts.
			if opts != nil {
				if headers, ok := opts["headers"].(map[string]any); ok {
					for k, v := range headers {
						req.Header.Set(k, fmt.Sprintf("%v", v))
					}
				}
			}

			client := &http.Client{
				Timeout: r.timeout,
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					// Re-check allowedHosts on redirect.
					if err := CheckAllowedHost(req.URL.String(), r.allowedHosts); err != nil {
						return err
					}
					return nil
				},
			}

			resp, err := client.Do(req)
			if err != nil {
				panic(vm.NewGoError(fmt.Errorf("http.%s: %w", strings.ToLower(method), err)))
			}
			defer resp.Body.Close()

			respBody, _ := io.ReadAll(resp.Body)

			// Return {status, body, headers}.
			result := vm.NewObject()
			result.Set("status", resp.StatusCode)
			result.Set("body", string(respBody))

			respHeaders := vm.NewObject()
			for k, vs := range resp.Header {
				respHeaders.Set(k, strings.Join(vs, ", "))
			}
			result.Set("headers", respHeaders)

			// Try to parse body as JSON for convenience.
			var parsed any
			if json.Unmarshal(respBody, &parsed) == nil {
				result.Set("json", parsed)
			}

			return result
		}
	}

	httpObj.Set("get", makeRequest("GET"))
	httpObj.Set("post", makeRequest("POST"))
	httpObj.Set("put", makeRequest("PUT"))
	httpObj.Set("patch", makeRequest("PATCH"))
	httpObj.Set("delete", makeRequest("DELETE"))

	vm.Set("http", httpObj)
}

// --- Host API: secrets.* ---

func (r *JSRuntime) injectSecretsAPI(vm *goja.Runtime) {
	secretsObj := vm.NewObject()
	for k, v := range r.secrets {
		secretsObj.Set(k, v)
	}
	vm.Set("secrets", secretsObj)
}

// --- Host API: config.* ---

func (r *JSRuntime) injectConfigAPI(vm *goja.Runtime) {
	configObj := vm.NewObject()
	for k, v := range r.config {
		configObj.Set(k, v)
	}
	vm.Set("config", configObj)
}

// --- Host API: log.* ---

func (r *JSRuntime) injectLogAPI(vm *goja.Runtime) {
	logObj := vm.NewObject()
	logObj.Set("info", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			log.Printf("[js] INFO: %s", call.Arguments[0].String())
		}
		return goja.Undefined()
	})
	logObj.Set("error", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			log.Printf("[js] ERROR: %s", call.Arguments[0].String())
		}
		return goja.Undefined()
	})
	vm.Set("log", logObj)
}

// --- Host API: files.read() ---

func (r *JSRuntime) injectFilesAPI(vm *goja.Runtime) {
	filesObj := vm.NewObject()
	filesObj.Set("read", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewGoError(fmt.Errorf("files.read: path required")))
		}
		relPath := call.Arguments[0].String()

		// Prevent directory traversal.
		cleaned := filepath.Clean(relPath)
		if strings.HasPrefix(cleaned, "..") || filepath.IsAbs(cleaned) {
			panic(vm.NewGoError(fmt.Errorf("files.read: path must be relative within app directory")))
		}

		fullPath := filepath.Join(r.appDir, cleaned)

		data, err := os.ReadFile(fullPath)
		if err != nil {
			panic(vm.NewGoError(fmt.Errorf("files.read: %w", err)))
		}
		return vm.ToValue(string(data))
	})
	vm.Set("files", filesObj)
}
