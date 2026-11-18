package error

import "errors"

var (
	ErrAPIKeyNotFound        = errors.New("api key not found")
	ErrAPIKeyUserNotEligible = errors.New("an api key can only be issued for an active user of the organization whose only role is agent")
	ErrAPIKeyNameRequired    = errors.New("api key name is required")
	ErrAPIKeyNameTooLong     = errors.New("api key name is longer than 255 characters")
	ErrCredentialRefused     = errors.New("credential refused")
)

// APIKeyRefusedError is ErrCredentialRefused with the reason, for the server log only: the
// caller always sees the same refusal.
type APIKeyRefusedError struct {
	Reason string
}

func (e *APIKeyRefusedError) Error() string { return "credential refused: " + e.Reason }

func (e *APIKeyRefusedError) Is(target error) bool { return target == ErrCredentialRefused }
