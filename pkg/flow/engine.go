package flow

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/NubeDev/bizzy/pkg/airunner"
)

// ToolCaller executes a named tool and returns the result.
type ToolCaller interface {
	CallTool(ctx context.Context, userID, toolName string, params map[string]any) (any, error)
}

// PromptRunner sends a prompt to an AI provider and returns the text response.
type PromptRunner interface {
	RunPrompt(ctx context.Context, userID, prompt string) (string, error)
}

// Engine executes flow DAGs. It follows a single-writer model: one main goroutine
// owns all run state while worker goroutines execute nodes and report results
// back via a channel.
type Engine struct {
	store    *Store
	registry *NodeTypeRegistry
	events   eventEmitter
	tools    ToolCaller
	prompts  PromptRunner
	agents   *airunner.Registry
	slack    SlackSender
	email    EmailSender

	// Resource limits.
	MaxParallelNodes int
	MaxRunsPerUser   int

	// Active run tracking.
	mu      sync.RWMutex
	cancels map[string]context.CancelFunc // runID -> cancel
}

// NewEngine creates a flow execution engine.
func NewEngine(store *Store, registry *NodeTypeRegistry) *Engine {
	return &Engine{
		store:            store,
		registry:         registry,
		MaxParallelNodes: 50,
		MaxRunsPerUser:   10,
		cancels:          make(map[string]context.CancelFunc),
	}
}

// SetBus attaches an event publisher.
func (e *Engine) SetBus(bus EventPublisher) {
	e.events = eventEmitter{bus: bus}
}

// SetTools sets the tool caller for tool: nodes.
func (e *Engine) SetTools(tools ToolCaller) {
	e.tools = tools
}

// SetPrompts sets the prompt runner for ai-prompt nodes.
func (e *Engine) SetPrompts(prompts PromptRunner) {
	e.prompts = prompts
}

// SetAgents sets the AI runner registry for ai-runner nodes.
func (e *Engine) SetAgents(agents *airunner.Registry) {
	e.agents = agents
}

// SetSlack sets the Slack sender for slack-send nodes.
func (e *Engine) SetSlack(slack SlackSender) {
	e.slack = slack
}

// SetEmail sets the email sender for email-send nodes.
func (e *Engine) SetEmail(email EmailSender) {
	e.email = email
}

// Registry returns the node type registry.
func (e *Engine) Registry() *NodeTypeRegistry {
	return e.registry
}

// Store returns the flow store.
func (e *Engine) Store() *Store {
	return e.store
}

// --- Flow execution ---

// StartRun creates and begins executing a flow run.
func (e *Engine) StartRun(ctx context.Context, flowID, userID string, inputs map[string]any, replyTo *ReplyInfo) (*FlowRun, error) {
	def, err := e.store.GetFlow(flowID)
	if err != nil {
		return nil, fmt.Errorf("flow %s not found: %w", flowID, err)
	}

	// Check concurrent run limit.
	if e.MaxRunsPerUser > 0 {
		count, _ := e.store.CountActiveRuns(userID)
		if int(count) >= e.MaxRunsPerUser {
			return nil, fmt.Errorf("concurrent run limit reached (%d)", e.MaxRunsPerUser)
		}
	}

	// Validate the flow.
	if verr := Validate(def, e.registry); verr != nil {
		return nil, verr
	}

	// Initialize node states.
	nodeStates := make(map[string]NodeState, len(def.Nodes))
	for _, n := range def.Nodes {
		nodeStates[n.ID] = NodeState{Status: NodePending}
	}

	run := &FlowRun{
		FlowID:      def.ID,
		FlowVersion: def.Version,
		FlowName:    def.Name,
		Status:      FlowRunRunning,
		Inputs:      inputs,
		NodeStates:  nodeStates,
		Variables:   make(map[string]any),
		UserID:      userID,
		ReplyTo:     replyTo,
	}

	if err := e.store.CreateRun(run); err != nil {
		return nil, fmt.Errorf("create run: %w", err)
	}

	e.events.flowStarted(run)

	// Execute asynchronously.
	go e.execute(ctx, run, def)

	return run, nil
}

