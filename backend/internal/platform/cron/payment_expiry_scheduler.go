package cron

import (
	"context"
	"log/slog"
	"time"

	"github.com/go-co-op/gocron/v2"
)

// paymentExpiryInterval: well under a minute, so an unpaid order is gone soon after its link expires.
const paymentExpiryInterval = 15 * time.Second

type paymentExpirer interface {
	ExpireDue(ctx context.Context) error
}

// PaymentExpiryScheduler voids agent orders whose payment link expired unpaid, every organization each run.
type PaymentExpiryScheduler struct {
	scheduler gocron.Scheduler
	expiry    paymentExpirer
	logger    *slog.Logger
	interval  time.Duration
}

func NewPaymentExpiryScheduler(expiry paymentExpirer, logger *slog.Logger) (*PaymentExpiryScheduler, error) {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return nil, err
	}
	return &PaymentExpiryScheduler{scheduler: scheduler, expiry: expiry, logger: logger, interval: paymentExpiryInterval}, nil
}

func (s *PaymentExpiryScheduler) Start() error {
	if _, err := s.scheduler.NewJob(gocron.DurationJob(s.interval), gocron.NewTask(s.expireJob),
		gocron.WithSingletonMode(gocron.LimitModeReschedule)); err != nil {
		s.logger.Error("Failed to register expireUnpaidOrders job", slog.Any("error", err))
		return err
	}
	s.logger.Info("Registered job: expireUnpaidOrders", slog.Duration("every", s.interval))
	s.scheduler.Start()
	return nil
}

// Stop waits for a running expiry, so a void in flight commits before the process exits.
func (s *PaymentExpiryScheduler) Stop() error {
	s.logger.Info("Stopping payment expiry scheduler...")
	return s.scheduler.Shutdown()
}

func (s *PaymentExpiryScheduler) expireJob() {
	ctx := context.Background()
	if err := s.expiry.ExpireDue(ctx); err != nil {
		s.logger.ErrorContext(ctx, "Cron job expireUnpaidOrders failed", slog.Any("error", err))
	}
}
