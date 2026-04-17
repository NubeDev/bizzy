// Package bus provides an embedded NATS event bus with JetStream persistence.
// It runs in-process with no external dependencies — same binary, no ops overhead.
package bus

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

// Bus wraps an embedded NATS server with JetStream for persistent pub/sub.
type Bus struct {
	server *server.Server
	conn   *nats.Conn
	js     nats.JetStreamContext
}

// New starts an embedded NATS server with JetStream persistence.
// Data is stored under dataDir/nats/. No external TCP listener — in-process only.
func New(dataDir string) (*Bus, error) {
	opts := &server.Options{
		DontListen: true, // no external TCP — in-process only
		StoreDir:   filepath.Join(dataDir, "nats"),
		JetStream:  true,
	}

	srv, err := server.NewServer(opts)
	if err != nil {
		return nil, fmt.Errorf("create nats server: %w", err)
	}
	go srv.Start()

	if !srv.ReadyForConnections(5 * time.Second) {
		return nil, fmt.Errorf("nats server not ready after 5s")
	}

	conn, err := nats.Connect("", nats.InProcessServer(srv))
	if err != nil {
		srv.Shutdown()
		return nil, fmt.Errorf("connect to embedded nats: %w", err)
	}

	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		srv.Shutdown()
		return nil, fmt.Errorf("enable jetstream: %w", err)
	}

	// Create streams for each event domain.
	streams := []nats.StreamConfig{
		{Name: "COMMANDS", Subjects: []string{"command.>"}, MaxAge: 24 * time.Hour},
		{Name: "WORKFLOWS", Subjects: []string{"workflow.>"}, MaxAge: 7 * 24 * time.Hour},
		{Name: "JOBS", Subjects: []string{"job.>"}, MaxAge: 24 * time.Hour},
		{Name: "TOOLS", Subjects: []string{"tool.>"}, MaxAge: 24 * time.Hour},
	}
	for _, cfg := range streams {
		if _, err := js.AddStream(&cfg); err != nil {
			conn.Close()
			srv.Shutdown()
			return nil, fmt.Errorf("create stream %s: %w", cfg.Name, err)
		}
	}

	return &Bus{server: srv, conn: conn, js: js}, nil
}

// Publish sends an event to the bus. JetStream persists it.
func (b *Bus) Publish(topic string, data any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	_, err = b.js.Publish(topic, payload)
	return err
}

// Subscribe creates a push-based ephemeral consumer.
func (b *Bus) Subscribe(pattern string, handler func(msg *nats.Msg)) (*nats.Subscription, error) {
	return b.js.Subscribe(pattern, handler,
		nats.MaxAckPending(64),
		nats.AckWait(30*time.Second),
	)
}

// SubscribeDurable creates a durable consumer that survives restarts.
func (b *Bus) SubscribeDurable(pattern, name string, handler func(msg *nats.Msg)) (*nats.Subscription, error) {
	return b.js.Subscribe(pattern, handler,
		nats.Durable(name),
		nats.MaxAckPending(64),
		nats.AckWait(30*time.Second),
	)
}

// Conn returns the underlying NATS connection for advanced use.
func (b *Bus) Conn() *nats.Conn {
	return b.conn
}

// JS returns the JetStream context for advanced use.
func (b *Bus) JS() nats.JetStreamContext {
	return b.js
}

// Close shuts down the connection and embedded server.
func (b *Bus) Close() {
	if b.conn != nil {
		b.conn.Close()
	}
	if b.server != nil {
		b.server.Shutdown()
	}
}