// execute is the main execution loop (single-writer).
func (e *Engine) execute(ctx context.Context, run *FlowRun, def *FlowDef) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Register cancel for external cancellation.
	e.mu.Lock()
	e.cancels[run.ID] = cancel
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		delete(e.cancels, run.ID)
		e.mu.Unlock()
	}()

	// Apply flow-level timeout.
	if timeoutSec := getIntOrDefault(def.Settings, "timeout", 3600); timeoutSec > 0 {
		var timeoutCancel context.CancelFunc
		ctx, timeoutCancel = context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
		defer timeoutCancel()
	}

	// Initialize input delivery map.
	deliveredInputs := make(map[string]map[string]any) // nodeID -> portHandle -> value

	results := make(chan nodeResult, len(def.Nodes))
	inflight := 0

	// Seed the trigger node.
	triggerID := e.findTriggerNode(def)
	if triggerID == "" {
		run.Status = FlowRunFailed
		run.Error = "no trigger node found"
		e.persistRun(run)
		return
	}
	e.markReady(run, triggerID)
	inflight += e.fireReadyNodes(ctx, run, def, results, deliveredInputs)

	for inflight > 0 {
		select {
		case <-ctx.Done():
			run.Status = FlowRunCancelled
			e.persistRun(run)
			e.events.flowCancelled(run)
			return

		case res := <-results:
			inflight--

			if res.Error != nil {
				cont := e.handleNodeError(run, def, res, deliveredInputs)
				if !cont {
					cancel()
					run.Status = FlowRunFailed
					e.persistRun(run)
					e.events.flowFailed(run, run.Error)
					return
				}
			} else {
				state := run.NodeStates[res.NodeID]
				state.Status = NodeCompleted
				state.Output = res.Output
				state.StartedAt = &res.StartedAt
				state.FinishedAt = &res.FinishedAt
				state.DurationMS = int(res.FinishedAt.Sub(res.StartedAt).Milliseconds())
				run.NodeStates[res.NodeID] = state

				nodeDef := def.GetNode(res.NodeID)
				e.events.nodeCompleted(run, nodeDef, state.DurationMS)

				// Handle output node — store as flow output.
				if nodeDef.Type == "output" {
					if po, ok := res.Output.(PortOutput); ok {
						run.Output = wrapOutput(po.Value)
					} else {
						run.Output = wrapOutput(res.Output)
					}
				}

				e.propagateOutputs(run, def, res.NodeID, res.Output, deliveredInputs)
			}

			e.persistRun(run)

			// Check for flow-level completion.
			if e.allTerminalsDone(run, def) {
				run.Status = FlowRunCompleted
				now := time.Now()
				run.FinishedAt = &now
				e.persistRun(run)
				e.events.flowCompleted(run)
				return
			}

			// Check for pause (approval gate).
			if run.Status == FlowRunWaitingApproval {
				return
			}

			inflight += e.fireReadyNodes(ctx, run, def, results, deliveredInputs)
		}
	}

	// No in-flight nodes and no terminals done.
	if run.Status == FlowRunRunning {
		run.Status = FlowRunCompleted
		now := time.Now()
		run.FinishedAt = &now
		e.persistRun(run)
		e.events.flowCompleted(run)
	}
}

// --- Node readiness and firing ---

func (e *Engine) findTriggerNode(def *FlowDef) string {
	for _, n := range def.Nodes {
		if n.Type == "trigger" {
			return n.ID
		}
	}
	return ""
}

func (e *Engine) markReady(run *FlowRun, nodeID string) {
	state := run.NodeStates[nodeID]
	state.Status = NodeReady
	run.NodeStates[nodeID] = state
}

func (e *Engine) getReadyNodes(run *FlowRun) []string {
	var ready []string
	for id, state := range run.NodeStates {
		if state.Status == NodeReady {
			ready = append(ready, id)
		}
	}
	return ready
}

func (e *Engine) fireReadyNodes(ctx context.Context, run *FlowRun, def *FlowDef, results chan<- nodeResult, deliveredInputs map[string]map[string]any) int {
	ready := e.getReadyNodes(run)

	// Enforce parallel node limit.
	if e.MaxParallelNodes > 0 && len(ready) > e.MaxParallelNodes {
		ready = ready[:e.MaxParallelNodes]
	}

	for _, nodeID := range ready {
		state := run.NodeStates[nodeID]
		now := time.Now()
		state.Status = NodeRunning
		state.StartedAt = &now
		// Capture inputs so they're visible in the run state for debugging.
		state.Input = e.gatherInputs(deliveredInputs, nodeID)
		run.NodeStates[nodeID] = state

		nodeDef := def.GetNode(nodeID)
		e.events.nodeStarted(run, nodeDef)

		go e.runNode(ctx, run, def, nodeID, results, deliveredInputs)
	}
	return len(ready)
}

