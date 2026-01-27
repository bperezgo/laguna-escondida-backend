package supplier

import (
	"strings"

	supplierError "laguna-escondida/backend/internal/domain/aggregate/supplier/error"
)

type Name struct {
	value string
}

func NewName(value string) (*Name, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, supplierError.NewMissingNameError()
	}

	return &Name{value: trimmed}, nil
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
