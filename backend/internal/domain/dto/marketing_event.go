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
	// Attendance is a presumption: visited is set by the job and staff may correct it to no_show.
	MarketingEventReservationVisited MarketingEventType = "reservation.visited"
	MarketingEventReservationNoShow  MarketingEventType = "reservation.no_show"
	// MarketingEventOrderExpired is an agent order voided unpaid; it is sent only as a notification.
	MarketingEventOrderExpired MarketingEventType = "order.expired"

	// The outbox types of the order notifications, told apart from the /backend/events rows of
	// the same change; their bodies carry the wire types order.paid and order.expired.
	MarketingEventOrderPaidNotification    MarketingEventType = "order.paid.notification"
	MarketingEventOrderExpiredNotification MarketingEventType = "order.expired.notification"

	// Delivery progress of an agent order; notifications only, recorded as <type>.notification.
	MarketingEventOrderReady          MarketingEventType = "order.ready"
	MarketingEventOrderOutForDelivery MarketingEventType = "order.out_for_delivery"
	MarketingEventOrderDelivered      MarketingEventType = "order.delivered"
	MarketingEventOrderDeliveryFailed MarketingEventType = "order.delivery_failed"
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

// MarketingDeliveryProgress is what a delivery notification tells the customer. It has no note:
// the failure note is for staff only.
type MarketingDeliveryProgress struct {
	OrderType   OrderType
	CourierName *string
	Reason      *DeliveryFailureReason
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
