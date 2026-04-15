package api

import (
	"net/http"

	"github.com/NubeDev/bizzy/pkg/auth"
	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/gin-gonic/gin"
)

// listMyTools returns all MCP tools available to the current user (based on installed apps).
func (a *API) listMyTools(c *gin.Context) {
	user := auth.GetUser(c)
	installs := a.AppInstalls.FindFunc(func(ai models.AppInstall) bool {
		return ai.UserID == user.ID && ai.Enabled
	})

	type toolInfo struct {
		Name    string `json:"name"`
		AppName string `json:"appName"`
		Type    string `json:"type"` // openapi, js
		Desc    string `json:"description"`
	}

	tools := make([]toolInfo, 0)

	for _, inst := range installs {
		app, ok := a.AppRegistry.Get(inst.AppName)
		if !ok {
			continue
		}
		// OpenAPI tools.
		if app.HasOpenAPI {
			// We can't easily list individual OpenAPI tool names without parsing the spec,
			// but the MCPFactory has that info. For now, note the app has OpenAPI tools.
			tools = append(tools, toolInfo{
				Name:    inst.AppName + ".*",
				AppName: inst.AppName,
				Type:    "openapi",
				Desc:    "OpenAPI-generated tools from " + inst.AppName,
			})
		}
		// JS tools.
		for _, m := range a.AppRegistry.GetTools(inst.AppName) {
			tools = append(tools, toolInfo{
				Name:    inst.AppName + "." + m.Name,
				AppName: inst.AppName,
				Type:    "js",
				Desc:    m.Description,
			})
		}
	}

	c.JSON(http.StatusOK, tools)
}

// listMyPrompts returns all MCP prompts available to the current user.
func (a *API) listMyPrompts(c *gin.Context) {
	user := auth.GetUser(c)
	installs := a.AppInstalls.FindFunc(func(ai models.AppInstall) bool {
		return ai.UserID == user.ID && ai.Enabled
	})

	type promptInfo struct {
		Name    string `json:"name"`
		AppName string `json:"appName"`
		Desc    string `json:"description"`
		Args    []struct {
			Name     string `json:"name"`
			Required bool   `json:"required"`
		} `json:"arguments,omitempty"`
	}

	prompts := make([]promptInfo, 0)
	for _, inst := range installs {
		for _, p := range a.AppRegistry.GetPrompts(inst.AppName) {
			pi := promptInfo{
				Name:    inst.AppName + "." + p.Name,
				AppName: inst.AppName,
				Desc:    p.Description,
			}
			for _, a := range p.Arguments {
				pi.Args = append(pi.Args, struct {
					Name     string `json:"name"`
					Required bool   `json:"required"`
				}{a.Name, a.Required})
			}
			prompts = append(prompts, pi)
		}
	}
	c.JSON(http.StatusOK, prompts)
}

// getPrompt renders a prompt with the given arguments.
func (a *API) getPrompt(c *gin.Context) {
	user := auth.GetUser(c)
	promptName := c.Param("name")

	// Find the prompt across installed apps.
	installs := a.AppInstalls.FindFunc(func(ai models.AppInstall) bool {
		return ai.UserID == user.ID && ai.Enabled
	})

	for _, inst := range installs {
		for _, p := range a.AppRegistry.GetPrompts(inst.AppName) {
			fullName := inst.AppName + "." + p.Name
			if fullName == promptName {
				// Parse arguments from query params.
				body := p.Body
				for _, arg := range p.Arguments {
					val := c.Query(arg.Name)
					if val != "" {
						body = replaceAll(body, "{{"+arg.Name+"}}", val)
					}
				}
				c.JSON(http.StatusOK, gin.H{
					"name":        fullName,
					"description": p.Description,
					"rendered":    body,
				})
				return
			}
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "prompt not found — is the app installed?"})
}

func replaceAll(s, old, new string) string {
	for {
		i := indexOf(s, old)
		if i < 0 {
			return s
		}
		s = s[:i] + new + s[i+len(old):]
	}
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
