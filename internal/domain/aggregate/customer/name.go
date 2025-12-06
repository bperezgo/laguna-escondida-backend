package customer

import (
	"strings"

	customerError "laguna-escondida/backend/internal/domain/aggregate/customer/error"
)

type Name struct {
	value string
}

func NewName(value string) (*Name, error) {
	trimmedValue := strings.TrimSpace(value)
	
	if trimmedValue == "" {
		return nil, customerError.NewMissingNameError()
	}

	return &Name{value: trimmedValue}, nil
}

func (n *Name) Value() string {
	return n.value
}

func (n *Name) String() string {
	return n.value
}

func (n *Name) Equals(other *Name) bool {
	if other == nil {
		return false
	}
	return n.value == other.value
}

