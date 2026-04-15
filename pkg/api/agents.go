package api

import (
	"net/http"

	"github.com/NubeDev/bizzy/pkg/auth"
	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/gin-gonic/gin"
)

// agentInfo describes an agent derived from an installed app.
type agentInfo struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Version     string      `json:"version"`
	Tools       []agentTool `json:"tools"`
}

// agentTool describes a single tool within an agent.
type agentTool struct {
	Name        string `json:"name"`
	Type        string `json:"type"`           // "openapi" or "js"
	Mode        string `json:"mode,omitempty"` // "" or "qa"
	Description string `json:"description"`
}

// listAgents returns all agents available to the authenticated user.
// Each installed+enabled app becomes an agent.
// The registry contains both system apps and store apps, so no fallback is needed.
func (a *API) listAgents(c *gin.Context) {
	user := auth.GetUser(c)
	installs := a.AppInstalls.FindFunc(func(ai models.AppInstall) bool {
		return ai.UserID == user.ID && ai.Enabled
	})

	agents := make([]agentInfo, 0, len(installs))
	for _, inst := range installs {
		app, ok := a.AppRegistry.Get(inst.AppName)
		if !ok {
			continue
		}
		agent := agentInfo{
			Name:        app.Name,
			Description: app.Description,
			Version:     app.Version,
			Tools:       make([]agentTool, 0),
		}
		if app.HasOpenAPI {
			agent.Tools = append(agent.Tools, agentTool{
				Name:        inst.AppName + ".*",
				Type:        "openapi",
				Description: "OpenAPI-generated tools from " + inst.AppName,
			})
		}
		for _, m := range a.AppRegistry.GetTools(inst.AppName) {
			agent.Tools = append(agent.Tools, agentTool{
				Name:        inst.AppName + "." + m.Name,
				Type:        "js",
				Mode:        m.Mode,
				Description: m.Description,
			})
		}
		agents = append(agents, agent)
	}

	c.JSON(http.StatusOK, agents)
}
