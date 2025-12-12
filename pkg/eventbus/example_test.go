package eventbus_test

import (
	"context"
	"fmt"
	"time"

	"laguna-escondida/backend/pkg/domain/dto"
	"laguna-escondida/backend/pkg/domain/ports"
	"laguna-escondida/backend/pkg/eventbus"

	"github.com/ThreeDotsLabs/watermill"
)

// OrderCreatedEvent is an example domain event.
type OrderCreatedEvent struct {
	dto.EventMetadata
	OrderID     string  `json:"order_id"`
	CustomerID  string  `json:"customer_id"`
	TotalAmount float64 `json:"total_amount"`
}

func (e OrderCreatedEvent) EventName() string {
	return "order.created"
}

// Ensure OrderCreatedEvent implements ports.Event
var _ ports.Event = (*OrderCreatedEvent)(nil)

func Example() {
	ctx := context.Background()
	logger := watermill.NopLogger{}

	eventBus := eventbus.NewGoChannelEventBus(logger)
	defer eventBus.Close()

	subscriber, err := eventbus.NewGoChannelEventSubscriber(eventBus.PubSub(), logger)
	if err != nil {
		panic(err)
	}

	// Create a typed event handler
	handler := eventbus.NewTypedEventHandler(
		"order.created",
		func(ctx context.Context, event *OrderCreatedEvent) error {
			fmt.Printf("Received order created event: OrderID=%s, Amount=%.2f\n",
				event.OrderID, event.TotalAmount)
			return nil
		},
	)

	if err := subscriber.Subscribe(handler); err != nil {
		panic(err)
	}

	go func() {
		if err := subscriber.Start(ctx); err != nil {
			fmt.Printf("Subscriber error: %v\n", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)

	event := &OrderCreatedEvent{
		EventMetadata: dto.EventMetadata{
			EventID:     "evt-123",
			OccurredAt:  time.Now(),
			AggregateID: "order-456",
		},
		OrderID:     "order-456",
		CustomerID:  "customer-789",
		TotalAmount: 150.00,
	}

	if err := eventBus.Publish(ctx, event); err != nil {
		panic(err)
	}

	time.Sleep(100 * time.Millisecond)

	subscriber.Close()
}
