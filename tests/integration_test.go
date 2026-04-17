// Package tests provides end-to-end integration tests for the nube-server.
//
// These tests start a real fakeserver (upstream device API) and a real nube-server,
// then exercise the full flow: bootstrap → users → apps → install → MCP tools → JS tools.
//
// Run:
//
//	go test ./tests/ -v -count=1
package tests

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NubeDev/bizzy/pkg/airunner"
	"github.com/NubeDev/bizzy/pkg/api"
	"github.com/NubeDev/bizzy/pkg/apps"
	"github.com/NubeDev/bizzy/pkg/jsondb"
	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/NubeDev/bizzy/pkg/services"
	"github.com/gin-gonic/gin"
)

// ---------- test environment ----------

type testEnv struct {
	t          *testing.T
	serverURL  string
	fakeURL    string
	adminToken string
	wsID       string
}

func setupEnv(t *testing.T) *testEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)

	// 1. Start fakeserver (upstream device API).
	fakeRouter := newFakeDeviceRouter()
	fakeLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen fakeserver: %v", err)
	}
	// Use "localhost" instead of "127.0.0.1" so it matches the app's allowedHosts ("localhost:*").
	_, fakePort, _ := net.SplitHostPort(fakeLn.Addr().String())
	fakeURL := "http://localhost:" + fakePort
	go http.Serve(fakeLn, fakeRouter)
	t.Cleanup(func() { fakeLn.Close() })

	// 2. Set up nube-server with temp data dir.
	dataDir := t.TempDir()
	appsDir, _ := filepath.Abs("../apps")

	workspaces, _ := jsondb.NewCollection[models.Workspace](filepath.Join(dataDir, "workspaces.json"))
	users, _ := jsondb.NewCollection[models.User](filepath.Join(dataDir, "users.json"))
	appInstalls, _ := jsondb.NewCollection[models.AppInstall](filepath.Join(dataDir, "app_installs.json"))

	registry, err := apps.NewRegistry(appsDir)
	if err != nil {
		t.Fatalf("load apps: %v", err)
	}
	mcpFactory := apps.NewMCPFactory(registry)

	runners := airunner.NewRegistry()
	agentSvc := &services.AgentService{
		MCPFactory:  mcpFactory,
		AppInstalls: appInstalls,
		Users:       users,
		Runners:     runners,
		Jobs:        airunner.NewJobStore(),
		AppRegistry: registry,
	}
	toolSvc := &services.ToolService{
		AppInstalls: appInstalls,
		AppRegistry: registry,
	}

	a := &api.API{
		Workspaces:  workspaces,
		Users:       users,
		AppInstalls: appInstalls,
		AppRegistry: registry,
		MCPFactory:  mcpFactory,
		Runners:     runners,
		Jobs:        agentSvc.Jobs,
		AgentSvc:    agentSvc,
		ToolSvc:     toolSvc,
	}

	router := a.SetupRouter()
	srvLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen nube-server: %v", err)
	}
	serverURL := "http://" + srvLn.Addr().String()
	go http.Serve(srvLn, router)
	t.Cleanup(func() { srvLn.Close() })

	return &testEnv{
		t:         t,
		serverURL: serverURL,
		fakeURL:   fakeURL,
	}
}

// ---------- HTTP helpers ----------

func (e *testEnv) get(path, token string) *httpResult {
	return e.do("GET", path, token, nil)
}

func (e *testEnv) post(path, token string, body any) *httpResult {
	return e.do("POST", path, token, body)
}

func (e *testEnv) patch(path, token string, body any) *httpResult {
	return e.do("PATCH", path, token, body)
}

func (e *testEnv) delete(path, token string) *httpResult {
	return e.do("DELETE", path, token, nil)
}

func (e *testEnv) do(method, path, token string, body any) *httpResult {
	e.t.Helper()
	var bodyReader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, e.serverURL+path, bodyReader)
	if err != nil {
		e.t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return &httpResult{status: resp.StatusCode, body: data, t: e.t}
}

