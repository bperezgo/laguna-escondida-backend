package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

// ReservationNotifier tells open staff screens that a reservation changed. It is called after the
// change is committed and is best effort: a screen that misses one reloads the day.
type ReservationNotifier interface {
	ReservationChanged(ctx context.Context, reservation *dto.Reservation)
}
