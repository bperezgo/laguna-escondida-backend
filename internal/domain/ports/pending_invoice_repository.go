package ports

import (
	"context"
	"encoding/json"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
)

// PendingInvoiceRepository manages the cloud electronic-invoice submission queue.
// Rows arrive either from service-layer calls to Create (for bills paid on this node) or
// from the sync applier (edge bills replicated via outbox). Consecutive and RequestPayload
// start as nil and are filled in by AssignConsecutive before the first submission attempt.
type PendingInvoiceRepository interface {
	// Create persists a new pending invoice row. The caller constructs the DTO (including
	// the ID and initial status) so the repository is a pure persistence adapter.
	Create(ctx context.Context, pendingInvoice *dto.PendingInvoice) error
	// ListDue returns pending rows whose next_attempt_at is null or due, ordered by
	// consecutive ascending (DIAN numbers submitted lowest-first, NULLs first), capped at limit.
	ListDue(ctx context.Context, limit int) ([]*dto.PendingInvoice, error)
	// AssignConsecutive persists the cloud-assigned consecutive and the built request payload.
	// A WHERE consecutive IS NULL guard makes it idempotent against concurrent cron ticks.
	AssignConsecutive(ctx context.Context, id string, consecutive int, requestPayload json.RawMessage) error
	MarkSubmitted(ctx context.Context, id string) error
	// MarkFailed increments attempts and records the error and the next eligible time.
	MarkFailed(ctx context.Context, id string, errMsg string, nextAttemptAt time.Time) error
}
