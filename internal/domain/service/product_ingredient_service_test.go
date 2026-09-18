package service

import (
	"context"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/shopspring/decimal"
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

func TestConfigureSideDish_Accept(t *testing.T) {
	ctx := context.Background()
	svc, mockIngredientRepo, _ := createTestProductIngredientService(t)

	plateID := "plate-1"
	saladID := "salad-1"
	edge := createTestIngredient("edge-salad", plateID, saladID, 1.0)

	mockIngredientRepo.On("FindByID", ctx, edge.ID).Return(edge, nil)
	mockIngredientRepo.On("Update", ctx, edge.ID, mock.MatchedBy(func(ing *dto.ProductIngredient) bool {
		return ing.IsSideDish && ing.MinQuantity == 0 && ing.MaxQuantity == 2 &&
			ing.DefaultQuantity.Equal(decimal.NewFromInt(1))
	})).Return(nil)

	got, err := svc.ConfigureSideDish(ctx, plateID, edge.ID, &dto.ConfigureSideDishRequest{
		DefaultQuantity: 1, MinQuantity: 0, MaxQuantity: 2,
	})

	require.NoError(t, err)
	require.True(t, got.IsSideDish)
	require.Equal(t, 0, got.MinQuantity)
	require.Equal(t, 2, got.MaxQuantity)
	require.True(t, got.DefaultQuantity.Equal(decimal.NewFromInt(1)))
}

func TestConfigureSideDish_RejectMaxBelowDefault(t *testing.T) {
	ctx := context.Background()
	svc, mockIngredientRepo, _ := createTestProductIngredientService(t)

	plateID := "plate-1"
	edge := createTestIngredient("edge-1", plateID, "salad-1", 1.0)
	mockIngredientRepo.On("FindByID", ctx, edge.ID).Return(edge, nil)

	got, err := svc.ConfigureSideDish(ctx, plateID, edge.ID, &dto.ConfigureSideDishRequest{
		DefaultQuantity: 2, MinQuantity: 0, MaxQuantity: 1,
	})

	require.ErrorIs(t, err, domainError.ErrInvalidSideDishBounds)
	require.Nil(t, got)
	mockIngredientRepo.AssertNotCalled(t, "Update")
}

func TestConfigureSideDish_RejectMinAboveDefault(t *testing.T) {
	ctx := context.Background()
	svc, mockIngredientRepo, _ := createTestProductIngredientService(t)

	plateID := "plate-1"
	edge := createTestIngredient("edge-1", plateID, "salad-1", 1.0)
	mockIngredientRepo.On("FindByID", ctx, edge.ID).Return(edge, nil)

	got, err := svc.ConfigureSideDish(ctx, plateID, edge.ID, &dto.ConfigureSideDishRequest{
		DefaultQuantity: 1, MinQuantity: 2, MaxQuantity: 3,
	})

	require.ErrorIs(t, err, domainError.ErrInvalidSideDishBounds)
	require.Nil(t, got)
	mockIngredientRepo.AssertNotCalled(t, "Update")
}

func TestConfigureSideDish_OfferedAlternative_DefaultZero(t *testing.T) {
	ctx := context.Background()
	svc, mockIngredientRepo, _ := createTestProductIngredientService(t)

	plateID := "plate-1"
	friesID := "fries-1"
	edge := createTestIngredient("edge-fries", plateID, friesID, 1.0)

	mockIngredientRepo.On("FindByID", ctx, edge.ID).Return(edge, nil)
	mockIngredientRepo.On("Update", ctx, edge.ID, mock.Anything).Return(nil)

	got, err := svc.ConfigureSideDish(ctx, plateID, edge.ID, &dto.ConfigureSideDishRequest{
		DefaultQuantity: 0, MinQuantity: 0, MaxQuantity: 2,
	})

	require.NoError(t, err)
	require.True(t, got.IsSideDish)
	require.True(t, got.DefaultQuantity.Equal(decimal.Zero))
}

func TestGetSideDishOptions_ReturnsOnlySideDishes(t *testing.T) {
	ctx := context.Background()
	svc, mockIngredientRepo, mockProductRepo := createTestProductIngredientService(t)

	plateID := "plate-1"
	plate := createTestProductWithType(plateID, "Plate", "platos", 1, 100.0, 0, dto.ProductTypeComposite)
	mockProductRepo.On("FindByID", ctx, plateID).Return(plate, nil)

	salad := &dto.ProductIngredientWithProduct{
		IngredientProductID: "salad-1",
		DefaultQuantity:     decimal.NewFromInt(1),
		IsSideDish:          true,
		MinQuantity:         0,
		MaxQuantity:         2,
	}
	protein := &dto.ProductIngredientWithProduct{
		IngredientProductID: "protein-1",
		DefaultQuantity:     decimal.NewFromInt(1),
		IsSideDish:          false,
	}
	mockIngredientRepo.On("FindByCompositeProductIDWithProducts", ctx, plateID).
		Return([]*dto.ProductIngredientWithProduct{salad, protein}, nil)

	options, err := svc.GetSideDishOptions(ctx, plateID)

	require.NoError(t, err)
	require.Len(t, options, 1)
	require.Equal(t, "salad-1", options[0].IngredientProductID)
	require.Equal(t, 1, options[0].DefaultQuantity)
	require.Equal(t, 2, options[0].MaxQuantity)
}
