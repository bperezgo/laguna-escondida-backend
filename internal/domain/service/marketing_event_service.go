package service

import (
	"context"
	"fmt"
	"time"

	marketingEvent "laguna-escondida/backend/internal/domain/aggregate/marketing_event"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/tenant"
)

// MarketingEventService records the events marketing-app acts on. It writes nothing for a subject
// without a channel (marketing-app never saw it), while the events URL is unset, or for an
// organization without a signing secret; the change it reports goes ahead either way.
type MarketingEventService struct {
	outbox        ports.MarketingEventOutbox
	organizations ports.OrganizationRepository
	secrets       ports.MarketingEventSecrets
	enabled       bool
}

func NewMarketingEventService(outbox ports.MarketingEventOutbox, organizations ports.OrganizationRepository, secrets ports.MarketingEventSecrets, enabled bool) *MarketingEventService {
	return &MarketingEventService{outbox: outbox, organizations: organizations, secrets: secrets, enabled: enabled}
}

func (s *MarketingEventService) RecordOrder(ctx context.Context, eventType dto.MarketingEventType, subject dto.MarketingOrderSubject) error {
	if subject.Origin == nil || subject.Origin.ChannelID == nil || subject.CustomerPhone == nil {
		return nil
	}
	org, loc, ok, err := s.organization(ctx)
	if err != nil || !ok {
		return err
	}
	event, err := marketingEvent.NewOrderEvent(eventType, subject, org.Currency, loc)
	if err != nil {
		return fmt.Errorf("%w: %w", domainError.ErrMarketingEventRecordError, err)
	}
	return s.append(ctx, event)
}

func (s *MarketingEventService) RecordReservation(ctx context.Context, reservation *dto.Reservation, at time.Time) error {
	eventType, reported := marketingEvent.ReservationEventType(reservation.Status)
	if !reported || reservation.Origin.ChannelID == nil {
		return nil
	}
	_, loc, ok, err := s.organization(ctx)
	if err != nil || !ok {
		return err
	}
	event, err := marketingEvent.NewReservationEvent(eventType, reservation, at, loc)
	if err != nil {
		return fmt.Errorf("%w: %w", domainError.ErrMarketingEventRecordError, err)
	}
	return s.append(ctx, event)
}

func (s *MarketingEventService) RecordFeedbackRequested(ctx context.Context, request *dto.FeedbackRequest, reservation *dto.Reservation) error {
	if reservation.Origin.ChannelID == nil {
		return nil
	}
	_, loc, ok, err := s.organization(ctx)
	if err != nil || !ok {
		return err
	}
	event, err := marketingEvent.NewFeedbackRequestedEvent(request, reservation, loc)
	if err != nil {
		return fmt.Errorf("%w: %w", domainError.ErrMarketingEventRecordError, err)
	}
	return s.append(ctx, event)
}

// organization reports ok=false when events are off for the context's organization.
func (s *MarketingEventService) organization(ctx context.Context) (*dto.Organization, *time.Location, bool, error) {
	if !s.enabled {
		return nil, nil, false, nil
	}
	orgID, err := tenant.RequireOrg(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	org, err := s.organizations.Get(ctx, orgID)
	if err != nil {
		return nil, nil, false, fmt.Errorf("%w: read organization: %w", domainError.ErrMarketingEventRecordError, err)
	}
	if _, ok := s.secrets.SecretFor(org.Slug); !ok {
		return nil, nil, false, nil
	}
	loc, err := time.LoadLocation(org.Timezone)
	if err != nil {
		loc = time.UTC
	}
	return org, loc, true, nil
}

func (s *MarketingEventService) append(ctx context.Context, event *dto.MarketingEvent) error {
	if err := s.outbox.Append(ctx, event); err != nil {
		return fmt.Errorf("%w: %w", domainError.ErrMarketingEventRecordError, err)
	}
	return nil
}
