package flow

import (
	"context"
	"fmt"
	"log"
	"sync"
)

// TriggerHandler manages a persistent listener for a deployed flow.
// Implementations handle the specific trigger type (cron, webhook, event, etc).
// The handler calls onTrigger when the trigger fires — that starts a new flow run.
type TriggerHandler interface {
	// Start begins listening. onTrigger is called each time the trigger fires.
	Start(ctx context.Context, def *FlowDef, onTrigger func(inputs map[string]any)) error
	// Stop shuts down the listener and releases resources.
	Stop() error
}

// TriggerFactory creates a TriggerHandler for a given trigger type.
// Register factories for "cron", "webhook", "event", etc.
type TriggerFactory func(triggerType string) (TriggerHandler, error)

// deployment tracks a deployed flow and its active trigger.
type deployment struct {
	def     *FlowDef
	handler TriggerHandler
	cancel  context.CancelFunc
	state   map[string]any // persistent in-memory state across runs (like PLC retain vars)
	stateMu sync.RWMutex
}

// runtime manages the deploy/undeploy lifecycle for flows.
// It's embedded in Engine — keeps deployment state separate from execution state.
type runtime struct {
	mu          sync.RWMutex
	deployments map[string]*deployment // flowID -> deployment
	factories   map[string]TriggerFactory
}

func newRuntime() *runtime {
	return &runtime{
		deployments: make(map[string]*deployment),
		factories:   make(map[string]TriggerFactory),
	}
}

// RegisterTrigger registers a factory for a trigger type (e.g. "cron", "webhook").
func (e *Engine) RegisterTrigger(triggerType string, factory TriggerFactory) {
	e.runtime.mu.Lock()
	defer e.runtime.mu.Unlock()
	e.runtime.factories[triggerType] = factory
}

// Deploy makes a flow "live" — its trigger starts listening and will
// auto-create runs when it fires. Manual triggers are tracked but have
// no active listener.
func (e *Engine) Deploy(ctx context.Context, def *FlowDef) error {
	e.runtime.mu.Lock()
	defer e.runtime.mu.Unlock()

	// Undeploy existing if re-deploying.
	if existing, ok := e.runtime.deployments[def.ID]; ok {
		e.stopDeployment(existing)
	}

	dep := &deployment{def: def, state: make(map[string]any)}

	// Read trigger config from the trigger node's Data map.
	triggerData := def.TriggerConfig()
	triggerType, _ := triggerData["type"].(string)

	if triggerType != "" && triggerType != "manual" {
		factory, ok := e.runtime.factories[triggerType]
		if !ok {
			log.Printf("[flow] deploy %s: no handler for trigger type %q, manual-only", def.Name, triggerType)
			e.runtime.deployments[def.ID] = dep
			return nil
		}

		handler, err := factory(triggerType)
		if err != nil {
			return fmt.Errorf("create trigger handler for %s: %w", def.Name, err)
		}

		triggerCtx, cancel := context.WithCancel(ctx)
		dep.handler = handler
		dep.cancel = cancel

		onTrigger := func(inputs map[string]any) {
			userID := def.UserID
			if userID == "" {
				userID = "system"
			}
			run, err := e.StartRun(triggerCtx, def.ID, userID, inputs, nil)
			if err != nil {
				log.Printf("[flow] trigger %s: failed to start run: %v", def.Name, err)
				return
			}
			log.Printf("[flow] trigger %s: started run %s", def.Name, run.ID)
		}

		if err := handler.Start(triggerCtx, def, onTrigger); err != nil {
			cancel()
			return fmt.Errorf("start trigger for %s: %w", def.Name, err)
		}

		log.Printf("[flow] deployed %s with %s trigger", def.Name, triggerType)
	} else {
		log.Printf("[flow] deployed %s (manual trigger)", def.Name)
	}

	e.runtime.deployments[def.ID] = dep
	return nil
}

// Undeploy stops a flow's trigger and removes it from the runtime.
func (e *Engine) Undeploy(flowID string) error {
	e.runtime.mu.Lock()
	defer e.runtime.mu.Unlock()

	dep, ok := e.runtime.deployments[flowID]
	if !ok {
		return nil // not deployed, nothing to do
	}

	e.stopDeployment(dep)
	delete(e.runtime.deployments, flowID)
	log.Printf("[flow] undeployed %s", dep.def.Name)
	return nil
}

// DeployAll loads all flow definitions from the store and deploys them.
// Called once on server boot to make the runtime self-starting.
func (e *Engine) DeployAll(ctx context.Context) {
	defs, err := e.store.ListFlows("")
	if err != nil {
		log.Printf("[flow] deploy-all: failed to list flows: %v", err)
		return
	}

	deployed := 0
	for _, def := range defs {
		if err := e.Deploy(ctx, &def); err != nil {
			log.Printf("[flow] deploy-all: %s: %v", def.Name, err)
			continue
		}
		deployed++
	}
	log.Printf("[flow] deployed %d/%d flows", deployed, len(defs))
}

// IsDeployed checks if a flow is currently deployed.
func (e *Engine) IsDeployed(flowID string) bool {
	e.runtime.mu.RLock()
	defer e.runtime.mu.RUnlock()
	_, ok := e.runtime.deployments[flowID]
	return ok
}

// Shutdown stops all deployed flows. Called on server shutdown.
func (e *Engine) Shutdown() {
	e.runtime.mu.Lock()
	defer e.runtime.mu.Unlock()

	for id, dep := range e.runtime.deployments {
		e.stopDeployment(dep)
		delete(e.runtime.deployments, id)
	}
	log.Println("[flow] runtime shutdown: all flows undeployed")
}

// GetFlowState reads a value from a flow's persistent in-memory state.
// Returns (value, true) if found, (nil, false) if not.
func (e *Engine) GetFlowState(flowID, key string) (any, bool) {
	e.runtime.mu.RLock()
	dep, ok := e.runtime.deployments[flowID]
	e.runtime.mu.RUnlock()
	if !ok {
		return nil, false
	}
	dep.stateMu.RLock()
	defer dep.stateMu.RUnlock()
	v, ok := dep.state[key]
	return v, ok
}

// SetFlowState writes a value to a flow's persistent in-memory state.
func (e *Engine) SetFlowState(flowID, key string, value any) {
	e.runtime.mu.RLock()
	dep, ok := e.runtime.deployments[flowID]
	e.runtime.mu.RUnlock()
	if !ok {
		return
	}
	dep.stateMu.Lock()
	dep.state[key] = value
	dep.stateMu.Unlock()
}

// GetFlowStateAll returns a snapshot of a flow's entire state map.
func (e *Engine) GetFlowStateAll(flowID string) map[string]any {
	e.runtime.mu.RLock()
	dep, ok := e.runtime.deployments[flowID]
	e.runtime.mu.RUnlock()
	if !ok {
		return nil
	}
	dep.stateMu.RLock()
	defer dep.stateMu.RUnlock()
	snapshot := make(map[string]any, len(dep.state))
	for k, v := range dep.state {
		snapshot[k] = v
	}
	return snapshot
}

func (e *Engine) stopDeployment(dep *deployment) {
	if dep.handler != nil {
		if err := dep.handler.Stop(); err != nil {
			log.Printf("[flow] stop trigger %s: %v", dep.def.Name, err)
		}
	}
	if dep.cancel != nil {
		dep.cancel()
	}
}
