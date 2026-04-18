package api

import (
	"net/http"
	"time"

	"github.com/NubeDev/bizzy/pkg/apps"
	"github.com/gin-gonic/gin"
)

// testToolRequest is the body for POST /api/apps/test-tool.
type testToolRequest struct {
	Script       string            `json:"script"`
	Helpers      string            `json:"helpers"`
	Params       map[string]any    `json:"params"`
	AllowedHosts []string          `json:"allowedHosts"`
	Settings     map[string]string `json:"settings"`
	Secrets      map[string]string `json:"secrets"`
	Timeout      string            `json:"timeout"`
}

// testToolResponse is returned by POST /api/apps/test-tool.
type testToolResponse struct {
	Output     any                 `json:"output"`
	Error      *string             `json:"error"`
	DurationMS float64             `json:"duration_ms"`
	HTTPLog    []apps.HTTPLogEntry `json:"http_log"`
}

// testTool executes a JS tool script in a sandboxed Goja runtime and returns
// the output along with an HTTP trace log. Stateless — nothing is persisted.
func (a *API) testTool(c *gin.Context) {
	var req testToolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}

	if req.Script == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "script is required"})
		return
	}

	// Parse timeout, default 30s, cap at 60s.
	timeout := 30 * time.Second
	if req.Timeout != "" {
		if d, err := time.ParseDuration(req.Timeout); err == nil {
			timeout = d
		}
	}
	if timeout > 60*time.Second {
		timeout = 60 * time.Second
	}

	if req.Settings == nil {
		req.Settings = make(map[string]string)
	}
	if req.Secrets == nil {
		req.Secrets = make(map[string]string)
	}
	if req.Params == nil {
		req.Params = make(map[string]any)
	}

	// Set up logging transport to capture HTTP traces.
	transport := apps.NewLoggingRoundTripper(nil)

	rt := apps.NewTestJSRuntime(req.AllowedHosts, req.Secrets, req.Settings, timeout, transport)

	start := time.Now()
	output, err := rt.ExecuteScript(req.Script, req.Helpers, req.Params)
	durationMS := time.Since(start).Seconds() * 1000

	resp := testToolResponse{
		Output:     output,
		DurationMS: durationMS,
		HTTPLog:    transport.Entries(),
	}
	if err != nil {
		errStr := err.Error()
		resp.Error = &errStr
	}

	c.JSON(http.StatusOK, resp)
}
