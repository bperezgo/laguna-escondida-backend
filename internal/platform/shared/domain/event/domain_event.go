package event

import "time"

// DomainEvent represents a domain event that can be published to the event bus.
// Implementations should provide event metadata and serialized payload.
type DomainEvent interface {
	EventID() string
	EventName() string
	OccurredAt() time.Time
	AggregateID() string
	Data() []byte
}
