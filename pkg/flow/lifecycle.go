package flow

import (
	"context"
	"fmt"
	"log"
	"time"
)

// --- Approval API ---

// ApproveNode resumes a flow run after an approval gate.
// Instead of launching a new execute goroutine, it signals the existing one
// via run.resumeCh to preserve single-writer semantics.
func (e *Engine) ApproveNode(_ context.Context, runID, nodeID string) error {
	liveRun := e.getLiveRun(runID)
	if liveRun == nil {
		return fmt.Errorf("run %s is not active in memory", runID)
	}

	state := liveRun.GetNodeState(nodeID)
	if state.Status != NodeWaiting {
		return fmt.Errorf("node %s is not waiting for approval", nodeID)
	}

	now := time.Now()
	state.Status = NodeCompleted
	state.Output = PortOutput{Port: "approved", Value: state.Input}
	state.FinishedAt = &now
	liveRun.SetNodeState(nodeID, state)

	e.events.flowApproved(liveRun, nodeID)

	liveRun.resumeCh <- approvalResult{
		NodeID: nodeID,
		Output: state.Output,
	}
	return nil
}

// RejectNode rejects an approval gate and resumes with the rejection path.
func (e *Engine) RejectNode(_ context.Context, runID, nodeID, feedback string) error {
	liveRun := e.getLiveRun(runID)
	if liveRun == nil {
		return fmt.Errorf("run %s is not active in memory", runID)
	}

	state := liveRun.GetNodeState(nodeID)
	if state.Status != NodeWaiting {
		return fmt.Errorf("node %s is not waiting for approval", nodeID)
	}

	now := time.Now()
	state.Status = NodeCompleted
	state.Output = PortOutput{Port: "rejected", Value: feedback}
	state.FinishedAt = &now
	liveRun.SetNodeState(nodeID, state)

	e.events.flowRejected(liveRun, nodeID, feedback)

	liveRun.resumeCh <- approvalResult{
		NodeID: nodeID,
		Output: state.Output,
	}
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
	for i := range runs {
		runs[i].Status = FlowRunFailed
		runs[i].Error = "interrupted by server restart"
		now := time.Now()
		runs[i].FinishedAt = &now
		e.store.SaveRun(&runs[i])
		log.Printf("[flow] recovery: marked run %s as failed (interrupted)", runs[i].ID)
	}

	// Re-queue pending runs.
	pending, err := e.store.ListRunsByStatus(FlowRunPending)
	if err != nil {
		log.Printf("[flow] recovery: failed to list pending runs: %v", err)
		return
	}
	for i := range pending {
		def, err := e.store.GetFlow(pending[i].FlowID)
		if err != nil {
			log.Printf("[flow] recovery: flow %s not found for run %s, marking failed", pending[i].FlowID, pending[i].ID)
			pending[i].Status = FlowRunFailed
			pending[i].Error = "flow definition not found"
			e.store.SaveRun(&pending[i])
			continue
		}
		log.Printf("[flow] recovery: re-queuing pending run %s", pending[i].ID)
		pending[i].resumeCh = make(chan approvalResult, 1)
		go e.execute(context.Background(), &pending[i], def)
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

// subgraphResult holds the node states produced by a single foreach iteration.
// States are collected locally to avoid concurrent writes to run.NodeStates.
type subgraphResult struct {
	Value  any
	States map[string]NodeState
}

// executeSubgraph runs a subset of nodes sequentially (for foreach iterations).
// It writes to a local state map; the caller merges results into run.NodeStates.
func (e *Engine) executeSubgraph(ctx context.Context, run *FlowRun, def *FlowDef, subgraphNodeIDs []string, iterPrefix string, input any) (*subgraphResult, error) {
	currentValue := input
	localStates := make(map[string]NodeState, len(subgraphNodeIDs))

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
		localStates[stateKey] = NodeState{Status: NodeRunning, StartedAt: &now}

		inputs := map[string]any{"input": currentValue, "item": input}
		output, err := e.dispatch(ctx, run, def, nodeDef, inputs)

		finished := time.Now()
		if err != nil {
			localStates[stateKey] = NodeState{
				Status: NodeFailed, Error: err.Error(),
				StartedAt: &now, FinishedAt: &finished,
			}
			return nil, err
		}

		localStates[stateKey] = NodeState{
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
	return &subgraphResult{Value: currentValue, States: localStates}, nil
}

