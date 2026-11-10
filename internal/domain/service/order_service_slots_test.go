package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	openBill "laguna-escondida/backend/internal/domain/aggregate/open_bill"
	"laguna-escondida/backend/internal/domain/dto"
	orderError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	pkgports "laguna-escondida/backend/pkg/domain/ports"
	pkgmocks "laguna-escondida/backend/pkg/domain/ports/mocks"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	slotPlateID       = "550e8400-e29b-41d4-a716-446655440101"
	slotSodaID        = "550e8400-e29b-41d4-a716-446655440102"
	slotArrozBlanco   = "550e8400-e29b-41d4-a716-446655440111"
	slotArrozCoco     = "550e8400-e29b-41d4-a716-446655440112"
	slotCanasta       = "550e8400-e29b-41d4-a716-446655440113"
	slotPapas         = "550e8400-e29b-41d4-a716-446655440114"
	slotPatacon       = "550e8400-e29b-41d4-a716-446655440115"
	slotEnsalada      = "550e8400-e29b-41d4-a716-446655440116"
	choiceArrozBlanco = "choice-arroz-blanco"
	choiceArrozCoco   = "choice-arroz-coco"
	choiceCanastas    = "choice-canastas"
	choicePapas       = "choice-papas"
	choicePatacon     = "choice-patacon"
	choiceEnsalada    = "choice-ensalada"
	choiceCanastaX    = "choice-canasta-extra"
)

func bundle(ingredientID string, qty int) []dto.PlateOptionChoiceItem {
	return []dto.PlateOptionChoiceItem{{IngredientProductID: ingredientID, Quantity: qty}}
}

func tilapiaSlots(withPatacon bool) []dto.PlateOptionSlot {
	acompanante := []dto.PlateOptionChoice{
		{ID: choiceCanastas, Name: "Canastas", IsDefault: true, Items: bundle(slotCanasta, 2)},
		{ID: choicePapas, Name: "Papas", Items: bundle(slotPapas, 1)},
	}
	if withPatacon {
		acompanante = append(acompanante, dto.PlateOptionChoice{ID: choicePatacon, Name: "Patacón", Items: bundle(slotPatacon, 1)})
	}
	return []dto.PlateOptionSlot{
		{ID: "slot-arroz", Name: "Arroz", Choices: []dto.PlateOptionChoice{
			{ID: choiceArrozBlanco, Name: "Arroz blanco", IsDefault: true, Items: bundle(slotArrozBlanco, 1)},
			{ID: choiceArrozCoco, Name: "Arroz de coco", Items: bundle(slotArrozCoco, 1)},
		}},
		{ID: "slot-acompanante", Name: "Acompañante", Choices: acompanante},
		{ID: "slot-ensalada", Name: "Ensalada", Choices: []dto.PlateOptionChoice{
			{ID: choiceEnsalada, Name: "Ensalada", IsDefault: true, Items: bundle(slotEnsalada, 1)},
			{ID: choiceCanastaX, Name: "Canasta extra", Items: bundle(slotCanasta, 1)},
		}},
	}
}

type slotOrderFixture struct {
	service   *OrderService
	products  *mocks.MockProductRepository
	openBills *mocks.MockOpenBillRepository
	slots     *mocks.MockPlateOptionSlotRepository
	outbox    []*dto.SyncOutboxEntry
	events    []pkgports.Event
}

