// Package services contains reusable application logic decoupled from HTTP handlers.
// These services can be consumed by REST handlers, CLI commands, workflow engines,
// or any other entry point without depending on gin or HTTP concerns.
package services

import (
	"fmt"
	"os"
	"time"

	"github.com/NubeDev/bizzy/pkg/airunner"
	"github.com/NubeDev/bizzy/pkg/apps"
	"github.com/NubeDev/bizzy/pkg/memory"
	"github.com/NubeDev/bizzy/pkg/models"
	"gorm.io/gorm"
)

// AgentService handles prompt enrichment, provider resolution, and session management.
// It extracts business logic that was previously duplicated across WebSocket, REST,
// and job handlers into a single reusable layer.
type AgentService struct {
	DB          *gorm.DB
	Memory      *memory.Store
	MCPFactory  *apps.MCPFactory
	Runners     *airunner.Registry
	Jobs        *airunner.JobStore
	AppRegistry *apps.Registry
}

// EnrichPrompt prepends server/user memory and installed app context to a prompt.
// This gives the AI awareness of available tools without the user picking an agent.
// For providers that support it, use BuildSystemPrompt + the raw user prompt instead.
func (s *AgentService) EnrichPrompt(userID, prompt string) string {
	sys := s.BuildSystemPrompt(userID)
	if sys != "" {
		prompt = sys + prompt
	}
	return prompt
}

// BuildSystemPrompt returns the system-level context (memory + app descriptions)
// as a separate string. Providers that support system messages (Ollama, OpenAI,
// Anthropic) should pass this via their system/role field instead of prepending
// it to the user prompt. This improves response quality because the model treats
// system instructions differently from user input.
func (s *AgentService) BuildSystemPrompt(userID string) string {
	var parts []string
	if s.Memory != nil {
		if prefix := s.Memory.BuildPromptPrefix(userID); prefix != "" {
			parts = append(parts, prefix)
		}
	}
	if s.MCPFactory != nil {
		installs := s.UserEnabledInstalls(userID)
		if appCtx := s.MCPFactory.BuildAppContext(installs); appCtx != "" {
			parts = append(parts, appCtx)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	result := ""
	for _, p := range parts {
		result += p
	}
	return result
}

// ResolveProvider returns the provider and model to use, applying user defaults
// when the request doesn't specify them.
func (s *AgentService) ResolveProvider(reqProvider, reqModel string, user models.User) (airunner.Provider, string) {
	provider := airunner.Provider(reqProvider)
	model := reqModel
	if provider == "" {
		if user.Preferences != nil && user.Preferences.DefaultProvider != "" {
			provider = airunner.Provider(user.Preferences.DefaultProvider)
		} else {
			provider = airunner.ProviderClaude
		}
	}
	if model == "" && user.Preferences != nil {
		model = user.Preferences.DefaultModel
	}
	return provider, model
}

// GetRunner returns a runner for the given provider, checking availability.
func (s *AgentService) GetRunner(provider airunner.Provider) (airunner.Runner, error) {
	runner, err := s.Runners.Get(provider)
	if err != nil {
		return nil, err
	}
	if !runner.Available() {
		return nil, fmt.Errorf("%s is not available", provider)
	}
	return runner, nil
}

// MCPURL returns the MCP server URL based on the NUBE_ADDR environment variable.
func (s *AgentService) MCPURL() string {
	return "http://localhost" + os.Getenv("NUBE_ADDR") + "/mcp"
}

// SessionParams contains the fields needed to persist a session.
type SessionParams struct {
	ID        string
	Agent     string
	Prompt    string // original prompt (before enrichment)
	UserID    string
	JobStatus string
	Result    *airunner.RunResult
}

// SaveSession persists a completed AI session.
func (s *AgentService) SaveSession(p SessionParams) error {
	session := models.Session{
		ID:              p.ID,
		Provider:        p.Result.Provider,
		Model:           p.Result.Model,
		ClaudeSessionID: p.Result.ClaudeSessionID,
		Agent:           p.Agent,
		Prompt:          p.Prompt,
		Result:          p.Result.Text,
		Status:          p.JobStatus,
		DurationMS:      p.Result.DurationMS,
		CostUSD:         p.Result.CostUSD,
		InputTokens:     p.Result.InputTokens,
		OutputTokens:    p.Result.OutputTokens,
		ToolCalls:       p.Result.ToolCalls,
		ToolCallLog:     convertToolCallLog(p.Result.ToolCallLog),
		UserID:          p.UserID,
		CreatedAt:       time.Now().UTC(),
	}
	return s.DB.Create(&session).Error
}

// ListSessions returns all sessions for a user.
func (s *AgentService) ListSessions(userID string) []models.Session {
	var sessions []models.Session
	s.DB.Where("user_id = ?", userID).Order("created_at DESC").Find(&sessions)
	return sessions
}

// GetSession returns a single session, validating ownership.
func (s *AgentService) GetSession(sessionID, userID string) (models.Session, error) {
	var ses models.Session
	result := s.DB.First(&ses, "id = ? AND user_id = ?", sessionID, userID)
	if result.Error != nil {
		return models.Session{}, fmt.Errorf("session not found")
	}
	return ses, nil
}

// GetUser returns a user by ID.
func (s *AgentService) GetUser(userID string) (models.User, error) {
	var user models.User
	if err := s.DB.First(&user, "id = ?", userID).Error; err != nil {
		return models.User{}, fmt.Errorf("user not found: %s", userID)
	}
	return user, nil
}

// UserEnabledInstalls returns the user's enabled app installs.
func (s *AgentService) UserEnabledInstalls(userID string) []models.AppInstall {
	var installs []models.AppInstall
	s.DB.Where("user_id = ? AND enabled = ?", userID, true).Find(&installs)
	return installs
}

// AgentInfo describes an agent derived from an installed app.
type AgentInfo struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Version     string      `json:"version"`
	Tools       []AgentTool `json:"tools"`
}

// AgentTool describes a single tool within an agent.
type AgentTool struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Mode        string `json:"mode,omitempty"`
	Description string `json:"description"`
}

