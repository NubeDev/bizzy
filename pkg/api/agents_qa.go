package api

import (
	"net/http"
	"time"

	"github.com/NubeDev/bizzy/pkg/apps"
	"github.com/NubeDev/bizzy/pkg/auth"
	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/gin-gonic/gin"
)

// callTool executes a JS tool directly via REST.
//
//	POST /api/agents/tools/:name
//	Body: {"product": "Rubix", "_submit": true, ...}
func (a *API) callTool(c *gin.Context) {
	user := auth.GetUser(c)
	toolName := c.Param("name")

	var params map[string]any
	if err := c.ShouldBindJSON(&params); err != nil {
		params = make(map[string]any)
	}

	runtime, manifest, err := a.resolveJSTool(user, toolName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	result, err := runtime.Execute(manifest.ScriptPath, params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// resolveJSTool finds a JS tool by namespaced name and returns a ready-to-use runtime.
func (a *API) resolveJSTool(user models.User, toolName string) (*apps.JSRuntime, *apps.ToolManifest, error) {
	installs := a.AppInstalls.FindFunc(func(ai models.AppInstall) bool {
		return ai.UserID == user.ID && ai.Enabled
	})

	for _, inst := range installs {
		app, ok := a.AppRegistry.Get(inst.AppName)
		if !ok {
			continue
		}

		for _, manifest := range a.AppRegistry.GetTools(inst.AppName) {
			fullName := inst.AppName + "." + manifest.Name
			if fullName != toolName {
				continue
			}

			secrets := make(map[string]string)
			config := make(map[string]string)
			for _, def := range app.Settings {
				val := inst.GetSetting(def.Key)
				if val == "" && def.Default != "" {
					val = def.Default
				}
				if def.Type == "secret" {
					secrets[def.Key] = val
				} else {
					config[def.Key] = val
				}
			}

			timeout := 5 * time.Second
			if app.Timeout != "" {
				if d, err := time.ParseDuration(app.Timeout); err == nil {
					timeout = d
				}
			}

			m := manifest // copy for pointer
			return apps.NewJSRuntime(app, secrets, config, timeout), &m, nil
		}
	}

	return nil, nil, &toolNotFoundError{toolName}
}

type toolNotFoundError struct{ name string }

func (e *toolNotFoundError) Error() string { return "tool not found: " + e.name }
