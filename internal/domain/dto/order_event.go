package dto

import (
	"encoding/json"
	"time"

	"laguna-escondida/backend/internal/platform/shared/domain/event"
)

const OrderCreatedEventName = "order.created"
const OrderDeletedEventName = "order.deleted"
const OrderUpdatedEventName = "order.updated"
const OrderReleasedToKitchenEventName = "order.released_to_kitchen"

// OrderCreatedEventProduct mirrors OrderProductItem field-for-field (same names, types, order)
// so NewOrderCreatedEvent/NewOrderUpdatedEvent can build it via a direct struct conversion
// (OrderCreatedEventProduct(p)). SideDishes carries the full resolved set (design D7) so both
// the stock handler and the SSE consumer read the same per-line selection off one event.
type OrderCreatedEventProduct struct {
	OpenBillProductID string              `json:"open_bill_product_id"`
	ProductID         string              `json:"product_id"`
	Quantity          int                 `json:"quantity"`
	Notes             *string             `json:"notes,omitempty"`
	SideDishes        []SideDishSelection `json:"side_dishes,omitempty"`
}

type OrderCreatedEvent struct {
	event.BaseEvent
	EventOrganization
	OpenBillID         string `json:"open_bill_id"`
	TemporalIdentifier string `json:"temporal_identifier"`
	CreatedByID        string `json:"created_by_id"`
	// CreatedAt is the order's creation instant. It is exported (unlike BaseEvent's
	// unexported occurredAt, which does NOT survive the JSON round-trip through the
	// event bus) so the SSE consumer can stamp the live "created" payload with a real
	// timestamp — otherwise the kitchen countdown starts from Go's zero time and every
	// new line renders as "¡URGENTE!" until a refresh reloads the DB snapshot.
	CreatedAt time.Time                  `json:"created_at"`
	Products  []OrderCreatedEventProduct `json:"products"`
	// OrderType lets the kitchen feed skip held orders; the stock handler ignores it.
	OrderType OrderType `json:"order_type"`
}

func NewOrderCreatedEvent(
	openBillID string,
	temporalIdentifier string,
	createdByID string,
	createdAt time.Time,
	products []OrderProductItem,
) OrderCreatedEvent {
	eventProducts := make([]OrderCreatedEventProduct, len(products))
	for i, p := range products {
		eventProducts[i] = OrderCreatedEventProduct(p)
	}

	return OrderCreatedEvent{
		BaseEvent:          event.NewBaseEvent(OrderCreatedEventName, openBillID),
		OpenBillID:         openBillID,
		TemporalIdentifier: temporalIdentifier,
		CreatedByID:        createdByID,
		CreatedAt:          createdAt,
		Products:           eventProducts,
	}
}

func (e OrderCreatedEvent) Data() []byte {
	payload := struct {
		EventID            string                     `json:"event_id"`
		EventName          string                     `json:"event_name"`
		AggregateID        string                     `json:"aggregate_id"`
		OccurredAt         time.Time                  `json:"occurred_at"`
		OpenBillID         string                     `json:"open_bill_id"`
		TemporalIdentifier string                     `json:"temporal_identifier"`
		CreatedByID        string                     `json:"created_by_id"`
		CreatedAt          time.Time                  `json:"created_at"`
		Products           []OrderCreatedEventProduct `json:"products"`
		OrderType          OrderType                  `json:"order_type"`
	}{
		EventID:            e.EventID(),
		EventName:          e.EventName(),
		AggregateID:        e.AggregateID(),
		OccurredAt:         e.OccurredAt(),
		OpenBillID:         e.OpenBillID,
		TemporalIdentifier: e.TemporalIdentifier,
		CreatedByID:        e.CreatedByID,
		CreatedAt:          e.CreatedAt,
		Products:           e.Products,
		OrderType:          e.OrderType,
	}
	data, _ := json.Marshal(payload)
	return data
}

var _ event.DomainEvent = (*OrderCreatedEvent)(nil)

