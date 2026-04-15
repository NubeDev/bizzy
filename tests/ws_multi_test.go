package tests

import (
	"encoding/json"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestAgentWSConcurrent runs two WebSocket sessions in parallel
// and verifies they get distinct session IDs and don't cross-contaminate.
func TestAgentWSConcurrent(t *testing.T) {
	token := "b0a8bd324bc82d2df34f5fa404a56c10302a8037dc1c1e028bbe60b81677d377"

	type sessionResult struct {
		sessionID string
		events    []map[string]any
		err       error
	}

	runSession := func(prompt string) sessionResult {
		var res sessionResult

		u := url.URL{Scheme: "ws", Host: "localhost:8090", Path: "/api/agents/run", RawQuery: "token=" + token}
		conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		if err != nil {
			res.err = err
			return res
		}
		defer conn.Close()

		// Read session.
		var session map[string]any
		conn.ReadJSON(&session)
		res.sessionID = session["session_id"].(string)

		// Send prompt.
		conn.WriteJSON(map[string]string{"prompt": prompt})

		conn.SetReadDeadline(time.Now().Add(2 * time.Minute))

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				break
			}
			var event map[string]any
			json.Unmarshal(msg, &event)
			res.events = append(res.events, event)

			if event["type"] == "done" || event["type"] == "error" {
				break
			}
		}
		return res
	}

	var wg sync.WaitGroup
	results := make([]sessionResult, 2)

	// Run two sessions concurrently with different prompts.
	prompts := []string{
		"reply with just the word ALPHA",
		"reply with just the word BRAVO",
	}

	for i, prompt := range prompts {
		wg.Add(1)
		go func(idx int, p string) {
			defer wg.Done()
			results[idx] = runSession(p)
		}(i, prompt)
	}

	wg.Wait()

	// Verify both completed.
	for i, r := range results {
		if r.err != nil {
			t.Fatalf("session %d failed: %v", i, r.err)
		}
		t.Logf("session %d: %s (%d events)", i, r.sessionID, len(r.events))
	}

	// Verify distinct session IDs.
	if results[0].sessionID == results[1].sessionID {
		t.Errorf("both sessions got the same ID: %s", results[0].sessionID)
	}

	// Verify every event in each session carries the correct session ID.
	for i, r := range results {
		for _, ev := range r.events {
			evSID, _ := ev["session_id"].(string)
			if evSID != r.sessionID {
				t.Errorf("session %d: event %s has session_id=%q, want %q",
					i, ev["type"], evSID, r.sessionID)
			}
		}

		// Must have a done event.
		last := r.events[len(r.events)-1]
		if last["type"] != "done" {
			t.Errorf("session %d: last event is %s, want done", i, last["type"])
		}
	}

	t.Logf("session 0: %s", results[0].sessionID)
	t.Logf("session 1: %s", results[1].sessionID)
}
