package mcpserver

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strPtr(s string) *string { return &s }

func feePtr(v int64) *decimal.Decimal {
	d := decimal.NewFromInt(v)
	return &d
}

func TestCheckDeliveryLocation_QuotesTheZoneFee(t *testing.T) {
	tests := []struct {
		name   string
		result dto.DeliveryZoneCheckResult
		want   string
	}{
		{"a zone with a fee", dto.DeliveryZoneCheckResult{Deliverable: true, ZoneName: strPtr("Centro"), DeliveryFee: feePtr(5000)},
			`{"deliverable":true,"zone_name":"Centro","delivery_fee":"5000"}`},
		{"a free zone", dto.DeliveryZoneCheckResult{Deliverable: true, ZoneName: strPtr("Norte"), DeliveryFee: feePtr(0)},
			`{"deliverable":true,"zone_name":"Norte","delivery_fee":"0"}`},
		{"outside every zone", dto.DeliveryZoneCheckResult{Deliverable: false}, `{"deliverable":false}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			backend := newFakeBackend(t)
			backend.override = func(w http.ResponseWriter, r *http.Request) bool {
				if r.URL.Path != "/api/delivery-zones/check" {
					return false
				}
				backend.writeJSON(w, http.StatusOK, tc.result)
				return true
			}
			cs := connectCustomer(t, backend, map[string]string{customerLocationHeader: insidePin})

			text, isErr := call(t, cs, "check_delivery_location", nil)

			require.False(t, isErr, text)
			assert.JSONEq(t, tc.want, text)
		})
	}
}

func TestCheckDeliveryLocation_TellsTheAgentToQuoteTheFee(t *testing.T) {
	backend := newFakeBackend(t)
	cs := connectCustomer(t, backend, nil)

	description := toolDescription(t, cs, "check_delivery_location")

	assert.Contains(t, description, "delivery fee before creating the order")
	assert.Contains(t, description, "free")
}

func TestCreateDeliveryOrder_ReturnsTheFeeInTheTotal(t *testing.T) {
	backend := newFakeBackend(t)
	backend.create = func(w http.ResponseWriter, _ *http.Request) {
		var req dto.CreateOrderRequest
		require.NoError(t, json.Unmarshal(backend.lastBody(), &req))
		order := backend.echo(req.OpenBillID, req.OrderType, req.Products)
		order.TotalAmount = decimal.NewFromInt(37000)
		order.DeliveryFee = feePtr(5000)
		backend.writeJSON(w, http.StatusCreated, order)
	}
	cs := connectCustomer(t, backend, map[string]string{customerPhoneHeader: "573005551234", customerLocationHeader: insidePin})

	text, isErr := call(t, cs, "create_delivery_order", deliveryArgs())

	require.False(t, isErr, text)
	var view orderView
	require.NoError(t, json.Unmarshal([]byte(text), &view))
	require.NotNil(t, view.DeliveryFee)
	assert.True(t, decimal.NewFromInt(5000).Equal(*view.DeliveryFee))
	assert.True(t, decimal.NewFromInt(37000).Equal(view.Total))
	sent := string(backend.requestsTo(http.MethodPost, "/api/orders")[0].Body)
	assert.NotContains(t, sent, "delivery_fee", "the agent never sends a fee")
}

func TestCreateDeliveryOrder_TakesNoFeeArgument(t *testing.T) {
	cs := connectCustomer(t, newFakeBackend(t), nil)
	res, err := cs.ListTools(t.Context(), nil)
	require.NoError(t, err)
	for _, tool := range res.Tools {
		if tool.Name == "create_delivery_order" {
			schema, err := json.Marshal(tool.InputSchema)
			require.NoError(t, err)
			assert.NotContains(t, string(schema), "delivery_fee")
			return
		}
	}
	t.Fatal("create_delivery_order is not served")
}

func TestGetMyOrders_ADeliveryShowsItsFee(t *testing.T) {
	backend := newFakeBackend(t)
	id := uuid.NewString()
	backend.orders = append(backend.orders, &fakeOrder{ID: id, Phone: callerPhone, Type: dto.OrderTypeDelivery, CreatedAt: time.Now().UTC()})
	backend.override = func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != "/api/orders/"+id {
			return false
		}
		detail := dto.OpenBillWithProducts{ID: id, CreatedAt: time.Now().UTC(), TotalAmount: decimal.NewFromInt(37000)}
		detail.OrderType = dto.OrderTypeDelivery
		detail.DeliveryFee = feePtr(5000)
		backend.writeJSON(w, http.StatusOK, detail)
		return true
	}
	cs := connectCustomer(t, backend, map[string]string{customerPhoneHeader: "573005551234"})

	text, isErr := call(t, cs, "get_my_orders", nil)

	require.False(t, isErr, text)
	var result myOrders
	require.NoError(t, json.Unmarshal([]byte(text), &result))
	require.Len(t, result.Orders, 1)
	require.NotNil(t, result.Orders[0].DeliveryFee)
	assert.True(t, decimal.NewFromInt(5000).Equal(*result.Orders[0].DeliveryFee))
	assert.True(t, decimal.NewFromInt(37000).Equal(result.Orders[0].Total), "the total includes it")
	assert.True(t, strings.Contains(text, `"delivery_fee":"5000"`), text)
}
