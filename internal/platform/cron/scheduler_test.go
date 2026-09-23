package cron

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// newReconcileScheduler builds a cloud scheduler whose only usable job is stock
// reconciliation; the other services stay nil because registerJobs never calls them.
func newReconcileScheduler(t *testing.T, stockCron string) (*Scheduler, *mocks.MockStockReconciliationRepository, *bytes.Buffer) {
	t.Helper()
	repo := mocks.NewMockStockReconciliationRepository(t)
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))

	scheduler, err := NewScheduler(
		nil, nil, nil,
		service.NewStockReconciliationService(repo, nil, nil, nil, logger),
		"0 * * * *", "30 * * * *", "* * * * *", stockCron,
		logger,
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = scheduler.Stop() })
	return scheduler, repo, logs
}

func TestScheduler_RegistersStockReconciliationJob(t *testing.T) {
	scheduler, _, logs := newReconcileScheduler(t, "15 3 * * *")

	require.NoError(t, scheduler.registerJobs())

	assert.Len(t, scheduler.scheduler.Jobs(), 4)
	assert.Contains(t, logs.String(), "Registered cron job: reconcileStock")
}

func TestScheduler_RejectsInvalidStockReconciliationCron(t *testing.T) {
	scheduler, _, _ := newReconcileScheduler(t, "not a cron")

	assert.Error(t, scheduler.registerJobs())
}

// The check is a detector: its own failure is logged, never propagated to the scheduler.
func TestScheduler_ReconcileStockJob_LogsFailureWithoutPropagating(t *testing.T) {
	scheduler, repo, logs := newReconcileScheduler(t, "15 3 * * *")
	repo.EXPECT().FindLedgerTotals(mock.Anything).Return(nil, assert.AnError).Once()

	assert.NotPanics(t, scheduler.reconcileStockJob)

	assert.Contains(t, logs.String(), "Cron job reconcileStock failed")
}

func TestScheduler_ReconcileStockJob_ReportsDivergenceWithoutCorrecting(t *testing.T) {
	scheduler, repo, logs := newReconcileScheduler(t, "15 3 * * *")
	repo.EXPECT().FindLedgerTotals(mock.Anything).Return([]dto.StockLedgerTotal{
		{ProductID: "product-ok", Amount: 10, LedgerSum: 10},
		{ProductID: "product-bad", Amount: 30, LedgerSum: 24},
	}, nil).Once()

	scheduler.reconcileStockJob()

	out := logs.String()
	assert.Contains(t, out, "product-bad")
	assert.Contains(t, out, "ledger_sum=24")
	assert.Contains(t, out, "Cron job reconcileStock found divergences")
	assert.False(t, strings.Contains(out, "product-ok"), "a matching product is not reported")
}
