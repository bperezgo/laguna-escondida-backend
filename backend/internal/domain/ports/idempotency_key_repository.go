package ports

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
)

// IdempotencyKeyRepository stores client idempotency keys. Claim is atomic: when two
// requests race on one key, exactly one gets claimed=true.
type IdempotencyKeyRepository interface {
	Claim(ctx context.Context, key *dto.IdempotencyKey) (claimed bool, err error)
	// Get returns nil when the key does not exist.
	Get(ctx context.Context, scope, key string) (*dto.IdempotencyKey, error)
	Complete(ctx context.Context, scope, key string, response []byte) error
	Release(ctx context.Context, scope, key string) error
	// ReleaseStale drops the key only if it is still in flight and older than olderThan.
	ReleaseStale(ctx context.Context, scope, key string, olderThan time.Duration) (released bool, err error)
}
