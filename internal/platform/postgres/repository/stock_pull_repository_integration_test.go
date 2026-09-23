package repository

import (
	"context"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// findStockPayload returns the payload for a product from a pull batch.
func findStockPayload(rows []dto.StockSyncPayload, productID string) (dto.StockSyncPayload, bool) {
	for _, row := range rows {
		if row.ProductID == productID {
			return row, true
		}
	}
	return dto.StockSyncPayload{}, false
}

func TestSyncReferenceRepository_FindChangedStock_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewSyncReferenceRepository(db.DB)
	ctx := context.Background()

	productID := seedProduct(t, db)
	cleanupStockAndLedger(t, db, productID)
	seedStock(t, db, productID, 1, 30)

	t.Run("changed since the beginning of time", func(t *testing.T) {
		rows, err := repo.FindChangedStock(ctx, time.Time{})
		require.NoError(t, err)

		row, found := findStockPayload(rows, productID)
		require.True(t, found)
		assert.Equal(t, 30, row.Amount)
		assert.Equal(t, "unit", row.UnitOfMeasure)
		assert.Equal(t, 1, row.Version)
		assert.Nil(t, row.DeletedAt)
		assert.False(t, row.UpdatedAt.IsZero())
	})

	t.Run("unchanged since its own cursor", func(t *testing.T) {
		rows, err := repo.FindChangedStock(ctx, time.Time{})
		require.NoError(t, err)
		row, found := findStockPayload(rows, productID)
		require.True(t, found)

		later, err := repo.FindChangedStock(ctx, row.UpdatedAt)
		require.NoError(t, err)
		_, stillThere := findStockPayload(later, productID)
		assert.False(t, stillThere, "a cursor at the row's own updated_at excludes it")
	})

	t.Run("a change after the cursor reappears", func(t *testing.T) {
		rows, err := repo.FindChangedStock(ctx, time.Time{})
		require.NoError(t, err)
		row, _ := findStockPayload(rows, productID)
		cursor := row.UpdatedAt

		require.NoError(t, NewStockRepository(db.DB).UpdateAmount(ctx, productID, 41))

		later, err := repo.FindChangedStock(ctx, cursor)
		require.NoError(t, err)
		changed, found := findStockPayload(later, productID)
		require.True(t, found)
		assert.Equal(t, 41, changed.Amount)
	})

	t.Run("a soft-deleted row still travels", func(t *testing.T) {
		rows, err := repo.FindChangedStock(ctx, time.Time{})
		require.NoError(t, err)
		row, _ := findStockPayload(rows, productID)
		cursor := row.UpdatedAt

		require.NoError(t, NewStockRepository(db.DB).Delete(ctx, productID))

		later, err := repo.FindChangedStock(ctx, cursor)
		require.NoError(t, err)
		deleted, found := findStockPayload(later, productID)
		require.True(t, found, "a removal must reach the edge, not just vanish from the batch")
		assert.NotNil(t, deleted.DeletedAt)
	})
}

func TestSyncReferenceRepository_ReplaceStockAmounts_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewSyncReferenceRepository(db.DB)
	ctx := context.Background()

	productID := seedProduct(t, db)
	cleanupStockAndLedger(t, db, productID)

	now := time.Now()
	payload := dto.StockSyncPayload{
		ProductID:     productID,
		Version:       1,
		Amount:        77,
		UnitOfMeasure: "unit",
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	t.Run("first refresh creates the row", func(t *testing.T) {
		require.NoError(t, repo.ReplaceStockAmounts(ctx, []dto.StockSyncPayload{payload}))
		assert.Equal(t, 77, currentAmount(t, db, productID))
	})

	t.Run("a later refresh replaces rather than accumulates", func(t *testing.T) {
		payload.Amount = 12
		payload.UpdatedAt = time.Now()
		require.NoError(t, repo.ReplaceStockAmounts(ctx, []dto.StockSyncPayload{payload}))
		assert.Equal(t, 12, currentAmount(t, db, productID), "the edge caches the cloud's number, it does not fold it")
	})

	t.Run("a tombstone removes the row from the edge", func(t *testing.T) {
		deletedAt := time.Now()
		payload.DeletedAt = &deletedAt
		payload.UpdatedAt = deletedAt
		require.NoError(t, repo.ReplaceStockAmounts(ctx, []dto.StockSyncPayload{payload}))

		var live int64
		require.NoError(t, db.DB.Raw(
			"SELECT count(*) FROM stock WHERE product_id = ? AND deleted_at IS NULL", productID,
		).Scan(&live).Error)
		assert.Equal(t, int64(0), live)
	})

	t.Run("an empty batch is a no-op", func(t *testing.T) {
		assert.NoError(t, repo.ReplaceStockAmounts(ctx, nil))
	})
}

func TestSyncStateRepository_StockPulledCursor_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewSyncStateRepository(db.DB)
	ctx := context.Background()

	peerNodeID := uuid.NewString()
	t.Cleanup(func() { db.DB.Exec("DELETE FROM sync_state WHERE peer_node_id = ?", peerNodeID) })

	t.Run("first read has no cursor", func(t *testing.T) {
		cursor, err := repo.GetStockPulledCursor(ctx, peerNodeID)
		require.NoError(t, err)
		assert.Nil(t, cursor, "never refreshed, so the first refresh takes everything")
	})

	at := time.Now().Truncate(time.Millisecond)

	t.Run("advance sets it", func(t *testing.T) {
		require.NoError(t, repo.AdvanceStockPulledCursor(ctx, peerNodeID, at))

		cursor, err := repo.GetStockPulledCursor(ctx, peerNodeID)
		require.NoError(t, err)
		require.NotNil(t, cursor)
		assert.WithinDuration(t, at, *cursor, time.Millisecond)
	})

	t.Run("a stale advance does not move it backwards", func(t *testing.T) {
		require.NoError(t, repo.AdvanceStockPulledCursor(ctx, peerNodeID, at.Add(-time.Hour)))

		cursor, err := repo.GetStockPulledCursor(ctx, peerNodeID)
		require.NoError(t, err)
		require.NotNil(t, cursor)
		assert.WithinDuration(t, at, *cursor, time.Millisecond)
	})

	t.Run("it is independent of the reference cursor", func(t *testing.T) {
		require.NoError(t, repo.AdvancePulledCursor(ctx, peerNodeID, at.Add(time.Hour)))

		stockCursor, err := repo.GetStockPulledCursor(ctx, peerNodeID)
		require.NoError(t, err)
		require.NotNil(t, stockCursor)
		assert.WithinDuration(t, at, *stockCursor, time.Millisecond,
			"advancing the reference pull must not move the stock bookmark")
	})
}
