package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/NubeDev/bizzy/pkg/airunner"
	"github.com/NubeDev/bizzy/pkg/auth"
	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/NubeDev/bizzy/pkg/services"
	"github.com/gin-gonic/gin"
)

// jobSubmitRequest is the JSON body for POST /api/agents/jobs.
type jobSubmitRequest struct {
	Prompt         string               `json:"prompt"   binding:"required"`
	Agent          string               `json:"agent,omitempty"`
	Provider       string               `json:"provider,omitempty"`        // "claude" (default), "ollama", "openai", etc.
	Model          string               `json:"model,omitempty"`
	ThinkingBudget string               `json:"thinking_budget,omitempty"` // "low", "medium", "high", or token count
	Attachments    []airunner.Attachment `json:"attachments,omitempty"`     // images/files attached to the prompt
}

// jobSubmitResponse is returned immediately after submitting a job.
type jobSubmitResponse struct {
	JobID  string             `json:"job_id"`
	Status airunner.JobStatus `json:"status"`
}

// submitJob creates a new async AI job.
//
//	POST /api/agents/jobs
func (a *API) submitJob(c *gin.Context) {
	user := auth.GetUser(c)

	var req jobSubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	provider, model := a.AgentSvc.ResolveProvider(req.Provider, req.Model, user)

	runner, err := a.AgentSvc.GetRunner(provider)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Agent != "" {
		if _, exists := a.AppRegistry.Get(req.Agent); !exists {
			c.JSON(http.StatusBadRequest, gin.H{"error": "agent not found: " + req.Agent})
			return
		}
	}

	systemPrompt := a.AgentSvc.BuildSystemPrompt(user.ID)
	prompt := a.AgentSvc.EnrichPrompt(user.ID, req.Prompt)

	jobID := models.GenerateID("job-")
	sessionID := models.GenerateID("ses-")
	mcpURL := a.AgentSvc.MCPURL()

	job := a.Jobs.Submit(
		jobID,
		string(provider),
		model,
		runner,
		airunner.RunConfig{
			Prompt:         prompt,
			SystemPrompt:   systemPrompt,
			MCPURL:         mcpURL,
			MCPToken:       user.Token,
			AllowedTools:   "mcp__nube__*",
			Model:          model,
			ThinkingBudget: req.ThinkingBudget,
			Attachments:    req.Attachments,
		},
		sessionID,
		user.ID,
	)

	// Persist session asynchronously when the job finishes.
	go func() {
		for {
			time.Sleep(500 * time.Millisecond)
			result := job.Result()
			if result == nil {
				continue
			}
			a.AgentSvc.SaveSession(services.SessionParams{
				ID:        sessionID,
				Agent:     req.Agent,
				Prompt:    req.Prompt,
				UserID:    user.ID,
				JobStatus: string(job.Status),
				Result:    result,
			})
			return
		}
	}()

	c.JSON(http.StatusAccepted, jobSubmitResponse{
		JobID:  jobID,
		Status: job.Status,
	})
}

// pollJob returns the current state and events for a job.
//
//	GET /api/agents/jobs/:id?after=<index>
func (a *API) pollJob(c *gin.Context) {
	jobID := c.Param("id")

	job, ok := a.Jobs.Get(jobID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	after := -1
	if v := c.Query("after"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			after = n
		}
	}

	view := airunner.JobView{
		ID:       job.ID,
		Status:   job.Status,
		Provider: job.Provider,
		Model:    job.Model,
		Events:   job.Events(after),
	}
	if result := job.Result(); result != nil {
		view.Result = result.Text
	}

	c.JSON(http.StatusOK, view)
}

// listJobs returns all jobs for the current user.
//
//	GET /api/agents/jobs
func (a *API) listJobs(c *gin.Context) {
	user := auth.GetUser(c)
	jobs := a.Jobs.List(user.ID)
	c.JSON(http.StatusOK, jobs)
}

// cancelJob cancels a running job.
//
//	DELETE /api/agents/jobs/:id
func (a *API) cancelJob(c *gin.Context) {
	jobID := c.Param("id")

	if err := a.Jobs.Cancel(jobID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "cancelled"})
}
