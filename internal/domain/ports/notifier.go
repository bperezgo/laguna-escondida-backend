package ports

import "context"

// Notifier defines the contract for sending event notifications to clients.
// Implementations handle the specific transport mechanism (SSE, WebSocket, etc.).
type Notifier interface {
	Notify(ctx context.Context, eventName string, data []byte) error
}
