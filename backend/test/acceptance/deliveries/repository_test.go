package deliveries

import (
	"context"
	"sync"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/platform/postgres"
	"laguna-escondida/backend/internal/platform/postgres/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func claim(orderID, courierID, name string, at time.Time) *dto.DeliveryStatusChange {
	return &dto.DeliveryStatusChange{
		EventID: uuid.NewString(), OrderID: orderID, Status: dto.DeliveryStatusOutForDelivery,
		CourierUserID: &courierID, CourierName: &name, ActorUserID: courierID, OccurredAt: at,
	}
}

func TestDeliveryRepository_TwoConcurrentClaimsHaveOneWinner(t *testing.T) {
	r := seedRestaurant(t)
	repo := repository.NewDeliveryRepository(cloudDB)
	uow := postgres.NewUnitOfWork(cloudDB)
	juan := r.seedUser("Juan", permissions.RoleCourier)
	pedro := r.seedUser("Pedro", permissions.RoleCourier)
	order := r.seedOrder(seededOrder{orderType: dto.OrderTypeDelivery, paidAt: at(time.Now()), lineStatus: []dto.CommandStatus{dto.CommandStatusCompleted}})

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		wins []string
	)
	start := make(chan struct{})
	for _, courier := range []struct{ id, name string }{{juan, "Juan"}, {pedro, "Pedro"}} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			err := uow.Do(r.ctx, func(ctx context.Context) error {
				changed, err := repo.ApplyStatusChange(ctx, claim(order, courier.id, courier.name, time.Now()))
				if changed {
					mu.Lock()
					wins = append(wins, courier.id)
					mu.Unlock()
				}
				return err
			})
			assert.NoError(t, err)
		}()
	}
	close(start)
	wg.Wait()

	require.Len(t, wins, 1, "exactly one claim wins")
	assert.Equal(t, int64(1), r.historyCount(order), "and only the winner is in the history")
	state, err := repo.FindTrackingState(r.ctx, order)
	require.NoError(t, err)
	assert.Equal(t, dto.DeliveryStatusOutForDelivery, state.Status)
	assert.Equal(t, wins[0], *state.CourierUserID)
}

func TestDeliveryRepository_StaleChangeIsNotApplied(t *testing.T) {
	r := seedRestaurant(t)
	repo := repository.NewDeliveryRepository(cloudDB)
	juan := r.seedUser("Juan", permissions.RoleCourier)
	order := r.seedOrder(seededOrder{orderType: dto.OrderTypeDelivery, paidAt: at(time.Now()), lineStatus: []dto.CommandStatus{dto.CommandStatusCompleted}})

	changed, err := repo.ApplyStatusChange(r.ctx, claim(order, juan, "Juan", time.Now()))
	require.NoError(t, err)
	require.True(t, changed)

	out := dto.DeliveryStatusOutForDelivery
	deliver := &dto.DeliveryStatusChange{EventID: uuid.NewString(), OrderID: order, ExpectedStatus: &out, ExpectedCourierUserID: &juan,
		Status: dto.DeliveryStatusDelivered, CourierUserID: &juan, CourierName: strPtr("Juan"), ActorUserID: juan, OccurredAt: time.Now()}
	stale := *deliver
	stale.EventID = uuid.NewString()
	stale.ExpectedStatus = nil
	changed, err = repo.ApplyStatusChange(r.ctx, &stale)
	require.NoError(t, err)
	assert.False(t, changed, "the order is no longer where the caller saw it")

	changed, err = repo.ApplyStatusChange(r.ctx, deliver)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, int64(2), r.historyCount(order))
}

