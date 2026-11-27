package cache

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
)

// OrganizationAccessTTL keeps a status or plan change within the spec's minute on every replica.
const OrganizationAccessTTL = 30 * time.Second

type CachedOrganizationAccessReader struct {
	inner  ports.OrganizationAccessReader
	cache  ports.KeyValueCache
	logger *slog.Logger
	ttl    time.Duration
}

func NewCachedOrganizationAccessReader(inner ports.OrganizationAccessReader, cache ports.KeyValueCache, logger *slog.Logger) ports.OrganizationAccessReader {
	return &CachedOrganizationAccessReader{inner: inner, cache: cache, logger: logger, ttl: OrganizationAccessTTL}
}

type accessEntry struct {
	Status      string     `json:"status"`
	Plan        string     `json:"plan"`
	TrialEndsAt *time.Time `json:"trial_ends_at"`
	PaidUntil   *time.Time `json:"paid_until,omitempty"`
	CancelledAt *time.Time `json:"cancelled_at,omitempty"`
	HasCard     bool       `json:"has_card,omitempty"`
}

// GetAccess treats every cache failure as a miss, so the cache can never refuse a request.
func (r *CachedOrganizationAccessReader) GetAccess(ctx context.Context, orgID string) (*dto.OrganizationAccess, error) {
	key := organizationAccessKey(orgID)
	raw, ok, err := r.cache.Get(ctx, key)
	if err != nil {
		r.logger.WarnContext(ctx, "organization access cache read failed", "key", key, "error", err)
	}
	if err == nil && ok {
		var e accessEntry
		if decodeErr := json.Unmarshal(raw, &e); decodeErr == nil {
			access := dto.OrganizationAccess(e)
			return &access, nil
		}
	}

	access, err := r.inner.GetAccess(ctx, orgID)
	if err != nil {
		return nil, err
	}
	encoded, _ := json.Marshal(accessEntry(*access))
	if setErr := r.cache.Set(ctx, key, encoded, r.ttl); setErr != nil {
		r.logger.WarnContext(ctx, "organization access cache write failed", "key", key, "error", setErr)
	}
	return access, nil
}

func organizationAccessKey(orgID string) string { return "org-access:" + orgID }

// OrganizationAccessInvalidator drops an organization's cached access after a billing change, which
// no organization update announces. Other replicas follow within the TTL.
type OrganizationAccessInvalidator struct {
	cache  ports.KeyValueCache
	logger *slog.Logger
}

func NewOrganizationAccessInvalidator(cache ports.KeyValueCache, logger *slog.Logger) ports.OrganizationAccessInvalidator {
	return &OrganizationAccessInvalidator{cache: cache, logger: logger}
}

func (i *OrganizationAccessInvalidator) InvalidateAccess(ctx context.Context, orgID string) {
	if err := i.cache.Delete(ctx, organizationAccessKey(orgID)); err != nil {
		i.logger.WarnContext(ctx, "organization access cache invalidation failed", "organization_id", orgID, "error", err)
	}
}

// InvalidatingOrganizationRepository drops an organization's cached access once an update commits,
// so the operator's change applies at once on this replica (other replicas within the TTL).
type InvalidatingOrganizationRepository struct {
	ports.OrganizationRepository
	cache  ports.KeyValueCache
	logger *slog.Logger
}

func NewInvalidatingOrganizationRepository(inner ports.OrganizationRepository, cache ports.KeyValueCache, logger *slog.Logger) ports.OrganizationRepository {
	return &InvalidatingOrganizationRepository{OrganizationRepository: inner, cache: cache, logger: logger}
}

func (r *InvalidatingOrganizationRepository) Update(ctx context.Context, id string, req *dto.UpdateOrganizationRequest) (*dto.Organization, error) {
	org, err := r.OrganizationRepository.Update(ctx, id, req)
	if err != nil {
		return nil, err
	}
	r.invalidate(ctx, id)
	return org, nil
}

func (r *InvalidatingOrganizationRepository) StartTrial(ctx context.Context, id string, trialEndsAt time.Time) (*dto.Organization, error) {
	org, err := r.OrganizationRepository.StartTrial(ctx, id, trialEndsAt)
	if err != nil {
		return nil, err
	}
	r.invalidate(ctx, id)
	return org, nil
}

func (r *InvalidatingOrganizationRepository) invalidate(ctx context.Context, id string) {
	if delErr := r.cache.Delete(ctx, organizationAccessKey(id)); delErr != nil {
		r.logger.WarnContext(ctx, "organization access cache invalidation failed", "organization_id", id, "error", delErr)
	}
}
