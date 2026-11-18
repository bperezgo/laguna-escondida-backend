package sync

import (
	"encoding/json"
	"testing"
	"time"

	openBill "laguna-escondida/backend/internal/domain/aggregate/open_bill"
	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres/repository"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const kitchenArea = "kitchen"

// createKitchenOrder writes one open order with a single kitchen line on the cloud DB and
// returns the order and line ids.
func (r *rig) createKitchenOrder(repo ports.OpenBillRepository, userID, productID string) (orderID, lineID string) {
	r.t.Helper()
	orderID, lineID = uuid.NewString(), uuid.NewString()
	area := kitchenArea
	line, err := openBill.NewOpenBillProduct(lineID, productID, 1, nil, &area, 1, userID)
	require.NoError(r.t, err)
	aggregate, err := openBill.NewAggregate(orderID, uuid.NewString(), nil, decimal.NewFromInt(5000), []*openBill.OpenBillProduct{line}, userID)
	require.NoError(r.t, err)
	require.NoError(r.t, repo.Create(r.ctx, aggregate))
	return orderID, lineID
}

func pendingLineIDs(t *testing.T, r *rig, repo ports.OpenBillRepository) map[string]dto.OpenBillProductSSE {
	t.Helper()
	lines, err := repo.FindPendingByArea(r.ctx, kitchenArea)
	require.NoError(t, err)
	byID := make(map[string]dto.OpenBillProductSSE, len(lines))
	for _, l := range lines {
		byID[l.OpenBillProductID] = *l
	}
	return byID
}

// A paid order's unfinished line stays on the board; voided and historic closed orders don't;
// completing the paid line moves it to the review with its real completion instant.
func TestKitchen_PaidOrderLinesOutlivePayment(t *testing.T) {
	r := newRig(t)
	userID, productID := r.seedCloudUserAndProduct(t)
	repo := repository.NewOpenBillRepository(r.cloudDB)

	paidOrder, paidLine := r.createKitchenOrder(repo, userID, productID)
	voidedOrder, voidedLine := r.createKitchenOrder(repo, userID, productID)
	historicOrder, historicLine := r.createKitchenOrder(repo, userID, productID)

	require.NoError(t, repo.ClosePaid(r.ctx, paidOrder, time.Now()))
	require.NoError(t, repo.Delete(r.ctx, voidedOrder))
	// An order closed before paid_at existed: deleted_at set, paid_at NULL.
	require.NoError(t, r.cloudDB.Exec("UPDATE open_bills SET deleted_at = now() WHERE id = ?", historicOrder).Error)

	board := pendingLineIDs(t, r, repo)
	assert.Contains(t, board, paidLine, "a paid order's unfinished line stays on the board")
	assert.NotContains(t, board, voidedLine, "a voided order's line leaves the board")
	assert.NotContains(t, board, historicLine, "a historic closed order's line does not reappear")

	_, err := repo.FindAggregateForKitchen(r.ctx, voidedOrder)
	require.Error(t, err, "a voided order is not loadable for kitchen transitions")
	_, err = repo.FindAggregateByID(r.ctx, paidOrder)
	require.Error(t, err, "the strict load still treats a paid order as closed")

	aggregate, err := repo.FindAggregateForKitchen(r.ctx, paidOrder)
	require.NoError(t, err)
	require.True(t, aggregate.IsPaid())
	require.NoError(t, aggregate.SetProductInProgress(paidLine))
	require.NoError(t, repo.UpdateProductStatus(r.ctx, aggregate))
	require.NoError(t, aggregate.CompleteProduct(paidLine))
	require.NoError(t, repo.UpdateProductStatus(r.ctx, aggregate))
	completedAt := *aggregate.GetProduct(paidLine).CompletedAt()

	assert.NotContains(t, pendingLineIDs(t, r, repo), paidLine, "the finished comanda leaves the board")

	review, err := repo.FindCompletedByAreaBetween(r.ctx, kitchenArea, completedAt.Add(-time.Minute), completedAt.Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, review, 1)
	assert.Equal(t, paidLine, review[0].OpenBillProductID)
	require.NotNil(t, review[0].CompletedAt)
	assert.WithinDuration(t, completedAt, *review[0].CompletedAt, time.Microsecond, "review reports completed_at, not updated_at")

	// A later write to the line must not move its completion.
	require.NoError(t, r.cloudDB.Exec("UPDATE open_bills_products SET updated_at = now() + interval '1 hour' WHERE id = ?", paidLine).Error)
	reloaded, err := repo.FindAggregateForKitchen(r.ctx, paidOrder)
	require.NoError(t, err)
	assert.WithinDuration(t, completedAt, *reloaded.GetProduct(paidLine).CompletedAt(), time.Microsecond)
	assert.NotNil(t, reloaded.GetProduct(paidLine).StartedAt())

	assert.Equal(t, int64(1), r.cloudCount("open_bills", "id = ? AND deleted_at IS NOT NULL AND paid_at IS NOT NULL", paidOrder),
		"line activity leaves the paid order closed")
}

func openBillPayload(orderID, lineID, userID, productID string, status dto.CommandStatus) dto.OpenBillSyncPayload {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return dto.OpenBillSyncPayload{
		ID:                 orderID,
		TemporalIdentifier: uuid.NewString(),
		TotalAmount:        decimal.NewFromInt(5000),
		Status:             dto.CommandStatusCreated,
		CreatedByID:        userID,
		Products: []dto.OpenBillSyncProduct{
			{OpenBillProductID: lineID, ProductID: productID, Quantity: 1, Status: status},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (r *rig) applyOpenBill(op dto.SyncOperation, payload dto.OpenBillSyncPayload) {
	r.t.Helper()
	raw, err := json.Marshal(payload)
	require.NoError(r.t, err)
	resp := r.cloudApply(&dto.SyncPushRequest{NodeID: r.identity.NodeID, Ops: []dto.SyncOutboxEntry{{
		OpID:         uuid.NewString(),
		OriginNodeID: r.identity.NodeID,
		EntityType:   dto.SyncEntityOpenBill,
		EntityID:     payload.ID,
		Operation:    op,
		Payload:      raw,
	}}})
	require.Len(r.t, resp.AckedOpIDs, 1)
}

type cloudOrderRow struct {
	DeletedAt *time.Time
	PaidAt    *time.Time
}

type cloudLineRow struct {
	Status      string
	StartedAt   *time.Time
	CompletedAt *time.Time
}

func (r *rig) cloudOrder(id string) cloudOrderRow {
	r.t.Helper()
	var row cloudOrderRow
	require.NoError(r.t, r.cloudDB.Table("open_bills").Select("deleted_at, paid_at").Where("id = ?", id).Scan(&row).Error)
	return row
}

func (r *rig) cloudLine(id string) cloudLineRow {
	r.t.Helper()
	var row cloudLineRow
	require.NoError(r.t, r.cloudDB.Table("open_bills_products").Select("status, started_at, completed_at").Where("id = ?", id).Scan(&row).Error)
	return row
}

func TestSync_PaidOrder_ReplicatesClosureAndLineTiming(t *testing.T) {
	r := newRig(t)
	userID, productID := r.seedCloudUserAndProduct(t)
	orderID, lineID := uuid.NewString(), uuid.NewString()

	r.applyOpenBill(dto.SyncOperationCreate, openBillPayload(orderID, lineID, userID, productID, dto.CommandStatusInProgress))

	// (a) the pay snapshot closes the order and marks it paid.
	paidAt := time.Now().UTC().Truncate(time.Microsecond)
	pay := openBillPayload(orderID, lineID, userID, productID, dto.CommandStatusInProgress)
	pay.ClosedAt, pay.PaidAt = &paidAt, &paidAt
	r.applyOpenBill(dto.SyncOperationUpdate, pay)

	order := r.cloudOrder(orderID)
	require.NotNil(t, order.DeletedAt, "pay closes the order on the cloud")
	require.NotNil(t, order.PaidAt, "and records it as paid, not voided")
	assert.WithinDuration(t, paidAt, *order.PaidAt, time.Microsecond)

	// (b) a later line completion on the paid order keeps it closed and paid.
	startedAt := paidAt.Add(-10 * time.Minute)
	completedAt := paidAt.Add(4 * time.Minute)
	complete := openBillPayload(orderID, lineID, userID, productID, dto.CommandStatusCompleted)
	complete.ClosedAt, complete.PaidAt = &paidAt, &paidAt
	complete.Products[0].StartedAt, complete.Products[0].CompletedAt = &startedAt, &completedAt
	r.applyOpenBill(dto.SyncOperationUpdate, complete)

	order = r.cloudOrder(orderID)
	assert.NotNil(t, order.DeletedAt, "line activity must not reopen a paid order")
	assert.NotNil(t, order.PaidAt)
	line := r.cloudLine(lineID)
	assert.Equal(t, string(dto.CommandStatusCompleted), line.Status)
	require.NotNil(t, line.StartedAt)
	require.NotNil(t, line.CompletedAt)
	assert.WithinDuration(t, startedAt, *line.StartedAt, time.Microsecond)
	assert.WithinDuration(t, completedAt, *line.CompletedAt, time.Microsecond)
}

// (c) a node predating this change sends none of the new fields: a snapshot still reopens
// a closed order exactly as before.
func TestSync_PaidOrder_LegacyPayloadReopensAsBefore(t *testing.T) {
	r := newRig(t)
	userID, productID := r.seedCloudUserAndProduct(t)
	orderID, lineID := uuid.NewString(), uuid.NewString()

	r.applyOpenBill(dto.SyncOperationCreate, openBillPayload(orderID, lineID, userID, productID, dto.CommandStatusCreated))
	raw, err := json.Marshal(dto.SyncTombstone{ID: orderID})
	require.NoError(t, err)
	r.cloudApply(&dto.SyncPushRequest{NodeID: r.identity.NodeID, Ops: []dto.SyncOutboxEntry{{
		OpID: uuid.NewString(), OriginNodeID: r.identity.NodeID, EntityType: dto.SyncEntityOpenBill,
		EntityID: orderID, Operation: dto.SyncOperationDelete, Payload: raw,
	}}})
	require.NotNil(t, r.cloudOrder(orderID).DeletedAt)

	r.applyOpenBill(dto.SyncOperationUpdate, openBillPayload(orderID, lineID, userID, productID, dto.CommandStatusCreated))

	order := r.cloudOrder(orderID)
	assert.Nil(t, order.DeletedAt, "a legacy snapshot reopens the order, as it always did")
	assert.Nil(t, order.PaidAt)
}
