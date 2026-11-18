package customer

import (
	"regexp"
	"strings"

	customerError "laguna-escondida/backend/internal/domain/aggregate/customer/error"
)

type DocumentNumber struct {
	value string
}

var documentNumberRegex = regexp.MustCompile(`^[0-9\-]+$`)

func NewDocumentNumber(value string) (*DocumentNumber, error) {
	trimmedValue := strings.TrimSpace(value)

	if trimmedValue == "" {
		return nil, customerError.NewMissingDocumentNumberError()
	}

	if !documentNumberRegex.MatchString(trimmedValue) {
		return nil, customerError.NewInvalidDocumentNumberError(value)
	}

	return &DocumentNumber{value: trimmedValue}, nil
}

func (d *DocumentNumber) Value() string {
	return d.value
}

func (d *DocumentNumber) String() string {
	return d.value
}

func (d *DocumentNumber) Equals(other *DocumentNumber) bool {
	if other == nil {
		return false
	}
	return d.value == other.value
}
