package tests

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestAgentWSRun tests the WebSocket agent run endpoint end-to-end.
// Requires a running nube-server on :8090 with a valid token.
//
// Run with: go test -v -run TestAgentWSRun ./tests/
func TestAgentWSRun(t *testing.T) {
	token := os.Getenv("NUBE_TOKEN")
	if token == "" {
		token = "b0a8bd324bc82d2df34f5fa404a56c10302a8037dc1c1e028bbe60b81677d377"
	}

	u := url.URL{Scheme: "ws", Host: "localhost:8090", Path: "/api/agents/run", RawQuery: "token=" + token}
	t.Logf("connecting to %s", u.String())

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Step 1: Read session event.
	var sessionEvent map[string]any
	if err := conn.ReadJSON(&sessionEvent); err != nil {
		t.Fatalf("read session: %v", err)
	}
	if sessionEvent["type"] != "session" {
		t.Fatalf("expected session event, got: %v", sessionEvent)
	}
	sessionID := sessionEvent["session_id"].(string)
	t.Logf("session: %s", sessionID)

	// Step 2: Send run request.
	req := map[string]string{
		"prompt": "say hello in one sentence",
	}
	if err := conn.WriteJSON(req); err != nil {
		t.Fatalf("write request: %v", err)
	}

	// Step 3: Read events until done.
	conn.SetReadDeadline(time.Now().Add(2 * time.Minute))

	var gotConnected, gotText, gotDone bool
	var resultText string

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			// Normal close or EOF after done.
			break
		}

		var event map[string]any
		if err := json.Unmarshal(msg, &event); err != nil {
			t.Logf("invalid json: %s", msg)
			continue
		}

		evType, _ := event["type"].(string)
		evSession, _ := event["session_id"].(string)

		// Every event must carry the session ID.
		if evSession != sessionID {
			t.Errorf("event %s has session_id=%q, want %q", evType, evSession, sessionID)
		}

		switch evType {
		case "connected":
			gotConnected = true
			t.Logf("  connected: model=%s", event["model"])
		case "tool_call":
			t.Logf("  tool_call: %s", event["name"])
		case "text":
			content, _ := event["content"].(string)
			resultText += content
			t.Logf("  text: %s", truncate(content, 80))
		case "done":
			gotDone = true
			t.Logf("  done: %vms, $%.4f", event["duration_ms"], event["cost_usd"])
		case "error":
			t.Fatalf("  error: %s", event["error"])
		}

		if evType == "done" {
			break
		}
	}

	if !gotConnected {
		t.Error("never received connected event")
	}
	if !gotText {
		// Text might be empty for very short responses combined in one event.
		if resultText == "" {
			t.Error("never received text content")
		}
	}
	if !gotDone {
		t.Error("never received done event")
	}

	t.Logf("result: %s", truncate(resultText, 200))

	// Step 4: Verify session appears in history.
	fmt.Printf("\nSession ID: %s\n", sessionID)
	fmt.Printf("Verify with: curl -s -H 'Authorization: Bearer %s' http://localhost:8090/api/agents/sessions/%s\n", token, sessionID)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
