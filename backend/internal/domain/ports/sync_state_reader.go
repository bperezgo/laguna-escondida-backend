package ports

import "laguna-escondida/backend/internal/domain/dto"

// SyncStateReader tells the sign-in context how the box's last pull went. It returns an empty
// state before the first attempt and after a success.
type SyncStateReader interface {
	SyncState() dto.SyncState
}
