package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"laguna-escondida/backend/internal/domain/aggregate/product"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type catalogImportMocks struct {
	products    *mocks.MockProductRepository
	ingredients *mocks.MockProductIngredientRepository
	slots       *mocks.MockPlateOptionSlotRepository
	stock       *mocks.MockStockRepository
	emitter     *mocks.MockStockMovementEmitter
	rolledBack  bool
}

func newTestCatalogImportService(t *testing.T, catalog ...*dto.Product) (*CatalogImportService, *catalogImportMocks) {
	m := &catalogImportMocks{
		products:    mocks.NewMockProductRepository(t),
		ingredients: mocks.NewMockProductIngredientRepository(t),
		slots:       mocks.NewMockPlateOptionSlotRepository(t),
		stock:       mocks.NewMockStockRepository(t),
		emitter:     mocks.NewMockStockMovementEmitter(t),
	}
	m.products.EXPECT().FindAll(mock.Anything, dto.ListProductsRequest{}).Return(catalog, nil).Maybe()
	svc := NewCatalogImportService(m.products, m.ingredients, m.slots, m.stock, rollbackUnitOfWork{rolledBack: &m.rolledBack}, m.emitter)
	return svc, m
}

// decodeImportDoc decodes like the handler does, unknown fields refused.
func decodeImportDoc(t *testing.T, raw string) *dto.CatalogImportDocument {
	t.Helper()
	var doc dto.CatalogImportDocument
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	require.NoError(t, dec.Decode(&doc))
	return &doc
}

func catalogProduct(id, name, sku string, productType dto.ProductType) *dto.Product {
	return &dto.Product{ID: id, Name: name, SKU: sku, ProductType: productType, Category: "insumos", UnitOfMeasure: dto.UnitOfMeasureGram, Version: 1}
}

func problemCodes(result *dto.CatalogImportResult) []string {
	codes := make([]string, len(result.Problems))
	for i, p := range result.Problems {
		codes[i] = p.Code
	}
	return codes
}

func requireProblem(t *testing.T, result *dto.CatalogImportResult, path, code string) dto.CatalogImportProblem {
	t.Helper()
	for _, p := range result.Problems {
		if p.Path == path && p.Code == code {
			return p
		}
	}
	require.Failf(t, "problem not reported", "want %s at %s, got %+v", code, path, result.Problems)
	return dto.CatalogImportProblem{}
}

const importDefaults = `"defaults": {"vat": 0, "ico": 8, "taxes_format": "percentage", "unit_of_measure": "g", "prep_area": "kitchen", "prep_priority": 0}`

// fullImportDoc has 3 ingredients, 2 sellables and 1 plate with a recipe and one slot.
const fullImportDoc = `{
	` + importDefaults + `,
	"ingredients": [
		{"ref": "arroz", "name": "Arroz blanco", "category": "insumos"},
		{"ref": "frijol", "name": "Frijol", "category": "insumos"},
		{"ref": "coco", "name": "Arroz de coco", "category": "insumos"}
	],
	"sellables": [
		{"ref": "coca", "name": "Coca-Cola 400ml", "category": "bebidas", "total_price_with_taxes": 5000, "unit_of_measure": "unit", "prep": {"area": "bar"}},
		{"ref": "limonada", "name": "Limonada", "category": "bebidas", "total_price_with_taxes": 7000, "unit_of_measure": "unit", "sku": "BAR-LIM"}
	],
	"plates": [
		{"ref": "bandeja", "name": "Bandeja paisa", "category": "platos", "total_price_with_taxes": 32000, "unit_of_measure": "unit",
		 "recipe": [{"ingredient": "frijol", "quantity": 150}],
		 "slots": [{"name": "Arroz", "choices": [
			{"name": "Blanco", "default": true, "items": [{"ingredient": "arroz", "quantity": 200}]},
			{"name": "De coco", "items": [{"ingredient": "coco", "quantity": 200}]},
			{"name": "Sin arroz"}
		 ]}]}
	],
	"opening_stock": [{"product": "arroz", "counted": 25000}]
}`

func TestCatalogImportService_Validate_ValidDocument(t *testing.T) {
	svc, _ := newTestCatalogImportService(t)

	result, err := svc.Validate(context.Background(), decodeImportDoc(t, fullImportDoc))

	require.NoError(t, err)
	assert.True(t, result.Valid, "%+v", result.Problems)
	assert.Empty(t, result.Problems)
	assert.Len(t, result.Digest, 64)
	assert.Equal(t, 6, result.Summary.Products)
	assert.Equal(t, 3, result.Summary.Ingredients)
	assert.Equal(t, 2, result.Summary.Sellables)
	assert.Equal(t, 1, result.Summary.Plates)
	assert.Equal(t, 1, result.Summary.RecipeLines)
	assert.Equal(t, 1, result.Summary.OptionSlots)
	assert.Equal(t, 3, result.Summary.OptionChoices)
	assert.Equal(t, 3, result.Summary.PreparationAreas)
	assert.Equal(t, 1, result.Summary.OpeningStock)
	assert.Contains(t, result.Summary.GeneratedSKUs, dto.CatalogImportGeneratedSKU{Ref: "coca", SKU: "BAR-COC-COL-400"})
	assert.NotContains(t, result.Summary.GeneratedSKUs, dto.CatalogImportGeneratedSKU{Ref: "limonada", SKU: "BAR-LIM"})
}

func TestCatalogImportService_Validate_PlateReferencesIngredientByRef(t *testing.T) {
	svc, _ := newTestCatalogImportService(t)
	doc := decodeImportDoc(t, `{`+importDefaults+`,
		"ingredients": [{"ref": "arroz", "name": "Arroz", "category": "insumos"}],
		"plates": [{"ref": "p", "name": "Arroz con pollo", "category": "platos", "total_price_with_taxes": 20000,
			"recipe": [{"ingredient": "arroz", "quantity": 200}]}]}`)

	result, err := svc.Validate(context.Background(), doc)

	require.NoError(t, err)
	assert.True(t, result.Valid, "%+v", result.Problems)
}

