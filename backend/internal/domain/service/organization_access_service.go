package service

import (
	"context"
	"fmt"
	"slices"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/plans"
	"laguna-escondida/backend/internal/domain/ports"
)

// AccessPrincipal is who makes an authenticated cloud request, as far as plans care.
type AccessPrincipal struct {
	APIKey  bool
	RoleIDs []int
}

// OrganizationAccessService is the one place that decides whether an organization may make a cloud request.
type OrganizationAccessService struct {
	reader ports.OrganizationAccessReader
	now    func() time.Time
}

func NewOrganizationAccessService(reader ports.OrganizationAccessReader) *OrganizationAccessService {
	return &OrganizationAccessService{reader: reader, now: time.Now}
}

// Check refuses, in this order: a suspended organization, a pending one, an expired trial, then a feature the plan excludes.
func (s *OrganizationAccessService) Check(ctx context.Context, orgID string, principal AccessPrincipal, routePath string) error {
	access, err := s.reader.GetAccess(ctx, orgID)
	if err != nil {
		return fmt.Errorf("organization access: %w", err)
	}
	if access.Status == dto.OrganizationStatusSuspended {
		return domainError.ErrOrganizationSuspended
	}
	if access.Status == dto.OrganizationStatusPending {
		return domainError.ErrOrganizationPending
	}
	if plans.Expired(access.Status, access.TrialEndsAt, s.now()) {
		return domainError.ErrTrialExpired
	}
	plan := plans.Plan(access.Plan)
	if (principal.APIKey || slices.Contains(principal.RoleIDs, permissions.RoleAgent)) && !plans.Includes(plan, plans.FeatureAgents) {
		return &domainError.PlanRequiredError{RequiredPlan: string(plans.RequiredPlan(plans.FeatureAgents))}
	}
	if feature := plans.FeatureForRoute(routePath); !plans.Includes(plan, feature) {
		return &domainError.PlanRequiredError{RequiredPlan: string(plans.RequiredPlan(feature))}
	}
	return nil
}

// Usable reports whether an organization may still act: not suspended, not pending and not in an expired trial.
func (s *OrganizationAccessService) Usable(ctx context.Context, orgID string) error {
	access, err := s.reader.GetAccess(ctx, orgID)
	if err != nil {
		return fmt.Errorf("organization access: %w", err)
	}
	if access.Status == dto.OrganizationStatusSuspended || access.Status == dto.OrganizationStatusPending ||
		plans.Expired(access.Status, access.TrialEndsAt, s.now()) {
		return domainError.ErrOrganizationInactive
	}
	return nil
}
