package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	rule "laguna-escondida/backend/internal/domain/aggregate/notification_rule"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/eventconsumer"
	"laguna-escondida/backend/internal/domain/ports"
)

const StaffNotificationsConsumerName = "staff-notifications"

var staffEventTitles = map[string]string{
	dto.DomainEventOrderCreated:         "Nuevo pedido",
	dto.DomainEventOrderPaid:            "Pedido pagado",
	dto.DomainEventReservationRequested: "Reserva por confirmar",
	dto.DomainEventReservationConfirmed: "Reserva confirmada",
}

// StaffNotificationConsumer turns domain events into one notification row per matching rule; sending is separate.
type StaffNotificationConsumer struct {
	rules         ports.NotificationRuleRepository
	notifications ports.StaffNotificationRepository
	organizations ports.OrganizationRepository
	zones         ports.DeliveryZoneRepository
	now           func() time.Time
}

func NewStaffNotificationConsumer(
	rules ports.NotificationRuleRepository,
	notifications ports.StaffNotificationRepository,
	organizations ports.OrganizationRepository,
	zones ports.DeliveryZoneRepository,
	now func() time.Time,
) *StaffNotificationConsumer {
	if now == nil {
		now = time.Now
	}
	return &StaffNotificationConsumer{rules: rules, notifications: notifications, organizations: organizations, zones: zones, now: now}
}

func (c *StaffNotificationConsumer) Name() string { return StaffNotificationsConsumerName }

func (c *StaffNotificationConsumer) Handles() []string {
	return []string{
		dto.DomainEventOrderCreated, dto.DomainEventOrderPaid,
		dto.DomainEventReservationRequested, dto.DomainEventReservationConfirmed,
	}
}

func (c *StaffNotificationConsumer) Handle(ctx context.Context, event dto.StoredEvent) error {
	subject, err := notificationSubject(event)
	if err != nil {
		return err
	}
	candidates, err := c.rules.ListEnabledForEvent(ctx, subject.Event)
	if err != nil {
		return fmt.Errorf("list rules for %s: %w", event.Name, err)
	}
	var matching []*dto.RuleWithChannel
	for _, candidate := range candidates {
		if rule.NewAggregateFromDTO(&candidate.Rule).Matches(subject) {
			matching = append(matching, candidate)
		}
	}
	if len(matching) == 0 {
		return nil
	}

	org, err := c.organizations.Get(ctx, event.OrganizationID)
	if err != nil {
		return fmt.Errorf("read organization: %w", err)
	}
	loc, err := time.LoadLocation(org.Timezone)
	if err != nil {
		return fmt.Errorf("load timezone %q: %w", org.Timezone, err)
	}
	message, err := c.buildMessage(ctx, event, org.Currency, loc)
	if err != nil {
		return err
	}

	now := c.now()
	for _, m := range matching {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("generate notification id: %w", err)
		}
		withPrefix := message
		withPrefix.Prefix = m.Rule.TitlePrefix
		notification := &dto.StaffNotification{
			ID: id.String(), OrganizationID: event.OrganizationID, RuleID: m.Rule.ID, ChannelID: m.Channel.ID,
			EventID: event.ID, Message: withPrefix, Status: dto.StaffNotificationPending,
			NextAttemptAt: now, ExpiresAt: event.OccurredAt.Add(time.Duration(m.Rule.ExpiresAfterMinutes) * time.Minute),
			CreatedAt: now,
		}
		if _, err := c.notifications.Insert(ctx, notification); err != nil {
			return fmt.Errorf("insert notification for rule %s: %w", m.Rule.ID, err)
		}
	}
	return nil
}

func (c *StaffNotificationConsumer) buildMessage(ctx context.Context, event dto.StoredEvent, currency string, loc *time.Location) (dto.StaffMessage, error) {
	title := staffEventTitles[event.Name]
	switch event.Name {
	case dto.DomainEventOrderCreated:
		e, err := eventconsumer.Decode[dto.OrderCreated](event)
		if err != nil {
			return dto.StaffMessage{}, err
		}
		zone, err := c.zoneName(ctx, e.DeliveryZoneID)
		if err != nil {
			return dto.StaffMessage{}, err
		}
		return buildOrderMessage(title, staffOrderFacts{e.Subject, e.OrderType, e.Total, e.CustomerName, e.CustomerPhone, zone}, currency, e.Time, loc), nil
	case dto.DomainEventOrderPaid:
		e, err := eventconsumer.Decode[dto.OrderPaid](event)
		if err != nil {
			return dto.StaffMessage{}, err
		}
		zone, err := c.zoneName(ctx, e.DeliveryZoneID)
		if err != nil {
			return dto.StaffMessage{}, err
		}
		return buildOrderMessage(title, staffOrderFacts{e.Subject, e.OrderType, e.Total, e.CustomerName, e.CustomerPhone, zone}, currency, e.Time, loc), nil
	case dto.DomainEventReservationRequested:
		e, err := eventconsumer.Decode[dto.ReservationRequested](event)
		if err != nil {
			return dto.StaffMessage{}, err
		}
		return buildReservationMessage(title, staffReservationFacts{e.GuestName, e.PartySize, e.StartsAt, e.Source, e.PendingReason}, e.Time, loc), nil
	case dto.DomainEventReservationConfirmed:
		e, err := eventconsumer.Decode[dto.ReservationConfirmed](event)
		if err != nil {
			return dto.StaffMessage{}, err
		}
		return buildReservationMessage(title, staffReservationFacts{e.GuestName, e.PartySize, e.StartsAt, e.Source, e.PendingReason}, e.Time, loc), nil
	}
	return dto.StaffMessage{}, fmt.Errorf("staff notifications do not handle event %q", event.Name)
}

func (c *StaffNotificationConsumer) zoneName(ctx context.Context, zoneID *string) (string, error) {
	if zoneID == nil {
		return "", nil
	}
	zone, err := c.zones.FindByID(ctx, *zoneID)
	if errors.Is(err, domainError.ErrDeliveryZoneNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read delivery zone: %w", err)
	}
	return zone.Name, nil
}

func notificationSubject(event dto.StoredEvent) (dto.NotificationSubject, error) {
	switch event.Name {
	case dto.DomainEventOrderCreated:
		e, err := eventconsumer.Decode[dto.OrderCreated](event)
		if err != nil {
			return dto.NotificationSubject{}, err
		}
		return dto.NotificationSubject{Event: dto.NotificationEventOrderCreated, OrderType: &e.OrderType}, nil
	case dto.DomainEventOrderPaid:
		e, err := eventconsumer.Decode[dto.OrderPaid](event)
		if err != nil {
			return dto.NotificationSubject{}, err
		}
		return dto.NotificationSubject{Event: dto.NotificationEventOrderPaid, OrderType: &e.OrderType}, nil
	case dto.DomainEventReservationRequested:
		e, err := eventconsumer.Decode[dto.ReservationRequested](event)
		if err != nil {
			return dto.NotificationSubject{}, err
		}
		return dto.NotificationSubject{Event: dto.NotificationEventReservationRequested, Source: &e.Source, PartySize: &e.PartySize}, nil
	case dto.DomainEventReservationConfirmed:
		e, err := eventconsumer.Decode[dto.ReservationConfirmed](event)
		if err != nil {
			return dto.NotificationSubject{}, err
		}
		return dto.NotificationSubject{Event: dto.NotificationEventReservationConfirmed, Source: &e.Source, ConfirmedBy: &e.ConfirmedBy, PartySize: &e.PartySize}, nil
	}
	return dto.NotificationSubject{}, fmt.Errorf("staff notifications do not handle event %q", event.Name)
}
