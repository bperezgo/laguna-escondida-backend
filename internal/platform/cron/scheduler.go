package cron

import (
	"context"
	"log/slog"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/go-co-op/gocron/v2"
)

type Scheduler struct {
	scheduler                gocron.Scheduler
	organizations            *service.OrganizationRunner
	invoiceService           *service.InvoiceService
	supportDocumentService   *service.SupportDocumentService
	invoiceSubmissionService *service.InvoiceSubmissionService
	stockReconciliation      *service.StockReconciliationService
	invoiceCron              string
	supportDocumentCron      string
	invoiceSubmitCron        string
	stockReconcileCron       string
	logger                   *slog.Logger
}

// NewScheduler runs every job once per organization through organizations, since tenant data
// is only reachable inside one organization's context.
func NewScheduler(
	organizations *service.OrganizationRunner,
	invoiceService *service.InvoiceService,
	supportDocumentService *service.SupportDocumentService,
	invoiceSubmissionService *service.InvoiceSubmissionService,
	stockReconciliation *service.StockReconciliationService,
	invoiceCron string,
	supportDocumentCron string,
	invoiceSubmitCron string,
	stockReconcileCron string,
	logger *slog.Logger,
) (*Scheduler, error) {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return nil, err
	}

	return &Scheduler{
		scheduler:                scheduler,
		organizations:            organizations,
		invoiceService:           invoiceService,
		supportDocumentService:   supportDocumentService,
		invoiceSubmissionService: invoiceSubmissionService,
		stockReconciliation:      stockReconciliation,
		invoiceCron:              invoiceCron,
		supportDocumentCron:      supportDocumentCron,
		invoiceSubmitCron:        invoiceSubmitCron,
		stockReconcileCron:       stockReconcileCron,
		logger:                   logger,
	}, nil
}

func (s *Scheduler) Start() error {
	if err := s.registerJobs(); err != nil {
		return err
	}
	s.scheduler.Start()
	s.logger.Info("Cron scheduler started")
	return nil
}

func (s *Scheduler) Stop() error {
	s.logger.Info("Stopping cron scheduler...")
	return s.scheduler.Shutdown()
}

func (s *Scheduler) registerJobs() error {
	_, err := s.scheduler.NewJob(
		gocron.CronJob(s.invoiceCron, false),
		gocron.NewTask(s.updateMissingDocumentURLsJob),
	)
	if err != nil {
		s.logger.Error("Failed to register updateMissingDocumentURLs cron job", slog.Any("error", err))
		return err
	}

	s.logger.Info("Registered cron job: updateMissingDocumentURLs", slog.String("cron", s.invoiceCron))

	_, err = s.scheduler.NewJob(
		gocron.CronJob(s.supportDocumentCron, false),
		gocron.NewTask(s.updateMissingSupportDocumentURLsJob),
	)
	if err != nil {
		s.logger.Error("Failed to register updateMissingSupportDocumentURLs cron job", slog.Any("error", err))
		return err
	}

	s.logger.Info("Registered cron job: updateMissingSupportDocumentURLs", slog.String("cron", s.supportDocumentCron))

	_, err = s.scheduler.NewJob(
		gocron.CronJob(s.invoiceSubmitCron, false),
		gocron.NewTask(s.submitPendingInvoicesJob),
	)
	if err != nil {
		s.logger.Error("Failed to register submitPendingInvoices cron job", slog.Any("error", err))
		return err
	}

	s.logger.Info("Registered cron job: submitPendingInvoices", slog.String("cron", s.invoiceSubmitCron))

	_, err = s.scheduler.NewJob(
		gocron.CronJob(s.stockReconcileCron, false),
		gocron.NewTask(s.reconcileStockJob),
	)
	if err != nil {
		s.logger.Error("Failed to register reconcileStock cron job", slog.Any("error", err))
		return err
	}

	s.logger.Info("Registered cron job: reconcileStock", slog.String("cron", s.stockReconcileCron))
	return nil
}

