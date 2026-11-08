package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/feedback"
	"laguna-escondida/backend/internal/domain/aggregate/shared"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
)

type FeedbackService struct {
	feedback        ports.FeedbackRepository
	reservations    ports.ReservationRepository
	unitOfWork      ports.UnitOfWork
	settings        ports.OrganizationSettingsRepository
	events          ports.MarketingEventRecorder
	phoneHomeRegion string
	now             func() time.Time
}

func NewFeedbackService(
	feedbackRepo ports.FeedbackRepository,
	reservations ports.ReservationRepository,
	unitOfWork ports.UnitOfWork,
	settings ports.OrganizationSettingsRepository,
	events ports.MarketingEventRecorder,
	phoneHomeRegion string,
	now func() time.Time,
) *FeedbackService {
	if now == nil {
		now = time.Now
	}
	return &FeedbackService{
		feedback:        feedbackRepo,
		reservations:    reservations,
		unitOfWork:      unitOfWork,
		settings:        settings,
		events:          events,
		phoneHomeRegion: phoneHomeRegion,
		now:             now,
	}
}

// RequestDue asks for feedback on every recent channel visit whose morning-after has come, each
// request committed with its feedback.requested event. It returns how many it opened.
func (s *FeedbackService) RequestDue(ctx context.Context) (int, error) {
	now := s.now()
	view, err := s.settings.Get(ctx)
	if err != nil {
		return 0, fmt.Errorf("%w: read organization timezone: %w", domainError.ErrFeedbackOperation, err)
	}
	loc, err := time.LoadLocation(view.Timezone)
	if err != nil {
		return 0, fmt.Errorf("%w: load organization timezone %q: %w", domainError.ErrFeedbackOperation, view.Timezone, err)
	}
	candidates, err := s.reservations.ListAwaitingFeedback(ctx, now.Add(-feedback.MaxVisitAge))
	if err != nil {
		return 0, fmt.Errorf("%w: list visits awaiting feedback: %w", domainError.ErrFeedbackOperation, err)
	}

	requested := 0
	for _, visit := range candidates {
		if visit.Status != dto.ReservationStatusVisited || visit.Origin.ChannelID == nil || !feedback.IsDue(visit, loc, now) {
			continue
		}
		request := feedback.NewRequest(visit, now)
		err := s.unitOfWork.Do(ctx, func(ctx context.Context) error {
			if err := s.feedback.CreateRequest(ctx, request); err != nil {
				return err
			}
			return s.events.RecordFeedbackRequested(ctx, request, visit)
		})
		if errors.Is(err, domainError.ErrFeedbackRequestExists) {
			continue
		}
		if err != nil {
			return requested, fmt.Errorf("%w: request feedback for reservation %s: %w", domainError.ErrFeedbackOperation, visit.ID, err)
		}
		requested++
	}
	return requested, nil
}

// Record attaches the guest's rating to their latest open request, replacing an earlier answer.
func (s *FeedbackService) Record(ctx context.Context, req *dto.RecordFeedbackRequest) (*dto.RecordFeedbackResponse, error) {
	now := s.now()
	phone, err := shared.NewPhone(req.CustomerPhone, s.phoneHomeRegion)
	if err != nil {
		return nil, &domainError.ValidationError{Field: "customer_phone", Reason: "must be a valid phone number"}
	}
	request, err := s.feedback.FindLatestOpenRequest(ctx, phone.Value(), now)
	if err != nil {
		return nil, err
	}
	answer, err := feedback.New(request.ID, req.Rating, req.Comment, now)
	if err != nil {
		return nil, err
	}
	saved, err := s.feedback.Save(ctx, answer)
	if err != nil {
		return nil, fmt.Errorf("%w: save feedback: %w", domainError.ErrFeedbackOperation, err)
	}
	view, err := s.settings.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: read feedback settings: %w", domainError.ErrFeedbackOperation, err)
	}
	return &dto.RecordFeedbackResponse{
		Feedback:  saved,
		Escalate:  saved.NeedsAttention,
		ReviewURL: feedback.ReviewLink(view.Feedback),
	}, nil
}

// List reads feedback recorded between two local dates, both inclusive; neither means this week.
func (s *FeedbackService) List(ctx context.Context, query dto.FeedbackListQuery) (*dto.FeedbackListResponse, error) {
	view, err := s.settings.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: read organization timezone: %w", domainError.ErrFeedbackOperation, err)
	}
	loc, err := time.LoadLocation(view.Timezone)
	if err != nil {
		return nil, fmt.Errorf("%w: load organization timezone %q: %w", domainError.ErrFeedbackOperation, view.Timezone, err)
	}
	from, err := localDate(query.From, "from", loc, startOfWeek(s.now().In(loc)))
	if err != nil {
		return nil, err
	}
	to, err := localDate(query.To, "to", loc, from.AddDate(0, 0, 6))
	if err != nil {
		return nil, err
	}
	end := to.AddDate(0, 0, 1)
	if !end.After(from) {
		return nil, &domainError.ValidationError{Field: "to", Reason: "must not be before from"}
	}
	items, err := s.feedback.List(ctx, dto.FeedbackListFilter{From: &from, To: &end, NeedsAttention: query.NeedsAttention})
	if err != nil {
		return nil, fmt.Errorf("%w: list feedback: %w", domainError.ErrFeedbackOperation, err)
	}
	if items == nil {
		items = []*dto.Feedback{}
	}
	return &dto.FeedbackListResponse{Feedback: items, Total: len(items)}, nil
}

func localDate(raw, field string, loc *time.Location, fallback time.Time) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	day, err := time.ParseInLocation(time.DateOnly, raw, loc)
	if err != nil {
		return time.Time{}, &domainError.ValidationError{Field: field, Reason: "must be a date as YYYY-MM-DD"}
	}
	return day, nil
}

// startOfWeek is Monday 00:00 of t's week, in t's location.
func startOfWeek(t time.Time) time.Time {
	offset := (int(t.Weekday()) + 6) % 7
	return time.Date(t.Year(), t.Month(), t.Day()-offset, 0, 0, 0, 0, t.Location())
}
