package tests

import (
	"encoding/json"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestAgentWSWithTools tests a WebSocket run that triggers MCP tool calls.
func TestAgentWSWithTools(t *testing.T) {
	token := os.Getenv("NUBE_TOKEN")
	if token == "" {
		token = "b0a8bd324bc82d2df34f5fa404a56c10302a8037dc1c1e028bbe60b81677d377"
	}

	u := url.URL{Scheme: "ws", Host: "localhost:8090", Path: "/api/agents/run", RawQuery: "token=" + token}

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Read session.
	var session map[string]any
	conn.ReadJSON(&session)
	sessionID := session["session_id"].(string)
	t.Logf("session: %s", sessionID)

	// Send request scoped to rubix-developer.
	conn.WriteJSON(map[string]string{
		"prompt": "use the device_summary tool and tell me how many devices there are, one line answer",
		"agent":  "rubix-developer",
	})

	conn.SetReadDeadline(time.Now().Add(2 * time.Minute))

	var toolCalls []string
	var resultText string

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var event map[string]any
		json.Unmarshal(msg, &event)

		evType, _ := event["type"].(string)

		switch evType {
		case "connected":
			t.Logf("  connected: %s", event["model"])
		case "tool_call":
			name, _ := event["name"].(string)
			toolCalls = append(toolCalls, name)
			t.Logf("  tool_call: %s", name)
		case "text":
			content, _ := event["content"].(string)
			resultText += content
		case "done":
			t.Logf("  done: %vms", event["duration_ms"])
		case "error":
			t.Errorf("  error: %s", event["error"])
		}

		if evType == "done" || evType == "error" {
			break
		}
	}

	t.Logf("result: %s", truncate(resultText, 200))
	t.Logf("tool calls: %v", toolCalls)

	if len(toolCalls) == 0 {
		t.Error("expected at least one tool call")
	}
	if resultText == "" {
		t.Error("expected non-empty result text")
	}
}
