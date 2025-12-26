package product

import (
	"regexp"

	productError "laguna-escondida/backend/internal/domain/aggregate/product/error"
)

// SKU represents a Stock Keeping Unit value object
// SKU must contain only alphanumeric characters (a-z, A-Z, 0-9) and hyphens (-)
type SKU struct {
	value string
}

var skuRegex = regexp.MustCompile(`^[a-zA-Z0-9\-]+$`)

// NewSKU creates a new SKU value object
// Returns an error if the SKU is empty or contains invalid characters
func NewSKU(value string) (*SKU, error) {
	if value == "" {
		return nil, productError.NewMissingSKUError()
	}

	if !skuRegex.MatchString(value) {
		return nil, productError.NewInvalidSKUError(value)
	}

	return &SKU{value: value}, nil
}

// Value returns the string value of the SKU
func (s *SKU) Value() string {
	return s.value
}

// String implements the fmt.Stringer interface
func (s *SKU) String() string {
	return s.value
}

// Equals checks if two SKU value objects are equal
func (s *SKU) Equals(other *SKU) bool {
	if other == nil {
		return false
	}
	return s.value == other.value
}
