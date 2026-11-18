package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/eventconsumer"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type relayFixture struct {
	store   *mocks.MockEventDeliveryStore
	service *EventRelayService
	now     time.Time
	ctx     context.Context
}

func newRelayFixture(t *testing.T, consumers ...*mocks.MockEventConsumer) *relayFixture {
	registered := make([]ports.EventConsumer, len(consumers))
	for i, c := range consumers {
		registered[i] = c
	}
	registry, err := eventconsumer.NewRegistry(registered...)
	require.NoError(t, err)
	now := crmAt(t, 8, 20, 0)
	f := &relayFixture{store: mocks.NewMockEventDeliveryStore(t), now: now, ctx: tenant.WithOrg(context.Background(), "org-1")}
	f.service = NewEventRelayService(f.store, registry, createMockUnitOfWork(t), discardLogger(), fixedClock(now))
	return f
}

func relayConsumer(t *testing.T, name string) *mocks.MockEventConsumer {
	c := mocks.NewMockEventConsumer(t)
	c.EXPECT().Name().Return(name).Maybe()
	c.EXPECT().Handles().Return([]string{dto.DomainEventOrderPaid}).Maybe()
	return c
}

func claimedDelivery(consumer string, attempts int) *dto.ClaimedDelivery {
	return &dto.ClaimedDelivery{
		Event:    dto.StoredEvent{ID: "e1", Name: dto.DomainEventOrderPaid, OrganizationID: "org-1"},
		Consumer: consumer, Attempts: attempts,
	}
}

func (f *relayFixture) claims(deliveries ...*dto.ClaimedDelivery) {
	f.store.EXPECT().ClaimDue(mock.Anything, f.now, f.now.Add(time.Minute), 100).Return(deliveries, nil).Once()
}

func TestEventRelayService_DispatchDue_OneConsumerFailsAnotherIsDone(t *testing.T) {
	good, bad := relayConsumer(t, "good"), relayConsumer(t, "bad")
	f := newRelayFixture(t, good, bad)
	f.claims(claimedDelivery("good", 1), claimedDelivery("bad", 1))
	good.EXPECT().Handle(mock.Anything, mock.Anything).Return(nil).Once()
	bad.EXPECT().Handle(mock.Anything, mock.Anything).Return(errors.New("boom")).Once()
	f.store.EXPECT().MarkDone(mock.Anything, "e1", "good", f.now).Return(nil).Once()
	f.store.EXPECT().MarkFailed(mock.Anything, "e1", "bad", dto.EventDeliveryPending, f.now.Add(time.Second), "boom").Return(nil).Once()

	done, err := f.service.DispatchDue(f.ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, done)
}

func TestEventRelayService_DispatchDue_BackoffCurve(t *testing.T) {
	tests := []struct {
		attempts int
		want     time.Duration
	}{
		{1, time.Second}, {2, 2 * time.Second}, {3, 4 * time.Second}, {4, 8 * time.Second},
		{8, 128 * time.Second}, {9, 256 * time.Second}, {10, 5 * time.Minute}, {19, 5 * time.Minute},
	}
	for _, tt := range tests {
		t.Run(tt.want.String(), func(t *testing.T) {
			c := relayConsumer(t, "c")
			f := newRelayFixture(t, c)
			f.claims(claimedDelivery("c", tt.attempts))
			c.EXPECT().Handle(mock.Anything, mock.Anything).Return(errors.New("boom")).Once()
			f.store.EXPECT().MarkFailed(mock.Anything, "e1", "c", dto.EventDeliveryPending, f.now.Add(tt.want), "boom").Return(nil).Once()

			_, err := f.service.DispatchDue(f.ctx)

			require.NoError(t, err)
		})
	}
}

func TestEventRelayService_DispatchDue_DeadAfterTwentyAttempts(t *testing.T) {
	c := relayConsumer(t, "c")
	f := newRelayFixture(t, c)
	f.claims(claimedDelivery("c", 20))
	c.EXPECT().Handle(mock.Anything, mock.Anything).Return(errors.New("boom")).Once()
	f.store.EXPECT().MarkFailed(mock.Anything, "e1", "c", dto.EventDeliveryDead, f.now, "boom").Return(nil).Once()

	done, err := f.service.DispatchDue(f.ctx)

	require.NoError(t, err)
	assert.Zero(t, done)
}

func TestEventRelayService_DispatchDue_UnknownConsumerIsDead(t *testing.T) {
	f := newRelayFixture(t)
	f.claims(claimedDelivery("gone", 1))
	f.store.EXPECT().MarkFailed(mock.Anything, "e1", "gone", dto.EventDeliveryDead, f.now, "unknown consumer").Return(nil).Once()

	done, err := f.service.DispatchDue(f.ctx)

	require.NoError(t, err)
	assert.Zero(t, done)
}

func TestEventRelayService_DispatchDue_HandlesInsideAUnitOfWorkForTheEventsOrganization(t *testing.T) {
	c := relayConsumer(t, "c")
	f := newRelayFixture(t, c)
	f.claims(claimedDelivery("c", 1))
	c.EXPECT().Handle(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, e dto.StoredEvent) error {
		org, ok := tenant.OrgFrom(ctx)
		require.True(t, ok)
		assert.Equal(t, "org-1", org)
		return nil
	}).Once()
	f.store.EXPECT().MarkDone(mock.Anything, "e1", "c", f.now).Return(nil).Once()

	_, err := f.service.DispatchDue(f.ctx)

	require.NoError(t, err)
}

func TestEventRelayService_DispatchDue_NoOrganization_Errors(t *testing.T) {
	f := newRelayFixture(t)

	_, err := f.service.DispatchDue(context.Background())

	require.Error(t, err)
}

func TestEventRelayService_DispatchDue_ClaimError(t *testing.T) {
	f := newRelayFixture(t)
	f.store.EXPECT().ClaimDue(mock.Anything, mock.Anything, mock.Anything, 100).Return(nil, errors.New("db down")).Once()

	_, err := f.service.DispatchDue(f.ctx)

	require.ErrorContains(t, err, "db down")
}

func TestEventRelayService_DispatchDue_SettleError(t *testing.T) {
	c := relayConsumer(t, "c")
	f := newRelayFixture(t, c)
	f.claims(claimedDelivery("c", 1))
	c.EXPECT().Handle(mock.Anything, mock.Anything).Return(nil).Once()
	f.store.EXPECT().MarkDone(mock.Anything, "e1", "c", f.now).Return(errors.New("db down")).Once()

	_, err := f.service.DispatchDue(f.ctx)

	require.ErrorContains(t, err, "db down")
}
