package eventbus

import (
	"context"
	"log"
	"sync"

	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/shared/domain/event"
	pkgports "laguna-escondida/backend/pkg/domain/ports"
)

// SSEEventBus routes domain events to registered notifiers based on EventName.
type SSEEventBus struct {
	mu      sync.RWMutex
	routing map[string][]ports.Notifier
}

func NewSSEEventBus() *SSEEventBus {
	return &SSEEventBus{
		routing: make(map[string][]ports.Notifier),
	}
}

// RegisterRoute adds a notifier for a specific event type.
func (b *SSEEventBus) RegisterRoute(eventName string, notifier ports.Notifier) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.routing[eventName] = append(b.routing[eventName], notifier)
}

// Publish routes the event to all registered notifiers for its EventName.
func (b *SSEEventBus) Publish(ctx context.Context, e pkgports.Event) error {
	domainEvent, ok := e.(event.DomainEvent)
	if !ok {
		log.Printf("SSEEventBus: event does not implement DomainEvent interface: %T", e)
		return nil
	}

	b.mu.RLock()
	notifiers := b.routing[domainEvent.EventName()]
	b.mu.RUnlock()

	for _, n := range notifiers {
		if err := n.Notify(ctx, domainEvent.EventName(), domainEvent.Data()); err != nil {
			log.Printf("SSEEventBus: failed to notify for event %s: %v", domainEvent.EventName(), err)
		}
	}

	return nil
}

var _ pkgports.EventBus = (*SSEEventBus)(nil)
