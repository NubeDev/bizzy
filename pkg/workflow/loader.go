package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// Store holds all loaded workflow definitions, keyed by "appName/workflowName".
type Store struct {
	mu        sync.RWMutex
	workflows map[string]*WorkflowDef // key: "appName/workflowName"
	byApp     map[string][]string     // appName -> list of workflow names
}

// NewStore creates an empty workflow store.
func NewStore() *Store {
	return &Store{
		workflows: make(map[string]*WorkflowDef),
		byApp:     make(map[string][]string),
	}
}

// LoadFromAppDir scans an app directory for workflows/*.yaml and registers them.
func (s *Store) LoadFromAppDir(appName, appDir string) error {
	wfDir := filepath.Join(appDir, "workflows")
	entries, err := os.ReadDir(wfDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no workflows dir is fine
		}
		return fmt.Errorf("read workflows dir: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear existing entries for this app (for reload support).
	if old, ok := s.byApp[appName]; ok {
		for _, name := range old {
			delete(s.workflows, appName+"/"+name)
		}
		delete(s.byApp, appName)
	}

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(wfDir, e.Name()))
		if err != nil {
			return fmt.Errorf("read %s: %w", e.Name(), err)
		}
		var def WorkflowDef
		if err := yaml.Unmarshal(data, &def); err != nil {
			return fmt.Errorf("parse %s: %w", e.Name(), err)
		}
		if def.Name == "" {
			return fmt.Errorf("%s: workflow name is required", e.Name())
		}
		key := appName + "/" + def.Name
		s.workflows[key] = &def
		s.byApp[appName] = append(s.byApp[appName], def.Name)
	}
	return nil
}

// Get returns a workflow definition by app and workflow name.
func (s *Store) Get(appName, workflowName string) (*WorkflowDef, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	def, ok := s.workflows[appName+"/"+workflowName]
	return def, ok
}

// ListByApp returns all workflow names for an app.
func (s *Store) ListByApp(appName string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.byApp[appName]
}

// ListAll returns all workflow definitions grouped by app name.
func (s *Store) ListAll() map[string][]*WorkflowDef {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string][]*WorkflowDef)
	for key, def := range s.workflows {
		// Key is "appName/workflowName", extract appName.
		for i := range key {
			if key[i] == '/' {
				appName := key[:i]
				out[appName] = append(out[appName], def)
				break
			}
		}
	}
	return out
}

// Validate checks a workflow definition for common errors.
func Validate(def *WorkflowDef) error {
	if def.Name == "" {
		return fmt.Errorf("workflow name is required")
	}
	if len(def.Stages) == 0 {
		return fmt.Errorf("workflow %q has no stages", def.Name)
	}
	seen := make(map[string]bool)
	for i, s := range def.Stages {
		if s.Name == "" {
			return fmt.Errorf("stage %d has no name", i)
		}
		if seen[s.Name] {
			return fmt.Errorf("duplicate stage name %q", s.Name)
		}
		seen[s.Name] = true

		if s.StageType() == "unknown" {
			return fmt.Errorf("stage %q: must set one of tool, prompt, approval, output, or type", s.Name)
		}

		switch s.OnFail {
		case "", "stop", "retry", "skip", "fallback":
			// ok
		default:
			return fmt.Errorf("stage %q: invalid on_fail %q", s.Name, s.OnFail)
		}
	}
	return nil
}
