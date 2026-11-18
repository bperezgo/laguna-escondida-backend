package error

import (
	"errors"
	"fmt"
)

var (
	ErrOrganizationNotFound  = errors.New("organization not found")
	ErrOrganizationSlugTaken = errors.New("organization slug is already taken")
	ErrOrganizationRequired  = errors.New("organization is required")
	// ErrOrganizationMismatch: an edge box was asked to act for an organization other than its own.
	ErrOrganizationMismatch = errors.New("organization does not match this box's")
)

// OrganizationValidationError names the rejected field so the caller can point at it.
type OrganizationValidationError struct {
	Field  string
	Reason string
}

func (e *OrganizationValidationError) Error() string {
	return fmt.Sprintf("invalid %s: %s", e.Field, e.Reason)
}
