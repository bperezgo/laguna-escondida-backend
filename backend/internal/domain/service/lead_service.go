package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/lead"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
)

const (
	leadDuplicateWindow = 24 * time.Hour
	leadCapWindow       = time.Hour
	leadCapPerWindow    = 300
	leadDefaultPageSize = 20
	leadMaxPageSize     = 100

	// LeadCapRetryAfter is what a caller is told to wait when the global cap is hit.
	LeadCapRetryAfter = 5 * time.Minute
)

type LeadService struct {
	repo          ports.LeadRepository
	alertsEnabled bool
	logger        *slog.Logger
	now           func() time.Time
}

// NewLeadService: alertsEnabled decides whether a new lead is queued for an operator alert.
func NewLeadService(repo ports.LeadRepository, alertsEnabled bool, logger *slog.Logger, now func() time.Time) *LeadService {
	if now == nil {
		now = time.Now
	}
	return &LeadService{repo: repo, alertsEnabled: alertsEnabled, logger: logger, now: now}
}

// Submit stores a public lead. A honeypot hit or a repeated phone answers success and stores nothing,
// so a bot learns nothing and a prospect who double-clicked isn't told off.
func (s *LeadService) Submit(ctx context.Context, req *dto.SubmitLeadRequest) error {
	if strings.TrimSpace(req.Website) != "" {
		return nil
	}
	now := s.now().UTC()
	l, dropped, err := lead.NewFromSubmission(req, now)
	if err != nil {
		return err
	}
	if len(dropped) > 0 {
		s.logger.WarnContext(ctx, "lead attribution fields dropped", slog.Any("fields", dropped))
	}

	recent, err := s.repo.FindRecentByPhone(ctx, l.Phone, now.Add(-leadDuplicateWindow))
	if err != nil {
		return fmt.Errorf("check duplicate lead: %w", err)
	}
	if recent != nil {
		return nil
	}
	count, err := s.repo.CountSince(ctx, now.Add(-leadCapWindow))
	if err != nil {
		return fmt.Errorf("count recent leads: %w", err)
	}
	if count >= leadCapPerWindow {
		return domainError.ErrLeadRateLimited
	}

	l.AlertStatus = dto.LeadAlertSkipped
	if s.alertsEnabled {
		l.AlertStatus = dto.LeadAlertPending
		l.AlertNextAttemptAt = &now
	}
	if err := s.repo.Create(ctx, l); err != nil {
		return fmt.Errorf("create lead: %w", err)
	}
	return nil
}

func (s *LeadService) List(ctx context.Context, filter dto.ListLeadsFilter) (*dto.LeadPage, error) {
	if filter.Status != "" && !dto.LeadStatus(filter.Status).Valid() {
		return nil, &domainError.ValidationError{Field: "status", Reason: "must be one of new, contacted, won, lost"}
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = leadDefaultPageSize
	}
	filter.PageSize = min(filter.PageSize, leadMaxPageSize)

	leads, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list leads: %w", err)
	}
	return &dto.LeadPage{Leads: leads, Total: total, Page: filter.Page, PageSize: filter.PageSize}, nil
}

func (s *LeadService) Get(ctx context.Context, id string) (*dto.Lead, error) {
	l, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get lead: %w", err)
	}
	return l, nil
}

func (s *LeadService) Update(ctx context.Context, id string, req *dto.UpdateLeadRequest) (*dto.Lead, error) {
	l, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get lead: %w", err)
	}
	if err := lead.ApplyUpdate(l, req, s.now().UTC()); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, l); err != nil {
		return nil, fmt.Errorf("update lead: %w", err)
	}
	return l, nil
}

func (s *LeadService) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete lead: %w", err)
	}
	return nil
}
