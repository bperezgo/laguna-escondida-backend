package sse

import (
	"context"
	"encoding/json"
	"log"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
)

// OpenBillProductNotifier adapts the OpenBillProductHub to implement the Notifier interface.
// It broadcasts events to all registered open bill product SSE clients across all areas.
type OpenBillProductNotifier struct {
	hub *OpenBillProductHub
}

func NewOpenBillProductNotifier(hub *OpenBillProductHub) *OpenBillProductNotifier {
	return &OpenBillProductNotifier{hub: hub}
}

func (n *OpenBillProductNotifier) Notify(ctx context.Context, eventName string, data []byte) error {
	var openBillProductData dto.OpenBillProductSSE
	if err := json.Unmarshal(data, &openBillProductData); err != nil {
		log.Printf("OpenBillProductNotifier: failed to unmarshal data for event %s: %v", eventName, err)
		return nil
	}

	event := OpenBillProductEvent{
		Type: eventName,
		Data: &openBillProductData,
	}

	n.hub.BroadcastAll(event)
	return nil
}

var _ ports.Notifier = (*OpenBillProductNotifier)(nil)
