package ports

import "context"

type StorageClient interface {
	Upload(ctx context.Context, key string, data []byte, contentType string) error
	Download(ctx context.Context, key string) ([]byte, error)
	GetPublicURL(key string) string
}
