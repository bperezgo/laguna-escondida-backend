package reservations

import (
	"context"
	"net/http"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/postgres"
	"laguna-escondida/backend/internal/platform/postgres/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReservationRepository_TheJobQueries(t *testing.T) {
	c := newCloud(t)
	r := c.newRestaurant()
	ctx := tenant.WithOrg(context.Background(), r.orgID)
	repo := repository.NewReservationRepository(cloudDB)
	whatsapp := c.agentReservation(r, "573005551234", "Ana", at(localDay(t, 1), 20, 0))
	byPhone := c.staffReservation(r, "3005559876", "Luis", at(localDay(t, 1), 20, 0))
	confirmed := c.withKey(r.managerKey, http.MethodPost, "/api/reservations/"+whatsapp.ID+"/confirm", nil)
	require.Equal(t, http.StatusOK, confirmed.status)
	startsAt := whatsapp.StartsAt

	due, err := repo.ListConfirmedStartedBefore(ctx, startsAt.Add(time.Minute))
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{whatsapp.ID, byPhone.ID}, []string{due[0].ID, due[1].ID})
	none, err := repo.ListConfirmedStartedBefore(ctx, startsAt)
	require.NoError(t, err)
	assert.Empty(t, none, "not yet past its start")

	visitedAt := time.Now()
	for _, id := range []string{whatsapp.ID, byPhone.ID} {
		current, findErr := repo.FindByID(ctx, id)
		require.NoError(t, findErr)
		current.Status, current.VisitedAt = dto.ReservationStatusVisited, &visitedAt
		require.NoError(t, repo.Update(ctx, current, dto.ReservationStatusConfirmed))
		assert.ErrorIs(t, repo.Update(ctx, current, dto.ReservationStatusConfirmed), domainError.ErrReservationTransition, "a stale status changes nothing")
	}

	awaiting, err := repo.ListAwaitingFeedback(ctx, visitedAt.Add(-time.Hour))
	require.NoError(t, err)
	require.Len(t, awaiting, 1, "only the visit that came through a channel")
	assert.Equal(t, whatsapp.ID, awaiting[0].ID)
	stale, err := repo.ListAwaitingFeedback(ctx, visitedAt.Add(time.Hour))
	require.NoError(t, err)
	assert.Empty(t, stale, "visited before the window")

	feedback := repository.NewFeedbackRepository(cloudDB)
	request := &dto.FeedbackRequest{ID: uuid.NewString(), ReservationID: whatsapp.ID, ContactID: whatsapp.ContactID, RequestedAt: time.Now(), ExpiresAt: time.Now().Add(7 * 24 * time.Hour)}
	require.NoError(t, feedback.CreateRequest(ctx, request))
	request.ID = uuid.NewString()
	assert.ErrorIs(t, feedback.CreateRequest(ctx, request), domainError.ErrFeedbackRequestExists, "one request per reservation")

	afterRequest, err := repo.ListAwaitingFeedback(ctx, visitedAt.Add(-time.Hour))
	require.NoError(t, err)
	assert.Empty(t, afterRequest)
	read, err := repo.FindByID(ctx, whatsapp.ID)
	require.NoError(t, err)
	assert.True(t, read.FeedbackRequested)

	open, err := feedback.FindLatestOpenRequest(ctx, guestPhone, time.Now())
	require.NoError(t, err)
	assert.Equal(t, whatsapp.ID, open.ReservationID)
	_, err = feedback.FindLatestOpenRequest(ctx, guestPhone, time.Now().Add(8*24*time.Hour))
	assert.ErrorIs(t, err, domainError.ErrNoVisitAwaitingFeedback, "closed after 7 days")
}

func TestMarketingEventOutbox_ClaimSettleAndOrgScope(t *testing.T) {
	newCloud(t)
	orgA, orgB := seedOrganization(t), seedOrganization(t)
	ctxA, ctxB := tenant.WithOrg(context.Background(), orgA), tenant.WithOrg(context.Background(), orgB)
	outbox := repository.NewMarketingEventOutboxRepository(cloudDB)
	uow := postgres.NewUnitOfWork(cloudDB)
	now := time.Now().UTC().Truncate(time.Microsecond)
	event := func() *dto.MarketingEvent {
		return &dto.MarketingEvent{EventID: uuid.NewString(), Type: dto.MarketingEventOrderPaid, Path: dto.MarketingEventsPathOrders,
			Body: []byte(`{"a":1}`), NextAttemptAt: now, CreatedAt: now}
	}

	kept, rolledBack := event(), event()
	require.NoError(t, uow.Do(ctxA, func(ctx context.Context) error { return outbox.Append(ctx, kept) }))
	_ = uow.Do(ctxA, func(ctx context.Context) error {
		require.NoError(t, outbox.Append(ctx, rolledBack))
		return assert.AnError
	})
	require.NoError(t, uow.Do(ctxB, func(ctx context.Context) error { return outbox.Append(ctx, event()) }))

	claimed, err := outbox.ClaimDue(ctxA, now, now.Add(2*time.Minute), 10)
	require.NoError(t, err)
	require.Len(t, claimed, 1, "the rolled-back row does not exist and org B's is not A's")
	assert.Equal(t, kept.EventID, claimed[0].EventID)
	assert.Equal(t, 1, claimed[0].Attempts)
	assert.Equal(t, `{"a":1}`, string(claimed[0].Body))

	again, err := outbox.ClaimDue(ctxA, now, now.Add(2*time.Minute), 10)
	require.NoError(t, err)
	assert.Empty(t, again, "leased until its next attempt")

	require.NoError(t, outbox.MarkFailed(ctxA, kept.EventID, dto.MarketingEventPending, now, "HTTP 503"))
	retried, err := outbox.ClaimDue(ctxA, now, now.Add(2*time.Minute), 10)
	require.NoError(t, err)
	require.Len(t, retried, 1)
	assert.Equal(t, 2, retried[0].Attempts)
	require.NoError(t, outbox.MarkSent(ctxA, kept.EventID, now))
	done, err := outbox.ClaimDue(ctxA, now.Add(time.Hour), now.Add(2*time.Hour), 10)
	require.NoError(t, err)
	assert.Empty(t, done, "sent is never sent again")
}

func TestMarketingEventOutbox_ConcurrentSendersSkipEachOthersRows(t *testing.T) {
	newCloud(t)
	org := seedOrganization(t)
	ctx := tenant.WithOrg(context.Background(), org)
	outbox := repository.NewMarketingEventOutboxRepository(cloudDB)
	now := time.Now().UTC()
	require.NoError(t, outbox.Append(ctx, &dto.MarketingEvent{EventID: uuid.NewString(), Type: dto.MarketingEventOrderPaid,
		Path: dto.MarketingEventsPathOrders, Body: []byte(`{}`), NextAttemptAt: now, CreatedAt: now}))

	holder := cloudDB.Begin()
	t.Cleanup(func() { holder.Rollback() })
	require.NoError(t, holder.Exec(`SELECT event_id FROM marketing_event_outbox WHERE organization_id = ? FOR UPDATE`, org).Error)

	claimed, err := outbox.ClaimDue(ctx, now, now.Add(time.Minute), 10)

	require.NoError(t, err)
	assert.Empty(t, claimed, "a row another sender holds is skipped, not waited on")
}
