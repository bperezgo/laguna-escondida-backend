package error

import "errors"

var (
	ErrProductCreationFailed             = errors.New("failed to create product")
	ErrProductUpdateFailed               = errors.New("failed to update product")
	ErrProductDeleteFailed               = errors.New("failed to delete product")
	ErrProductResponsibilityNotFound     = errors.New("product responsibility not found")
	ErrProductResponsibilityUpdateFailed = errors.New("failed to update product responsibility")
	ErrProductResponsibilityDeleteFailed = errors.New("failed to delete product responsibility")
)
