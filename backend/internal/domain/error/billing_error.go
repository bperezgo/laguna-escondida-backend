package error

import (
	"errors"
	"fmt"
)

var (
	// ErrCardRefused: the provider refused the card token, or the payment source is not usable.
	ErrCardRefused = errors.New("card refused")
	// ErrPaymentProviderUnavailable: the provider did not answer, or answered something unusable.
	ErrPaymentProviderUnavailable = errors.New("payment provider unavailable")
	// ErrCardEntryOff: the cloud has no billing keys, so no card can be saved.
	ErrCardEntryOff = errors.New("card entry is off")
)

type BillingValidationError struct {
	Field  string
	Reason string
}

func (e *BillingValidationError) Error() string {
	return fmt.Sprintf("invalid %s: %s", e.Field, e.Reason)
}
