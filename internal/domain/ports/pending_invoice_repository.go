package ports

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
)

// PendingInvoiceRepository is the read/mark side of the local electronic-invoice submission
// queue, used by the background submitter. Rows are enqueued atomically with the bill inside
// BillRepository.Create (the reserved consecutive originates there), so there is no Enqueue
// here. The queue is never replicated between nodes.
type PendingInvoiceRepository interface {
	// ListDue returns pending rows whose next_attempt_at is null or due, ordered by
	// consecutive ascending (DIAN numbers submitted lowest-first), capped at limit.
	ListDue(ctx context.Context, limit int) ([]*dto.PendingInvoice, error)
	MarkSubmitted(ctx context.Context, id string) error
	// MarkFailed increments attempts and records the error and the next eligible time.
	MarkFailed(ctx context.Context, id string, errMsg string, nextAttemptAt time.Time) error
}