func TestDeliveryRepository_TrackingStateAndDerivedStatus(t *testing.T) {
	r := seedRestaurant(t)
	repo := repository.NewDeliveryRepository(cloudDB)
	now := time.Now()

	cooking := r.seedOrder(seededOrder{orderType: dto.OrderTypeDelivery, paidAt: at(now), lineStatus: []dto.CommandStatus{dto.CommandStatusCompleted, dto.CommandStatusInProgress}})
	done := r.seedOrder(seededOrder{orderType: dto.OrderTypeTakeaway, paidAt: at(now), lineStatus: []dto.CommandStatus{dto.CommandStatusCompleted, dto.CommandStatusCancelled}})
	unpaid := r.seedOrder(seededOrder{orderType: dto.OrderTypeDelivery, lineStatus: []dto.CommandStatus{dto.CommandStatusCreated}})
	voided := r.seedOrder(seededOrder{orderType: dto.OrderTypeDelivery, voided: true, lineStatus: []dto.CommandStatus{dto.CommandStatusCreated}})
	dineIn := r.seedOrder(seededOrder{orderType: dto.OrderTypeDineIn, paidAt: at(now), lineStatus: []dto.CommandStatus{dto.CommandStatusCompleted}})

	for _, tc := range []struct {
		id     string
		status dto.DeliveryStatus
		paid   bool
		voided bool
	}{
		{cooking, dto.DeliveryStatusPreparing, true, false},
		{done, dto.DeliveryStatusReady, true, false},
		{unpaid, "", false, false},
		{voided, "", false, true},
		{dineIn, "", true, false},
	} {
		state, err := repo.FindTrackingState(r.ctx, tc.id)
		require.NoError(t, err)
		assert.Equal(t, tc.status, state.Status, tc.id)
		assert.Equal(t, tc.paid, state.Paid, tc.id)
		assert.Equal(t, tc.voided, state.Voided, tc.id)
	}

	other := seedRestaurant(t)
	_, err := repository.NewDeliveryRepository(cloudDB).FindTrackingState(other.ctx, cooking)
	require.Error(t, err, "another organization's order is not found")
}

func TestDeliveryRepository_MarkReadyOnce(t *testing.T) {
	r := seedRestaurant(t)
	repo := repository.NewDeliveryRepository(cloudDB)
	order := r.seedOrder(seededOrder{orderType: dto.OrderTypeTakeaway, paidAt: at(time.Now()), lineStatus: []dto.CommandStatus{dto.CommandStatusCompleted}})
	dineIn := r.seedOrder(seededOrder{orderType: dto.OrderTypeDineIn, paidAt: at(time.Now()), lineStatus: []dto.CommandStatus{dto.CommandStatusCompleted}})

	first, err := repo.MarkReady(r.ctx, order, time.Now())
	require.NoError(t, err)
	assert.True(t, first)
	again, err := repo.MarkReady(r.ctx, order, time.Now())
	require.NoError(t, err)
	assert.False(t, again, "ready_at is set once")
	untracked, err := repo.MarkReady(r.ctx, dineIn, time.Now())
	require.NoError(t, err)
	assert.False(t, untracked)
}

