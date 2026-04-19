package api

import (
	"encoding/json"
	"log"
	"strings"
	"sync"

	"github.com/NubeDev/bizzy/pkg/ws"
	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go"
)

// handleEventWS upgrades to WebSocket and serves real-time bus events.
// Replaces the SSE endpoint with a bidirectional protocol:
//
//	WS /api/events/ws?token=<auth>
//
//	Client → Server (subscribe):
//	  {"subscribe": "flow.>"}
//	  {"subscribe": "job.>", "filter": {"flow_id": "flow-abc"}}
//
//	Client → Server (unsubscribe):
//	  {"unsubscribe": "flow.>"}
//
//	Server → Client (events):
//	  {"topic": "flow.node.completed", "data": {...}}
//
// Events are user-scoped: the authenticated user only sees events for their
// own flows/commands/jobs. Admins can pass ?all=true for the full stream.
func (a *API) handleEventWS(c *gin.Context) {
	if a.CmdRouter == nil {
		c.JSON(503, gin.H{"error": "event bus not configured"})
		return
	}

	user, ok := ws.AuthFromQuery(c, a.DB)
	if !ok {
		return
	}
	isAdmin := user.Role == "admin"
	showAll := c.Query("all") == "true" && isAdmin

	conn, err := ws.Upgrade(c.Writer, c.Request)
	if err != nil {
		log.Printf("[events-ws] upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	bus := a.CmdRouter.Bus()
	userID := user.ID

	// Track active subscriptions so we can clean up on unsubscribe or disconnect.
	var mu sync.Mutex
	subs := make(map[string]*nats.Subscription) // topic pattern → subscription

	defer func() {
		mu.Lock()
		for _, sub := range subs {
			sub.Unsubscribe()
		}
		mu.Unlock()
	}()

	// eventMsg is what we send to the client.
	type eventMsg struct {
		Topic string          `json:"topic"`
		Data  json.RawMessage `json:"data"`
	}

	// subscribe creates a NATS subscription for a topic pattern and forwards
	// matching events to the WS client (filtered by user_id).
	subscribe := func(pattern string, filter map[string]any) {
		mu.Lock()
		if _, exists := subs[pattern]; exists {
			mu.Unlock()
			return // already subscribed
		}
		mu.Unlock()

		sub, err := bus.Subscribe(pattern, func(msg *nats.Msg) {
			// User-scope filtering.
			if !showAll {
				var envelope struct {
					UserID string `json:"user_id"`
				}
				json.Unmarshal(msg.Data, &envelope)
				if envelope.UserID == "" {
					var cmdEnvelope struct {
						Command struct {
							UserID string `json:"user_id"`
						} `json:"command"`
					}
					json.Unmarshal(msg.Data, &cmdEnvelope)
					envelope.UserID = cmdEnvelope.Command.UserID
				}
				if envelope.UserID != "" && envelope.UserID != userID {
					msg.Ack()
					return
				}
			}

			// Optional field-level filter (e.g. {"flow_id": "flow-abc"}).
			if len(filter) > 0 {
				var data map[string]any
				if json.Unmarshal(msg.Data, &data) == nil {
					match := true
					for k, v := range filter {
						if data[k] != v {
							match = false
							break
						}
					}
					if !match {
						msg.Ack()
						return
					}
				}
			}

			conn.WriteJSON(eventMsg{
				Topic: msg.Subject,
				Data:  msg.Data,
			})
			msg.Ack()
		})
		if err != nil {
			log.Printf("[events-ws] subscribe %s failed: %v", pattern, err)
			return
		}

		mu.Lock()
		subs[pattern] = sub
		mu.Unlock()
	}

	unsubscribe := func(pattern string) {
		mu.Lock()
		if sub, ok := subs[pattern]; ok {
			sub.Unsubscribe()
			delete(subs, pattern)
		}
		mu.Unlock()
	}

	// Read loop — process subscribe/unsubscribe messages.
	for {
		var msg struct {
			Subscribe   string         `json:"subscribe,omitempty"`
			Unsubscribe string         `json:"unsubscribe,omitempty"`
			Filter      map[string]any `json:"filter,omitempty"`
		}
		if err := conn.ReadJSON(&msg); err != nil {
			// Client disconnected or error — done.
			break
		}

		if topic := strings.TrimSpace(msg.Subscribe); topic != "" {
			subscribe(topic, msg.Filter)
		}
		if topic := strings.TrimSpace(msg.Unsubscribe); topic != "" {
			unsubscribe(topic)
		}
	}
}
