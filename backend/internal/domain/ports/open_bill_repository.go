package ports

import (
	"context"
	"time"

	openBill "laguna-escondida/backend/internal/domain/aggregate/open_bill"
	"laguna-escondida/backend/internal/domain/dto"
)

type OpenBillRepository interface {
	Create(ctx context.Context, aggregate *openBill.Aggregate) error
	ExistsActiveByTemporalIdentifier(ctx context.Context, temporalIdentifier string) (bool, error)
	FindByID(ctx context.Context, id string) (*dto.OpenBillWithProducts, error)
	FindAggregateByID(ctx context.Context, id string) (*openBill.Aggregate, error)
	// FindAggregateForKitchen also loads paid orders, whose lines keep cooking after payment.
	FindAggregateForKitchen(ctx context.Context, id string) (*openBill.Aggregate, error)
	FindAll(ctx context.Context, filter dto.OrderListFilter) ([]*dto.OpenBillWithCreator, error)
	FindDeletedByCreatedAtBetween(ctx context.Context, from, to time.Time, filter dto.OrderListFilter) ([]*dto.OpenBillWithCreator, error)
	FindByIDWithProducts(ctx context.Context, id string) (*dto.OpenBillWithProducts, error)
	FindByIDIncludingDeletedWithProducts(ctx context.Context, id string) (*dto.OpenBillWithProducts, error)
	Update(ctx context.Context, aggregate *openBill.Aggregate) error
	UpdateProductStatus(ctx context.Context, aggregate *openBill.Aggregate) error
	Delete(ctx context.Context, openBillID string) error
	// DeleteWithReason voids an open order as Delete does, recording why.
	DeleteWithReason(ctx context.Context, openBillID, reason string) error
	// SetPayment records how an agent order is paid, and when a manual one expires, right after it is created.
	SetPayment(ctx context.Context, openBillID string, strategy dto.PaymentStrategy, expiresAt *time.Time) error
	// SetPaymentReview marks (startedAt set) or clears (nil) the review of an unpaid, unvoided manual order.
	// updated=false when the order is not one of those.
	SetPaymentReview(ctx context.Context, openBillID string, startedAt *time.Time) (updated bool, err error)
	// FindPaymentState reads an order even when it is paid or voided; found=false when it does not exist.
	FindPaymentState(ctx context.Context, openBillID string) (state *dto.OrderPaymentState, found bool, err error)
	// ListManualDue is the ids of manual orders past their payment expiry, unpaid, unvoided and not under review.
	ListManualDue(ctx context.Context, now time.Time) ([]string, error)
	ClosePaid(ctx context.Context, openBillID string, paidAt time.Time) error
	GetProductPreparationResponsibilities(ctx context.Context, productIDs []string) ([]dto.ProductPreparationResponsibilityWithProduct, error)
	FindPendingByArea(ctx context.Context, area string) ([]*dto.OpenBillProductSSE, error)
	FindCompletedByAreaBetween(ctx context.Context, area string, from, to time.Time) ([]*dto.OpenBillProductSSE, error)
}
