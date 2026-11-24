package ports

import "context"

// BoxSecretRepository keeps the secrets an edge box creates for itself. It is edge-only and
// not tenant-scoped: the box's database belongs to one restaurant.
type BoxSecretRepository interface {
	// GetOrCreate returns the secret stored under name, storing candidate first when there is
	// none. Concurrent first calls all return the one value that won.
	GetOrCreate(ctx context.Context, name, candidate string) (string, error)
}
