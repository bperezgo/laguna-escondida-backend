package service

import (
	"context"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// BSP-18 composite-stock behavior. Helpers (createTestStockEventHandler,
// createTestProductWithType, createTestIngredient, createTestStock) live in
// stock_event_handler_test.go — same package, reused here.
//
// Red map for the current single-level implementation:
//   - AC2/AC3/AC7 fail because decreaseStockForProduct/increaseStockForProduct expand
//     only one level: a COMPOSITE used as an ingredient is decremented on its own stock
//     row instead of having its sub-recipe expanded.
//   - AC1/AC4/AC8 already hold on the current code — they are regression guards that must
//     stay green after the recursive change lands.

// AC1: Single-level decrement (regression).
// COMPOSITE C with one recipe row (I, 3); selling 2×C drops I by 3×2 = 6 and writes no
// stock row for C. (Expected GREEN — single level is already implemented.)
func TestHandleOrderCreated_SingleLevelComposite_AC1(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, mockIngredientRepo := createTestStockEventHandler(t)

	cID := "composite-c"
	iID := "ingredient-i"

	composite := createTestProductWithType(cID, "Blackberry Juice", "bebidas", 1, 100.0, 0, dto.ProductTypeComposite)
	ingredient := createTestProductWithType(iID, "Blackberry", "insumos", 1, 10.0, 0, dto.ProductTypeIngredient)
	recipe := []*dto.ProductIngredient{createTestIngredient("ing-ci", cID, iID, 3.0)}
	iStock := createTestStock(iID, 1, 100)

	event := dto.OrderCreatedEvent{
		OpenBillID: "order-1",
		Products:   []dto.OrderCreatedEventProduct{{ProductID: cID, Quantity: 2}},
	}

	mockProductRepo.On("FindByID", ctx, cID).Return(composite, nil)
	mockIngredientRepo.On("FindByCompositeProductID", ctx, cID).Return(recipe, nil)
	mockProductRepo.On("FindByID", ctx, iID).Return(ingredient, nil)
	mockStockRepo.On("FindByProductID", ctx, iID).Return(iStock, nil)
	mockStockRepo.On("UpdateAmount", ctx, iID, 94).Return(nil).Once() // 100 - 3×2
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.MatchedBy(func(h *dto.HistoricStock) bool {
		return h.ProductID == iID && h.Change == -6
	})).Return(nil).Once()

	err := handler.HandleOrderCreated(ctx, event)

	require.NoError(t, err)
	mockStockRepo.AssertNotCalled(t, "FindByProductID", ctx, cID)
	mockStockRepo.AssertNotCalled(t, "UpdateAmount", ctx, cID, mock.Anything)
	mockStockRepo.AssertExpectations(t)
	mockProductRepo.AssertExpectations(t)
	mockIngredientRepo.AssertExpectations(t)
}

// AC2: Two-level decrement.
// COMPOSITE C -> (B, 2), where B is COMPOSITE -> (L, 5) and L is INGREDIENT. Selling 4×C
// drops L by 2×5×4 = 40; no stock row is written for C or B.
// (Expected RED: current code decrements B's own stock instead of expanding B -> L.)
func TestHandleOrderCreated_TwoLevelComposite_AC2(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, mockIngredientRepo := createTestStockEventHandler(t)

	cID := "composite-c"
	bID := "composite-b"
	lID := "leaf-l"

	cProd := createTestProductWithType(cID, "Combo", "platos", 1, 100.0, 0, dto.ProductTypeComposite)
	bProd := createTestProductWithType(bID, "Sub Combo", "platos", 1, 50.0, 0, dto.ProductTypeComposite)
	lProd := createTestProductWithType(lID, "Rice", "insumos", 1, 5.0, 0, dto.ProductTypeIngredient)
	lStock := createTestStock(lID, 1, 100)

	event := dto.OrderCreatedEvent{
		OpenBillID: "order-1",
		Products:   []dto.OrderCreatedEventProduct{{ProductID: cID, Quantity: 4}},
	}

	mockProductRepo.On("FindByID", ctx, cID).Return(cProd, nil)
	mockIngredientRepo.On("FindByCompositeProductID", ctx, cID).
		Return([]*dto.ProductIngredient{createTestIngredient("ing-cb", cID, bID, 2.0)}, nil)
	mockProductRepo.On("FindByID", ctx, bID).Return(bProd, nil)
	mockIngredientRepo.On("FindByCompositeProductID", ctx, bID).
		Return([]*dto.ProductIngredient{createTestIngredient("ing-bl", bID, lID, 5.0)}, nil)
	mockProductRepo.On("FindByID", ctx, lID).Return(lProd, nil)
	mockStockRepo.On("FindByProductID", ctx, lID).Return(lStock, nil)
	mockStockRepo.On("UpdateAmount", ctx, lID, 60).Return(nil).Once() // 100 - 2×5×4
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.MatchedBy(func(h *dto.HistoricStock) bool {
		return h.ProductID == lID && h.Change == -40
	})).Return(nil).Once()

	err := handler.HandleOrderCreated(ctx, event)

	require.NoError(t, err)
	// The intermediate composite B and the top composite C must never touch a stock row.
	mockStockRepo.AssertNotCalled(t, "FindByProductID", ctx, bID)
	mockStockRepo.AssertNotCalled(t, "UpdateAmount", ctx, bID, mock.Anything)
	mockStockRepo.AssertNotCalled(t, "FindByProductID", ctx, cID)
	mockStockRepo.AssertExpectations(t)
}