func TestDeliveryRepository_ListMirrorsTheDashboard(t *testing.T) {
	r := seedRestaurant(t)
	repo := repository.NewDeliveryRepository(cloudDB)
	juan := r.seedUser("Juan", permissions.RoleCourier)
	now := time.Now()
	yesterday := now.Add(-26 * time.Hour)
	dayStart := now.Add(-2 * time.Hour)

	// Yesterday's undelivered order: still failed, still listed.
	failedYesterday := r.seedOrder(seededOrder{orderType: dto.OrderTypeDelivery, paidAt: at(yesterday), lineStatus: []dto.CommandStatus{dto.CommandStatusCompleted}})
	_, err := repo.ApplyStatusChange(r.ctx, claim(failedYesterday, juan, "Juan", yesterday))
	require.NoError(t, err)
	out := dto.DeliveryStatusOutForDelivery
	reason := dto.DeliveryFailureWrongAddress
	_, err = repo.ApplyStatusChange(r.ctx, &dto.DeliveryStatusChange{EventID: uuid.NewString(), OrderID: failedYesterday,
		ExpectedStatus: &out, ExpectedCourierUserID: &juan, Status: dto.DeliveryStatusFailed, CourierUserID: &juan,
		CourierName: strPtr("Juan"), Reason: &reason, Note: strPtr("no existe"), ActorUserID: juan, OccurredAt: yesterday.Add(time.Hour)})
	require.NoError(t, err)

	deliveredYesterday := r.seedOrder(seededOrder{orderType: dto.OrderTypeTakeaway, paidAt: at(yesterday), lineStatus: []dto.CommandStatus{dto.CommandStatusCompleted}})
	_, err = repo.ApplyStatusChange(r.ctx, &dto.DeliveryStatusChange{EventID: uuid.NewString(), OrderID: deliveredYesterday,
		Status: dto.DeliveryStatusDelivered, ActorUserID: juan, OccurredAt: yesterday})
	require.NoError(t, err)
	deliveredToday := r.seedOrder(seededOrder{orderType: dto.OrderTypeTakeaway, paidAt: at(now), lineStatus: []dto.CommandStatus{dto.CommandStatusCompleted}})
	_, err = repo.ApplyStatusChange(r.ctx, &dto.DeliveryStatusChange{EventID: uuid.NewString(), OrderID: deliveredToday,
		Status: dto.DeliveryStatusDelivered, ActorUserID: juan, OccurredAt: now})
	require.NoError(t, err)

	onTheRoad := r.seedOrder(seededOrder{orderType: dto.OrderTypeDelivery, paidAt: at(now), lineStatus: []dto.CommandStatus{dto.CommandStatusCompleted}})
	_, err = repo.ApplyStatusChange(r.ctx, claim(onTheRoad, juan, "Juan", now))
	require.NoError(t, err)
	preparing := r.seedOrder(seededOrder{orderType: dto.OrderTypeDelivery, paidAt: at(now), lineStatus: []dto.CommandStatus{dto.CommandStatusInProgress}})
	ready := r.seedOrder(seededOrder{orderType: dto.OrderTypeTakeaway, paidAt: at(now), lineStatus: []dto.CommandStatus{dto.CommandStatusCompleted}})
	r.seedOrder(seededOrder{orderType: dto.OrderTypeDineIn, paidAt: at(now), lineStatus: []dto.CommandStatus{dto.CommandStatusCompleted}})
	r.seedOrder(seededOrder{orderType: dto.OrderTypeDelivery, lineStatus: []dto.CommandStatus{dto.CommandStatusCreated}})
	r.seedOrder(seededOrder{orderType: dto.OrderTypeDelivery, voided: true, lineStatus: []dto.CommandStatus{dto.CommandStatusCreated}})

	ids := func(orders []dto.DeliveryOrder) []string {
		out := make([]string, 0, len(orders))
		for _, o := range orders {
			out = append(out, o.ID)
		}
		return out
	}

	all, err := repo.List(r.ctx, dto.DeliveryListFilter{}, dayStart)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{failedYesterday, deliveredToday, onTheRoad, preparing, ready}, ids(all))

	byID := map[string]dto.DeliveryOrder{}
	for _, o := range all {
		byID[o.ID] = o
	}
	assert.Equal(t, dto.DeliveryStatusFailed, byID[failedYesterday].DeliveryStatus)
	require.NotNil(t, byID[failedYesterday].FailureReason)
	assert.Equal(t, dto.DeliveryFailureWrongAddress, *byID[failedYesterday].FailureReason)
	assert.Equal(t, dto.DeliveryStatusPreparing, byID[preparing].DeliveryStatus)
	assert.Equal(t, dto.DeliveryStatusReady, byID[ready].DeliveryStatus)
	require.NotNil(t, byID[onTheRoad].Courier)
	assert.Equal(t, "Juan", byID[onTheRoad].Courier.Name)
	assert.NotNil(t, byID[onTheRoad].DeliveryMapURL, "deliveries carry their map link")
	require.Len(t, byID[onTheRoad].Lines, 1)
	assert.Equal(t, "Mojarra frita", byID[onTheRoad].Lines[0].Name)
	assert.Equal(t, 2, byID[onTheRoad].Lines[0].Quantity)

	road, err := repo.List(r.ctx, dto.DeliveryListFilter{Status: dto.DeliveryStatusOutForDelivery}, dayStart)
	require.NoError(t, err)
	assert.Equal(t, []string{onTheRoad}, ids(road))
	prep, err := repo.List(r.ctx, dto.DeliveryListFilter{Status: dto.DeliveryStatusPreparing}, dayStart)
	require.NoError(t, err)
	assert.Equal(t, []string{preparing}, ids(prep))
	takeaway, err := repo.List(r.ctx, dto.DeliveryListFilter{OrderType: dto.OrderTypeTakeaway}, dayStart)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{deliveredToday, ready}, ids(takeaway))
}

