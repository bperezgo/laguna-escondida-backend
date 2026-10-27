package service

import (
	"context"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/ordering"
	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/ports"
)

type OrganizationSettingsService struct {
	repo ports.OrganizationSettingsRepository
	now  func() time.Time
}

func NewOrganizationSettingsService(repo ports.OrganizationSettingsRepository) *OrganizationSettingsService {
	return &OrganizationSettingsService{repo: repo, now: time.Now}
}

func (s *OrganizationSettingsService) Get(ctx context.Context) (*dto.OrganizationSettingsResponse, error) {
	view, err := s.repo.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("get organization settings: %w", err)
	}
	return s.respond(view)
}

// Update merges under the repository's row lock, so a pause and a schedule edit saved together
// don't overwrite each other.
func (s *OrganizationSettingsService) Update(ctx context.Context, req *dto.UpdateOrganizationSettingsRequest) (*dto.OrganizationSettingsResponse, error) {
	saved, err := s.repo.UpdateOrdering(ctx, func(current dto.OrderingSettings) (dto.OrderingSettings, error) {
		return ordering.ApplyPatch(current, req.Ordering)
	})
	if err != nil {
		return nil, fmt.Errorf("update ordering settings: %w", err)
	}
	return s.respond(saved)
}

// RequiredPermissions is what the patch's fields need beyond reading: the pause and the hours are granted apart.
func (s *OrganizationSettingsService) RequiredPermissions(req *dto.UpdateOrganizationSettingsRequest) []permissions.Permission {
	var required []permissions.Permission
	if ordering.TouchesPause(req.Ordering) {
		required = append(required, permissions.OrderingPause)
	}
	if ordering.TouchesSchedule(req.Ordering) {
		required = append(required, permissions.OrderingManage)
	}
	return required
}

func (s *OrganizationSettingsService) respond(view *dto.OrganizationSettingsView) (*dto.OrganizationSettingsResponse, error) {
	settings := ordering.Normalize(view.Ordering)
	availability, err := EvaluateOrderingAvailability(settings, view.Timezone, s.now())
	if err != nil {
		return nil, err
	}
	return &dto.OrganizationSettingsResponse{Ordering: settings, Availability: availability, Timezone: view.Timezone}, nil
}

// EvaluateOrderingAvailability is the one place availability is computed, for the settings read and the order gate.
func EvaluateOrderingAvailability(settings dto.OrderingSettings, timezone string, now time.Time) (dto.OrderingAvailability, error) {
	loc := time.UTC
	// Only a schedule needs the zone, so an unconfigured box never depends on its tz database.
	if len(settings.Delivery.Schedule) > 0 || len(settings.Takeaway.Schedule) > 0 {
		var err error
		if loc, err = time.LoadLocation(timezone); err != nil {
			return dto.OrderingAvailability{}, fmt.Errorf("load organization timezone %q: %w", timezone, err)
		}
	}
	return dto.OrderingAvailability{
		Delivery: ordering.Evaluate(settings.Delivery, loc, now),
		Takeaway: ordering.Evaluate(settings.Takeaway, loc, now),
	}, nil
}
