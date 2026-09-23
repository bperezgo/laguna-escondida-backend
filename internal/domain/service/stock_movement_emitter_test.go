package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/platform/stockmovement"
	"laguna-escondida/backend/pkg/eventbus"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// capturingOutbox records every op a write path queues, so a test can assert both what is
// queued and what is not.
func capturingOutbox(t *testing.T, captured *[]*dto.SyncOutboxEntry) *mocks.MockSyncOutboxRepository {
	t.Helper()
	mockOutbox := mocks.NewMockSyncOutboxRepository(t)
	mockOutbox.EXPECT().
		Append(mock.Anything, mock.AnythingOfType("*dto.SyncOutboxEntry")).
		Run(func(_ context.Context, entry *dto.SyncOutboxEntry) { *captured = append(*captured, entry) }).
		Return(nil).Maybe()
	return mockOutbox
}

func stockServiceWithEmitter(t *testing.T, emitter ports.StockMovementEmitter) (*StockService, *mocks.MockStockRepository, *mocks.MockProductRepository) {
	t.Helper()
	mockStockRepo := mocks.NewMockStockRepository(t)
	mockProductRepo := mocks.NewMockProductRepository(t)
	return NewStockService(mockStockRepo, mockProductRepo, createMockUnitOfWork(t), emitter), mockStockRepo, mockProductRepo
}

// expectAdjust sets up the repository calls one AddOrDecreaseStock makes.
func expectAdjust(ctx context.Context, stockRepo *mocks.MockStockRepository, productRepo *mocks.MockProductRepository, productID string, from, change int) {
	product := createTestProduct(productID, "Test Product", "Category A", 1, 100.0, 19.0)
	productRepo.On("FindByID", ctx, productID).Return(product, nil).Once()
	stockRepo.On("FindByProductID", ctx, productID).Return(createTestStock(productID, 1, from), nil).Once()
	stockRepo.On("UpdateAmount", ctx, productID, from+change).Return(nil).Once()
	stockRepo.On("CreateHistoricRecord", ctx, mock.AnythingOfType("*dto.HistoricStock")).Return(nil).Once()
}

func TestStockService_EdgeConfiguration_QueuesOneMovementOp(t *testing.T) {
	ctx := context.Background()
	var captured []*dto.SyncOutboxEntry
	emitter := stockmovement.NewOutboxEmitter(capturingOutbox(t, &captured), testNodeID)
	service, stockRepo, productRepo := stockServiceWithEmitter(t, emitter)

	productID := "product-1"
	expectAdjust(ctx, stockRepo, productRepo, productID, 100, -12)

	require.NoError(t, service.AddOrDecreaseStock(ctx, &dto.AddOrDecreaseStockRequest{ProductID: productID, Change: -12}))

	require.Len(t, captured, 1, "exactly one op per movement")
	op := captured[0]
	assert.Equal(t, dto.SyncEntityHistoricStock, op.EntityType)
	assert.Equal(t, testNodeID, op.OriginNodeID)

	var payload dto.HistoricStockSyncPayload
	require.NoError(t, json.Unmarshal(op.Payload, &payload))
	assert.Equal(t, -12, payload.Change, "the op carries the change")
	assert.Equal(t, dto.StockMovementKindAdjustment, payload.Kind)
	assert.Equal(t, op.OpID, payload.OpID, "the movement op reuses the ledger row's op_id (1:1)")
}

func TestStockService_CloudConfiguration_QueuesNothing(t *testing.T) {
	ctx := context.Background()
	var captured []*dto.SyncOutboxEntry
	// The cloud's emitter never reaches an outbox at all; the recorder proves it.
	mockOutbox := capturingOutbox(t, &captured)
	service, stockRepo, productRepo := stockServiceWithEmitter(t, stockmovement.NewNoopEmitter())

	productID := "product-1"
	expectAdjust(ctx, stockRepo, productRepo, productID, 100, -12)

	require.NoError(t, service.AddOrDecreaseStock(ctx, &dto.AddOrDecreaseStockRequest{ProductID: productID, Change: -12}))

	assert.Empty(t, captured, "a cloud-origin stock op would never be delivered to anyone")
	mockOutbox.AssertNotCalled(t, "Append", mock.Anything, mock.Anything)
}

// A sale on the restaurant queues its movement and nothing else — no snapshot the cloud
// could assign.
func TestStockEventHandler_EdgeConfiguration_QueuesOnlyTheMovement(t *testing.T) {
	ctx := context.Background()
	var captured []*dto.SyncOutboxEntry
	emitter := stockmovement.NewOutboxEmitter(capturingOutbox(t, &captured), testNodeID)

	mockStockRepo := mocks.NewMockStockRepository(t)
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockIngredientRepo := mocks.NewMockProductIngredientRepository(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewStockEventHandler(
		mockStockRepo, mockProductRepo, mockIngredientRepo,
		eventbus.NewProductLockManager(), createMockUnitOfWork(t), emitter, logger,
	)

	productID := "product-1"
	product := createTestProductWithType(productID, "Test Product", "Category A", 1, 100.0, 19.0, dto.ProductTypeSellable)
	mockProductRepo.On("FindByID", ctx, productID).Return(product, nil).Twice()
	mockStockRepo.On("FindByProductID", ctx, productID).Return(createTestStock(productID, 1, 100), nil).Once()
	mockStockRepo.On("UpdateAmount", ctx, productID, 96).Return(nil).Once()
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.AnythingOfType("*dto.HistoricStock")).Return(nil).Once()

	require.NoError(t, handler.HandleOrderCreated(ctx, dto.OrderCreatedEvent{
		OpenBillID: "order-1",
		Products:   []dto.OrderCreatedEventProduct{{ProductID: productID, Quantity: 4}},
	}))

	require.Len(t, captured, 1)
	assert.Equal(t, dto.SyncEntityHistoricStock, captured[0].EntityType)

	var payload dto.HistoricStockSyncPayload
	require.NoError(t, json.Unmarshal(captured[0].Payload, &payload))
	assert.Equal(t, -4, payload.Change)
	assert.Equal(t, dto.StockMovementKindSale, payload.Kind)
}

// A failed emit rolls the whole movement back: the ledger row and its replication commit
// together or not at all.
func TestStockService_EmitFailureFailsTheWrite(t *testing.T) {
	ctx := context.Background()
	mockEmitter := mocks.NewMockStockMovementEmitter(t)
	mockEmitter.EXPECT().Emit(mock.Anything, mock.Anything).Return(errors.New("outbox down")).Once()
	service, stockRepo, productRepo := stockServiceWithEmitter(t, mockEmitter)

	productID := "product-1"
	expectAdjust(ctx, stockRepo, productRepo, productID, 100, -12)

	assert.Error(t, service.AddOrDecreaseStock(ctx, &dto.AddOrDecreaseStockRequest{ProductID: productID, Change: -12}))
}
