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
	// ErrChargePending: the organization already has a charge waiting for the provider's answer.
	ErrChargePending = errors.New("a charge is pending")
	// ErrChargeNotFound: no charge has the reference a transaction carries.
	ErrChargeNotFound = errors.New("charge not found")
	// ErrBillingTransactionNotFound: the provider knows no transaction with that id or reference.
	ErrBillingTransactionNotFound = errors.New("billing transaction not found")
	// ErrNoSubscription: the organization has no saved card.
	ErrNoSubscription = errors.New("organization has no subscription")
	// ErrSubscriptionLapsed: the due date has passed, so a cancellation can no longer be removed.
	ErrSubscriptionLapsed = errors.New("subscription lapsed")
	// ErrOrganizationNotStopped: recovery is only for an expired trial or a missed payment.
	ErrOrganizationNotStopped = errors.New("organization is not stopped")
	// ErrChargingOff: the cloud has no events or integrity secret, so nothing can be charged.
	ErrChargingOff = errors.New("charging is off")
)

type BillingValidationError struct {
	Field  string
	Reason string
}

func (e *BillingValidationError) Error() string {
	return fmt.Sprintf("invalid %s: %s", e.Field, e.Reason)
}
