package sse

import (
	"context"
	"encoding/json"

	"laguna-escondida/backend/internal/domain/ports"
)

// CommandNotifier adapts the SSE Hub to implement the Notifier interface.
// It broadcasts events to all registered SSE clients across all areas.
type CommandNotifier struct {
	hub *Hub
}

func NewCommandNotifier(hub *Hub) *CommandNotifier {
	return &CommandNotifier{hub: hub}
}

func (n *CommandNotifier) Notify(ctx context.Context, eventName string, data []byte) error {
	var eventData any
	if err := json.Unmarshal(data, &eventData); err != nil {
		eventData = string(data)
	}

	event := Event{
		Type: eventName,
		Data: eventData,
	}

	n.hub.BroadcastAll(event)
	return nil
}

var _ ports.Notifier = (*CommandNotifier)(nil)
