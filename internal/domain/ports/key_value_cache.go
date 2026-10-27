package ports

import (
	"context"
	"time"
)

// KeyValueCache stores bytes so a shared cache (Redis) can replace the in-memory one unchanged.
type KeyValueCache interface {
	// Get reports false on a miss or an expired entry.
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}
