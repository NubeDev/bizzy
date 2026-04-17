package jsondb

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ConfigFile is a thread-safe, file-backed single-value JSON store.
// Unlike Collection (which stores a list), ConfigFile stores one value.
type ConfigFile[T any] struct {
	mu       sync.RWMutex
	filePath string
	value    T
}

// NewConfigFile loads a config from a JSON file, or uses defaultVal if the file doesn't exist.
func NewConfigFile[T any](filePath string, defaultVal T) (*ConfigFile[T], error) {
	c := &ConfigFile[T]{filePath: filePath, value: defaultVal}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil // use default
		}
		return nil, fmt.Errorf("configfile: read %s: %w", filePath, err)
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &c.value); err != nil {
			return nil, fmt.Errorf("configfile: unmarshal %s: %w", filePath, err)
		}
	}
	return c, nil
}

// Get returns the current value.
func (c *ConfigFile[T]) Get() T {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.value
}

// Set replaces the value and flushes to disk.
func (c *ConfigFile[T]) Set(val T) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value = val
	return c.flush()
}

func (c *ConfigFile[T]) flush() error {
	data, err := json.MarshalIndent(c.value, "", "  ")
	if err != nil {
		return fmt.Errorf("configfile: marshal: %w", err)
	}
	dir := filepath.Dir(c.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("configfile: mkdir %s: %w", dir, err)
	}
	tmp := c.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("configfile: write tmp: %w", err)
	}
	if err := os.Rename(tmp, c.filePath); err != nil {
		return fmt.Errorf("configfile: rename: %w", err)
	}
	return nil
}
