package eventbus

import (
	"context"
	"encoding/json"
	"fmt"

	"laguna-escondida/backend/pkg/domain/ports"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
	"github.com/google/uuid"
)

// GoChannelEventBus implements ports.EventBus using Watermill's gochannel for in-memory pub/sub.
type GoChannelEventBus struct {
	pubSub *gochannel.GoChannel
	logger watermill.LoggerAdapter
}

// NewGoChannelEventBus creates a new in-memory event bus using Go channels.
func NewGoChannelEventBus(logger watermill.LoggerAdapter) *GoChannelEventBus {
	if logger == nil {
		logger = watermill.NopLogger{}
	}

	pubSub := gochannel.NewGoChannel(
		gochannel.Config{
			OutputChannelBuffer: 100,
			Persistent:          false,
		},
		logger,
	)

	return &GoChannelEventBus{
		pubSub: pubSub,
		logger: logger,
	}
}

// Publish publishes an event to the event bus.
func (b *GoChannelEventBus) Publish(ctx context.Context, event ports.Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	msg := message.NewMessage(uuid.NewString(), payload)
	msg.SetContext(ctx)

	topic := event.EventName()

	if err := b.pubSub.Publish(topic, msg); err != nil {
		return fmt.Errorf("failed to publish event to topic %s: %w", topic, err)
	}

	return nil
}

// PubSub returns the underlying gochannel.GoChannel for subscriber access.
func (b *GoChannelEventBus) PubSub() *gochannel.GoChannel {
	return b.pubSub
}

// Close closes the event bus and releases resources.
func (b *GoChannelEventBus) Close() error {
	return b.pubSub.Close()
}

var _ ports.EventBus = (*GoChannelEventBus)(nil)
