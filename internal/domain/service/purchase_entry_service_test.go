package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func createTestPurchaseEntryRequest() *dto.CreatePurchaseEntryRequest {
	return &dto.CreatePurchaseEntryRequest{
		SupplierID: "550e8400-e29b-41d4-a716-446655440100",
		Items: []dto.CreatePurchaseEntryItemRequest{
			{ProductID: "550e8400-e29b-41d4-a716-446655440101", Quantity: "2", UnitCost: "10.50"},
		},
	}
}

// TestCreatePurchaseEntry_WritesOutboxRowInTransaction asserts the transactional
// outbox (Option A): creating a purchase entry appends exactly one purchase_entry
// outbox row, stamped with this node's id and the create operation, and the payload
// is the full entry snapshot.
func TestCreatePurchaseEntry_WritesOutboxRowInTransaction(t *testing.T) {
	ctx := context.Background()
	mockPurchaseEntryRepo := mocks.NewMockPurchaseEntryRepository(t)
	mockSupplierRepo := mocks.NewMockSupplierRepository(t)
	mockSupplierCatalogRepo := mocks.NewMockSupplierCatalogRepository(t)
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockStorage := mocks.NewMockStorageClient(t)
	mockEventBus := createMockEventBus(t)
	mockUnitOfWork := createMockUnitOfWork(t)
	mockOutbox := mocks.NewMockSyncOutboxRepository(t)

	purchaseEntryService := NewPurchaseEntryService(
		mockPurchaseEntryRepo, mockSupplierRepo, mockSupplierCatalogRepo, mockProductRepo,
		mockStorage, mockEventBus, mockUnitOfWork, mockOutbox, dto.SyncIdentity{NodeID: testNodeID}, slog.Default(), "org-1",
	)

	req := createTestPurchaseEntryRequest()
	productID := req.Items[0].ProductID

	mockSupplierRepo.On("FindByID", ctx, req.SupplierID).Return(&dto.Supplier{}, nil)
	mockProductRepo.On("FindByIDs", ctx, []string{productID}).Return([]*dto.Product{{ID: productID}}, nil)
	mockPurchaseEntryRepo.On("Create", ctx, mock.AnythingOfType("*purchase_entry.Aggregate")).Return(nil)
	// Supplier catalog update is best-effort and happens after commit.
	mockSupplierCatalogRepo.On("FindBySupplierAndProduct", ctx, req.SupplierID, productID).Return(nil, errors.New("not found")).Maybe()
	mockSupplierCatalogRepo.On("Create", ctx, mock.AnythingOfType("*dto.SupplierCatalog")).Return(nil).Maybe()

	var captured *dto.SyncOutboxEntry
	mockOutbox.EXPECT().
		Append(mock.Anything, mock.AnythingOfType("*dto.SyncOutboxEntry")).
		Run(func(_ context.Context, entry *dto.SyncOutboxEntry) { captured = entry }).
		Return(nil).
		Once()

	result, err := purchaseEntryService.CreatePurchaseEntry(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)

	require.NotNil(t, captured)
	assert.NotEmpty(t, captured.OpID, "service must set a client-generated op_id")
	assert.Equal(t, testNodeID, captured.OriginNodeID)
	assert.Equal(t, dto.SyncEntityPurchaseEntry, captured.EntityType)
	assert.Equal(t, dto.SyncOperationCreate, captured.Operation)
	assert.Equal(t, result.ID, captured.EntityID)

	var snapshot dto.PurchaseEntry
	require.NoError(t, json.Unmarshal(captured.Payload, &snapshot))
	assert.Equal(t, result.ID, snapshot.ID)
}

func TestCreatePurchaseEntry_SupplierNotFound(t *testing.T) {
	ctx := context.Background()
	mockPurchaseEntryRepo := mocks.NewMockPurchaseEntryRepository(t)
	mockSupplierRepo := mocks.NewMockSupplierRepository(t)
	mockSupplierCatalogRepo := mocks.NewMockSupplierCatalogRepository(t)
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockStorage := mocks.NewMockStorageClient(t)
	mockEventBus := createMockEventBus(t)
	mockUnitOfWork := createMockUnitOfWork(t)
	mockOutbox := createMockSyncOutboxRepository(t)

	purchaseEntryService := NewPurchaseEntryService(
		mockPurchaseEntryRepo, mockSupplierRepo, mockSupplierCatalogRepo, mockProductRepo,
		mockStorage, mockEventBus, mockUnitOfWork, mockOutbox, dto.SyncIdentity{NodeID: testNodeID}, slog.Default(), "org-1",
	)

	req := createTestPurchaseEntryRequest()
	mockSupplierRepo.On("FindByID", ctx, req.SupplierID).Return(nil, errors.New("not found"))

	result, err := purchaseEntryService.CreatePurchaseEntry(ctx, req)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainError.ErrSupplierNotFound)
}

func TestCreatePurchaseEntry_ProductNotFound(t *testing.T) {
	ctx := context.Background()
	mockPurchaseEntryRepo := mocks.NewMockPurchaseEntryRepository(t)
	mockSupplierRepo := mocks.NewMockSupplierRepository(t)
	mockSupplierCatalogRepo := mocks.NewMockSupplierCatalogRepository(t)
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockStorage := mocks.NewMockStorageClient(t)
	mockEventBus := createMockEventBus(t)
	mockUnitOfWork := createMockUnitOfWork(t)
	mockOutbox := createMockSyncOutboxRepository(t)

	purchaseEntryService := NewPurchaseEntryService(
		mockPurchaseEntryRepo, mockSupplierRepo, mockSupplierCatalogRepo, mockProductRepo,
		mockStorage, mockEventBus, mockUnitOfWork, mockOutbox, dto.SyncIdentity{NodeID: testNodeID}, slog.Default(), "org-1",
	)

	req := createTestPurchaseEntryRequest()
	productID := req.Items[0].ProductID

	mockSupplierRepo.On("FindByID", ctx, req.SupplierID).Return(&dto.Supplier{}, nil)
	mockProductRepo.On("FindByIDs", ctx, []string{productID}).Return([]*dto.Product{}, nil)

	result, err := purchaseEntryService.CreatePurchaseEntry(ctx, req)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainError.ErrProductNotFound)
}
