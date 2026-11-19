package ports

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
)

// LeadRepository is platform data: no organization scopes it.
type LeadRepository interface {
	Create(ctx context.Context, lead *dto.Lead) error
	Get(ctx context.Context, id string) (*dto.Lead, error)
	// List returns the page newest first and the total matching the filter.
	List(ctx context.Context, filter dto.ListLeadsFilter) ([]*dto.Lead, int64, error)
	// Update stores status, notes and the update time only.
	Update(ctx context.Context, lead *dto.Lead) error
	Delete(ctx context.Context, id string) error
	// FindRecentByPhone returns a lead with that phone created at or after since, or nil.
	FindRecentByPhone(ctx context.Context, phone string, since time.Time) (*dto.Lead, error)
	CountSince(ctx context.Context, since time.Time) (int, error)
	// ClaimDueAlerts leases up to limit pending alerts due at now, skipping rows another sender
	// holds, by moving their next attempt to leaseUntil and counting the attempt.
	ClaimDueAlerts(ctx context.Context, now, leaseUntil time.Time, limit int) ([]*dto.Lead, error)
	// MarkAlert stores an attempt's outcome: sent, pending with its next attempt, or failed.
	MarkAlert(ctx context.Context, id string, status dto.LeadAlertStatus, nextAttemptAt time.Time, lastError string) error
}
