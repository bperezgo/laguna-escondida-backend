package ports

import "context"

type SSENotifier interface {
	NotifyArea(ctx context.Context, area string, eventType string, data any) error
}
