package main

import (
	"fmt"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/service"
	pkgports "laguna-escondida/backend/pkg/domain/ports"
	"laguna-escondida/backend/pkg/eventbus"
)

// subscribeKitchenFeed wires the order events that drive the live kitchen board.
func subscribeKitchenFeed(sub pkgports.EventSubscriber, orders *service.OrderService) error {
	handlers := []pkgports.EventHandler{
		eventbus.NewTypedEventHandler(dto.OrderCreatedEventName, orders.HandleOrderCreatedSSE),
		eventbus.NewTypedEventHandler(dto.OrderDeletedEventName, orders.HandleOrderDeletedSSE),
		eventbus.NewTypedEventHandler(dto.OrderUpdatedEventName, orders.HandleOrderUpdatedSSE),
		eventbus.NewTypedEventHandler(dto.OrderReleasedToKitchenEventName, orders.HandleOrderReleasedToKitchenSSE),
	}
	return subscribeAll(sub, handlers)
}

// subscribeStockHandlers must never include the order-released event: stock was already
// deducted when the order was created.
func subscribeStockHandlers(sub pkgports.EventSubscriber, stock *service.StockEventHandler) error {
	handlers := []pkgports.EventHandler{
		eventbus.NewTypedEventHandler(dto.OrderCreatedEventName, stock.HandleOrderCreated),
		eventbus.NewTypedEventHandler(dto.OrderUpdatedEventName, stock.HandleOrderUpdated),
		eventbus.NewTypedEventHandler(dto.OrderDeletedEventName, stock.HandleOrderDeleted),
		eventbus.NewTypedEventHandler(dto.PurchaseEntryCreatedEventName, stock.HandlePurchaseEntryCreated),
	}
	return subscribeAll(sub, handlers)
}

func subscribeAll(sub pkgports.EventSubscriber, handlers []pkgports.EventHandler) error {
	for _, h := range handlers {
		if err := sub.Subscribe(h); err != nil {
			return fmt.Errorf("subscribe to %s: %w", h.EventName(), err)
		}
	}
	return nil
}