func (e *Engine) runNode(ctx context.Context, run *FlowRun, def *FlowDef, nodeID string, results chan<- nodeResult, deliveredInputs map[string]map[string]any) {
	nodeDef := def.GetNode(nodeID)
	started := time.Now()

	inputs := e.gatherInputs(deliveredInputs, nodeID)
	output, err := e.dispatch(ctx, run, nodeDef, inputs)

	results <- nodeResult{
		NodeID:     nodeID,
		Output:     output,
		Error:      err,
		StartedAt:  started,
		FinishedAt: time.Now(),
	}
}

// gatherInputs collects delivered input values for a node.
func (e *Engine) gatherInputs(deliveredInputs map[string]map[string]any, nodeID string) map[string]any {
	if inputs, ok := deliveredInputs[nodeID]; ok {
		return inputs
	}
	return make(map[string]any)
}

// --- Output propagation ---

func (e *Engine) propagateOutputs(run *FlowRun, def *FlowDef, nodeID string, output any, deliveredInputs map[string]map[string]any) {
	for _, edge := range def.EdgesFrom(nodeID) {
		portValue, active := resolvePortValue(output, edge.SourceHandle)

		// Skip edges from inactive output ports (e.g. condition's "false" port
		// when the condition evaluated to true). Without this, both branches fire.
		if !active {
			continue
		}

		// Evaluate edge condition.
		if edge.Condition != "" {
			if !evalCondition(edge.Condition, portValue) {
				continue
			}
		}

		e.deliverInput(deliveredInputs, edge.Target, edge.TargetHandle, portValue)

		if e.isNodeReady(run, def, edge.Target, deliveredInputs) {
			e.markReady(run, edge.Target)
		}
	}
}

// resolvePortValue extracts the value for a specific output port handle.
// Returns (value, active). active=false means this port didn't fire (wrong branch).
func resolvePortValue(output any, sourceHandle string) (any, bool) {
	switch o := output.(type) {
	case PortOutput:
		if o.Port == sourceHandle {
			return o.Value, true
		}
		return nil, false // wrong port — don't deliver
	case RaceOutput:
		if o.Port == sourceHandle {
			return o.Value, true
		}
		if sourceHandle == "winner" {
			return o.Winner, true
		}
		return nil, false
	default:
		// Single-output nodes — any handle gets the full output.
		return output, true
	}
}

func (e *Engine) deliverInput(deliveredInputs map[string]map[string]any, targetNode, targetPort string, value any) {
	if _, ok := deliveredInputs[targetNode]; !ok {
		deliveredInputs[targetNode] = make(map[string]any)
	}
	deliveredInputs[targetNode][targetPort] = value
}

// isNodeReady checks if all required inputs for a node have been delivered.
func (e *Engine) isNodeReady(run *FlowRun, def *FlowDef, nodeID string, deliveredInputs map[string]map[string]any) bool {
	state := run.NodeStates[nodeID]
	if state.Status != NodePending {
		return false
	}

	// Find all edges targeting this node.
	incomingEdges := def.EdgesTo(nodeID)
	if len(incomingEdges) == 0 {
		return false
	}

	// Check that all incoming edge target ports have been delivered.
	delivered := deliveredInputs[nodeID]
	if delivered == nil {
		return false
	}

	node := def.GetNode(nodeID)

	// For merge nodes, wait for ALL connected inputs.
	if node.Type == "merge" {
		for _, edge := range incomingEdges {
			if _, ok := delivered[edge.TargetHandle]; !ok {
				return false
			}
		}
		return true
	}

	// For race nodes, ready when ANY input arrives.
	if node.Type == "race" {
		return len(delivered) > 0
	}

	// For all other nodes, ready when all connected required ports have values.
	connectedPorts := make(map[string]bool)
	for _, edge := range incomingEdges {
		connectedPorts[edge.TargetHandle] = true
	}
	for port := range connectedPorts {
		if _, ok := delivered[port]; !ok {
			return false
		}
	}
	return true
}

// allTerminalsDone checks if all output/error terminal nodes have completed.
func (e *Engine) allTerminalsDone(run *FlowRun, def *FlowDef) bool {
	hasTerminal := false
	for _, n := range def.Nodes {
		if n.Type == "output" || n.Type == "error" {
			hasTerminal = true
			state := run.NodeStates[n.ID]
			if state.Status == NodeCompleted || state.Status == NodeFailed || state.Status == NodeSkipped {
				return true // At least one terminal is done.
			}
		}
	}
	return !hasTerminal // If no terminals, consider done.
}

// --- Error handling ---

