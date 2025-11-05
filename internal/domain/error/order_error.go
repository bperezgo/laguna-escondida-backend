package error

import "errors"

var (
	ErrProductNotFound     = errors.New("product not found")
	ErrInvalidProductIDs   = errors.New("invalid product ids")
	ErrOrderCreationFailed = errors.New("failed to create order")
)
