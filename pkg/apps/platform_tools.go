package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gorm.io/gorm"
)

// registerPlatformTools adds Go-native platform.* tools to the MCP server.
// These give the AI read-only access to the user's own platform data —
// sessions, installs, app store, and usage stats — via direct DB queries.
func registerPlatformTools(srv *mcp.Server, userID string, db *gorm.DB) {
	registerListSessions(srv, userID, db)
	registerGetSession(srv, userID, db)
	registerListInstalls(srv, userID, db)
	registerSearchApps(srv, db)
	registerUsageStats(srv, userID, db)
}

// --- platform.list_sessions ---

func registerListSessions(srv *mcp.Server, userID string, db *gorm.DB) {
	schema := jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"limit":    {Type: "number", Description: "Max results to return (default 20, max 100)"},
			"provider": {Type: "string", Description: "Filter by provider (claude, ollama, openai, etc.)"},
			"status":   {Type: "string", Description: "Filter by status (ok, error, running)"},
		},
	}

	srv.AddTool(&mcp.Tool{
		Name:        "platform.list_sessions",
		Description: "List your recent AI sessions with provider, model, cost, and duration",
		InputSchema: &schema,
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit := intParam(req, "limit", 20)
		if limit > 100 {
			limit = 100
		}

		q := db.Where("user_id = ?", userID).Order("created_at DESC").Limit(limit)
		if v := strParam(req, "provider"); v != "" {
			q = q.Where("provider = ?", v)
		}
		if v := strParam(req, "status"); v != "" {
			q = q.Where("status = ?", v)
		}

		var sessions []models.Session
		if err := q.Find(&sessions).Error; err != nil {
			return toolError("query failed: " + err.Error()), nil
		}

		type row struct {
			ID            string    `json:"id"`
			Provider      string    `json:"provider"`
			Model         string    `json:"model,omitempty"`
			PromptPreview string    `json:"prompt_preview"`
			Status        string    `json:"status"`
			DurationMS    int       `json:"duration_ms"`
			CostUSD       float64   `json:"cost_usd"`
			ToolCalls     int       `json:"tool_calls"`
			CreatedAt     time.Time `json:"created_at"`
		}

		rows := make([]row, len(sessions))
		for i, s := range sessions {
			preview := s.Prompt
			if len(preview) > 120 {
				preview = preview[:120] + "…"
			}
			rows[i] = row{
				ID:            s.ID,
				Provider:      s.Provider,
				Model:         s.Model,
				PromptPreview: preview,
				Status:        s.Status,
				DurationMS:    s.DurationMS,
				CostUSD:       s.CostUSD,
				ToolCalls:     s.ToolCalls,
				CreatedAt:     s.CreatedAt,
			}
		}
		return toolJSON(rows)
	})
}

// --- platform.get_session ---

func registerGetSession(srv *mcp.Server, userID string, db *gorm.DB) {
	schema := jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"session_id": {Type: "string", Description: "The session ID to retrieve"},
		},
		Required: []string{"session_id"},
	}

	srv.AddTool(&mcp.Tool{
		Name:        "platform.get_session",
		Description: "Get full details of a specific session including the AI response and tool call log",
		InputSchema: &schema,
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := strParam(req, "session_id")
		if id == "" {
			return toolError("session_id is required"), nil
		}

		var session models.Session
		if err := db.Where("id = ? AND user_id = ?", id, userID).First(&session).Error; err != nil {
			return toolError("session not found"), nil
		}

		return toolJSON(session)
	})
}

// --- platform.list_installs ---

func registerListInstalls(srv *mcp.Server, userID string, db *gorm.DB) {
	schema := jsonschema.Schema{Type: "object"}

	srv.AddTool(&mcp.Tool{
		Name:        "platform.list_installs",
		Description: "List your installed apps and their status",
		InputSchema: &schema,
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var installs []models.AppInstall
		if err := db.Where("user_id = ?", userID).Order("created_at DESC").Find(&installs).Error; err != nil {
			return toolError("query failed: " + err.Error()), nil
		}

		type row struct {
			AppName   string    `json:"app_name"`
			Enabled   bool      `json:"enabled"`
			Version   string    `json:"version"`
			Stale     bool      `json:"stale"`
			CreatedAt time.Time `json:"installed_at"`
		}

		rows := make([]row, len(installs))
		for i, inst := range installs {
			rows[i] = row{
				AppName:   inst.AppName,
				Enabled:   inst.Enabled,
				Version:   inst.AppVersion,
				Stale:     inst.Stale,
				CreatedAt: inst.CreatedAt,
			}
		}
		return toolJSON(rows)
	})
}

// --- platform.search_apps ---

