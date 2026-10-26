package ports

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
)

// APIKeyRepository stores organization API keys. Every method but FindByHash acts on the
// context's organization.
type APIKeyRepository interface {
	Create(ctx context.Context, key *dto.APIKey, hash string) error
	List(ctx context.Context) ([]*dto.APIKey, error)
	// Revoke keeps the first revocation time; returns error.ErrAPIKeyNotFound for a key outside the organization.
	Revoke(ctx context.Context, id string, at time.Time) error
	// FindByHash deliberately ignores the org scope: the key is what names the organization.
	// Returns error.ErrAPIKeyNotFound when no key has the hash.
	FindByHash(ctx context.Context, hash string) (*dto.APIKeyRecord, error)
	TouchLastUsed(ctx context.Context, id string, at time.Time) error
}
