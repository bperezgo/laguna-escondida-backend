package error

import "errors"

var (
	ErrStockNotFound          = errors.New("stock not found")
	ErrStockAlreadyExists     = errors.New("stock already exists for this product")
	ErrStockCreationFailed    = errors.New("failed to create stock")
	ErrStockUpdateFailed      = errors.New("failed to update stock")
	ErrStockDeleteFailed      = errors.New("failed to delete stock")
	ErrProductVersionMismatch = errors.New("product version does not match stock version")
	ErrStockDeleted           = errors.New("cannot update deleted stock")
)
