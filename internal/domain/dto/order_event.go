package dto

import (
	"time"

	"github.com/google/uuid"
)

const OrderCreatedEventName = "order.created"
const OrderDeletedEventName = "order.deleted"

type OrderCreatedEventProduct struct {
	OpenBillProductID string  `json:"open_bill_product_id"`
	ProductID         string  `json:"product_id"`
	Quantity          int     `json:"quantity"`
	Notes             *string `json:"notes,omitempty"`
}

type OrderCreatedEvent struct {
	EventID            string                     `json:"event_id"`
	OpenBillID         string                     `json:"open_bill_id"`
	TemporalIdentifier string                     `json:"temporal_identifier"`
	CreatedByID        string                     `json:"created_by_id"`
	Products           []OrderCreatedEventProduct `json:"products"`
	OccurredAt         time.Time                  `json:"occurred_at"`
}

func (e OrderCreatedEvent) EventName() string {
	return OrderCreatedEventName
}

func NewOrderCreatedEvent(
	openBillID string,
	temporalIdentifier string,
	createdByID string,
	products []OrderProductItem,
) OrderCreatedEvent {
	eventProducts := make([]OrderCreatedEventProduct, len(products))
	for i, p := range products {
		eventProducts[i] = OrderCreatedEventProduct(p)
	}

	return OrderCreatedEvent{
		EventID:            uuid.NewString(),
		OpenBillID:         openBillID,
		TemporalIdentifier: temporalIdentifier,
		CreatedByID:        createdByID,
		Products:           eventProducts,
		OccurredAt:         time.Now(),
	}
}

type OrderDeletedEvent struct {
	EventID    string    `json:"event_id"`
	OpenBillID string    `json:"open_bill_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

func (e OrderDeletedEvent) EventName() string {
	return OrderDeletedEventName
}

func NewOrderDeletedEvent(openBillID string) OrderDeletedEvent {
	return OrderDeletedEvent{
		EventID:    uuid.NewString(),
		OpenBillID: openBillID,
		OccurredAt: time.Now(),
	}
}

const OrderUpdatedEventName = "order.updated"

type OrderUpdatedEvent struct {
	EventID            string                     `json:"event_id"`
	OpenBillID         string                     `json:"open_bill_id"`
	TemporalIdentifier string                     `json:"temporal_identifier"`
	CreatedByID        string                     `json:"created_by_id"`
	PreviousProducts   []OrderCreatedEventProduct `json:"previous_products"`
	CurrentProducts    []OrderCreatedEventProduct `json:"current_products"`
	OccurredAt         time.Time                  `json:"occurred_at"`
}

func (e OrderUpdatedEvent) EventName() string {
	return OrderUpdatedEventName
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
		EventID:            uuid.NewString(),
		OpenBillID:         openBillID,
		TemporalIdentifier: temporalIdentifier,
		CreatedByID:        createdByID,
		PreviousProducts:   prevProducts,
		CurrentProducts:    currProducts,
		OccurredAt:         time.Now(),
	}
}
