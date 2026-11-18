package customer

import (
	customerError "laguna-escondida/backend/internal/domain/aggregate/customer/error"
	"laguna-escondida/backend/internal/domain/dto"
)

type DocumentType struct {
	value dto.DocumentType
}

func NewDocumentType(value dto.DocumentType) (*DocumentType, error) {
	if value != dto.DocumentTypeNationalIdentificationNumber &&
		value != dto.DocumentTypeNIT {
		return nil, customerError.NewInvalidDocumentTypeError(string(value))
	}

	return &DocumentType{value: value}, nil
}

func (d *DocumentType) Value() dto.DocumentType {
	return d.value
}

func (d *DocumentType) String() string {
	return string(d.value)
}

func (d *DocumentType) Equals(other *DocumentType) bool {
	if other == nil {
		return false
	}
	return d.value == other.value
}