// ListAgents returns all agents (installed+enabled apps) for a user.
func (s *AgentService) ListAgents(userID string) []AgentInfo {
	installs := s.UserEnabledInstalls(userID)
	agents := make([]AgentInfo, 0, len(installs))

	for _, inst := range installs {
		app, ok := s.AppRegistry.Get(inst.AppName)
		if !ok {
			continue
		}
		agent := AgentInfo{
			Name:        app.Name,
			Description: app.Description,
			Version:     app.Version,
			Tools:       make([]AgentTool, 0),
		}
		if app.HasOpenAPI {
			agent.Tools = append(agent.Tools, AgentTool{
				Name:        inst.AppName + ".*",
				Type:        "openapi",
				Description: "OpenAPI-generated tools from " + inst.AppName,
			})
		}
		for _, m := range s.AppRegistry.GetTools(inst.AppName) {
			agent.Tools = append(agent.Tools, AgentTool{
				Name:        inst.AppName + "." + m.Name,
				Type:        "js",
				Mode:        m.Mode,
				Description: m.Description,
			})
		}
		agents = append(agents, agent)
	}
	return agents
}

// convertToolCallLog converts airunner entries to model entries.
func convertToolCallLog(entries []airunner.ToolCallEntry) []models.ToolCallEntry {
	if len(entries) == 0 {
		return nil
	}
	out := make([]models.ToolCallEntry, len(entries))
	for i, e := range entries {
		out[i] = models.ToolCallEntry{
			Name:        e.Name,
			DurationMS:  e.DurationMS,
			Status:      e.Status,
			Error:       e.Error,
			InputBytes:  e.InputBytes,
			OutputBytes: e.OutputBytes,
		}
	}
	return out
}