type OrderDeletedEvent struct {
	event.BaseEvent
	EventOrganization
	OpenBillID string `json:"open_bill_id"`
	// Products are the order's line items captured before the soft-delete, so the stock
	// handler can restore on-hand inventory without re-reading the deleted order. They are
	// exported (like OrderCreatedEvent.Products) so they survive the JSON round-trip through
	// the event bus — BaseEvent's unexported fields do not.
	Products  []OrderCreatedEventProduct `json:"products"`
	OrderType OrderType                  `json:"order_type"`
}

func NewOrderDeletedEvent(openBillID string, products []OpenBillProductDetail) OrderDeletedEvent {
	eventProducts := make([]OrderCreatedEventProduct, len(products))
	for i, p := range products {
		eventProducts[i] = OrderCreatedEventProduct{
			OpenBillProductID: p.OpenBillProductID,
			ProductID:         p.Product.ID,
			Quantity:          p.Quantity,
			Notes:             p.Notes,
			SideDishes:        p.SideDishes,
		}
	}

	return OrderDeletedEvent{
		BaseEvent:  event.NewBaseEvent(OrderDeletedEventName, openBillID),
		OpenBillID: openBillID,
		Products:   eventProducts,
	}
}

func (e OrderDeletedEvent) Data() []byte {
	payload := struct {
		EventID     string                     `json:"event_id"`
		EventName   string                     `json:"event_name"`
		AggregateID string                     `json:"aggregate_id"`
		OccurredAt  time.Time                  `json:"occurred_at"`
		OpenBillID  string                     `json:"open_bill_id"`
		Products    []OrderCreatedEventProduct `json:"products"`
		OrderType   OrderType                  `json:"order_type"`
	}{
		EventID:     e.EventID(),
		EventName:   e.EventName(),
		AggregateID: e.AggregateID(),
		OccurredAt:  e.OccurredAt(),
		OpenBillID:  e.OpenBillID,
		Products:    e.Products,
		OrderType:   e.OrderType,
	}
	data, _ := json.Marshal(payload)
	return data
}

var _ event.DomainEvent = (*OrderDeletedEvent)(nil)

type OrderUpdatedEvent struct {
	event.BaseEvent
	EventOrganization
	OpenBillID         string                     `json:"open_bill_id"`
	TemporalIdentifier string                     `json:"temporal_identifier"`
	CreatedByID        string                     `json:"created_by_id"`
	PreviousProducts   []OrderCreatedEventProduct `json:"previous_products"`
	CurrentProducts    []OrderCreatedEventProduct `json:"current_products"`
	OrderType          OrderType                  `json:"order_type"`
}

func NewOrderUpdatedEvent(
	openBillID string,
	temporalIdentifier string,
	createdByID string,
	previousProducts []OpenBillProductDetail,
	currentProducts []OrderProductItem,
) OrderUpdatedEvent {
	prevProducts := make([]OrderCreatedEventProduct, len(previousProducts))
	for i, p := range previousProducts {
		prevProducts[i] = OrderCreatedEventProduct{
			OpenBillProductID: p.OpenBillProductID,
			ProductID:         p.Product.ID,
			Quantity:          p.Quantity,
			Notes:             p.Notes,
			SideDishes:        p.SideDishes,
		}
	}

	currProducts := make([]OrderCreatedEventProduct, len(currentProducts))
	for i, p := range currentProducts {
		currProducts[i] = OrderCreatedEventProduct(p)
	}

	return OrderUpdatedEvent{
		BaseEvent:          event.NewBaseEvent(OrderUpdatedEventName, openBillID),
		OpenBillID:         openBillID,
		TemporalIdentifier: temporalIdentifier,
		CreatedByID:        createdByID,
		PreviousProducts:   prevProducts,
		CurrentProducts:    currProducts,
	}
}

