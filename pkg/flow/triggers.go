package flow

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// triggerSchedule reads the "schedule" field from the trigger node's Data.
func triggerSchedule(def *FlowDef) string {
	data := def.TriggerConfig()
	if data == nil {
		return ""
	}
	s, _ := data["schedule"].(string)
	return s
}

// --- Cron Trigger ---

// CronTrigger fires a flow on a cron schedule (5-field: min hour dom month dow).
type CronTrigger struct {
	cancel context.CancelFunc
}

func (t *CronTrigger) Start(ctx context.Context, def *FlowDef, onTrigger func(inputs map[string]any)) error {
	schedule := triggerSchedule(def)
	if schedule == "" {
		return fmt.Errorf("cron trigger requires a schedule")
	}

	ctx, t.cancel = context.WithCancel(ctx)

	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				if matchesCron(schedule, now) {
					log.Printf("[flow-cron] %s: schedule matched, triggering", def.Name)
					onTrigger(map[string]any{
						"_trigger":   "cron",
						"_schedule":  schedule,
						"_timestamp": now.UTC().Format(time.RFC3339),
					})
				}
			}
		}
	}()

	log.Printf("[flow-cron] %s: started with schedule %q", def.Name, schedule)
	return nil
}

func (t *CronTrigger) Stop() error {
	if t.cancel != nil {
		t.cancel()
	}
	return nil
}

// --- Interval Trigger ---

// IntervalTrigger fires a flow on a fixed interval (e.g. "10s", "5m", "1h").
type IntervalTrigger struct {
	cancel context.CancelFunc
}

func (t *IntervalTrigger) Start(ctx context.Context, def *FlowDef, onTrigger func(inputs map[string]any)) error {
	schedule := triggerSchedule(def)
	if schedule == "" {
		return fmt.Errorf("interval trigger requires a schedule (e.g. '10s', '5m')")
	}

	dur, err := time.ParseDuration(schedule)
	if err != nil {
		return fmt.Errorf("invalid interval %q: %w", schedule, err)
	}
	if dur < 1*time.Second {
		return fmt.Errorf("interval must be at least 1s, got %s", dur)
	}

	ctx, t.cancel = context.WithCancel(ctx)

	go func() {
		ticker := time.NewTicker(dur)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				log.Printf("[flow-interval] %s: tick, triggering", def.Name)
				onTrigger(map[string]any{
					"_trigger":   "interval",
					"_interval":  schedule,
					"_timestamp": now.UTC().Format(time.RFC3339),
				})
			}
		}
	}()

	log.Printf("[flow-interval] %s: started with interval %s", def.Name, dur)
	return nil
}

func (t *IntervalTrigger) Stop() error {
	if t.cancel != nil {
		t.cancel()
	}
	return nil
}

// --- Cron matching ---
//
// Supports standard 5-field cron: minute hour dom month dow
// Each field supports: *, integers, ranges (1-5), lists (1,3,5),
// steps (*/5, 1-30/2), and combinations (1-5,10,15-20).

func matchesCron(expr string, t time.Time) bool {
	var minute, hour, dom, month, dow string
	_, err := fmt.Sscanf(expr, "%s %s %s %s %s", &minute, &hour, &dom, &month, &dow)
	if err != nil {
		return false
	}

	return fieldMatches(minute, t.Minute(), 0, 59) &&
		fieldMatches(hour, t.Hour(), 0, 23) &&
		fieldMatches(dom, t.Day(), 1, 31) &&
		fieldMatches(month, int(t.Month()), 1, 12) &&
		fieldMatches(dow, int(t.Weekday()), 0, 6)
}

// fieldMatches checks if value is matched by a cron field expression.
// min/max define the valid range for this field (used for */step expansion).
func fieldMatches(field string, value, min, max int) bool {
	if field == "*" {
		return true
	}
	// Split on comma for lists: "1,3,5" or "1-5,10"
	for _, part := range splitComma(field) {
		if partMatches(part, value, min, max) {
			return true
		}
	}
	return false
}

// partMatches handles a single element: "5", "1-5", "*/2", "1-30/2"
func partMatches(part string, value, min, max int) bool {
	// Check for step: "*/5" or "1-30/2"
	step := 1
	if idx := indexOf(part, '/'); idx >= 0 {
		s, ok := atoi(part[idx+1:])
		if !ok || s <= 0 {
			return false
		}
		step = s
		part = part[:idx]
	}

	// Now part is "*", "5", or "1-30"
	var lo, hi int
	if part == "*" {
		lo, hi = min, max
	} else if idx := indexOf(part, '-'); idx >= 0 {
		var okLo, okHi bool
		lo, okLo = atoi(part[:idx])
		hi, okHi = atoi(part[idx+1:])
		if !okLo || !okHi {
			return false
		}
	} else {
		v, ok := atoi(part)
		if !ok {
			return false
		}
		lo, hi = v, v
	}

	// Check if value falls in [lo, hi] at the given step.
	if value < lo || value > hi {
		return false
	}
	return (value-lo)%step == 0
}

