package product

import (
	productError "laguna-escondida/backend/internal/domain/aggregate/product/error"
)

type ProductType string

const (
	ProductTypeSellable   ProductType = "SELLABLE"
	ProductTypeIngredient ProductType = "INGREDIENT"
	ProductTypeComposite  ProductType = "COMPOSITE"
	ProductTypeBoth       ProductType = "BOTH"
)

var validProductTypes = map[ProductType]bool{
	ProductTypeSellable:   true,
	ProductTypeIngredient: true,
	ProductTypeComposite:  true,
	ProductTypeBoth:       true,
}

func NewProductType(value string) (ProductType, error) {
	if value == "" {
		return "", productError.NewMissingProductTypeError()
	}

	pt := ProductType(value)
	if !validProductTypes[pt] {
		return "", productError.NewInvalidProductTypeError(value)
	}

	return pt, nil
}

func (pt ProductType) Value() string {
	return string(pt)
}

func (pt ProductType) String() string {
	return string(pt)
}

func (pt ProductType) Equals(other ProductType) bool {
	return pt == other
}

func (pt ProductType) IsSellable() bool {
	return pt == ProductTypeSellable || pt == ProductTypeBoth || pt == ProductTypeComposite
}

func (pt ProductType) IsIngredient() bool {
	return pt == ProductTypeIngredient || pt == ProductTypeBoth
}
