package dto

import "time"

// EventMetadata contains common metadata for all domain events.
type EventMetadata struct {
	EventID     string    `json:"event_id"`
	OccurredAt  time.Time `json:"occurred_at"`
	AggregateID string    `json:"aggregate_id,omitempty"`
}
