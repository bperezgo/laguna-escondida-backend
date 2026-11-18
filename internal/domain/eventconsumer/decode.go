package eventconsumer

import (
	"encoding/json"
	"fmt"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
)

func Decode[T ports.DomainEvent](stored dto.StoredEvent) (T, error) {
	var event T
	if err := json.Unmarshal(stored.Payload, &event); err != nil {
		return event, fmt.Errorf("decode %s event %s: %w", stored.Name, stored.ID, err)
	}
	return event, nil
}
