package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

type OpenBillProductSSENotifier interface {
	NotifyArea(ctx context.Context, area string, eventType string, data *dto.OpenBillProductSSE) error
}
