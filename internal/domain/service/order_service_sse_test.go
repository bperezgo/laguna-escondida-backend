package service

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newSSEService wires an OrderService with just the collaborators the SSE handlers touch.
func newSSEService(openBillRepo *mocks.MockOpenBillRepository, productRepo *mocks.MockProductRepository, notifier *mocks.MockOpenBillProductSSENotifier) *OrderService {
	return NewOrderServiceWithSSE(
		discardLogger(),
		openBillRepo,
		productRepo,
		nil, nil, dto.PendingInvoiceStatusPending, nil, nil, nil, nil,
		notifier,
		nil, dto.SyncIdentity{NodeID: testNodeID}, nil,
	)
}

// 6.1: the created SSE payload carries the resolved side dishes with names resolved at the seam.
func TestHandleOrderCreatedSSE_CarriesResolvedSideDishesWithNames(t *testing.T) {
	ctx := context.Background()
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockNotifier := mocks.NewMockOpenBillProductSSENotifier(t)
	service := newSSEService(mockOpenBillRepo, mockProductRepo, mockNotifier)

	plateID := "plate-1"
	saladID := "salad-1"
	canastaID := "canasta-1"

	event := dto.OrderCreatedEvent{
		OpenBillID:         "order-1",
		TemporalIdentifier: "TABLE-01",
		Products: []dto.OrderCreatedEventProduct{{
			OpenBillProductID: "line-1",
			ProductID:         plateID,
			Quantity:          1,
			SideDishes: []dto.SideDishSelection{
				{IngredientProductID: saladID, Quantity: 0},
				{IngredientProductID: canastaID, Quantity: 3},
			},
		}},
	}

	mockOpenBillRepo.On("GetProductPreparationResponsibilities", ctx, []string{plateID}).
		Return([]dto.ProductPreparationResponsibilityWithProduct{
			{ProductID: plateID, ProductName: "Plate", Area: "cocina", Priority: 1},
		}, nil)
	mockProductRepo.On("FindByIDs", ctx, mock.Anything).Return([]*dto.Product{
		createTestProduct(saladID, "Ensalada", "insumos", 1, 5.0, 0),
		createTestProduct(canastaID, "Canasta", "insumos", 1, 5.0, 0),
	}, nil)

	var captured *dto.OpenBillProductSSE
	mockNotifier.On("NotifyArea", ctx, "cocina", OpenBillProductCreatedEventType, mock.Anything).
		Run(func(args mock.Arguments) { captured, _ = args.Get(3).(*dto.OpenBillProductSSE) }).Return(nil)

	require.NoError(t, service.HandleOrderCreatedSSE(ctx, event))
	require.NotNil(t, captured)
	require.Len(t, captured.SideDishes, 2)
	byName := map[string]int{}
	for _, sd := range captured.SideDishes {
		byName[sd.Name] = sd.Quantity
	}
	require.Equal(t, 0, byName["Ensalada"])
	require.Equal(t, 3, byName["Canasta"])
}

// 6.2: editing only the side dishes on an existing line still re-notifies the kitchen with the
// new resolved selection.
func TestHandleOrderUpdatedSSE_ReNotifiesOnSideDishChange(t *testing.T) {
	ctx := context.Background()
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockNotifier := mocks.NewMockOpenBillProductSSENotifier(t)
	service := newSSEService(mockOpenBillRepo, mockProductRepo, mockNotifier)

	plateID := "plate-1"
	canastaID := "canasta-1"

	line := func(canastaQty int) dto.OrderCreatedEventProduct {
		return dto.OrderCreatedEventProduct{
			OpenBillProductID: "line-1", ProductID: plateID, Quantity: 1,
			SideDishes: []dto.SideDishSelection{{IngredientProductID: canastaID, Quantity: canastaQty}},
		}
	}

	event := dto.OrderUpdatedEvent{
		OpenBillID:       "order-1",
		PreviousProducts: []dto.OrderCreatedEventProduct{line(2)},
		CurrentProducts:  []dto.OrderCreatedEventProduct{line(3)},
	}

	mockOpenBillRepo.On("GetProductPreparationResponsibilities", ctx, mock.Anything).
		Return([]dto.ProductPreparationResponsibilityWithProduct{
			{ProductID: plateID, ProductName: "Plate", Area: "cocina", Priority: 1},
		}, nil)
	mockOpenBillRepo.On("FindByIDWithProducts", ctx, "order-1").Return(&dto.OpenBillWithProducts{
		Products: []dto.OpenBillProductDetail{{OpenBillProductID: "line-1", CreatedAt: time.Now()}},
	}, nil)
	mockProductRepo.On("FindByIDs", ctx, mock.Anything).Return([]*dto.Product{
		createTestProduct(canastaID, "Canasta", "insumos", 1, 5.0, 0),
	}, nil)

	var captured *dto.OpenBillProductSSE
	mockNotifier.On("NotifyArea", ctx, "cocina", OpenBillProductUpdatedEventType, mock.Anything).
		Run(func(args mock.Arguments) { captured, _ = args.Get(3).(*dto.OpenBillProductSSE) }).Return(nil)

	require.NoError(t, service.HandleOrderUpdatedSSE(ctx, event))
	require.NotNil(t, captured, "an edit touching only side dishes must re-notify the area")
	require.Len(t, captured.SideDishes, 1)
	require.Equal(t, "Canasta", captured.SideDishes[0].Name)
	require.Equal(t, 3, captured.SideDishes[0].Quantity)
}
