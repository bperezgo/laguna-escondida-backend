package customer

import (
	"regexp"
	"strings"

	customerError "laguna-escondida/backend/internal/domain/aggregate/customer/error"
)

type Email struct {
	value string
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

func NewEmail(value string) (*Email, error) {
	trimmedValue := strings.TrimSpace(value)

	if trimmedValue == "" {
		return nil, customerError.NewMissingEmailError()
	}

	if !emailRegex.MatchString(trimmedValue) {
		return nil, customerError.NewInvalidEmailError(value)
	}

	return &Email{value: trimmedValue}, nil
}

func (e *Email) Value() string {
	return e.value
}

func (e *Email) String() string {
	return e.value
}

func (e *Email) Equals(other *Email) bool {
	if other == nil {
		return false
	}
	return e.value == other.value
}
