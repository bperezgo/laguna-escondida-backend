package error

import "errors"

var (
	ErrInvalidPlateOptionSlots      = errors.New("invalid plate option slots")
	ErrPlateOptionSlotsUpdateFailed = errors.New("failed to update plate option slots")
	ErrBundleIngredientNotFound     = errors.New("a bundle ingredient does not exist")
)
