package eventbus

import (
	"context"

	"laguna-escondida/backend/pkg/domain/ports"
)

// DetachedEventBus publishes with the request's DB connection stripped from ctx, since
// subscribers run on other goroutines, often after the request has finished.
type DetachedEventBus struct {
	inner  ports.EventBus
	detach func(context.Context) context.Context
}

func NewDetachedEventBus(inner ports.EventBus, detach func(context.Context) context.Context) *DetachedEventBus {
	return &DetachedEventBus{inner: inner, detach: detach}
}

func (b *DetachedEventBus) Publish(ctx context.Context, e ports.Event) error {
	return b.inner.Publish(b.detach(ctx), e)
}

var _ ports.EventBus = (*DetachedEventBus)(nil)
