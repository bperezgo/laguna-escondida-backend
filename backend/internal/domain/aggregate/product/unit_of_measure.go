package product

import (
	productError "laguna-escondida/backend/internal/domain/aggregate/product/error"
)

type UnitOfMeasure string

const (
	UnitOfMeasureUnit       UnitOfMeasure = "unit"
	UnitOfMeasureKilogram   UnitOfMeasure = "kg"
	UnitOfMeasureGram       UnitOfMeasure = "g"
	UnitOfMeasureLiter      UnitOfMeasure = "l"
	UnitOfMeasureMilliliter UnitOfMeasure = "ml"
)

var validUnitsOfMeasure = map[UnitOfMeasure]bool{
	UnitOfMeasureUnit:       true,
	UnitOfMeasureKilogram:   true,
	UnitOfMeasureGram:       true,
	UnitOfMeasureLiter:      true,
	UnitOfMeasureMilliliter: true,
}

func NewUnitOfMeasure(value string) (UnitOfMeasure, error) {
	if value == "" {
		return "", productError.NewMissingUnitOfMeasureError()
	}

	uom := UnitOfMeasure(value)
	if !validUnitsOfMeasure[uom] {
		return "", productError.NewInvalidUnitOfMeasureError(value)
	}

	return uom, nil
}

func (uom UnitOfMeasure) Value() string {
	return string(uom)
}

func (uom UnitOfMeasure) String() string {
	return string(uom)
}

func (uom UnitOfMeasure) Equals(other UnitOfMeasure) bool {
	return uom == other
}
