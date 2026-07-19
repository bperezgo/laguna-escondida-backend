package dto

import (
	"encoding/json"
	"time"

	"laguna-escondida/backend/internal/platform/shared/domain/event"
)

const OrderCreatedEventName = "order.created"
const OrderDeletedEventName = "order.deleted"
const OrderUpdatedEventName = "order.updated"

type OrderCreatedEventProduct struct {
	OpenBillProductID string  `json:"open_bill_product_id"`
	ProductID         string  `json:"product_id"`
	Quantity          int     `json:"quantity"`
	Notes             *string `json:"notes,omitempty"`
}

type OrderCreatedEvent struct {
	event.BaseEvent
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
	}
	data, _ := json.Marshal(payload)
	return data
}

var _ event.DomainEvent = (*OrderCreatedEvent)(nil)

type OrderDeletedEvent struct {
	event.BaseEvent
	OpenBillID string `json:"open_bill_id"`
	// Products are the order's line items captured before the soft-delete, so the stock
	// handler can restore on-hand inventory without re-reading the deleted order. They are
	// exported (like OrderCreatedEvent.Products) so they survive the JSON round-trip through
	// the event bus — BaseEvent's unexported fields do not.
	Products []OrderCreatedEventProduct `json:"products"`
}

func NewOrderDeletedEvent(openBillID string, products []OpenBillProductDetail) OrderDeletedEvent {
	eventProducts := make([]OrderCreatedEventProduct, len(products))
	for i, p := range products {
		eventProducts[i] = OrderCreatedEventProduct{
			OpenBillProductID: p.OpenBillProductID,
			ProductID:         p.Product.ID,
			Quantity:          p.Quantity,
			Notes:             p.Notes,
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
	}{
		EventID:     e.EventID(),
		EventName:   e.EventName(),
		AggregateID: e.AggregateID(),
		OccurredAt:  e.OccurredAt(),
		OpenBillID:  e.OpenBillID,
		Products:    e.Products,
	}
	data, _ := json.Marshal(payload)
	return data
}

var _ event.DomainEvent = (*OrderDeletedEvent)(nil)

type OrderUpdatedEvent struct {
	event.BaseEvent
	OpenBillID         string                     `json:"open_bill_id"`
	TemporalIdentifier string                     `json:"temporal_identifier"`
	CreatedByID        string                     `json:"created_by_id"`
	PreviousProducts   []OrderCreatedEventProduct `json:"previous_products"`
	CurrentProducts    []OrderCreatedEventProduct `json:"current_products"`
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
	}
	data, _ := json.Marshal(payload)
	return data
}

var _ event.DomainEvent = (*OrderUpdatedEvent)(nil)