func TestCatalogImportService_Validate_PlateDefinedBeforeItsIngredient(t *testing.T) {
	svc, _ := newTestCatalogImportService(t)
	doc := decodeImportDoc(t, `{"plates": [{"ref": "p", "name": "Frijolada", "category": "platos", "total_price_with_taxes": 15000,
			"recipe": [{"ingredient": "frijol", "quantity": 150}]}],
		"ingredients": [{"ref": "frijol", "name": "Frijol", "category": "insumos"}],
		`+importDefaults+`}`)

	result, err := svc.Validate(context.Background(), doc)

	require.NoError(t, err)
	assert.True(t, result.Valid, "%+v", result.Problems)
}

func TestCatalogImportService_Validate_DuplicateRef(t *testing.T) {
	svc, _ := newTestCatalogImportService(t)
	doc := decodeImportDoc(t, `{`+importDefaults+`,
		"ingredients": [{"ref": "papa", "name": "Papa criolla", "category": "insumos"}],
		"sellables": [{"ref": "papa", "name": "Papas fritas", "category": "acompañantes", "total_price_with_taxes": 6000}]}`)

	result, err := svc.Validate(context.Background(), doc)

	require.NoError(t, err)
	assert.False(t, result.Valid)
	requireProblem(t, result, "/sellables/0", ProblemDuplicateRef)
	assert.Len(t, result.Problems, 1)
}

func TestCatalogImportService_Validate_UnknownRef(t *testing.T) {
	svc, _ := newTestCatalogImportService(t)
	doc := decodeImportDoc(t, `{`+importDefaults+`,
		"plates": [{"ref": "p", "name": "Mojarra", "category": "platos", "total_price_with_taxes": 30000,
			"recipe": [{"ingredient": "yuca", "quantity": 100}]}]}`)

	result, err := svc.Validate(context.Background(), doc)

	require.NoError(t, err)
	requireProblem(t, result, "/plates/0/recipe/0", ProblemUnknownRef)
}

func TestCatalogImportService_Validate_SeveralProblemsReportedAtOnce(t *testing.T) {
	// The strict mocks fail the test if anything is written.
	svc, _ := newTestCatalogImportService(t)
	doc := decodeImportDoc(t, `{`+importDefaults+`,
		"ingredients": [{"ref": "arroz", "name": "Arroz", "category": "insumos"}],
		"plates": [
			{"ref": "p1", "name": "Mojarra", "category": "platos", "total_price_with_taxes": 30000,
			 "recipe": [{"ingredient": "yuca", "quantity": 100}]},
			{"ref": "p2", "name": "Tilapia", "category": "platos", "total_price_with_taxes": 30000,
			 "slots": [{"name": "Arroz", "choices": [{"name": "Blanco", "items": [{"ingredient": "arroz", "quantity": 1}]}, {"name": "Sin arroz"}]}]}
		]}`)

	result, err := svc.Validate(context.Background(), doc)

	require.NoError(t, err)
	assert.False(t, result.Valid)
	requireProblem(t, result, "/plates/0/recipe/0", ProblemUnknownRef)
	requireProblem(t, result, "/plates/1/slots/0", ProblemSlotDefaultCount)
	assert.Len(t, result.Problems, 2)
}

func TestCatalogImportService_Validate_PlateReusesAnExistingIngredient(t *testing.T) {
	limon := catalogProduct("01900000-0000-7000-8000-0000000000a1", "Limón", "ING-LIM", dto.ProductTypeIngredient)
	svc, _ := newTestCatalogImportService(t, limon)
	doc := decodeImportDoc(t, `{`+importDefaults+`,
		"existing": [{"ref": "limon", "name": "limón "}],
		"sellables": [],
		"plates": [{"ref": "p", "name": "Pescado al limón", "category": "platos", "total_price_with_taxes": 30000,
			"recipe": [{"ingredient": "limon", "quantity": 1}]}]}`)

	result, err := svc.Validate(context.Background(), doc)

	require.NoError(t, err)
	assert.True(t, result.Valid, "%+v", result.Problems)
	assert.Equal(t, 1, result.Summary.Products)
	assert.Equal(t, 1, result.Summary.Existing)
}

func TestCatalogImportService_Validate_ExistingBySKU(t *testing.T) {
	limon := catalogProduct("01900000-0000-7000-8000-0000000000a1", "Limón", "ING-LIM", dto.ProductTypeIngredient)
	svc, _ := newTestCatalogImportService(t, limon)
	doc := decodeImportDoc(t, `{`+importDefaults+`,
		"existing": [{"ref": "limon", "sku": "ING-LIM"}, {"ref": "otro", "sku": "ing-lim"}]}`)

	result, err := svc.Validate(context.Background(), doc)

	require.NoError(t, err)
	assert.Equal(t, []string{ProblemExistingNotFound}, problemCodes(result), "sku match is exact")
	requireProblem(t, result, "/existing/1", ProblemExistingNotFound)
}

func TestCatalogImportService_Validate_ExistingNeedsExactlyOneKey(t *testing.T) {
	svc, _ := newTestCatalogImportService(t)
	doc := decodeImportDoc(t, `{"existing": [{"ref": "a"}, {"ref": "b", "sku": "X", "name": "Y"}]}`)

	result, err := svc.Validate(context.Background(), doc)

	require.NoError(t, err)
	requireProblem(t, result, "/existing/0", ProblemMissingField)
	requireProblem(t, result, "/existing/1", ProblemInvalidField)
}

func TestCatalogImportService_Validate_ExistingAmbiguousName(t *testing.T) {
	svc, _ := newTestCatalogImportService(t,
		catalogProduct("01900000-0000-7000-8000-0000000000a1", "Limón", "ING-LIM", dto.ProductTypeIngredient),
		catalogProduct("01900000-0000-7000-8000-0000000000a2", "limon", "ING-LIM-2", dto.ProductTypeIngredient),
		catalogProduct("01900000-0000-7000-8000-0000000000a3", "LIMÓN", "ING-LIM-3", dto.ProductTypeIngredient),
	)
	doc := decodeImportDoc(t, `{"existing": [{"ref": "limon", "name": "Limón"}]}`)

	result, err := svc.Validate(context.Background(), doc)

	require.NoError(t, err)
	requireProblem(t, result, "/existing/0", ProblemExistingAmbiguous)
}

