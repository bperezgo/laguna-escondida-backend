package supplier

import (
	"regexp"
	"strings"

	supplierError "laguna-escondida/backend/internal/domain/aggregate/supplier/error"
)

// Phone number validation: allows digits, spaces, dashes, parentheses, and plus sign
var phoneRegex = regexp.MustCompile(`^[\d\s\-\(\)\+]+$`)

type Phone struct {
	value string
}

func NewPhone(value string) (*Phone, error) {
	trimmed := strings.TrimSpace(value)

	// Empty phone is allowed (optional field)
	if trimmed == "" {
		return &Phone{value: ""}, nil
	}

	if !phoneRegex.MatchString(trimmed) {
		return nil, supplierError.NewInvalidPhoneError(value)
	}

	return &Phone{value: trimmed}, nil
}

func (p *Phone) Value() string {
	return p.value
}

func (p *Phone) String() string {
	return p.value
}

func (p *Phone) IsEmpty() bool {
	return p.value == ""
}

func (p *Phone) Equals(other *Phone) bool {
	if other == nil {
		return false
	}
	return p.value == other.value
}