// AC3: Void restores a multi-level recipe exactly (mirror of AC2).
// Given the AC2 recipe and an order that sold 4×C, voiding (deleting) that order increases
// L by exactly 40; C and B are untouched.
// (Expected RED: current restore path expands only one level, crediting B not L.)
func TestHandleOrderDeleted_TwoLevelComposite_AC3(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, mockIngredientRepo := createTestStockEventHandler(t)

	cID := "composite-c"
	bID := "composite-b"
	lID := "leaf-l"

	cProd := createTestProductWithType(cID, "Combo", "platos", 1, 100.0, 0, dto.ProductTypeComposite)
	bProd := createTestProductWithType(bID, "Sub Combo", "platos", 1, 50.0, 0, dto.ProductTypeComposite)
	lProd := createTestProductWithType(lID, "Rice", "insumos", 1, 5.0, 0, dto.ProductTypeIngredient)
	lStock := createTestStock(lID, 1, 100)

	event := dto.OrderDeletedEvent{
		OpenBillID: "order-1",
		Products:   []dto.OrderCreatedEventProduct{{ProductID: cID, Quantity: 4}},
	}

	mockProductRepo.On("FindByID", ctx, cID).Return(cProd, nil)
	mockIngredientRepo.On("FindByCompositeProductID", ctx, cID).
		Return([]*dto.ProductIngredient{createTestIngredient("ing-cb", cID, bID, 2.0)}, nil)
	mockProductRepo.On("FindByID", ctx, bID).Return(bProd, nil)
	mockIngredientRepo.On("FindByCompositeProductID", ctx, bID).
		Return([]*dto.ProductIngredient{createTestIngredient("ing-bl", bID, lID, 5.0)}, nil)
	mockProductRepo.On("FindByID", ctx, lID).Return(lProd, nil)
	mockStockRepo.On("FindByProductID", ctx, lID).Return(lStock, nil)
	mockStockRepo.On("UpdateAmount", ctx, lID, 140).Return(nil).Once() // 100 + 2×5×4
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.MatchedBy(func(h *dto.HistoricStock) bool {
		return h.ProductID == lID && h.Change == 40
	})).Return(nil).Once()

	err := handler.HandleOrderDeleted(ctx, event)

	require.NoError(t, err)
	mockStockRepo.AssertNotCalled(t, "FindByProductID", ctx, bID)
	mockStockRepo.AssertNotCalled(t, "UpdateAmount", ctx, bID, mock.Anything)
	mockStockRepo.AssertNotCalled(t, "FindByProductID", ctx, cID)
	mockStockRepo.AssertExpectations(t)
}