func TestCatalogImportService_Validate_NewEntryCollidesWithAnExistingProduct(t *testing.T) {
	svc, _ := newTestCatalogImportService(t,
		catalogProduct("01900000-0000-7000-8000-0000000000b1", "Coca-Cola 400ml", "BAR-COC-COL-400", dto.ProductTypeSellable))
	doc := decodeImportDoc(t, `{`+importDefaults+`,
		"sellables": [{"ref": "coca", "name": " coca-cola 400ml", "category": "bebidas", "total_price_with_taxes": 5000}]}`)

	result, err := svc.Validate(context.Background(), doc)

	require.NoError(t, err)
	requireProblem(t, result, "/sellables/0", ProblemNameConflict)
}

func TestCatalogImportService_Validate_NameConflictInsideTheDocument(t *testing.T) {
	svc, _ := newTestCatalogImportService(t)
	doc := decodeImportDoc(t, `{`+importDefaults+`,
		"ingredients": [{"ref": "a", "name": "Arroz", "category": "insumos"}],
		"sellables": [{"ref": "b", "name": "ARROZ ", "category": "acompañantes", "total_price_with_taxes": 4000}]}`)

	result, err := svc.Validate(context.Background(), doc)

	require.NoError(t, err)
	requireProblem(t, result, "/sellables/0", ProblemNameConflict)
	assert.Len(t, result.Problems, 1)
}

func TestCatalogImportService_Validate_SKUConflicts(t *testing.T) {
	svc, _ := newTestCatalogImportService(t,
		catalogProduct("01900000-0000-7000-8000-0000000000b1", "Coca-Cola", "BAR-COC", dto.ProductTypeSellable))
	doc := decodeImportDoc(t, `{`+importDefaults+`,
		"sellables": [
			{"ref": "a", "name": "Pepsi", "category": "bebidas", "total_price_with_taxes": 4000, "sku": "bar-coc"},
			{"ref": "b", "name": "Sprite", "category": "bebidas", "total_price_with_taxes": 4000, "sku": "BAR-SPR"},
			{"ref": "c", "name": "Fanta", "category": "bebidas", "total_price_with_taxes": 4000, "sku": "bar-spr"}
		]}`)

	result, err := svc.Validate(context.Background(), doc)

	require.NoError(t, err)
	requireProblem(t, result, "/sellables/0", ProblemSKUConflict)
	requireProblem(t, result, "/sellables/2", ProblemSKUConflict)
	assert.Len(t, result.Problems, 2)
}

func TestCatalogImportService_Validate_InvalidGivenSKU(t *testing.T) {
	svc, _ := newTestCatalogImportService(t)
	doc := decodeImportDoc(t, `{`+importDefaults+`,
		"ingredients": [{"ref": "a", "name": "Arroz", "category": "insumos", "sku": "ARROZ BLANCO"}]}`)

	result, err := svc.Validate(context.Background(), doc)

	require.NoError(t, err)
	requireProblem(t, result, "/ingredients/0", ProblemInvalidField)
}

func TestCatalogImportService_Validate_PlateInsideAPlate(t *testing.T) {
	svc, _ := newTestCatalogImportService(t)
	doc := decodeImportDoc(t, `{`+importDefaults+`,
		"sellables": [{"ref": "coca", "name": "Coca-Cola", "category": "bebidas", "total_price_with_taxes": 4000}],
		"plates": [
			{"ref": "a", "name": "Bandeja", "category": "platos", "total_price_with_taxes": 30000},
			{"ref": "b", "name": "Combo", "category": "platos", "total_price_with_taxes": 35000,
			 "recipe": [{"ingredient": "a", "quantity": 1}, {"ingredient": "coca", "quantity": 1}]}
		]}`)

	result, err := svc.Validate(context.Background(), doc)

	require.NoError(t, err)
	requireProblem(t, result, "/plates/1/recipe/0", ProblemInvalidLinkType)
	requireProblem(t, result, "/plates/1/recipe/1", ProblemInvalidLinkType)
}

func TestCatalogImportService_Validate_ExistingLinkMustBeAnIngredient(t *testing.T) {
	svc, _ := newTestCatalogImportService(t,
		catalogProduct("01900000-0000-7000-8000-0000000000b1", "Bandeja", "COC-BAN", dto.ProductTypeComposite),
		catalogProduct("01900000-0000-7000-8000-0000000000b2", "Papa", "ING-PAP", dto.ProductTypeBoth))
	doc := decodeImportDoc(t, `{`+importDefaults+`,
		"existing": [{"ref": "bandeja", "sku": "COC-BAN"}, {"ref": "papa", "sku": "ING-PAP"}],
		"plates": [{"ref": "b", "name": "Combo", "category": "platos", "total_price_with_taxes": 35000,
			 "recipe": [{"ingredient": "bandeja", "quantity": 1}, {"ingredient": "papa", "quantity": 1}]}]}`)

	result, err := svc.Validate(context.Background(), doc)

	require.NoError(t, err)
	requireProblem(t, result, "/plates/0/recipe/0", ProblemInvalidLinkType)
	assert.Len(t, result.Problems, 1)
}

func TestCatalogImportService_Validate_RecipeQuantityMustBePositive(t *testing.T) {
	svc, _ := newTestCatalogImportService(t)
	doc := decodeImportDoc(t, `{`+importDefaults+`,
		"ingredients": [{"ref": "a", "name": "Arroz", "category": "insumos"}, {"ref": "b", "name": "Sal", "category": "insumos"}],
		"plates": [{"ref": "p", "name": "Arroz", "category": "platos", "total_price_with_taxes": 3000,
			"recipe": [{"ingredient": "a", "quantity": 0}, {"ingredient": "b", "quantity": 0.5}, {"ingredient": "b", "quantity": 1}]}]}`)

	result, err := svc.Validate(context.Background(), doc)

	require.NoError(t, err)
	requireProblem(t, result, "/plates/0/recipe/0", ProblemInvalidQuantity)
	requireProblem(t, result, "/plates/0/recipe/2", ProblemDuplicateItem)
	requireProblem(t, result, "/plates/0", ProblemNameConflict)
	assert.Len(t, result.Problems, 3, "a fractional recipe quantity is fine: %+v", result.Problems)
}

