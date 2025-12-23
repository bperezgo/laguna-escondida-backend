package error

import "errors"

var (
	ErrProductNotFound      = errors.New("product not found")
	ErrInvalidProductIDs    = errors.New("invalid product ids")
	ErrOrderCreationFailed  = errors.New("failed to create order")
	ErrOrderNotFound        = errors.New("order not found")
	ErrOrderUpdateFailed    = errors.New("failed to update order")
	ErrOrderPaymentFailed   = errors.New("failed to pay order")
	ErrOrderDeletionFailed  = errors.New("failed to delete order")
	ErrBillOwnerNotFound    = errors.New("bill owner not found")
)
