package ports

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
)

// MarketingEventOutbox records an event inside the caller's transaction, so it commits or rolls
// back with the change it reports.
type MarketingEventOutbox interface {
	Append(ctx context.Context, event *dto.MarketingEvent) error
}

// MarketingEventStore is the sender's side of the outbox, for the context's organization.
type MarketingEventStore interface {
	// ClaimDue leases up to limit pending events due at now, skipping rows another sender holds,
	// by moving their next attempt to leaseUntil and counting the attempt.
	ClaimDue(ctx context.Context, now, leaseUntil time.Time, limit int) ([]*dto.MarketingEvent, error)
	MarkSent(ctx context.Context, eventID string, at time.Time) error
	// MarkFailed stores the outcome of a failed attempt: pending with its next attempt, or a final status.
	MarkFailed(ctx context.Context, eventID string, status dto.MarketingEventStatus, nextAttemptAt time.Time, lastError string) error
}

// MarketingEventTransport POSTs signed bytes to marketing-app. A status of 0 with an error means
// no answer came (timeout, connection error).
type MarketingEventTransport interface {
	Send(ctx context.Context, path string, body []byte, signature string) (int, error)
}

// MarketingEventSecrets holds each organization's signing secret, by slug.
type MarketingEventSecrets interface {
	SecretFor(slug string) (string, bool)
}

// MarketingEventRecorder writes the event a change produces, when it should produce one.
type MarketingEventRecorder interface {
	RecordOrder(ctx context.Context, eventType dto.MarketingEventType, subject dto.MarketingOrderSubject) error
	// RecordReservation records the event for the status the reservation just entered, if any.
	RecordReservation(ctx context.Context, reservation *dto.Reservation, at time.Time) error
	RecordFeedbackRequested(ctx context.Context, request *dto.FeedbackRequest, reservation *dto.Reservation) error
}

// OrderOriginReader reads what the cloud already holds of an order before a peer's snapshot of it is applied.
type OrderOriginReader interface {
	// FindOrderOrigin reports found=false for an order the cloud has never stored.
	FindOrderOrigin(ctx context.Context, orderID string) (state *dto.OrderOriginState, found bool, err error)
}
