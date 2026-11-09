package ports

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
)

// ReservationRepository stores the context's organization's reservations. Reads fill in the
// contact's phone and whether feedback was requested.
type ReservationRepository interface {
	Create(ctx context.Context, reservation *dto.Reservation) error
	// Update writes the lifecycle fields only (who, when and origin never change), and only while the
	// stored status is still from; otherwise it returns error.ErrReservationTransition.
	Update(ctx context.Context, reservation *dto.Reservation, from dto.ReservationStatus) error
	// FindByID returns error.ErrReservationNotFound when there is none.
	FindByID(ctx context.Context, id string) (*dto.Reservation, error)
	// List orders by starts_at, oldest first.
	List(ctx context.Context, filter dto.ReservationListFilter) ([]*dto.Reservation, error)
	// ListConfirmedStartedBefore returns confirmed reservations whose starts_at is before cutoff.
	ListConfirmedStartedBefore(ctx context.Context, cutoff time.Time) ([]*dto.Reservation, error)
	// ListAwaitingFeedback returns visited reservations with a channel and no feedback request,
	// that became visited at or after visitedSince.
	ListAwaitingFeedback(ctx context.Context, visitedSince time.Time) ([]*dto.Reservation, error)
}
