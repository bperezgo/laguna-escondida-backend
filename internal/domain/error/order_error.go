package error

import "errors"

var (
	ErrProductNotFound             = errors.New("product not found")
	ErrInvalidProductIDs           = errors.New("invalid product ids")
	ErrOrderCreationFailed         = errors.New("failed to create order")
	ErrDuplicateTemporalIdentifier = errors.New("an active order with this temporal identifier already exists")
	ErrOrderNotFound               = errors.New("order not found")
	ErrOrderUpdateFailed           = errors.New("failed to update order")
	ErrOrderPaymentFailed          = errors.New("failed to pay order")
	ErrOrderDeletionFailed         = errors.New("failed to delete order")
	ErrBillOwnerNotFound           = errors.New("bill owner not found")
	ErrInvalidSideDishSelection    = errors.New("side-dish selection references an ingredient that is not a side-dish option of the product")
	ErrSideDishQuantityOutOfBounds = errors.New("side-dish quantity is outside the configured min/max bounds")
)