func TestCatalogImportService_Validate_FractionalChoiceQuantity(t *testing.T) {
	svc, _ := newTestCatalogImportService(t)
	doc := decodeImportDoc(t, `{`+importDefaults+`,
		"ingredients": [{"ref": "a", "name": "Arroz", "category": "insumos"}],
		"plates": [{"ref": "p", "name": "Tilapia", "category": "platos", "total_price_with_taxes": 30000,
			"slots": [{"name": "Arroz", "choices": [{"name": "Blanco", "default": true, "items": [{"ingredient": "a", "quantity": 0.5}]}]}]}]}`)

	result, err := svc.Validate(context.Background(), doc)

	require.NoError(t, err)
	requireProblem(t, result, "/plates/0/slots/0/choices/0/items/0", ProblemInvalidQuantity)
	assert.Len(t, result.Problems, 1)
}

func TestCatalogImportService_Validate_SlotDefaults(t *testing.T) {
	tests := []struct {
		name     string
		choices  string
		wantCode string
	}{
		{"no default", `[{"name": "Blanco"}, {"name": "Coco"}]`, ProblemSlotDefaultCount},
		{"two defaults", `[{"name": "Blanco", "default": true}, {"name": "Coco", "default": true}]`, ProblemSlotDefaultCount},
		{"no choices", `[]`, ProblemInvalidSlot},
		{"empty choice allowed", `[{"name": "Sin arroz", "default": true}]`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _ := newTestCatalogImportService(t)
			doc := decodeImportDoc(t, `{`+importDefaults+`,
				"plates": [{"ref": "p", "name": "Tilapia", "category": "platos", "total_price_with_taxes": 30000,
					"slots": [{"name": "Arroz", "choices": `+tt.choices+`}]}]}`)

			result, err := svc.Validate(context.Background(), doc)

			require.NoError(t, err)
			if tt.wantCode == "" {
				assert.True(t, result.Valid, "%+v", result.Problems)
				return
			}
			requireProblem(t, result, "/plates/0/slots/0", tt.wantCode)
			assert.Len(t, result.Problems, 1)
		})
	}
}

func TestCatalogImportService_Validate_SlotNamesAndItems(t *testing.T) {
	svc, _ := newTestCatalogImportService(t)
	doc := decodeImportDoc(t, `{`+importDefaults+`,
		"ingredients": [{"ref": "a", "name": "Arroz", "category": "insumos"}],
		"plates": [{"ref": "p", "name": "Tilapia", "category": "platos", "total_price_with_taxes": 30000,
			"slots": [
				{"name": " ", "choices": [{"name": "Blanco", "default": true}]},
				{"name": "Arroz", "choices": [{"name": "", "default": true}, {"name": "Doble", "items": [{"ingredient": "a", "quantity": 1}, {"ingredient": "a", "quantity": 2}]}]},
				{"name": "arroz", "choices": [{"name": "X", "default": true}]}
			]}]}`)

	result, err := svc.Validate(context.Background(), doc)

	require.NoError(t, err)
	requireProblem(t, result, "/plates/0/slots/0", ProblemInvalidSlot)
	requireProblem(t, result, "/plates/0/slots/1/choices/0", ProblemInvalidSlot)
	requireProblem(t, result, "/plates/0/slots/1/choices/1/items/1", ProblemDuplicateItem)
	requireProblem(t, result, "/plates/0/slots/2", ProblemInvalidSlot)
}

func TestCatalogImportService_Validate_RequiredValueMissingEverywhere(t *testing.T) {
	svc, _ := newTestCatalogImportService(t)
	doc := decodeImportDoc(t, `{"ingredients": [{"ref": "a", "name": "Arroz", "category": "insumos"}]}`)

	result, err := svc.Validate(context.Background(), doc)

	require.NoError(t, err)
	problem := requireProblem(t, result, "/ingredients/0", ProblemMissingField)
	assert.Contains(t, problem.Message, "unit_of_measure")
}

func TestCatalogImportService_Validate_SellableFieldsMissingEverywhere(t *testing.T) {
	svc, _ := newTestCatalogImportService(t)
	doc := decodeImportDoc(t, `{"sellables": [{"ref": "a", "name": "Limonada", "category": "bebidas"}]}`)

	result, err := svc.Validate(context.Background(), doc)

	require.NoError(t, err)
	var messages []string
	for _, p := range result.Problems {
		assert.Equal(t, ProblemMissingField, p.Code)
		assert.Equal(t, "/sellables/0", p.Path)
		messages = append(messages, p.Message)
	}
	joined := strings.Join(messages, "\n")
	for _, field := range []string{"unit_of_measure", "total_price_with_taxes", "vat", "ico", "taxes_format", "prep.area", "prep.priority"} {
		assert.Contains(t, joined, field)
	}
}

func TestCatalogImportService_Validate_InvalidUnitAndIngredientFields(t *testing.T) {
	svc, _ := newTestCatalogImportService(t)
	doc := decodeImportDoc(t, `{`+importDefaults+`,
		"ingredients": [
			{"ref": "a", "name": "Arroz", "category": "insumos", "unit_of_measure": "lb"},
			{"ref": "b", "name": "Sal", "category": "insumos", "total_price_with_taxes": 1000},
			{"ref": "c", "name": "Papa", "category": "insumos", "product_type": "COMPOSITE"},
			{"ref": "d", "name": "Papa criolla", "category": "insumos", "product_type": "BOTH", "total_price_with_taxes": 5000}
		]}`)

	result, err := svc.Validate(context.Background(), doc)

	require.NoError(t, err)
	requireProblem(t, result, "/ingredients/0", ProblemInvalidUnit)
	requireProblem(t, result, "/ingredients/1", ProblemInvalidField)
	requireProblem(t, result, "/ingredients/2", ProblemInvalidField)
	assert.Len(t, result.Problems, 3, "BOTH with a price is fine: %+v", result.Problems)
}

