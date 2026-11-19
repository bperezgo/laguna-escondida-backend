package cron

import (
	"context"
	"log/slog"
	"time"

	"github.com/go-co-op/gocron/v2"
)

const leadAlertInterval = 15 * time.Second

type leadAlertDeliverer interface {
	DeliverDue(ctx context.Context) (int, error)
}

// LeadAlertScheduler sends the operator's lead alerts. Leads belong to no organization, so unlike
// the event relay this is one job for the platform, not one per organization.
type LeadAlertScheduler struct {
	scheduler gocron.Scheduler
	alerts    leadAlertDeliverer
	logger    *slog.Logger
	interval  time.Duration
}

func NewLeadAlertScheduler(alerts leadAlertDeliverer, logger *slog.Logger) (*LeadAlertScheduler, error) {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return nil, err
	}
	return &LeadAlertScheduler{scheduler: scheduler, alerts: alerts, logger: logger, interval: leadAlertInterval}, nil
}

func (s *LeadAlertScheduler) Start() error {
	if _, err := s.scheduler.NewJob(gocron.DurationJob(s.interval), gocron.NewTask(s.deliverJob),
		gocron.WithSingletonMode(gocron.LimitModeReschedule)); err != nil {
		s.logger.Error("Failed to register deliverLeadAlerts job", slog.Any("error", err))
		return err
	}
	s.logger.Info("Registered job: deliverLeadAlerts", slog.Duration("every", s.interval))
	s.scheduler.Start()
	return nil
}

// Stop waits for a running delivery, so an alert in flight settles before the process exits.
func (s *LeadAlertScheduler) Stop() error {
	s.logger.Info("Stopping lead alert scheduler...")
	return s.scheduler.Shutdown()
}

func (s *LeadAlertScheduler) deliverJob() {
	ctx := context.Background()
	if _, err := s.alerts.DeliverDue(ctx); err != nil {
		s.logger.ErrorContext(ctx, "Cron job deliverLeadAlerts failed", slog.Any("error", err))
	}
}
