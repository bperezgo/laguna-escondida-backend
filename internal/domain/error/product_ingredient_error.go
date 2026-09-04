package error

import "errors"

var (
	ErrProductIngredientNotFound       = errors.New("product ingredient not found")
	ErrProductIngredientAlreadyExists  = errors.New("ingredient already exists for this composite product")
	ErrProductIngredientCreationFailed = errors.New("failed to create product ingredient")
	ErrProductIngredientUpdateFailed   = errors.New("failed to update product ingredient")
	ErrProductIngredientDeleteFailed   = errors.New("failed to delete product ingredient")
	ErrInvalidIngredientQuantity       = errors.New("invalid ingredient quantity")
	ErrProductNotComposite             = errors.New("product is not a composite product")
	ErrIngredientCannotBeSelf          = errors.New("a product cannot be an ingredient of itself")
	ErrIngredientCycle                 = errors.New("adding this ingredient would create a cycle in the recipe graph")
)