func TestCatalogImportService_Validate_DomainProductRulesApply(t *testing.T) {
	svc, _ := newTestCatalogImportService(t)
	doc := decodeImportDoc(t, `{`+importDefaults+`,
		"sellables": [
			{"ref": "a", "name": "Gratis", "category": "bebidas", "total_price_with_taxes": 0},
			{"ref": "b", "name": "Sin impuestos", "category": "bebidas", "total_price_with_taxes": 1000, "vat": 0, "ico": 0}
		]}`)

	result, err := svc.Validate(context.Background(), doc)

	require.NoError(t, err)
	requireProblem(t, result, "/sellables/0", ProblemInvalidField)
	requireProblem(t, result, "/sellables/1", ProblemInvalidField)
}

func TestCatalogImportService_Validate_GeneratedSKUAvoidsCollisions(t *testing.T) {
	svc, _ := newTestCatalogImportService(t,
		catalogProduct("01900000-0000-7000-8000-0000000000c1", "Arroz integral viejo", "ING-ARR-INT", dto.ProductTypeIngredient))
	doc := decodeImportDoc(t, `{"defaults": {"unit_of_measure": "g"},
		"ingredients": [{"ref": "a", "name": "Arroz integral", "category": "insumos"}, {"ref": "b", "name": "Arroz de coco", "category": "insumos"}]}`)

	result, err := svc.Validate(context.Background(), doc)

	require.NoError(t, err)
	assert.True(t, result.Valid, "%+v", result.Problems)
	assert.Equal(t, []dto.CatalogImportGeneratedSKU{{Ref: "a", SKU: "ING-ARR-INT-2"}, {Ref: "b", SKU: "ING-ARR-COC"}}, result.Summary.GeneratedSKUs)
}

func TestCatalogImportService_Validate_GeneratedSKUNeverTakesASKUGivenLater(t *testing.T) {
	svc, _ := newTestCatalogImportService(t)
	doc := decodeImportDoc(t, `{`+importDefaults+`,
		"ingredients": [
			{"ref": "a", "name": "Arroz integral", "category": "insumos"},
			{"ref": "b", "name": "Arroz importado", "category": "insumos", "sku": "ING-ARR-INT"}
		]}`)

	result, err := svc.Validate(context.Background(), doc)

	require.NoError(t, err)
	assert.True(t, result.Valid, "%+v", result.Problems)
	assert.Equal(t, []dto.CatalogImportGeneratedSKU{{Ref: "a", SKU: "ING-ARR-INT-2"}}, result.Summary.GeneratedSKUs)
}

func TestCatalogImportService_Validate_OpeningStock(t *testing.T) {
	svc, _ := newTestCatalogImportService(t,
		catalogProduct("01900000-0000-7000-8000-0000000000a1", "Limón", "ING-LIM", dto.ProductTypeIngredient))
	doc := decodeImportDoc(t, `{`+importDefaults+`,
		"existing": [{"ref": "limon", "sku": "ING-LIM"}],
		"ingredients": [{"ref": "a", "name": "Arroz", "category": "insumos"}, {"ref": "b", "name": "Sal", "category": "insumos"}],
		"opening_stock": [
			{"product": "limon", "counted": 10},
			{"product": "a", "counted": -1},
			{"product": "b", "counted": 2.5},
			{"product": "nada", "counted": 1},
			{"product": "a", "counted": 3}
		]}`)

	result, err := svc.Validate(context.Background(), doc)

	require.NoError(t, err)
	requireProblem(t, result, "/opening_stock/0", ProblemStockOnExisting)
	requireProblem(t, result, "/opening_stock/1", ProblemInvalidQuantity)
	requireProblem(t, result, "/opening_stock/2", ProblemInvalidQuantity)
	requireProblem(t, result, "/opening_stock/3", ProblemUnknownRef)
	requireProblem(t, result, "/opening_stock/4", ProblemDuplicateItem)
	assert.Len(t, result.Problems, 5)
}

func manyIngredients(n int) string {
	entries := make([]string, n)
	for i := range entries {
		entries[i] = fmt.Sprintf(`{"ref": "i%d", "name": "Insumo %d", "category": "insumos"}`, i, i)
	}
	return strings.Join(entries, ",")
}

func TestCatalogImportService_Validate_TooManyProducts(t *testing.T) {
	svc, _ := newTestCatalogImportService(t)
	doc := decodeImportDoc(t, `{`+importDefaults+`, "ingredients": [`+manyIngredients(CatalogImportMaxProducts+1)+`]}`)

	result, err := svc.Validate(context.Background(), doc)

	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.Equal(t, []string{ProblemLimitExceeded}, problemCodes(result))
}

func TestCatalogImportService_Validate_ThousandProductsIsAllowed(t *testing.T) {
	svc, _ := newTestCatalogImportService(t)
	doc := decodeImportDoc(t, `{`+importDefaults+`, "ingredients": [`+manyIngredients(CatalogImportMaxProducts)+`]}`)

	result, err := svc.Validate(context.Background(), doc)

	require.NoError(t, err)
	assert.True(t, result.Valid, "%+v", result.Problems)
}

func repeatJSON(n int, item func(i int) string) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = item(i)
	}
	return strings.Join(parts, ",")
}

