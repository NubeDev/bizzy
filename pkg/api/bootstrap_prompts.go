package api

import (
	"net/http"
	"strings"

	"github.com/NubeDev/bizzy/pkg/bootstrap"
	"github.com/gin-gonic/gin"
)

// listBootstrapPrompts returns all built-in reference prompts.
// These are available without installing the bizzy-dev app.
//
//	GET /api/bootstrap/prompts
func (a *API) listBootstrapPrompts(c *gin.Context) {
	c.JSON(http.StatusOK, bootstrap.List())
}

// getBootstrapPrompt returns a single bootstrap prompt by name, with optional
// argument substitution via query parameters.
//
//	GET /api/bootstrap/prompts/:name
func (a *API) getBootstrapPrompt(c *gin.Context) {
	name := c.Param("name")

	p, err := bootstrap.Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Apply argument substitution from query params (e.g. ?description=...).
	body := p.Body
	for _, arg := range p.Arguments {
		if val := c.Query(arg.Name); val != "" {
			body = strings.ReplaceAll(body, "{{"+arg.Name+"}}", val)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"name":        p.Name,
		"description": p.Description,
		"arguments":   p.Arguments,
		"body":        body,
	})
}
