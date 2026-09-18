package service

import (
	"context"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Side-dish stock consumption (BSP-33). A plate's recipe is salad(1), canasta(2), protein(1);
// salad and canasta are side dishes, protein is fixed. The line's resolved side-dish selection
// rides on the event and overrides the per-plate amount consumed for those ingredients, while
// fixed ingredients always consume their default recipe amount.

const (
	plateID   = "plate-1"
	saladID   = "salad-1"
	canastaID = "canasta-1"
	proteinID = "protein-1"
)

func platedRecipe() []*dto.ProductIngredient {
	return []*dto.ProductIngredient{
		createTestIngredient("ing-salad", plateID, saladID, 1.0),
		createTestIngredient("ing-canasta", plateID, canastaID, 2.0),
		createTestIngredient("ing-protein", plateID, proteinID, 1.0),
	}
}

// 5.1 backward compat: a plate with no side-dish selection consumes exactly the default recipe.
func TestHandleOrderCreated_SideDish_DefaultPlateConsumesDefaults(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, mockIngredientRepo := createTestStockEventHandler(t)

	plate := createTestProductWithType(plateID, "Plate", "platos", 1, 100.0, 0, dto.ProductTypeComposite)
	salad := createTestProductWithType(saladID, "Salad", "insumos", 1, 5.0, 0, dto.ProductTypeIngredient)
	canasta := createTestProductWithType(canastaID, "Canasta", "insumos", 1, 5.0, 0, dto.ProductTypeIngredient)
	protein := createTestProductWithType(proteinID, "Protein", "insumos", 1, 5.0, 0, dto.ProductTypeIngredient)

	event := dto.OrderCreatedEvent{
		OpenBillID: "order-1",
		Products:   []dto.OrderCreatedEventProduct{{ProductID: plateID, Quantity: 1}},
	}

	mockProductRepo.On("FindByID", ctx, plateID).Return(plate, nil)
	mockIngredientRepo.On("FindByCompositeProductID", ctx, plateID).Return(platedRecipe(), nil)
	mockProductRepo.On("FindByID", ctx, saladID).Return(salad, nil)
	mockProductRepo.On("FindByID", ctx, canastaID).Return(canasta, nil)
	mockProductRepo.On("FindByID", ctx, proteinID).Return(protein, nil)
	mockStockRepo.On("FindByProductID", ctx, saladID).Return(createTestStock(saladID, 1, 100), nil)
	mockStockRepo.On("FindByProductID", ctx, canastaID).Return(createTestStock(canastaID, 1, 100), nil)
	mockStockRepo.On("FindByProductID", ctx, proteinID).Return(createTestStock(proteinID, 1, 100), nil)
	mockStockRepo.On("UpdateAmount", ctx, saladID, 99).Return(nil).Once()   // 100 - 1
	mockStockRepo.On("UpdateAmount", ctx, canastaID, 98).Return(nil).Once() // 100 - 2
	mockStockRepo.On("UpdateAmount", ctx, proteinID, 99).Return(nil).Once() // 100 - 1
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.Anything).Return(nil)

	require.NoError(t, handler.HandleOrderCreated(ctx, event))
	mockStockRepo.AssertExpectations(t)
}

// 5.2: salad 0 / canasta 3 consumes 0 salad (skipped), 3 canasta, and the default protein.
func TestHandleOrderCreated_SideDish_ConsumptionFollowsSelection(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, mockIngredientRepo := createTestStockEventHandler(t)

	plate := createTestProductWithType(plateID, "Plate", "platos", 1, 100.0, 0, dto.ProductTypeComposite)
	canasta := createTestProductWithType(canastaID, "Canasta", "insumos", 1, 5.0, 0, dto.ProductTypeIngredient)
	protein := createTestProductWithType(proteinID, "Protein", "insumos", 1, 5.0, 0, dto.ProductTypeIngredient)

	event := dto.OrderCreatedEvent{
		OpenBillID: "order-1",
		Products: []dto.OrderCreatedEventProduct{{
			ProductID: plateID,
			Quantity:  1,
			SideDishes: []dto.SideDishSelection{
				{IngredientProductID: saladID, Quantity: 0},
				{IngredientProductID: canastaID, Quantity: 3},
			},
		}},
	}

	mockProductRepo.On("FindByID", ctx, plateID).Return(plate, nil)
	mockIngredientRepo.On("FindByCompositeProductID", ctx, plateID).Return(platedRecipe(), nil)
	mockProductRepo.On("FindByID", ctx, canastaID).Return(canasta, nil)
	mockProductRepo.On("FindByID", ctx, proteinID).Return(protein, nil)
	mockStockRepo.On("FindByProductID", ctx, canastaID).Return(createTestStock(canastaID, 1, 100), nil)
	mockStockRepo.On("FindByProductID", ctx, proteinID).Return(createTestStock(proteinID, 1, 100), nil)
	mockStockRepo.On("UpdateAmount", ctx, canastaID, 97).Return(nil).Once() // 100 - 3
	mockStockRepo.On("UpdateAmount", ctx, proteinID, 99).Return(nil).Once() // 100 - 1
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.Anything).Return(nil)

	require.NoError(t, handler.HandleOrderCreated(ctx, event))

	// Salad at 0 consumes nothing and must never touch a stock row.
	mockProductRepo.AssertNotCalled(t, "FindByID", ctx, saladID)
	mockStockRepo.AssertNotCalled(t, "FindByProductID", ctx, saladID)
	mockStockRepo.AssertExpectations(t)
}

