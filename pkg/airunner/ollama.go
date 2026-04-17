package airunner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// OllamaRunner drives the Ollama local LLM server via its native /api/chat
// endpoint (not the OpenAI-compat /v1 endpoint).
//
// Config:
//   - OLLAMA_HOST env var (default http://localhost:11434)
//
// Tool calling: when MCPURL and MCPToken are set in RunConfig, the runner
// connects to the MCP server, fetches available tools, and runs a server-side
// agent loop — sending tools to Ollama, executing tool calls, and feeding
// results back until the model produces a final text response.
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

// --- Ollama API types ---

// ollamaChatRequest is the JSON body for POST /api/chat.
type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Tools    []ollamaTool    `json:"tools,omitempty"`
}

type ollamaMessage struct {
	Role      string           `json:"role"`    // "system", "user", "assistant", "tool"
	Content   string           `json:"content"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}

// ollamaTool is the Ollama function-calling tool schema.
type ollamaTool struct {
	Type     string             `json:"type"` // "function"
	Function ollamaToolFunction `json:"function"`
}

type ollamaToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// ollamaToolCall is a tool invocation returned by the model.
type ollamaToolCall struct {
	Function struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	} `json:"function"`
}

// ollamaChatChunk is a single line of the streaming response.
type ollamaChatChunk struct {
	Message struct {
		Role      string           `json:"role"`
		Content   string           `json:"content"`
		ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
	} `json:"message"`
	Done            bool `json:"done"`
	TotalDuration   int64 `json:"total_duration,omitempty"` // nanoseconds
	PromptEvalCount int   `json:"prompt_eval_count,omitempty"`
	EvalCount       int   `json:"eval_count,omitempty"`
}

// --- Run ---

const maxAgentSteps = 10

func (r *OllamaRunner) Run(ctx context.Context, cfg RunConfig, sessionID string, onEvent func(Event)) RunResult {
	var result RunResult
	result.Provider = string(ProviderOllama)

	model := cfg.Model
	if model == "" {
		model = "gemma3" // sensible default
	}
	result.Model = model

	// Build initial messages.
	var messages []ollamaMessage
	if cfg.SystemPrompt != "" {
		messages = append(messages, ollamaMessage{Role: "system", Content: cfg.SystemPrompt})
	}
	messages = append(messages, ollamaMessage{Role: "user", Content: cfg.Prompt})

	// Try to connect to MCP and fetch tools for the agent loop.
	var tools []ollamaTool
	var mcpClient *mcpToolClient
	if cfg.MCPURL != "" && cfg.MCPToken != "" {
		client, err := newMCPToolClient(ctx, cfg.MCPURL, cfg.MCPToken)
		if err != nil {
			log.Printf("[ollama] MCP client connect failed (tools disabled): %v", err)
		} else {
			mcpClient = client
			defer mcpClient.close()

			mcpTools, err := mcpClient.listTools(ctx)
			if err != nil {
				log.Printf("[ollama] MCP list tools failed (tools disabled): %v", err)
			} else {
				tools = convertMCPToOllamaTools(mcpTools)
				log.Printf("[ollama] loaded %d tools from MCP", len(tools))
			}
		}
	}

	start := time.Now()

	// Agent loop: send prompt with tools, execute tool calls, feed back results.
	for step := 0; step < maxAgentSteps; step++ {
		text, toolCalls, inputTok, outputTok, err := r.chat(ctx, model, messages, tools, sessionID, onEvent)
		if err != nil {
			return result
		}

		result.InputTokens += inputTok
		result.OutputTokens += outputTok

		// No tool calls → final text response, we're done.
		if len(toolCalls) == 0 {
			result.Text = text
			break
		}

		// Add assistant message with tool calls to history.
		messages = append(messages, ollamaMessage{
			Role:      "assistant",
			ToolCalls: toolCalls,
		})

		// Execute each tool call via MCP.
		for _, tc := range toolCalls {
			toolName := tc.Function.Name
			onEvent(Event{
				Type:      "tool_call",
				Provider:  string(ProviderOllama),
				SessionID: sessionID,
				Name:      toolName,
			})

			entry := ToolCallEntry{Name: toolName, Status: "ok"}
			toolStart := time.Now()

			argsJSON, _ := json.Marshal(tc.Function.Arguments)
			entry.InputBytes = len(argsJSON)

			toolResult, callErr := mcpClient.callTool(ctx, toolName, argsJSON)

			entry.DurationMS = int(time.Since(toolStart).Milliseconds())
			if callErr != nil {
				entry.Status = "error"
				entry.Error = callErr.Error()
				toolResult = "error: " + callErr.Error()
			} else {
				entry.OutputBytes = len(toolResult)
			}

			result.ToolCallLog = append(result.ToolCallLog, entry)
			result.ToolCalls++

			// Feed tool result back to Ollama.
			messages = append(messages, ollamaMessage{
				Role:    "tool",
				Content: toolResult,
			})
		}
	}

	elapsed := time.Since(start)
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

// chat performs a single streaming Ollama /api/chat request.
// It returns the accumulated text, any tool calls, and token counts.
// Text chunks are emitted as "text" events via onEvent.
func (r *OllamaRunner) chat(
	ctx context.Context,
	model string,
	messages []ollamaMessage,
	tools []ollamaTool,
	sessionID string,
	onEvent func(Event),
) (text string, toolCalls []ollamaToolCall, inputTokens, outputTokens int, err error) {

	body, _ := json.Marshal(ollamaChatRequest{
		Model:    model,
		Messages: messages,
		Stream:   true,
		Tools:    tools,
	})

	req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, r.getHost()+"/api/chat", bytes.NewReader(body))
	if reqErr != nil {
		onEvent(Event{Type: "error", Provider: string(ProviderOllama), SessionID: sessionID,
			Error: "build request: " + reqErr.Error()})
		return "", nil, 0, 0, reqErr
	}
	req.Header.Set("Content-Type", "application/json")

	resp, respErr := http.DefaultClient.Do(req)
	if respErr != nil {
		onEvent(Event{Type: "error", Provider: string(ProviderOllama), SessionID: sessionID,
			Error: "ollama request: " + respErr.Error()})
		return "", nil, 0, 0, respErr
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		msg := fmt.Sprintf("ollama HTTP %d: %s", resp.StatusCode, string(errBody))
		onEvent(Event{Type: "error", Provider: string(ProviderOllama), SessionID: sessionID, Error: msg})
		return "", nil, 0, 0, fmt.Errorf("%s", msg)
	}

	// Emit connected on the first iteration only (caller can track this).
	onEvent(Event{Type: "connected", Provider: string(ProviderOllama), SessionID: sessionID, Model: model})

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)

	var textBuf bytes.Buffer

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var chunk ollamaChatChunk
		if jsonErr := json.Unmarshal(line, &chunk); jsonErr != nil {
			continue
		}

		// Collect tool calls from the response.
		if len(chunk.Message.ToolCalls) > 0 {
			toolCalls = append(toolCalls, chunk.Message.ToolCalls...)
		}

		// Emit text chunks.
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
			inputTokens = chunk.PromptEvalCount
			outputTokens = chunk.EvalCount
			break
		}
	}

	return textBuf.String(), toolCalls, inputTokens, outputTokens, nil
}

// convertMCPToOllamaTools converts MCP tool definitions to Ollama's function-calling format.
func convertMCPToOllamaTools(mcpTools []*mcp.Tool) []ollamaTool {
	tools := make([]ollamaTool, 0, len(mcpTools))
	for _, t := range mcpTools {
		// Marshal the InputSchema to JSON — it's already a JSON Schema object.
		params, err := json.Marshal(t.InputSchema)
		if err != nil {
			log.Printf("[ollama] skip tool %s: cannot marshal schema: %v", t.Name, err)
			continue
		}

		tools = append(tools, ollamaTool{
			Type: "function",
			Function: ollamaToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  params,
			},
		})
	}
	return tools
}
