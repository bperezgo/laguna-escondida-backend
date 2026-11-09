package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

type MarketingEventType string

const (
	MarketingEventOrderPaid            MarketingEventType = "order.paid"
	MarketingEventOrderCancelled       MarketingEventType = "order.cancelled"
	MarketingEventReservationConfirmed MarketingEventType = "reservation.confirmed"
	MarketingEventReservationDeclined  MarketingEventType = "reservation.declined"
	MarketingEventReservationCancelled MarketingEventType = "reservation.cancelled"
	MarketingEventFeedbackRequested    MarketingEventType = "feedback.requested"
)

const (
	MarketingEventsPathOrders        = "/backend/events"
	MarketingEventsPathNotifications = "/backend/notifications"
)

type MarketingEventStatus string

const (
	MarketingEventPending  MarketingEventStatus = "pending"
	MarketingEventSent     MarketingEventStatus = "sent"
	MarketingEventRejected MarketingEventStatus = "rejected"
	MarketingEventExpired  MarketingEventStatus = "expired"
)

// MarketingEvent is one outbox row. Body is the exact bytes signed and sent on every attempt.
type MarketingEvent struct {
	EventID       string
	Type          MarketingEventType
	Path          string
	Body          []byte
	Status        MarketingEventStatus
	Attempts      int
	NextAttemptAt time.Time
	LastError     *string
	CreatedAt     time.Time
	SentAt        *time.Time
}

// MarketingOrderSubject is what an order event needs from the order.
type MarketingOrderSubject struct {
	OrderID       string
	CustomerPhone *string
	Origin        *Origin
	Amount        decimal.Decimal
	OccurredAt    time.Time
}

// MarketingDeliveryOutcome is how the receiver answered one attempt; Status is 0 when no answer came.
type MarketingDeliveryOutcome struct {
	Status int
	Err    error
}

// OrderOriginState is an order's payment state and who to tell about it.
type OrderOriginState struct {
	PaidAt        *time.Time
	CustomerPhone *string
	Origin        *Origin
}
