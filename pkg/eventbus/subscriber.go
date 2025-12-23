package eventbus

import (
	"context"
	"fmt"

	"laguna-escondida/backend/pkg/domain/ports"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/router/middleware"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
)

// GoChannelEventSubscriber implements ports.EventSubscriber using Watermill's router.
type GoChannelEventSubscriber struct {
	router   *message.Router
	pubSub   *gochannel.GoChannel
	logger   watermill.LoggerAdapter
	handlers []ports.EventHandler
}

// NewGoChannelEventSubscriber creates a new event subscriber using the provided GoChannel PubSub.
func NewGoChannelEventSubscriber(pubSub *gochannel.GoChannel, logger watermill.LoggerAdapter) (*GoChannelEventSubscriber, error) {
	if logger == nil {
		logger = watermill.NopLogger{}
	}

	router, err := message.NewRouter(message.RouterConfig{}, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create router: %w", err)
	}

	router.AddMiddleware(middleware.Recoverer)

	return &GoChannelEventSubscriber{
		router:   router,
		pubSub:   pubSub,
		logger:   logger,
		handlers: make([]ports.EventHandler, 0),
	}, nil
}

// Subscribe registers an event handler for a specific event type.
func (s *GoChannelEventSubscriber) Subscribe(handler ports.EventHandler) error {
	topic := handler.EventName()

	s.router.AddConsumerHandler(
		fmt.Sprintf("handler-%s", topic),
		topic,
		s.pubSub,
		func(msg *message.Message) error {
			if err := handler.Handle(msg.Context(), msg.Payload); err != nil {
				s.logger.Error("Failed to handle event", err, watermill.LogFields{
					"topic":      topic,
					"message_id": msg.UUID,
				})
				return err
			}
			msg.Ack()
			return nil
		},
	)

	s.handlers = append(s.handlers, handler)
	return nil
}

// Start starts the event subscriber and begins processing events.
func (s *GoChannelEventSubscriber) Start(ctx context.Context) error {
	return s.router.Run(ctx)
}

// Close stops the event subscriber.
func (s *GoChannelEventSubscriber) Close() error {
	return s.router.Close()
}

var _ ports.EventSubscriber = (*GoChannelEventSubscriber)(nil)
