package repository_test

import (
	"context"
	"encoding/json"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/platform/postgres/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSyncOutboxRepository_Append_RequiresOpID verifies the adapter rejects an entry
// without a caller-provided op_id, before touching the database. Identity is the
// service's responsibility, not the adapter's.
func TestSyncOutboxRepository_Append_RequiresOpID(t *testing.T) {
	repo := repository.NewSyncOutboxRepository(nil)

	err := repo.Append(context.Background(), &dto.SyncOutboxEntry{
		OriginNodeID: uuid.NewString(),
		EntityType:   dto.SyncEntityOpenBill,
		EntityID:     uuid.NewString(),
		Operation:    dto.SyncOperationCreate,
		Payload:      json.RawMessage(`{}`),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "op_id")
}