func TestDeliveryRepository_HistoryFailedThenDelivered(t *testing.T) {
	r := seedRestaurant(t)
	repo := repository.NewDeliveryRepository(cloudDB)
	manager := r.seedUser("Gerente", permissions.RoleManager)
	juan := r.seedUser("Juan", permissions.RoleCourier)
	pedro := r.seedUser("Pedro", permissions.RoleCourier)
	paidAt := time.Now().Add(-time.Hour).UTC().Truncate(time.Microsecond)
	order := r.seedOrder(seededOrder{orderType: dto.OrderTypeDelivery, paidAt: &paidAt, lineStatus: []dto.CommandStatus{dto.CommandStatusCompleted}})
	readyAt := paidAt.Add(10 * time.Minute)
	_, err := repo.MarkReady(r.ctx, order, readyAt)
	require.NoError(t, err)

	base := paidAt.Add(15 * time.Minute)
	out, failed := dto.DeliveryStatusOutForDelivery, dto.DeliveryStatusFailed
	reason := dto.DeliveryFailureWrongAddress
	steps := []*dto.DeliveryStatusChange{
		{Status: out, CourierUserID: &juan, CourierName: strPtr("Juan"), ActorUserID: manager, OccurredAt: base},
		{ExpectedStatus: &out, ExpectedCourierUserID: &juan, Status: failed, CourierUserID: &juan, CourierName: strPtr("Juan"),
			Reason: &reason, Note: strPtr("la dirección no existe"), ActorUserID: juan, OccurredAt: base.Add(time.Minute)},
		{ExpectedStatus: &failed, ExpectedCourierUserID: &juan, Status: out, CourierUserID: &pedro, CourierName: strPtr("Pedro"), ActorUserID: manager, OccurredAt: base.Add(2 * time.Minute)},
		{ExpectedStatus: &out, ExpectedCourierUserID: &pedro, Status: dto.DeliveryStatusDelivered, CourierUserID: &pedro, CourierName: strPtr("Pedro"), ActorUserID: pedro, OccurredAt: base.Add(3 * time.Minute)},
	}
	for _, step := range steps {
		step.EventID, step.OrderID = uuid.NewString(), order
		changed, applyErr := repo.ApplyStatusChange(r.ctx, step)
		require.NoError(t, applyErr)
		require.True(t, changed)
	}

	history, err := repo.History(r.ctx, order)
	require.NoError(t, err)
	assert.WithinDuration(t, paidAt, history.PaidAt, time.Microsecond)
	require.NotNil(t, history.ReadyAt)
	assert.WithinDuration(t, readyAt, *history.ReadyAt, time.Microsecond)
	require.Len(t, history.Entries, 4)
	assert.Equal(t, []dto.DeliveryStatus{out, failed, out, dto.DeliveryStatusDelivered},
		[]dto.DeliveryStatus{history.Entries[0].Status, history.Entries[1].Status, history.Entries[2].Status, history.Entries[3].Status})
	failedEntry := history.Entries[1]
	assert.Equal(t, "Juan", failedEntry.Courier.Name)
	assert.Equal(t, juan, *failedEntry.Courier.UserID)
	assert.Equal(t, reason, *failedEntry.Reason)
	assert.Equal(t, "la dirección no existe", *failedEntry.Note)
	assert.Equal(t, "Juan", failedEntry.Actor.Name)
	assert.Equal(t, "Pedro", history.Entries[2].Courier.Name)
	assert.Equal(t, "Gerente", history.Entries[2].Actor.Name)
}