// AC4: A BOTH ingredient is consumed, not expanded.
// COMPOSITE C -> (P, 2) where P.product_type = BOTH; selling 1×C drops P by 2 and P's own
// recipe is never expanded. (Expected GREEN — the finished item is consumed, not exploded.)
func TestHandleOrderCreated_BothIngredientConsumed_AC4(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, mockIngredientRepo := createTestStockEventHandler(t)

	cID := "composite-c"
	pID := "both-p"

	cProd := createTestProductWithType(cID, "Plate", "platos", 1, 100.0, 0, dto.ProductTypeComposite)
	pProd := createTestProductWithType(pID, "Cheese", "insumos", 1, 20.0, 0, dto.ProductTypeBoth)
	recipe := []*dto.ProductIngredient{createTestIngredient("ing-cp", cID, pID, 2.0)}
	pStock := createTestStock(pID, 1, 50)

	event := dto.OrderCreatedEvent{
		OpenBillID: "order-1",
		Products:   []dto.OrderCreatedEventProduct{{ProductID: cID, Quantity: 1}},
	}

	mockProductRepo.On("FindByID", ctx, cID).Return(cProd, nil)
	mockIngredientRepo.On("FindByCompositeProductID", ctx, cID).Return(recipe, nil)
	mockProductRepo.On("FindByID", ctx, pID).Return(pProd, nil)
	mockStockRepo.On("FindByProductID", ctx, pID).Return(pStock, nil)
	mockStockRepo.On("UpdateAmount", ctx, pID, 48).Return(nil).Once() // 50 - 2
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.MatchedBy(func(h *dto.HistoricStock) bool {
		return h.ProductID == pID && h.Change == -2
	})).Return(nil).Once()

	err := handler.HandleOrderCreated(ctx, event)

	require.NoError(t, err)
	// A BOTH product used as an ingredient must consume its own stock, not expand its recipe.
	mockIngredientRepo.AssertNotCalled(t, "FindByCompositeProductID", ctx, pID)
	mockStockRepo.AssertExpectations(t)
}

// AC7: Sell-time expansion terminates on a cyclic graph.
// A repo whose rows form a cycle C -> B -> C (constructed directly). Selling C visits each
// product at most once, logs a warning, and returns without hanging or erroring; no leaf is
// ever reached so no stock row is written.
// (Expected RED: current code treats B — a COMPOSITE used as an ingredient — as a leaf and
// tries to write B's stock; the .Once() bounds also catch a guard-less naive recursion.)
func TestHandleOrderCreated_CyclicGraphTerminates_AC7(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, mockIngredientRepo := createTestStockEventHandler(t)

	cID := "composite-c"
	bID := "composite-b"

	cProd := createTestProductWithType(cID, "Plate C", "platos", 1, 100.0, 0, dto.ProductTypeComposite)
	bProd := createTestProductWithType(bID, "Plate B", "platos", 1, 100.0, 0, dto.ProductTypeComposite)

	event := dto.OrderCreatedEvent{
		OpenBillID: "order-1",
		Products:   []dto.OrderCreatedEventProduct{{ProductID: cID, Quantity: 1}},
	}

	mockProductRepo.On("FindByID", ctx, cID).Return(cProd, nil)
	mockProductRepo.On("FindByID", ctx, bID).Return(bProd, nil)
	// The cycle: C -> B -> C. Each node's recipe must be fetched at most once (visited set).
	mockIngredientRepo.On("FindByCompositeProductID", ctx, cID).
		Return([]*dto.ProductIngredient{createTestIngredient("ing-cb", cID, bID, 1.0)}, nil).Once()
	mockIngredientRepo.On("FindByCompositeProductID", ctx, bID).
		Return([]*dto.ProductIngredient{createTestIngredient("ing-bc", bID, cID, 1.0)}, nil).Once()

	err := handler.HandleOrderCreated(ctx, event)

	require.NoError(t, err)
	// The graph never reaches a leaf, so no stock row is ever written.
	mockStockRepo.AssertNotCalled(t, "UpdateAmount")
	mockStockRepo.AssertNotCalled(t, "FindByProductID")
	mockIngredientRepo.AssertExpectations(t)
}

// AC8: Selling below zero is allowed.
// Leaf L has stock 3; a sale requiring 10 of L drives L to -7 with no error returned.
// (Expected GREEN — there is deliberately no insufficient-stock guard.)
func TestHandleOrderCreated_SellingBelowZero_AC8(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, _ := createTestStockEventHandler(t)

	lID := "leaf-l"
	lProd := createTestProductWithType(lID, "Rice", "insumos", 1, 5.0, 0, dto.ProductTypeIngredient)
	lStock := createTestStock(lID, 1, 3)

	event := dto.OrderCreatedEvent{
		OpenBillID: "order-1",
		Products:   []dto.OrderCreatedEventProduct{{ProductID: lID, Quantity: 10}},
	}

	mockProductRepo.On("FindByID", ctx, lID).Return(lProd, nil)
	mockStockRepo.On("FindByProductID", ctx, lID).Return(lStock, nil)
	mockStockRepo.On("UpdateAmount", ctx, lID, -7).Return(nil).Once() // 3 - 10
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.MatchedBy(func(h *dto.HistoricStock) bool {
		return h.ProductID == lID && h.Change == -10
	})).Return(nil).Once()

	err := handler.HandleOrderCreated(ctx, event)

	require.NoError(t, err)
	mockStockRepo.AssertExpectations(t)
}