func newSlotOrderFixture(t *testing.T) *slotOrderFixture {
	f := &slotOrderFixture{
		products:  mocks.NewMockProductRepository(t),
		openBills: mocks.NewMockOpenBillRepository(t),
		slots:     mocks.NewMockPlateOptionSlotRepository(t),
	}
	outbox := mocks.NewMockSyncOutboxRepository(t)
	outbox.EXPECT().Append(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, entry *dto.SyncOutboxEntry) error {
			f.outbox = append(f.outbox, entry)
			return nil
		}).Maybe()
	bus := pkgmocks.NewMockEventBus(t)
	bus.EXPECT().Publish(mock.Anything, mock.Anything).
		Run(func(_ context.Context, e pkgports.Event) { f.events = append(f.events, e) }).
		Return(nil).Maybe()
	f.openBills.EXPECT().ExistsActiveByTemporalIdentifier(mock.Anything, mock.Anything).Return(false, nil).Maybe()
	f.openBills.EXPECT().GetProductPreparationResponsibilities(mock.Anything, mock.Anything).
		Return([]dto.ProductPreparationResponsibilityWithProduct{}, nil).Maybe()
	ingredients := mocks.NewMockProductIngredientRepository(t)
	pending := mocks.NewMockPendingInvoiceRepository(t)

	f.service = NewOrderService(f.openBills, f.products, nil, pending, dto.PendingInvoiceStatusPending, nil,
		createMockUnitOfWork(t), bus, outbox, dto.SyncIdentity{NodeID: testNodeID}, ingredients, nil, "CO",
		WithOrderPlateOptionSlots(f.slots))
	return f
}

func slotPlate() *dto.Product {
	p := createTestProductWithType(slotPlateID, "Tilapia", "platos", 1, 30000, 0, dto.ProductTypeComposite)
	p.TotalPriceWithTaxes = decimal.NewFromInt(30000)
	return p
}

func tilapiaLine(choices ...string) dto.OrderProductItem {
	return dto.OrderProductItem{OpenBillProductID: uuidPlaceholder2, ProductID: slotPlateID, Quantity: 1, Choices: choices}
}

func (f *slotOrderFixture) create(t *testing.T, line dto.OrderProductItem, product *dto.Product) (*dto.OpenBill, *openBill.Aggregate, error) {
	t.Helper()
	ctx := createTestContext()
	f.products.On("FindByIDs", ctx, []string{product.ID}).Return([]*dto.Product{product}, nil)
	var captured *openBill.Aggregate
	f.openBills.On("Create", ctx, mock.AnythingOfType("*open_bill.Aggregate")).
		Run(func(args mock.Arguments) { captured, _ = args.Get(1).(*openBill.Aggregate) }).Return(nil).Maybe()
	result, err := f.service.CreateOrder(ctx, &dto.CreateOrderRequest{
		OpenBillID: uuidPlaceholder0, TemporalIdentifier: "TABLE-01", Products: []dto.OrderProductItem{line},
	}, createTestUser())
	return result, captured, err
}

func choiceNames(choices []dto.LineChoice) []string {
	out := make([]string, len(choices))
	for i, c := range choices {
		out[i] = c.SlotName + ": " + c.ChoiceName
	}
	return out
}

func selectionMap(selections []dto.SideDishSelection) map[string]int {
	out := map[string]int{}
	for _, s := range selections {
		out[s.IngredientProductID] = s.Quantity
	}
	return out
}

func TestOrderService_CreateOrder_SlotPlate_NoChoicesTakesDefaults(t *testing.T) {
	f := newSlotOrderFixture(t)
	f.slots.On("FindByCompositeProductID", mock.Anything, slotPlateID).Return(tilapiaSlots(true), nil)

	result, captured, err := f.create(t, tilapiaLine(), slotPlate())

	require.NoError(t, err)
	assert.True(t, result.TotalAmount.Equal(decimal.NewFromInt(30000)), "choices never change the price")
	line := captured.Products()[0]
	assert.Equal(t, []string{"Arroz: Arroz blanco", "Acompañante: Canastas", "Ensalada: Ensalada"}, choiceNames(line.Choices()))
	assert.Equal(t, map[string]int{slotArrozBlanco: 1, slotCanasta: 2, slotEnsalada: 1}, selectionMap(line.SideDishes()))
}

