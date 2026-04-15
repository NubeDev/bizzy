package tests

import (
	"encoding/json"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestQAConversational tests the full conversational Q&A flow:
// connect → receive session → send flow → answer questions one by one → get result.
func TestQAConversational(t *testing.T) {
	token := os.Getenv("NUBE_TOKEN")
	if token == "" {
		token = "b0a8bd324bc82d2df34f5fa404a56c10302a8037dc1c1e028bbe60b81677d377"
	}

	u := url.URL{Scheme: "ws", Host: "localhost:8090", Path: "/api/agents/qa", RawQuery: "token=" + token}
	t.Logf("connecting to %s", u.String())

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(3 * time.Minute))

	// Step 1: Read session event.
	var ev map[string]any
	conn.ReadJSON(&ev)
	if ev["type"] != "session" {
		t.Fatalf("expected session, got: %v", ev)
	}
	sessionID := ev["session_id"].(string)
	t.Logf("session: %s", sessionID)

	// Step 2: Send the flow request.
	conn.WriteJSON(map[string]string{"flow": "nube-marketing.marketing_plan_qa"})

	// Step 3: Answer questions as they come.
	answers := map[string]string{
		"product":  "Rubix Edge Controller",
		"audience": "Systems integrators",
		"budget":   "$10k - $50k",
		"timeline": "1 quarter",
		"channels": "linkedin,email",
		"notes":    "Focus on AU market",
	}

	questionsAsked := 0
	var resultText string
	var gotGenerating, gotDone bool

	for {
		var event map[string]any
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Logf("connection closed: %v", err)
			break
		}
		json.Unmarshal(msg, &event)

		evType, _ := event["type"].(string)
		t.Logf("  %s", formatEvent(event))

		switch evType {
		case "question":
			questionsAsked++
			field, _ := event["field"].(string)
			answer, ok := answers[field]
			if !ok {
				answer = "skip" // default for unexpected questions
			}
			t.Logf("    → answering %s: %q", field, answer)
			conn.WriteJSON(map[string]any{"answer": answer})

		case "generating":
			gotGenerating = true

		case "connected":
			// Claude connected

		case "tool_call":
			// Claude calling a tool

		case "text":
			content, _ := event["content"].(string)
			resultText += content

		case "done":
			gotDone = true

		case "error":
			t.Fatalf("error: %s", event["error"])
		}

		if evType == "done" {
			break
		}
	}

	t.Logf("questions asked: %d", questionsAsked)
	t.Logf("result length: %d chars", len(resultText))

	if questionsAsked < 4 {
		t.Errorf("expected at least 4 questions, got %d", questionsAsked)
	}
	if !gotGenerating {
		t.Error("never received generating event")
	}
	if !gotDone {
		t.Error("never received done event")
	}
	if resultText == "" {
		t.Error("empty result text")
	}

	t.Logf("result preview: %s", truncate(resultText, 200))
}

func formatEvent(ev map[string]any) string {
	evType, _ := ev["type"].(string)
	switch evType {
	case "question":
		return "question: " + str(ev["field"]) + " — " + str(ev["label"])
	case "generating":
		return "generating: " + str(ev["message"])
	case "connected":
		return "connected: " + str(ev["model"])
	case "tool_call":
		return "tool_call: " + str(ev["name"])
	case "text":
		return "text: " + truncate(str(ev["content"]), 80)
	case "done":
		return "done"
	case "error":
		return "error: " + str(ev["error"])
	default:
		return evType
	}
}

func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
