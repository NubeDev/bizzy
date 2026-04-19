package flow

import (
	"errors"
	"fmt"
)

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

		deliverInput(deliveredInputs, edge.Target, edge.TargetHandle, portValue)

		if isNodeReady(run, def, edge.Target, deliveredInputs) {
			markReady(run, edge.Target)
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

func deliverInput(deliveredInputs map[string]map[string]any, targetNode, targetPort string, value any) {
	if _, ok := deliveredInputs[targetNode]; !ok {
		deliveredInputs[targetNode] = make(map[string]any)
	}
	deliveredInputs[targetNode][targetPort] = value
}

// isNodeReady checks if all required inputs for a node have been delivered.
func isNodeReady(run *FlowRun, def *FlowDef, nodeID string, deliveredInputs map[string]map[string]any) bool {
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

// allTerminalsDone checks if any output/error terminal node has completed.
// Returns false if there are no terminal nodes — the main loop's inflight==0
// exit handles completion for flows without terminals.
func allTerminalsDone(run *FlowRun, def *FlowDef) bool {
	for _, n := range def.Nodes {
		if n.Type == "output" || n.Type == "error" {
			state := run.NodeStates[n.ID]
			if state.Status == NodeCompleted || state.Status == NodeFailed || state.Status == NodeSkipped {
				return true
			}
		}
	}
	return false
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

		inputs := gatherInputs(deliveredInputs, res.NodeID)
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
		e.events.nodeSkipped(run, node, res.Error.Error())
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
