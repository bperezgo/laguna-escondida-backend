package repository

import (
	"context"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newProductSyncPayload(id string) dto.ProductSyncPayload {
	now := time.Now()
	return dto.ProductSyncPayload{
		ID:                  id,
		Name:                "Synced Product",
		Category:            "test",
		ProductType:         "SELLABLE",
		UnitOfMeasure:       "unit",
		Version:             1,
		UnitPrice:           decimal.NewFromInt(10),
		VAT:                 decimal.Zero,
		VATAmount:           decimal.Zero,
		ICO:                 decimal.Zero,
		ICOAmount:           decimal.Zero,
		SKU:                 "SKU-" + id[:8],
		TotalPriceWithTaxes: decimal.NewFromInt(10),
		CreatedAt:           now,
		UpdatedAt:           now,
	}
}

func TestSyncReferenceRepository_FindChangedProducts_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewSyncReferenceRepository(db.DB)

	productID := seedProduct(t, db)
	ctx := context.Background()

	// Read the product's own updated_at back through the repo so the cursor and the
	// stored timestamp share one driver conversion (avoids timestamp-without-tz vs
	// local-time skew at the boundary).
	all, err := repo.FindChangedProducts(ctx, time.Time{})
	require.NoError(t, err)
	var seeded *dto.ProductSyncPayload
	for i := range all {
		if all[i].ID == productID {
			seeded = &all[i]
			break
		}
	}
	require.NotNil(t, seeded, "seeded product is returned for a beginning-of-time cursor")

	// A cursor just before its updated_at includes it.
	included, err := repo.FindChangedProducts(ctx, seeded.UpdatedAt.Add(-time.Second))
	require.NoError(t, err)
	assert.True(t, containsProduct(included, productID), "product changed after the cursor is returned")

	// A strictly-greater cursor at its own updated_at excludes it.
	excluded, err := repo.FindChangedProducts(ctx, seeded.UpdatedAt.Add(time.Second))
	require.NoError(t, err)
	assert.False(t, containsProduct(excluded, productID), "product older than the cursor is excluded")
}

func containsProduct(products []dto.ProductSyncPayload, id string) bool {
	for _, p := range products {
		if p.ID == id {
			return true
		}
	}
	return false
}

func TestSyncReferenceRepository_UpsertProducts_InsertThenSoftDelete_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewSyncReferenceRepository(db.DB)

	id := uuid.NewString()
	t.Cleanup(func() { db.DB.Exec("DELETE FROM products WHERE id = ?", id) })
	ctx := context.Background()

	// Insert via upsert.
	payload := newProductSyncPayload(id)
	require.NoError(t, repo.UpsertProducts(ctx, []dto.ProductSyncPayload{payload}))

	var name string
	db.DB.Raw("SELECT name FROM products WHERE id = ?", id).Scan(&name)
	assert.Equal(t, "Synced Product", name)

	// Re-upsert with a new name + a tombstone (deleted_at set).
	deletedAt := time.Now()
	payload.Name = "Renamed"
	payload.UpdatedAt = deletedAt
	payload.DeletedAt = &deletedAt
	require.NoError(t, repo.UpsertProducts(ctx, []dto.ProductSyncPayload{payload}))

	var updatedName string
	var deleted *time.Time
	db.DB.Raw("SELECT name, deleted_at FROM products WHERE id = ?", id).Row().Scan(&updatedName, &deleted)
	assert.Equal(t, "Renamed", updatedName, "upsert updates the existing row")
	assert.NotNil(t, deleted, "tombstone propagates the soft-delete")
}

func TestSyncReferenceRepository_UpsertUsers_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewSyncReferenceRepository(db.DB)

	id := uuid.NewString()
	t.Cleanup(func() { db.DB.Exec("DELETE FROM users WHERE id = ?", id) })
	ctx := context.Background()

	now := time.Now()
	require.NoError(t, repo.UpsertUsers(ctx, []dto.UserSyncPayload{{
		ID: id, Username: "synced-" + id[:8], Name: "Synced", Password: "hash", CreatedAt: now, UpdatedAt: now,
	}}))

	var password string
	db.DB.Raw("SELECT password FROM users WHERE id = ?", id).Scan(&password)
	assert.Equal(t, "hash", password, "password hash syncs so the edge can authenticate offline")
}

func TestSyncReferenceRepository_UpsertSuppliers_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewSyncReferenceRepository(db.DB)

	id := uuid.NewString()
	t.Cleanup(func() { db.DB.Exec("DELETE FROM suppliers WHERE id = ?", id) })
	ctx := context.Background()

	now := time.Now()
	require.NoError(t, repo.UpsertSuppliers(ctx, []dto.SupplierSyncPayload{{
		ID: id, Name: "Synced Supplier", CreatedAt: now, UpdatedAt: now,
	}}))

	var name string
	db.DB.Raw("SELECT name FROM suppliers WHERE id = ?", id).Scan(&name)
	assert.Equal(t, "Synced Supplier", name)
}
