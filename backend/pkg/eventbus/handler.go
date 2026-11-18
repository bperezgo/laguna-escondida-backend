package eventbus

import (
	"context"
	"encoding/json"
	"fmt"

	"laguna-escondida/backend/pkg/domain/ports"
)

// TypedEventHandler provides type-safe event handling.
// It wraps a typed handler function and handles JSON unmarshaling.
type TypedEventHandler[T ports.Event] struct {
	eventName string
	handleFn  func(ctx context.Context, event T) error
}

// NewTypedEventHandler creates a new typed event handler.
// The eventName should match the EventName() returned by the event type T.
func NewTypedEventHandler[T ports.Event](eventName string, handleFn func(ctx context.Context, event T) error) *TypedEventHandler[T] {
	return &TypedEventHandler[T]{
		eventName: eventName,
		handleFn:  handleFn,
	}
}

// Handle implements ports.EventHandler by unmarshaling the payload and calling the typed handler.
func (h *TypedEventHandler[T]) Handle(ctx context.Context, payload []byte) error {
	var event T
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}
	return h.handleFn(ctx, event)
}

// EventName returns the event name/topic this handler subscribes to.
func (h *TypedEventHandler[T]) EventName() string {
	return h.eventName
}

var _ ports.EventHandler = (*TypedEventHandler[ports.Event])(nil)
