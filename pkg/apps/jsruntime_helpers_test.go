package apps

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// tools.call()
// ---------------------------------------------------------------------------

// mockToolCaller implements ToolCaller for testing.
type mockToolCaller struct {
	tools map[string]func(params map[string]any) (map[string]any, error)
}

func (m *mockToolCaller) CallTool(toolName string, params map[string]any) (map[string]any, error) {
	fn, ok := m.tools[toolName]
	if !ok {
		return nil, fmt.Errorf("tool %s not found in this app", toolName)
	}
	return fn(params)
}

func TestToolsAPI_Call(t *testing.T) {
	tc := &mockToolCaller{
		tools: map[string]func(params map[string]any) (map[string]any, error){
			"list_items": func(params map[string]any) (map[string]any, error) {
				return map[string]any{
					"items": []any{
						map[string]any{"name": "item1"},
						map[string]any{"name": "item2"},
					},
				}, nil
			},
			"broken": func(params map[string]any) (map[string]any, error) {
				return nil, fmt.Errorf("something went wrong")
			},
		},
	}

	rt := newTestRuntime(nil)
	rt.SetToolCaller(tc)

	t.Run("call another tool", func(t *testing.T) {
		script := `function handle(params) {
			var resp = tools.call("list_items", {});
			if (resp.error) return {error: resp.error};
			return {count: resp.result.items.length};
		}`
		result, err := rt.ExecuteScript(script, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		count, _ := result["count"].(int64)
		if count != 2 {
			t.Fatalf("expected 2, got %v", result["count"])
		}
	})

	t.Run("tool not found", func(t *testing.T) {
		script := `function handle(params) {
			return tools.call("nonexistent", {});
		}`
		result, err := rt.ExecuteScript(script, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		errStr, _ := result["error"].(string)
		if !strings.Contains(errStr, "not found") {
			t.Fatalf("expected not found error, got %v", result)
		}
	})

	t.Run("tool returns error", func(t *testing.T) {
		script := `function handle(params) {
			return tools.call("broken", {});
		}`
		result, err := rt.ExecuteScript(script, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		errStr, _ := result["error"].(string)
		if !strings.Contains(errStr, "something went wrong") {
			t.Fatalf("expected error, got %v", result)
		}
	})

	t.Run("compose tools", func(t *testing.T) {
		script := `function handle(params) {
			var items = tools.call("list_items", {});
			if (items.error) return items;
			var names = [];
			for (var i = 0; i < items.result.items.length; i++) {
				names.push(items.result.items[i].name);
			}
			return {names: names, total: names.length};
		}`
		result, err := rt.ExecuteScript(script, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		total, _ := result["total"].(int64)
		if total != 2 {
			t.Fatalf("expected 2, got %v", result["total"])
		}
	})
}

func TestToolsAPI_NotAvailable(t *testing.T) {
	rt := newTestRuntime(nil)
	// No tool caller set
	script := `function handle(params) { return tools.call("anything", {}); }`
	result, err := rt.ExecuteScript(script, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	errStr, _ := result["error"].(string)
	if !strings.Contains(errStr, "not available") {
		t.Fatalf("expected 'not available', got %v", result)
	}
}

// ---------------------------------------------------------------------------
// base64.encode/decode
// ---------------------------------------------------------------------------

func TestBase64API(t *testing.T) {
	rt := newTestRuntime(nil)

	t.Run("encode", func(t *testing.T) {
		result, err := rt.ExecuteScript(`function handle(p) { return {encoded: base64.encode("hello world")}; }`, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		expected := base64.StdEncoding.EncodeToString([]byte("hello world"))
		if result["encoded"] != expected {
			t.Fatalf("expected %s, got %v", expected, result["encoded"])
		}
	})

	t.Run("decode", func(t *testing.T) {
		encoded := base64.StdEncoding.EncodeToString([]byte("hello world"))
		script := fmt.Sprintf(`function handle(p) { return {decoded: base64.decode("%s")}; }`, encoded)
		result, err := rt.ExecuteScript(script, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if result["decoded"] != "hello world" {
			t.Fatalf("expected 'hello world', got %v", result["decoded"])
		}
	})

	t.Run("roundtrip", func(t *testing.T) {
		script := `function handle(p) {
			var enc = base64.encode("user:password123");
			var dec = base64.decode(enc);
			return {original: "user:password123", decoded: dec, match: dec === "user:password123"};
		}`
		result, err := rt.ExecuteScript(script, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if result["match"] != true {
			t.Fatalf("roundtrip failed: %v", result)
		}
	})

	t.Run("basic auth pattern", func(t *testing.T) {
		script := `function handle(p) {
			var creds = base64.encode("admin:secret");
			return {header: "Basic " + creds};
		}`
		result, err := rt.ExecuteScript(script, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		header, _ := result["header"].(string)
		if !strings.HasPrefix(header, "Basic ") {
			t.Fatalf("expected Basic auth header, got %v", header)
		}
	})
}

// ---------------------------------------------------------------------------
// url.buildQuery/parse
// ---------------------------------------------------------------------------

func TestURLAPI(t *testing.T) {
	rt := newTestRuntime(nil)

	t.Run("buildQuery", func(t *testing.T) {
		script := `function handle(p) {
			var qs = url.buildQuery({city: "New York", limit: 5, active: true});
			return {query: qs};
		}`
		result, err := rt.ExecuteScript(script, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		qs, _ := result["query"].(string)
		if !strings.Contains(qs, "city=New+York") && !strings.Contains(qs, "city=New%20York") {
			t.Fatalf("expected city param, got %s", qs)
		}
		if !strings.Contains(qs, "limit=5") {
			t.Fatalf("expected limit param, got %s", qs)
		}
	})

	t.Run("buildQuery empty", func(t *testing.T) {
		result, err := rt.ExecuteScript(`function handle(p) { return {q: url.buildQuery({})}; }`, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if result["q"] != "" {
			t.Fatalf("expected empty string, got %v", result["q"])
		}
	})

	t.Run("parse", func(t *testing.T) {
		script := `function handle(p) {
			return url.parse("https://api.example.com:8080/v1/data?key=val#section");
		}`
		result, err := rt.ExecuteScript(script, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if result["protocol"] != "https" {
			t.Fatalf("expected https, got %v", result["protocol"])
		}
		if result["host"] != "api.example.com:8080" {
			t.Fatalf("expected api.example.com:8080, got %v", result["host"])
		}
		if result["path"] != "/v1/data" {
			t.Fatalf("expected /v1/data, got %v", result["path"])
		}
		if result["query"] != "key=val" {
			t.Fatalf("expected key=val, got %v", result["query"])
		}
		if result["hash"] != "section" {
			t.Fatalf("expected section, got %v", result["hash"])
		}
	})
}

// ---------------------------------------------------------------------------
// crypto.sha256/sha1/md5/hmac
// ---------------------------------------------------------------------------

func TestCryptoAPI(t *testing.T) {
	rt := newTestRuntime(nil)

	t.Run("sha256", func(t *testing.T) {
		result, err := rt.ExecuteScript(`function handle(p) { return {hash: crypto.sha256("hello")}; }`, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		expected := sha256Hex("hello")
		if result["hash"] != expected {
			t.Fatalf("expected %s, got %v", expected, result["hash"])
		}
	})

	t.Run("md5", func(t *testing.T) {
		result, err := rt.ExecuteScript(`function handle(p) { return {hash: crypto.md5("hello")}; }`, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		hash, _ := result["hash"].(string)
		if len(hash) != 32 { // md5 hex is 32 chars
			t.Fatalf("expected 32-char md5 hex, got %d chars: %s", len(hash), hash)
		}
	})

	t.Run("hmac sha256", func(t *testing.T) {
		script := `function handle(p) {
			return {sig: crypto.hmac("sha256", "mysecret", "data to sign")};
		}`
		result, err := rt.ExecuteScript(script, "", nil)
		if err != nil {
			t.Fatal(err)
		}

		// Verify against Go's crypto
		mac := hmac.New(sha256.New, []byte("mysecret"))
		mac.Write([]byte("data to sign"))
		expected := hex.EncodeToString(mac.Sum(nil))

		if result["sig"] != expected {
			t.Fatalf("expected %s, got %v", expected, result["sig"])
		}
	})

	t.Run("webhook verification pattern", func(t *testing.T) {
		script := `function handle(p) {
			var body = '{"event":"push","repo":"bizzy"}';
			var sig = crypto.hmac("sha256", p.secret, body);
			return {signature: "sha256=" + sig, body_hash: crypto.sha256(body)};
		}`
		result, err := rt.ExecuteScript(script, "", map[string]any{"secret": "webhook_secret_123"})
		if err != nil {
			t.Fatal(err)
		}
		sig, _ := result["signature"].(string)
		if !strings.HasPrefix(sig, "sha256=") {
			t.Fatalf("expected sha256= prefix, got %v", sig)
		}
		bodyHash, _ := result["body_hash"].(string)
		if len(bodyHash) != 64 { // sha256 hex is 64 chars
			t.Fatalf("expected 64-char sha256 hex, got %d", len(bodyHash))
		}
	})

	t.Run("unsupported algorithm", func(t *testing.T) {
		script := `function handle(p) { return {hash: crypto.hmac("sha512", "key", "data")}; }`
		_, err := rt.ExecuteScript(script, "", nil)
		if err == nil {
			t.Fatal("expected error for unsupported algo")
		}
		if !strings.Contains(err.Error(), "unsupported") {
			t.Fatalf("expected unsupported error, got %v", err)
		}
	})
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// ---------------------------------------------------------------------------
// env.get()
// ---------------------------------------------------------------------------

func TestEnvAPI(t *testing.T) {
	rt := newTestRuntime(nil)
	rt.envAllowlist = []string{"BIZZY_", "NUBE_", "TEST_BIZZY_"}

	t.Run("allowed prefix", func(t *testing.T) {
		os.Setenv("BIZZY_TEST_VAR", "hello_from_env")
		defer os.Unsetenv("BIZZY_TEST_VAR")

		result, err := rt.ExecuteScript(`function handle(p) { return {val: env.get("BIZZY_TEST_VAR")}; }`, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if result["val"] != "hello_from_env" {
			t.Fatalf("expected hello_from_env, got %v", result["val"])
		}
	})

	t.Run("blocked prefix returns empty", func(t *testing.T) {
		os.Setenv("SECRET_KEY", "should_not_see_this")
		defer os.Unsetenv("SECRET_KEY")

		result, err := rt.ExecuteScript(`function handle(p) { return {val: env.get("SECRET_KEY")}; }`, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if result["val"] != "" {
			t.Fatalf("expected empty string for blocked var, got %v", result["val"])
		}
	})

	t.Run("PATH is blocked", func(t *testing.T) {
		result, err := rt.ExecuteScript(`function handle(p) { return {val: env.get("PATH")}; }`, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if result["val"] != "" {
			t.Fatalf("PATH should be blocked, got %v", result["val"])
		}
	})

	t.Run("unset var returns empty", func(t *testing.T) {
		result, err := rt.ExecuteScript(`function handle(p) { return {val: env.get("BIZZY_NONEXISTENT_12345")}; }`, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if result["val"] != "" {
			t.Fatalf("expected empty for unset var, got %v", result["val"])
		}
	})

	t.Run("fallback pattern", func(t *testing.T) {
		script := `function handle(p) {
			var host = env.get("NUBE_CUSTOM_HOST") || "http://localhost:8080";
			return {host: host};
		}`
		result, err := rt.ExecuteScript(script, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if result["host"] != "http://localhost:8080" {
			t.Fatalf("expected fallback, got %v", result["host"])
		}
	})
}

// ---------------------------------------------------------------------------
// Integration: combine multiple APIs in one tool
// ---------------------------------------------------------------------------

func TestCombinedAPIs(t *testing.T) {
	tc := &mockToolCaller{
		tools: map[string]func(params map[string]any) (map[string]any, error){
			"get_data": func(params map[string]any) (map[string]any, error) {
				return map[string]any{"value": "secret_data_123"}, nil
			},
		},
	}

	rt := newTestRuntime(nil)
	rt.SetToolCaller(tc)
	rt.envAllowlist = []string{"BIZZY_"}

	script := `function handle(params) {
		// 1. Call another tool to get data
		var data = tools.call("get_data", {});
		if (data.error) return {error: data.error};

		// 2. Hash the data
		var hash = crypto.sha256(data.result.value);

		// 3. Base64 encode for transport
		var encoded = base64.encode(data.result.value);

		// 4. Build a query string
		var qs = url.buildQuery({hash: hash, format: "json"});

		// 5. Check env
		var debug = env.get("BIZZY_DEBUG") || "false";

		return {
			hash: hash,
			encoded: encoded,
			query: qs,
			debug: debug,
			original: data.result.value
		};
	}`

	result, err := rt.ExecuteScript(script, "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if result["original"] != "secret_data_123" {
		t.Fatalf("tools.call failed: %v", result)
	}
	hash, _ := result["hash"].(string)
	if len(hash) != 64 {
		t.Fatalf("expected sha256 hash, got %v", hash)
	}
	encoded, _ := result["encoded"].(string)
	decoded, _ := base64.StdEncoding.DecodeString(encoded)
	if string(decoded) != "secret_data_123" {
		t.Fatalf("base64 roundtrip failed: %v", encoded)
	}
	qs, _ := result["query"].(string)
	if !strings.Contains(qs, "format=json") {
		t.Fatalf("query string missing format: %v", qs)
	}
}
