package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/eventconsumer"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/tenant"
)

const (
	eventRelayBatch      = 100
	eventRelayLease      = time.Minute
	eventRelayFirstRetry = time.Second
	eventRelayMaxRetry   = 5 * time.Minute
	eventRelayMaxAttempt = 20
)

// EventRelayService hands one organization's due deliveries to their consumers. Delivery is at
// least once, per consumer, in no particular order.
type EventRelayService struct {
	store     ports.EventDeliveryStore
	consumers *eventconsumer.Registry
	uow       ports.UnitOfWork
	logger    *slog.Logger
	now       func() time.Time
}

func NewEventRelayService(
	store ports.EventDeliveryStore,
	consumers *eventconsumer.Registry,
	uow ports.UnitOfWork,
	logger *slog.Logger,
	now func() time.Time,
) *EventRelayService {
	if now == nil {
		now = time.Now
	}
	return &EventRelayService{store: store, consumers: consumers, uow: uow, logger: logger, now: now}
}

// DispatchDue handles the context's organization's due deliveries and returns how many completed.
func (s *EventRelayService) DispatchDue(ctx context.Context) (int, error) {
	if _, err := tenant.RequireOrg(ctx); err != nil {
		return 0, err
	}
	now := s.now()
	claimed, err := s.store.ClaimDue(ctx, now, now.Add(eventRelayLease), eventRelayBatch)
	if err != nil {
		return 0, fmt.Errorf("claim due event deliveries: %w", err)
	}
	done := 0
	for _, delivery := range claimed {
		ok, err := s.dispatch(ctx, delivery)
		if err != nil {
			return done, err
		}
		if ok {
			done++
		}
	}
	return done, nil
}

func (s *EventRelayService) dispatch(ctx context.Context, d *dto.ClaimedDelivery) (bool, error) {
	log := s.logger.With(
		slog.String("event_id", d.Event.ID), slog.String("event", d.Event.Name),
		slog.String("consumer", d.Consumer), slog.Int("attempts", d.Attempts))

	consumer, ok := s.consumers.Get(d.Consumer)
	if !ok {
		log.ErrorContext(ctx, "event delivery names an unknown consumer; marking it dead")
		return false, s.store.MarkFailed(ctx, d.Event.ID, d.Consumer, dto.EventDeliveryDead, s.now(), "unknown consumer")
	}

	handleErr := s.uow.Do(tenant.WithOrg(ctx, d.Event.OrganizationID), func(ctx context.Context) error {
		return consumer.Handle(ctx, d.Event)
	})
	now := s.now()
	if handleErr == nil {
		return true, s.store.MarkDone(ctx, d.Event.ID, d.Consumer, now)
	}

	if d.Attempts >= eventRelayMaxAttempt {
		log.ErrorContext(ctx, "event delivery failed for good; marking it dead", slog.Any("error", handleErr))
		return false, s.store.MarkFailed(ctx, d.Event.ID, d.Consumer, dto.EventDeliveryDead, now, handleErr.Error())
	}
	log.WarnContext(ctx, "event delivery failed; retrying", slog.Any("error", handleErr))
	return false, s.store.MarkFailed(ctx, d.Event.ID, d.Consumer, dto.EventDeliveryPending, now.Add(eventRelayRetryDelay(d.Attempts)), handleErr.Error())
}

// eventRelayRetryDelay doubles from 1s per attempt made, capped at 5 minutes.
func eventRelayRetryDelay(attempts int) time.Duration {
	delay := eventRelayFirstRetry
	for i := 1; i < attempts && delay < eventRelayMaxRetry; i++ {
		delay *= 2
	}
	if delay > eventRelayMaxRetry {
		return eventRelayMaxRetry
	}
	return delay
}
