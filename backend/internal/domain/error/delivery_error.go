package error

import (
	"errors"
	"fmt"
)

var (
	ErrDeliveryOrderNotFound = errors.New("delivery order not found")
	ErrDeliveryTransition    = errors.New("the order cannot make that delivery change")
	ErrDeliveryForbidden     = errors.New("you may not make that delivery change")
	ErrDeliveryOperation     = errors.New("delivery operation failed")
)

// DeliveryTransitionError carries the order's current status for the 409; empty when the order
// has none (unpaid, voided or dine-in).
type DeliveryTransitionError struct {
	Current string
	Action  string
}

func (e *DeliveryTransitionError) Error() string {
	if e.Current == "" {
		return fmt.Sprintf("an order without a delivery status cannot %s", e.Action)
	}
	return fmt.Sprintf("the order is %s and cannot %s", e.Current, e.Action)
}

func (e *DeliveryTransitionError) Unwrap() error {
	return ErrDeliveryTransition
}
