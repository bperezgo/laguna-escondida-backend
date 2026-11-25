// Package memory is a process-local ports.KeyValueCache: exact on one instance, invisible to others.
package memory

import (
	"context"
	"sync"
	"time"

	"laguna-escondida/backend/internal/domain/ports"
)

type entry struct {
	value     []byte
	expiresAt time.Time
}

type Cache struct {
	mu         sync.RWMutex
	entries    map[string]entry
	maxEntries int
	now        func() time.Time
}

type Option func(*Cache)

// WithMaxEntries bounds the cache: once full, Set drops new keys (no eviction) until Sweep frees space.
func WithMaxEntries(n int) Option {
	return func(c *Cache) { c.maxEntries = n }
}

// New starts nothing: reads skip expired entries, and Sweep removes them while it runs.
func New(opts ...Option) *Cache {
	c := &Cache{entries: map[string]entry{}, now: time.Now}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

var _ ports.KeyValueCache = (*Cache)(nil)

func (c *Cache) Get(_ context.Context, key string) ([]byte, bool, error) {
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || !c.now().Before(e.expiresAt) {
		return nil, false, nil
	}
	return append([]byte(nil), e.value...), true, nil
}

func (c *Cache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.entries[key]; !exists && c.maxEntries > 0 && len(c.entries) >= c.maxEntries {
		return nil
	}
	c.entries[key] = entry{value: append([]byte(nil), value...), expiresAt: c.now().Add(ttl)}
	return nil
}

func (c *Cache) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
	return nil
}

// Sweep drops expired entries every interval until ctx is done.
func (c *Cache) Sweep(ctx context.Context, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.sweep()
		}
	}
}

func (c *Cache) sweep() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	for key, e := range c.entries {
		if !now.Before(e.expiresAt) {
			delete(c.entries, key)
		}
	}
}

func (c *Cache) len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}
