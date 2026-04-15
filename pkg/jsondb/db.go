// Package jsondb provides a simple file-backed JSON database with in-memory caching.
package jsondb

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Record is the interface that all stored items must implement.
type Record interface {
	GetID() string
}

// Collection is a typed, thread-safe, file-backed JSON collection.
type Collection[T Record] struct {
	mu       sync.RWMutex
	filePath string
	items    []T
}

// NewCollection creates or loads a collection from a JSON file.
func NewCollection[T Record](filePath string) (*Collection[T], error) {
	c := &Collection[T]{filePath: filePath}
	if err := c.load(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Collection[T]) load() error {
	data, err := os.ReadFile(c.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			c.items = []T{}
			return nil
		}
		return fmt.Errorf("jsondb: read %s: %w", c.filePath, err)
	}
	if len(data) == 0 {
		c.items = []T{}
		return nil
	}
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return fmt.Errorf("jsondb: unmarshal %s: %w", c.filePath, err)
	}
	c.items = items
	return nil
}

func (c *Collection[T]) flush() error {
	data, err := json.MarshalIndent(c.items, "", "  ")
	if err != nil {
		return fmt.Errorf("jsondb: marshal: %w", err)
	}
	dir := filepath.Dir(c.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("jsondb: mkdir %s: %w", dir, err)
	}
	tmp := c.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("jsondb: write tmp: %w", err)
	}
	if err := os.Rename(tmp, c.filePath); err != nil {
		return fmt.Errorf("jsondb: rename: %w", err)
	}
	return nil
}

// All returns a copy of all items.
func (c *Collection[T]) All() []T {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]T, len(c.items))
	copy(out, c.items)
	return out
}

// Get returns the item with the given ID, or false if not found.
func (c *Collection[T]) Get(id string) (T, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, item := range c.items {
		if item.GetID() == id {
			return item, true
		}
	}
	var zero T
	return zero, false
}

// FindFunc returns items matching the predicate.
func (c *Collection[T]) FindFunc(fn func(T) bool) []T {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var out []T
	for _, item := range c.items {
		if fn(item) {
			out = append(out, item)
		}
	}
	return out
}

// FindOne returns the first item matching the predicate, or false if none.
func (c *Collection[T]) FindOne(fn func(T) bool) (T, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, item := range c.items {
		if fn(item) {
			return item, true
		}
	}
	var zero T
	return zero, false
}

// Create adds a new item. Returns error if the ID already exists.
func (c *Collection[T]) Create(item T) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, existing := range c.items {
		if existing.GetID() == item.GetID() {
			return fmt.Errorf("jsondb: duplicate id %q", item.GetID())
		}
	}
	c.items = append(c.items, item)
	return c.flush()
}

// Update replaces the item with the matching ID. Returns error if not found.
func (c *Collection[T]) Update(item T) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, existing := range c.items {
		if existing.GetID() == item.GetID() {
			c.items[i] = item
			return c.flush()
		}
	}
	return fmt.Errorf("jsondb: not found %q", item.GetID())
}

// Delete removes the item with the given ID. Returns error if not found.
func (c *Collection[T]) Delete(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, item := range c.items {
		if item.GetID() == id {
			c.items = append(c.items[:i], c.items[i+1:]...)
			return c.flush()
		}
	}
	return fmt.Errorf("jsondb: not found %q", id)
}

// Count returns the number of items.
func (c *Collection[T]) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}
