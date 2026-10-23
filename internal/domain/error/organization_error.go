package error

import (
	"errors"
	"fmt"
)

var (
	ErrOrganizationNotFound  = errors.New("organization not found")
	ErrOrganizationSlugTaken = errors.New("organization slug is already taken")
)

// OrganizationValidationError names the rejected field so the caller can point at it.
type OrganizationValidationError struct {
	Field  string
	Reason string
}

func (e *OrganizationValidationError) Error() string {
	return fmt.Sprintf("invalid %s: %s", e.Field, e.Reason)
}