func TestOrderService_CreateOrder_SlotPlate_SwapsTwoSides(t *testing.T) {
	f := newSlotOrderFixture(t)
	f.slots.On("FindByCompositeProductID", mock.Anything, slotPlateID).Return(tilapiaSlots(true), nil)

	_, captured, err := f.create(t, tilapiaLine(choiceArrozCoco, choicePapas), slotPlate())

	require.NoError(t, err)
	line := captured.Products()[0]
	assert.Equal(t, []string{"Arroz: Arroz de coco", "Acompañante: Papas", "Ensalada: Ensalada"}, choiceNames(line.Choices()))
	assert.Equal(t, map[string]int{slotArrozCoco: 1, slotPapas: 1, slotEnsalada: 1}, selectionMap(line.SideDishes()))
}

func TestOrderService_CreateOrder_SlotPlate_SumsCanastasAcrossSlots(t *testing.T) {
	f := newSlotOrderFixture(t)
	f.slots.On("FindByCompositeProductID", mock.Anything, slotPlateID).Return(tilapiaSlots(true), nil)

	_, captured, err := f.create(t, tilapiaLine(choiceCanastaX), slotPlate())

	require.NoError(t, err)
	assert.Equal(t, map[string]int{slotArrozBlanco: 1, slotCanasta: 3}, selectionMap(captured.Products()[0].SideDishes()))
}

func TestOrderService_CreateOrder_SlotPlate_CarriesChoicesToOutboxAndEvent(t *testing.T) {
	f := newSlotOrderFixture(t)
	f.slots.On("FindByCompositeProductID", mock.Anything, slotPlateID).Return(tilapiaSlots(true), nil)

	_, _, err := f.create(t, tilapiaLine(choicePapas), slotPlate())

	require.NoError(t, err)
	require.Len(t, f.outbox, 1)
	var payload dto.OpenBillSyncPayload
	require.NoError(t, json.Unmarshal(f.outbox[0].Payload, &payload))
	require.Len(t, payload.Products, 1)
	assert.Equal(t, []string{"Arroz: Arroz blanco", "Acompañante: Papas", "Ensalada: Ensalada"}, choiceNames(payload.Products[0].Choices))

	require.Len(t, f.events, 1)
	created, ok := f.events[0].(dto.OrderCreatedEvent)
	require.True(t, ok)
	assert.Equal(t, []string{"Arroz: Arroz blanco", "Acompañante: Papas", "Ensalada: Ensalada"}, choiceNames(created.Products[0].Choices))
	assert.Equal(t, map[string]int{slotArrozBlanco: 1, slotPapas: 1, slotEnsalada: 1}, selectionMap(created.Products[0].SideDishes))
}

func TestOrderService_CreateOrder_SlotPlate_RejectsInvalidLines(t *testing.T) {
	tests := []struct {
		name    string
		line    dto.OrderProductItem
		message string
	}{
		{"both rices", tilapiaLine(choiceArrozBlanco, choiceArrozCoco), `the "Arroz" slot of Tilapia takes one choice`},
		{"the same choice twice", tilapiaLine(choicePapas, choicePapas), `the "Acompañante" slot of Tilapia takes one choice`},
		{"a choice of another plate", tilapiaLine("choice-of-the-mojarra"), "choice choice-of-the-mojarra is not an option of Tilapia"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newSlotOrderFixture(t)
			f.slots.On("FindByCompositeProductID", mock.Anything, slotPlateID).Return(tilapiaSlots(true), nil)

			result, _, err := f.create(t, tt.line, slotPlate())

			require.ErrorIs(t, err, orderError.ErrInvalidLineChoice)
			assert.Contains(t, err.Error(), tt.message)
			assert.Nil(t, result)
			f.openBills.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
			assert.Empty(t, f.outbox)
		})
	}
}

func TestOrderService_CreateOrder_ChoicesOnADrink(t *testing.T) {
	f := newSlotOrderFixture(t)
	soda := createTestProductWithType(slotSodaID, "Gaseosa", "bebidas", 1, 4000, 0, dto.ProductTypeSellable)
	line := dto.OrderProductItem{OpenBillProductID: uuidPlaceholder2, ProductID: slotSodaID, Quantity: 1, Choices: []string{choicePapas}}

	result, _, err := f.create(t, line, soda)

	require.ErrorIs(t, err, orderError.ErrInvalidLineChoice)
	assert.Contains(t, err.Error(), "Gaseosa has no option slots")
	assert.Nil(t, result)
	f.slots.AssertNotCalled(t, "FindByCompositeProductID", mock.Anything, mock.Anything)
}

