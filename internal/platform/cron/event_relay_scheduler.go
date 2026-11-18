package cron

import (
	"context"
	"log/slog"
	"time"

	"laguna-escondida/backend/internal/domain/service"

	"github.com/go-co-op/gocron/v2"
)

const eventRelayInterval = time.Second

// EventRelayScheduler runs the durable event relay once per organization. It doesn't depend on any
// consumer's feature being on.
type EventRelayScheduler struct {
	scheduler gocron.Scheduler
	runner    *service.OrganizationRunner
	relay     *service.EventRelayService
	logger    *slog.Logger
}

func NewEventRelayScheduler(runner *service.OrganizationRunner, relay *service.EventRelayService, logger *slog.Logger) (*EventRelayScheduler, error) {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return nil, err
	}
	return &EventRelayScheduler{scheduler: scheduler, runner: runner, relay: relay, logger: logger}, nil
}

func (s *EventRelayScheduler) Start() error {
	if _, err := s.scheduler.NewJob(gocron.DurationJob(eventRelayInterval), gocron.NewTask(s.dispatchJob),
		gocron.WithSingletonMode(gocron.LimitModeReschedule)); err != nil {
		s.logger.Error("Failed to register dispatchDomainEvents job", slog.Any("error", err))
		return err
	}
	s.logger.Info("Registered job: dispatchDomainEvents", slog.Duration("every", eventRelayInterval))
	s.scheduler.Start()
	return nil
}

// Stop waits for running jobs, so a delivery in flight settles before the process exits.
func (s *EventRelayScheduler) Stop() error {
	s.logger.Info("Stopping event relay scheduler...")
	return s.scheduler.Shutdown()
}

func (s *EventRelayScheduler) dispatchJob() {
	ctx := context.Background()
	failures, err := s.runner.ForEach(ctx, service.AllActiveOrganizations, func(ctx context.Context) error {
		_, err := s.relay.DispatchDue(ctx)
		return err
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "Cron job dispatchDomainEvents could not list organizations", slog.Any("error", err))
		return
	}
	for _, failure := range failures {
		s.logger.ErrorContext(ctx, "Cron job dispatchDomainEvents failed",
			slog.String("organization_id", failure.OrganizationID), slog.Any("error", failure.Err))
	}
}
