package apps

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/dop251/goja"
)

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
			if r.transport != nil {
				client.Transport = r.transport
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

// --- Host API: plugins.* ---

func (r *JSRuntime) injectPluginsAPI(vm *goja.Runtime) {
	pluginsObj := vm.NewObject()

	if r.pluginQuery == nil {
		// No plugin system available — all methods return safe defaults.
		pluginsObj.Set("exists", func(call goja.FunctionCall) goja.Value {
			return vm.ToValue(false)
		})
		pluginsObj.Set("info", func(call goja.FunctionCall) goja.Value {
			return goja.Null()
		})
		pluginsObj.Set("list", func(call goja.FunctionCall) goja.Value {
			return vm.ToValue([]string{})
		})
		pluginsObj.Set("call", func(call goja.FunctionCall) goja.Value {
			return vm.ToValue(map[string]any{"error": "plugin system not available"})
		})
		vm.Set("plugins", pluginsObj)
		return
	}

	pq := r.pluginQuery

	// plugins.exists(name) → boolean
	pluginsObj.Set("exists", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(false)
		}
		return vm.ToValue(pq.PluginExists(call.Arguments[0].String()))
	})

	// plugins.info(name) → {name, version, status, services, tools} or null
	pluginsObj.Set("info", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		info := pq.PluginInfo(call.Arguments[0].String())
		if info == nil {
			return goja.Null()
		}
		return vm.ToValue(map[string]any{
			"name":     info.Name,
			"version":  info.Version,
			"status":   info.Status,
			"services": info.Services,
			"tools":    info.Tools,
		})
	})

	// plugins.list(serviceFilter?) → [name, ...]
	pluginsObj.Set("list", func(call goja.FunctionCall) goja.Value {
		filter := ""
		if len(call.Arguments) >= 1 {
			filter = call.Arguments[0].String()
		}
		return vm.ToValue(pq.PluginList(filter))
	})

	// plugins.call(pluginName, toolName, params) → {result: ...} or {error: "..."}
	pluginsObj.Set("call", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 3 {
			panic(vm.NewGoError(fmt.Errorf("plugins.call: requires pluginName, toolName, params")))
		}
		pluginName := call.Arguments[0].String()
		toolName := call.Arguments[1].String()

		var params map[string]any
		if exported := call.Arguments[2].Export(); exported != nil {
			if m, ok := exported.(map[string]any); ok {
				params = m
			}
		}
		if params == nil {
			params = make(map[string]any)
		}

		result, err := pq.CallPluginTool(pluginName, toolName, params)
		if err != nil {
			return vm.ToValue(map[string]any{"error": err.Error()})
		}
		return vm.ToValue(map[string]any{"result": result})
	})

	vm.Set("plugins", pluginsObj)
}

// --- Host API: tools.call() ---

func (r *JSRuntime) injectToolsAPI(vm *goja.Runtime) {
	toolsObj := vm.NewObject()

	if r.toolCaller == nil {
		toolsObj.Set("call", func(call goja.FunctionCall) goja.Value {
			return vm.ToValue(map[string]any{"error": "tools.call not available in this context"})
		})
		vm.Set("tools", toolsObj)
		return
	}

	tc := r.toolCaller

	// tools.call(toolName, params) → {result: ...} or {error: "..."}
	toolsObj.Set("call", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			panic(vm.NewGoError(fmt.Errorf("tools.call: requires toolName, params")))
		}
		toolName := call.Arguments[0].String()

		var params map[string]any
		if exported := call.Arguments[1].Export(); exported != nil {
			if m, ok := exported.(map[string]any); ok {
				params = m
			}
		}
		if params == nil {
			params = make(map[string]any)
		}

		result, err := tc.CallTool(toolName, params)
		if err != nil {
			return vm.ToValue(map[string]any{"error": err.Error()})
		}
		return vm.ToValue(map[string]any{"result": result})
	})

	vm.Set("tools", toolsObj)
}

// --- Host API: base64.encode/decode ---

