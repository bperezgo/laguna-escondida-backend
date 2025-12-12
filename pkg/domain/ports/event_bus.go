package ports

import "context"

// Event represents a domain event that can be published to the event bus.
// Implementations should define the topic/routing key via EventName().
type Event interface {
	EventName() string
}

// EventHandler handles events of a specific type.
// The handler receives raw event data and is responsible for unmarshaling.
type EventHandler interface {
	Handle(ctx context.Context, payload []byte) error
	EventName() string
}

// EventBus defines the contract for publishing domain events.
type EventBus interface {
	Publish(ctx context.Context, event Event) error
}

// EventSubscriber defines the contract for subscribing to domain events.
type EventSubscriber interface {
	Subscribe(handler EventHandler) error
	Start(ctx context.Context) error
	Close() error
}
