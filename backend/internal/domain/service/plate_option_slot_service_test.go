package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	optTilapiaID      = "01900000-0000-7000-8000-000000000001"
	optArrozBlancoID  = "01900000-0000-7000-8000-000000000011"
	optArrozCocoID    = "01900000-0000-7000-8000-000000000012"
	optCanastaID      = "01900000-0000-7000-8000-000000000013"
	optPapasID        = "01900000-0000-7000-8000-000000000014"
	optPataconID      = "01900000-0000-7000-8000-000000000015"
	optEnsaladaID     = "01900000-0000-7000-8000-000000000016"
	optForeignProduct = "01900000-0000-7000-8000-000000000099"
)

type slotServiceMocks struct {
	slots    *mocks.MockPlateOptionSlotRepository
	products *mocks.MockProductRepository
}

func newTestPlateOptionSlotService(t *testing.T) (*PlateOptionSlotService, slotServiceMocks) {
	m := slotServiceMocks{
		slots:    mocks.NewMockPlateOptionSlotRepository(t),
		products: mocks.NewMockProductRepository(t),
	}
	return NewPlateOptionSlotService(m.slots, m.products), m
}

func tilapiaProduct() *dto.Product {
	return createTestProductWithType(optTilapiaID, "Tilapia", "platos", 1, 30000, 0, dto.ProductTypeComposite)
}

func bundleItem(ingredientID string, qty int) dto.PlateOptionChoiceItemInput {
	return dto.PlateOptionChoiceItemInput{IngredientProductID: ingredientID, Quantity: qty}
}

func tilapiaSlotsRequest() *dto.ReplacePlateOptionSlotsRequest {
	return &dto.ReplacePlateOptionSlotsRequest{Slots: []dto.PlateOptionSlotInput{
		{Name: "Arroz", Choices: []dto.PlateOptionChoiceInput{
			{Name: "Arroz blanco", IsDefault: true, Items: []dto.PlateOptionChoiceItemInput{bundleItem(optArrozBlancoID, 1)}},
			{Name: "Arroz de coco", Items: []dto.PlateOptionChoiceItemInput{bundleItem(optArrozCocoID, 1)}},
		}},
		{Name: "Acompañante", Choices: []dto.PlateOptionChoiceInput{
			{Name: "Canastas", IsDefault: true, Items: []dto.PlateOptionChoiceItemInput{bundleItem(optCanastaID, 2)}},
			{Name: "Papas", Items: []dto.PlateOptionChoiceItemInput{bundleItem(optPapasID, 1)}},
			{Name: "Patacón", Items: []dto.PlateOptionChoiceItemInput{bundleItem(optPataconID, 1)}},
		}},
		{Name: "Ensalada", Choices: []dto.PlateOptionChoiceInput{
			{Name: "Ensalada", IsDefault: true, Items: []dto.PlateOptionChoiceItemInput{bundleItem(optEnsaladaID, 1)}},
			{Name: "Canasta extra", Items: []dto.PlateOptionChoiceItemInput{bundleItem(optCanastaID, 1)}},
		}},
	}}
}

func bundleProducts(ids ...string) []*dto.Product {
	out := make([]*dto.Product, len(ids))
	for i, id := range ids {
		out[i] = createTestProductWithType(id, id, "insumos", 1, 0, 0, dto.ProductTypeIngredient)
	}
	return out
}

func expectBundleLookup(m slotServiceMocks, ids ...string) {
	m.products.On("FindByIDs", mock.Anything, mock.MatchedBy(func(got []string) bool {
		return assert.ObjectsAreEqual(len(ids), len(got))
	})).Return(bundleProducts(ids...), nil)
}

