package ports

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
)

type DomainEvent interface {
	EventID() string
	EventName() string
	SubjectID() string
	OccurredAt() time.Time
}

// EventPublisher stores an event; call it inside the unit of work of the change it reports, so both commit or roll back together.
type EventPublisher interface {
	Publish(ctx context.Context, event DomainEvent) error
}

// EventConsumer must be idempotent by event id; Name is stored in deliveries, so it must stay stable.
type EventConsumer interface {
	Name() string
	Handles() []string
	Handle(ctx context.Context, event dto.StoredEvent) error
}

// EventDeliveryStore is the relay's side of the bus, for the context's organization.
type EventDeliveryStore interface {
	// ClaimDue leases up to limit pending deliveries due at now, skipping rows another relay holds,
	// by moving their next attempt to leaseUntil and counting the attempt.
	ClaimDue(ctx context.Context, now, leaseUntil time.Time, limit int) ([]*dto.ClaimedDelivery, error)
	MarkDone(ctx context.Context, eventID, consumer string, at time.Time) error
	// MarkFailed stores a failed attempt: pending with its next attempt, or dead once the limit is reached.
	MarkFailed(ctx context.Context, eventID, consumer string, status dto.EventDeliveryStatus, nextAttemptAt time.Time, lastError string) error
}