func TestOrderService_CreateOrder_ChoicesOnAPlateWithoutSlots(t *testing.T) {
	f := newSlotOrderFixture(t)
	f.slots.On("FindByCompositeProductID", mock.Anything, slotPlateID).Return(nil, nil)

	result, _, err := f.create(t, tilapiaLine(choicePapas), slotPlate())

	require.ErrorIs(t, err, orderError.ErrInvalidLineChoice)
	assert.Nil(t, result)
}

func TestOrderService_CreateOrder_PlateWithoutSlotsTakesNothingExtra(t *testing.T) {
	f := newSlotOrderFixture(t)
	f.slots.On("FindByCompositeProductID", mock.Anything, slotPlateID).Return(nil, nil)

	_, captured, err := f.create(t, tilapiaLine(), slotPlate())

	require.NoError(t, err)
	assert.Nil(t, captured.Products()[0].Choices())
	assert.Nil(t, captured.Products()[0].SideDishes())
}

func TestOrderService_CreateOrder_SlotPlate_RejectsSideDishQuantities(t *testing.T) {
	f := newSlotOrderFixture(t)
	line := dto.OrderProductItem{OpenBillProductID: uuidPlaceholder2, ProductID: slotPlateID, Quantity: 1,
		SideDishes: []dto.SideDishSelection{{IngredientProductID: slotCanasta, Quantity: 3}}}

	result, _, err := f.create(t, line, slotPlate())

	require.ErrorIs(t, err, orderError.ErrSideDishesNotSupported)
	assert.Nil(t, result)
	f.slots.AssertNotCalled(t, "FindByCompositeProductID", mock.Anything, mock.Anything)
}

func (f *slotOrderFixture) update(t *testing.T, previous dto.OpenBillProductDetail, existing *openBill.OpenBillProduct, line dto.OrderProductItem) (*dto.OpenBill, error) {
	t.Helper()
	ctx := createTestContext()
	f.openBills.On("FindByIDWithProducts", ctx, billID1).Return(&dto.OpenBillWithProducts{
		ID: billID1, TemporalIdentifier: "TABLE-01", Products: []dto.OpenBillProductDetail{previous},
		CreatedBy: dto.OpenBillCreator{ID: "user-123"}, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}, nil)
	f.openBills.On("FindAggregateByID", ctx, billID1).Return(createTestOpenBillAggregate(billID1, []*openBill.OpenBillProduct{existing}), nil)
	f.products.On("FindByIDs", ctx, []string{slotPlateID}).Return([]*dto.Product{slotPlate()}, nil)
	f.openBills.On("Update", ctx, mock.AnythingOfType("*open_bill.Aggregate")).Return(nil).Maybe()
	return f.service.UpdateOrder(ctx, billID1, &dto.UpdateOrderRequest{Products: []dto.OrderProductItem{line}},
		dto.UserDomain{ID: "00000000-0000-0000-0000-000000000001"})
}

func takenWithPatacon(t *testing.T) (dto.OpenBillProductDetail, *openBill.OpenBillProduct) {
	t.Helper()
	choices := []dto.LineChoice{
		{SlotID: "slot-arroz", SlotName: "Arroz", ChoiceID: choiceArrozBlanco, ChoiceName: "Arroz blanco"},
		{SlotID: "slot-acompanante", SlotName: "Acompañante", ChoiceID: choicePatacon, ChoiceName: "Patacón"},
		{SlotID: "slot-ensalada", SlotName: "Ensalada", ChoiceID: choiceEnsalada, ChoiceName: "Ensalada"},
	}
	sideDishes := []dto.SideDishSelection{
		{IngredientProductID: slotArrozBlanco, Quantity: 1},
		{IngredientProductID: slotPatacon, Quantity: 1},
		{IngredientProductID: slotEnsalada, Quantity: 1},
	}
	existing, err := openBill.NewOpenBillProduct(uuidPlaceholder2, slotPlateID, 1, nil, nil, 0, "user-123")
	require.NoError(t, err)
	existing.SetSideDishes(sideDishes)
	existing.SetChoices(choices)
	return dto.OpenBillProductDetail{
		OpenBillProductID: uuidPlaceholder2, Product: *slotPlate(), Quantity: 1,
		SideDishes: sideDishes, Choices: choices,
	}, existing
}

