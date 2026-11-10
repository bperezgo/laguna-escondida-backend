package service

import (
	"context"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// These tests exercise the BSP-18 cycle guard in AddIngredient. They intentionally
// reference domainError.ErrIngredientCycle, which Stage 3 must add — until then this
// file fails to compile, which is the expected red for AC5 and AC6.

func createTestProductIngredientService(t *testing.T) (*ProductIngredientService, *mocks.MockProductIngredientRepository, *mocks.MockProductRepository) {
	mockIngredientRepo := mocks.NewMockProductIngredientRepository(t)
	mockProductRepo := mocks.NewMockProductRepository(t)
	svc := NewProductIngredientService(mockIngredientRepo, mockProductRepo)
	return svc, mockIngredientRepo, mockProductRepo
}

// AC5: Direct cycle rejected at add time.
// Given COMPOSITE A already has ingredient B (COMPOSITE), when AddIngredient(B, {ingredient: A})
// is called, it returns ErrIngredientCycle and creates no ingredient row.
func TestAddIngredient_DirectCycle_Rejected_AC5(t *testing.T) {
	ctx := context.Background()
	svc, mockIngredientRepo, mockProductRepo := createTestProductIngredientService(t)

	aID := "prod-a"
	bID := "prod-b"

	productA := createTestProductWithType(aID, "Plate A", "platos", 1, 100.0, 0, dto.ProductTypeComposite)
	productB := createTestProductWithType(bID, "Plate B", "platos", 1, 100.0, 0, dto.ProductTypeComposite)

	// Existing edge A -> B (A already has ingredient B).
	edgeAB := createTestIngredient("edge-ab", aID, bID, 1.0)

	// Full graph view, order-independent, so any traversal the guard performs sees a
	// consistent picture. Adding B -> A would close the cycle A -> B -> A.
	mockProductRepo.On("FindByID", ctx, aID).Return(productA, nil).Maybe()
	mockProductRepo.On("FindByID", ctx, bID).Return(productB, nil).Maybe()
	mockIngredientRepo.On("FindByCompositeProductID", ctx, aID).Return([]*dto.ProductIngredient{edgeAB}, nil).Maybe()
	mockIngredientRepo.On("FindByCompositeProductID", ctx, bID).Return([]*dto.ProductIngredient{}, nil).Maybe()
	mockIngredientRepo.On("Create", ctx, mock.Anything).Return(nil).Maybe()

	got, err := svc.AddIngredient(ctx, bID, &dto.AddIngredientRequest{IngredientProductID: aID, Quantity: "2"})

	require.ErrorIs(t, err, domainError.ErrIngredientCycle)
	require.Nil(t, got)
	mockIngredientRepo.AssertNotCalled(t, "Create")
}

// AC6: Indirect cycle rejected at add time.
// Given A->B and B->C ingredient edges, when AddIngredient(C, {ingredient: A}) is called,
// it returns ErrIngredientCycle and creates no row.
func TestAddIngredient_IndirectCycle_Rejected_AC6(t *testing.T) {
	ctx := context.Background()
	svc, mockIngredientRepo, mockProductRepo := createTestProductIngredientService(t)

	aID := "prod-a"
	bID := "prod-b"
	cID := "prod-c"

	productA := createTestProductWithType(aID, "Plate A", "platos", 1, 100.0, 0, dto.ProductTypeComposite)
	productB := createTestProductWithType(bID, "Plate B", "platos", 1, 100.0, 0, dto.ProductTypeComposite)
	productC := createTestProductWithType(cID, "Plate C", "platos", 1, 100.0, 0, dto.ProductTypeComposite)

	edgeAB := createTestIngredient("edge-ab", aID, bID, 1.0)
	edgeBC := createTestIngredient("edge-bc", bID, cID, 1.0)

	// Adding C -> A would close the cycle A -> B -> C -> A.
	mockProductRepo.On("FindByID", ctx, aID).Return(productA, nil).Maybe()
	mockProductRepo.On("FindByID", ctx, bID).Return(productB, nil).Maybe()
	mockProductRepo.On("FindByID", ctx, cID).Return(productC, nil).Maybe()
	mockIngredientRepo.On("FindByCompositeProductID", ctx, aID).Return([]*dto.ProductIngredient{edgeAB}, nil).Maybe()
	mockIngredientRepo.On("FindByCompositeProductID", ctx, bID).Return([]*dto.ProductIngredient{edgeBC}, nil).Maybe()
	mockIngredientRepo.On("FindByCompositeProductID", ctx, cID).Return([]*dto.ProductIngredient{}, nil).Maybe()
	mockIngredientRepo.On("Create", ctx, mock.Anything).Return(nil).Maybe()

	got, err := svc.AddIngredient(ctx, cID, &dto.AddIngredientRequest{IngredientProductID: aID, Quantity: "2"})

	require.ErrorIs(t, err, domainError.ErrIngredientCycle)
	require.Nil(t, got)
	mockIngredientRepo.AssertNotCalled(t, "Create")
}
