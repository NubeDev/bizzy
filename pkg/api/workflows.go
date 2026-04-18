package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/NubeDev/bizzy/pkg/airunner"
	"github.com/NubeDev/bizzy/pkg/auth"
	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/NubeDev/bizzy/pkg/services"
	"github.com/NubeDev/bizzy/pkg/workflow"
	"github.com/gin-gonic/gin"
)

// workflowRunRequest is the JSON body for POST /api/workflows/run.
type workflowRunRequest struct {
	WorkflowID string         `json:"workflow_id" binding:"required"` // client-generated UUID
	App        string         `json:"app"         binding:"required"`
	Workflow   string         `json:"workflow"     binding:"required"`
	Inputs     map[string]any `json:"inputs"`
}

// workflowRunResponse is the response for POST /api/workflows/run.
type workflowRunResponse struct {
	WorkflowID   string                `json:"workflow_id"`
	Status       models.WorkflowStatus `json:"status"`
	CurrentStage string                `json:"current_stage"`
}

// runWorkflow starts a new workflow run.
//
//	POST /api/workflows/run
func (a *API) runWorkflow(c *gin.Context) {
	user := auth.GetUser(c)

	var req workflowRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Inputs == nil {
		req.Inputs = make(map[string]any)
	}

	run := models.WorkflowRun{
		ID:       req.WorkflowID,
		AppName:  req.App,
		Workflow: req.Workflow,
		Inputs:   req.Inputs,
		UserID:   user.ID,
	}

	result, err := a.Workflows.Start(run)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, workflowRunResponse{
		WorkflowID:   result.ID,
		Status:       result.Status,
		CurrentStage: result.CurrentStage(),
	})
}

// getWorkflowRun returns the current state of a workflow run.
//
//	GET /api/workflows/:id
func (a *API) getWorkflowRun(c *gin.Context) {
	id := c.Param("id")

	run, ok := a.Workflows.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow run not found"})
		return
	}

	c.JSON(http.StatusOK, run)
}

// listWorkflowRuns returns workflow runs filtered by optional query params.
//
//	GET /api/workflows?app=X&status=Y
func (a *API) listWorkflowRuns(c *gin.Context) {
	user := auth.GetUser(c)
	appName := c.Query("app")
	status := models.WorkflowStatus(c.Query("status"))

	runs := a.Workflows.List(user.ID, appName, status)
	if runs == nil {
		runs = []models.WorkflowRun{}
	}

	c.JSON(http.StatusOK, gin.H{"runs": runs})
}

// approveWorkflow sends an approval or rejection to a waiting workflow.
//
//	POST /api/workflows/:id/approve
func (a *API) approveWorkflow(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Action   string `json:"action"   binding:"required"` // approve, reject, cancel
		Feedback string `json:"feedback,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := a.Workflows.Approve(id, req.Action, req.Feedback); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// cancelWorkflow cancels a running workflow.
//
//	POST /api/workflows/:id/cancel
func (a *API) cancelWorkflow(c *gin.Context) {
	id := c.Param("id")

	if err := a.Workflows.Cancel(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"workflow_id": id, "status": "cancelled"})
}

// listWorkflowDefs returns all available workflow definitions.
//
//	GET /api/workflows/definitions
func (a *API) listWorkflowDefs(c *gin.Context) {
	all := a.WorkflowStore.ListAll()

	type defView struct {
		App         string   `json:"app"`
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Depends     []string `json:"depends,omitempty"`
		Inputs      []workflow.InputDef `json:"inputs,omitempty"`
		StageCount  int      `json:"stage_count"`
	}

	var defs []defView
	for app, wfs := range all {
		for _, wf := range wfs {
			defs = append(defs, defView{
				App:         app,
				Name:        wf.Name,
				Description: wf.Description,
				Depends:     wf.Depends,
				Inputs:      wf.Inputs,
				StageCount:  len(wf.Stages),
			})
		}
	}
	if defs == nil {
		defs = []defView{}
	}

	c.JSON(http.StatusOK, defs)
}

// --- ToolCaller and PromptRunner bridges ---

// WorkflowToolCaller bridges the workflow engine to the ToolService.
type WorkflowToolCaller struct {
	ToolSvc *services.ToolService
}

func NewWorkflowToolCaller(toolSvc *services.ToolService) *WorkflowToolCaller {
	return &WorkflowToolCaller{ToolSvc: toolSvc}
}

func (tc *WorkflowToolCaller) CallTool(ctx context.Context, userID, toolName string, params map[string]any) (any, error) {
	result, err := tc.ToolSvc.CallTool(userID, toolName, params)
	if err != nil {
		return nil, fmt.Errorf("tool %s: %w", toolName, err)
	}
	return result, nil
}

// WorkflowPromptRunner bridges the workflow engine to the AgentService.
type WorkflowPromptRunner struct {
	AgentSvc *services.AgentService
}

func NewWorkflowPromptRunner(agentSvc *services.AgentService) *WorkflowPromptRunner {
	return &WorkflowPromptRunner{AgentSvc: agentSvc}
}

func (pr *WorkflowPromptRunner) RunPrompt(ctx context.Context, userID, prompt string) (string, error) {
	user, err := pr.AgentSvc.GetUser(userID)
	if err != nil {
		return "", err
	}

	provider, model := pr.AgentSvc.ResolveProvider("", "", user)

	runner, err := pr.AgentSvc.GetRunner(provider)
	if err != nil {
		return "", fmt.Errorf("provider %s: %w", provider, err)
	}

	systemPrompt := pr.AgentSvc.BuildSystemPrompt(user.ID)
	prompt = pr.AgentSvc.EnrichPrompt(user.ID, prompt)

	sessionID := models.GenerateID("ses-")
	mcpURL := pr.AgentSvc.MCPURL()

	result := runner.Run(ctx, airunner.RunConfig{
		Prompt:       prompt,
		SystemPrompt: systemPrompt,
		MCPURL:       mcpURL,
		MCPToken:     user.Token,
		AllowedTools: "mcp__nube__*",
		Model:        model,
	}, sessionID, func(ev airunner.Event) {
		// Events are discarded for workflow prompt stages.
	})

	if result.Text == "" {
		return "", fmt.Errorf("AI returned empty response")
	}
	return result.Text, nil
}
