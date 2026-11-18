package supplier

import (
	"net/mail"
	"strings"

	supplierError "laguna-escondida/backend/internal/domain/aggregate/supplier/error"
)

type Email struct {
	value string
}

func NewEmail(value string) (*Email, error) {
	trimmed := strings.TrimSpace(value)

	// Empty email is allowed (optional field)
	if trimmed == "" {
		return &Email{value: ""}, nil
	}

	addr, err := mail.ParseAddress(trimmed)
	if err != nil {
		return nil, supplierError.NewInvalidEmailError(value)
	}

	// Use the address part only (in case input was "Name <email@example.com>")
	return &Email{value: addr.Address}, nil
}

func (e *Email) Value() string {
	return e.value
}

func (e *Email) String() string {
	return e.value
}

func (e *Email) IsEmpty() bool {
	return e.value == ""
}

func (e *Email) Equals(other *Email) bool {
	if other == nil {
		return false
	}
	return e.value == other.value
}
