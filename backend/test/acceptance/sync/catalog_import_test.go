package sync

import (
	"encoding/json"
	"strings"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/postgres"
	"laguna-escondida/backend/internal/platform/postgres/repository"
	"laguna-escondida/backend/internal/platform/stockmovement"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const importedCatalog = `{
	"defaults": {"vat": 0, "ico": 8, "taxes_format": "percentage", "unit_of_measure": "g", "prep_area": "kitchen", "prep_priority": 1},
	"ingredients": [
		{"ref": "arroz-blanco", "name": "Arroz blanco", "category": "Insumos"},
		{"ref": "arroz-coco", "name": "Arroz de coco", "category": "Insumos"},
		{"ref": "tilapia-cruda", "name": "Tilapia entera", "category": "Insumos", "unit_of_measure": "unit"}
	],
	"sellables": [{"ref": "coca", "name": "Coca-Cola 400ml", "category": "Bebidas", "unit_of_measure": "unit", "total_price_with_taxes": 5000, "prep": {"area": "bar"}}],
	"plates": [{"ref": "tilapia", "name": "Tilapia frita", "category": "Platos", "unit_of_measure": "unit", "total_price_with_taxes": 32000,
		"recipe": [{"ingredient": "tilapia-cruda", "quantity": 1}],
		"slots": [{"name": "Arroz", "choices": [
			{"name": "Arroz blanco", "default": true, "items": [{"ingredient": "arroz-blanco", "quantity": 200}]},
			{"name": "Arroz de coco", "items": [{"ingredient": "arroz-coco", "quantity": 200}]}]}]}],
	"opening_stock": [{"product": "arroz-blanco", "counted": 25000}, {"product": "tilapia-cruda", "counted": 40}]
}`

// An imported catalog is ordinary catalog data: the box receives it through the same pull as
// one built endpoint by endpoint, products, recipes, slots, preparation areas and stock alike.
func TestSync_Pull_ImportedCatalogReachesTheEdge(t *testing.T) {
	r := newRig(t)
	imports := service.NewCatalogImportService(
		repository.NewProductRepository(cloudGDB), repository.NewProductIngredientRepository(cloudGDB),
		repository.NewPlateOptionSlotRepository(cloudGDB), repository.NewStockRepository(cloudGDB),
		postgres.NewUnitOfWork(cloudGDB), stockmovement.NewNoopEmitter(),
	)
	var doc dto.CatalogImportDocument
	dec := json.NewDecoder(strings.NewReader(importedCatalog))
	dec.DisallowUnknownFields()
	require.NoError(t, dec.Decode(&doc))
	validation, err := imports.Validate(r.ctx, &doc)
	require.NoError(t, err)
	require.True(t, validation.Valid, "%+v", validation.Problems)
	applied, err := imports.Apply(r.ctx, &dto.ApplyCatalogImportRequest{CatalogImportDocument: doc, ExpectedDigest: validation.Digest})
	require.NoError(t, err)
	id := func(ref string) string { return applied.Products[ref].ID }

	res := r.pull()

	assert.Equal(t, 5, res.Products)
	assert.Equal(t, 1, res.ProductIngredients)
	assert.Equal(t, 1, res.PlateOptionSlots)
	assert.Equal(t, 2, res.ProductResponsibilities, "the sellable and the plate")
	for ref, created := range applied.Products {
		onEdge, ok := r.edgeProductByID(created.ID)
		require.True(t, ok, "%s reaches the box", ref)
		assert.Equal(t, created.SKU, onEdge.SKU)
	}
	coca, _ := r.edgeProductByID(id("coca"))
	assert.Equal(t, "0.08", coca.ICO.String())
	assert.Equal(t, int64(1), r.edgeCount("product_ingredients", "composite_product_id = ? AND ingredient_product_id = ?", id("tilapia"), id("tilapia-cruda")))
	assert.Equal(t, int64(1), r.edgeCount("product_preparation_responsibilities", "product_id = ? AND area = 'bar' AND priority = 1", id("coca")))
	assert.Equal(t, int64(1), r.edgeCount("product_preparation_responsibilities", "product_id = ? AND area = 'kitchen'", id("tilapia")))

	cloudSlots, err := repository.NewPlateOptionSlotRepository(cloudGDB).FindByCompositeProductID(r.ctx, id("tilapia"))
	require.NoError(t, err)
	edgeSlots, err := repository.NewPlateOptionSlotRepository(edgeGDB).FindByCompositeProductID(r.ctx, id("tilapia"))
	require.NoError(t, err)
	assert.Equal(t, choiceNamesOf(cloudSlots), choiceNamesOf(edgeSlots))
	require.Len(t, edgeSlots, 1)
	assert.Equal(t, cloudSlots[0].Choices[0].ID, edgeSlots[0].Choices[0].ID, "the box keeps the cloud's ids")
	assert.Equal(t, id("arroz-blanco"), edgeSlots[0].Choices[0].Items[0].IngredientProductID)

	assert.Equal(t, 2, r.refreshEdgeStock().Stock)
	assert.Equal(t, 25000, r.edgeStockAmount(id("arroz-blanco")))
	assert.Equal(t, 40, r.edgeStockAmount(id("tilapia-cruda")))
	assert.Equal(t, 25000, r.cloudLedgerSum(id("arroz-blanco")), "the opening count explains the cloud's on-hand")

	assert.Zero(t, r.pull().Products, "nothing new to pull")
}
