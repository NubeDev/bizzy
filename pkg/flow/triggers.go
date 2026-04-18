package flow

import (
	"context"
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

func matchesCron(expr string, t time.Time) bool {
	var minute, hour, dom, month, dow string
	_, err := fmt.Sscanf(expr, "%s %s %s %s %s", &minute, &hour, &dom, &month, &dow)
	if err != nil {
		return false
	}

	return fieldMatches(minute, t.Minute()) &&
		fieldMatches(hour, t.Hour()) &&
		fieldMatches(dom, t.Day()) &&
		fieldMatches(month, int(t.Month())) &&
		fieldMatches(dow, int(t.Weekday()))
}

func fieldMatches(field string, value int) bool {
	if field == "*" {
		return true
	}
	var v int
	if _, err := fmt.Sscanf(field, "%d", &v); err == nil {
		return v == value
	}
	return false
}

// --- Registration ---

func RegisterBuiltinTriggers(e *Engine) {
	e.RegisterTrigger("cron", func(triggerType string) (TriggerHandler, error) {
		return &CronTrigger{}, nil
	})
	e.RegisterTrigger("interval", func(triggerType string) (TriggerHandler, error) {
		return &IntervalTrigger{}, nil
	})
}
