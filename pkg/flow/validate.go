package flow

import (
	"fmt"
	"strings"
)

// ValidationError collects multiple validation issues.
type ValidationError struct {
	Errors []string `json:"errors"`
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("flow validation failed: %s", strings.Join(e.Errors, "; "))
}

func (e *ValidationError) add(msg string, args ...any) {
	e.Errors = append(e.Errors, fmt.Sprintf(msg, args...))
}

func (e *ValidationError) hasErrors() bool {
	return len(e.Errors) > 0
}

// Validate checks a flow definition for structural correctness.
// It performs cycle detection, port validation, type checking, and subgraph analysis.
func Validate(def *FlowDef, registry *NodeTypeRegistry) *ValidationError {
	ve := &ValidationError{}

	if len(def.Nodes) == 0 {
		ve.add("flow has no nodes")
		return ve
	}

	// Build lookup maps.
	nodeByID := make(map[string]*FlowNodeDef, len(def.Nodes))
	for i := range def.Nodes {
		n := &def.Nodes[i]
		if _, dup := nodeByID[n.ID]; dup {
			ve.add("duplicate node ID: %s", n.ID)
		}
		nodeByID[n.ID] = n
	}

	// 1. Exactly one trigger node.
	triggerCount := 0
	var triggerID string
	for _, n := range def.Nodes {
		if n.Type == "trigger" {
			triggerCount++
			triggerID = n.ID
		}
	}
	if triggerCount == 0 {
		ve.add("flow must have exactly one trigger node")
	} else if triggerCount > 1 {
		ve.add("flow must have exactly one trigger node, found %d", triggerCount)
	}

	// 2. All node types registered.
	for _, n := range def.Nodes {
		if !registry.Has(n.Type) && !strings.HasPrefix(n.Type, "tool:") {
			ve.add("node %s: unknown type %q", n.ID, n.Type)
		}
	}

	// 3. Edge validity: source and target nodes exist.
	edgesByTarget := make(map[string][]FlowEdgeDef) // target node -> edges
	edgesBySource := make(map[string][]FlowEdgeDef) // source node -> edges
	for _, e := range def.Edges {
		if _, ok := nodeByID[e.Source]; !ok {
			ve.add("edge %s: source node %q not found", e.ID, e.Source)
		}
		if _, ok := nodeByID[e.Target]; !ok {
			ve.add("edge %s: target node %q not found", e.ID, e.Target)
		}
		edgesByTarget[e.Target] = append(edgesByTarget[e.Target], e)
		edgesBySource[e.Source] = append(edgesBySource[e.Source], e)
	}

	// 4. At least one terminal node (output or error).
	hasTerminal := false
	for _, n := range def.Nodes {
		if n.Type == "output" || n.Type == "error" {
			hasTerminal = true
			break
		}
	}
	if !hasTerminal {
		ve.add("flow must have at least one output or error terminal node")
	}

	// 5. Required ports must be connected.
	for _, n := range def.Nodes {
		typeDef, ok := registry.Get(n.Type)
		if !ok {
			continue
		}
		ports := typeDef.Ports
		if n.Ports != nil {
			ports = *n.Ports
		}
		connectedInputs := make(map[string]bool)
		for _, e := range edgesByTarget[n.ID] {
			connectedInputs[e.TargetHandle] = true
		}
		for _, p := range ports.Inputs {
			if p.Required && !connectedInputs[p.Handle] {
				ve.add("node %s (%s): required input port %q not connected", n.ID, n.Type, p.Handle)
			}
		}
	}

	// 6. Cycle detection (Kahn's algorithm for topological sort).
	if triggerID != "" {
		if hasCycle := detectCycle(def, nodeByID); hasCycle {
			ve.add("flow contains a cycle (not a valid DAG)")
		}
	}

	// 7. Trigger config validation.
	if def.Trigger != nil && def.Trigger.Type == "cron" && def.Trigger.Schedule == "" {
		ve.add("cron trigger must have a schedule")
	}

	// 8. ForEach subgraph boundary validation.
	validateForEachSubgraphs(def, nodeByID, edgesBySource, edgesByTarget, ve)

	if ve.hasErrors() {
		return ve
	}
	return nil
}

// detectCycle uses Kahn's algorithm to check for cycles.
func detectCycle(def *FlowDef, nodeByID map[string]*FlowNodeDef) bool {
	// Build adjacency and in-degree.
	inDegree := make(map[string]int, len(def.Nodes))
	for _, n := range def.Nodes {
		inDegree[n.ID] = 0
	}
	for _, e := range def.Edges {
		inDegree[e.Target]++
	}

	// Seed queue with zero in-degree nodes.
	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	visited := 0
	for len(queue) > 0 {
		nodeID := queue[0]
		queue = queue[1:]
		visited++

		for _, e := range def.Edges {
			if e.Source == nodeID {
				inDegree[e.Target]--
				if inDegree[e.Target] == 0 {
					queue = append(queue, e.Target)
				}
			}
		}
	}

	return visited != len(def.Nodes)
}

// TopologicalSort returns nodes in topological order.
func TopologicalSort(def *FlowDef) ([]string, error) {
	inDegree := make(map[string]int, len(def.Nodes))
	for _, n := range def.Nodes {
		inDegree[n.ID] = 0
	}
	for _, e := range def.Edges {
		inDegree[e.Target]++
	}

	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	var order []string
	for len(queue) > 0 {
		nodeID := queue[0]
		queue = queue[1:]
		order = append(order, nodeID)

		for _, e := range def.Edges {
			if e.Source == nodeID {
				inDegree[e.Target]--
				if inDegree[e.Target] == 0 {
					queue = append(queue, e.Target)
				}
			}
		}
	}

	if len(order) != len(def.Nodes) {
		return nil, fmt.Errorf("cycle detected in flow graph")
	}
	return order, nil
}

// validateForEachSubgraphs identifies nodes reachable from foreach item ports
// and stores them in the foreach node's Data for runtime use.
func validateForEachSubgraphs(def *FlowDef, nodeByID map[string]*FlowNodeDef, edgesBySource, edgesByTarget map[string][]FlowEdgeDef, ve *ValidationError) {
	for i := range def.Nodes {
		n := &def.Nodes[i]
		if n.Type != "foreach" {
			continue
		}
		if n.Data == nil {
			n.Data = make(map[string]any)
		}

		// Walk forward from the foreach's "item" output port.
		subgraph := make(map[string]bool)
		var walk func(nodeID string)
		walk = func(nodeID string) {
			if subgraph[nodeID] {
				return
			}
			subgraph[nodeID] = true
			for _, e := range edgesBySource[nodeID] {
				walk(e.Target)
			}
		}

		// Find edges from this foreach node's "item" port.
		for _, e := range edgesBySource[n.ID] {
			if e.SourceHandle == "item" {
				walk(e.Target)
			}
		}

		// Store subgraph node IDs.
		ids := make([]string, 0, len(subgraph))
		for id := range subgraph {
			ids = append(ids, id)
		}
		n.Data["_subgraph_nodes"] = ids
	}
}
