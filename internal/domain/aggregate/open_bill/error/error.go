package error

import "errors"

var (
	ErrInvalidOpenBillID        = errors.New("OPEN_BILL_INVALID_ID: open_bill_id must be a valid UUID")
	ErrInvalidOpenBillProductID = errors.New("OPEN_BILL_PRODUCT_INVALID_ID: open_bill_product_id must be a valid UUID")
	ErrInvalidProductID         = errors.New("OPEN_BILL_PRODUCT_INVALID_PRODUCT_ID: product_id must be a valid UUID")
	ErrInvalidQuantity          = errors.New("OPEN_BILL_PRODUCT_INVALID_QUANTITY: quantity must be greater than 0")

	// OpenBillProduct status errors
	ErrOpenBillProductNotFound          = errors.New("OPEN_BILL_PRODUCT_NOT_FOUND: open_bill_product not found")
	ErrProductAlreadyCompleted          = errors.New("OPEN_BILL_PRODUCT_ALREADY_COMPLETED: product is already completed")
	ErrProductNotCompleted              = errors.New("OPEN_BILL_PRODUCT_NOT_COMPLETED: product is not completed")
	ErrProductAlreadyCancelled          = errors.New("OPEN_BILL_PRODUCT_ALREADY_CANCELLED: product is already cancelled")
	ErrCannotCompleteProduct            = errors.New("OPEN_BILL_PRODUCT_CANNOT_COMPLETE: product cannot be completed from current status")
	ErrCannotCancelProduct              = errors.New("OPEN_BILL_PRODUCT_CANNOT_CANCEL: product cannot be cancelled from current status")
	ErrCannotSetInProgressFromCancelled = errors.New("OPEN_BILL_PRODUCT_CANNOT_SET_IN_PROGRESS: cannot set in_progress from cancelled status")

	// OpenBill status errors
	ErrOpenBillAlreadyCompleted = errors.New("OPEN_BILL_ALREADY_COMPLETED: open_bill is already completed")
	ErrOpenBillAlreadyCancelled = errors.New("OPEN_BILL_ALREADY_CANCELLED: open_bill is already cancelled")
)