type httpResult struct {
	status int
	body   []byte
	t      *testing.T
}

func (r *httpResult) expectStatus(code int) *httpResult {
	r.t.Helper()
	if r.status != code {
		r.t.Fatalf("expected status %d, got %d: %s", code, r.status, string(r.body))
	}
	return r
}

func (r *httpResult) json() map[string]any {
	r.t.Helper()
	var m map[string]any
	if err := json.Unmarshal(r.body, &m); err != nil {
		r.t.Fatalf("unmarshal response: %v\nbody: %s", err, string(r.body))
	}
	return m
}

func (r *httpResult) jsonArray() []any {
	r.t.Helper()
	var arr []any
	if err := json.Unmarshal(r.body, &arr); err != nil {
		r.t.Fatalf("unmarshal array: %v\nbody: %s", err, string(r.body))
	}
	return arr
}

func (r *httpResult) getString(key string) string {
	r.t.Helper()
	m := r.json()
	v, ok := m[key]
	if !ok {
		r.t.Fatalf("key %q not in response: %v", key, m)
	}
	s, ok := v.(string)
	if !ok {
		r.t.Fatalf("key %q is not a string: %v", key, v)
	}
	return s
}

// ---------- MCP helpers ----------

type mcpSession struct {
	t         *testing.T
	serverURL string
	token     string
	sessionID string
}

func (e *testEnv) mcpInit(token string) *mcpSession {
	e.t.Helper()
	body := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "test", "version": "1.0"},
		},
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", e.serverURL+"/mcp", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatalf("mcp init: %v", err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	sessionID := resp.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		e.t.Fatal("no Mcp-Session-Id header in init response")
	}

	return &mcpSession{
		t:         e.t,
		serverURL: e.serverURL,
		token:     token,
		sessionID: sessionID,
	}
}

func (s *mcpSession) call(method string, params map[string]any) map[string]any {
	s.t.Helper()
	body := map[string]any{
		"jsonrpc": "2.0",
		"id":      time.Now().UnixNano(),
		"method":  method,
		"params":  params,
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", s.serverURL+"/mcp", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Mcp-Session-Id", s.sessionID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.t.Fatalf("mcp %s: %v", method, err)
	}
	defer resp.Body.Close()

	// Parse SSE: find "data:" lines.
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			var result map[string]any
			if err := json.Unmarshal([]byte(line[6:]), &result); err == nil {
				return result
			}
		}
	}
	s.t.Fatalf("mcp %s: no data in SSE response", method)
	return nil
}

func (s *mcpSession) listTools() []string {
	s.t.Helper()
	result := s.call("tools/list", map[string]any{})
	res, _ := result["result"].(map[string]any)
	tools, _ := res["tools"].([]any)
	var names []string
	for _, t := range tools {
		tm, _ := t.(map[string]any)
		names = append(names, tm["name"].(string))
	}
	return names
}

func (s *mcpSession) listPrompts() []string {
	s.t.Helper()
	result := s.call("prompts/list", map[string]any{})
	res, _ := result["result"].(map[string]any)
	prompts, _ := res["prompts"].([]any)
	var names []string
	for _, p := range prompts {
		pm, _ := p.(map[string]any)
		names = append(names, pm["name"].(string))
	}
	return names
}

func (s *mcpSession) callTool(name string, args map[string]any) (string, bool) {
	s.t.Helper()
	result := s.call("tools/call", map[string]any{
		"name":      name,
		"arguments": args,
	})
	res, _ := result["result"].(map[string]any)
	content, _ := res["content"].([]any)
	isError, _ := res["isError"].(bool)
	if len(content) == 0 {
		return "", isError
	}
	first, _ := content[0].(map[string]any)
	text, _ := first["text"].(string)
	return text, isError
}

// ---------- fakeserver (inline) ----------

