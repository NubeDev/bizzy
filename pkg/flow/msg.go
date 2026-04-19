package flow

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Node-RED message convention
//
// Every wire in a flow carries a msg — a map[string]any with three conventional keys:
//
//   msg.payload  — the main data (sensor reading, API response, tool result, etc.)
//   msg.topic    — optional label or category (e.g. MQTT topic, flow name)
//   msg._msgid   — unique message ID for tracing
//
// Nodes receive a msg, process it, and emit a msg. Node settings configured in
// the panel act as defaults; msg-level properties override them (e.g. msg.url
// overrides the HTTP request node's URL setting). Nodes that transform data set
// msg.payload to the new value and preserve other msg properties.

// NewMsg creates a new flow message with the given payload and a unique ID.
func NewMsg(payload any) map[string]any {
	return map[string]any{
		"payload":    payload,
		"_msgid":     generateMsgID(),
		"_timestamp": time.Now().UTC().Format(time.RFC3339),
	}
}

// MsgPayload extracts the payload from a msg. If the value is not a msg
// envelope (no "payload" key), returns the value as-is for backward compat.
func MsgPayload(msg any) any {
	if m, ok := msg.(map[string]any); ok {
		if p, exists := m["payload"]; exists {
			return p
		}
	}
	return msg
}

// MsgTopic extracts the topic from a msg. Returns "" if not set.
func MsgTopic(msg any) string {
	if m, ok := msg.(map[string]any); ok {
		if t, ok := m["topic"].(string); ok {
			return t
		}
	}
	return ""
}

// MsgSet creates a copy of the msg with a new payload, preserving all other
// properties (topic, _msgid, any custom fields set by upstream nodes).
// If msg is not a map, creates a new msg envelope.
func MsgSet(msg any, payload any) map[string]any {
	result := make(map[string]any)
	if m, ok := msg.(map[string]any); ok {
		for k, v := range m {
			result[k] = v
		}
	} else {
		result["_msgid"] = generateMsgID()
	}
	result["payload"] = payload
	return result
}

// MsgGet reads a property from the msg. Returns (value, true) if found.
// This is how nodes read msg-level overrides (e.g. msg.url, msg.method).
func MsgGet(msg any, key string) (any, bool) {
	if m, ok := msg.(map[string]any); ok {
		v, exists := m[key]
		return v, exists
	}
	return nil, false
}

// MsgGetString reads a string property from the msg. Returns "" if not found.
func MsgGetString(msg any, key string) string {
	if v, ok := MsgGet(msg, key); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// IsMsg checks if a value looks like a msg envelope (has a "payload" key).
func IsMsg(v any) bool {
	if m, ok := v.(map[string]any); ok {
		_, has := m["payload"]
		return has
	}
	return false
}

// resolveFromMsg reads a value: msg property first, then node data, then "".
// This implements the Node-RED pattern where msg properties override node settings.
func resolveFromMsg(msg any, data map[string]any, key string) string {
	if s := MsgGetString(msg, key); s != "" {
		return s
	}
	if data != nil {
		if s, ok := data[key].(string); ok {
			return s
		}
	}
	return ""
}

func generateMsgID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
