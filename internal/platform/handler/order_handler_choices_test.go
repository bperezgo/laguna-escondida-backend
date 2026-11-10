package handler

import (
	"bytes"
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

const (
	choiceOrderID = "550e8400-e29b-41d4-a716-446655440010"
	choiceSodaID  = "550e8400-e29b-41d4-a716-446655440001"
	choiceLineID  = "660e8400-e29b-41d4-a716-446655440001"
	choiceUserID  = "550e8400-e29b-41d4-a716-446655440020"
)

func choiceOrderService(t *testing.T, repo *mocks.MockOpenBillRepository, products *mocks.MockProductRepository) *service.OrderService {
	repo.EXPECT().GetProductPreparationResponsibilities(mock.Anything, mock.Anything).
		Return([]dto.ProductPreparationResponsibilityWithProduct{}, nil).Maybe()
	products.EXPECT().FindByIDs(mock.Anything, []string{choiceSodaID}).
		Return([]*dto.Product{{ID: choiceSodaID, Name: "Gaseosa", ProductType: dto.ProductTypeSellable}}, nil)
	return service.NewOrderService(repo, products, nil, nil, dto.PendingInvoiceStatusPending, nil, nil, nil, nil,
		dto.SyncIdentity{}, nil, nil, "CO", service.WithOrderPlateOptionSlots(mocks.NewMockPlateOptionSlotRepository(t)))
}

const sodaWithChoiceLine = `{"open_bill_product_id":"` + choiceLineID + `","product_id":"` + choiceSodaID + `","quantity":1,"choices":["550e8400-e29b-41d4-a716-446655440099"]}`

func TestOrderHandler_CreateOrder_ChoicesOnADrink_Returns400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := mocks.NewMockOpenBillRepository(t)
	repo.EXPECT().ExistsActiveByTemporalIdentifier(mock.Anything, mock.Anything).Return(false, nil)
	orderService := choiceOrderService(t, repo, mocks.NewMockProductRepository(t))

	router := gin.New()
	router.POST("/api/orders", func(c *gin.Context) { c.Set("user_id", choiceUserID) }, NewOrderHandler(orderService, nil).CreateOrderHandler)
	body := `{"open_bill_id":"` + choiceOrderID + `","temporal_identifier":"550e8400-e29b-41d4-a716-446655440011","products":[` + sodaWithChoiceLine + `]}`
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/orders", bytes.NewBufferString(body)))

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "Gaseosa has no option slots")
}

func TestOrderHandler_UpdateOrder_InvalidChoice_Returns400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := mocks.NewMockOpenBillRepository(t)
	aggregate, err := openBill.NewAggregate(choiceOrderID, "T1", nil, decimal.Zero, nil, choiceUserID)
	require.NoError(t, err)
	repo.EXPECT().FindByIDWithProducts(mock.Anything, choiceOrderID).Return(&dto.OpenBillWithProducts{ID: choiceOrderID}, nil)
	repo.EXPECT().FindAggregateByID(mock.Anything, choiceOrderID).Return(aggregate, nil)
	orderService := choiceOrderService(t, repo, mocks.NewMockProductRepository(t))

	router := gin.New()
	router.PUT("/api/orders/:id", func(c *gin.Context) { c.Set("user_id", choiceUserID) }, NewOrderHandler(orderService, nil).UpdateOrderHandler)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/orders/"+choiceOrderID,
		bytes.NewBufferString(`{"products":[`+sodaWithChoiceLine+`]}`)))

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid plate choice")
}

func TestOrderHandler_GetOrder_ReturnsLineChoices(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := mocks.NewMockOpenBillRepository(t)
	repo.EXPECT().FindByIDWithProducts(mock.Anything, choiceOrderID).Return(&dto.OpenBillWithProducts{
		ID: choiceOrderID, CreatedAt: time.Now(), UpdatedAt: time.Now(),
		Products: []dto.OpenBillProductDetail{{
			OpenBillProductID: choiceLineID, Quantity: 1,
			Choices: []dto.LineChoice{{SlotID: "slot-1", SlotName: "Acompañante", ChoiceID: "choice-1", ChoiceName: "Papas"}},
		}},
	}, nil)
	orderService := service.NewOrderService(repo, nil, nil, nil, dto.PendingInvoiceStatusPending, nil, nil, nil, nil,
		dto.SyncIdentity{}, nil, nil, "CO")

	router := gin.New()
	router.GET("/api/orders/:id", NewOrderHandler(orderService, nil).GetOpenBillWithProductsHandler)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/orders/"+choiceOrderID, nil))

	require.Equal(t, http.StatusOK, rec.Code)
	var got struct {
		Products []struct {
			Choices []map[string]string `json:"choices"`
		} `json:"products"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, []map[string]string{{"slot_id": "slot-1", "slot_name": "Acompañante", "choice_id": "choice-1", "choice_name": "Papas"}},
		got.Products[0].Choices)
}
