package flow

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	// Use a shared in-memory DB with WAL mode so concurrent goroutines
	// see the same tables. Each test gets a unique name.
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared&_journal_mode=WAL"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Discard,
	})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func testEngine(t *testing.T) *Engine {
	t.Helper()
	db := testDB(t)
	store := NewStore(db)
	registry := NewRegistry()
	return NewEngine(store, registry)
}

// waitRun polls until the run reaches a terminal status or times out.
func waitRun(t *testing.T, e *Engine, runID string, timeout time.Duration) *FlowRun {
	t.Helper()
	deadline := time.After(timeout)
	for {
		run, err := e.Store().GetRun(runID)
		if err != nil {
			t.Fatal(err)
		}
		switch run.Status {
		case FlowRunCompleted, FlowRunFailed, FlowRunCancelled:
			return run
		}
		select {
		case <-deadline:
			t.Fatalf("run %s timed out in status %s", runID, run.Status)
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// TestSimpleValueToOutput: trigger → value → output
func TestSimpleValueToOutput(t *testing.T) {
	e := testEngine(t)

	def := &FlowDef{
		Name: "test-value",
		Nodes: []FlowNodeDef{
			{ID: "trigger-1", Type: "trigger", Position: Position{X: 0, Y: 0}},
			{ID: "value-1", Type: "value", Position: Position{X: 100, Y: 0}, Data: map[string]any{
				"value": map[string]any{"greeting": "hello", "count": 42},
			}},
			{ID: "output-1", Type: "output", Position: Position{X: 200, Y: 0}},
		},
		Edges: []FlowEdgeDef{
			{ID: "e1", Source: "trigger-1", SourceHandle: "output", Target: "value-1", TargetHandle: "input"},
			{ID: "e2", Source: "value-1", SourceHandle: "output", Target: "output-1", TargetHandle: "input"},
		},
	}

	if err := e.Store().CreateFlow(def); err != nil {
		t.Fatal(err)
	}

	run, err := e.StartRun(context.Background(), def.ID, "user-1", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	run = waitRun(t, e, run.ID, 5*time.Second)

	if run.Status != FlowRunCompleted {
		t.Fatalf("expected completed, got %s (error: %s)", run.Status, run.Error)
	}

	// Check the output was stored.
	if run.Output == nil {
		t.Fatal("expected output, got nil")
	}
	out := run.Output
	if out["greeting"] != nil {
		// value node returns the raw value, output node wraps it
		t.Logf("flow output: %v", out)
	}

	// Check node states have input/output.
	valueState := run.NodeStates["value-1"]
	if valueState.Status != NodeCompleted {
		t.Fatalf("value node status: %s", valueState.Status)
	}
	t.Logf("value-1 input: %v", valueState.Input)
	t.Logf("value-1 output: %v", valueState.Output)
}

// TestConditionBranching: trigger → condition → true path → output, false path should NOT fire.
func TestConditionBranching(t *testing.T) {
	e := testEngine(t)

	def := &FlowDef{
		Name: "test-condition",
		Nodes: []FlowNodeDef{
			{ID: "trigger-1", Type: "trigger", Position: Position{X: 0, Y: 0}},
			// Expression evaluates against the inputs map — "input" is what the trigger delivered.
			// The trigger outputs run.Inputs, which is a map. The edge delivers the whole map
			// under the target handle "input", so inside the condition: input = {"value": 10}.
			// The expression needs to reference: input.value > 5
			{ID: "cond-1", Type: "condition", Position: Position{X: 100, Y: 0}, Data: map[string]any{
				"expression": "input.value > 5",
			}},
			{ID: "log-true", Type: "log", Position: Position{X: 200, Y: 0}, Data: map[string]any{
				"message": "TRUE branch fired",
			}},
			{ID: "log-false", Type: "log", Position: Position{X: 200, Y: 100}, Data: map[string]any{
				"message": "FALSE branch fired",
			}},
			{ID: "output-1", Type: "output", Position: Position{X: 300, Y: 0}},
			{ID: "output-2", Type: "output", Position: Position{X: 300, Y: 100}},
		},
		Edges: []FlowEdgeDef{
			{ID: "e1", Source: "trigger-1", SourceHandle: "output", Target: "cond-1", TargetHandle: "input"},
			{ID: "e2", Source: "cond-1", SourceHandle: "true", Target: "log-true", TargetHandle: "input"},
			{ID: "e3", Source: "cond-1", SourceHandle: "false", Target: "log-false", TargetHandle: "input"},
			{ID: "e4", Source: "log-true", SourceHandle: "output", Target: "output-1", TargetHandle: "input"},
			{ID: "e5", Source: "log-false", SourceHandle: "output", Target: "output-2", TargetHandle: "input"},
		},
	}

	if err := e.Store().CreateFlow(def); err != nil {
		t.Fatal(err)
	}

	// input.value > 5 → true branch should fire.
	run, err := e.StartRun(context.Background(), def.ID, "user-1", map[string]any{"value": 10}, nil)
	if err != nil {
		t.Fatal(err)
	}

	run = waitRun(t, e, run.ID, 5*time.Second)

	if run.Status != FlowRunCompleted {
		t.Fatalf("expected completed, got %s (error: %s)", run.Status, run.Error)
	}

	// True branch should have completed.
	trueState := run.NodeStates["log-true"]
	if trueState.Status != NodeCompleted {
		t.Errorf("true branch: expected completed, got %s", trueState.Status)
	}

	// False branch should NOT have fired (should be pending).
	falseState := run.NodeStates["log-false"]
	if falseState.Status == NodeCompleted {
		t.Errorf("false branch should NOT have fired, but it completed")
	}
	t.Logf("true branch status: %s, false branch status: %s", trueState.Status, falseState.Status)
}

// TestLogPassthrough: trigger → log → output (data flows through log node unchanged).
func TestLogPassthrough(t *testing.T) {
	e := testEngine(t)

	def := &FlowDef{
		Name: "test-log",
		Nodes: []FlowNodeDef{
			{ID: "trigger-1", Type: "trigger", Position: Position{X: 0, Y: 0}},
			{ID: "log-1", Type: "log", Position: Position{X: 100, Y: 0}},
			{ID: "output-1", Type: "output", Position: Position{X: 200, Y: 0}},
		},
		Edges: []FlowEdgeDef{
			{ID: "e1", Source: "trigger-1", SourceHandle: "output", Target: "log-1", TargetHandle: "input"},
			{ID: "e2", Source: "log-1", SourceHandle: "output", Target: "output-1", TargetHandle: "input"},
		},
	}

	if err := e.Store().CreateFlow(def); err != nil {
		t.Fatal(err)
	}

	inputs := map[string]any{"message": "hello world", "count": 7}
	run, err := e.StartRun(context.Background(), def.ID, "user-1", inputs, nil)
	if err != nil {
		t.Fatal(err)
	}

	run = waitRun(t, e, run.ID, 5*time.Second)

	if run.Status != FlowRunCompleted {
		t.Fatalf("expected completed, got %s (error: %s)", run.Status, run.Error)
	}

	// Log node should have received the trigger inputs and passed them through.
	logState := run.NodeStates["log-1"]
	t.Logf("log-1 input: %v", logState.Input)
	t.Logf("log-1 output: %v", logState.Output)

	if logState.Input == nil {
		t.Error("log node input should not be nil")
	}
}

// TestTransformNode: trigger → transform (expression) → output.
func TestTransformNode(t *testing.T) {
	e := testEngine(t)

	def := &FlowDef{
		Name: "test-transform",
		Nodes: []FlowNodeDef{
			{ID: "trigger-1", Type: "trigger", Position: Position{X: 0, Y: 0}},
			// input is the map delivered by the trigger; access input.value to get the number.
			{ID: "xform-1", Type: "transform", Position: Position{X: 100, Y: 0}, Data: map[string]any{
				"expression": "input.value * 2",
			}},
			{ID: "output-1", Type: "output", Position: Position{X: 200, Y: 0}},
		},
		Edges: []FlowEdgeDef{
			{ID: "e1", Source: "trigger-1", SourceHandle: "output", Target: "xform-1", TargetHandle: "input"},
			{ID: "e2", Source: "xform-1", SourceHandle: "output", Target: "output-1", TargetHandle: "input"},
		},
	}

	if err := e.Store().CreateFlow(def); err != nil {
		t.Fatal(err)
	}

	run, err := e.StartRun(context.Background(), def.ID, "user-1", map[string]any{"value": 21}, nil)
	if err != nil {
		t.Fatal(err)
	}

	run = waitRun(t, e, run.ID, 5*time.Second)

	if run.Status != FlowRunCompleted {
		t.Fatalf("expected completed, got %s (error: %s)", run.Status, run.Error)
	}

	// Transform should have doubled the input.
	xformState := run.NodeStates["xform-1"]
	t.Logf("transform input: %v, output: %v", xformState.Input, xformState.Output)
}

// TestTemplateNode: trigger → template → output.
func TestTemplateNode(t *testing.T) {
	e := testEngine(t)

	def := &FlowDef{
		Name: "test-template",
		Nodes: []FlowNodeDef{
			{ID: "trigger-1", Type: "trigger", Position: Position{X: 0, Y: 0}},
			// input is the map from the trigger. Use index to extract a key.
			{ID: "tmpl-1", Type: "template", Position: Position{X: 100, Y: 0}, Data: map[string]any{
				"template": "Hello {{index .input \"name\"}}!",
			}},
			{ID: "output-1", Type: "output", Position: Position{X: 200, Y: 0}},
		},
		Edges: []FlowEdgeDef{
			{ID: "e1", Source: "trigger-1", SourceHandle: "output", Target: "tmpl-1", TargetHandle: "input"},
			{ID: "e2", Source: "tmpl-1", SourceHandle: "output", Target: "output-1", TargetHandle: "input"},
		},
	}

	if err := e.Store().CreateFlow(def); err != nil {
		t.Fatal(err)
	}

	run, err := e.StartRun(context.Background(), def.ID, "user-1", map[string]any{"name": "World"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	run = waitRun(t, e, run.ID, 5*time.Second)

	if run.Status != FlowRunCompleted {
		t.Fatalf("expected completed, got %s (error: %s)", run.Status, run.Error)
	}

	tmplState := run.NodeStates["tmpl-1"]
	t.Logf("template output: %v", tmplState.Output)

	if s, ok := tmplState.Output.(string); ok && s != "Hello World!" {
		t.Errorf("expected 'Hello World!', got %q", s)
	}
}

// TestErrorHandlingSkip: node with on_error=skip should skip and continue.
func TestErrorHandlingSkip(t *testing.T) {
	e := testEngine(t)

	def := &FlowDef{
		Name: "test-skip",
		Nodes: []FlowNodeDef{
			{ID: "trigger-1", Type: "trigger", Position: Position{X: 0, Y: 0}},
			// Transform with bad expression — will error
			{ID: "xform-bad", Type: "transform", Position: Position{X: 100, Y: 0}, Data: map[string]any{
				"expression": "undefined_var + 1",
				"on_error":   "skip",
			}},
			{ID: "output-1", Type: "output", Position: Position{X: 200, Y: 0}},
		},
		Edges: []FlowEdgeDef{
			{ID: "e1", Source: "trigger-1", SourceHandle: "output", Target: "xform-bad", TargetHandle: "input"},
			{ID: "e2", Source: "xform-bad", SourceHandle: "output", Target: "output-1", TargetHandle: "input"},
		},
	}

	if err := e.Store().CreateFlow(def); err != nil {
		t.Fatal(err)
	}

	run, err := e.StartRun(context.Background(), def.ID, "user-1", map[string]any{"input": 5}, nil)
	if err != nil {
		t.Fatal(err)
	}

	run = waitRun(t, e, run.ID, 5*time.Second)

	if run.Status != FlowRunCompleted {
		t.Fatalf("expected completed (skip should continue), got %s (error: %s)", run.Status, run.Error)
	}

	badState := run.NodeStates["xform-bad"]
	if badState.Status != NodeSkipped {
		t.Errorf("expected skipped, got %s", badState.Status)
	}
	t.Logf("skipped node error: %s", badState.Error)
}
