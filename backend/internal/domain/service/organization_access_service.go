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

// BillingGrace is how long a live subscription keeps working past its due date, so the charge and
// its retries can finish.
const BillingGrace = 72 * time.Hour

// StoppedReason is ErrTrialExpired for an ended trial, ErrPaymentRequired for a lapsed paid period
// and nil otherwise. It is computed at every check, so no job has to change a status.
func StoppedReason(access *dto.OrganizationAccess, now time.Time) error {
	switch access.Status {
	case dto.OrganizationStatusTrial:
		if access.TrialEndsAt == nil {
			return nil
		}
		end := *access.TrialEndsAt
		if access.HasCard && access.CancelledAt == nil {
			end = end.Add(BillingGrace)
		}
		if !now.Before(end) {
			return domainError.ErrTrialExpired
		}
	case dto.OrganizationStatusActive:
		// Without a paid period the operator activated it by hand: it never stops for lack of payment.
		if access.PaidUntil == nil {
			return nil
		}
		end := *access.PaidUntil
		if access.CancelledAt == nil {
			end = end.Add(BillingGrace)
		}
		if !now.Before(end) {
			return domainError.ErrPaymentRequired
		}
	}
	return nil
}

func isStopped(access *dto.OrganizationAccess, now time.Time) bool {
	return StoppedReason(access, now) != nil
}

// Check refuses, in this order: a suspended organization, a pending one, an expired trial or a
// lapsed payment, then a feature the plan excludes.
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
	if err := StoppedReason(access, s.now()); err != nil {
		return err
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

// Usable reports whether an organization may still act: not suspended, not pending, not in an expired trial and not unpaid.
func (s *OrganizationAccessService) Usable(ctx context.Context, orgID string) error {
	access, err := s.reader.GetAccess(ctx, orgID)
	if err != nil {
		return fmt.Errorf("organization access: %w", err)
	}
	if access.Status == dto.OrganizationStatusSuspended || access.Status == dto.OrganizationStatusPending ||
		StoppedReason(access, s.now()) != nil {
		return domainError.ErrOrganizationInactive
	}
	return nil
}

// Stopped is ErrTrialExpired or ErrPaymentRequired when the organization stopped for one of them,
// and nil for every other organization, working or not.
func (s *OrganizationAccessService) Stopped(ctx context.Context, orgID string) error {
	access, err := s.reader.GetAccess(ctx, orgID)
	if err != nil {
		return fmt.Errorf("organization access: %w", err)
	}
	return StoppedReason(access, s.now())
}

// SetupState is what a setup token's organization is in, as far as the setup routes care.
type SetupState string

const (
	SetupStatePending SetupState = "pending"
	SetupStateStopped SetupState = "stopped"
	SetupStateWorking SetupState = "working"
	// SetupStateClosed is a suspended organization: no setup route is for it.
	SetupStateClosed SetupState = "closed"
)

func (s *OrganizationAccessService) SetupState(ctx context.Context, orgID string) (SetupState, error) {
	access, err := s.reader.GetAccess(ctx, orgID)
	if err != nil {
		return "", fmt.Errorf("organization access: %w", err)
	}
	stopped := isStopped(access, s.now())
	switch {
	case access.Status == dto.OrganizationStatusPending:
		return SetupStatePending, nil
	case access.Status == dto.OrganizationStatusSuspended:
		return SetupStateClosed, nil
	case stopped:
		return SetupStateStopped, nil
	default:
		return SetupStateWorking, nil
	}
}

// Pending reports whether the organization still waits for its trial to start.
func (s *OrganizationAccessService) Pending(ctx context.Context, orgID string) (bool, error) {
	access, err := s.reader.GetAccess(ctx, orgID)
	if err != nil {
		return false, fmt.Errorf("organization access: %w", err)
	}
	return access.Status == dto.OrganizationStatusPending, nil
}
