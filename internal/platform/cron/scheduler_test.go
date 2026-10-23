package cron

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	cronOrgA         = "019a0000-0000-7000-8000-00000000000a"
	cronOrgB         = "019a0000-0000-7000-8000-00000000000b"
	cronOrgSuspended = "019a0000-0000-7000-8000-00000000000c"
)

// cronOrganizations is two active organizations and a suspended one, each run in its own session.
func cronOrganizations(t *testing.T) *service.OrganizationRunner {
	t.Helper()
	orgs := mocks.NewMockOrganizationRepository(t)
	orgs.On("List", mock.Anything).Return([]*dto.Organization{
		{ID: cronOrgA, Status: dto.OrganizationStatusActive},
		{ID: cronOrgB, Status: dto.OrganizationStatusActive},
		{ID: cronOrgSuspended, Status: dto.OrganizationStatusSuspended},
	}, nil).Maybe()
	sessions := mocks.NewMockSessionOpener(t)
	sessions.On("Open", mock.Anything).Return(
		func(ctx context.Context) context.Context { return ctx },
		func(context.Context) func() { return func() {} },
	).Maybe()
	return service.NewOrganizationRunner(orgs, sessions)
}

// inCronOrg matches a context acting for orgID.
func inCronOrg(orgID string) func(ctx context.Context) bool {
	return func(ctx context.Context) bool {
		got, ok := tenant.OrgFrom(ctx)
		return ok && got == orgID
	}
}

// newReconcileScheduler builds a cloud scheduler whose only usable job is stock
// reconciliation; the other services stay nil because registerJobs never calls them.
func newReconcileScheduler(t *testing.T, stockCron string) (*Scheduler, *mocks.MockStockReconciliationRepository, *bytes.Buffer) {
	t.Helper()
	repo := mocks.NewMockStockReconciliationRepository(t)
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))

	scheduler, err := NewScheduler(
		cronOrganizations(t), nil, nil, nil,
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
	repo.EXPECT().FindLedgerTotals(mock.Anything).Return(nil, assert.AnError)

	assert.NotPanics(t, scheduler.reconcileStockJob)

	assert.Contains(t, logs.String(), "Cron job reconcileStock failed")
}

func TestScheduler_ReconcileStockJob_ReportsDivergenceWithoutCorrecting(t *testing.T) {
	scheduler, repo, logs := newReconcileScheduler(t, "15 3 * * *")
	repo.EXPECT().FindLedgerTotals(mock.MatchedBy(inCronOrg(cronOrgA))).Return([]dto.StockLedgerTotal{
		{ProductID: "product-ok", Amount: 10, LedgerSum: 10},
		{ProductID: "product-bad", Amount: 30, LedgerSum: 24},
	}, nil).Once()
	repo.EXPECT().FindLedgerTotals(mock.MatchedBy(inCronOrg(cronOrgB))).Return(nil, nil).Once()

	scheduler.reconcileStockJob()

	out := logs.String()
	assert.Contains(t, out, "product-bad")
	assert.Contains(t, out, "ledger_sum=24")
	assert.Contains(t, out, "Cron job reconcileStock found divergences")
	assert.False(t, strings.Contains(out, "product-ok"), "a matching product is not reported")
}

// Each organization is reconciled on its own data; one failing is logged with its id and the
// other still runs; a suspended one is skipped.
func TestScheduler_ReconcileStockJob_RunsPerOrganization(t *testing.T) {
	scheduler, repo, logs := newReconcileScheduler(t, "15 3 * * *")
	repo.EXPECT().FindLedgerTotals(mock.MatchedBy(inCronOrg(cronOrgA))).Return(nil, assert.AnError).Once()
	repo.EXPECT().FindLedgerTotals(mock.MatchedBy(inCronOrg(cronOrgB))).Return([]dto.StockLedgerTotal{
		{ProductID: "product-b", Amount: 5, LedgerSum: 5},
	}, nil).Once()

	scheduler.reconcileStockJob()

	out := logs.String()
	assert.Contains(t, out, "Cron job reconcileStock failed")
	assert.Contains(t, out, "organization_id="+cronOrgA)
	assert.Contains(t, out, `msg="Cron job reconcileStock completed" organization_id=`+cronOrgB)
	assert.NotContains(t, out, cronOrgSuspended)
	repo.AssertNotCalled(t, "FindLedgerTotals", mock.MatchedBy(inCronOrg(cronOrgSuspended)))
}
