package error

import (
	"errors"
	"fmt"
)

var (
	ErrContactNotFound           = errors.New("contact not found")
	ErrContactAlreadyExists      = errors.New("a contact with this phone already exists")
	ErrReservationNotFound       = errors.New("reservation not found")
	ErrReservationTransition     = errors.New("the reservation cannot move to that status")
	ErrNoVisitAwaitingFeedback   = errors.New("no visit awaiting feedback")
	ErrFeedbackRequestExists     = errors.New("feedback was already requested for this reservation")
	ErrReservationOperation      = errors.New("reservation operation failed")
	ErrFeedbackOperation         = errors.New("feedback operation failed")
	ErrMarketingEventRecordError = errors.New("marketing event could not be recorded")
)

// ValidationError names the rejected field, e.g. party_size or feedback.review_url.
type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("invalid %s: %s", e.Field, e.Reason)
}

// ReservationTransitionError says which move was refused, so a 409 can name both states.
type ReservationTransitionError struct {
	From string
	To   string
}

func (e *ReservationTransitionError) Error() string {
	return fmt.Sprintf("a %s reservation cannot become %s", e.From, e.To)
}

func (e *ReservationTransitionError) Unwrap() error {
	return ErrReservationTransition
}
