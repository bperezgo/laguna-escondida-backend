package error

import (
	"errors"
	"fmt"
)

var (
	ErrIntegrationNotFound        = errors.New("organization integration not found")
	ErrIntegrationProviderUnknown = errors.New("unknown integration provider")
	// ErrIntegrationsUnavailable: the cloud has no key to encrypt integration secrets.
	ErrIntegrationsUnavailable = errors.New("organization integrations are not configured for this deployment")
)

// IntegrationValidationError names the rejected field, never its value.
type IntegrationValidationError struct {
	Field  string
	Reason string
}

func (e *IntegrationValidationError) Error() string {
	if e.Field == "" {
		return e.Reason
	}
	return fmt.Sprintf("invalid %s: %s", e.Field, e.Reason)
}
