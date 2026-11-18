package httpclient

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testMarketingOrg    = "019a0000-0000-7000-8000-0000000000c1"
	testMarketingSlug   = "laguna-escondida"
	testMarketingSecret = "s3cret"
)

type marketingSecrets struct{}

func (marketingSecrets) SecretFor(slug string) (string, bool) {
	return testMarketingSecret, slug == testMarketingSlug
}

// memoryStore is the outbox as the sender sees it: one pending event, due whenever its next attempt has come.
type memoryStore struct {
	mu    sync.Mutex
	event dto.MarketingEvent
}

func (s *memoryStore) ClaimDue(_ context.Context, now, leaseUntil time.Time, _ int) ([]*dto.MarketingEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.event.Status != dto.MarketingEventPending || s.event.NextAttemptAt.After(now) {
		return nil, nil
	}
	s.event.Attempts++
	s.event.NextAttemptAt = leaseUntil
	claimed := s.event
	return []*dto.MarketingEvent{&claimed}, nil
}

func (s *memoryStore) MarkSent(_ context.Context, _ string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.event.Status, s.event.SentAt = dto.MarketingEventSent, &at
	return nil
}

func (s *memoryStore) MarkFailed(_ context.Context, _ string, status dto.MarketingEventStatus, next time.Time, lastError string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.event.Status, s.event.NextAttemptAt, s.event.LastError = status, next, &lastError
	return nil
}

func (s *memoryStore) snapshot() dto.MarketingEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.event
}

type received struct {
	path      string
	body      []byte
	signature string
}

// receiver answers with statuses in turn (the last one repeats) and records every request.
func receiver(t *testing.T, delay time.Duration, statuses ...int) (*httptest.Server, func() []received) {
	var mu sync.Mutex
	var got []received
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		got = append(got, received{path: r.URL.Path, body: body, signature: r.Header.Get("X-Signature-256")})
		status := statuses[min(len(got), len(statuses))-1]
		mu.Unlock()
		if delay > 0 {
			time.Sleep(delay)
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(server.Close)
	return server, func() []received {
		mu.Lock()
		defer mu.Unlock()
		return append([]received(nil), got...)
	}
}

type deliveryRig struct {
	store   *memoryStore
	service *service.MarketingEventDeliveryService
	now     *time.Time
}

func newDeliveryRig(t *testing.T, baseURL string) *deliveryRig {
	start := time.Date(2026, 11, 8, 20, 0, 0, 0, time.UTC)
	rig := &deliveryRig{
		store: &memoryStore{event: dto.MarketingEvent{
			EventID: "019a0000-0000-7000-8000-0000000000e1", Type: dto.MarketingEventOrderPaid, Path: dto.MarketingEventsPathOrders,
			Body:   []byte(`{"event_id":"019a0000-0000-7000-8000-0000000000e1","type":"order.paid","amount":"45000"}`),
			Status: dto.MarketingEventPending, NextAttemptAt: start, CreatedAt: start,
		}},
		now: &start,
	}
	orgs := mocks.NewMockOrganizationRepository(t)
	orgs.EXPECT().Get(mock.Anything, testMarketingOrg).Return(&dto.Organization{ID: testMarketingOrg, Slug: testMarketingSlug}, nil).Maybe()
	transport := NewMarketingEventsClient(NewClient(slog.New(slog.DiscardHandler)), baseURL+"/")
	rig.service = service.NewMarketingEventDeliveryService(rig.store, transport, orgs, marketingSecrets{}, slog.New(slog.DiscardHandler),
		func() time.Time { return *rig.now })
	return rig
}

// run delivers whatever is due, then moves the clock to the next attempt.
func (r *deliveryRig) run(t *testing.T, ctx context.Context) {
	t.Helper()
	_, err := r.service.DeliverDue(tenant.WithOrg(ctx, testMarketingOrg))
	require.NoError(t, err)
	if next := r.store.snapshot().NextAttemptAt; next.After(*r.now) {
		*r.now = next
	}
}

func TestMarketingEventsDelivery_Answers(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		wantStatus dto.MarketingEventStatus
		wantDelay  time.Duration
	}{
		{name: "200", status: http.StatusOK, wantStatus: dto.MarketingEventSent},
		{name: "202", status: http.StatusAccepted, wantStatus: dto.MarketingEventSent},
		{name: "400", status: http.StatusBadRequest, wantStatus: dto.MarketingEventRejected},
		{name: "401", status: http.StatusUnauthorized, wantStatus: dto.MarketingEventPending, wantDelay: time.Hour},
		{name: "404 before marketing-app ships the path", status: http.StatusNotFound, wantStatus: dto.MarketingEventPending, wantDelay: 10 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, requests := receiver(t, 0, tt.status)
			rig := newDeliveryRig(t, server.URL)
			start := *rig.now

			_, err := rig.service.DeliverDue(tenant.WithOrg(t.Context(), testMarketingOrg))

			require.NoError(t, err)
			got := rig.store.snapshot()
			assert.Equal(t, tt.wantStatus, got.Status)
			if tt.wantDelay > 0 {
				assert.Equal(t, start.Add(tt.wantDelay), got.NextAttemptAt)
			}
			require.Len(t, requests(), 1)
			assert.Equal(t, "/backend/events", requests()[0].path)
		})
	}
}

func TestMarketingEventsDelivery_TheReceiverIsDown(t *testing.T) {
	server, requests := receiver(t, 0, 503, 503, 503, 200)
	rig := newDeliveryRig(t, server.URL)

	for range 6 {
		rig.run(t, t.Context())
	}

	got := requests()
	require.Len(t, got, 4, "delivered on the fourth attempt and not sent again")
	for _, r := range got {
		assert.Equal(t, got[0].body, r.body, "the same bytes every time")
		assert.Equal(t, got[0].signature, r.signature)
	}
	assert.Equal(t, dto.MarketingEventSent, rig.store.snapshot().Status)
	assert.Equal(t, 4, rig.store.snapshot().Attempts)
}

func TestMarketingEventsDelivery_ATimeoutIsRetried(t *testing.T) {
	server, _ := receiver(t, 500*time.Millisecond, 200)
	rig := newDeliveryRig(t, server.URL)
	start := *rig.now
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	_, err := rig.service.DeliverDue(tenant.WithOrg(ctx, testMarketingOrg))

	require.NoError(t, err)
	got := rig.store.snapshot()
	assert.Equal(t, dto.MarketingEventPending, got.Status)
	assert.Equal(t, start.Add(10*time.Second), got.NextAttemptAt)
	require.NotNil(t, got.LastError)
	assert.Contains(t, *got.LastError, "send marketing event")
}

func TestMarketingEventsDelivery_TheSignatureMatchesTheBytes(t *testing.T) {
	server, requests := receiver(t, 0, 200)
	rig := newDeliveryRig(t, server.URL)

	rig.run(t, t.Context())

	require.Len(t, requests(), 1)
	r := requests()[0]
	mac := hmac.New(sha256.New, []byte(testMarketingSecret))
	mac.Write(r.body)
	assert.Equal(t, "sha256="+hex.EncodeToString(mac.Sum(nil)), r.signature)
	assert.Equal(t, rig.store.snapshot().Body, r.body)
}