// 5.3: raising canasta from 2 to 3 on an existing line decrements exactly one more canasta.
func TestHandleOrderUpdated_SideDish_RaiseDecrementsOneMore(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, mockIngredientRepo := createTestStockEventHandler(t)

	plate := createTestProductWithType(plateID, "Plate", "platos", 1, 100.0, 0, dto.ProductTypeComposite)
	canasta := createTestProductWithType(canastaID, "Canasta", "insumos", 1, 5.0, 0, dto.ProductTypeIngredient)
	salad := createTestProductWithType(saladID, "Salad", "insumos", 1, 5.0, 0, dto.ProductTypeIngredient)
	protein := createTestProductWithType(proteinID, "Protein", "insumos", 1, 5.0, 0, dto.ProductTypeIngredient)

	line := func(canastaQty int) dto.OrderCreatedEventProduct {
		return dto.OrderCreatedEventProduct{
			OpenBillProductID: "line-1", ProductID: plateID, Quantity: 1,
			SideDishes: []dto.SideDishSelection{
				{IngredientProductID: saladID, Quantity: 1},
				{IngredientProductID: canastaID, Quantity: canastaQty},
			},
		}
	}

	event := dto.OrderUpdatedEvent{
		OpenBillID:       "order-1",
		PreviousProducts: []dto.OrderCreatedEventProduct{line(2)},
		CurrentProducts:  []dto.OrderCreatedEventProduct{line(3)},
	}

	mockProductRepo.On("FindByID", ctx, plateID).Return(plate, nil)
	mockIngredientRepo.On("FindByCompositeProductID", ctx, plateID).Return(platedRecipe(), nil)
	mockProductRepo.On("FindByID", ctx, canastaID).Return(canasta, nil)
	mockProductRepo.On("FindByID", ctx, saladID).Return(salad, nil)
	mockProductRepo.On("FindByID", ctx, proteinID).Return(protein, nil)
	mockStockRepo.On("FindByProductID", ctx, canastaID).Return(createTestStock(canastaID, 1, 100), nil)
	mockStockRepo.On("FindByProductID", ctx, saladID).Return(createTestStock(saladID, 1, 100), nil)
	mockStockRepo.On("FindByProductID", ctx, proteinID).Return(createTestStock(proteinID, 1, 100), nil)
	// Previous (canasta 2) is restored, current (canasta 3) is consumed: net one more canasta out.
	mockStockRepo.On("UpdateAmount", ctx, canastaID, 102).Return(nil).Once() // restore +2
	mockStockRepo.On("UpdateAmount", ctx, canastaID, 97).Return(nil).Once()  // consume -3
	// Salad (1) and protein (1) restore then re-consume: net zero, one restore + one consume each.
	mockStockRepo.On("UpdateAmount", ctx, saladID, 101).Return(nil).Once()
	mockStockRepo.On("UpdateAmount", ctx, saladID, 99).Return(nil).Once()
	mockStockRepo.On("UpdateAmount", ctx, proteinID, 101).Return(nil).Once()
	mockStockRepo.On("UpdateAmount", ctx, proteinID, 99).Return(nil).Once()
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.Anything).Return(nil)

	require.NoError(t, handler.HandleOrderUpdated(ctx, event))
	mockStockRepo.AssertExpectations(t)
}