func (e OrderUpdatedEvent) Data() []byte {
	payload := struct {
		EventID            string                     `json:"event_id"`
		EventName          string                     `json:"event_name"`
		AggregateID        string                     `json:"aggregate_id"`
		OccurredAt         time.Time                  `json:"occurred_at"`
		OpenBillID         string                     `json:"open_bill_id"`
		TemporalIdentifier string                     `json:"temporal_identifier"`
		CreatedByID        string                     `json:"created_by_id"`
		PreviousProducts   []OrderCreatedEventProduct `json:"previous_products"`
		CurrentProducts    []OrderCreatedEventProduct `json:"current_products"`
		OrderType          OrderType                  `json:"order_type"`
	}{
		EventID:            e.EventID(),
		EventName:          e.EventName(),
		AggregateID:        e.AggregateID(),
		OccurredAt:         e.OccurredAt(),
		OpenBillID:         e.OpenBillID,
		TemporalIdentifier: e.TemporalIdentifier,
		CreatedByID:        e.CreatedByID,
		PreviousProducts:   e.PreviousProducts,
		CurrentProducts:    e.CurrentProducts,
		OrderType:          e.OrderType,
	}
	data, _ := json.Marshal(payload)
	return data
}

var _ event.DomainEvent = (*OrderUpdatedEvent)(nil)

// OrderReleasedToKitchenEvent puts a paid takeaway/delivery order's lines on the kitchen board.
// A separate event, not a re-published OrderCreatedEvent: the stock handler subscribes to that
// one and would deduct the order's stock a second time.
type OrderReleasedToKitchenEvent struct {
	event.BaseEvent
	EventOrganization
	OpenBillID         string                     `json:"open_bill_id"`
	TemporalIdentifier string                     `json:"temporal_identifier"`
	CreatedByID        string                     `json:"created_by_id"`
	OrderType          OrderType                  `json:"order_type"`
	PaidAt             time.Time                  `json:"paid_at"`
	Products           []OrderCreatedEventProduct `json:"products"`
}

// NewOrderReleasedToKitchenEvent skips cancelled lines: they never reach the board.
func NewOrderReleasedToKitchenEvent(
	openBillID string,
	temporalIdentifier string,
	createdByID string,
	orderType OrderType,
	paidAt time.Time,
	lines []OpenBillProductDetail,
) OrderReleasedToKitchenEvent {
	products := make([]OrderCreatedEventProduct, 0, len(lines))
	for _, p := range lines {
		if p.Status == CommandStatusCancelled {
			continue
		}
		products = append(products, OrderCreatedEventProduct{
			OpenBillProductID: p.OpenBillProductID,
			ProductID:         p.Product.ID,
			Quantity:          p.Quantity,
			Notes:             p.Notes,
			SideDishes:        p.SideDishes,
		})
	}

	return OrderReleasedToKitchenEvent{
		BaseEvent:          event.NewBaseEvent(OrderReleasedToKitchenEventName, openBillID),
		OpenBillID:         openBillID,
		TemporalIdentifier: temporalIdentifier,
		CreatedByID:        createdByID,
		OrderType:          orderType,
		PaidAt:             paidAt,
		Products:           products,
	}
}

func (e OrderReleasedToKitchenEvent) Data() []byte {
	payload := struct {
		EventID            string                     `json:"event_id"`
		EventName          string                     `json:"event_name"`
		AggregateID        string                     `json:"aggregate_id"`
		OccurredAt         time.Time                  `json:"occurred_at"`
		OpenBillID         string                     `json:"open_bill_id"`
		TemporalIdentifier string                     `json:"temporal_identifier"`
		CreatedByID        string                     `json:"created_by_id"`
		OrderType          OrderType                  `json:"order_type"`
		PaidAt             time.Time                  `json:"paid_at"`
		Products           []OrderCreatedEventProduct `json:"products"`
	}{
		EventID:            e.EventID(),
		EventName:          e.EventName(),
		AggregateID:        e.AggregateID(),
		OccurredAt:         e.OccurredAt(),
		OpenBillID:         e.OpenBillID,
		TemporalIdentifier: e.TemporalIdentifier,
		CreatedByID:        e.CreatedByID,
		OrderType:          e.OrderType,
		PaidAt:             e.PaidAt,
		Products:           e.Products,
	}
	data, _ := json.Marshal(payload)
	return data
}

var _ event.DomainEvent = (*OrderReleasedToKitchenEvent)(nil)
