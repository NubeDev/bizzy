package apps

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// mockPluginQuery implements PluginQuerySource for unit testing the
// plugins.* Goja host API without NATS or a real plugin registry.
type mockPluginQuery struct {
	plugins map[string]*PluginInfoResult
	callFn  func(pluginName, toolName string, params map[string]any) (any, error)
}

func (m *mockPluginQuery) PluginExists(name string) bool {
	info, ok := m.plugins[name]
	return ok && info.Status == "active"
}

func (m *mockPluginQuery) PluginInfo(name string) *PluginInfoResult {
	return m.plugins[name]
}

func (m *mockPluginQuery) PluginList(serviceFilter string) []string {
	var names []string
	for name, info := range m.plugins {
		if info.Status != "active" {
			continue
		}
		if serviceFilter != "" {
			found := false
			for _, s := range info.Services {
				if s == serviceFilter {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		names = append(names, name)
	}
	return names
}

func (m *mockPluginQuery) CallPluginTool(pluginName, toolName string, params map[string]any) (any, error) {
	if m.callFn != nil {
		return m.callFn(pluginName, toolName, params)
	}
	return nil, nil
}

// newTestRuntime creates a JSRuntime with a mock plugin query source for testing.
func newTestRuntime(pq PluginQuerySource) *JSRuntime {
	rt := &JSRuntime{
		allowedHosts: []string{},
		secrets:      map[string]string{},
		config:       map[string]string{},
		timeout:      5 * time.Second,
	}
	if pq != nil {
		rt.SetPluginQuery(pq)
	}
	return rt
}

func TestPluginsAPI_Exists(t *testing.T) {
	mock := &mockPluginQuery{
		plugins: map[string]*PluginInfoResult{
			"starter": {Name: "starter", Version: "0.1.0", Status: "active", Services: []string{"tools"}, Tools: []string{"echo"}},
			"crashed": {Name: "crashed", Version: "1.0.0", Status: "crashed", Services: []string{"tools"}, Tools: []string{"foo"}},
		},
	}
	rt := newTestRuntime(mock)

	t.Run("active plugin exists", func(t *testing.T) {
		result, err := rt.ExecuteScript(`function handle(params) { return {exists: plugins.exists("starter")}; }`, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if result["exists"] != true {
			t.Fatalf("expected true, got %v", result["exists"])
		}
	})

	t.Run("crashed plugin does not exist", func(t *testing.T) {
		result, err := rt.ExecuteScript(`function handle(params) { return {exists: plugins.exists("crashed")}; }`, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if result["exists"] != false {
			t.Fatalf("expected false, got %v", result["exists"])
		}
	})

	t.Run("unknown plugin does not exist", func(t *testing.T) {
		result, err := rt.ExecuteScript(`function handle(params) { return {exists: plugins.exists("nope")}; }`, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if result["exists"] != false {
			t.Fatalf("expected false, got %v", result["exists"])
		}
	})
}

func TestPluginsAPI_Info(t *testing.T) {
	mock := &mockPluginQuery{
		plugins: map[string]*PluginInfoResult{
			"starter": {Name: "starter", Version: "0.1.0", Status: "active", Services: []string{"tools"}, Tools: []string{"echo"}},
		},
	}
	rt := newTestRuntime(mock)

	t.Run("known plugin returns info", func(t *testing.T) {
		result, err := rt.ExecuteScript(`function handle(params) { return plugins.info("starter"); }`, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if result["name"] != "starter" {
			t.Fatalf("expected name=starter, got %v", result["name"])
		}
		if result["version"] != "0.1.0" {
			t.Fatalf("expected version=0.1.0, got %v", result["version"])
		}
		if result["status"] != "active" {
			t.Fatalf("expected status=active, got %v", result["status"])
		}
	})

	t.Run("unknown plugin returns null", func(t *testing.T) {
		result, err := rt.ExecuteScript(`function handle(params) { return {info: plugins.info("nope")}; }`, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if result["info"] != nil {
			t.Fatalf("expected null, got %v", result["info"])
		}
	})
}

func TestPluginsAPI_List(t *testing.T) {
	mock := &mockPluginQuery{
		plugins: map[string]*PluginInfoResult{
			"starter":  {Name: "starter", Version: "0.1.0", Status: "active", Services: []string{"tools"}, Tools: []string{"echo"}},
			"notifier": {Name: "notifier", Version: "1.0.0", Status: "active", Services: []string{"handler"}, Tools: []string{}},
			"dead":     {Name: "dead", Version: "1.0.0", Status: "crashed", Services: []string{"tools"}, Tools: []string{}},
		},
	}
	rt := newTestRuntime(mock)

	t.Run("list all active", func(t *testing.T) {
		result, err := rt.ExecuteScript(`function handle(params) { return {count: plugins.list().length}; }`, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		// Should have 2 active plugins (starter, notifier), not the crashed one
		count, _ := result["count"].(int64)
		if count != 2 {
			t.Fatalf("expected 2 active plugins, got %v", result["count"])
		}
	})

	t.Run("list filtered by service", func(t *testing.T) {
		result, err := rt.ExecuteScript(`function handle(params) {
			var list = plugins.list("tools");
			return {count: list.length, first: list[0]};
		}`, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		count, _ := result["count"].(int64)
		if count != 1 {
			t.Fatalf("expected 1 tools plugin, got %v", result["count"])
		}
		if result["first"] != "starter" {
			t.Fatalf("expected starter, got %v", result["first"])
		}
	})
}

func TestPluginsAPI_Call(t *testing.T) {
	mock := &mockPluginQuery{
		plugins: map[string]*PluginInfoResult{
			"starter": {Name: "starter", Version: "0.1.0", Status: "active", Services: []string{"tools"}, Tools: []string{"echo"}},
		},
		callFn: func(pluginName, toolName string, params map[string]any) (any, error) {
			// Simulate the starter plugin's echo tool
			text, _ := params["text"].(string)
			return map[string]any{
				"plugin":  pluginName,
				"tool":    toolName,
				"message": text,
			}, nil
		},
	}
	rt := newTestRuntime(mock)

	t.Run("successful call", func(t *testing.T) {
		script := `function handle(params) {
			var resp = plugins.call("starter", "echo", {text: "hello world"});
			return resp;
		}`
		result, err := rt.ExecuteScript(script, "", nil)
		if err != nil {
			t.Fatal(err)
		}

		// Result should be {result: {plugin: "starter", tool: "echo", message: "hello world"}}
		inner, ok := result["result"].(map[string]any)
		if !ok {
			t.Fatalf("expected result map, got %T: %v", result["result"], result)
		}
		if inner["message"] != "hello world" {
			t.Fatalf("expected message 'hello world', got %v", inner["message"])
		}
		if inner["plugin"] != "starter" {
			t.Fatalf("expected plugin 'starter', got %v", inner["plugin"])
		}
	})

	t.Run("error returned as error field", func(t *testing.T) {
		mockErr := &mockPluginQuery{
			plugins: map[string]*PluginInfoResult{},
			callFn: func(_, _ string, _ map[string]any) (any, error) {
				return nil, json.Unmarshal([]byte("invalid"), nil) // forces an error
			},
		}
		rt2 := newTestRuntime(mockErr)
		script := `function handle(params) {
			var resp = plugins.call("bad", "tool", {});
			return resp;
		}`
		result, err := rt2.ExecuteScript(script, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		errStr, ok := result["error"].(string)
		if !ok || errStr == "" {
			t.Fatalf("expected error string, got %v", result)
		}
	})
}

func TestPluginsAPI_GracefulWithoutPlugins(t *testing.T) {
	// No plugin query source — simulates NATS bus disabled
	rt := newTestRuntime(nil)

	t.Run("exists returns false", func(t *testing.T) {
		result, err := rt.ExecuteScript(`function handle(params) { return {exists: plugins.exists("anything")}; }`, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if result["exists"] != false {
			t.Fatalf("expected false, got %v", result["exists"])
		}
	})

	t.Run("list returns empty", func(t *testing.T) {
		result, err := rt.ExecuteScript(`function handle(params) { return {count: plugins.list().length}; }`, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		count, _ := result["count"].(int64)
		if count != 0 {
			t.Fatalf("expected 0, got %v", count)
		}
	})

	t.Run("call returns error", func(t *testing.T) {
		result, err := rt.ExecuteScript(`function handle(params) { return plugins.call("x", "y", {}); }`, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		errStr, _ := result["error"].(string)
		if !strings.Contains(errStr, "not available") {
			t.Fatalf("expected 'not available' error, got %v", result)
		}
	})

	t.Run("info returns null", func(t *testing.T) {
		result, err := rt.ExecuteScript(`function handle(params) { return {info: plugins.info("x")}; }`, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if result["info"] != nil {
			t.Fatalf("expected null, got %v", result["info"])
		}
	})
}

func TestPluginsAPI_RealWorldPattern(t *testing.T) {
	// Test a realistic pattern: app checks if plugin exists, calls it if so, falls back otherwise
	mock := &mockPluginQuery{
		plugins: map[string]*PluginInfoResult{
			"starter": {Name: "starter", Version: "0.1.0", Status: "active", Services: []string{"tools"}, Tools: []string{"echo"}},
		},
		callFn: func(pluginName, toolName string, params map[string]any) (any, error) {
			text, _ := params["text"].(string)
			return map[string]any{"message": text}, nil
		},
	}
	rt := newTestRuntime(mock)

	script := `function handle(params) {
		var result = {};

		if (plugins.exists("starter")) {
			var info = plugins.info("starter");
			result.plugin_version = info.version;
			result.tool_count = info.tools.length;

			var resp = plugins.call("starter", "echo", {text: "integration test"});
			if (resp.error) {
				result.error = resp.error;
			} else {
				result.echo = resp.result.message;
			}
		} else {
			result.fallback = "starter plugin not available";
		}

		result.all_plugins = plugins.list();
		result.tool_plugins = plugins.list("tools");

		return result;
	}`

	result, err := rt.ExecuteScript(script, "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if result["plugin_version"] != "0.1.0" {
		t.Fatalf("expected version 0.1.0, got %v", result["plugin_version"])
	}
	// Goja returns int64 for integer values
	toolCount, ok := result["tool_count"].(int64)
	if !ok {
		t.Fatalf("expected int64 for tool_count, got %T: %v", result["tool_count"], result["tool_count"])
	}
	if toolCount != 1 {
		t.Fatalf("expected 1 tool, got %v", toolCount)
	}
	if result["echo"] != "integration test" {
		t.Fatalf("expected echo='integration test', got %v", result["echo"])
	}
	if result["fallback"] != nil {
		t.Fatalf("should not have fallback, got %v", result["fallback"])
	}
}
