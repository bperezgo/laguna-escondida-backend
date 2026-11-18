package repository

import (
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The app scope on its own: org B's context neither sees nor changes Laguna's rows.
func TestTenantScope_AnotherOrganizationCannotReachLagunasRows_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	productID := seedProduct(t, db)
	orgB := seedOrganization(t, db)
	asB := tenant.WithOrg(lagunaCtx(), orgB)

	_, err := NewProductRepository(db.DB).FindByID(asB, productID)
	assert.Error(t, err, "Laguna's product is not found from org B")

	got, err := NewProductRepository(db.DB).FindByIDs(asB, []string{productID})
	require.NoError(t, err)
	assert.Empty(t, got)

	now := time.Now().UTC()
	err = NewSyncReferenceRepository(db.DB).UpsertProducts(asB, []dto.ProductSyncPayload{{
		ID: productID, Name: "Overwritten by B", Category: "test", ProductType: "SELLABLE", UnitOfMeasure: "unit",
		Version: 1, UnitPrice: decimal.NewFromInt(1), VAT: decimal.Zero, VATAmount: decimal.Zero, ICO: decimal.Zero,
		ICOAmount: decimal.Zero, SKU: "B-" + productID[:8], TotalPriceWithTaxes: decimal.NewFromInt(1), CreatedAt: now, UpdatedAt: now,
	}})
	require.NoError(t, err)

	var stored productModel
	require.NoError(t, db.DB.First(&stored, "id = ?", productID).Error)
	assert.Equal(t, "Applier Product", stored.Name, "an upsert colliding on id leaves Laguna's row alone")
	assert.Equal(t, lagunaOrgID, stored.OrganizationID)
}

func TestTenantScope_CreateStampsTheContextsOrganization_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	orgB := seedOrganization(t, db)
	asB := tenant.WithOrg(lagunaCtx(), orgB)
	users := NewUserRepository(db.DB)

	user := &dto.User{ID: uuid.NewString(), Username: "cajero-" + orgB[:8], Name: "Cajero B", Password: "x", Active: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	require.NoError(t, users.Create(asB, user))
	t.Cleanup(func() { db.DB.Exec("DELETE FROM users WHERE id = ?", user.ID) })

	var orgOfRow string
	require.NoError(t, db.DB.Raw("SELECT organization_id FROM users WHERE id = ?", user.ID).Scan(&orgOfRow).Error)
	assert.Equal(t, orgB, orgOfRow)

	_, err := users.FindByUsername(lagunaCtx(), user.Username)
	assert.Error(t, err, "Laguna does not see B's user")
	found, err := users.FindByUsername(asB, user.Username)
	require.NoError(t, err)
	assert.Equal(t, user.ID, found.ID)
}
