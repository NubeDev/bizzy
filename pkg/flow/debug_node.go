package flow

import (
	"context"
	"time"
)

// executeDebug implements the debug node — a passthrough that captures
// the message flowing through it into the run's debug log. Like Node-RED's
// debug node, it records data for inspection without modifying the message.
//
// Config:
//   - label:  display label in the debug panel (default: node ID)
//   - output: "full" (entire msg), "payload" (msg.payload only), or an
//     expression key to extract (default: "payload")
//   - active: false to mute this debug node (default: true)
func executeDebug(_ context.Context, ec *ExecContext) (any, error) {
	msg := inputMsg(ec)

	// Check if the debug node is muted.
	if active, ok := ec.Node.Data["active"].(bool); ok && !active {
		return msg, nil // muted — pass through silently
	}

	label := getStringOrDefault(ec.Node.Data, "label", ec.Node.ID)
	output := getStringOrDefault(ec.Node.Data, "output", "payload")

	// Determine what to capture.
	var value any
	switch output {
	case "full":
		value = msg
	case "payload":
		value = MsgPayload(msg)
	default:
		// Treat as a key to extract from the payload.
		if p, ok := MsgPayload(msg).(map[string]any); ok {
			value = p[output]
		} else {
			value = MsgPayload(msg)
		}
	}

	entry := DebugEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		NodeID:    ec.Node.ID,
		Label:     label,
		MsgID:     MsgGetString(msg, "_msgid"),
		Value:     value,
	}

	// Append to the run's debug log.
	ec.Run.AppendDebug(entry)

	// Publish as a live event for the WS debug panel.
	if ec.Engine != nil {
		ec.Engine.events.debugEntry(ec.Run, entry)
	}

	return msg, nil // passthrough — msg is unchanged
}