func registerSearchApps(srv *mcp.Server, db *gorm.DB) {
	schema := jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"query":    {Type: "string", Description: "Search by name or description"},
			"category": {Type: "string", Description: "Filter by category (iot-devices, analytics, devops, marketing, etc.)"},
			"limit":    {Type: "number", Description: "Max results (default 20, max 50)"},
		},
	}

	srv.AddTool(&mcp.Tool{
		Name:        "platform.search_apps",
		Description: "Search the app store for apps by name, description, or category",
		InputSchema: &schema,
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit := intParam(req, "limit", 20)
		if limit > 50 {
			limit = 50
		}

		q := db.Model(&models.StoreApp{}).Where("visibility = ?", models.VisibilityPublic).Order("install_count DESC").Limit(limit)

		if v := strParam(req, "category"); v != "" {
			q = q.Where("category = ?", v)
		}
		if v := strParam(req, "query"); v != "" {
			like := "%" + v + "%"
			q = q.Where("name LIKE ? OR display_name LIKE ? OR description LIKE ?", like, like, like)
		}

		var apps []models.StoreApp
		if err := q.Find(&apps).Error; err != nil {
			return toolError("query failed: " + err.Error()), nil
		}

		type row struct {
			Name         string  `json:"name"`
			DisplayName  string  `json:"display_name"`
			Description  string  `json:"description"`
			Category     string  `json:"category"`
			AvgRating    float64 `json:"avg_rating"`
			InstallCount int     `json:"install_count"`
		}

		rows := make([]row, len(apps))
		for i, a := range apps {
			rows[i] = row{
				Name:         a.Name,
				DisplayName:  a.DisplayName,
				Description:  a.Description,
				Category:     a.Category,
				AvgRating:    a.AvgRating,
				InstallCount: a.InstallCount,
			}
		}
		return toolJSON(rows)
	})
}

// --- platform.usage_stats ---

func registerUsageStats(srv *mcp.Server, userID string, db *gorm.DB) {
	schema := jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"days": {Type: "number", Description: "Number of days to look back (default 7, max 90)"},
		},
	}

	srv.AddTool(&mcp.Tool{
		Name:        "platform.usage_stats",
		Description: "Get your usage summary: sessions, tokens, and cost over a time range",
		InputSchema: &schema,
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		days := intParam(req, "days", 7)
		if days > 90 {
			days = 90
		}
		since := time.Now().AddDate(0, 0, -days)

		// Overall stats.
		type totals struct {
			Sessions int     `json:"total_sessions"`
			Tokens   int     `json:"total_tokens"`
			Cost     float64 `json:"total_cost_usd"`
		}
		var t totals
		db.Model(&models.Session{}).
			Where("user_id = ? AND created_at > ?", userID, since).
			Select("COUNT(*) as sessions, COALESCE(SUM(input_tokens + output_tokens), 0) as tokens, COALESCE(SUM(cost_usd), 0) as cost").
			Scan(&t)

		// Per-provider breakdown.
		type providerRow struct {
			Provider string  `json:"provider"`
			Sessions int     `json:"sessions"`
			Tokens   int     `json:"tokens"`
			Cost     float64 `json:"cost_usd"`
		}
		var byProvider []providerRow
		db.Model(&models.Session{}).
			Where("user_id = ? AND created_at > ?", userID, since).
			Select("provider, COUNT(*) as sessions, COALESCE(SUM(input_tokens + output_tokens), 0) as tokens, COALESCE(SUM(cost_usd), 0) as cost").
			Group("provider").
			Scan(&byProvider)

		result := struct {
			Days       int           `json:"days"`
			totals
			ByProvider []providerRow `json:"by_provider"`
		}{
			Days:       days,
			totals:     t,
			ByProvider: byProvider,
		}
		return toolJSON(result)
	})
}

// --- helpers ---

// parseArgs unmarshals the raw JSON arguments from a CallToolRequest into a map.
func parseArgs(req *mcp.CallToolRequest) map[string]any {
	if req.Params.Arguments == nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(req.Params.Arguments, &m); err != nil {
		return nil
	}
	return m
}

func strParam(req *mcp.CallToolRequest, key string) string {
	args := parseArgs(req)
	if args == nil {
		return ""
	}
	v, ok := args[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return fmt.Sprintf("%v", v)
	}
	return s
}

func intParam(req *mcp.CallToolRequest, key string, def int) int {
	args := parseArgs(req)
	if args == nil {
		return def
	}
	v, ok := args[key]
	if !ok {
		return def
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return def
	}
}

func toolJSON(v any) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return toolError("marshal error: " + err.Error()), nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
	}, nil
}

func toolError(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: "error: " + msg}},
		IsError: true,
	}
}
