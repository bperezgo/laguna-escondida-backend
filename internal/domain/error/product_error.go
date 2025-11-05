package error

import "errors"

var (
	ErrProductCreationFailed = errors.New("failed to create product")
	ErrProductUpdateFailed   = errors.New("failed to update product")
	ErrProductDeleteFailed   = errors.New("failed to delete product")
)
