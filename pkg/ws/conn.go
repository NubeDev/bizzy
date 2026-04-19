// Package ws provides shared WebSocket infrastructure: upgrade, auth,
// concurrent-write-safe connections, and ping/pong keepalive.
package ws

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Upgrader is the shared WebSocket upgrader. All WS endpoints use this
// so upgrade behaviour is consistent.
var Upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

const (
	// writeWait is the max time to wait for a write to complete.
	writeWait = 10 * time.Second

	// pongWait is how long we wait for a pong before considering the connection dead.
	pongWait = 60 * time.Second

	// pingPeriod must be less than pongWait.
	pingPeriod = 50 * time.Second
)

// Conn wraps a gorilla/websocket.Conn with:
//   - Write serialization (gorilla connections are not safe for concurrent writes)
//   - Ping/pong keepalive
//   - Clean close helper
type Conn struct {
	raw  *websocket.Conn
	mu   sync.Mutex
	done chan struct{}
}

// Upgrade upgrades an HTTP connection to a WebSocket and returns a Conn.
func Upgrade(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	raw, err := Upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}
	c := &Conn{
		raw:  raw,
		done: make(chan struct{}),
	}
	c.raw.SetReadDeadline(time.Now().Add(pongWait))
	c.raw.SetPongHandler(func(string) error {
		c.raw.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	go c.pingLoop()
	return c, nil
}

// WriteJSON sends a JSON message, serialized with other writes.
func (c *Conn) WriteJSON(v any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.raw.SetWriteDeadline(time.Now().Add(writeWait))
	return c.raw.WriteJSON(v)
}

// ReadJSON reads the next JSON message from the client.
func (c *Conn) ReadJSON(v any) error {
	return c.raw.ReadJSON(v)
}

// Close sends a close frame and shuts down the connection.
func (c *Conn) Close() {
	select {
	case <-c.done:
		return // already closed
	default:
		close(c.done)
	}
	c.mu.Lock()
	c.raw.WriteMessage(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
	)
	c.mu.Unlock()
	c.raw.Close()
}

// Raw returns the underlying gorilla connection for cases where direct
// access is needed (e.g. ReadMessage).
func (c *Conn) Raw() *websocket.Conn {
	return c.raw
}

// Done returns a channel that is closed when the connection is closed.
func (c *Conn) Done() <-chan struct{} {
	return c.done
}

func (c *Conn) pingLoop() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			c.mu.Lock()
			c.raw.SetWriteDeadline(time.Now().Add(writeWait))
			err := c.raw.WriteMessage(websocket.PingMessage, nil)
			c.mu.Unlock()
			if err != nil {
				return
			}
		}
	}
}
