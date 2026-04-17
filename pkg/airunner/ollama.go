package airunner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// OllamaRunner drives the Ollama local LLM server via its native /api/chat
// endpoint (not the OpenAI-compat /v1 endpoint).
//
// Config:
//   - OLLAMA_HOST env var (default http://localhost:11434)
//
// Phase 2: text chat only, no tool calling.
type OllamaRunner struct {
	host string // resolved once in Available()
}

func (r *OllamaRunner) Name() Provider { return ProviderOllama }

// Configure sets the Ollama host from the admin config.
func (r *OllamaRunner) Configure(host, _ string) {
	if host != "" {
		r.host = host
	}
}

func (r *OllamaRunner) getHost() string {
	if r.host != "" {
		return r.host
	}
	if h := os.Getenv("OLLAMA_HOST"); h != "" {
		r.host = h
	} else {
		r.host = "http://localhost:11434"
	}
	return r.host
}

// Available checks if Ollama is reachable by hitting GET /api/tags.
func (r *OllamaRunner) Available() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.getHost()+"/api/tags", nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// InstalledModels returns the list of model names installed in Ollama.
func (r *OllamaRunner) InstalledModels() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.getHost()+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var body struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	names := make([]string, len(body.Models))
	for i, m := range body.Models {
		names[i] = m.Name
	}
	return names, nil
}

// ollamaChatRequest is the JSON body for POST /api/chat.
type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ollamaChatChunk is a single line of the streaming response.
type ollamaChatChunk struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Done              bool    `json:"done"`
	TotalDuration     int64   `json:"total_duration,omitempty"`      // nanoseconds
	PromptEvalCount   int     `json:"prompt_eval_count,omitempty"`
	EvalCount         int     `json:"eval_count,omitempty"`
}

func (r *OllamaRunner) Run(ctx context.Context, cfg RunConfig, sessionID string, onEvent func(Event)) RunResult {
	var result RunResult
	result.Provider = string(ProviderOllama)

	model := cfg.Model
	if model == "" {
		model = "gemma3" // sensible default
	}
	result.Model = model

	// Build request body.
	body, _ := json.Marshal(ollamaChatRequest{
		Model: model,
		Messages: []ollamaMessage{
			{Role: "user", Content: cfg.Prompt},
		},
		Stream: true,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.getHost()+"/api/chat", bytes.NewReader(body))
	if err != nil {
		onEvent(Event{Type: "error", Provider: string(ProviderOllama), SessionID: sessionID,
			Error: "build request: " + err.Error()})
		return result
	}
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		onEvent(Event{Type: "error", Provider: string(ProviderOllama), SessionID: sessionID,
			Error: "ollama request: " + err.Error()})
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		onEvent(Event{Type: "error", Provider: string(ProviderOllama), SessionID: sessionID,
			Error: fmt.Sprintf("ollama HTTP %d: %s", resp.StatusCode, string(errBody))})
		return result
	}

	onEvent(Event{Type: "connected", Provider: string(ProviderOllama), SessionID: sessionID, Model: model})

	// Stream newline-delimited JSON chunks.
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)

	var textBuf bytes.Buffer

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var chunk ollamaChatChunk
		if err := json.Unmarshal(line, &chunk); err != nil {
			continue
		}

		if chunk.Message.Content != "" {
			textBuf.WriteString(chunk.Message.Content)
			onEvent(Event{
				Type:      "text",
				Provider:  string(ProviderOllama),
				SessionID: sessionID,
				Content:   chunk.Message.Content,
			})
		}

		if chunk.Done {
			result.InputTokens = chunk.PromptEvalCount
			result.OutputTokens = chunk.EvalCount
			break
		}
	}

	elapsed := time.Since(start)
	result.Text = textBuf.String()
	result.DurationMS = int(elapsed.Milliseconds())
	result.CostUSD = 0 // local, free

	onEvent(Event{
		Type:       "done",
		Provider:   string(ProviderOllama),
		SessionID:  sessionID,
		DurationMS: result.DurationMS,
	})

	return result
}