func newFakeDeviceRouter() http.Handler {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	type device struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Model  string `json:"model"`
		IP     string `json:"ip"`
		Online bool   `json:"online"`
	}

	devices := map[string]device{
		"dev-001": {ID: "dev-001", Name: "Office HVAC", Model: "RC-5", IP: "192.168.1.10", Online: true},
		"dev-002": {ID: "dev-002", Name: "Lobby Sensor", Model: "ZS-1", IP: "192.168.1.11", Online: true},
		"dev-003": {ID: "dev-003", Name: "Roof Unit", Model: "RC-5", IP: "192.168.1.12", Online: false},
	}

	r.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	r.GET("/api/v1/devices", func(c *gin.Context) {
		list := make([]device, 0)
		for _, d := range devices {
			list = append(list, d)
		}
		c.JSON(200, gin.H{"data": list, "count": len(list)})
	})
	r.GET("/api/v1/devices/:id", func(c *gin.Context) {
		d, ok := devices[c.Param("id")]
		if !ok {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}
		c.JSON(200, d)
	})
	r.PATCH("/api/v1/devices/:id", func(c *gin.Context) {
		d, ok := devices[c.Param("id")]
		if !ok {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}
		var update map[string]any
		c.ShouldBindJSON(&update)
		if v, ok := update["online"].(bool); ok {
			d.Online = v
		}
		if v, ok := update["name"].(string); ok {
			d.Name = v
		}
		devices[d.ID] = d
		c.JSON(200, d)
	})

	return r
}

// ===================================================================
// TESTS
// ===================================================================

func TestHealthCheck(t *testing.T) {
	env := setupEnv(t)

	r := env.get("/health", "")
	r.expectStatus(200)
	m := r.json()
	if m["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", m["status"])
	}
}

func TestBootstrap(t *testing.T) {
	env := setupEnv(t)

	// First bootstrap succeeds.
	r := env.post("/bootstrap", "", map[string]any{
		"workspaceName": "TestCo",
		"adminName":     "Admin",
		"adminEmail":    "admin@test.com",
	})
	r.expectStatus(201)
	m := r.json()

	admin, _ := m["admin"].(map[string]any)
	if admin["role"] != "admin" {
		t.Fatalf("expected admin role, got %v", admin["role"])
	}
	if admin["token"] == nil || admin["token"] == "" {
		t.Fatal("expected token in bootstrap response")
	}

	// Second bootstrap is rejected.
	r2 := env.post("/bootstrap", "", map[string]any{
		"workspaceName": "Hack",
		"adminName":     "Bad",
		"adminEmail":    "bad@evil.com",
	})
	r2.expectStatus(409)
}

func TestAuthFlow(t *testing.T) {
	env := setupEnv(t)
	env.bootstrap()

	t.Run("valid token", func(t *testing.T) {
		r := env.get("/users/me", env.adminToken)
		r.expectStatus(200)
		if r.getString("email") != "admin@test.com" {
			t.Fatalf("unexpected email: %s", r.getString("email"))
		}
	})

	t.Run("missing token", func(t *testing.T) {
		r := env.get("/users/me", "")
		r.expectStatus(401)
	})

	t.Run("invalid token", func(t *testing.T) {
		r := env.get("/users/me", "bad-token-123")
		r.expectStatus(401)
	})

	t.Run("non-admin blocked from admin routes", func(t *testing.T) {
		joe := env.createUser("Joe", "joe@test.com", "user")
		r := env.post("/workspaces", joe, map[string]any{"name": "hack"})
		r.expectStatus(403)
	})

	t.Run("admin impersonation", func(t *testing.T) {
		joeToken := env.createUser("Joe2", "joe2@test.com", "user")
		joeID := env.getUserID(joeToken)

		// Admin impersonates Joe.
		req, _ := http.NewRequest("GET", env.serverURL+"/users/me", nil)
		req.Header.Set("Authorization", "Bearer "+env.adminToken)
		req.Header.Set("X-Act-As-User", joeID)
		resp, _ := http.DefaultClient.Do(req)
		defer resp.Body.Close()
		var m map[string]any
		json.NewDecoder(resp.Body).Decode(&m)
		if m["name"] != "Joe2" {
			t.Fatalf("expected Joe2, got %v", m["name"])
		}
	})
}

