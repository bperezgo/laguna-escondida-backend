package service

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newStockReconciliationService(t *testing.T) (*StockReconciliationService, *mocks.MockStockReconciliationRepository) {
	repo := mocks.NewMockStockReconciliationRepository(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewStockReconciliationService(repo, mocks.NewMockStockRepository(t), createMockUnitOfWork(t), createMockStockMovementEmitter(t), logger), repo
}

func TestStockReconciliationService_Reconcile_MatchingProductIsNotReported(t *testing.T) {
	ctx := context.Background()
	service, repo := newStockReconciliationService(t)

	repo.EXPECT().FindLedgerTotals(ctx).Return([]dto.StockLedgerTotal{
		{ProductID: "product-1", Amount: 27, LedgerSum: 27},
	}, nil).Once()

	report, err := service.Reconcile(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, report.CheckedProducts)
	assert.Empty(t, report.Diverged)
}

func TestStockReconciliationService_Reconcile_DivergedProductIsReportedWithBothValues(t *testing.T) {
	ctx := context.Background()
	service, repo := newStockReconciliationService(t)

	repo.EXPECT().FindLedgerTotals(ctx).Return([]dto.StockLedgerTotal{
		{ProductID: "product-ok", Amount: 10, LedgerSum: 10},
		{ProductID: "product-bad", Amount: 30, LedgerSum: 24},
	}, nil).Once()

	report, err := service.Reconcile(ctx)

	require.NoError(t, err)
	assert.Equal(t, 2, report.CheckedProducts)
	require.Len(t, report.Diverged, 1)
	assert.Equal(t, dto.StockDivergence{
		ProductID: "product-bad",
		Amount:    30,
		LedgerSum: 24,
		Delta:     6,
	}, report.Diverged[0])
}

// A product with no movements is only diverged if its amount is non-zero: an untouched
// product legitimately sums to nothing.
func TestStockReconciliationService_Reconcile_ProductWithNoMovements(t *testing.T) {
	ctx := context.Background()
	service, repo := newStockReconciliationService(t)

	repo.EXPECT().FindLedgerTotals(ctx).Return([]dto.StockLedgerTotal{
		{ProductID: "never-moved", Amount: 0, LedgerSum: 0},
		{ProductID: "amount-without-history", Amount: 15, LedgerSum: 0},
	}, nil).Once()

	report, err := service.Reconcile(ctx)

	require.NoError(t, err)
	require.Len(t, report.Diverged, 1)
	assert.Equal(t, "amount-without-history", report.Diverged[0].ProductID)
	assert.Equal(t, 15, report.Diverged[0].Delta)
}

func TestStockReconciliationService_Reconcile_RepositoryError(t *testing.T) {
	ctx := context.Background()
	service, repo := newStockReconciliationService(t)

	repo.EXPECT().FindLedgerTotals(ctx).Return(nil, errors.New("boom")).Once()

	report, err := service.Reconcile(ctx)

	require.Error(t, err)
	assert.Nil(t, report)
}

func TestStockReconciliationService_Freshness_EdgeHasPushed(t *testing.T) {
	ctx := context.Background()
	service, repo := newStockReconciliationService(t)

	appliedAt := time.Now().Add(-90 * time.Second)
	repo.EXPECT().LastEdgeMovementAppliedAt(ctx).Return(&appliedAt, nil).Once()

	freshness, err := service.Freshness(ctx)

	require.NoError(t, err)
	require.NotNil(t, freshness.LastMovementAppliedAt)
	require.NotNil(t, freshness.StalenessSeconds)
	assert.InDelta(t, 90, *freshness.StalenessSeconds, 5)
}

func TestStockReconciliationService_Freshness_EdgeHasNeverPushed(t *testing.T) {
	ctx := context.Background()
	service, repo := newStockReconciliationService(t)

	repo.EXPECT().LastEdgeMovementAppliedAt(ctx).Return(nil, nil).Once()

	freshness, err := service.Freshness(ctx)

	require.NoError(t, err)
	assert.Nil(t, freshness.LastMovementAppliedAt, "unknown, not fresh")
	assert.Nil(t, freshness.StalenessSeconds)
}
