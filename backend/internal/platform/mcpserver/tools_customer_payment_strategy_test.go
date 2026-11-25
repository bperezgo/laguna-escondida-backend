package mcpserver

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func manualPayment(expires time.Time, review bool) dto.OrderFulfillment {
	manual := dto.PaymentStrategyManual
	instructions := "Llave Bre-B 3001234567, a nombre de Laguna Escondida SAS"
	return dto.OrderFulfillment{
		PaymentStrategy: &manual, PaymentExpiresAt: &expires, PaymentInstructions: &instructions, PaymentUnderReview: &review,
	}
}

func TestCreateDeliveryOrder_AManualOrderComesWithInstructionsAndNoLink(t *testing.T) {
	expires := time.Date(2026, 11, 24, 18, 0, 0, 0, time.UTC)
	backend := newFakeBackend(t)
	backend.create = func(w http.ResponseWriter, _ *http.Request) {
		order := dto.OpenBill{ID: uuid.NewString(), TotalAmount: decimal.NewFromInt(32000)}
		order.OrderType = dto.OrderTypeTakeaway
		order.OrderFulfillment = manualPayment(expires, false)
		backend.writeJSON(w, http.StatusCreated, order)
	}
	cs := connectCustomer(t, backend, map[string]string{customerPhoneHeader: "573005551234"})

	text, isErr := call(t, cs, "create_delivery_order", map[string]any{"order_type": "takeaway", "lines": []map[string]any{{"product_id": "prod-Bandeja", "quantity": 1}}})

	require.False(t, isErr, text)
	var view orderView
	require.NoError(t, json.Unmarshal([]byte(text), &view))
	assert.Equal(t, paymentAwaiting, view.PaymentState)
	require.NotNil(t, view.PaymentStrategy)
	assert.Equal(t, dto.PaymentStrategyManual, *view.PaymentStrategy)
	require.NotNil(t, view.PaymentInstructions)
	assert.Contains(t, *view.PaymentInstructions, "Llave Bre-B")
	require.NotNil(t, view.PaymentExpiresAt)
	assert.True(t, expires.Equal(*view.PaymentExpiresAt))
	assert.Nil(t, view.PaymentURL)
	assert.NotContains(t, text, "payment_url")
}

func TestGetMyOrders_AManualOrderBeingChecked(t *testing.T) {
	expires := time.Now().UTC().Add(30 * time.Minute)
	backend := newFakeBackend(t)
	backend.orders = []*fakeOrder{
		{ID: uuid.NewString(), Phone: callerPhone, Type: dto.OrderTypeTakeaway, CreatedAt: time.Now().UTC().Add(-5 * time.Minute), Payment: manualPayment(expires, true)},
	}
	cs := connectCustomer(t, backend, map[string]string{customerPhoneHeader: "573005551234"})

	text, isErr := call(t, cs, "get_my_orders", nil)

	require.False(t, isErr, text)
	var result myOrders
	require.NoError(t, json.Unmarshal([]byte(text), &result))
	require.Len(t, result.Orders, 1)
	order := result.Orders[0]
	assert.Equal(t, paymentAwaiting, order.PaymentState)
	assert.Equal(t, dto.PaymentStrategyManual, *order.PaymentStrategy)
	require.NotNil(t, order.PaymentUnderReview)
	assert.True(t, *order.PaymentUnderReview)
	assert.Contains(t, *order.PaymentInstructions, "Llave Bre-B")
	assert.Nil(t, order.PaymentURL)
}

func TestGetMyOrders_AnExpiredManualOrderHasNoInstructions(t *testing.T) {
	backend := newFakeBackend(t)
	backend.orders = []*fakeOrder{
		{ID: uuid.NewString(), Phone: callerPhone, Type: dto.OrderTypeTakeaway, CreatedAt: time.Now().UTC().Add(-time.Hour), Expired: true,
			Payment: manualPayment(time.Now().UTC().Add(-time.Minute), false)},
	}
	cs := connectCustomer(t, backend, map[string]string{customerPhoneHeader: "573005551234"})

	text, isErr := call(t, cs, "get_my_orders", nil)

	require.False(t, isErr, text)
	var result myOrders
	require.NoError(t, json.Unmarshal([]byte(text), &result))
	require.Len(t, result.Orders, 1)
	assert.Equal(t, paymentExpired, result.Orders[0].PaymentState)
	assert.Nil(t, result.Orders[0].PaymentInstructions)
	assert.Nil(t, result.Orders[0].PaymentExpiresAt)
	assert.NotNil(t, result.Orders[0].PaymentStrategy, "the strategy is still the order's")
}

func TestGetOrderingAvailability_ReportsThePaymentStrategy(t *testing.T) {
	manual := dto.PaymentStrategyManual
	backend := newFakeBackend(t)
	backend.settings = &dto.OrganizationSettingsResponse{Timezone: "America/Bogota", PaymentStrategy: &manual}
	cs := connectCustomer(t, backend, nil)

	text, isErr := call(t, cs, "get_ordering_availability", map[string]any{})

	require.False(t, isErr, text)
	assert.Contains(t, text, `"payment_strategy":"manual"`)
	assert.NotContains(t, text, "payment_terms", "no terms for a manual restaurant")
}