func TestTokenRotation(t *testing.T) {
	env := setupEnv(t)
	env.bootstrap()
	joeToken := env.createUser("Joe", "joe@test.com", "user")
	joeID := env.getUserID(joeToken)

	// Joe rotates own token.
	r := env.post("/users/"+joeID+"/token", joeToken, nil)
	r.expectStatus(200)
	newToken := r.getString("token")

	if newToken == joeToken {
		t.Fatal("token should have changed")
	}

	// Old token no longer works.
	env.get("/users/me", joeToken).expectStatus(401)

	// New token works.
	env.get("/users/me", newToken).expectStatus(200)
}

func TestTokenRevocation(t *testing.T) {
	env := setupEnv(t)
	env.bootstrap()
	joeToken := env.createUser("Joe", "joe@test.com", "user")
	joeID := env.getUserID(joeToken)

	// Admin revokes Joe's token.
	env.delete("/users/"+joeID+"/token", env.adminToken).expectStatus(200)

	// Joe's token no longer works.
	env.get("/users/me", joeToken).expectStatus(401)
}

func TestWorkspaceCRUD(t *testing.T) {
	env := setupEnv(t)
	env.bootstrap()

	t.Run("create workspace", func(t *testing.T) {
		r := env.post("/workspaces", env.adminToken, map[string]any{"name": "NewCo"})
		r.expectStatus(201)
		if r.getString("name") != "NewCo" {
			t.Fatal("workspace name mismatch")
		}
	})

	t.Run("list workspaces", func(t *testing.T) {
		arr := env.get("/workspaces", env.adminToken).expectStatus(200).jsonArray()
		if len(arr) < 2 {
			t.Fatalf("expected at least 2 workspaces, got %d", len(arr))
		}
	})

	t.Run("delete workspace with users fails", func(t *testing.T) {
		env.delete("/workspaces/"+env.wsID, env.adminToken).expectStatus(409)
	})
}

func TestAppCatalog(t *testing.T) {
	env := setupEnv(t)
	env.bootstrap()

	t.Run("list apps", func(t *testing.T) {
		arr := env.get("/apps", env.adminToken).expectStatus(200).jsonArray()
		if len(arr) < 2 {
			t.Fatalf("expected at least 2 apps, got %d", len(arr))
		}

		// Check app names.
		names := make(map[string]bool)
		for _, a := range arr {
			am, _ := a.(map[string]any)
			names[am["name"].(string)] = true
		}
		if !names["rubix-developer"] {
			t.Fatal("missing rubix-developer app")
		}
		if !names["nube-marketing"] {
			t.Fatal("missing nube-marketing app")
		}
	})

	t.Run("get app detail", func(t *testing.T) {
		r := env.get("/apps/rubix-developer", env.adminToken).expectStatus(200)
		m := r.json()
		app, _ := m["app"].(map[string]any)
		if app["hasOpenAPI"] != true {
			t.Fatal("rubix-developer should have OpenAPI")
		}
		if app["hasTools"] != true {
			t.Fatal("rubix-developer should have JS tools")
		}
		prompts, _ := m["prompts"].([]any)
		if len(prompts) == 0 {
			t.Fatal("rubix-developer should have prompts")
		}
	})

	t.Run("unknown app returns 404", func(t *testing.T) {
		env.get("/apps/nonexistent", env.adminToken).expectStatus(404)
	})
}