func TestPlateOptionSlotService_Replace_ConfiguresTheTilapia(t *testing.T) {
	ctx := context.Background()
	svc, m := newTestPlateOptionSlotService(t)

	m.products.On("FindByID", ctx, optTilapiaID).Return(tilapiaProduct(), nil)
	expectBundleLookup(m, optArrozBlancoID, optArrozCocoID, optCanastaID, optPapasID, optPataconID, optEnsaladaID)
	m.slots.On("FindByCompositeProductID", ctx, optTilapiaID).Return(nil, nil).Once()

	var written []dto.PlateOptionSlot
	m.slots.On("ReplaceForCompositeProduct", ctx, optTilapiaID, mock.Anything).
		Run(func(args mock.Arguments) { written, _ = args.Get(2).([]dto.PlateOptionSlot) }).
		Return(nil)
	m.slots.On("FindByCompositeProductID", ctx, optTilapiaID).
		Return(func(context.Context, string) []dto.PlateOptionSlot { return written }, nil).Once()

	got, err := svc.Replace(ctx, optTilapiaID, tilapiaSlotsRequest())

	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, []string{"Arroz", "Acompañante", "Ensalada"}, []string{got[0].Name, got[1].Name, got[2].Name})
	for i, slot := range got {
		assert.Equal(t, i, slot.Position)
		assert.Equal(t, optTilapiaID, slot.CompositeProductID)
		assert.NotEmpty(t, slot.ID)
	}
	acompanante := got[1]
	require.Len(t, acompanante.Choices, 3)
	assert.Equal(t, "Canastas", acompanante.Choices[0].Name)
	assert.True(t, acompanante.Choices[0].IsDefault)
	assert.False(t, acompanante.Choices[1].IsDefault)
	assert.Equal(t, []dto.PlateOptionChoiceItem{{
		ID: acompanante.Choices[0].Items[0].ID, IngredientProductID: optCanastaID, Quantity: 2,
		CreatedAt: acompanante.Choices[0].Items[0].CreatedAt, UpdatedAt: acompanante.Choices[0].Items[0].UpdatedAt,
	}}, acompanante.Choices[0].Items)
}

func TestPlateOptionSlotService_Replace_KeepsIDsOfResentChoices(t *testing.T) {
	ctx := context.Background()
	svc, m := newTestPlateOptionSlotService(t)
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	existing := []dto.PlateOptionSlot{{
		ID: "slot-acomp", CompositeProductID: optTilapiaID, Name: "Acompañante", CreatedAt: created,
		Choices: []dto.PlateOptionChoice{
			{ID: "choice-canastas", Name: "Canastas", IsDefault: true, CreatedAt: created,
				Items: []dto.PlateOptionChoiceItem{{ID: "item-canasta", IngredientProductID: optCanastaID, Quantity: 2, CreatedAt: created}}},
			{ID: "choice-papas", Name: "Papas", CreatedAt: created},
			{ID: "choice-patacon", Name: "Patacón", CreatedAt: created},
		},
	}}
	req := &dto.ReplacePlateOptionSlotsRequest{Slots: []dto.PlateOptionSlotInput{{
		ID: "slot-acomp", Name: "Acompañante", Choices: []dto.PlateOptionChoiceInput{
			{ID: "choice-canastas", Name: "Canastas", IsDefault: true, Items: []dto.PlateOptionChoiceItemInput{bundleItem(optCanastaID, 2)}},
			{ID: "choice-papas", Name: "Papas", Items: []dto.PlateOptionChoiceItemInput{bundleItem(optPapasID, 1)}},
		},
	}}}

	m.products.On("FindByID", ctx, optTilapiaID).Return(tilapiaProduct(), nil)
	expectBundleLookup(m, optCanastaID, optPapasID)
	m.slots.On("FindByCompositeProductID", ctx, optTilapiaID).Return(existing, nil)

	var written []dto.PlateOptionSlot
	m.slots.On("ReplaceForCompositeProduct", ctx, optTilapiaID, mock.Anything).
		Run(func(args mock.Arguments) { written, _ = args.Get(2).([]dto.PlateOptionSlot) }).
		Return(nil)

	_, err := svc.Replace(ctx, optTilapiaID, req)

	require.NoError(t, err)
	require.Len(t, written, 1)
	assert.Equal(t, "slot-acomp", written[0].ID)
	assert.Equal(t, created, written[0].CreatedAt)
	require.Len(t, written[0].Choices, 2, "Patacón is left out, so the repository removes it")
	assert.Equal(t, "choice-canastas", written[0].Choices[0].ID)
	assert.Equal(t, "choice-papas", written[0].Choices[1].ID)
	assert.Equal(t, "item-canasta", written[0].Choices[0].Items[0].ID)
	assert.NotEqual(t, "", written[0].Choices[1].Items[0].ID)
}