// 5.3: lowering canasta from 3 to 1 on an existing line restores exactly two canasta.
func TestHandleOrderUpdated_SideDish_LowerRestoresTwo(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, mockIngredientRepo := createTestStockEventHandler(t)

	plate := createTestProductWithType(plateID, "Plate", "platos", 1, 100.0, 0, dto.ProductTypeComposite)
	canasta := createTestProductWithType(canastaID, "Canasta", "insumos", 1, 5.0, 0, dto.ProductTypeIngredient)
	salad := createTestProductWithType(saladID, "Salad", "insumos", 1, 5.0, 0, dto.ProductTypeIngredient)
	protein := createTestProductWithType(proteinID, "Protein", "insumos", 1, 5.0, 0, dto.ProductTypeIngredient)

	line := func(canastaQty int) dto.OrderCreatedEventProduct {
		return dto.OrderCreatedEventProduct{
			OpenBillProductID: "line-1", ProductID: plateID, Quantity: 1,
			SideDishes: []dto.SideDishSelection{
				{IngredientProductID: saladID, Quantity: 1},
				{IngredientProductID: canastaID, Quantity: canastaQty},
			},
		}
	}

	event := dto.OrderUpdatedEvent{
		OpenBillID:       "order-1",
		PreviousProducts: []dto.OrderCreatedEventProduct{line(3)},
		CurrentProducts:  []dto.OrderCreatedEventProduct{line(1)},
	}

	mockProductRepo.On("FindByID", ctx, plateID).Return(plate, nil)
	mockIngredientRepo.On("FindByCompositeProductID", ctx, plateID).Return(platedRecipe(), nil)
	mockProductRepo.On("FindByID", ctx, canastaID).Return(canasta, nil)
	mockProductRepo.On("FindByID", ctx, saladID).Return(salad, nil)
	mockProductRepo.On("FindByID", ctx, proteinID).Return(protein, nil)
	mockStockRepo.On("FindByProductID", ctx, canastaID).Return(createTestStock(canastaID, 1, 100), nil)
	mockStockRepo.On("FindByProductID", ctx, saladID).Return(createTestStock(saladID, 1, 100), nil)
	mockStockRepo.On("FindByProductID", ctx, proteinID).Return(createTestStock(proteinID, 1, 100), nil)
	// Restore previous (canasta 3) then consume current (canasta 1): net two canasta back.
	mockStockRepo.On("UpdateAmount", ctx, canastaID, 103).Return(nil).Once() // restore +3
	mockStockRepo.On("UpdateAmount", ctx, canastaID, 99).Return(nil).Once()  // consume -1
	mockStockRepo.On("UpdateAmount", ctx, saladID, 101).Return(nil).Once()
	mockStockRepo.On("UpdateAmount", ctx, saladID, 99).Return(nil).Once()
	mockStockRepo.On("UpdateAmount", ctx, proteinID, 101).Return(nil).Once()
	mockStockRepo.On("UpdateAmount", ctx, proteinID, 99).Return(nil).Once()
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.Anything).Return(nil)

	require.NoError(t, handler.HandleOrderUpdated(ctx, event))
	mockStockRepo.AssertExpectations(t)
}

// 5.4: deleting an order restores side-dish stock from the recorded selection (salad 0, canasta 3).
func TestHandleOrderDeleted_SideDish_RestoresRecordedSelection(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, mockIngredientRepo := createTestStockEventHandler(t)

	plate := createTestProductWithType(plateID, "Plate", "platos", 1, 100.0, 0, dto.ProductTypeComposite)
	canasta := createTestProductWithType(canastaID, "Canasta", "insumos", 1, 5.0, 0, dto.ProductTypeIngredient)
	protein := createTestProductWithType(proteinID, "Protein", "insumos", 1, 5.0, 0, dto.ProductTypeIngredient)

	event := dto.OrderDeletedEvent{
		OpenBillID: "order-1",
		Products: []dto.OrderCreatedEventProduct{{
			ProductID: plateID,
			Quantity:  1,
			SideDishes: []dto.SideDishSelection{
				{IngredientProductID: saladID, Quantity: 0},
				{IngredientProductID: canastaID, Quantity: 3},
			},
		}},
	}

	mockProductRepo.On("FindByID", ctx, plateID).Return(plate, nil)
	mockIngredientRepo.On("FindByCompositeProductID", ctx, plateID).Return(platedRecipe(), nil)
	mockProductRepo.On("FindByID", ctx, canastaID).Return(canasta, nil)
	mockProductRepo.On("FindByID", ctx, proteinID).Return(protein, nil)
	mockStockRepo.On("FindByProductID", ctx, canastaID).Return(createTestStock(canastaID, 1, 100), nil)
	mockStockRepo.On("FindByProductID", ctx, proteinID).Return(createTestStock(proteinID, 1, 100), nil)
	mockStockRepo.On("UpdateAmount", ctx, canastaID, 103).Return(nil).Once() // restore +3
	mockStockRepo.On("UpdateAmount", ctx, proteinID, 101).Return(nil).Once() // restore +1
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.Anything).Return(nil)

	require.NoError(t, handler.HandleOrderDeleted(ctx, event))

	mockProductRepo.AssertNotCalled(t, "FindByID", ctx, saladID)
	mockStockRepo.AssertNotCalled(t, "FindByProductID", ctx, saladID)
	mockStockRepo.AssertExpectations(t)
}