// forEachOrganization runs fn for every organization in scope and logs each failure with its
// organization, so one organization's outage never stops or hides the others' runs.
func (s *Scheduler) forEachOrganization(job string, scope service.OrganizationScope, fn func(ctx context.Context) error) {
	ctx := context.Background()
	failures, err := s.organizations.ForEach(ctx, scope, fn)
	if err != nil {
		s.logger.ErrorContext(ctx, "Cron job "+job+" could not list organizations", slog.Any("error", err))
		return
	}
	for _, failure := range failures {
		s.logger.ErrorContext(ctx, "Cron job "+job+" failed",
			slog.String("organization_id", failure.OrganizationID),
			slog.Any("error", failure.Err),
		)
	}
}

// reconcileStockJob reports products whose on-hand disagrees with the movements behind it.
// Report-only: nothing is corrected, because either number can be the wrong one and only a
// person can say which. A failure is logged and the run ends — the check is a detector, so
// its own outage must not take anything else down.
func (s *Scheduler) reconcileStockJob() {
	s.forEachOrganization("reconcileStock", service.AllActiveOrganizations, s.reconcileStock)
}

func (s *Scheduler) reconcileStock(ctx context.Context) error {
	report, err := s.stockReconciliation.Reconcile(ctx)
	if err != nil {
		return err
	}
	orgID, _ := tenant.OrgFrom(ctx)

	if len(report.Diverged) == 0 {
		s.logger.InfoContext(ctx, "Cron job reconcileStock completed",
			slog.String("organization_id", orgID),
			slog.Int("checked_products", report.CheckedProducts),
		)
		return nil
	}

	for _, divergence := range report.Diverged {
		s.logger.WarnContext(ctx, "Stock amount disagrees with its movement history",
			slog.String("organization_id", orgID),
			slog.String("product_id", divergence.ProductID),
			slog.Int("amount", divergence.Amount),
			slog.Int("ledger_sum", divergence.LedgerSum),
			slog.Int("delta", divergence.Delta),
		)
	}
	s.logger.WarnContext(ctx, "Cron job reconcileStock found divergences",
		slog.String("organization_id", orgID),
		slog.Int("checked_products", report.CheckedProducts),
		slog.Int("diverged_products", len(report.Diverged)),
	)
	return nil
}

func (s *Scheduler) submitPendingInvoicesJob() {
	s.logger.Info("Cron job started: submitPendingInvoices")
	s.forEachOrganization("submitPendingInvoices", service.EInvoicingOrganizations, s.invoiceSubmissionService.SubmitDue)
}

func (s *Scheduler) updateMissingDocumentURLsJob() {
	s.logger.Info("Cron job started: updateMissingDocumentURLs")
	s.forEachOrganization("updateMissingDocumentURLs", service.EInvoicingOrganizations, func(ctx context.Context) error {
		response, err := s.invoiceService.UpdateMissingDocumentURLs(ctx)
		if err != nil {
			return err
		}
		s.logDocumentURLUpdate(ctx, "updateMissingDocumentURLs", response)
		return nil
	})
}

func (s *Scheduler) updateMissingSupportDocumentURLsJob() {
	s.logger.Info("Cron job started: updateMissingSupportDocumentURLs")
	s.forEachOrganization("updateMissingSupportDocumentURLs", service.EInvoicingOrganizations, func(ctx context.Context) error {
		response, err := s.supportDocumentService.UpdateMissingDocumentURLs(ctx)
		if err != nil {
			return err
		}
		s.logDocumentURLUpdate(ctx, "updateMissingSupportDocumentURLs", response)
		return nil
	})
}

func (s *Scheduler) logDocumentURLUpdate(ctx context.Context, job string, response *dto.UpdateDocumentURLsResponse) {
	orgID, _ := tenant.OrgFrom(ctx)
	s.logger.InfoContext(ctx, "Cron job "+job+" completed",
		slog.String("organization_id", orgID),
		slog.Int("updated_count", response.UpdatedCount),
	)
	if len(response.FailedBills) > 0 {
		s.logger.WarnContext(ctx, "Cron job "+job+": some documents failed to update",
			slog.String("organization_id", orgID),
			slog.Int("failed_count", len(response.FailedBills)),
		)
	}
}