func TestPlateOptionSlotService_Replace_UnknownIDsGetFreshOnes(t *testing.T) {
	ctx := context.Background()
	svc, m := newTestPlateOptionSlotService(t)
	req := &dto.ReplacePlateOptionSlotsRequest{Slots: []dto.PlateOptionSlotInput{{
		ID: "slot-of-another-plate", Name: "Arroz", Choices: []dto.PlateOptionChoiceInput{
			{ID: "choice-of-another-plate", Name: "Arroz blanco", IsDefault: true},
		},
	}}}

	m.products.On("FindByID", ctx, optTilapiaID).Return(tilapiaProduct(), nil)
	m.slots.On("FindByCompositeProductID", ctx, optTilapiaID).Return(nil, nil)
	var written []dto.PlateOptionSlot
	m.slots.On("ReplaceForCompositeProduct", ctx, optTilapiaID, mock.Anything).
		Run(func(args mock.Arguments) { written, _ = args.Get(2).([]dto.PlateOptionSlot) }).
		Return(nil)

	_, err := svc.Replace(ctx, optTilapiaID, req)

	require.NoError(t, err)
	assert.NotEqual(t, "slot-of-another-plate", written[0].ID)
	assert.NotEqual(t, "choice-of-another-plate", written[0].Choices[0].ID)
	assert.Empty(t, written[0].Choices[0].Items, "an empty bundle is a valid \"without\" choice")
}

