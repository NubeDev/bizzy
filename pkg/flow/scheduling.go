package flow

import (
	"context"
	"time"
)

// --- Node readiness and firing ---

func findTriggerNode(def *FlowDef) string {
	for _, n := range def.Nodes {
		if n.Type == "trigger" {
			return n.ID
		}
	}
	return ""
}

func markReady(run *FlowRun, nodeID string) {
	state := run.NodeStates[nodeID]
	state.Status = NodeReady
	run.NodeStates[nodeID] = state
}

func getReadyNodes(run *FlowRun) []string {
	var ready []string
	for id, state := range run.NodeStates {
		if state.Status == NodeReady {
			ready = append(ready, id)
		}
	}
	return ready
}

func (e *Engine) fireReadyNodes(ctx context.Context, run *FlowRun, def *FlowDef, results chan<- nodeResult, deliveredInputs map[string]map[string]any) int {
	ready := getReadyNodes(run)

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
		state.Input = gatherInputs(deliveredInputs, nodeID)
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

	inputs := gatherInputs(deliveredInputs, nodeID)
	output, err := e.dispatch(ctx, run, def, nodeDef, inputs)

	results <- nodeResult{
		NodeID:     nodeID,
		Output:     output,
		Error:      err,
		StartedAt:  started,
		FinishedAt: time.Now(),
	}
}

// gatherInputs collects delivered input values for a node.
func gatherInputs(deliveredInputs map[string]map[string]any, nodeID string) map[string]any {
	if inputs, ok := deliveredInputs[nodeID]; ok {
		return inputs
	}
	return make(map[string]any)
}