func TestDeliveryRepository_JuansQueue(t *testing.T) {
	r := seedRestaurant(t)
	repo := repository.NewDeliveryRepository(cloudDB)
	juan := r.seedUser("Juan", permissions.RoleCourier)
	pedro := r.seedUser("Pedro", permissions.RoleCourier)
	now := time.Now()
	completed := []dto.CommandStatus{dto.CommandStatusCompleted}

	orderA := r.seedOrder(seededOrder{orderType: dto.OrderTypeDelivery, paidAt: at(now), lineStatus: completed})
	orderB := r.seedOrder(seededOrder{orderType: dto.OrderTypeDelivery, paidAt: at(now), lineStatus: completed})
	orderC := r.seedOrder(seededOrder{orderType: dto.OrderTypeDelivery, paidAt: at(now), lineStatus: completed})
	noon := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, time.UTC)
	for _, c := range []*dto.DeliveryStatusChange{
		claim(orderA, juan, "Juan", noon.Add(10*time.Minute)),
		claim(orderB, juan, "Juan", noon.Add(5*time.Minute)),
		claim(orderC, pedro, "Pedro", noon.Add(7*time.Minute)),
	} {
		_, err := repo.ApplyStatusChange(r.ctx, c)
		require.NoError(t, err)
	}

	mine, err := repo.OnTheRoad(r.ctx, juan)
	require.NoError(t, err)
	require.Len(t, mine, 2)
	assert.Equal(t, []string{orderB, orderA}, []string{mine[0].ID, mine[1].ID}, "oldest dispatch first, never Pedro's")

	readyLater := r.seedOrder(seededOrder{orderType: dto.OrderTypeDelivery, paidAt: at(now), lineStatus: completed})
	readyEarlier := r.seedOrder(seededOrder{orderType: dto.OrderTypeDelivery, paidAt: at(now), lineStatus: completed})
	r.seedOrder(seededOrder{orderType: dto.OrderTypeDelivery, paidAt: at(now), lineStatus: []dto.CommandStatus{dto.CommandStatusInProgress}})
	r.seedOrder(seededOrder{orderType: dto.OrderTypeTakeaway, paidAt: at(now), lineStatus: completed})
	_, err = repo.MarkReady(r.ctx, readyLater, now.Add(-time.Minute))
	require.NoError(t, err)
	_, err = repo.MarkReady(r.ctx, readyEarlier, now.Add(-5*time.Minute))
	require.NoError(t, err)

	claimable, err := repo.Claimable(r.ctx)
	require.NoError(t, err)
	require.Len(t, claimable, 2, "only ready delivery orders with no courier")
	assert.Equal(t, []string{readyEarlier, readyLater}, []string{claimable[0].ID, claimable[1].ID})
}

func TestDeliveryRepository_ActiveCouriers(t *testing.T) {
	r := seedRestaurant(t)
	repo := repository.NewDeliveryRepository(cloudDB)
	juan := r.seedUser("Juan", permissions.RoleCourier)
	inactive := r.seedUser("Inactivo", permissions.RoleCourier)
	require.NoError(t, cloudDB.Exec("UPDATE users SET active = false WHERE id = ?", inactive).Error)
	waiter := r.seedUser("Mesero", permissions.RoleServer)
	other := seedRestaurant(t)
	other.seedUser("Ajeno", permissions.RoleCourier)

	couriers, err := repo.ActiveCouriers(r.ctx)
	require.NoError(t, err)
	assert.Equal(t, []dto.CourierUser{{ID: juan, Name: "Juan"}}, couriers)

	found, ok, err := repo.FindActiveCourier(r.ctx, juan)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "Juan", found.Name)
	for _, id := range []string{inactive, waiter} {
		_, ok, err = repo.FindActiveCourier(r.ctx, id)
		require.NoError(t, err)
		assert.False(t, ok, id)
	}
}

func strPtr(s string) *string { return &s }
