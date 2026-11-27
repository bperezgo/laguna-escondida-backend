package cron

import (
	"context"
	"log/slog"
	"time"

	"github.com/go-co-op/gocron/v2"
)

const (
	// chargeDueInterval keeps a charge well within the hour the spec allows after a due date.
	chargeDueInterval = 10 * time.Minute
	// chargeReconcileInterval: a charge is looked up from ten minutes old, so within fifteen it is settled.
	chargeReconcileInterval = 5 * time.Minute
)

type subscriptionCharger interface {
	ChargeDue(ctx context.Context) error
	Reconcile(ctx context.Context) error
}

// ChargeScheduler charges due subscriptions and looks up the charges with no answer. Every replica
// runs it: the billing row's lock, not this scheduler, keeps a charge from being made twice.
type ChargeScheduler struct {
	scheduler         gocron.Scheduler
	charges           subscriptionCharger
	logger            *slog.Logger
	chargeInterval    time.Duration
	reconcileInterval time.Duration
}

func NewChargeScheduler(charges subscriptionCharger, logger *slog.Logger) (*ChargeScheduler, error) {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return nil, err
	}
	return &ChargeScheduler{scheduler: scheduler, charges: charges, logger: logger,
		chargeInterval: chargeDueInterval, reconcileInterval: chargeReconcileInterval}, nil
}

func (s *ChargeScheduler) Start() error {
	jobs := []struct {
		name     string
		interval time.Duration
		run      func()
	}{
		{"chargeDueSubscriptions", s.chargeInterval, s.chargeDueJob},
		{"reconcileSubscriptionCharges", s.reconcileInterval, s.reconcileJob},
	}
	for _, job := range jobs {
		if _, err := s.scheduler.NewJob(gocron.DurationJob(job.interval), gocron.NewTask(job.run),
			gocron.WithSingletonMode(gocron.LimitModeReschedule), gocron.WithStartAt(gocron.WithStartImmediately())); err != nil {
			s.logger.Error("Failed to register job", slog.String("job", job.name), slog.Any("error", err))
			return err
		}
		s.logger.Info("Registered job: "+job.name, slog.Duration("every", job.interval))
	}
	s.scheduler.Start()
	return nil
}

// Stop waits for a running job, so a charge in flight is stored before the process exits.
func (s *ChargeScheduler) Stop() error {
	s.logger.Info("Stopping charge scheduler...")
	return s.scheduler.Shutdown()
}

func (s *ChargeScheduler) chargeDueJob() {
	ctx := context.Background()
	if err := s.charges.ChargeDue(ctx); err != nil {
		s.logger.ErrorContext(ctx, "Cron job chargeDueSubscriptions failed", slog.Any("error", err))
	}
}

func (s *ChargeScheduler) reconcileJob() {
	ctx := context.Background()
	if err := s.charges.Reconcile(ctx); err != nil {
		s.logger.ErrorContext(ctx, "Cron job reconcileSubscriptionCharges failed", slog.Any("error", err))
	}
}