func TestPlateOptionSlotService_Replace_EmptyListRemovesEverySlot(t *testing.T) {
	ctx := context.Background()
	svc, m := newTestPlateOptionSlotService(t)

	m.products.On("FindByID", ctx, optTilapiaID).Return(tilapiaProduct(), nil)
	m.slots.On("FindByCompositeProductID", ctx, optTilapiaID).Return([]dto.PlateOptionSlot{{ID: "slot-1"}}, nil).Once()
	m.slots.On("ReplaceForCompositeProduct", ctx, optTilapiaID, []dto.PlateOptionSlot{}).Return(nil)
	m.slots.On("FindByCompositeProductID", ctx, optTilapiaID).Return(nil, nil).Once()

	got, err := svc.Replace(ctx, optTilapiaID, &dto.ReplacePlateOptionSlotsRequest{})

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestPlateOptionSlotService_Replace_RejectsInvalidDocuments(t *testing.T) {
	choice := func(name string, isDefault bool, items ...dto.PlateOptionChoiceItemInput) dto.PlateOptionChoiceInput {
		return dto.PlateOptionChoiceInput{Name: name, IsDefault: isDefault, Items: items}
	}
	slot := func(name string, choices ...dto.PlateOptionChoiceInput) dto.PlateOptionSlotInput {
		return dto.PlateOptionSlotInput{Name: name, Choices: choices}
	}

	tests := []struct {
		name    string
		slots   []dto.PlateOptionSlotInput
		message string
	}{
		{"two defaults", []dto.PlateOptionSlotInput{slot("Arroz",
			choice("Arroz blanco", true, bundleItem(optArrozBlancoID, 1)), choice("Arroz de coco", true, bundleItem(optArrozCocoID, 1)))},
			`slot "Arroz" must have exactly one default choice, got 2`},
		{"no default", []dto.PlateOptionSlotInput{slot("Arroz",
			choice("Arroz blanco", false, bundleItem(optArrozBlancoID, 1)), choice("Arroz de coco", false, bundleItem(optArrozCocoID, 1)))},
			`slot "Arroz" must have exactly one default choice, got 0`},
		{"empty slot name", []dto.PlateOptionSlotInput{slot("  ", choice("Arroz blanco", true))}, "slot 1 has no name"},
		{"duplicate slot name", []dto.PlateOptionSlotInput{
			slot("Arroz", choice("Arroz blanco", true)), slot("arroz", choice("Arroz de coco", true))},
			`slot "arroz" appears twice`},
		{"no choices", []dto.PlateOptionSlotInput{slot("Arroz")}, `slot "Arroz" has no choices`},
		{"empty choice name", []dto.PlateOptionSlotInput{slot("Arroz", choice("", true))}, "has a choice with no name"},
		{"duplicate choice name", []dto.PlateOptionSlotInput{slot("Arroz", choice("Blanco", true), choice("Blanco", false))},
			`has the choice "Blanco" twice`},
		{"zero quantity", []dto.PlateOptionSlotInput{slot("Arroz", choice("Blanco", true, bundleItem(optArrozBlancoID, 0)))},
			"quantity of at least 1"},
		{"ingredient twice in a bundle", []dto.PlateOptionSlotInput{slot("Acompañante",
			choice("Canastas", true, bundleItem(optCanastaID, 1), bundleItem(optCanastaID, 1)))}, "lists an ingredient twice"},
		{"malformed ingredient id", []dto.PlateOptionSlotInput{slot("Arroz", choice("Blanco", true, bundleItem("rice", 1)))},
			"invalid ingredient id"},
		{"choice id sent twice", []dto.PlateOptionSlotInput{{Name: "Arroz", Choices: []dto.PlateOptionChoiceInput{
			{ID: "c1", Name: "Blanco", IsDefault: true}, {ID: "c1", Name: "Coco"}}}}, "choice id c1 is sent twice"},
		{"slot id sent twice", []dto.PlateOptionSlotInput{
			{ID: "s1", Name: "Arroz", Choices: []dto.PlateOptionChoiceInput{choice("Blanco", true)}},
			{ID: "s1", Name: "Ensalada", Choices: []dto.PlateOptionChoiceInput{choice("Ensalada", true)}}},
			"slot id s1 is sent twice"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			svc, m := newTestPlateOptionSlotService(t)
			m.products.On("FindByID", ctx, optTilapiaID).Return(tilapiaProduct(), nil)

			got, err := svc.Replace(ctx, optTilapiaID, &dto.ReplacePlateOptionSlotsRequest{Slots: tt.slots})

			require.ErrorIs(t, err, domainError.ErrInvalidPlateOptionSlots)
			assert.Contains(t, err.Error(), tt.message)
			assert.Nil(t, got)
			m.slots.AssertNotCalled(t, "ReplaceForCompositeProduct", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestPlateOptionSlotService_Replace_IngredientOfAnotherOrganization(t *testing.T) {
	ctx := context.Background()
	svc, m := newTestPlateOptionSlotService(t)
	req := &dto.ReplacePlateOptionSlotsRequest{Slots: []dto.PlateOptionSlotInput{{Name: "Arroz", Choices: []dto.PlateOptionChoiceInput{
		{Name: "Arroz blanco", IsDefault: true, Items: []dto.PlateOptionChoiceItemInput{bundleItem(optArrozBlancoID, 1)}},
		{Name: "Arroz ajeno", Items: []dto.PlateOptionChoiceItemInput{bundleItem(optForeignProduct, 1)}},
	}}}}

	m.products.On("FindByID", ctx, optTilapiaID).Return(tilapiaProduct(), nil)
	// The organization-scoped lookup never returns the other organization's product.
	m.products.On("FindByIDs", ctx, []string{optArrozBlancoID, optForeignProduct}).Return(bundleProducts(optArrozBlancoID), nil)

	got, err := svc.Replace(ctx, optTilapiaID, req)

	require.ErrorIs(t, err, domainError.ErrProductNotFound)
	assert.Nil(t, got)
	m.slots.AssertNotCalled(t, "ReplaceForCompositeProduct", mock.Anything, mock.Anything, mock.Anything)
}

func TestPlateOptionSlotService_Replace_PlateAsItsOwnIngredient(t *testing.T) {
	ctx := context.Background()
	svc, m := newTestPlateOptionSlotService(t)
	req := &dto.ReplacePlateOptionSlotsRequest{Slots: []dto.PlateOptionSlotInput{{Name: "Extra", Choices: []dto.PlateOptionChoiceInput{
		{Name: "Otra tilapia", IsDefault: true, Items: []dto.PlateOptionChoiceItemInput{bundleItem(optTilapiaID, 1)}},
	}}}}

	m.products.On("FindByID", ctx, optTilapiaID).Return(tilapiaProduct(), nil)

	_, err := svc.Replace(ctx, optTilapiaID, req)

	require.ErrorIs(t, err, domainError.ErrIngredientCannotBeSelf)
}

func TestPlateOptionSlotService_Replace_NonCompositeProduct(t *testing.T) {
	ctx := context.Background()
	svc, m := newTestPlateOptionSlotService(t)
	soda := createTestProductWithType("soda-1", "Gaseosa", "bebidas", 1, 4000, 0, dto.ProductTypeSellable)
	m.products.On("FindByID", ctx, "soda-1").Return(soda, nil)

	got, err := svc.Replace(ctx, "soda-1", tilapiaSlotsRequest())

	require.ErrorIs(t, err, domainError.ErrProductNotComposite)
	assert.Nil(t, got)
}

func TestPlateOptionSlotService_Replace_ProductNotFound(t *testing.T) {
	ctx := context.Background()
	svc, m := newTestPlateOptionSlotService(t)
	m.products.On("FindByID", ctx, optTilapiaID).Return(nil, errors.New("record not found"))

	_, err := svc.Replace(ctx, optTilapiaID, tilapiaSlotsRequest())

	require.ErrorIs(t, err, domainError.ErrProductNotFound)
}

func TestPlateOptionSlotService_Replace_RepositoryFailures(t *testing.T) {
	boom := errors.New("db down")
	tests := []struct {
		name  string
		setup func(ctx context.Context, m slotServiceMocks)
	}{
		{"ingredient lookup", func(ctx context.Context, m slotServiceMocks) {
			m.products.On("FindByIDs", ctx, mock.Anything).Return(nil, boom)
		}},
		{"existing slots", func(ctx context.Context, m slotServiceMocks) {
			expectBundleLookup(m, optArrozBlancoID, optArrozCocoID, optCanastaID, optPapasID, optPataconID, optEnsaladaID)
			m.slots.On("FindByCompositeProductID", ctx, optTilapiaID).Return(nil, boom)
		}},
		{"replace", func(ctx context.Context, m slotServiceMocks) {
			expectBundleLookup(m, optArrozBlancoID, optArrozCocoID, optCanastaID, optPapasID, optPataconID, optEnsaladaID)
			m.slots.On("FindByCompositeProductID", ctx, optTilapiaID).Return(nil, nil)
			m.slots.On("ReplaceForCompositeProduct", ctx, optTilapiaID, mock.Anything).Return(boom)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			svc, m := newTestPlateOptionSlotService(t)
			m.products.On("FindByID", ctx, optTilapiaID).Return(tilapiaProduct(), nil)
			tt.setup(ctx, m)

			_, err := svc.Replace(ctx, optTilapiaID, tilapiaSlotsRequest())

			require.ErrorIs(t, err, domainError.ErrPlateOptionSlotsUpdateFailed)
			require.ErrorIs(t, err, boom)
		})
	}
}

func TestPlateOptionSlotService_Get(t *testing.T) {
	ctx := context.Background()
	svc, m := newTestPlateOptionSlotService(t)
	slots := []dto.PlateOptionSlot{{ID: "slot-1", Name: "Arroz"}}
	m.products.On("FindByID", ctx, optTilapiaID).Return(tilapiaProduct(), nil)
	m.slots.On("FindByCompositeProductID", ctx, optTilapiaID).Return(slots, nil)

	got, err := svc.Get(ctx, optTilapiaID)

	require.NoError(t, err)
	assert.Equal(t, slots, got)
}

func TestPlateOptionSlotService_Get_ProductNotFound(t *testing.T) {
	ctx := context.Background()
	svc, m := newTestPlateOptionSlotService(t)
	m.products.On("FindByID", ctx, optTilapiaID).Return(nil, errors.New("record not found"))

	_, err := svc.Get(ctx, optTilapiaID)

	require.ErrorIs(t, err, domainError.ErrProductNotFound)
}

func TestPlateOptionSlotService_Get_RepositoryFailure(t *testing.T) {
	ctx := context.Background()
	svc, m := newTestPlateOptionSlotService(t)
	m.products.On("FindByID", ctx, optTilapiaID).Return(tilapiaProduct(), nil)
	m.slots.On("FindByCompositeProductID", ctx, optTilapiaID).Return(nil, errors.New("db down"))

	_, err := svc.Get(ctx, optTilapiaID)

	require.Error(t, err)
}