func (e *Engine) handleNodeError(run *FlowRun, def *FlowDef, res nodeResult, deliveredInputs map[string]map[string]any) bool {
	node := def.GetNode(res.NodeID)
	strategy := getErrorStrategy(node)
	state := run.NodeStates[res.NodeID]

	// Handle approval sentinel.
	if errors.Is(res.Error, ErrApprovalRequired) {
		state.Status = NodeWaiting
		run.NodeStates[res.NodeID] = state
		run.Status = FlowRunWaitingApproval

		inputs := e.gatherInputs(deliveredInputs, res.NodeID)
		e.events.waitingApproval(run, res.NodeID, inputs)
		e.persistRun(run)
		return true
	}

	switch strategy {
	case "retry":
		max := getMaxRetries(node)
		if state.Retries < max {
			state.Retries++
			state.Status = NodeReady
			run.NodeStates[res.NodeID] = state
			return true
		}

	case "skip":
		state.Status = NodeSkipped
		state.Error = res.Error.Error()
		run.NodeStates[res.NodeID] = state
		e.propagateOutputs(run, def, res.NodeID, nil, deliveredInputs)
		return true

	case "fallback":
		state.Status = NodeCompleted
		state.Error = res.Error.Error()
		run.NodeStates[res.NodeID] = state
		e.propagateOutputs(run, def, res.NodeID, PortOutput{Port: "error", Value: res.Error.Error()}, deliveredInputs)
		return true
	}

	// "stop" — fail the whole flow.
	state.Status = NodeFailed
	state.Error = res.Error.Error()
	run.NodeStates[res.NodeID] = state
	run.Error = fmt.Sprintf("node %s failed: %s", res.NodeID, res.Error.Error())
	e.events.nodeFailed(run, node, res.Error.Error())
	return false
}

// --- Approval API ---

// ApproveNode resumes a flow run after an approval gate.
func (e *Engine) ApproveNode(ctx context.Context, runID, nodeID string) error {
	run, err := e.store.GetRun(runID)
	if err != nil {
		return fmt.Errorf("run not found: %w", err)
	}
	if run.Status != FlowRunWaitingApproval {
		return fmt.Errorf("run is not waiting for approval (status: %s)", run.Status)
	}

	def, err := e.store.GetFlow(run.FlowID)
	if err != nil {
		return fmt.Errorf("flow not found: %w", err)
	}

	state := run.NodeStates[nodeID]
	if state.Status != NodeWaiting {
		return fmt.Errorf("node %s is not waiting for approval", nodeID)
	}

	state.Status = NodeCompleted
	state.Output = PortOutput{Port: "approved", Value: state.Input}
	run.NodeStates[nodeID] = state
	run.Status = FlowRunRunning

	// Rebuild deliveredInputs from completed node outputs.
	deliveredInputs := e.rebuildDeliveredInputs(run, def)

	e.propagateOutputs(run, def, nodeID, state.Output, deliveredInputs)
	e.persistRun(run)

	go e.execute(ctx, run, def)
	return nil
}

// RejectNode rejects an approval gate and resumes with the rejection path.
func (e *Engine) RejectNode(ctx context.Context, runID, nodeID, feedback string) error {
	run, err := e.store.GetRun(runID)
	if err != nil {
		return fmt.Errorf("run not found: %w", err)
	}
	if run.Status != FlowRunWaitingApproval {
		return fmt.Errorf("run is not waiting for approval (status: %s)", run.Status)
	}

	def, err := e.store.GetFlow(run.FlowID)
	if err != nil {
		return fmt.Errorf("flow not found: %w", err)
	}

	state := run.NodeStates[nodeID]
	if state.Status != NodeWaiting {
		return fmt.Errorf("node %s is not waiting for approval", nodeID)
	}

	state.Status = NodeCompleted
	state.Output = PortOutput{Port: "rejected", Value: feedback}
	run.NodeStates[nodeID] = state
	run.Status = FlowRunRunning

	deliveredInputs := e.rebuildDeliveredInputs(run, def)
	e.propagateOutputs(run, def, nodeID, state.Output, deliveredInputs)
	e.persistRun(run)

	go e.execute(ctx, run, def)
	return nil
}

// CancelRun cancels a running flow.
func (e *Engine) CancelRun(runID string) error {
	e.mu.RLock()
	cancel, ok := e.cancels[runID]
	e.mu.RUnlock()

	if ok {
		cancel()
		return nil
	}

	// If not in-memory, update DB directly.
	run, err := e.store.GetRun(runID)
	if err != nil {
		return err
	}
	if run.Status == FlowRunRunning || run.Status == FlowRunPending || run.Status == FlowRunWaitingApproval {
		run.Status = FlowRunCancelled
		now := time.Now()
		run.FinishedAt = &now
		return e.store.SaveRun(run)
	}
	return fmt.Errorf("run %s is not cancellable (status: %s)", runID, run.Status)
}

