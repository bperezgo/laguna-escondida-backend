package cron

import (
	"context"
	"log/slog"

	"laguna-escondida/backend/internal/domain/service"

	"github.com/go-co-op/gocron/v2"
)

type Scheduler struct {
	scheduler                gocron.Scheduler
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

func NewScheduler(
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

// reconcileStockJob reports products whose on-hand disagrees with the movements behind it.
// Report-only: nothing is corrected, because either number can be the wrong one and only a
// person can say which. A failure is logged and the run ends — the check is a detector, so
// its own outage must not take anything else down.
func (s *Scheduler) reconcileStockJob() {
	ctx := context.Background()
	report, err := s.stockReconciliation.Reconcile(ctx)
	if err != nil {
		s.logger.ErrorContext(ctx, "Cron job reconcileStock failed", slog.Any("error", err))
		return
	}

	if len(report.Diverged) == 0 {
		s.logger.InfoContext(ctx, "Cron job reconcileStock completed",
			slog.Int("checked_products", report.CheckedProducts),
		)
		return
	}

	for _, divergence := range report.Diverged {
		s.logger.WarnContext(ctx, "Stock amount disagrees with its movement history",
			slog.String("product_id", divergence.ProductID),
			slog.Int("amount", divergence.Amount),
			slog.Int("ledger_sum", divergence.LedgerSum),
			slog.Int("delta", divergence.Delta),
		)
	}
	s.logger.WarnContext(ctx, "Cron job reconcileStock found divergences",
		slog.Int("checked_products", report.CheckedProducts),
		slog.Int("diverged_products", len(report.Diverged)),
	)
}

func (s *Scheduler) submitPendingInvoicesJob() {
	s.logger.Info("Cron job started: submitPendingInvoices")

	ctx := context.Background()
	if err := s.invoiceSubmissionService.SubmitDue(ctx); err != nil {
		s.logger.ErrorContext(ctx, "Cron job submitPendingInvoices failed", slog.Any("error", err))
		return
	}

	s.logger.InfoContext(ctx, "Cron job submitPendingInvoices completed")
}

func (s *Scheduler) updateMissingDocumentURLsJob() {
	s.logger.Info("Cron job started: updateMissingDocumentURLs")

	ctx := context.Background()
	response, err := s.invoiceService.UpdateMissingDocumentURLs(ctx)
	if err != nil {
		s.logger.ErrorContext(ctx, "Cron job updateMissingDocumentURLs failed", slog.Any("error", err))
		return
	}

	s.logger.InfoContext(ctx, "Cron job updateMissingDocumentURLs completed",
		slog.Int("updated_count", response.UpdatedCount),
	)
	if len(response.FailedBills) > 0 {
		s.logger.WarnContext(ctx, "Cron job updateMissingDocumentURLs: some bills failed to update",
			slog.Int("failed_count", len(response.FailedBills)),
		)
	}
}

func (s *Scheduler) updateMissingSupportDocumentURLsJob() {
	s.logger.Info("Cron job started: updateMissingSupportDocumentURLs")

	ctx := context.Background()
	response, err := s.supportDocumentService.UpdateMissingDocumentURLs(ctx)
	if err != nil {
		s.logger.ErrorContext(ctx, "Cron job updateMissingSupportDocumentURLs failed", slog.Any("error", err))
		return
	}

	s.logger.InfoContext(ctx, "Cron job updateMissingSupportDocumentURLs completed",
		slog.Int("updated_count", response.UpdatedCount),
	)
	if len(response.FailedBills) > 0 {
		s.logger.WarnContext(ctx, "Cron job updateMissingSupportDocumentURLs: some documents failed to update",
			slog.Int("failed_count", len(response.FailedBills)),
		)
	}
}
