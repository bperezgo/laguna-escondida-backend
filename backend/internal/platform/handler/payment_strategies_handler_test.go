package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newPaymentSettingsRouter(t *testing.T) (*gin.Engine, *mocks.MockPaymentSettingsRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := mocks.NewMockPaymentSettingsRepository(t)
	h := NewPaymentSettingsHandler(service.NewPaymentSettingsService(repo))
	router := gin.New()
	router.GET("/api/organization/payment-settings", h.GetHandler)
	router.PUT("/api/organization/payment-settings", h.PutHandler)
	return router, repo
}

func serveJSON(router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestPaymentSettingsHandler_Get_DefaultsToWompi(t *testing.T) {
	router, repo := newPaymentSettingsRouter(t)
	repo.EXPECT().Get(mock.Anything).Return(nil, false, nil)

	rec := serveJSON(router, http.MethodGet, "/api/organization/payment-settings", "")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"strategy":"wompi","manual_instructions":"","manual_window_minutes":60}`, rec.Body.String())
}

func TestPaymentSettingsHandler_Put_Manual(t *testing.T) {
	router, repo := newPaymentSettingsRouter(t)
	repo.EXPECT().Upsert(mock.Anything, &dto.PaymentSettings{
		Strategy: dto.PaymentStrategyManual, ManualInstructions: "Llave Bre-B 3001234567", ManualWindowMinutes: 60,
	}).Return(nil)

	rec := serveJSON(router, http.MethodPut, "/api/organization/payment-settings",
		`{"strategy":"manual","manual_instructions":"Llave Bre-B 3001234567","manual_window_minutes":60}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"strategy":"manual","manual_instructions":"Llave Bre-B 3001234567","manual_window_minutes":60}`, rec.Body.String())
}

func TestPaymentSettingsHandler_Put_Refused(t *testing.T) {
	tests := []struct{ name, body, field string }{
		{"manual without instructions", `{"strategy":"manual","manual_instructions":"  "}`, "manual_instructions"},
		{"unknown strategy", `{"strategy":"cash"}`, "strategy"},
		{"window too short", `{"strategy":"wompi","manual_window_minutes":5}`, "manual_window_minutes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, _ := newPaymentSettingsRouter(t)

			rec := serveJSON(router, http.MethodPut, "/api/organization/payment-settings", tt.body)

			require.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Contains(t, rec.Body.String(), `"field":"`+tt.field+`"`)
		})
	}
}

func TestPaymentSettingsHandler_Put_InvalidBody(t *testing.T) {
	router, _ := newPaymentSettingsRouter(t)

	rec := serveJSON(router, http.MethodPut, "/api/organization/payment-settings", `{"manual_window_minutes":"soon"}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func newPaymentReviewRouter(t *testing.T) (*gin.Engine, *mocks.MockOpenBillRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := mocks.NewMockOpenBillRepository(t)
	orders := service.NewOrderService(repo, nil, nil, nil, dto.PendingInvoiceStatusPending, nil, nil, nil, nil, dto.SyncIdentity{}, nil, nil, "CO")
	h := NewOrderHandler(orders, nil)
	router := gin.New()
	router.PUT("/api/orders/:id/payment-review", h.MarkPaymentReviewHandler)
	router.DELETE("/api/orders/:id/payment-review", h.ClearPaymentReviewHandler)
	return router, repo
}

func TestOrderHandler_PaymentReview_Refused(t *testing.T) {
	const orderID = "550e8400-e29b-41d4-a716-446655440010"
	manual, wompi := dto.PaymentStrategyManual, dto.PaymentStrategyWompi
	tests := []struct {
		name   string
		state  *dto.OrderPaymentState
		found  bool
		status int
	}{
		{"unknown", nil, false, http.StatusNotFound},
		{"paid", &dto.OrderPaymentState{Strategy: &manual, Paid: true}, true, http.StatusConflict},
		{"voided", &dto.OrderPaymentState{Strategy: &manual, Voided: true}, true, http.StatusConflict},
		{"wompi", &dto.OrderPaymentState{Strategy: &wompi}, true, http.StatusConflict},
	}
	for _, tt := range tests {
		for _, method := range []string{http.MethodPut, http.MethodDelete} {
			t.Run(tt.name+" "+method, func(t *testing.T) {
				router, repo := newPaymentReviewRouter(t)
				repo.EXPECT().FindPaymentState(mock.Anything, orderID).Return(tt.state, tt.found, nil)

				rec := serveJSON(router, method, "/api/orders/"+orderID+"/payment-review", "")

				assert.Equal(t, tt.status, rec.Code)
			})
		}
	}
}

func TestOrderHandler_PaymentReview_Marks(t *testing.T) {
	const orderID = "550e8400-e29b-41d4-a716-446655440010"
	manual := dto.PaymentStrategyManual
	router, repo := newPaymentReviewRouter(t)
	repo.EXPECT().FindPaymentState(mock.Anything, orderID).Return(&dto.OrderPaymentState{Strategy: &manual}, true, nil)
	started := tenantTestTime()
	repo.EXPECT().FindByIDWithProducts(mock.Anything, orderID).Return(&dto.OpenBillWithProducts{
		ID: orderID, OrderFulfillment: dto.OrderFulfillment{PaymentStrategy: &manual, PaymentReviewStartedAt: &started},
	}, nil)

	rec := serveJSON(router, http.MethodPut, "/api/orders/"+orderID+"/payment-review", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"payment_under_review":true`)
}

func tenantTestTime() time.Time { return time.Date(2026, 11, 24, 17, 0, 0, 0, time.UTC) }
