package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

type CommandItemSSENotifier interface {
	NotifyArea(ctx context.Context, area string, eventType string, data *dto.CommandItemSSE) error
}
