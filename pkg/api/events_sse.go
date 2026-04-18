package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/NubeDev/bizzy/pkg/auth"
	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go"
)

// handleEventStream serves an SSE stream of bus events filtered by user.
//
//	GET /api/events/stream?topics=workflow.>,job.>
//
// Events are user-scoped: the authenticated user only sees events for their
// own commands/workflows/jobs. Admins can pass ?all=true for the full stream.
func (a *API) handleEventStream(c *gin.Context) {
	if a.CmdRouter == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "command bus not configured"})
		return
	}

	user := auth.GetUser(c)
	isAdmin := user.Role == "admin"
	showAll := c.Query("all") == "true" && isAdmin

	// Parse requested topics (default: all).
	topicsParam := c.DefaultQuery("topics", "command.>,workflow.>,job.>,tool.>")
	topics := strings.Split(topicsParam, ",")

	// Set SSE headers.
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Flush()

	bus := a.CmdRouter.Bus()
	ctx := c.Request.Context()

	// Subscribe to each topic pattern.
	var subs []*nats.Subscription
	for _, topic := range topics {
		topic = strings.TrimSpace(topic)
		if topic == "" {
			continue
		}

		sub, err := bus.Subscribe(topic, func(msg *nats.Msg) {
			// Filter by user unless admin with ?all=true.
			if !showAll {
				var envelope struct {
					UserID string `json:"user_id"`
				}
				json.Unmarshal(msg.Data, &envelope)

				// Also check nested command.user_id.
				if envelope.UserID == "" {
					var cmdEnvelope struct {
						Command struct {
							UserID string `json:"user_id"`
						} `json:"command"`
					}
					json.Unmarshal(msg.Data, &cmdEnvelope)
					envelope.UserID = cmdEnvelope.Command.UserID
				}

				if envelope.UserID != "" && envelope.UserID != user.ID {
					msg.Ack()
					return
				}
			}

			data := fmt.Sprintf("data: {\"topic\":%q,\"data\":%s}\n\n",
				msg.Subject, string(msg.Data))
			c.Writer.WriteString(data)
			c.Writer.Flush()
			msg.Ack()
		})
		if err != nil {
			continue
		}
		subs = append(subs, sub)
	}

	// Wait until client disconnects.
	<-ctx.Done()

	// Cleanup subscriptions.
	for _, sub := range subs {
		sub.Unsubscribe()
	}
}
