package bootstrap

import (
	"context"
	"fmt"
	"log/slog"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"
	pkgports "laguna-escondida/backend/pkg/domain/ports"
	"laguna-escondida/backend/pkg/eventbus"
)

// organizationEvent is any event that says which organization it was raised in.
type organizationEvent interface {
	pkgports.Event
	Organization() string
}

// inEventOrganization runs handle for the organization the event names, in a session of its
// own. An event without one is logged and dropped: handling it would act for no organization,
// and retrying can't add one.
func inEventOrganization[T organizationEvent](sessions ports.SessionOpener, logger *slog.Logger, handle func(ctx context.Context, event T) error) func(ctx context.Context, event T) error {
	return func(ctx context.Context, event T) error {
		orgID := event.Organization()
		if orgID == "" {
			logger.ErrorContext(ctx, "Event without an organization rejected", slog.String("event", event.EventName()))
			return nil
		}
		orgCtx, release := sessions.Open(tenant.WithOrg(ctx, orgID))
		defer release()
		return handle(orgCtx, event)
	}
}

// subscribeKitchenFeed wires the order events that drive the live kitchen board.
func subscribeKitchenFeed(sub pkgports.EventSubscriber, orders *service.OrderService, sessions ports.SessionOpener, logger *slog.Logger) error {
	handlers := []pkgports.EventHandler{
		eventbus.NewTypedEventHandler(dto.OrderCreatedEventName, inEventOrganization(sessions, logger, orders.HandleOrderCreatedSSE)),
		eventbus.NewTypedEventHandler(dto.OrderDeletedEventName, inEventOrganization(sessions, logger, orders.HandleOrderDeletedSSE)),
		eventbus.NewTypedEventHandler(dto.OrderUpdatedEventName, inEventOrganization(sessions, logger, orders.HandleOrderUpdatedSSE)),
		eventbus.NewTypedEventHandler(dto.OrderReleasedToKitchenEventName, inEventOrganization(sessions, logger, orders.HandleOrderReleasedToKitchenSSE)),
	}
	return subscribeAll(sub, handlers)
}

// subscribeStockHandlers must never include the order-released event: stock was already
// deducted when the order was created.
func subscribeStockHandlers(sub pkgports.EventSubscriber, stock *service.StockEventHandler, sessions ports.SessionOpener, logger *slog.Logger) error {
	handlers := []pkgports.EventHandler{
		eventbus.NewTypedEventHandler(dto.OrderCreatedEventName, inEventOrganization(sessions, logger, stock.HandleOrderCreated)),
		eventbus.NewTypedEventHandler(dto.OrderUpdatedEventName, inEventOrganization(sessions, logger, stock.HandleOrderUpdated)),
		eventbus.NewTypedEventHandler(dto.OrderDeletedEventName, inEventOrganization(sessions, logger, stock.HandleOrderDeleted)),
		eventbus.NewTypedEventHandler(dto.PurchaseEntryCreatedEventName, inEventOrganization(sessions, logger, stock.HandlePurchaseEntryCreated)),
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