func TestAppInstallFlow(t *testing.T) {
	env := setupEnv(t)
	env.bootstrap()

	t.Run("install app with settings", func(t *testing.T) {
		r := env.post("/apps/rubix-developer/install", env.adminToken, map[string]any{
			"settings": map[string]any{
				"rubix_host":  env.fakeURL,
				"rubix_token": "secret-123",
			},
		})
		r.expectStatus(201)
		m := r.json()
		if m["appName"] != "rubix-developer" {
			t.Fatal("wrong app name")
		}
		if m["enabled"] != true {
			t.Fatal("should be enabled by default")
		}
	})

	t.Run("duplicate install rejected", func(t *testing.T) {
		env.post("/apps/rubix-developer/install", env.adminToken, map[string]any{
			"settings": map[string]any{"rubix_host": env.fakeURL},
		}).expectStatus(409)
	})

	t.Run("install prompt-only app", func(t *testing.T) {
		env.post("/apps/nube-marketing/install", env.adminToken, map[string]any{}).expectStatus(201)
	})

	t.Run("list installs", func(t *testing.T) {
		arr := env.get("/app-installs", env.adminToken).expectStatus(200).jsonArray()
		if len(arr) != 2 {
			t.Fatalf("expected 2 installs, got %d", len(arr))
		}
	})

	t.Run("disable install", func(t *testing.T) {
		// Get install ID.
		arr := env.get("/app-installs", env.adminToken).expectStatus(200).jsonArray()
		installID := arr[0].(map[string]any)["id"].(string)

		r := env.patch("/app-installs/"+installID, env.adminToken, map[string]any{
			"enabled": false,
		})
		r.expectStatus(200)
		if r.json()["enabled"] != false {
			t.Fatal("should be disabled")
		}
	})

	t.Run("uninstall", func(t *testing.T) {
		arr := env.get("/app-installs", env.adminToken).expectStatus(200).jsonArray()
		installID := arr[1].(map[string]any)["id"].(string)

		env.delete("/app-installs/"+installID, env.adminToken).expectStatus(200)

		arr2 := env.get("/app-installs", env.adminToken).expectStatus(200).jsonArray()
		if len(arr2) != 1 {
			t.Fatalf("expected 1 install after uninstall, got %d", len(arr2))
		}
	})
}

func TestMCPPerUserScoping(t *testing.T) {
	env := setupEnv(t)
	env.bootstrap()

	// Admin installs both apps.
	env.post("/apps/rubix-developer/install", env.adminToken, map[string]any{
		"settings": map[string]any{"rubix_host": env.fakeURL, "rubix_token": "tok"},
	}).expectStatus(201)
	env.post("/apps/nube-marketing/install", env.adminToken, map[string]any{}).expectStatus(201)

	// Create Joe with no apps.
	joeToken := env.createUser("Joe", "joe@test.com", "user")

	t.Run("admin sees all tools and prompts", func(t *testing.T) {
		s := env.mcpInit(env.adminToken)
		tools := s.listTools()
		if len(tools) < 9 {
			t.Fatalf("admin should have 9+ tools, got %d: %v", len(tools), tools)
		}
		// Should include both OpenAPI and JS tools.
		found := map[string]bool{}
		for _, name := range tools {
			found[name] = true
		}
		if !found["rubix-developer.device_summary"] {
			t.Fatal("missing JS tool rubix-developer.device_summary")
		}
		if !found["rubix-developer.listDevices"] {
			t.Fatal("missing OpenAPI tool rubix-developer.listDevices")
		}

		prompts := s.listPrompts()
		if len(prompts) < 3 {
			t.Fatalf("admin should have 3+ prompts, got %d: %v", len(prompts), prompts)
		}
	})

	t.Run("joe sees zero tools (no apps)", func(t *testing.T) {
		s := env.mcpInit(joeToken)
		tools := s.listTools()
		if len(tools) != 0 {
			t.Fatalf("joe should have 0 tools, got %d: %v", len(tools), tools)
		}
		prompts := s.listPrompts()
		if len(prompts) != 0 {
			t.Fatalf("joe should have 0 prompts, got %d: %v", len(prompts), prompts)
		}
	})

	t.Run("joe installs marketing only", func(t *testing.T) {
		env.post("/apps/nube-marketing/install", joeToken, map[string]any{}).expectStatus(201)

		s := env.mcpInit(joeToken)
		tools := s.listTools()
		if len(tools) != 0 {
			t.Fatalf("joe should have 0 tools (marketing is prompt-only), got %d", len(tools))
		}
		prompts := s.listPrompts()
		if len(prompts) != 2 {
			t.Fatalf("joe should have 2 prompts, got %d: %v", len(prompts), prompts)
		}
	})
}