func TestCustomerTools_DescriptionsTellTheManualRules(t *testing.T) {
	backend := newFakeBackend(t)
	cs := connectCustomer(t, backend, map[string]string{customerPhoneHeader: "573005551234"})
	tools, err := cs.ListTools(t.Context(), nil)
	require.NoError(t, err)
	descriptions := map[string]string{}
	for _, tl := range tools.Tools {
		descriptions[tl.Name] = tl.Description
	}

	assert.Contains(t, descriptions["create_delivery_order"], "payment_instructions")
	assert.Contains(t, descriptions["create_delivery_order"], "payment_expires_at")
	assert.Contains(t, descriptions["update_my_order"], "payment_instructions")
	assert.Contains(t, descriptions["get_my_orders"], "payment_under_review")
	assert.Contains(t, descriptions["get_ordering_availability"], "payment_strategy")
}

func TestCreateDeliveryOrder_TheTermsFlagReachesTheBackendAndTheQRComesBack(t *testing.T) {
	backend := newFakeBackend(t)
	backend.create = func(w http.ResponseWriter, _ *http.Request) {
		url, qr := "https://checkout.wompi.co/l/wl_A", "https://api.example.com/api/payments/qr/tok-A"
		order := dto.OpenBill{ID: uuid.NewString(), TotalAmount: decimal.NewFromInt(32000), PaymentURL: &url, PaymentQRURL: &qr}
		order.OrderType = dto.OrderTypeTakeaway
		backend.writeJSON(w, http.StatusCreated, order)
	}
	cs := connectCustomer(t, backend, map[string]string{customerPhoneHeader: "573005551234"})

	text, isErr := call(t, cs, "create_delivery_order", map[string]any{
		"order_type": "takeaway", "payment_terms_accepted": true, "lines": []map[string]any{{"product_id": "prod-Bandeja", "quantity": 1}},
	})

	require.False(t, isErr, text)
	assert.True(t, sentOrder(t, backend).PaymentTermsAccepted)
	assert.Contains(t, text, `"payment_qr_url":"https://api.example.com/api/payments/qr/tok-A"`)
	assert.Contains(t, text, `"payment_url":"https://checkout.wompi.co/l/wl_A"`)
}

func TestCreateDeliveryOrder_WithoutTheFlagTheBackendIsToldNo(t *testing.T) {
	backend := newFakeBackend(t)
	backend.create = func(w http.ResponseWriter, _ *http.Request) {
		order := dto.OpenBill{ID: uuid.NewString(), TotalAmount: decimal.NewFromInt(32000)}
		order.OrderType = dto.OrderTypeTakeaway
		backend.writeJSON(w, http.StatusCreated, order)
	}
	cs := connectCustomer(t, backend, map[string]string{customerPhoneHeader: "573005551234"})

	text, isErr := call(t, cs, "create_delivery_order", map[string]any{"order_type": "takeaway", "lines": []map[string]any{{"product_id": "prod-Bandeja", "quantity": 1}}})

	require.False(t, isErr, text)
	assert.False(t, sentOrder(t, backend).PaymentTermsAccepted)
	assert.NotContains(t, text, "payment_qr_url")
}

func TestGetOrderingAvailability_ReportsThePaymentTerms(t *testing.T) {
	wompi := dto.PaymentStrategyWompi
	backend := newFakeBackend(t)
	backend.settings = &dto.OrganizationSettingsResponse{
		Timezone: "America/Bogota", PaymentStrategy: &wompi,
		PaymentTerms: &dto.PaymentTerms{TermsURL: "https://wompi.com/terms.pdf", PersonalDataURL: "https://wompi.com/data.pdf"},
	}
	cs := connectCustomer(t, backend, nil)

	text, isErr := call(t, cs, "get_ordering_availability", map[string]any{})

	require.False(t, isErr, text)
	assert.Contains(t, text, `"payment_terms":{"terms_url":"https://wompi.com/terms.pdf","personal_data_url":"https://wompi.com/data.pdf"}`)
}

func TestGetMyOrders_AnAwaitingOrderShowsItsQR(t *testing.T) {
	backend := newFakeBackend(t)
	backend.orders = []*fakeOrder{{
		ID: uuid.NewString(), Phone: callerPhone, Type: dto.OrderTypeTakeaway, CreatedAt: time.Now().UTC().Add(-time.Minute),
		PaymentURL: "https://checkout.wompi.co/l/wl_A", PaymentQRURL: "https://api.example.com/api/payments/qr/tok-A",
	}}
	cs := connectCustomer(t, backend, map[string]string{customerPhoneHeader: "573005551234"})

	text, isErr := call(t, cs, "get_my_orders", nil)

	require.False(t, isErr, text)
	assert.Contains(t, text, `"payment_qr_url":"https://api.example.com/api/payments/qr/tok-A"`)
}

// sentOrder is the order request the tool posted to the backend.
func sentOrder(t *testing.T, backend *fakeBackend) dto.CreateOrderRequest {
	t.Helper()
	requests := backend.requestsTo(http.MethodPost, "/api/orders")
	require.Len(t, requests, 1)
	var sent dto.CreateOrderRequest
	require.NoError(t, json.Unmarshal(requests[0].Body, &sent))
	return sent
}
