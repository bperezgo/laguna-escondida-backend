package service

import (
	"context"
	"encoding/json"
	"fmt"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
)

// MarketingPaymentSyncApplier records order.paid when a pushed snapshot is what first marks an
// agent order paid in the cloud: a takeaway taken by the agent and paid at the restaurant's till.
type MarketingPaymentSyncApplier struct {
	inner  ports.SyncApplier
	orders ports.OrderOriginReader
	events ports.MarketingEventRecorder
}

func NewMarketingPaymentSyncApplier(inner ports.SyncApplier, orders ports.OrderOriginReader, events ports.MarketingEventRecorder) *MarketingPaymentSyncApplier {
	return &MarketingPaymentSyncApplier{inner: inner, orders: orders, events: events}
}

func (a *MarketingPaymentSyncApplier) Apply(ctx context.Context, op *dto.SyncOutboxEntry) error {
	if op.Operation == dto.SyncOperationDelete {
		return a.inner.Apply(ctx, op)
	}
	var payload dto.OpenBillSyncPayload
	if err := json.Unmarshal(op.Payload, &payload); err != nil || payload.PaidAt == nil {
		return a.inner.Apply(ctx, op)
	}
	before, found, err := a.orders.FindOrderOrigin(ctx, payload.ID)
	if err != nil {
		return fmt.Errorf("read order before applying its payment: %w", err)
	}
	if err := a.inner.Apply(ctx, op); err != nil {
		return err
	}
	if !found || before.PaidAt != nil || before.Origin == nil {
		return nil
	}
	return a.events.RecordOrder(ctx, dto.MarketingEventOrderPaid, dto.MarketingOrderSubject{
		OrderID:       payload.ID,
		CustomerPhone: before.CustomerPhone,
		Origin:        before.Origin,
		Amount:        payload.TotalAmount,
		OccurredAt:    *payload.PaidAt,
	})
}
