package eventbus

import (
	"context"
	"fmt"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/eventconsumer"
	"laguna-escondida/backend/internal/domain/ports"
	pkgports "laguna-escondida/backend/pkg/domain/ports"
)

const BridgeConsumerName = "in-memory-bridge"

// InMemoryBridgeConsumer republishes durable events on the in-memory bus, so its typed handlers
// (SSE among them) can consume them unchanged.
type InMemoryBridgeConsumer struct {
	bus    pkgports.EventBus
	events []string
}

var _ ports.EventConsumer = (*InMemoryBridgeConsumer)(nil)

// NewInMemoryBridgeConsumer refuses order.created: it is already published in memory after commit, so forwarding it would deliver it twice.
func NewInMemoryBridgeConsumer(bus pkgports.EventBus, events []string) (*InMemoryBridgeConsumer, error) {
	for _, name := range events {
		if name == dto.DomainEventOrderCreated {
			return nil, fmt.Errorf("bridge must not forward %s: it is already published in memory", name)
		}
		if _, ok := bridgeDecoders[name]; !ok {
			return nil, fmt.Errorf("bridge cannot forward unknown event %q", name)
		}
	}
	return &InMemoryBridgeConsumer{bus: bus, events: append([]string(nil), events...)}, nil
}

func (c *InMemoryBridgeConsumer) Name() string { return BridgeConsumerName }

func (c *InMemoryBridgeConsumer) Handles() []string { return append([]string(nil), c.events...) }

func (c *InMemoryBridgeConsumer) Handle(ctx context.Context, event dto.StoredEvent) error {
	decode, ok := bridgeDecoders[event.Name]
	if !ok {
		return fmt.Errorf("bridge cannot forward unknown event %q", event.Name)
	}
	forwarded, err := decode(event)
	if err != nil {
		return err
	}
	return c.bus.Publish(ctx, forwarded)
}

var bridgeDecoders = map[string]func(dto.StoredEvent) (pkgports.Event, error){
	dto.DomainEventOrderPaid:            decodeForBridge[dto.OrderPaid],
	dto.DomainEventReservationRequested: decodeForBridge[dto.ReservationRequested],
	dto.DomainEventReservationConfirmed: decodeForBridge[dto.ReservationConfirmed],
}

func decodeForBridge[T interface {
	ports.DomainEvent
	pkgports.Event
}](stored dto.StoredEvent) (pkgports.Event, error) {
	event, err := eventconsumer.Decode[T](stored)
	if err != nil {
		return nil, err
	}
	return event, nil
}
