// Package cache holds the typed decorators that put a ports.KeyValueCache in front of a repository.
package cache

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/tenant"
)

// OrganizationSettingsTTL bounds how stale another replica, or a timezone edited through the
// platform routes, can leave availability.
const OrganizationSettingsTTL = 30 * time.Second

type CachedOrganizationSettingsRepository struct {
	inner  ports.OrganizationSettingsRepository
	cache  ports.KeyValueCache
	logger *slog.Logger
	ttl    time.Duration
}

func NewCachedOrganizationSettingsRepository(inner ports.OrganizationSettingsRepository, cache ports.KeyValueCache, logger *slog.Logger) ports.OrganizationSettingsRepository {
	return &CachedOrganizationSettingsRepository{inner: inner, cache: cache, logger: logger, ttl: OrganizationSettingsTTL}
}

// The key comes only from the context's organization: a wrong key would leak one tenant's settings to another.
func settingsKey(ctx context.Context) (string, error) {
	orgID, err := tenant.RequireOrg(ctx)
	if err != nil {
		return "", err
	}
	return "org-settings:" + orgID, nil
}

// Get treats every cache failure as a miss, so the cache can never block an order.
func (r *CachedOrganizationSettingsRepository) Get(ctx context.Context) (*dto.OrganizationSettingsView, error) {
	key, err := settingsKey(ctx)
	if err != nil {
		return nil, err
	}
	raw, ok, err := r.cache.Get(ctx, key)
	if err != nil {
		r.logger.WarnContext(ctx, "organization settings cache read failed", "key", key, "error", err)
	}
	if err == nil && ok {
		var view dto.OrganizationSettingsView
		if decodeErr := json.Unmarshal(raw, &view); decodeErr == nil {
			return &view, nil
		} else {
			r.logger.WarnContext(ctx, "organization settings cache entry is unreadable", "key", key, "error", decodeErr)
		}
	}

	view, err := r.inner.Get(ctx)
	if err != nil {
		return nil, err
	}
	if encoded, encodeErr := json.Marshal(view); encodeErr == nil {
		if setErr := r.cache.Set(ctx, key, encoded, r.ttl); setErr != nil {
			r.logger.WarnContext(ctx, "organization settings cache write failed", "key", key, "error", setErr)
		}
	}
	return view, nil
}

// UpdateOrdering drops the entry once the inner write has committed, so this instance's next read reloads it.
func (r *CachedOrganizationSettingsRepository) UpdateOrdering(ctx context.Context, merge ports.OrderingMerge) (*dto.OrganizationSettingsView, error) {
	key, err := settingsKey(ctx)
	if err != nil {
		return nil, err
	}
	saved, err := r.inner.UpdateOrdering(ctx, merge)
	if err != nil {
		return nil, err
	}
	if err := r.cache.Delete(ctx, key); err != nil {
		r.logger.WarnContext(ctx, "organization settings cache invalidation failed", "key", key, "error", err)
	}
	return saved, nil
}
