package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	openBill "laguna-escondida/backend/internal/domain/aggregate/open_bill"
	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestOrderHandler_CancelOpenBillProduct_PaidOrder_Returns409(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const (
		orderID   = "550e8400-e29b-41d4-a716-446655440010"
		lineID    = "660e8400-e29b-41d4-a716-446655440001"
		productID = "550e8400-e29b-41d4-a716-446655440001"
		userID    = "550e8400-e29b-41d4-a716-446655440020"
	)
	area := "kitchen"
	line, err := openBill.NewOpenBillProductFromRepository(lineID, productID, 1, nil, dto.CommandStatusInProgress, &area, 1, userID)
	require.NoError(t, err)
	paidAt := time.Now()
	aggregate, err := openBill.NewAggregateFromRepository(orderID, "T1", decimal.NewFromInt(100), nil,
		[]*openBill.OpenBillProduct{line}, userID, paidAt, paidAt, &paidAt, &paidAt)
	require.NoError(t, err)

	repo := mocks.NewMockOpenBillRepository(t)
	repo.EXPECT().FindAggregateForKitchen(mock.Anything, orderID).Return(aggregate, nil).Once()
	orderService := service.NewOrderService(repo, nil, nil, nil, dto.PendingInvoiceStatusPending, nil, nil, nil, nil, dto.SyncIdentity{}, nil, nil, "CO")

	router := gin.New()
	router.PATCH("/api/orders/:id/products/:open_bill_product_id/cancel",
		func(c *gin.Context) { c.Set("user_id", userID) },
		NewOrderHandler(orderService, nil).CancelOpenBillProductHandler)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/api/orders/"+orderID+"/products/"+lineID+"/cancel", nil))

	assert.Equal(t, http.StatusConflict, rec.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "A line of a paid order cannot be cancelled", body["error"])
}
