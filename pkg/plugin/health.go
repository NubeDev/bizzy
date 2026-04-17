package plugin

import (
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

// HealthConfig controls the health monitor's thresholds.
type HealthConfig struct {
	// CheckInterval is how often we scan for stale heartbeats (default 10s).
	CheckInterval time.Duration
	// StaleThreshold is how long since the last heartbeat before incrementing
	// the failure counter (default 15s — slightly more than one heartbeat).
	StaleThreshold time.Duration
	// MaxFailures is consecutive missed checks before marking crashed (default 3).
	MaxFailures int
}

func (c *HealthConfig) withDefaults() HealthConfig {
	out := *c
	if out.CheckInterval == 0 {
		out.CheckInterval = 10 * time.Second
	}
	if out.StaleThreshold == 0 {
		out.StaleThreshold = 15 * time.Second
	}
	if out.MaxFailures == 0 {
		out.MaxFailures = 3
	}
	return out
}

// OnCrashedFunc is called when a plugin is detected as crashed.
// The registry provides this callback to decouple health monitoring
// from tool removal logic.
type OnCrashedFunc func(name string)

// HealthMonitor watches plugin heartbeats and detects crashes.
type HealthMonitor struct {
	nc   *nats.Conn
	cfg  HealthConfig
	sub  *nats.Subscription
	stop chan struct{}

	mu      sync.RWMutex
	plugins map[string]*PluginState // shared reference from registry

	onCrashed OnCrashedFunc
}

// NewHealthMonitor creates a monitor. The plugins map is shared with the
// registry — both read/write under their own locks, but the monitor only
// touches LastHeartbeat and HealthFailures.
func NewHealthMonitor(nc *nats.Conn, plugins map[string]*PluginState, mu *sync.RWMutex, cfg HealthConfig, onCrashed OnCrashedFunc) *HealthMonitor {
	cfg = cfg.withDefaults()
	return &HealthMonitor{
		nc:        nc,
		cfg:       cfg,
		plugins:   plugins,
		onCrashed: onCrashed,
		stop:      make(chan struct{}),
	}
}

// Start subscribes to heartbeats and begins the check loop.
func (h *HealthMonitor) Start() error {
	sub, err := h.nc.Subscribe(SubjectHealthWildcard, h.handleHeartbeat)
	if err != nil {
		return err
	}
	h.sub = sub
	go h.checkLoop()
	return nil
}

// Stop unsubscribes and stops the check loop.
func (h *HealthMonitor) Stop() {
	close(h.stop)
	if h.sub != nil {
		h.sub.Unsubscribe()
	}
}

func (h *HealthMonitor) handleHeartbeat(msg *nats.Msg) {
	// Subject format: extension.health.<name>
	parts := strings.SplitN(msg.Subject, ".", 3)
	if len(parts) < 3 {
		return
	}
	name := parts[2]

	var hm HealthMessage
	json.Unmarshal(msg.Data, &hm) // best-effort parse

	h.mu.Lock()
	if p, ok := h.plugins[name]; ok {
		p.LastHeartbeat = time.Now()
		p.HealthFailures = 0
	}
	h.mu.Unlock()
}

func (h *HealthMonitor) checkLoop() {
	ticker := time.NewTicker(h.cfg.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-h.stop:
			return
		case <-ticker.C:
			h.check()
		}
	}
}

func (h *HealthMonitor) check() {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	for name, p := range h.plugins {
		if p.Status != "active" {
			continue
		}
		// Zero heartbeat means plugin just registered — give it a grace period.
		if p.LastHeartbeat.IsZero() {
			if now.Sub(p.RegisteredAt) > h.cfg.StaleThreshold {
				p.HealthFailures++
			}
		} else if now.Sub(p.LastHeartbeat) > h.cfg.StaleThreshold {
			p.HealthFailures++
		}

		if p.HealthFailures >= h.cfg.MaxFailures {
			p.Status = "crashed"
			log.Printf("[plugins] %s: marked crashed after %d missed heartbeats", name, p.HealthFailures)
			if h.onCrashed != nil {
				h.onCrashed(name)
			}
		}
	}
}
