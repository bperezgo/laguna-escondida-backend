package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type deliveryFixture struct {
	store     *mocks.MockMarketingEventStore
	transport *mocks.MockMarketingEventTransport
	service   *MarketingEventDeliveryService
	now       time.Time
}

func newDeliveryFixture(t *testing.T, secrets staticSecrets) *deliveryFixture {
	now := crmAt(t, 8, 20, 0)
	f := &deliveryFixture{store: mocks.NewMockMarketingEventStore(t), transport: mocks.NewMockMarketingEventTransport(t), now: now}
	f.service = NewMarketingEventDeliveryService(f.store, f.transport, crmOrganizations(t), secrets, discardLogger(), fixedClock(now))
	return f
}

func dueEvent(attempts int, createdAt time.Time) *dto.MarketingEvent {
	return &dto.MarketingEvent{
		EventID: "e1", Type: dto.MarketingEventOrderPaid, Path: dto.MarketingEventsPathOrders,
		Body: []byte(`{"event_id":"e1"}`), Status: dto.MarketingEventPending, Attempts: attempts, CreatedAt: createdAt,
	}
}

func (f *deliveryFixture) claims(events ...*dto.MarketingEvent) {
	f.store.EXPECT().ClaimDue(mock.Anything, f.now, f.now.Add(2*time.Minute), 50).Return(events, nil).Once()
}

func TestMarketingEventDeliveryService_ReceiverAnswers(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		sendErr    error
		attempts   int
		wantSent   bool
		wantStatus dto.MarketingEventStatus
		wantDelay  time.Duration
	}{
		{name: "200 delivered", status: 200, attempts: 1, wantSent: true},
		{name: "204 delivered", status: 204, attempts: 1, wantSent: true},
		{name: "400 rejected for good", status: 400, attempts: 1, wantStatus: dto.MarketingEventRejected},
		{name: "401 retried hourly", status: 401, attempts: 1, wantStatus: dto.MarketingEventPending, wantDelay: time.Hour},
		{name: "404 retried like a 5xx", status: 404, attempts: 1, wantStatus: dto.MarketingEventPending, wantDelay: 10 * time.Second},
		{name: "503 first retry after 10s", status: 503, attempts: 1, wantStatus: dto.MarketingEventPending, wantDelay: 10 * time.Second},
		{name: "503 third attempt waits 40s", status: 503, attempts: 3, wantStatus: dto.MarketingEventPending, wantDelay: 40 * time.Second},
		{name: "500 retries cap at 30 minutes", status: 500, attempts: 20, wantStatus: dto.MarketingEventPending, wantDelay: 30 * time.Minute},
		{name: "a timeout is retried", sendErr: errors.New("context deadline exceeded"), attempts: 2, wantStatus: dto.MarketingEventPending, wantDelay: 20 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newDeliveryFixture(t, staticSecrets{crmOrgSlug: crmSecret})
			event := dueEvent(tt.attempts, f.now.Add(-time.Hour))
			f.claims(event)
			f.transport.EXPECT().Send(mock.Anything, "/backend/events", event.Body, mock.Anything).Return(tt.status, tt.sendErr).Once()
			if tt.wantSent {
				f.store.EXPECT().MarkSent(mock.Anything, "e1", f.now).Return(nil).Once()
			} else {
				f.store.EXPECT().MarkFailed(mock.Anything, "e1", tt.wantStatus, f.now.Add(tt.wantDelay), mock.Anything).Return(nil).Once()
			}

			delivered, err := f.service.DeliverDue(crmContext())

			require.NoError(t, err)
			assert.Equal(t, tt.wantSent, delivered == 1)
		})
	}
}

func TestMarketingEventDeliveryService_SignsTheExactBytes(t *testing.T) {
	f := newDeliveryFixture(t, staticSecrets{crmOrgSlug: crmSecret})
	event := dueEvent(1, f.now)
	f.claims(event)
	var signature string
	f.transport.EXPECT().Send(mock.Anything, mock.Anything, event.Body, mock.Anything).
		Run(func(_ context.Context, _ string, _ []byte, s string) { signature = s }).Return(200, nil).Once()
	f.store.EXPECT().MarkSent(mock.Anything, "e1", mock.Anything).Return(nil).Once()

	_, err := f.service.DeliverDue(crmContext())

	require.NoError(t, err)
	mac := hmac.New(sha256.New, []byte(crmSecret))
	mac.Write(event.Body)
	assert.Equal(t, "sha256="+hex.EncodeToString(mac.Sum(nil)), signature)
}

func TestMarketingEventDeliveryService_ExpiresAfter72Hours(t *testing.T) {
	f := newDeliveryFixture(t, staticSecrets{crmOrgSlug: crmSecret})
	f.claims(dueEvent(40, f.now.Add(-73*time.Hour)))
	f.store.EXPECT().MarkFailed(mock.Anything, "e1", dto.MarketingEventExpired, f.now, mock.Anything).Return(nil).Once()

	delivered, err := f.service.DeliverDue(crmContext())

	require.NoError(t, err)
	assert.Zero(t, delivered)
	f.transport.AssertNotCalled(t, "Send", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestMarketingEventDeliveryService_NoSecretSendsNothing(t *testing.T) {
	f := newDeliveryFixture(t, staticSecrets{})

	delivered, err := f.service.DeliverDue(crmContext())

	require.NoError(t, err)
	assert.Zero(t, delivered)
	f.store.AssertNotCalled(t, "ClaimDue", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestMarketingEventDeliveryService_Failures(t *testing.T) {
	t.Run("claim", func(t *testing.T) {
		f := newDeliveryFixture(t, staticSecrets{crmOrgSlug: crmSecret})
		f.store.EXPECT().ClaimDue(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("db down")).Once()
		_, err := f.service.DeliverDue(crmContext())
		require.ErrorContains(t, err, "db down")
	})
	t.Run("settle", func(t *testing.T) {
		f := newDeliveryFixture(t, staticSecrets{crmOrgSlug: crmSecret})
		f.claims(dueEvent(1, f.now), dueEvent(1, f.now))
		f.transport.EXPECT().Send(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(200, nil).Once()
		f.store.EXPECT().MarkSent(mock.Anything, mock.Anything, mock.Anything).Return(errors.New("db down")).Once()
		_, err := f.service.DeliverDue(crmContext())
		require.ErrorContains(t, err, "db down")
	})
	t.Run("organization", func(t *testing.T) {
		orgs := mocks.NewMockOrganizationRepository(t)
		orgs.EXPECT().Get(mock.Anything, crmOrgID).Return(nil, errors.New("db down")).Once()
		svc := NewMarketingEventDeliveryService(mocks.NewMockMarketingEventStore(t), mocks.NewMockMarketingEventTransport(t), orgs, staticSecrets{}, discardLogger(), nil)
		_, err := svc.DeliverDue(crmContext())
		require.ErrorContains(t, err, "db down")
	})
	t.Run("no organization", func(t *testing.T) {
		svc := NewMarketingEventDeliveryService(mocks.NewMockMarketingEventStore(t), mocks.NewMockMarketingEventTransport(t), mocks.NewMockOrganizationRepository(t), staticSecrets{}, discardLogger(), nil)
		_, err := svc.DeliverDue(t.Context())
		require.Error(t, err)
	})
}