// splitComma splits a string by commas without allocating for the common
// single-element case.
func splitComma(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	return append(parts, s[start:])
}

// indexOf returns the index of the first occurrence of c in s, or -1.
func indexOf(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// atoi parses a non-negative integer without importing strconv.
func atoi(s string) (int, bool) {
	if len(s) == 0 {
		return 0, false
	}
	n := 0
	for _, c := range []byte(s) {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}

// --- Webhook Trigger ---

// WebhookTrigger fires a flow when an HTTP request hits its registered path.
// The engine holds a map of active webhook handlers; the HTTP route delegates
// to Engine.HandleWebhook.
type WebhookTrigger struct {
	engine *Engine
	path   string
}

func (t *WebhookTrigger) Start(_ context.Context, def *FlowDef, onTrigger func(inputs map[string]any)) error {
	path := webhookPath(def)
	if path == "" {
		return fmt.Errorf("webhook trigger requires a 'path' in trigger config")
	}

	t.path = path

	t.engine.webhookMu.Lock()
	t.engine.webhooks[path] = onTrigger
	t.engine.webhookMu.Unlock()

	log.Printf("[flow-webhook] %s: registered at /hooks/flow/%s", def.Name, path)
	return nil
}

func (t *WebhookTrigger) Stop() error {
	if t.engine == nil || t.path == "" {
		return nil
	}
	t.engine.webhookMu.Lock()
	delete(t.engine.webhooks, t.path)
	t.engine.webhookMu.Unlock()
	return nil
}

func webhookPath(def *FlowDef) string {
	data := def.TriggerConfig()
	if data == nil {
		return ""
	}
	if p, ok := data["webhook_path"].(string); ok && p != "" {
		return p
	}
	// Default to the flow name.
	return def.Name
}

// HandleWebhook dispatches an incoming HTTP request to the matching flow trigger.
// Returns true if a handler was found.
func (e *Engine) HandleWebhook(path string, body map[string]any) bool {
	e.webhookMu.RLock()
	handler, ok := e.webhooks[path]
	e.webhookMu.RUnlock()
	if !ok {
		return false
	}
	if body == nil {
		body = make(map[string]any)
	}
	body["_trigger"] = "webhook"
	body["_path"] = path
	handler(body)
	return true
}

// --- Event Trigger ---

// EventSubscriber subscribes to NATS topics for event-triggered flows.
type EventSubscriber interface {
	Subscribe(pattern string, handler func(data []byte)) (func(), error)
}

// EventTrigger fires a flow when a matching NATS event is published.
type EventTrigger struct {
	engine      *Engine
	unsubscribe func()
}

func (t *EventTrigger) Start(_ context.Context, def *FlowDef, onTrigger func(inputs map[string]any)) error {
	data := def.TriggerConfig()
	topic, _ := data["event"].(string)
	if topic == "" {
		return fmt.Errorf("event trigger requires an 'event' topic in trigger config")
	}

	if t.engine.eventSub == nil {
		return fmt.Errorf("event trigger: no event subscriber configured")
	}

	unsub, err := t.engine.eventSub.Subscribe(topic, func(payload []byte) {
		inputs := map[string]any{
			"_trigger":   "event",
			"_topic":     topic,
			"_timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		// Try to parse payload as JSON.
		var parsed any
		if json.Unmarshal(payload, &parsed) == nil {
			inputs["event"] = parsed
		} else {
			inputs["event"] = string(payload)
		}
		log.Printf("[flow-event] %s: event on %s, triggering", def.Name, topic)
		onTrigger(inputs)
	})
	if err != nil {
		return fmt.Errorf("subscribe to %s: %w", topic, err)
	}
	t.unsubscribe = unsub

	log.Printf("[flow-event] %s: subscribed to %s", def.Name, topic)
	return nil
}

func (t *EventTrigger) Stop() error {
	if t.unsubscribe != nil {
		t.unsubscribe()
	}
	return nil
}

// --- Registration ---

func RegisterBuiltinTriggers(e *Engine) {
	e.RegisterTrigger("cron", func(triggerType string) (TriggerHandler, error) {
		return &CronTrigger{}, nil
	})
	e.RegisterTrigger("interval", func(triggerType string) (TriggerHandler, error) {
		return &IntervalTrigger{}, nil
	})
	e.RegisterTrigger("webhook", func(triggerType string) (TriggerHandler, error) {
		return &WebhookTrigger{engine: e}, nil
	})
	e.RegisterTrigger("event", func(triggerType string) (TriggerHandler, error) {
		return &EventTrigger{engine: e}, nil
	})
}