func (r *JSRuntime) injectBase64API(vm *goja.Runtime) {
	b64Obj := vm.NewObject()

	// base64.encode(str) → string
	b64Obj.Set("encode", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewGoError(fmt.Errorf("base64.encode: input required")))
		}
		input := call.Arguments[0].String()
		return vm.ToValue(base64.StdEncoding.EncodeToString([]byte(input)))
	})

	// base64.decode(str) → string (or error on invalid input)
	b64Obj.Set("decode", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewGoError(fmt.Errorf("base64.decode: input required")))
		}
		input := call.Arguments[0].String()
		decoded, err := base64.StdEncoding.DecodeString(input)
		if err != nil {
			// Try URL-safe encoding as fallback.
			decoded, err = base64.URLEncoding.DecodeString(input)
			if err != nil {
				panic(vm.NewGoError(fmt.Errorf("base64.decode: %w", err)))
			}
		}
		return vm.ToValue(string(decoded))
	})

	vm.Set("base64", b64Obj)
}

// --- Host API: url.buildQuery/parse ---

func (r *JSRuntime) injectURLAPI(vm *goja.Runtime) {
	urlObj := vm.NewObject()

	// url.buildQuery({key: "value", ...}) → "key=value&..."
	urlObj.Set("buildQuery", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue("")
		}
		exported := call.Arguments[0].Export()
		params, ok := exported.(map[string]any)
		if !ok {
			return vm.ToValue("")
		}
		vals := url.Values{}
		for k, v := range params {
			vals.Set(k, fmt.Sprintf("%v", v))
		}
		return vm.ToValue(vals.Encode())
	})

	// url.parse(urlStr) → {protocol, host, path, query, hash}
	urlObj.Set("parse", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewGoError(fmt.Errorf("url.parse: url required")))
		}
		raw := call.Arguments[0].String()
		u, err := url.Parse(raw)
		if err != nil {
			panic(vm.NewGoError(fmt.Errorf("url.parse: %w", err)))
		}
		return vm.ToValue(map[string]any{
			"protocol": u.Scheme,
			"host":     u.Host,
			"path":     u.Path,
			"query":    u.RawQuery,
			"hash":     u.Fragment,
		})
	})

	vm.Set("url", urlObj)
}

// --- Host API: crypto.sha256/hmac ---

func (r *JSRuntime) injectCryptoAPI(vm *goja.Runtime) {
	cryptoObj := vm.NewObject()

	hashFn := func(algo string) func(goja.FunctionCall) goja.Value {
		return func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) < 1 {
				panic(vm.NewGoError(fmt.Errorf("crypto.%s: data required", algo)))
			}
			data := []byte(call.Arguments[0].String())
			var h hash.Hash
			switch algo {
			case "sha256":
				h = sha256.New()
			case "sha1":
				h = sha1.New()
			case "md5":
				h = md5.New()
			default:
				panic(vm.NewGoError(fmt.Errorf("crypto.%s: unsupported algorithm", algo)))
			}
			h.Write(data)
			return vm.ToValue(hex.EncodeToString(h.Sum(nil)))
		}
	}

	// crypto.sha256(data) → hex string
	cryptoObj.Set("sha256", hashFn("sha256"))
	// crypto.sha1(data) → hex string
	cryptoObj.Set("sha1", hashFn("sha1"))
	// crypto.md5(data) → hex string
	cryptoObj.Set("md5", hashFn("md5"))

	// crypto.hmac(algo, key, data) → hex string
	cryptoObj.Set("hmac", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 3 {
			panic(vm.NewGoError(fmt.Errorf("crypto.hmac: requires algo, key, data")))
		}
		algo := call.Arguments[0].String()
		key := []byte(call.Arguments[1].String())
		data := []byte(call.Arguments[2].String())

		var h func() hash.Hash
		switch algo {
		case "sha256":
			h = sha256.New
		case "sha1":
			h = sha1.New
		case "md5":
			h = md5.New
		default:
			panic(vm.NewGoError(fmt.Errorf("crypto.hmac: unsupported algorithm: %s", algo)))
		}

		mac := hmac.New(h, key)
		mac.Write(data)
		return vm.ToValue(hex.EncodeToString(mac.Sum(nil)))
	})

	vm.Set("crypto", cryptoObj)
}

// --- Host API: env.get() ---

func (r *JSRuntime) injectEnvAPI(vm *goja.Runtime) {
	envObj := vm.NewObject()

	// env.get(key) → string or empty string
	// Only allows reading env vars that match the allowlist prefixes.
	envObj.Set("get", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue("")
		}
		key := call.Arguments[0].String()

		allowed := false
		for _, prefix := range r.envAllowlist {
			if strings.HasPrefix(key, prefix) {
				allowed = true
				break
			}
		}
		if !allowed {
			return vm.ToValue("")
		}
		return vm.ToValue(os.Getenv(key))
	})

	vm.Set("env", envObj)
}
