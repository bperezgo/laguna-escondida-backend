package dto

import (
	"time"

	"github.com/google/uuid"
)

const OrderCreatedEventName = "order.created"

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
		eventProducts[i] = OrderCreatedEventProduct{
			ProductID: p.ProductID,
			Quantity:  p.Quantity,
			Notes:     p.Notes,
		}
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
