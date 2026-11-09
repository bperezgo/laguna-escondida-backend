package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/reservation"
	"laguna-escondida/backend/internal/domain/aggregate/shared"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
)

type ReservationService struct {
	reservations    ports.ReservationRepository
	contacts        ports.ContactRecorder
	unitOfWork      ports.UnitOfWork
	settings        ports.OrganizationSettingsRepository
	events          ports.MarketingEventRecorder
	live            ports.ReservationNotifier
	phoneHomeRegion string
	logger          *slog.Logger
	now             func() time.Time
}

func NewReservationService(
	reservations ports.ReservationRepository,
	contacts ports.ContactRecorder,
	unitOfWork ports.UnitOfWork,
	settings ports.OrganizationSettingsRepository,
	events ports.MarketingEventRecorder,
	live ports.ReservationNotifier,
	phoneHomeRegion string,
	logger *slog.Logger,
	now func() time.Time,
) *ReservationService {
	if now == nil {
		now = time.Now
	}
	if live == nil {
		live = noReservationNotifier{}
	}
	return &ReservationService{
		reservations:    reservations,
		contacts:        contacts,
		unitOfWork:      unitOfWork,
		settings:        settings,
		events:          events,
		live:            live,
		phoneHomeRegion: phoneHomeRegion,
		logger:          logger,
		now:             now,
	}
}

// Create stores a WhatsApp reservation as requested and a staff-entered one as confirmed, and
// creates or updates the guest's contact in the same transaction.
func (s *ReservationService) Create(ctx context.Context, req *dto.CreateReservationRequest) (*dto.Reservation, error) {
	now := s.now()
	phone, err := shared.NewPhone(req.CustomerPhone, s.phoneHomeRegion)
	if err != nil {
		return nil, &domainError.ValidationError{Field: "customer_phone", Reason: "must be a valid phone number"}
	}
	origin, dropped, err := shared.NewOrigin(req.Origin, now)
	if err != nil {
		return nil, err
	}
	if len(dropped) > 0 {
		s.logger.WarnContext(ctx, "dropped invalid attribution fields from a reservation", slog.Any("fields", dropped))
	}
	if err = reservation.ValidateRequest(req, now); err != nil {
		return nil, err
	}

	var created *dto.Reservation
	err = s.unitOfWork.Do(ctx, func(ctx context.Context) error {
		name := req.Name
		contact, touchErr := s.contacts.Touch(ctx, dto.ContactTouch{Phone: phone.Value(), Name: &name, Attribution: origin.Attribution, At: now})
		if touchErr != nil {
			return fmt.Errorf("%w: %w", domainError.ErrReservationOperation, touchErr)
		}
		aggregate, buildErr := reservation.NewFromCreateRequest(req, contact.ID, phone.Value(), origin, now)
		if buildErr != nil {
			return buildErr
		}
		created = aggregate.ToDTO()
		if createErr := s.reservations.Create(ctx, created); createErr != nil {
			return fmt.Errorf("%w: create reservation: %w", domainError.ErrReservationOperation, createErr)
		}
		return s.events.RecordReservation(ctx, created, now)
	})
	if err != nil {
		return nil, err
	}
	s.live.ReservationChanged(ctx, created)
	return created, nil
}

func (s *ReservationService) Get(ctx context.Context, id string) (*dto.Reservation, error) {
	return s.reservations.FindByID(ctx, id)
}

func (s *ReservationService) Confirm(ctx context.Context, id string) (*dto.Reservation, error) {
	return s.transition(ctx, id, func(a *reservation.Aggregate, now time.Time) error { return a.Confirm(now) })
}

func (s *ReservationService) Decline(ctx context.Context, id string, reason *string) (*dto.Reservation, error) {
	return s.transition(ctx, id, func(a *reservation.Aggregate, now time.Time) error { return a.Decline(reason, now) })
}

func (s *ReservationService) CancelByStaff(ctx context.Context, id string) (*dto.Reservation, error) {
	return s.transition(ctx, id, func(a *reservation.Aggregate, now time.Time) error { return a.Cancel(dto.CancelledByStaff, now) })
}

func (s *ReservationService) MarkNoShow(ctx context.Context, id string) (*dto.Reservation, error) {
	return s.transition(ctx, id, func(a *reservation.Aggregate, now time.Time) error { return a.MarkNoShow(now) })
}

// CancelByCustomer answers "not found" for another phone's reservation and for one the guest can
// no longer cancel, so the agent learns nothing about ids that aren't the caller's.
func (s *ReservationService) CancelByCustomer(ctx context.Context, id, rawPhone string) (*dto.Reservation, error) {
	phone, err := shared.NewPhone(rawPhone, s.phoneHomeRegion)
	if err != nil {
		return nil, &domainError.ValidationError{Field: "customer_phone", Reason: "must be a valid phone number"}
	}
	result, err := s.transition(ctx, id, func(a *reservation.Aggregate, now time.Time) error {
		if a.ContactPhone() != phone.Value() || !a.CustomerMayCancel(now) {
			return domainError.ErrReservationNotFound
		}
		return a.Cancel(dto.CancelledByCustomer, now)
	})
	if errors.Is(err, domainError.ErrReservationTransition) {
		return nil, domainError.ErrReservationNotFound
	}
	return result, err
}

