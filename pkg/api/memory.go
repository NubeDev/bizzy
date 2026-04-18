package api

import (
	"net/http"

	"github.com/NubeDev/bizzy/pkg/auth"
	"github.com/gin-gonic/gin"
)

// --- Memory API ---

type memoryBody struct {
	Content string `json:"content"`
}

// getServerMemory returns the server-wide memory.
//
//	GET /api/memory/server
func (a *API) getServerMemory(c *gin.Context) {
	c.JSON(http.StatusOK, memoryBody{Content: a.Memory.GetServer()})
}

// setServerMemory replaces the server-wide memory (admin only).
//
//	PUT /api/memory/server
func (a *API) setServerMemory(c *gin.Context) {
	var body memoryBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := a.Memory.SetServer(body.Content); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, memoryBody{Content: body.Content})
}

// getMyMemory returns the authenticated user's memory.
//
//	GET /api/memory/me
func (a *API) getMyMemory(c *gin.Context) {
	user := auth.GetUser(c)
	c.JSON(http.StatusOK, memoryBody{Content: a.Memory.GetUser(user.ID)})
}

// setMyMemory replaces the authenticated user's memory.
//
//	PUT /api/memory/me
func (a *API) setMyMemory(c *gin.Context) {
	user := auth.GetUser(c)
	var body memoryBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := a.Memory.SetUser(user.ID, body.Content); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, memoryBody{Content: body.Content})
}

// appendMyMemory appends a line to the authenticated user's memory.
//
//	POST /api/memory/me
func (a *API) appendMyMemory(c *gin.Context) {
	user := auth.GetUser(c)
	var body memoryBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content is required"})
		return
	}
	if err := a.Memory.AppendUser(user.ID, body.Content); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, memoryBody{Content: a.Memory.GetUser(user.ID)})
}
