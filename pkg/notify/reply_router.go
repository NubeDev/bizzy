// Package notify subscribes to bus events and routes replies back through
// the originating adapter's reply channel.
package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/NubeDev/bizzy/pkg/bus"
	"github.com/NubeDev/bizzy/pkg/command"
	"github.com/nats-io/nats.go"
)

// ReplyRouter watches for completion events and sends results back through
// the originating adapter's reply channel.
type ReplyRouter struct {
	bus      *bus.Bus
	adapters command.AdapterRegistry
}

// NewReplyRouter creates a reply router.
func NewReplyRouter(b *bus.Bus, adapters command.AdapterRegistry) *ReplyRouter {
	return &ReplyRouter{bus: b, adapters: adapters}
}

// Start subscribes to relevant bus events and begins routing replies.
func (r *ReplyRouter) Start() error {
	// Sync commands — immediate result.
	if _, err := r.bus.SubscribeDurable(bus.TopicCommandCompleted, "reply-router-sync", func(msg *nats.Msg) {
		var ev command.CommandResultEvent
		if err := json.Unmarshal(msg.Data, &ev); err != nil {
			msg.Ack()
			return
		}

		r.sendReply(ev.Command.ReplyTo, formatResult(ev.Result))
		msg.Ack()
	}); err != nil {
		return fmt.Errorf("subscribe command.completed: %w", err)
	}

	// Async commands — ack immediately, real result comes later.
	if _, err := r.bus.SubscribeDurable(bus.TopicCommandAccepted, "reply-router-async", func(msg *nats.Msg) {
		var ev command.CommandResultEvent
		if err := json.Unmarshal(msg.Data, &ev); err != nil {
			msg.Ack()
			return
		}

		text := fmt.Sprintf("Started %s (%s)", ev.Result.Message, ev.Result.ID)
		r.sendReply(ev.Command.ReplyTo, text)
		msg.Ack()
	}); err != nil {
		return fmt.Errorf("subscribe command.accepted: %w", err)
	}

	// Command errors.
	if _, err := r.bus.SubscribeDurable(bus.TopicCommandFailed, "reply-router-err", func(msg *nats.Msg) {
		var ev command.CommandResultEvent
		if err := json.Unmarshal(msg.Data, &ev); err != nil {
			msg.Ack()
			return
		}

		r.sendReply(ev.Command.ReplyTo, "Error: "+ev.Error)
		msg.Ack()
	}); err != nil {
		return fmt.Errorf("subscribe command.failed: %w", err)
	}

	log.Println("[reply-router] started")
	return nil
}

func (r *ReplyRouter) sendReply(replyTo command.ReplyInfo, text string) {
	if replyTo.Channel == "" {
		return
	}

	ch, err := r.adapters.BuildReply(replyTo)
	if err != nil {
		log.Printf("[reply-router] reply dropped: channel=%s err=%v", replyTo.Channel, err)
		return
	}

	if err := ch.Send(context.Background(), command.ReplyMessage{Text: text}); err != nil {
		log.Printf("[reply-router] send failed: channel=%s err=%v", replyTo.Channel, err)
	}
}

func formatResult(r command.Result) string {
	if r.Message != "" {
		return r.Message
	}
	if r.Output != nil {
		if s, ok := r.Output.(string); ok {
			return s
		}
		b, _ := json.MarshalIndent(r.Output, "", "  ")
		return string(b)
	}
	return "Done"
}
