package api

import (
	"net/http"

	"github.com/NubeDev/bizzy/pkg/auth"
	"github.com/NubeDev/bizzy/pkg/flow"
	"github.com/gin-gonic/gin"
)

// --- Flow Definition CRUD ---

func (a *API) createFlow(c *gin.Context) {
	user := auth.GetUser(c)
	var def flow.FlowDef
	if err := c.ShouldBindJSON(&def); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	def.UserID = user.ID

	// Validate before saving.
	if verr := flow.Validate(&def, a.FlowEngine.Registry()); verr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": verr.Error(), "errors": verr.Errors})
		return
	}

	if err := a.FlowEngine.Store().CreateFlow(&def); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, def)
}

func (a *API) listFlows(c *gin.Context) {
	user := auth.GetUser(c)
	defs, err := a.FlowEngine.Store().ListFlows(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, defs)
}

func (a *API) getFlow(c *gin.Context) {
	id := c.Param("id")
	def, err := a.FlowEngine.Store().GetFlow(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "flow not found"})
		return
	}
	c.JSON(http.StatusOK, def)
}

func (a *API) updateFlow(c *gin.Context) {
	id := c.Param("id")
	existing, err := a.FlowEngine.Store().GetFlow(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "flow not found"})
		return
	}

	var def flow.FlowDef
	if err := c.ShouldBindJSON(&def); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Preserve immutable fields.
	def.ID = existing.ID
	def.UserID = existing.UserID
	def.CreatedAt = existing.CreatedAt
	def.Version = existing.Version

	// Validate.
	if verr := flow.Validate(&def, a.FlowEngine.Registry()); verr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": verr.Error(), "errors": verr.Errors})
		return
	}

	if err := a.FlowEngine.Store().UpdateFlow(&def); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, def)
}

func (a *API) deleteFlow(c *gin.Context) {
	id := c.Param("id")
	if err := a.FlowEngine.Store().DeleteFlow(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (a *API) duplicateFlow(c *gin.Context) {
	id := c.Param("id")
	dup, err := a.FlowEngine.Store().DuplicateFlow(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, dup)
}

// --- Flow Execution ---

type runFlowRequest struct {
	Inputs map[string]any `json:"inputs"`
}

func (a *API) runFlow(c *gin.Context) {
	user := auth.GetUser(c)
	flowID := c.Param("id")

	var req runFlowRequest
	c.ShouldBindJSON(&req) // inputs are optional

	run, err := a.FlowEngine.StartRun(c.Request.Context(), flowID, user.ID, req.Inputs, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, run)
}

func (a *API) listFlowRuns(c *gin.Context) {
	flowID := c.Param("id")
	runs, err := a.FlowEngine.Store().ListRuns(flowID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, runs)
}

func (a *API) getFlowRun(c *gin.Context) {
	runID := c.Param("runId")
	run, err := a.FlowEngine.Store().GetRun(runID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "run not found"})
		return
	}
	c.JSON(http.StatusOK, run)
}

func (a *API) approveFlowNode(c *gin.Context) {
	runID := c.Param("runId")
	nodeID := c.Param("nodeId")

	if err := a.FlowEngine.ApproveNode(c.Request.Context(), runID, nodeID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"approved": true})
}

type rejectFlowRequest struct {
	Feedback string `json:"feedback"`
}

func (a *API) rejectFlowNode(c *gin.Context) {
	runID := c.Param("runId")
	nodeID := c.Param("nodeId")

	var req rejectFlowRequest
	c.ShouldBindJSON(&req)

	if err := a.FlowEngine.RejectNode(c.Request.Context(), runID, nodeID, req.Feedback); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rejected": true})
}

func (a *API) cancelFlowRun(c *gin.Context) {
	runID := c.Param("runId")
	if err := a.FlowEngine.CancelRun(runID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"cancelled": true})
}

// --- Node Type Catalog ---

func (a *API) listNodeTypes(c *gin.Context) {
	types := a.FlowEngine.Registry().All()

	// Group by category for easier frontend consumption.
	grouped := make(map[string][]flow.NodeTypeDef)
	for _, t := range types {
		grouped[t.Category] = append(grouped[t.Category], t)
	}

	c.JSON(http.StatusOK, gin.H{
		"types":   types,
		"grouped": grouped,
	})
}

func (a *API) getNodeType(c *gin.Context) {
	typ := c.Param("type")
	def, ok := a.FlowEngine.Registry().Get(typ)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "node type not found"})
		return
	}
	c.JSON(http.StatusOK, def)
}

// --- Validation ---

func (a *API) validateFlow(c *gin.Context) {
	var def flow.FlowDef
	if err := c.ShouldBindJSON(&def); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if verr := flow.Validate(&def, a.FlowEngine.Registry()); verr != nil {
		c.JSON(http.StatusOK, gin.H{"valid": false, "errors": verr.Errors})
		return
	}
	c.JSON(http.StatusOK, gin.H{"valid": true, "errors": []string{}})
}
