package error

import (
	"errors"
	"fmt"
)

var (
	ErrNotificationChannelNotFound    = errors.New("notification channel not found")
	ErrNotificationChannelOperation   = errors.New("notification channel operation failed")
	ErrNotificationRuleNotFound       = errors.New("notification rule not found")
	ErrNotificationRuleOperation      = errors.New("notification rule operation failed")
	ErrNotificationChannelUnreachable = errors.New("notification channel unreachable")
)

// NotificationChannelUnreachableError carries the destination's status; 0 when it never answered.
type NotificationChannelUnreachableError struct {
	DestinationStatus int
}

func (e *NotificationChannelUnreachableError) Error() string {
	return fmt.Sprintf("%s (destination status %d)", ErrNotificationChannelUnreachable, e.DestinationStatus)
}

func (e *NotificationChannelUnreachableError) Unwrap() error {
	return ErrNotificationChannelUnreachable
}
