package cron

import (
	"context"

	"laguna-escondida/backend/internal/domain/service"

	"github.com/go-co-op/gocron/v2"
	"go.uber.org/zap"
)

type Scheduler struct {
	scheduler      gocron.Scheduler
	invoiceService *service.InvoiceService
	logger         *zap.Logger
}

func NewScheduler(invoiceService *service.InvoiceService, logger *zap.Logger) (*Scheduler, error) {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return nil, err
	}

	return &Scheduler{
		scheduler:      scheduler,
		invoiceService: invoiceService,
		logger:         logger,
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
		gocron.CronJob("0 * * * *", false),
		gocron.NewTask(s.updateMissingDocumentURLsJob),
	)
	if err != nil {
		s.logger.Error("Failed to register updateMissingDocumentURLs cron job", zap.Error(err))
		return err
	}

	s.logger.Info("Registered cron job: updateMissingDocumentURLs (runs every hour at minute 0)")
	return nil
}

func (s *Scheduler) updateMissingDocumentURLsJob() {
	s.logger.Info("Cron job started: updateMissingDocumentURLs")

	ctx := context.Background()
	response, err := s.invoiceService.UpdateMissingDocumentURLs(ctx)
	if err != nil {
		s.logger.Error("Cron job updateMissingDocumentURLs failed", zap.Error(err))
		return
	}

	s.logger.Info("Cron job updateMissingDocumentURLs completed",
		zap.Int("updated_count", response.UpdatedCount),
	)
	if len(response.FailedBills) > 0 {
		s.logger.Warn("Cron job updateMissingDocumentURLs: some bills failed to update",
			zap.Int("failed_count", len(response.FailedBills)),
		)
	}
}
