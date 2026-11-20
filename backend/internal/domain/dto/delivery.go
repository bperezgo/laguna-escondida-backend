package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// DeliveryStatus is where a paid takeaway or delivery order is; preparing and ready are derived
// from its lines, the others are the last recorded change.
type DeliveryStatus string

const (
	DeliveryStatusPreparing      DeliveryStatus = "preparing"
	DeliveryStatusReady          DeliveryStatus = "ready"
	DeliveryStatusOutForDelivery DeliveryStatus = "out_for_delivery"
	DeliveryStatusDelivered      DeliveryStatus = "delivered"
	DeliveryStatusFailed         DeliveryStatus = "failed"
)

func (s DeliveryStatus) Valid() bool {
	switch s {
	case DeliveryStatusPreparing, DeliveryStatusReady, DeliveryStatusOutForDelivery, DeliveryStatusDelivered, DeliveryStatusFailed:
		return true
	}
	return false
}

// IsRecorded reports whether the status is stored rather than derived from the lines.
func (s DeliveryStatus) IsRecorded() bool {
	return s == DeliveryStatusOutForDelivery || s == DeliveryStatusDelivered || s == DeliveryStatusFailed
}

type DeliveryFailureReason string

const (
	DeliveryFailureCustomerUnreachable DeliveryFailureReason = "customer_unreachable"
	DeliveryFailureWrongAddress        DeliveryFailureReason = "wrong_address"
	DeliveryFailureRefused             DeliveryFailureReason = "refused"
	DeliveryFailureOther               DeliveryFailureReason = "other"
)

func (r DeliveryFailureReason) Valid() bool {
	switch r {
	case DeliveryFailureCustomerUnreachable, DeliveryFailureWrongAddress, DeliveryFailureRefused, DeliveryFailureOther:
		return true
	}
	return false
}

type DeliveryAction string

const (
	DeliveryActionDispatch DeliveryAction = "dispatch"
	DeliveryActionDeliver  DeliveryAction = "deliver"
	DeliveryActionFail     DeliveryAction = "fail"
)

const (
	MaxCourierNameLength  = 80
	MaxDeliveryNoteLength = 500
)

// DeliveryCourier is a courier user (UserID set) or a third party known only by name.
type DeliveryCourier struct {
	UserID *string `json:"user_id,omitempty"`
	Name   string  `json:"name"`
}

type DispatchDeliveryRequest struct {
	CourierUserID *string `json:"courier_user_id,omitempty"`
	CourierName   *string `json:"courier_name,omitempty"`
}

type FailDeliveryRequest struct {
	Reason DeliveryFailureReason `json:"reason"`
	Note   *string               `json:"note,omitempty"`
}

type DeliveryListFilter struct {
	Status    DeliveryStatus
	OrderType OrderType
}

type DeliveryOrderLine struct {
	Name     string        `json:"name"`
	Quantity int           `json:"quantity"`
	Notes    *string       `json:"notes,omitempty"`
	Status   CommandStatus `json:"status"`
}

// DeliveryOrder carries no amounts: couriers read it too, and only need to know it is paid.
type DeliveryOrder struct {
	ID                 string                 `json:"id"`
	TemporalIdentifier string                 `json:"temporal_identifier"`
	Descriptor         *string                `json:"descriptor,omitempty"`
	OrderType          OrderType              `json:"order_type"`
	CustomerName       *string                `json:"customer_name,omitempty"`
	CustomerPhone      *string                `json:"customer_phone,omitempty"`
	DeliveryAddress    *string                `json:"delivery_address,omitempty"`
	DeliveryLat        *decimal.Decimal       `json:"delivery_lat,omitempty"`
	DeliveryLng        *decimal.Decimal       `json:"delivery_lng,omitempty"`
	DeliveryMapURL     *string                `json:"delivery_map_url,omitempty"`
	DeliveryStatus     DeliveryStatus         `json:"delivery_status"`
	StatusSince        time.Time              `json:"status_since"`
	Courier            *DeliveryCourier       `json:"courier,omitempty"`
	FailureReason      *DeliveryFailureReason `json:"failure_reason,omitempty"`
	Paid               bool                   `json:"paid"`
	PaidAt             time.Time              `json:"paid_at"`
	ReadyAt            *time.Time             `json:"ready_at,omitempty"`
	CreatedAt          time.Time              `json:"created_at"`
	Lines              []DeliveryOrderLine    `json:"lines"`
}

type DeliveryOrderList struct {
	Orders []DeliveryOrder `json:"orders"`
}

type CourierQueue struct {
	Claimable []DeliveryOrder `json:"claimable"`
	OnTheRoad []DeliveryOrder `json:"on_the_road"`
}

type DeliveryActorRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type DeliveryHistoryEntry struct {
	Status     DeliveryStatus         `json:"status"`
	OccurredAt time.Time              `json:"occurred_at"`
	Actor      *DeliveryActorRef      `json:"actor,omitempty"`
	Courier    *DeliveryCourier       `json:"courier,omitempty"`
	Reason     *DeliveryFailureReason `json:"reason,omitempty"`
	Note       *string                `json:"note,omitempty"`
}

type DeliveryHistory struct {
	OrderID string                 `json:"order_id"`
	PaidAt  time.Time              `json:"paid_at"`
	ReadyAt *time.Time             `json:"ready_at,omitempty"`
	Entries []DeliveryHistoryEntry `json:"entries"`
}

type CourierUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type CourierList struct {
	Couriers []CourierUser `json:"couriers"`
}

// DeliveryTrackingState is what the transition policy needs to know about an order.
type DeliveryTrackingState struct {
	OrderID       string
	OrderType     OrderType
	Paid          bool
	Voided        bool
	Status        DeliveryStatus
	CourierUserID *string
	CourierName   *string
	Fulfillment   OrderFulfillment
}

// DeliveryStatusChange is one recorded change: the open_bills update and its history entry.
// ExpectedStatus/ExpectedCourierUserID are what the policy saw; nil status means not recorded yet.
type DeliveryStatusChange struct {
	EventID               string
	OrderID               string
	ExpectedStatus        *DeliveryStatus
	ExpectedCourierUserID *string
	Status                DeliveryStatus
	CourierUserID         *string
	CourierName           *string
	Reason                *DeliveryFailureReason
	Note                  *string
	ActorUserID           string
	OccurredAt            time.Time
}
