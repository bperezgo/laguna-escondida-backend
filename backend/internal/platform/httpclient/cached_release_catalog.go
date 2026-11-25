package httpclient

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
)

const (
	// releaseCacheLifetime is how long a newly published release may wait to be offered.
	releaseCacheLifetime = 10 * time.Minute
	// releaseRetryAfterFailure keeps a GitHub outage from making every visitor wait on the timeout.
	releaseRetryAfterFailure = time.Minute
)

type cachedCatalogValue interface {
	[]dto.EdgeRelease | *dto.EdgeManifest
}

// cachedValue keeps the last successful read; a failed refresh keeps serving it.
type cachedValue[T cachedCatalogValue] struct {
	mu        sync.Mutex
	value     T
	hasValue  bool
	err       error
	nextCheck time.Time
}

func (c *cachedValue[T]) get(ctx context.Context, now time.Time, read func() (T, error), onStale func(error)) (T, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if now.Before(c.nextCheck) {
		if c.hasValue {
			return c.value, nil
		}
		return c.value, c.err
	}
	value, err := read()
	if err == nil {
		c.value, c.hasValue, c.err, c.nextCheck = value, true, nil, now.Add(releaseCacheLifetime)
		return value, nil
	}
	// A caller that gave up says nothing about GitHub: the next caller tries again.
	if ctx.Err() == nil {
		c.err, c.nextCheck = err, now.Add(releaseRetryAfterFailure)
	}
	if c.hasValue {
		onStale(err)
		return c.value, nil
	}
	return value, err
}

// CachedReleaseCatalog holds what it read in memory, per instance.
type CachedReleaseCatalog struct {
	upstream ports.EdgeReleaseCatalog
	now      func() time.Time
	logger   *slog.Logger

	releases cachedValue[[]dto.EdgeRelease]

	manifestsMu sync.Mutex
	manifests   map[string]*cachedValue[*dto.EdgeManifest]
}

func NewCachedReleaseCatalog(upstream ports.EdgeReleaseCatalog, logger *slog.Logger, now func() time.Time) ports.EdgeReleaseCatalog {
	if now == nil {
		now = time.Now
	}
	return &CachedReleaseCatalog{upstream: upstream, now: now, logger: logger, manifests: map[string]*cachedValue[*dto.EdgeManifest]{}}
}

func (c *CachedReleaseCatalog) ListEdgeReleases(ctx context.Context) ([]dto.EdgeRelease, error) {
	return c.releases.get(ctx, c.now(),
		func() ([]dto.EdgeRelease, error) { return c.upstream.ListEdgeReleases(ctx) },
		func(err error) {
			c.logger.Warn("Refreshing the edge release list failed; serving the last one", "error", err)
		})
}

func (c *CachedReleaseCatalog) GetEdgeManifest(ctx context.Context, version string) (*dto.EdgeManifest, error) {
	c.manifestsMu.Lock()
	entry, ok := c.manifests[version]
	if !ok {
		entry = &cachedValue[*dto.EdgeManifest]{}
		c.manifests[version] = entry
	}
	c.manifestsMu.Unlock()
	return entry.get(ctx, c.now(),
		func() (*dto.EdgeManifest, error) { return c.upstream.GetEdgeManifest(ctx, version) },
		func(err error) {
			c.logger.Warn("Refreshing an edge manifest failed; serving the last one", "version", version, "error", err)
		})
}
