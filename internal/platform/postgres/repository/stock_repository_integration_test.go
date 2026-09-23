package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests for the stock ledger's movement kind. They run against the local
// Postgres the migrations were applied to and are gated behind RUN_INTEGRATION_TESTS.

func TestStockRepository_HistoricRecord_KindRoundTrips_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewStockRepository(db.DB)
	ctx := context.Background()
	productID := seedProduct(t, db)

	for _, kind := range []dto.StockMovementKind{
		dto.StockMovementKindSale,
		dto.StockMovementKindPurchase,
		dto.StockMovementKindAdjustment,
		dto.StockMovementKindCount,
		dto.StockMovementKindOpeningBalance,
		dto.StockMovementKindUnknown,
	} {
		opID := uuid.NewString()
		require.NoError(t, repo.CreateHistoricRecord(ctx, &dto.HistoricStock{
			OpID:          opID,
			ProductID:     productID,
			UnitOfMeasure: dto.UnitOfMeasureUnit,
			CreatedAt:     time.Now(),
			Change:        1,
			Kind:          kind,
		}))
	}
	t.Cleanup(func() { db.DB.Exec("DELETE FROM historic_stock WHERE product_id = ?", productID) })

	rows, err := repo.FindHistoricByProductID(ctx, productID)
	require.NoError(t, err)
	require.Len(t, rows, 6)

	got := make(map[dto.StockMovementKind]int)
	for _, row := range rows {
		got[row.Kind]++
	}
	assert.Equal(t, map[dto.StockMovementKind]int{
		dto.StockMovementKindSale:           1,
		dto.StockMovementKindPurchase:       1,
		dto.StockMovementKindAdjustment:     1,
		dto.StockMovementKindCount:          1,
		dto.StockMovementKindOpeningBalance: 1,
		dto.StockMovementKindUnknown:        1,
	}, got)
}

// A movement written without a kind — the shape a peer that predates the column sends —
// must land as unknown rather than violating the column's CHECK.
func TestStockRepository_HistoricRecord_MissingKindDefaultsToUnknown_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewStockRepository(db.DB)
	ctx := context.Background()
	productID := seedProduct(t, db)

	var payload dto.HistoricStockSyncPayload
	require.NoError(t, json.Unmarshal([]byte(`{
		"op_id": "`+uuid.NewString()+`",
		"product_id": "`+productID+`",
		"unit_of_measure": "unit",
		"change": -3,
		"created_at": "2026-01-01T00:00:00Z"
	}`), &payload))
	assert.Equal(t, dto.StockMovementKind(""), payload.Kind, "a legacy peer sends no kind")

	require.NoError(t, repo.CreateHistoricRecord(ctx, &dto.HistoricStock{
		OpID:          payload.OpID,
		ProductID:     payload.ProductID,
		UnitOfMeasure: dto.UnitOfMeasure(payload.UnitOfMeasure),
		CreatedAt:     payload.CreatedAt,
		Change:        payload.Change,
		Kind:          payload.Kind,
	}))
	t.Cleanup(func() { db.DB.Exec("DELETE FROM historic_stock WHERE product_id = ?", productID) })

	rows, err := repo.FindHistoricByProductID(ctx, productID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, dto.StockMovementKindUnknown, rows[0].Kind)
	assert.Equal(t, -3, rows[0].Change)
}
