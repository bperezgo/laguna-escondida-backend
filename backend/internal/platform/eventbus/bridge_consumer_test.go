package eventbus

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/pkg/eventbus"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemoryBridgeConsumer_ForwardsToATypedHandler(t *testing.T) {
	logger := watermill.NopLogger{}
	bus := eventbus.NewGoChannelEventBus(logger)
	t.Cleanup(func() { _ = bus.Close() })
	subscriber, err := eventbus.NewGoChannelEventSubscriber(bus.PubSub(), logger)
	require.NoError(t, err)
	t.Cleanup(func() { _ = subscriber.Close() })

	received := make(chan dto.OrderPaid, 1)
	require.NoError(t, subscriber.Subscribe(eventbus.NewTypedEventHandler(dto.DomainEventOrderPaid,
		func(_ context.Context, e dto.OrderPaid) error {
			received <- e
			return nil
		})))
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = subscriber.Start(ctx) }()
	<-time.After(200 * time.Millisecond)

	bridge, err := NewInMemoryBridgeConsumer(bus, []string{dto.DomainEventOrderPaid})
	require.NoError(t, err)
	assert.Equal(t, []string{dto.DomainEventOrderPaid}, bridge.Handles())

	sent := dto.OrderPaid{
		DomainEventMeta: dto.DomainEventMeta{ID: "e1", Name: dto.DomainEventOrderPaid, OrganizationID: "org-1", Subject: "o1", Time: time.Now().UTC()},
		OrderType:       dto.OrderTypeDelivery, Total: decimal.NewFromInt(42000), AmountPaid: decimal.NewFromInt(42000),
	}
	payload, err := json.Marshal(sent)
	require.NoError(t, err)

	require.NoError(t, bridge.Handle(context.Background(), dto.StoredEvent{ID: "e1", Name: dto.DomainEventOrderPaid, OrganizationID: "org-1", Payload: payload}))

	select {
	case got := <-received:
		assert.Equal(t, "e1", got.ID)
		assert.Equal(t, "o1", got.SubjectID())
		assert.True(t, sent.Total.Equal(got.Total))
	case <-time.After(3 * time.Second):
		t.Fatal("forwarded event never reached the in-memory handler")
	}
}

func TestInMemoryBridgeConsumer_RefusesOrderCreated(t *testing.T) {
	_, err := NewInMemoryBridgeConsumer(eventbus.NewGoChannelEventBus(nil), []string{dto.DomainEventOrderCreated})

	require.ErrorContains(t, err, "order.created")
}

func TestInMemoryBridgeConsumer_UnknownEvent(t *testing.T) {
	bus := eventbus.NewGoChannelEventBus(nil)
	_, err := NewInMemoryBridgeConsumer(bus, []string{"nope"})
	require.Error(t, err)

	empty, err := NewInMemoryBridgeConsumer(bus, nil)
	require.NoError(t, err)
	assert.Empty(t, empty.Handles())
	require.Error(t, empty.Handle(context.Background(), dto.StoredEvent{Name: "nope"}))
}

func TestInMemoryBridgeConsumer_BadPayload(t *testing.T) {
	bridge, err := NewInMemoryBridgeConsumer(eventbus.NewGoChannelEventBus(nil), []string{dto.DomainEventReservationRequested})
	require.NoError(t, err)

	err = bridge.Handle(context.Background(), dto.StoredEvent{ID: "e1", Name: dto.DomainEventReservationRequested, Payload: []byte("{")})

	require.Error(t, err)
}
