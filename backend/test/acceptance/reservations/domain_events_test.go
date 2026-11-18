package reservations

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/eventconsumer"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/postgres"
	"laguna-escondida/backend/internal/platform/postgres/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubConsumer struct{ name string }

func (c stubConsumer) Name() string                                  { return c.name }
func (c stubConsumer) Handles() []string                             { return []string{dto.DomainEventOrderPaid} }
func (c stubConsumer) Handle(context.Context, dto.StoredEvent) error { return nil }

func TestDomainEventRepository_PublishClaimAndOrgScope(t *testing.T) {
	newCloud(t)
	orgA, orgB := seedOrganization(t), seedOrganization(t)
	ctxA, ctxB := tenant.WithOrg(context.Background(), orgA), tenant.WithOrg(context.Background(), orgB)
	registry, err := eventconsumer.NewRegistry(stubConsumer{"first"}, stubConsumer{"second"})
	require.NoError(t, err)
	repo := repository.NewDomainEventRepository(cloudDB, registry)
	uow := postgres.NewUnitOfWork(cloudDB)
	now := time.Now().UTC().Truncate(time.Microsecond)
	event := func() dto.OrderPaid {
		return dto.OrderPaid{DomainEventMeta: dto.DomainEventMeta{
			ID: uuid.NewString(), Name: dto.DomainEventOrderPaid, Subject: uuid.NewString(), Time: now,
		}, OrderType: dto.OrderTypeDelivery}
	}

	kept, rolledBack := event(), event()
	require.NoError(t, uow.Do(ctxA, func(ctx context.Context) error { return repo.Publish(ctx, kept) }))
	_ = uow.Do(ctxA, func(ctx context.Context) error {
		require.NoError(t, repo.Publish(ctx, rolledBack))
		return assert.AnError
	})
	require.NoError(t, uow.Do(ctxB, func(ctx context.Context) error { return repo.Publish(ctx, event()) }))

	var stored int64
	require.NoError(t, cloudDB.Table("domain_events").Where("id = ?", rolledBack.ID).Count(&stored).Error)
	assert.Zero(t, stored, "a rolled-back unit of work leaves no event")

	claimed, err := repo.ClaimDue(ctxA, now.Add(time.Second), now.Add(time.Minute), 10)
	require.NoError(t, err)
	require.Len(t, claimed, 2, "one delivery per registered consumer, none of org B's")
	assert.ElementsMatch(t, []string{"first", "second"}, []string{claimed[0].Consumer, claimed[1].Consumer})
	for _, c := range claimed {
		assert.Equal(t, kept.ID, c.Event.ID)
		assert.Equal(t, orgA, c.Event.OrganizationID)
		assert.Equal(t, 1, c.Attempts)
		var payload struct {
			ID string `json:"id"`
		}
		require.NoError(t, json.Unmarshal(c.Event.Payload, &payload))
		assert.Equal(t, kept.ID, payload.ID)
	}

	again, err := repo.ClaimDue(ctxA, now.Add(2*time.Second), now.Add(time.Minute), 10)
	require.NoError(t, err)
	assert.Empty(t, again, "a claimed delivery is not reclaimed within its lease")

	require.NoError(t, repo.MarkDone(ctxA, kept.ID, "first", now))
	require.NoError(t, repo.MarkFailed(ctxA, kept.ID, "second", dto.EventDeliveryPending, now.Add(time.Second), "boom"))
	retried, err := repo.ClaimDue(ctxA, now.Add(2*time.Second), now.Add(time.Minute), 10)
	require.NoError(t, err)
	require.Len(t, retried, 1)
	assert.Equal(t, "second", retried[0].Consumer)
	assert.Equal(t, 2, retried[0].Attempts)

	require.NoError(t, repo.MarkFailed(ctxA, kept.ID, "second", dto.EventDeliveryDead, now, "gave up"))
	dead, err := repo.ClaimDue(ctxA, now.Add(time.Hour), now.Add(2*time.Hour), 10)
	require.NoError(t, err)
	assert.Empty(t, dead, "done and dead are never claimed")

	other, err := repo.ClaimDue(ctxB, now.Add(time.Second), now.Add(time.Minute), 10)
	require.NoError(t, err)
	require.Len(t, other, 2, "org B sees only its own event's deliveries")
	assert.NotEqual(t, kept.ID, other[0].Event.ID)
}
