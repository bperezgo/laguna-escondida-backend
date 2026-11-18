package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

const (
	DomainEventOrderCreated         = "order.created"
	DomainEventOrderPaid            = "order.paid"
	DomainEventReservationRequested = "reservation.requested"
	DomainEventReservationConfirmed = "reservation.confirmed"
)

// DomainEventMeta is what every durable event carries; ID is a UUID v7 and Subject is the id of the order or reservation.
type DomainEventMeta struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	OrganizationID string    `json:"organization_id"`
	Subject        string    `json:"subject_id"`
	Time           time.Time `json:"occurred_at"`
}

func (m DomainEventMeta) EventID() string       { return m.ID }
func (m DomainEventMeta) EventName() string     { return m.Name }
func (m DomainEventMeta) SubjectID() string     { return m.Subject }
func (m DomainEventMeta) OccurredAt() time.Time { return m.Time }

type OrderCreated struct {
	DomainEventMeta
	OrderType      OrderType       `json:"order_type"`
	Total          decimal.Decimal `json:"total"`
	CustomerName   *string         `json:"customer_name,omitempty"`
	CustomerPhone  *string         `json:"customer_phone,omitempty"`
	DeliveryZoneID *string         `json:"delivery_zone_id,omitempty"`
}

type OrderPaid struct {
	DomainEventMeta
	OrderType      OrderType       `json:"order_type"`
	Total          decimal.Decimal `json:"total"`
	AmountPaid     decimal.Decimal `json:"amount_paid"`
	CustomerName   *string         `json:"customer_name,omitempty"`
	CustomerPhone  *string         `json:"customer_phone,omitempty"`
	DeliveryZoneID *string         `json:"delivery_zone_id,omitempty"`
}

type ReservationRequested struct {
	DomainEventMeta
	GuestName     string                    `json:"guest_name"`
	GuestPhone    string                    `json:"guest_phone"`
	PartySize     int                       `json:"party_size"`
	StartsAt      time.Time                 `json:"starts_at"`
	Source        ReservationSource         `json:"source"`
	PendingReason *ReservationPendingReason `json:"pending_reason,omitempty"`
}

type ConfirmedBy string

const (
	ConfirmedByAuto  ConfirmedBy = "auto"
	ConfirmedByStaff ConfirmedBy = "staff"
)

type ReservationConfirmed struct {
	DomainEventMeta
	GuestName     string                    `json:"guest_name"`
	GuestPhone    string                    `json:"guest_phone"`
	PartySize     int                       `json:"party_size"`
	StartsAt      time.Time                 `json:"starts_at"`
	Source        ReservationSource         `json:"source"`
	PendingReason *ReservationPendingReason `json:"pending_reason,omitempty"`
	ConfirmedBy   ConfirmedBy               `json:"confirmed_by"`
}

// StoredEvent is an event as persisted; Payload is the concrete event's JSON.
type StoredEvent struct {
	ID             string
	Name           string
	OrganizationID string
	SubjectID      string
	OccurredAt     time.Time
	Payload        []byte
}

type EventDeliveryStatus string

const (
	EventDeliveryPending EventDeliveryStatus = "pending"
	EventDeliveryDone    EventDeliveryStatus = "done"
	EventDeliveryDead    EventDeliveryStatus = "dead"
)

// ClaimedDelivery is one leased delivery with the event it is for; Attempts includes the claim.
type ClaimedDelivery struct {
	Event    StoredEvent
	Consumer string
	Attempts int
}