func TestCatalogImportService_Validate_NestedLimits(t *testing.T) {
	plate := func(recipe, slots string) string {
		return `{"plates": [{"ref": "p", "name": "Plato", "category": "platos", "total_price_with_taxes": 1000, "recipe": [` + recipe + `], "slots": [` + slots + `]}]}`
	}
	line := func(i int) string { return fmt.Sprintf(`{"ingredient": "i%d", "quantity": 1}`, i) }
	choice := func(i int) string { return fmt.Sprintf(`{"name": "c%d"}`, i) }
	slot := func(i int) string {
		return fmt.Sprintf(`{"name": "s%d", "choices": [{"name": "x", "default": true}]}`, i)
	}

	tests := []struct {
		name string
		doc  string
		path string
	}{
		{"recipe lines", plate(repeatJSON(CatalogImportMaxRecipeLines+1, line), ""), "/plates/0/recipe"},
		{"slots", plate("", repeatJSON(CatalogImportMaxSlots+1, slot)), "/plates/0/slots"},
		{"choices", plate("", `{"name": "s", "choices": [`+repeatJSON(CatalogImportMaxChoices+1, choice)+`]}`), "/plates/0/slots/0/choices"},
		{"items", plate("", `{"name": "s", "choices": [{"name": "x", "default": true, "items": [`+repeatJSON(CatalogImportMaxChoiceItems+1, line)+`]}]}`), "/plates/0/slots/0/choices/0/items"},
		{"opening stock", `{"opening_stock": [` + repeatJSON(CatalogImportMaxOpeningStock+1, func(i int) string { return fmt.Sprintf(`{"product": "i%d", "counted": 1}`, i) }) + `]}`, "/opening_stock"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _ := newTestCatalogImportService(t)

			result, err := svc.Validate(context.Background(), decodeImportDoc(t, tt.doc))

			require.NoError(t, err)
			requireProblem(t, result, tt.path, ProblemLimitExceeded)
			assert.Equal(t, []string{ProblemLimitExceeded}, problemCodes(result))
		})
	}
}

func TestCatalogImportService_Digest_IgnoresKeyOrderAndFoldsDefaults(t *testing.T) {
	a := decodeImportDoc(t, `{"defaults": {"unit_of_measure": "g"}, "ingredients": [{"ref": "a", "name": "Arroz", "category": "insumos"}]}`)
	b := decodeImportDoc(t, `{"ingredients": [{"category": "insumos", "name": "Arroz", "ref": "a"}], "defaults": {"unit_of_measure": "g"}}`)
	c := decodeImportDoc(t, `{"ingredients": [{"ref": "a", "name": "Arroz", "category": "insumos", "unit_of_measure": "g", "product_type": "INGREDIENT"}]}`)

	da, err := catalogImportDigest(a)
	require.NoError(t, err)
	db, err := catalogImportDigest(b)
	require.NoError(t, err)
	dc, err := catalogImportDigest(c)
	require.NoError(t, err)

	assert.Equal(t, da, db)
	assert.Equal(t, da, dc)
}

func TestCatalogImportService_Digest_ChangesWithThePrice(t *testing.T) {
	a := decodeImportDoc(t, fullImportDoc)
	b := decodeImportDoc(t, strings.Replace(fullImportDoc, `"total_price_with_taxes": 32000`, `"total_price_with_taxes": 33000`, 1))

	da, err := catalogImportDigest(a)
	require.NoError(t, err)
	db, err := catalogImportDigest(b)
	require.NoError(t, err)

	assert.NotEqual(t, da, db)
}

func validatedApply(t *testing.T, svc *CatalogImportService, raw string) *dto.ApplyCatalogImportRequest {
	t.Helper()
	doc := decodeImportDoc(t, raw)
	validation, err := svc.Validate(context.Background(), doc)
	require.NoError(t, err)
	return &dto.ApplyCatalogImportRequest{CatalogImportDocument: *doc, ExpectedDigest: validation.Digest}
}

func TestCatalogImportService_Apply_SuccessfulApply(t *testing.T) {
	svc, m := newTestCatalogImportService(t)
	req := validatedApply(t, svc, fullImportDoc)

	var created []*product.Aggregate
	m.products.EXPECT().CreateMany(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, aggs []*product.Aggregate) error {
		created = aggs
		return nil
	}).Once()
	var recipe []*dto.ProductIngredient
	m.ingredients.EXPECT().CreateMany(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, lines []*dto.ProductIngredient) error {
		recipe = lines
		return nil
	}).Once()
	var slots []dto.PlateOptionSlot
	var slotPlateID string
	m.slots.EXPECT().ReplaceForCompositeProduct(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, id string, s []dto.PlateOptionSlot) error {
		slotPlateID, slots = id, s
		return nil
	}).Once()
	var responsibilities []*dto.ProductPreparationResponsibility
	m.products.EXPECT().CreatePreparationResponsibilities(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, r []*dto.ProductPreparationResponsibility) error {
		responsibilities = r
		return nil
	}).Once()
	var stocks []*dto.Stock
	m.stock.EXPECT().BulkCreateOrUpdate(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, s []*dto.Stock) error {
		stocks = s
		return nil
	}).Once()
	var movement *dto.HistoricStock
	m.stock.EXPECT().CreateHistoricRecord(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, h *dto.HistoricStock) error {
		movement = h
		return nil
	}).Once()
	m.emitter.EXPECT().Emit(mock.Anything, mock.Anything).Return(nil).Once()

	result, err := svc.Apply(context.Background(), req)

	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.True(t, result.Applied)
	require.Len(t, result.Products, 6)

	byRef := make(map[string]*dto.Product, len(created))
	require.Len(t, created, 6)
	names := make([]string, len(created))
	for i, agg := range created {
		p := agg.ToDTO()
		names[i] = p.Name
		for ref, out := range result.Products {
			if out.ID == p.ID {
				byRef[ref] = p
				assert.Equal(t, p.SKU, out.SKU)
			}
		}
	}
	assert.Equal(t, []string{"Arroz blanco", "Frijol", "Arroz de coco", "Coca-Cola 400ml", "Limonada", "Bandeja paisa"}, names, "ingredients first, then sellables, then plates")
	require.Len(t, byRef, 6)
	assert.Equal(t, dto.ProductTypeIngredient, byRef["arroz"].ProductType)
	assert.Equal(t, dto.ProductTypeSellable, byRef["coca"].ProductType)
	assert.Equal(t, dto.ProductTypeComposite, byRef["bandeja"].ProductType)
	assert.Equal(t, "BAR-LIM", byRef["limonada"].SKU)
	assert.True(t, byRef["coca"].ICO.Equal(decimal.NewFromFloat(0.08)), "default ICO 8 applied, got %s", byRef["coca"].ICO)
	assert.Equal(t, dto.UnitOfMeasureGram, byRef["arroz"].UnitOfMeasure)

	require.Len(t, recipe, 1)
	assert.Equal(t, byRef["bandeja"].ID, recipe[0].CompositeProductID)
	assert.Equal(t, byRef["frijol"].ID, recipe[0].IngredientProductID)
	assert.True(t, recipe[0].DefaultQuantity.Equal(decimal.NewFromInt(150)))

	assert.Equal(t, byRef["bandeja"].ID, slotPlateID)
	require.Len(t, slots, 1)
	assert.Equal(t, "Arroz", slots[0].Name)
	require.Len(t, slots[0].Choices, 3)
	assert.True(t, slots[0].Choices[0].IsDefault)
	assert.Equal(t, byRef["arroz"].ID, slots[0].Choices[0].Items[0].IngredientProductID)
	assert.Equal(t, 200, slots[0].Choices[0].Items[0].Quantity)
	assert.Equal(t, byRef["coco"].ID, slots[0].Choices[1].Items[0].IngredientProductID)
	assert.Empty(t, slots[0].Choices[2].Items)

	areas := make(map[string]string, len(responsibilities))
	for _, r := range responsibilities {
		areas[r.ProductID] = r.Area
	}
	assert.Equal(t, map[string]string{byRef["coca"].ID: "bar", byRef["limonada"].ID: "kitchen", byRef["bandeja"].ID: "kitchen"}, areas)

	require.Len(t, stocks, 1)
	assert.Equal(t, byRef["arroz"].ID, stocks[0].ProductID)
	assert.Equal(t, 25000, stocks[0].Amount)
	require.NotNil(t, movement)
	assert.Equal(t, 25000, movement.Change)
	assert.Equal(t, dto.StockMovementKindCount, movement.Kind)
	assert.NotEmpty(t, movement.OpID)
	assert.False(t, m.rolledBack)
}