func TestJSToolExecution(t *testing.T) {
	env := setupEnv(t)
	env.bootstrap()

	env.post("/apps/rubix-developer/install", env.adminToken, map[string]any{
		"settings": map[string]any{
			"rubix_host":  env.fakeURL,
			"rubix_token": "test-tok",
		},
	}).expectStatus(201)

	s := env.mcpInit(env.adminToken)

	t.Run("device_summary", func(t *testing.T) {
		text, isError := s.callTool("rubix-developer.device_summary", map[string]any{})
		if isError {
			t.Fatalf("tool returned error: %s", text)
		}

		var result map[string]any
		if err := json.Unmarshal([]byte(text), &result); err != nil {
			t.Fatalf("parse result: %v\ntext: %s", err, text)
		}

		total, _ := result["total"].(float64)
		if total != 3 {
			t.Fatalf("expected 3 total devices, got %v", total)
		}

		online, _ := result["online"].(float64)
		if online != 2 {
			t.Fatalf("expected 2 online, got %v", online)
		}

		offline, _ := result["offline"].(float64)
		if offline != 1 {
			t.Fatalf("expected 1 offline, got %v", offline)
		}
	})

	t.Run("restart_device", func(t *testing.T) {
		text, isError := s.callTool("rubix-developer.restart_device", map[string]any{"id": "dev-001"})
		if isError {
			t.Fatalf("tool returned error: %s", text)
		}

		var result map[string]any
		json.Unmarshal([]byte(text), &result)
		if result["message"] == nil {
			t.Fatalf("expected message in result: %s", text)
		}
	})

	t.Run("restart nonexistent device", func(t *testing.T) {
		text, _ := s.callTool("rubix-developer.restart_device", map[string]any{"id": "dev-999"})

		var result map[string]any
		json.Unmarshal([]byte(text), &result)
		if result["error"] == nil {
			t.Fatalf("expected error for missing device: %s", text)
		}
	})
}

func TestJSToolIsolation(t *testing.T) {
	env := setupEnv(t)
	env.bootstrap()

	// Two users install the same app with different hosts.
	env.post("/apps/rubix-developer/install", env.adminToken, map[string]any{
		"settings": map[string]any{
			"rubix_host":  env.fakeURL,
			"rubix_token": "admin-tok",
		},
	}).expectStatus(201)

	joeToken := env.createUser("Joe", "joe@test.com", "user")
	env.post("/apps/rubix-developer/install", joeToken, map[string]any{
		"settings": map[string]any{
			"rubix_host":  env.fakeURL,
			"rubix_token": "joe-tok",
		},
	}).expectStatus(201)

	// Both should get results (same fakeserver for this test, but proves isolation path).
	adminSession := env.mcpInit(env.adminToken)
	joeSession := env.mcpInit(joeToken)

	adminText, adminErr := adminSession.callTool("rubix-developer.device_summary", map[string]any{})
	joeText, joeErr := joeSession.callTool("rubix-developer.device_summary", map[string]any{})

	if adminErr {
		t.Fatalf("admin tool error: %s", adminText)
	}
	if joeErr {
		t.Fatalf("joe tool error: %s", joeText)
	}

	// Both should get 3 devices.
	for _, text := range []string{adminText, joeText} {
		var result map[string]any
		json.Unmarshal([]byte(text), &result)
		if total, _ := result["total"].(float64); total != 3 {
			t.Fatalf("expected 3 devices, got %v in: %s", total, text)
		}
	}
}

func TestAppReload(t *testing.T) {
	env := setupEnv(t)
	env.bootstrap()

	// Reload apps.
	r := env.post("/admin/reload-apps", env.adminToken, nil)
	r.expectStatus(200)
	m := r.json()
	reloaded, _ := m["reloaded"].(float64)
	if reloaded < 2 {
		t.Fatalf("expected at least 2 apps reloaded, got %v", reloaded)
	}
}

