package cron

import (
	"context"
	"log/slog"
	"time"

	"laguna-escondida/backend/internal/domain/service"

	"github.com/go-co-op/gocron/v2"
)

const (
	eventRelayInterval        = time.Second
	staffNotificationInterval = 5 * time.Second
)

// EventRelayScheduler runs the durable event relay once per organization. It doesn't depend on any
// consumer's feature being on.
type EventRelayScheduler struct {
	scheduler gocron.Scheduler
	runner    *service.OrganizationRunner
	relay     *service.EventRelayService
	staff     *service.StaffNotificationDeliveryService
	logger    *slog.Logger
}

func NewEventRelayScheduler(runner *service.OrganizationRunner, relay *service.EventRelayService, logger *slog.Logger) (*EventRelayScheduler, error) {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return nil, err
	}
	return &EventRelayScheduler{scheduler: scheduler, runner: runner, relay: relay, logger: logger}, nil
}

// WithStaffNotifications adds the staff notification delivery job; without it only the relay runs.
func (s *EventRelayScheduler) WithStaffNotifications(delivery *service.StaffNotificationDeliveryService) *EventRelayScheduler {
	s.staff = delivery
	return s
}

func (s *EventRelayScheduler) Start() error {
	if _, err := s.scheduler.NewJob(gocron.DurationJob(eventRelayInterval), gocron.NewTask(s.dispatchJob),
		gocron.WithSingletonMode(gocron.LimitModeReschedule)); err != nil {
		s.logger.Error("Failed to register dispatchDomainEvents job", slog.Any("error", err))
		return err
	}
	s.logger.Info("Registered job: dispatchDomainEvents", slog.Duration("every", eventRelayInterval))
	if s.staff != nil {
		if _, err := s.scheduler.NewJob(gocron.DurationJob(staffNotificationInterval), gocron.NewTask(s.deliverStaffNotificationsJob),
			gocron.WithSingletonMode(gocron.LimitModeReschedule)); err != nil {
			s.logger.Error("Failed to register deliverStaffNotifications job", slog.Any("error", err))
			return err
		}
		s.logger.Info("Registered job: deliverStaffNotifications", slog.Duration("every", staffNotificationInterval))
	}
	s.scheduler.Start()
	return nil
}

// Stop waits for running jobs, so a delivery in flight settles before the process exits.
func (s *EventRelayScheduler) Stop() error {
	s.logger.Info("Stopping event relay scheduler...")
	return s.scheduler.Shutdown()
}

func (s *EventRelayScheduler) dispatchJob() {
	s.forEachOrganization("dispatchDomainEvents", func(ctx context.Context) error {
		_, err := s.relay.DispatchDue(ctx)
		return err
	})
}

func (s *EventRelayScheduler) deliverStaffNotificationsJob() {
	s.forEachOrganization("deliverStaffNotifications", func(ctx context.Context) error {
		_, err := s.staff.DeliverDue(ctx)
		return err
	})
}

func (s *EventRelayScheduler) forEachOrganization(job string, fn func(ctx context.Context) error) {
	ctx := context.Background()
	failures, err := s.runner.ForEach(ctx, service.AllActiveOrganizations, fn)
	if err != nil {
		s.logger.ErrorContext(ctx, "Cron job "+job+" could not list organizations", slog.Any("error", err))
		return
	}
	for _, failure := range failures {
		s.logger.ErrorContext(ctx, "Cron job "+job+" failed",
			slog.String("organization_id", failure.OrganizationID), slog.Any("error", failure.Err))
	}
}