func TestOrderService_UpdateOrder_SlotPlate_ChangesAChoice(t *testing.T) {
	f := newSlotOrderFixture(t)
	f.slots.On("FindByCompositeProductID", mock.Anything, slotPlateID).Return(tilapiaSlots(true), nil)
	previous, existing := takenWithPatacon(t)

	_, err := f.update(t, previous, existing, tilapiaLine(choiceCanastas))

	require.NoError(t, err)
	require.Len(t, f.outbox, 1)
	var payload dto.OpenBillSyncPayload
	require.NoError(t, json.Unmarshal(f.outbox[0].Payload, &payload))
	assert.Equal(t, []string{"Arroz: Arroz blanco", "Acompañante: Canastas", "Ensalada: Ensalada"}, choiceNames(payload.Products[0].Choices))

	require.Len(t, f.events, 1)
	updated, ok := f.events[0].(dto.OrderUpdatedEvent)
	require.True(t, ok)
	assert.Equal(t, "Patacón", updated.PreviousProducts[0].Choices[1].ChoiceName)
	assert.Equal(t, "Canastas", updated.CurrentProducts[0].Choices[1].ChoiceName)
	assert.Equal(t, map[string]int{slotArrozBlanco: 1, slotCanasta: 2, slotEnsalada: 1}, selectionMap(updated.CurrentProducts[0].SideDishes))
}

func TestOrderService_UpdateOrder_SlotPlate_ChoiceRemovedSinceTaken(t *testing.T) {
	f := newSlotOrderFixture(t)
	f.slots.On("FindByCompositeProductID", mock.Anything, slotPlateID).Return(tilapiaSlots(false), nil)
	previous, existing := takenWithPatacon(t)

	result, err := f.update(t, previous, existing, tilapiaLine(choicePatacon))

	require.ErrorIs(t, err, orderError.ErrInvalidLineChoice)
	assert.Nil(t, result)
	f.openBills.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	assert.Empty(t, f.outbox)
}

func TestOrderService_UpdateOrder_SlotPlate_RejectsBothRices(t *testing.T) {
	f := newSlotOrderFixture(t)
	f.slots.On("FindByCompositeProductID", mock.Anything, slotPlateID).Return(tilapiaSlots(true), nil)
	previous, existing := takenWithPatacon(t)

	result, err := f.update(t, previous, existing, tilapiaLine(choiceArrozBlanco, choiceArrozCoco))

	require.ErrorIs(t, err, orderError.ErrInvalidLineChoice)
	assert.Contains(t, err.Error(), `"Arroz"`)
	assert.Nil(t, result)
	f.openBills.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestOrderService_CreateOrder_SlotPlate_SlotLookupFails(t *testing.T) {
	f := newSlotOrderFixture(t)
	f.slots.On("FindByCompositeProductID", mock.Anything, slotPlateID).Return(nil, assert.AnError)

	result, _, err := f.create(t, tilapiaLine(), slotPlate())

	require.ErrorIs(t, err, assert.AnError)
	assert.Nil(t, result)
}

func TestOrderService_CreateOrder_SlotWithoutDefault(t *testing.T) {
	f := newSlotOrderFixture(t)
	slots := tilapiaSlots(true)
	slots[0].Choices[0].IsDefault = false
	f.slots.On("FindByCompositeProductID", mock.Anything, slotPlateID).Return(slots, nil)

	_, _, err := f.create(t, tilapiaLine(), slotPlate())

	require.ErrorIs(t, err, orderError.ErrInvalidLineChoice)
	assert.Contains(t, err.Error(), "has no default")
}
