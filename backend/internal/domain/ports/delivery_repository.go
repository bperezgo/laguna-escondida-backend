package ports

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
)

// DeliveryRepository owns the delivery columns of open_bills and their history. Every read is
// limited to paid takeaway and delivery orders of the context's organization.
type DeliveryRepository interface {
	// FindTrackingState returns ErrDeliveryOrderNotFound for an id outside the organization.
	FindTrackingState(ctx context.Context, orderID string) (*dto.DeliveryTrackingState, error)
	// ApplyStatusChange updates the order only if it still holds the expected status and courier,
	// and then appends the history entry; false means someone changed it first.
	ApplyStatusChange(ctx context.Context, change *dto.DeliveryStatusChange) (bool, error)
	// MarkReady stamps ready_at on a paid takeaway or delivery order once; false when already set.
	MarkReady(ctx context.Context, orderID string, at time.Time) (bool, error)
	// List is every order not delivered, plus those delivered since deliveredSince.
	List(ctx context.Context, filter dto.DeliveryListFilter, deliveredSince time.Time) ([]dto.DeliveryOrder, error)
	FindOrder(ctx context.Context, orderID string) (*dto.DeliveryOrder, error)
	History(ctx context.Context, orderID string) (*dto.DeliveryHistory, error)
	// Claimable is ready delivery orders with no courier, oldest ready first.
	Claimable(ctx context.Context) ([]dto.DeliveryOrder, error)
	// OnTheRoad is the courier's orders out for delivery, in the order they went out.
	OnTheRoad(ctx context.Context, courierUserID string) ([]dto.DeliveryOrder, error)
	ActiveCouriers(ctx context.Context) ([]dto.CourierUser, error)
	// FindActiveCourier returns false for a user who is inactive, deleted or not a courier.
	FindActiveCourier(ctx context.Context, userID string) (*dto.CourierUser, bool, error)
}
