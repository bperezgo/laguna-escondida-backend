package service

import (
	"context"
	"fmt"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/tenant"
)

// OrganizationRunner runs work that has no request behind it once per organization, each run
// in that organization's context and database session, so it reads and writes only its rows.
type OrganizationRunner struct {
	orgs     ports.OrganizationRepository
	sessions ports.SessionOpener
}

func NewOrganizationRunner(orgs ports.OrganizationRepository, sessions ports.SessionOpener) *OrganizationRunner {
	return &OrganizationRunner{orgs: orgs, sessions: sessions}
}

// OrganizationScope picks which organizations a job runs for.
type OrganizationScope int

const (
	// AllActiveOrganizations skips suspended organizations.
	AllActiveOrganizations OrganizationScope = iota
	// EInvoicingOrganizations also skips organizations without e-invoicing configured.
	EInvoicingOrganizations
)

// ForEach runs job for every organization in scope. One organization's failure doesn't stop
// the others; each comes back with its organization's id for the caller to log. The error is
// only for not being able to list the organizations at all.
func (r *OrganizationRunner) ForEach(ctx context.Context, scope OrganizationScope, job func(ctx context.Context) error) ([]dto.OrganizationJobFailure, error) {
	orgs, err := r.orgs.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list organizations: %w", err)
	}
	var failures []dto.OrganizationJobFailure
	for _, org := range orgs {
		if org.Status != dto.OrganizationStatusActive {
			continue
		}
		if scope == EInvoicingOrganizations && org.EInvoicing == nil {
			continue
		}
		if err := r.run(ctx, org.ID, job); err != nil {
			failures = append(failures, dto.OrganizationJobFailure{OrganizationID: org.ID, Err: err})
		}
	}
	return failures, nil
}

// Run runs job for one organization, as ForEach does for each.
func (r *OrganizationRunner) Run(ctx context.Context, orgID string, job func(ctx context.Context) error) error {
	return r.run(ctx, orgID, job)
}

// RunBySlug runs job for the organization a caller without one (the admin API key) named.
// It returns error.ErrOrganizationRequired for an empty slug and error.ErrOrganizationNotFound
// for an unknown one.
func (r *OrganizationRunner) RunBySlug(ctx context.Context, slug string, job func(ctx context.Context) error) error {
	if slug == "" {
		return domainError.ErrOrganizationRequired
	}
	org, err := r.orgs.GetBySlug(ctx, slug)
	if err != nil {
		return err
	}
	return r.run(ctx, org.ID, job)
}

func (r *OrganizationRunner) run(ctx context.Context, orgID string, job func(ctx context.Context) error) error {
	orgCtx, release := r.sessions.Open(tenant.WithOrg(ctx, orgID))
	defer release()
	return job(orgCtx)
}
