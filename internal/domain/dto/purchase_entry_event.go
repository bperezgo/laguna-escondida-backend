package dto

import (
	"encoding/json"
	"time"

	"laguna-escondida/backend/internal/platform/shared/domain/event"

	"github.com/shopspring/decimal"
)

const PurchaseEntryCreatedEventName = "purchase_entry.created"

type PurchaseEntryCreatedEventItem struct {
	ProductID string          `json:"product_id"`
	Quantity  decimal.Decimal `json:"quantity"`
}

type PurchaseEntryCreatedEvent struct {
	event.BaseEvent
	PurchaseEntryID string                          `json:"purchase_entry_id"`
	SupplierID      string                          `json:"supplier_id"`
	Items           []PurchaseEntryCreatedEventItem `json:"items"`
}

func NewPurchaseEntryCreatedEvent(id, supplierID string, items []*PurchaseEntryItem) PurchaseEntryCreatedEvent {
	eventItems := make([]PurchaseEntryCreatedEventItem, len(items))
	for i, item := range items {
		eventItems[i] = PurchaseEntryCreatedEventItem{
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
		}
	}
	return PurchaseEntryCreatedEvent{
		BaseEvent:       event.NewBaseEvent(PurchaseEntryCreatedEventName, id),
		PurchaseEntryID: id,
		SupplierID:      supplierID,
		Items:           eventItems,
	}
}

func (e PurchaseEntryCreatedEvent) Data() []byte {
	payload := struct {
		EventID         string                          `json:"event_id"`
		EventName       string                          `json:"event_name"`
		AggregateID     string                          `json:"aggregate_id"`
		OccurredAt      time.Time                       `json:"occurred_at"`
		PurchaseEntryID string                          `json:"purchase_entry_id"`
		SupplierID      string                          `json:"supplier_id"`
		Items           []PurchaseEntryCreatedEventItem `json:"items"`
	}{
		EventID:         e.EventID(),
		EventName:       e.EventName(),
		AggregateID:     e.AggregateID(),
		OccurredAt:      e.OccurredAt(),
		PurchaseEntryID: e.PurchaseEntryID,
		SupplierID:      e.SupplierID,
		Items:           e.Items,
	}
	data, _ := json.Marshal(payload)
	return data
}

var _ event.DomainEvent = (*PurchaseEntryCreatedEvent)(nil)
