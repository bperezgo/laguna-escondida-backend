package bootstrap

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"
	pkgports "laguna-escondida/backend/pkg/domain/ports"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

	require.NoError(t, subscribeStockHandlers(sub, &service.StockEventHandler{}, nil, slog.Default()))

	assert.NotContains(t, sub.names, dto.OrderReleasedToKitchenEventName, "a release must not deduct stock again")
	assert.Contains(t, sub.names, dto.OrderCreatedEventName, "stock is still reserved at creation")
}

func TestSubscribeKitchenFeed_IncludesOrderRelease(t *testing.T) {
	sub := &recordingSubscriber{}

	require.NoError(t, subscribeKitchenFeed(sub, &service.OrderService{}, nil, slog.Default()))

	assert.ElementsMatch(t, []string{
		dto.OrderCreatedEventName, dto.OrderUpdatedEventName, dto.OrderDeletedEventName, dto.OrderReleasedToKitchenEventName,
	}, sub.names)
}

func TestInEventOrganization_HandlesTheEventForItsOrganizationInItsOwnSession(t *testing.T) {
	sessions := mocks.NewMockSessionOpener(t)
	released := false
	sessions.On("Open", mock.Anything).Return(
		func(ctx context.Context) context.Context { return ctx },
		func(context.Context) func() { return func() { released = true } },
	).Once()
	var handledFor string
	handle := inEventOrganization(sessions, slog.Default(), func(ctx context.Context, _ dto.PurchaseEntryCreatedEvent) error {
		handledFor, _ = tenant.OrgFrom(ctx)
		return nil
	})
	event := dto.NewPurchaseEntryCreatedEvent("pe-1", "sup-1", nil)
	event.OrganizationID = testOrgID

	require.NoError(t, handle(tenant.WithOrg(context.Background(), "some-other-org"), event))

	assert.Equal(t, testOrgID, handledFor, "the event's organization, not whatever the context carried")
	assert.True(t, released)
}

func TestInEventOrganization_EventWithoutOrganizationIsRejectedAndLogged(t *testing.T) {
	logs := &bytes.Buffer{}
	handled := false
	handle := inEventOrganization(mocks.NewMockSessionOpener(t), slog.New(slog.NewTextHandler(logs, nil)),
		func(context.Context, dto.OrderDeletedEvent) error {
			handled = true
			return nil
		})

	require.NoError(t, handle(context.Background(), dto.NewOrderDeletedEvent("bill-1", nil)), "acked so it isn't redelivered")

	assert.False(t, handled)
	assert.Contains(t, logs.String(), "Event without an organization rejected")
	assert.Contains(t, logs.String(), dto.OrderDeletedEventName)
}
