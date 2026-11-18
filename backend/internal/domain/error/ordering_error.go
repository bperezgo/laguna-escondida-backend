package error

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrOrderingPaused       = errors.New("ordering is paused")
	ErrOutsideOrderingHours = errors.New("outside ordering hours")
)

// OrderingPausedError carries what the caller relays: which type is paused and why.
type OrderingPausedError struct {
	OrderType    string
	PausedReason *string
}

func (e *OrderingPausedError) Error() string {
	return fmt.Sprintf("%s orders are paused", e.OrderType)
}

func (e *OrderingPausedError) Unwrap() error {
	return ErrOrderingPaused
}

// OutsideOrderingHoursError: NextOpensAt is nil when no window opens within the next week.
type OutsideOrderingHoursError struct {
	OrderType   string
	NextOpensAt *time.Time
}

func (e *OutsideOrderingHoursError) Error() string {
	return fmt.Sprintf("%s orders are outside their hours", e.OrderType)
}

func (e *OutsideOrderingHoursError) Unwrap() error {
	return ErrOutsideOrderingHours
}

// OrderingSettingsValidationError names the rejected field, e.g. ordering.delivery.schedule[2].
type OrderingSettingsValidationError struct {
	Field  string
	Reason string
}

func (e *OrderingSettingsValidationError) Error() string {
	return fmt.Sprintf("invalid %s: %s", e.Field, e.Reason)
}
