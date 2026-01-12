package event

import (
	"time"

	"github.com/google/uuid"
)

// BaseEvent provides common metadata for all domain events.
// Concrete events should embed this struct and implement the Data() method.
type BaseEvent struct {
	eventID     string
	eventName   string
	aggregateID string
	occurredAt  time.Time
}

func NewBaseEvent(eventName, aggregateID string) BaseEvent {
	return BaseEvent{
		eventID:     uuid.NewString(),
		eventName:   eventName,
		aggregateID: aggregateID,
		occurredAt:  time.Now(),
	}
}

func (e BaseEvent) EventID() string {
	return e.eventID
}

func (e BaseEvent) EventName() string {
	return e.eventName
}

func (e BaseEvent) OccurredAt() time.Time {
	return e.occurredAt
}

func (e BaseEvent) AggregateID() string {
	return e.aggregateID
}