func TestAppReloadRequiresAdmin(t *testing.T) {
	env := setupEnv(t)
	env.bootstrap()

	joeToken := env.createUser("Joe", "joe@test.com", "user")
	env.post("/admin/reload-apps", joeToken, nil).expectStatus(403)
}

func TestHostAllowlistEnforcement(t *testing.T) {
	env := setupEnv(t)
	env.bootstrap()

	// Create a temp app with empty allowedHosts.
	appDir := filepath.Join(t.TempDir(), "blocked-app")
	os.MkdirAll(filepath.Join(appDir, "tools"), 0755)

	// Write app.yaml with no allowed hosts.
	os.WriteFile(filepath.Join(appDir, "app.yaml"), []byte(`
name: blocked-app
version: 1.0.0
description: test app with no allowed hosts
permissions:
  allowedHosts: []
settings: []
`), 0644)

	// Write a tool that tries to make an HTTP call.
	os.WriteFile(filepath.Join(appDir, "tools", "fetch.json"), []byte(`{
		"name": "fetch",
		"description": "tries to fetch a URL",
		"params": {"url": {"type": "string", "required": true}}
	}`), 0644)
	os.WriteFile(filepath.Join(appDir, "tools", "fetch.js"), []byte(`
function handle(params) {
  var resp = http.get(params.url);
  return {status: resp.status, body: resp.body};
}
`), 0644)

	// Load this app manually.
	app, err := apps.LoadApp(appDir)
	if err != nil {
		t.Fatalf("load app: %v", err)
	}

	manifests, _ := apps.LoadToolManifests(app)
	if len(manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(manifests))
	}

	// Execute the tool — should fail because allowedHosts is empty.
	runtime := apps.NewJSRuntime(app, nil, nil, 5*time.Second)
	_, err = runtime.Execute(manifests[0].ScriptPath, map[string]any{"url": "http://evil.com/hack"})
	if err == nil {
		t.Fatal("expected error for blocked host")
	}
	if !strings.Contains(err.Error(), "allowedHosts") {
		t.Fatalf("expected allowedHosts error, got: %v", err)
	}
}

func TestMCPWithInvalidToken(t *testing.T) {
	env := setupEnv(t)

	// MCP init with bad token should fail.
	body := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "test", "version": "1.0"},
		},
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", env.serverURL+"/mcp", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer invalid-token")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should fail (either 401 from our handler or error from MCP SDK with nil server).
	if resp.StatusCode == 200 {
		// Check if the SSE response contains an error.
		data, _ := io.ReadAll(resp.Body)
		body := string(data)
		if !strings.Contains(body, "error") && !strings.Contains(body, "401") {
			t.Fatalf("expected error for invalid token, got 200 with: %s", body)
		}
	}
	// 401 or 500 are both acceptable — the key is it doesn't succeed.
}

// ---------- helper methods on testEnv ----------

func (e *testEnv) bootstrap() {
	e.t.Helper()
	r := e.post("/bootstrap", "", map[string]any{
		"workspaceName": "TestCo",
		"adminName":     "Admin",
		"adminEmail":    "admin@test.com",
	})
	r.expectStatus(201)
	m := r.json()
	admin, _ := m["admin"].(map[string]any)
	e.adminToken = admin["token"].(string)
	ws, _ := m["workspace"].(map[string]any)
	e.wsID = ws["id"].(string)
}

func (e *testEnv) createUser(name, email, role string) string {
	e.t.Helper()
	r := e.post("/workspaces/"+e.wsID+"/users", e.adminToken, map[string]any{
		"name":  name,
		"email": email,
		"role":  role,
	})
	r.expectStatus(201)
	return r.getString("token")
}

func (e *testEnv) getUserID(token string) string {
	e.t.Helper()
	r := e.get("/users/me", token)
	r.expectStatus(200)
	return r.getString("id")
}

// Ensure unused imports don't break.
var _ = fmt.Sprintf
var _ = os.Getenv
