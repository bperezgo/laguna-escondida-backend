package error

import "errors"

var (
	ErrInvalidOpenBillID        = errors.New("OPEN_BILL_INVALID_ID: open_bill_id must be a valid UUID")
	ErrInvalidOpenBillProductID = errors.New("OPEN_BILL_PRODUCT_INVALID_ID: open_bill_product_id must be a valid UUID")
	ErrInvalidProductID         = errors.New("OPEN_BILL_PRODUCT_INVALID_PRODUCT_ID: product_id must be a valid UUID")
	ErrInvalidQuantity          = errors.New("OPEN_BILL_PRODUCT_INVALID_QUANTITY: quantity must be greater than 0")
)
