package main

import (
	"context"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/service"
	pkgports "laguna-escondida/backend/pkg/domain/ports"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingSubscriber struct {
	names []string
}

func (r *recordingSubscriber) Subscribe(h pkgports.EventHandler) error {
	r.names = append(r.names, h.EventName())
	return nil
}

func (r *recordingSubscriber) Start(context.Context) error { return nil }

func (r *recordingSubscriber) Close() error { return nil }

func TestSubscribeStockHandlers_IgnoresOrderRelease(t *testing.T) {
	sub := &recordingSubscriber{}

	require.NoError(t, subscribeStockHandlers(sub, &service.StockEventHandler{}))

	assert.NotContains(t, sub.names, dto.OrderReleasedToKitchenEventName, "a release must not deduct stock again")
	assert.Contains(t, sub.names, dto.OrderCreatedEventName, "stock is still reserved at creation")
}

func TestSubscribeKitchenFeed_IncludesOrderRelease(t *testing.T) {
	sub := &recordingSubscriber{}

	require.NoError(t, subscribeKitchenFeed(sub, &service.OrderService{}))

	assert.ElementsMatch(t, []string{
		dto.OrderCreatedEventName, dto.OrderUpdatedEventName, dto.OrderDeletedEventName, dto.OrderReleasedToKitchenEventName,
	}, sub.names)
}