func TestCatalogImportService_Apply_PlateDefinedBeforeItsIngredientLinksTheCreatedIngredient(t *testing.T) {
	svc, m := newTestCatalogImportService(t)
	req := validatedApply(t, svc, `{"plates": [{"ref": "p", "name": "Frijolada", "category": "platos", "total_price_with_taxes": 15000,
			"recipe": [{"ingredient": "frijol", "quantity": 150}]}],
		"ingredients": [{"ref": "frijol", "name": "Frijol", "category": "insumos"}],
		`+importDefaults+`}`)

	m.products.EXPECT().CreateMany(mock.Anything, mock.Anything).Return(nil).Once()
	var recipe []*dto.ProductIngredient
	m.ingredients.EXPECT().CreateMany(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, lines []*dto.ProductIngredient) error {
		recipe = lines
		return nil
	}).Once()
	m.products.EXPECT().CreatePreparationResponsibilities(mock.Anything, mock.Anything).Return(nil).Once()

	result, err := svc.Apply(context.Background(), req)

	require.NoError(t, err)
	require.Len(t, recipe, 1)
	assert.Equal(t, result.Products["frijol"].ID, recipe[0].IngredientProductID)
	assert.Equal(t, result.Products["p"].ID, recipe[0].CompositeProductID)
}

func TestCatalogImportService_Apply_ReusesAnExistingIngredientWithoutChangingIt(t *testing.T) {
	limon := catalogProduct("01900000-0000-7000-8000-0000000000a1", "Limón", "ING-LIM", dto.ProductTypeIngredient)
	svc, m := newTestCatalogImportService(t, limon)
	req := validatedApply(t, svc, `{`+importDefaults+`,
		"existing": [{"ref": "limon", "name": "limón "}],
		"plates": [{"ref": "p", "name": "Pescado al limón", "category": "platos", "total_price_with_taxes": 30000,
			"recipe": [{"ingredient": "limon", "quantity": 1}]}]}`)

	var created []*product.Aggregate
	m.products.EXPECT().CreateMany(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, aggs []*product.Aggregate) error {
		created = aggs
		return nil
	}).Once()
	var recipe []*dto.ProductIngredient
	m.ingredients.EXPECT().CreateMany(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, lines []*dto.ProductIngredient) error {
		recipe = lines
		return nil
	}).Once()
	m.products.EXPECT().CreatePreparationResponsibilities(mock.Anything, mock.Anything).Return(nil).Once()

	result, err := svc.Apply(context.Background(), req)

	require.NoError(t, err)
	require.Len(t, created, 1)
	assert.Equal(t, "Pescado al limón", created[0].ToDTO().Name)
	require.Len(t, recipe, 1)
	assert.Equal(t, limon.ID, recipe[0].IngredientProductID)
	assert.NotContains(t, result.Products, "limon")
	// The strict product mock fails the test on any Update call.
}

func TestCatalogImportService_Apply_WithAProblemWritesNothing(t *testing.T) {
	svc, m := newTestCatalogImportService(t)
	raw := `{` + importDefaults + `,
		"ingredients": [{"ref": "a", "name": "Arroz", "category": "insumos"}],
		"plates": [
			{"ref": "p1", "name": "Mojarra", "category": "platos", "total_price_with_taxes": 30000},
			{"ref": "p2", "name": "Tilapia", "category": "platos", "total_price_with_taxes": 30000,
			 "slots": [{"name": "Arroz", "choices": [{"name": "Blanco", "default": true}, {"name": "Coco", "default": true}]}]}
		]}`
	doc := decodeImportDoc(t, raw)
	digest, err := catalogImportDigest(doc)
	require.NoError(t, err)

	result, err := svc.Apply(context.Background(), &dto.ApplyCatalogImportRequest{CatalogImportDocument: *doc, ExpectedDigest: digest})

	require.ErrorIs(t, err, domainError.ErrCatalogImportInvalid)
	require.NotNil(t, result)
	assert.False(t, result.Applied)
	requireProblem(t, result, "/plates/1/slots/0", ProblemSlotDefaultCount)
	assert.True(t, m.rolledBack)
	m.products.AssertNotCalled(t, "CreateMany", mock.Anything, mock.Anything)
}

