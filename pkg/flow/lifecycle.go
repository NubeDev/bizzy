package flow

import (
	"context"
	"fmt"
	"log"
	"time"
)

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
	deliveredInputs := rebuildDeliveredInputs(run, def)

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

	deliveredInputs := rebuildDeliveredInputs(run, def)
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

// rebuildDeliveredInputs reconstructs the delivered inputs map from completed node outputs.
// Used when resuming a run (e.g. after approval).
func rebuildDeliveredInputs(run *FlowRun, def *FlowDef) map[string]map[string]any {
	delivered := make(map[string]map[string]any)
	for nodeID, state := range run.NodeStates {
		if state.Status == NodeCompleted {
			for _, edge := range def.EdgesFrom(nodeID) {
				portValue, active := resolvePortValue(state.Output, edge.SourceHandle)
				if !active {
					continue
				}
				deliverInput(delivered, edge.Target, edge.TargetHandle, portValue)
			}
		}
	}
	return delivered
}

// executeSubgraph runs a subset of nodes sequentially (for foreach iterations).
func (e *Engine) executeSubgraph(ctx context.Context, run *FlowRun, def *FlowDef, subgraphNodeIDs []string, iterPrefix string, input any) (any, error) {
	currentValue := input

	for _, nodeID := range subgraphNodeIDs {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		nodeDef := def.GetNode(nodeID)
		if nodeDef == nil {
			continue
		}

		stateKey := fmt.Sprintf("%s:%s", iterPrefix, nodeID)
		now := time.Now()
		run.NodeStates[stateKey] = NodeState{Status: NodeRunning, StartedAt: &now}

		inputs := map[string]any{"input": currentValue, "item": input}
		output, err := e.dispatch(ctx, run, def, nodeDef, inputs)

		finished := time.Now()
		if err != nil {
			run.NodeStates[stateKey] = NodeState{
				Status: NodeFailed, Error: err.Error(),
				StartedAt: &now, FinishedAt: &finished,
			}
			return nil, err
		}

		run.NodeStates[stateKey] = NodeState{
			Status: NodeCompleted, Output: output,
			StartedAt: &now, FinishedAt: &finished,
			DurationMS: int(finished.Sub(now).Milliseconds()),
		}

		if po, ok := output.(PortOutput); ok {
			currentValue = po.Value
		} else {
			currentValue = output
		}
	}
	return currentValue, nil
}

