package sse

import (
	"context"
	"encoding/json"
	"log"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
)

// CommandItemNotifier adapts the CommandItemHub to implement the Notifier interface.
// It broadcasts events to all registered command item SSE clients across all areas.
type CommandItemNotifier struct {
	hub *CommandItemHub
}

func NewCommandItemNotifier(hub *CommandItemHub) *CommandItemNotifier {
	return &CommandItemNotifier{hub: hub}
}

func (n *CommandItemNotifier) Notify(ctx context.Context, eventName string, data []byte) error {
	var commandItemData dto.CommandItemSSE
	if err := json.Unmarshal(data, &commandItemData); err != nil {
		log.Printf("CommandItemNotifier: failed to unmarshal data for event %s: %v", eventName, err)
		return nil
	}

	event := CommandItemEvent{
		Type: eventName,
		Data: &commandItemData,
	}

	n.hub.BroadcastAll(event)
	return nil
}

var _ ports.Notifier = (*CommandItemNotifier)(nil)