// --- Startup recovery ---

// RecoverRuns handles in-progress runs after server restart.
func (e *Engine) RecoverRuns() {
	// Mark interrupted running runs as failed.
	runs, err := e.store.ListRunsByStatus(FlowRunRunning)
	if err != nil {
		log.Printf("[flow] recovery: failed to list running runs: %v", err)
		return
	}
	for _, run := range runs {
		run.Status = FlowRunFailed
		run.Error = "interrupted by server restart"
		now := time.Now()
		run.FinishedAt = &now
		e.store.SaveRun(&run)
		log.Printf("[flow] recovery: marked run %s as failed (interrupted)", run.ID)
	}

	// Re-queue pending runs.
	pending, err := e.store.ListRunsByStatus(FlowRunPending)
	if err != nil {
		log.Printf("[flow] recovery: failed to list pending runs: %v", err)
		return
	}
	for _, run := range pending {
		def, err := e.store.GetFlow(run.FlowID)
		if err != nil {
			log.Printf("[flow] recovery: flow %s not found for run %s, marking failed", run.FlowID, run.ID)
			run.Status = FlowRunFailed
			run.Error = "flow definition not found"
			e.store.SaveRun(&run)
			continue
		}
		log.Printf("[flow] recovery: re-queuing pending run %s", run.ID)
		go e.execute(context.Background(), &run, def)
	}

	// Waiting-approval runs need no action.
	waiting, _ := e.store.ListRunsByStatus(FlowRunWaitingApproval)
	if len(waiting) > 0 {
		log.Printf("[flow] recovery: %d runs waiting for approval", len(waiting))
	}
}

// --- Helpers ---

// wrapOutput converts any value to map[string]any for GORM-compatible storage.
func wrapOutput(v any) map[string]any {
	if v == nil {
		return nil
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{"value": v}
}

func (e *Engine) persistRun(run *FlowRun) {
	if err := e.store.SaveRun(run); err != nil {
		log.Printf("[flow] failed to persist run %s: %v", run.ID, err)
	}
}

// rebuildDeliveredInputs reconstructs the delivered inputs map from completed node outputs.
// Used when resuming a run (e.g. after approval).
func (e *Engine) rebuildDeliveredInputs(run *FlowRun, def *FlowDef) map[string]map[string]any {
	deliveredInputs := make(map[string]map[string]any)
	for nodeID, state := range run.NodeStates {
		if state.Status == NodeCompleted {
			for _, edge := range def.EdgesFrom(nodeID) {
				portValue, active := resolvePortValue(state.Output, edge.SourceHandle)
				if !active {
					continue
				}
				e.deliverInput(deliveredInputs, edge.Target, edge.TargetHandle, portValue)
			}
		}
	}
	return deliveredInputs
}

// executeSubgraph runs a subset of nodes (for foreach iterations).
func (e *Engine) executeSubgraph(ctx context.Context, run *FlowRun, subgraphNodeIDs []string, iterPrefix string, input any) (any, error) {
	// Simplified subgraph execution — runs nodes sequentially within the foreach.
	// Each node in the subgraph gets the previous node's output as input.
	currentValue := input

	for _, nodeID := range subgraphNodeIDs {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Get the flow definition from the run's flow.
		def, err := e.store.GetFlow(run.FlowID)
		if err != nil {
			return nil, err
		}

		nodeDef := def.GetNode(nodeID)
		if nodeDef == nil {
			continue
		}

		stateKey := fmt.Sprintf("%s:%s", iterPrefix, nodeID)
		now := time.Now()
		run.NodeStates[stateKey] = NodeState{Status: NodeRunning, StartedAt: &now}

		inputs := map[string]any{"input": currentValue, "item": input}
		output, err := e.dispatch(ctx, run, nodeDef, inputs)

		finished := time.Now()
		if err != nil {
			run.NodeStates[stateKey] = NodeState{
				Status:     NodeFailed,
				Error:      err.Error(),
				StartedAt:  &now,
				FinishedAt: &finished,
			}
			return nil, err
		}

		run.NodeStates[stateKey] = NodeState{
			Status:     NodeCompleted,
			Output:     output,
			StartedAt:  &now,
			FinishedAt: &finished,
			DurationMS: int(finished.Sub(now).Milliseconds()),
		}

		// Extract value from PortOutput for chaining.
		if po, ok := output.(PortOutput); ok {
			currentValue = po.Value
		} else {
			currentValue = output
		}
	}

	return currentValue, nil
}