func TestCatalogImportService_Apply_DigestMismatch(t *testing.T) {
	svc, m := newTestCatalogImportService(t)
	req := validatedApply(t, svc, fullImportDoc)
	changed := decodeImportDoc(t, strings.Replace(fullImportDoc, `"total_price_with_taxes": 32000`, `"total_price_with_taxes": 3200`, 1))
	req.CatalogImportDocument = *changed

	result, err := svc.Apply(context.Background(), req)

	require.ErrorIs(t, err, domainError.ErrCatalogImportDigestMismatch)
	require.NotNil(t, result)
	assert.Equal(t, []string{ProblemDigestMismatch}, problemCodes(result))
	assert.False(t, result.Applied)
	m.products.AssertNotCalled(t, "CreateMany", mock.Anything, mock.Anything)
}

func TestCatalogImportService_Apply_MissingDigestIsAMismatch(t *testing.T) {
	svc, _ := newTestCatalogImportService(t)

	_, err := svc.Apply(context.Background(), &dto.ApplyCatalogImportRequest{CatalogImportDocument: *decodeImportDoc(t, fullImportDoc)})

	require.ErrorIs(t, err, domainError.ErrCatalogImportDigestMismatch)
}

func TestCatalogImportService_Apply_RepeatedApplyDoesNotDuplicate(t *testing.T) {
	doc := decodeImportDoc(t, fullImportDoc)
	var catalog []*dto.Product
	for _, f := range []dto.CatalogImportProductFields{
		doc.Ingredients[0].CatalogImportProductFields, doc.Ingredients[1].CatalogImportProductFields, doc.Ingredients[2].CatalogImportProductFields,
		doc.Sellables[0].CatalogImportProductFields, doc.Sellables[1].CatalogImportProductFields, doc.Plates[0].CatalogImportProductFields,
	} {
		catalog = append(catalog, catalogProduct("01900000-0000-7000-8000-"+fmt.Sprintf("%012d", len(catalog)), f.Name, "X-"+f.Ref, dto.ProductTypeIngredient))
	}
	svc, _ := newTestCatalogImportService(t, catalog...)
	digest, err := catalogImportDigest(doc)
	require.NoError(t, err)

	result, err := svc.Apply(context.Background(), &dto.ApplyCatalogImportRequest{CatalogImportDocument: *doc, ExpectedDigest: digest})

	require.ErrorIs(t, err, domainError.ErrCatalogImportInvalid)
	for _, path := range []string{"/ingredients/0", "/ingredients/1", "/ingredients/2", "/sellables/0", "/sellables/1", "/plates/0"} {
		requireProblem(t, result, path, ProblemNameConflict)
	}
}

func TestCatalogImportService_Apply_WriteFailingHalfwayRollsBack(t *testing.T) {
	svc, m := newTestCatalogImportService(t)
	req := validatedApply(t, svc, fullImportDoc)
	m.products.EXPECT().CreateMany(mock.Anything, mock.Anything).Return(nil).Once()
	m.ingredients.EXPECT().CreateMany(mock.Anything, mock.Anything).Return(errors.New("connection reset")).Once()

	result, err := svc.Apply(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, m.rolledBack, "the failure is returned through the unit of work, so the products roll back")
	m.slots.AssertNotCalled(t, "ReplaceForCompositeProduct", mock.Anything, mock.Anything, mock.Anything)
}

func TestCatalogImportService_Validate_CatalogLoadFails(t *testing.T) {
	m := mocks.NewMockProductRepository(t)
	m.EXPECT().FindAll(mock.Anything, dto.ListProductsRequest{}).Return(nil, errors.New("db down"))
	svc := NewCatalogImportService(m, nil, nil, nil, nil, nil)

	_, err := svc.Validate(context.Background(), &dto.CatalogImportDocument{})

	require.Error(t, err)
}

func TestCatalogImportService_APIDocNamesEveryProblemCode(t *testing.T) {
	doc, err := os.ReadFile("../../../docs/api/catalog-import.md")
	require.NoError(t, err)
	for _, code := range CatalogImportProblemCodes {
		assert.True(t, bytes.Contains(doc, []byte("`"+code+"`")), "docs/api/catalog-import.md does not name %s", code)
	}
}

func TestCatalogImportService_HugeExponentsAreRefusedBeforeAnyWork(t *testing.T) {
	// No FindAll and no digest: the strict mocks fail the test if the catalog is read.
	m := mocks.NewMockProductRepository(t)
	svc := NewCatalogImportService(m, nil, nil, nil, nil, nil)
	doc := decodeImportDoc(t, `{
		"defaults": {"ico": 1e2000000000},
		"sellables": [{"ref": "a", "name": "A", "category": "c", "total_price_with_taxes": 123456789012345678901234567890123}],
		"plates": [{"ref": "p", "name": "P", "category": "c", "recipe": [{"ingredient": "x", "quantity": 1e-2000000000}],
			"slots": [{"name": "s", "choices": [{"name": "c", "default": true, "items": [{"ingredient": "x", "quantity": 1e999999999}]}]}]}],
		"opening_stock": [{"product": "a", "counted": 1e2000000000}]
	}`)

	result, err := svc.Validate(context.Background(), doc)

	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.Empty(t, result.Digest)
	for _, path := range []string{"/defaults", "/sellables/0", "/plates/0/recipe/0", "/plates/0/slots/0/choices/0/items/0", "/opening_stock/0"} {
		assert.True(t, slices.ContainsFunc(result.Problems, func(p dto.CatalogImportProblem) bool { return p.Path == path }), path)
	}

	_, err = svc.Apply(context.Background(), &dto.ApplyCatalogImportRequest{CatalogImportDocument: *doc, ExpectedDigest: "x"})
	require.ErrorIs(t, err, domainError.ErrCatalogImportInvalid)
}

func TestCatalogImportService_OrdinaryNumbersPassTheMagnitudeCheck(t *testing.T) {
	assert.Nil(t, refuseOversizedDecimals(decodeImportDoc(t, fullImportDoc)))
	assert.Nil(t, refuseOversizedDecimals(decodeImportDoc(t, `{"sellables": [{"ref": "a", "name": "A", "category": "c", "total_price_with_taxes": 99999999999.99, "vat": 19.5, "ico": 0.001}]}`)))
}