func (s *ReservationService) transition(ctx context.Context, id string, apply func(*reservation.Aggregate, time.Time) error) (*dto.Reservation, error) {
	now := s.now()
	var updated *dto.Reservation
	err := s.unitOfWork.Do(ctx, func(ctx context.Context) error {
		current, err := s.reservations.FindByID(ctx, id)
		if err != nil {
			return err
		}
		aggregate := reservation.FromDTO(current)
		if err := apply(aggregate, now); err != nil {
			return err
		}
		updated = aggregate.ToDTO()
		if err := s.reservations.Update(ctx, updated, current.Status); err != nil {
			if errors.Is(err, domainError.ErrReservationTransition) {
				return &domainError.ReservationTransitionError{From: string(current.Status), To: string(updated.Status)}
			}
			return fmt.Errorf("%w: update reservation: %w", domainError.ErrReservationOperation, err)
		}
		if !aggregate.GuestIsNotified(now) {
			return nil
		}
		return s.events.RecordReservation(ctx, updated, now)
	})
	if err != nil {
		return nil, err
	}
	s.live.ReservationChanged(ctx, updated)
	return updated, nil
}

// List reads a local day (default today), or a phone's reservations from an optional time on.
func (s *ReservationService) List(ctx context.Context, query dto.ReservationListQuery) (*dto.ReservationListResponse, error) {
	filter, err := s.listFilter(ctx, query)
	if err != nil {
		return nil, err
	}
	reservations, err := s.reservations.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("%w: list reservations: %w", domainError.ErrReservationOperation, err)
	}
	if reservations == nil {
		reservations = []*dto.Reservation{}
	}
	return &dto.ReservationListResponse{Reservations: reservations, Total: len(reservations)}, nil
}

func (s *ReservationService) listFilter(ctx context.Context, query dto.ReservationListQuery) (dto.ReservationListFilter, error) {
	var filter dto.ReservationListFilter
	if status := strings.TrimSpace(query.Status); status != "" {
		for _, part := range strings.Split(status, ",") {
			value := dto.ReservationStatus(strings.TrimSpace(part))
			if !isReservationStatus(value) {
				return filter, &domainError.ValidationError{Field: "status", Reason: "must be requested, confirmed, visited, declined, cancelled or no_show"}
			}
			filter.Statuses = append(filter.Statuses, value)
		}
	}

	hasDate := strings.TrimSpace(query.Date) != ""
	if rawFrom := strings.TrimSpace(query.From); rawFrom != "" {
		from, err := time.Parse(time.RFC3339, rawFrom)
		if err != nil {
			return filter, &domainError.ValidationError{Field: "from", Reason: "must be a date and time with offset (RFC 3339)"}
		}
		filter.From = &from
	}

	if raw := strings.TrimSpace(query.Phone); raw != "" {
		phone, err := shared.NewPhone(raw, s.phoneHomeRegion)
		if err != nil {
			return filter, &domainError.ValidationError{Field: "phone", Reason: "must be a valid phone number"}
		}
		filter.Phone = phone.Value()
		if !hasDate {
			return filter, nil
		}
	} else if filter.From != nil {
		if hasDate {
			return filter, &domainError.ValidationError{Field: "from", Reason: "cannot be combined with date"}
		}
		return filter, nil
	}

	loc, err := s.location(ctx)
	if err != nil {
		return filter, err
	}
	day := s.now().In(loc)
	if raw := strings.TrimSpace(query.Date); raw != "" {
		if day, err = time.ParseInLocation(time.DateOnly, raw, loc); err != nil {
			return filter, &domainError.ValidationError{Field: "date", Reason: "must be a date as YYYY-MM-DD"}
		}
	}
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc)
	end := start.AddDate(0, 0, 1)
	filter.From, filter.To = &start, &end
	return filter, nil
}

// MarkVisited moves every confirmed reservation two hours past its start to visited.
func (s *ReservationService) MarkVisited(ctx context.Context) (int, error) {
	now := s.now()
	due, err := s.reservations.ListConfirmedStartedBefore(ctx, now.Add(-reservation.VisitedAfter))
	if err != nil {
		return 0, fmt.Errorf("%w: list confirmed reservations: %w", domainError.ErrReservationOperation, err)
	}
	marked := 0
	for _, current := range due {
		aggregate := reservation.FromDTO(current)
		if err := aggregate.MarkVisited(now); err != nil {
			continue
		}
		visited := aggregate.ToDTO()
		err := s.reservations.Update(ctx, visited, current.Status)
		if errors.Is(err, domainError.ErrReservationTransition) {
			// Staff changed it since the list was read; their change stands.
			continue
		}
		if err != nil {
			return marked, fmt.Errorf("%w: mark reservation %s visited: %w", domainError.ErrReservationOperation, current.ID, err)
		}
		s.live.ReservationChanged(ctx, visited)
		marked++
	}
	return marked, nil
}

func (s *ReservationService) location(ctx context.Context) (*time.Location, error) {
	view, err := s.settings.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: read organization timezone: %w", domainError.ErrReservationOperation, err)
	}
	loc, err := time.LoadLocation(view.Timezone)
	if err != nil {
		return nil, fmt.Errorf("%w: load organization timezone %q: %w", domainError.ErrReservationOperation, view.Timezone, err)
	}
	return loc, nil
}

type noReservationNotifier struct{}

func (noReservationNotifier) ReservationChanged(context.Context, *dto.Reservation) {}

func isReservationStatus(status dto.ReservationStatus) bool {
	switch status {
	case dto.ReservationStatusRequested, dto.ReservationStatusConfirmed, dto.ReservationStatusVisited,
		dto.ReservationStatusDeclined, dto.ReservationStatusCancelled, dto.ReservationStatusNoShow:
		return true
	}
	return false
}
