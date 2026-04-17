// Package memory provides a simple file-backed memory store for persistent
// AI context. Two scopes: server-wide (shared) and per-user (private).
package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Store manages server and per-user memory files.
type Store struct {
	mu      sync.RWMutex
	dataDir string // e.g. data/memory/
}

// NewStore creates a memory store rooted at dataDir/memory/.
func NewStore(dataDir string) *Store {
	dir := filepath.Join(dataDir, "memory")
	os.MkdirAll(dir, 0755)
	os.MkdirAll(filepath.Join(dir, "users"), 0755)
	return &Store{dataDir: dir}
}

// GetServer returns the server-wide memory.
func (s *Store) GetServer() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.readFile("server.md")
}

// SetServer replaces the server-wide memory.
func (s *Store) SetServer(content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeFile("server.md", content)
}

// GetUser returns a user's memory.
func (s *Store) GetUser(userID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.readFile(filepath.Join("users", userID+".md"))
}

// SetUser replaces a user's memory.
func (s *Store) SetUser(userID, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeFile(filepath.Join("users", userID+".md"), content)
}

// AppendUser appends a line to a user's memory.
func (s *Store) AppendUser(userID, line string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing := s.readFile(filepath.Join("users", userID+".md"))
	if existing != "" && !strings.HasSuffix(existing, "\n") {
		existing += "\n"
	}
	return s.writeFile(filepath.Join("users", userID+".md"), existing+line+"\n")
}

// BuildPromptPrefix builds the memory prefix to prepend to a prompt.
// Returns empty string if no memory exists.
func (s *Store) BuildPromptPrefix(userID string) string {
	server := s.GetServer()
	user := s.GetUser(userID)

	if server == "" && user == "" {
		return ""
	}

	var b strings.Builder
	if server != "" {
		b.WriteString("[Server Memory]\n")
		b.WriteString(strings.TrimSpace(server))
		b.WriteString("\n\n")
	}
	if user != "" {
		b.WriteString("[User Memory]\n")
		b.WriteString(strings.TrimSpace(user))
		b.WriteString("\n\n")
	}
	return b.String()
}

func (s *Store) readFile(rel string) string {
	data, err := os.ReadFile(filepath.Join(s.dataDir, rel))
	if err != nil {
		return ""
	}
	return string(data)
}

func (s *Store) writeFile(rel string, content string) error {
	path := filepath.Join(s.dataDir, rel)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("memory: mkdir: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0644); err != nil {
		return fmt.Errorf("memory: write: %w", err)
	}
	return os.Rename(tmp, path)
}
