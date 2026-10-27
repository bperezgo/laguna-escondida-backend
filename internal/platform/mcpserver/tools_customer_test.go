package mcpserver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"laguna-escondida/backend/internal/domain/dto"
)

const (
	callerPhone = "+573005551234"
	otherPhone  = "+573119998877"
	insidePin   = "4.5981,-74.076"
)

// ---------------------------------------------------------------------------
// Fake backend: records every non-signin request and serves a small order store.
// ---------------------------------------------------------------------------

type recordedRequest struct {
	Method string
	Path   string
	Query  string
	Body   []byte
}

type fakeOrder struct {
	ID        string
	Phone     string
	Type      dto.OrderType
	CreatedAt time.Time
	Voided    bool
	Paid      bool
	Lines     []dto.OpenBillProductDetail
}

type fakeBackend struct {
	t        *testing.T
	mu       sync.Mutex
	requests []recordedRequest
	orders   []*fakeOrder
	products []*dto.Product
	sides    map[string][]*dto.SideDishOption
	settings *dto.OrganizationSettingsResponse
	create   http.HandlerFunc
	server   *httptest.Server
}

func newFakeBackend(t *testing.T) *fakeBackend {
	b := &fakeBackend{t: t, sides: map[string][]*dto.SideDishOption{}}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/signin", func(w http.ResponseWriter, _ *http.Request) { writeSignIn(w) })
	mux.HandleFunc("GET /api/products", func(w http.ResponseWriter, _ *http.Request) {
		b.writeJSON(w, http.StatusOK, dto.ProductListResponse{Products: b.products})
	})
	mux.HandleFunc("GET /api/products/{id}/side-dishes", func(w http.ResponseWriter, r *http.Request) {
		b.writeJSON(w, http.StatusOK, dto.SideDishOptionListResponse{SideDishes: b.sides[r.PathValue("id")]})
	})
	mux.HandleFunc("GET /api/delivery-zones/check", func(w http.ResponseWriter, _ *http.Request) {
		name := "Centro"
		b.writeJSON(w, http.StatusOK, dto.DeliveryZoneCheckResult{Deliverable: true, ZoneName: &name})
	})
	mux.HandleFunc("GET /api/organization/settings", func(w http.ResponseWriter, _ *http.Request) {
		if b.settings == nil {
			b.writeJSON(w, http.StatusForbidden, map[string]string{"error": "Insufficient permissions"})
			return
		}
		b.writeJSON(w, http.StatusOK, b.settings)
	})
	mux.HandleFunc("GET /api/orders", func(w http.ResponseWriter, r *http.Request) {
		b.writeJSON(w, http.StatusOK, b.list(r.URL.Query().Get("customer_phone"), false))
	})
	mux.HandleFunc("GET /api/orders/closed", func(w http.ResponseWriter, r *http.Request) {
		b.writeJSON(w, http.StatusOK, b.list(r.URL.Query().Get("customer_phone"), true))
	})
	mux.HandleFunc("GET /api/orders/{id}", func(w http.ResponseWriter, r *http.Request) {
		b.writeDetail(w, r.PathValue("id"), false)
	})
	mux.HandleFunc("GET /api/orders/closed/{id}", func(w http.ResponseWriter, r *http.Request) {
		b.writeDetail(w, r.PathValue("id"), true)
	})
	mux.HandleFunc("POST /api/orders", func(w http.ResponseWriter, r *http.Request) {
		if b.create != nil {
			b.create(w, r)
			return
		}
		var req dto.CreateOrderRequest
		require.NoError(t, json.Unmarshal(b.lastBody(), &req))
		b.writeJSON(w, http.StatusCreated, b.echo(req.OpenBillID, req.OrderType, req.Products))
	})
	mux.HandleFunc("PUT /api/orders/{id}", func(w http.ResponseWriter, r *http.Request) {
		var req dto.UpdateOrderRequest
		require.NoError(t, json.Unmarshal(b.lastBody(), &req))
		b.writeJSON(w, http.StatusOK, b.echo(r.PathValue("id"), dto.OrderTypeDelivery, req.Products))
	})
	mux.HandleFunc("DELETE /api/orders/{id}", func(w http.ResponseWriter, _ *http.Request) {
		b.writeJSON(w, http.StatusOK, map[string]string{"message": "Order deleted successfully"})
	})

	b.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/auth/signin" {
			body, _ := io.ReadAll(r.Body)
			b.mu.Lock()
			b.requests = append(b.requests, recordedRequest{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Body: body})
			b.mu.Unlock()
		}
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(b.server.Close)
	return b
}

func (b *fakeBackend) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	require.NoError(b.t, json.NewEncoder(w).Encode(v))
}

func (b *fakeBackend) lastBody() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.requests[len(b.requests)-1].Body
}

func (b *fakeBackend) recorded() []recordedRequest {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]recordedRequest(nil), b.requests...)
}

func (b *fakeBackend) requestsTo(method, prefix string) []recordedRequest {
	var out []recordedRequest
	for _, r := range b.recorded() {
		if r.Method == method && strings.HasPrefix(r.Path, prefix) {
			out = append(out, r)
		}
	}
	return out
}

func (b *fakeBackend) list(phone string, closed bool) dto.OpenBillListResponse {
	var out []*dto.OpenBillWithCreator
	for _, o := range b.orders {
		isClosed := o.Paid || o.Voided
		if o.Phone != phone || isClosed != closed {
			continue
		}
		item := &dto.OpenBillWithCreator{ID: o.ID, CreatedAt: o.CreatedAt}
		if o.Paid {
			paidAt := o.CreatedAt.Add(time.Minute)
			item.PaidAt = &paidAt
		}
		out = append(out, item)
	}
	return dto.OpenBillListResponse{OpenBills: out}
}

func (b *fakeBackend) writeDetail(w http.ResponseWriter, id string, closed bool) {
	for _, o := range b.orders {
		if o.ID != id || (o.Paid || o.Voided) != closed {
			continue
		}
		detail := dto.OpenBillWithProducts{ID: o.ID, CreatedAt: o.CreatedAt, TotalAmount: decimal.NewFromInt(32000), Products: o.Lines}
		detail.OrderType = o.Type
		if o.Paid {
			paidAt := o.CreatedAt.Add(time.Minute)
			detail.PaidAt = &paidAt
		}
		b.writeJSON(w, http.StatusOK, detail)
		return
	}
	b.writeJSON(w, http.StatusNotFound, map[string]string{"error": "Order not found"})
}

func (b *fakeBackend) echo(id string, orderType dto.OrderType, items []dto.OrderProductItem) dto.OpenBill {
	order := dto.OpenBill{ID: id, TotalAmount: decimal.NewFromInt(32000)}
	order.OrderType = orderType
	for _, item := range items {
		order.Products = append(order.Products, dto.Product{ID: item.ProductID, Name: "Plato " + item.ProductID[:4]})
	}
	return order
}

func line(id, name string, qty int, status dto.CommandStatus) dto.OpenBillProductDetail {
	return dto.OpenBillProductDetail{OpenBillProductID: id, Product: dto.Product{ID: "prod-" + name, Name: name}, Quantity: qty, Status: status}
}

// ---------------------------------------------------------------------------
// MCP sessions
// ---------------------------------------------------------------------------

type headerTransport struct {
	headers map[string]string
	next    http.RoundTripper
}

func (h headerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	for k, v := range h.headers {
		r.Header.Set(k, v)
	}
	return h.next.RoundTrip(r)
}

// connectCustomer serves the customer toolset over streamable HTTP, as in production, with the
// agent runtime's headers set on every request.
func connectCustomer(t *testing.T, backend *fakeBackend, headers map[string]string) *mcp.ClientSession {
	t.Helper()
	server := NewMCPServer(testClient(backend.server.URL), ToolsetCustomer)
	mcpHTTP := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil))
	t.Cleanup(mcpHTTP.Close)

	client := mcp.NewClient(&mcp.Implementation{Name: "agent-runtime", Version: "test"}, nil)
	cs, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:   mcpHTTP.URL,
		HTTPClient: &http.Client{Transport: headerTransport{headers: headers, next: http.DefaultTransport}},
	}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func connectInMemory(t *testing.T, s *mcp.Server) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	serverT, clientT := mcp.NewInMemoryTransports()
	_, err := s.Connect(ctx, serverT, nil)
	require.NoError(t, err)
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "test"}, nil).Connect(ctx, clientT, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func call(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) (string, bool) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	require.NoError(t, err)
	return toolText(t, res), res.IsError
}

func toolNames(t *testing.T, cs *mcp.ClientSession) []string {
	t.Helper()
	res, err := cs.ListTools(context.Background(), nil)
	require.NoError(t, err)
	names := make([]string, 0, len(res.Tools))
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
	}
	sort.Strings(names)
	return names
}

func isUUIDv7(id string) bool {
	parsed, err := uuid.Parse(id)
	return err == nil && parsed.Version() == 7
}

// ---------------------------------------------------------------------------
// 7.1 Toolset selection
// ---------------------------------------------------------------------------

var customerToolNames = []string{
	"cancel_my_order", "check_delivery_location", "create_delivery_order", "get_my_orders",
	"get_ordering_availability", "get_side_dish_options", "list_menu", "update_my_order",
}

func TestNewMCPServer_CustomerToolsetIsExactlyTheEightTools(t *testing.T) {
	cs := connectInMemory(t, NewMCPServer(testClient("http://backend.invalid"), ToolsetCustomer))

	assert.Equal(t, customerToolNames, toolNames(t, cs))
}

func TestNewMCPServer_FullToolsetKeepsStaffToolsAndAddsZones(t *testing.T) {
	names := toolNames(t, connectInMemory(t, NewMCPServer(testClient("http://backend.invalid"), ToolsetFull)))

	for _, name := range []string{"pay_order", "delete_order", "list_active_orders", "list_delivery_zones", "create_delivery_zone", "update_delivery_zone", "delete_delivery_zone"} {
		assert.Contains(t, names, name)
	}
	for _, name := range customerToolNames {
		assert.NotContains(t, names, name, "customer tools are not in the staff toolset")
	}
}

func TestCustomerToolset_StaffToolIsUnknown(t *testing.T) {
	backend := newFakeBackend(t)
	cs := connectCustomer(t, backend, map[string]string{customerPhoneHeader: "573005551234"})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "pay_order", Arguments: map[string]any{}})

	if err == nil {
		assert.True(t, res.IsError, "pay_order must not run on the customer server")
	}
	assert.Empty(t, backend.recorded(), "nothing reaches the backend")
}

func TestNewConfig_Toolset(t *testing.T) {
	t.Setenv("LAGUNA_API_URL", "http://localhost:8080")
	t.Setenv("LAGUNA_USERNAME", "svc")
	t.Setenv("LAGUNA_PASSWORD", "secret")

	t.Setenv("MCP_TOOLSET", "")
	cfg, err := NewConfig()
	require.NoError(t, err)
	assert.Equal(t, ToolsetFull, cfg.Toolset, "unset means full")

	t.Setenv("MCP_TOOLSET", "customer")
	cfg, err = NewConfig()
	require.NoError(t, err)
	assert.Equal(t, ToolsetCustomer, cfg.Toolset)
	assert.Error(t, cfg.ValidateTransport(true), "customer is HTTP only")
	assert.NoError(t, cfg.ValidateTransport(false))

	t.Setenv("MCP_TOOLSET", "staff")
	_, err = NewConfig()
	assert.Error(t, err, "an unknown toolset stops startup")
}

// ---------------------------------------------------------------------------
// 7.2 Zone tools (full toolset)
// ---------------------------------------------------------------------------

func TestZoneTools_PathsAndBodies(t *testing.T) {
	var got []recordedRequest
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/signin", func(w http.ResponseWriter, _ *http.Request) { writeSignIn(w) })
	mux.HandleFunc("/api/delivery-zones/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got = append(got, recordedRequest{Method: r.Method, Path: r.URL.Path, Body: body})
		_, _ = w.Write([]byte(`{}`))
	})
	mux.HandleFunc("/api/delivery-zones", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got = append(got, recordedRequest{Method: r.Method, Path: r.URL.Path, Body: body})
		_, _ = w.Write([]byte(`{}`))
	})
	backend := httptest.NewServer(mux)
	defer backend.Close()
	cs := connectInMemory(t, NewMCPServer(testClient(backend.URL), ToolsetFull))
	boundary := `{"type":"Polygon","coordinates":[[[-74.1,4.58],[-74.06,4.58],[-74.06,4.62],[-74.1,4.62],[-74.1,4.58]]]}`

	_, isErr := call(t, cs, "list_delivery_zones", nil)
	require.False(t, isErr)
	_, isErr = call(t, cs, "create_delivery_zone", map[string]any{"name": "Centro", "boundary": boundary})
	require.False(t, isErr)
	_, isErr = call(t, cs, "update_delivery_zone", map[string]any{"id": "zone-1", "active": false})
	require.False(t, isErr)
	_, isErr = call(t, cs, "delete_delivery_zone", map[string]any{"id": "zone-1"})
	require.False(t, isErr)
	text, isErr := call(t, cs, "create_delivery_zone", map[string]any{"name": "Rota", "boundary": "not json"})
	require.True(t, isErr)
	assert.Contains(t, text, "not valid JSON")

	require.Len(t, got, 4, "the invalid boundary never reaches the backend")
	assert.Equal(t, "GET /api/delivery-zones", got[0].Method+" "+got[0].Path)
	assert.Equal(t, "POST /api/delivery-zones", got[1].Method+" "+got[1].Path)
	assert.JSONEq(t, `{"name":"Centro","boundary":`+boundary+`}`, string(got[1].Body), "the boundary is sent as a JSON object")
	assert.Equal(t, "PUT /api/delivery-zones/zone-1", got[2].Method+" "+got[2].Path)
	assert.JSONEq(t, `{"active":false}`, string(got[2].Body))
	assert.Equal(t, "DELETE /api/delivery-zones/zone-1", got[3].Method+" "+got[3].Path)
}

// ---------------------------------------------------------------------------
// 7.3 Header helpers
// ---------------------------------------------------------------------------

func TestCustomerTools_HeaderFailuresNeverReachTheBackend(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		tool    string
		args    map[string]any
		wantErr string
	}{
		{"phone missing", nil, "get_my_orders", nil, "customer phone is missing"},
		{"phone invalid", map[string]string{customerPhoneHeader: "12345"}, "get_my_orders", nil, "not a valid international number"},
		{"phone missing on cancel", nil, "cancel_my_order", map[string]any{"order_id": "o-1"}, "customer phone is missing"},
		{"location missing", map[string]string{customerPhoneHeader: "573005551234"}, "check_delivery_location", nil, "share their location"},
		{"location not a pair", map[string]string{customerLocationHeader: "4.59"}, "check_delivery_location", nil, "is invalid"},
		{"location not numbers", map[string]string{customerLocationHeader: "north,west"}, "check_delivery_location", nil, "is invalid"},
		{"latitude out of range", map[string]string{customerLocationHeader: "95,-74.08"}, "check_delivery_location", nil, "is invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := newFakeBackend(t)
			cs := connectCustomer(t, backend, tt.headers)

			text, isErr := call(t, cs, tt.tool, tt.args)

			assert.True(t, isErr)
			assert.Contains(t, text, tt.wantErr)
			assert.Empty(t, backend.recorded(), "zero backend requests")
		})
	}
}

func TestCustomerTools_ForeignSenderIsReadAsInternational(t *testing.T) {
	backend := newFakeBackend(t)
	cs := connectCustomer(t, backend, map[string]string{customerPhoneHeader: "34612345678"})

	_, isErr := call(t, cs, "get_my_orders", nil)

	require.False(t, isErr)
	lists := backend.requestsTo(http.MethodGet, "/api/orders")
	require.NotEmpty(t, lists)
	assert.Equal(t, "customer_phone=%2B34612345678", lists[0].Query)
}

// ---------------------------------------------------------------------------
// 7.4 Menu and location
// ---------------------------------------------------------------------------

func TestListMenu_OnlySellableWithSideDishFlag(t *testing.T) {
	backend := newFakeBackend(t)
	desc := "Con arroz y patacón"
	backend.products = []*dto.Product{
		{ID: "plate", Name: "Bandeja", Category: "Platos", ProductType: dto.ProductTypeComposite, Description: &desc, TotalPriceWithTaxes: decimal.NewFromInt(32000)},
		{ID: "simple-plate", Name: "Arepa", Category: "Platos", ProductType: dto.ProductTypeComposite, TotalPriceWithTaxes: decimal.NewFromInt(9000)},
		{ID: "soda", Name: "Gaseosa", Category: "Bebidas", ProductType: dto.ProductTypeSellable, TotalPriceWithTaxes: decimal.NewFromInt(5000)},
		{ID: "tomato", Name: "Tomate", Category: "Insumos", ProductType: dto.ProductTypeIngredient},
	}
	backend.sides["plate"] = []*dto.SideDishOption{{IngredientProductID: "rice", IngredientProduct: &dto.Product{Name: "Arroz"}, DefaultQuantity: 1, MaxQuantity: 2}}
	cs := connectCustomer(t, backend, map[string]string{customerPhoneHeader: "573005551234"})

	text, isErr := call(t, cs, "list_menu", nil)

	require.False(t, isErr, text)
	var menu []menuItem
	require.NoError(t, json.Unmarshal([]byte(text), &menu))
	byID := map[string]menuItem{}
	for _, m := range menu {
		byID[m.ProductID] = m
	}
	assert.NotContains(t, byID, "tomato", "an ingredient is never on the menu")
	require.Len(t, byID, 3)
	assert.True(t, byID["plate"].HasSideDishOptions)
	assert.False(t, byID["simple-plate"].HasSideDishOptions)
	assert.False(t, byID["soda"].HasSideDishOptions)
	assert.True(t, byID["plate"].Price.Equal(decimal.NewFromInt(32000)))
	assert.Equal(t, "product_type=SELLABLE%2CCOMPOSITE%2CBOTH", backend.requestsTo(http.MethodGet, "/api/products")[0].Query)
	assert.Empty(t, backend.requestsTo(http.MethodGet, "/api/products/soda"), "only composites are asked for side dishes")
}

func TestGetSideDishOptions_Trimmed(t *testing.T) {
	backend := newFakeBackend(t)
	backend.sides["plate"] = []*dto.SideDishOption{{IngredientProductID: "rice", IngredientProduct: &dto.Product{Name: "Arroz", SKU: "secret-sku"}, DefaultQuantity: 1, MinQuantity: 0, MaxQuantity: 2}}
	cs := connectCustomer(t, backend, nil)

	text, isErr := call(t, cs, "get_side_dish_options", map[string]any{"product_id": "plate"})

	require.False(t, isErr, text)
	assert.JSONEq(t, `[{"ingredient_product_id":"rice","name":"Arroz","default_quantity":1,"min_quantity":0,"max_quantity":2}]`, text)
}

func TestCheckDeliveryLocation_UsesTheHeaderPin(t *testing.T) {
	backend := newFakeBackend(t)
	cs := connectCustomer(t, backend, map[string]string{customerLocationHeader: " 4.5981 , -74.076 "})

	text, isErr := call(t, cs, "check_delivery_location", nil)

	require.False(t, isErr, text)
	assert.JSONEq(t, `{"deliverable":true,"zone_name":"Centro"}`, text)
	checks := backend.requestsTo(http.MethodGet, "/api/delivery-zones/check")
	require.Len(t, checks, 1)
	assert.Equal(t, "lat=4.5981&lng=-74.076", checks[0].Query)
}

// ---------------------------------------------------------------------------
// 7.5 create_delivery_order
// ---------------------------------------------------------------------------

func deliveryArgs() map[string]any {
	return map[string]any{
		"order_type":       "delivery",
		"customer_name":    "Ana",
		"delivery_address": "Cra 7 # 12-34, apto 501",
		"lines": []map[string]any{
			{"product_id": "11112222-0000-0000-0000-000000000001", "quantity": 2, "notes": "sin cebolla"},
			{"product_id": "33334444-0000-0000-0000-000000000002", "quantity": 1,
				"side_dishes": []map[string]any{{"ingredient_product_id": "rice", "quantity": 0}}},
		},
	}
}

func TestCreateDeliveryOrder_PostsHeaderPhoneAndPinWithGeneratedIDs(t *testing.T) {
	backend := newFakeBackend(t)
	cs := connectCustomer(t, backend, map[string]string{customerPhoneHeader: "573005551234", customerLocationHeader: insidePin})

	text, isErr := call(t, cs, "create_delivery_order", deliveryArgs())

	require.False(t, isErr, text)
	posts := backend.requestsTo(http.MethodPost, "/api/orders")
	require.Len(t, posts, 1)
	var sent dto.CreateOrderRequest
	require.NoError(t, json.Unmarshal(posts[0].Body, &sent))
	assert.Equal(t, dto.OrderTypeDelivery, sent.OrderType)
	assert.Equal(t, callerPhone, *sent.CustomerPhone, "phone from the header, normalized")
	assert.Equal(t, "4.5981", *sent.DeliveryLat)
	assert.Equal(t, "-74.076", *sent.DeliveryLng)
	assert.Equal(t, "Cra 7 # 12-34, apto 501", *sent.DeliveryAddress)
	assert.Equal(t, "Ana", *sent.CustomerName)
	assert.True(t, isUUIDv7(sent.OpenBillID))
	assert.True(t, isUUIDv7(sent.TemporalIdentifier))
	require.Len(t, sent.Products, 2)
	for _, p := range sent.Products {
		assert.True(t, isUUIDv7(p.OpenBillProductID))
	}
	assert.Equal(t, []dto.SideDishSelection{{IngredientProductID: "rice", Quantity: 0}}, sent.Products[1].SideDishes)

	var view orderView
	require.NoError(t, json.Unmarshal([]byte(text), &view))
	assert.Equal(t, sent.OpenBillID, view.OrderID)
	assert.Equal(t, paymentAwaiting, view.PaymentState)
	require.Len(t, view.Lines, 2)
	assert.Equal(t, 2, view.Lines[0].Quantity)
	assert.NotEmpty(t, view.Lines[0].Name)
}

func TestCreateDeliveryOrder_TakeawayNeedsNoPin(t *testing.T) {
	backend := newFakeBackend(t)
	cs := connectCustomer(t, backend, map[string]string{customerPhoneHeader: "573005551234"})
	args := deliveryArgs()
	args["order_type"] = "takeaway"

	text, isErr := call(t, cs, "create_delivery_order", args)

	require.False(t, isErr, text)
	var sent dto.CreateOrderRequest
	require.NoError(t, json.Unmarshal(backend.requestsTo(http.MethodPost, "/api/orders")[0].Body, &sent))
	assert.Nil(t, sent.DeliveryLat)
	assert.Nil(t, sent.DeliveryAddress)
}

func TestCreateDeliveryOrder_RejectedBeforeTheBackend(t *testing.T) {
	both := map[string]string{customerPhoneHeader: "573005551234", customerLocationHeader: insidePin}
	tests := []struct {
		name    string
		headers map[string]string
		mutate  func(map[string]any)
		wantErr string
	}{
		{"dine-in", both, func(a map[string]any) { a["order_type"] = "dine_in" }, "takeaway or delivery"},
		{"delivery without a pin", map[string]string{customerPhoneHeader: "573005551234"}, func(map[string]any) {}, "share their location"},
		{"delivery without address", both, func(a map[string]any) { delete(a, "delivery_address") }, "delivery_address"},
		{"no lines", both, func(a map[string]any) { a["lines"] = []map[string]any{} }, "at least one line"},
		{"zero quantity", both, func(a map[string]any) {
			a["lines"] = []map[string]any{{"product_id": "p", "quantity": 0}}
		}, "at least 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := newFakeBackend(t)
			cs := connectCustomer(t, backend, tt.headers)
			args := deliveryArgs()
			tt.mutate(args)

			text, isErr := call(t, cs, "create_delivery_order", args)

			assert.True(t, isErr)
			assert.Contains(t, text, tt.wantErr)
			assert.Empty(t, backend.recorded(), "no order is created")
		})
	}
}

func TestCreateDeliveryOrder_BackendErrorsStateTheReason(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr string
	}{
		{"outside zone", http.StatusUnprocessableEntity, `{"error":"The delivery location is outside every delivery zone"}`, "outside delivery zone"},
		{"unknown product", http.StatusNotFound, `{"error":"One or more products not found"}`, "One or more products not found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := newFakeBackend(t)
			backend.create = func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}
			cs := connectCustomer(t, backend, map[string]string{customerPhoneHeader: "573005551234", customerLocationHeader: insidePin})

			text, isErr := call(t, cs, "create_delivery_order", deliveryArgs())

			assert.True(t, isErr)
			assert.Contains(t, text, tt.wantErr)
			assert.NotContains(t, text, "/api/orders", "the backend path is not leaked to the model")
		})
	}
}

func TestCreateDeliveryOrder_OrderingUnavailableStatesWhy(t *testing.T) {
	tests := []struct {
		name     string
		args     map[string]any
		body     string
		wantErr  []string
		withheld string
	}{
		{
			name:     "delivery paused between the check and the order",
			args:     deliveryArgs(),
			body:     `{"error":"Delivery orders are paused","code":"ORDERING_PAUSED","order_type":"delivery","paused_reason":"cocina saturada"}`,
			wantErr:  []string{"delivery orders are paused", "cocina saturada", "no order was created"},
			withheld: "outside delivery zone",
		},
		{
			name: "takeaway after hours",
			args: map[string]any{"order_type": "takeaway", "lines": []map[string]any{{"product_id": "p-1", "quantity": 1}}},
			body: `{"error":"Takeaway orders are outside their hours","code":"OUTSIDE_ORDERING_HOURS","order_type":"takeaway",
				"next_opens_at":"2026-11-02T11:30:00-05:00"}`,
			wantErr:  []string{"takeaway is closed right now", "2026-11-02T11:30:00-05:00", "no order was created"},
			withheld: "outside delivery zone",
		},
		{
			name:     "outside hours with nothing scheduled this week",
			args:     deliveryArgs(),
			body:     `{"error":"Delivery orders are outside their hours","code":"OUTSIDE_ORDERING_HOURS","order_type":"delivery","next_opens_at":null}`,
			wantErr:  []string{"delivery is closed right now", "no opening is scheduled in the next seven days"},
			withheld: "outside delivery zone",
		},
		{
			name:     "paused without a reason",
			args:     deliveryArgs(),
			body:     `{"error":"Delivery orders are paused","code":"ORDERING_PAUSED","order_type":"delivery","paused_reason":null}`,
			wantErr:  []string{"delivery orders are paused", "no reason given"},
			withheld: "outside delivery zone",
		},
		{
			name:     "zone code still reads as outside the zone",
			args:     deliveryArgs(),
			body:     `{"error":"The delivery location is outside every delivery zone","code":"OUTSIDE_DELIVERY_ZONE"}`,
			wantErr:  []string{"outside delivery zone"},
			withheld: "paused",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := newFakeBackend(t)
			backend.create = func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnprocessableEntity)
				_, _ = w.Write([]byte(tt.body))
			}
			cs := connectCustomer(t, backend, map[string]string{customerPhoneHeader: "573005551234", customerLocationHeader: insidePin})

			text, isErr := call(t, cs, "create_delivery_order", tt.args)

			assert.True(t, isErr)
			for _, want := range tt.wantErr {
				assert.Contains(t, text, want)
			}
			assert.NotContains(t, text, tt.withheld)
		})
	}
}

// ---------------------------------------------------------------------------
// get_ordering_availability
// ---------------------------------------------------------------------------

func TestGetOrderingAvailability_ReturnsAvailabilityAndTimezoneOnly(t *testing.T) {
	bogota := time.FixedZone("-05", -5*3600)
	closes := time.Date(2026, 10, 26, 22, 30, 0, 0, bogota)
	paused, reason := dto.AvailabilityReasonPaused, "cocina saturada"
	backend := newFakeBackend(t)
	backend.settings = &dto.OrganizationSettingsResponse{
		Ordering: dto.OrderingSettings{Takeaway: dto.OrderTypeSettings{Schedule: []dto.OpeningWindow{{Day: "mon", Opens: "11:30", Closes: "22:30"}}}},
		Availability: dto.OrderingAvailability{
			Delivery: dto.Availability{Reason: &paused, PausedReason: &reason},
			Takeaway: dto.Availability{Available: true, ClosesAt: &closes},
		},
		Timezone: "America/Bogota",
	}
	// No phone or location header: the tool needs neither.
	cs := connectCustomer(t, backend, nil)

	text, isErr := call(t, cs, "get_ordering_availability", map[string]any{})

	require.False(t, isErr, text)
	assert.JSONEq(t, `{
		"delivery": {"available": false, "reason": "paused", "paused_reason": "cocina saturada", "closes_at": null, "next_opens_at": null},
		"takeaway": {"available": true, "reason": null, "paused_reason": null, "closes_at": "2026-10-26T22:30:00-05:00", "next_opens_at": null},
		"timezone": "America/Bogota"
	}`, text)
	assert.NotContains(t, text, "schedule", "schedules stay internal")
	requests := backend.requestsTo(http.MethodGet, "/api/organization/settings")
	require.Len(t, requests, 1)
	assert.Len(t, backend.recorded(), 1, "it creates nothing")
}

func TestGetOrderingAvailability_ClosedForTheNight(t *testing.T) {
	bogota := time.FixedZone("-05", -5*3600)
	outside := dto.AvailabilityReasonOutsideHours
	deliveryOpens := time.Date(2026, 10, 27, 18, 0, 0, 0, bogota)
	takeawayOpens := time.Date(2026, 10, 27, 11, 30, 0, 0, bogota)
	backend := newFakeBackend(t)
	backend.settings = &dto.OrganizationSettingsResponse{
		Availability: dto.OrderingAvailability{
			Delivery: dto.Availability{Reason: &outside, NextOpensAt: &deliveryOpens},
			Takeaway: dto.Availability{Reason: &outside, NextOpensAt: &takeawayOpens},
		},
		Timezone: "America/Bogota",
	}
	cs := connectCustomer(t, backend, nil)

	text, isErr := call(t, cs, "get_ordering_availability", map[string]any{})

	require.False(t, isErr, text)
	var got orderingAvailability
	require.NoError(t, json.Unmarshal([]byte(text), &got))
	assert.Equal(t, outside, *got.Delivery.Reason)
	assert.Equal(t, outside, *got.Takeaway.Reason)
	assert.True(t, deliveryOpens.Equal(*got.Delivery.NextOpensAt))
	assert.True(t, takeawayOpens.Equal(*got.Takeaway.NextOpensAt))
}

func TestGetOrderingAvailability_BackendRefusal(t *testing.T) {
	backend := newFakeBackend(t)
	cs := connectCustomer(t, backend, nil)

	text, isErr := call(t, cs, "get_ordering_availability", map[string]any{})

	assert.True(t, isErr)
	assert.Contains(t, text, "the ordering availability could not be loaded: Insufficient permissions")
}

func TestGetOrderingAvailability_DescriptionAsksToCallItFirst(t *testing.T) {
	cs := connectInMemory(t, NewMCPServer(testClient("http://backend.invalid"), ToolsetCustomer))
	res, err := cs.ListTools(context.Background(), nil)
	require.NoError(t, err)
	for _, tool := range res.Tools {
		if tool.Name == "get_ordering_availability" {
			assert.Contains(t, tool.Description, "Call it before taking an order")
			assert.Contains(t, tool.Description, "offer the other one")
			return
		}
	}
	t.Fatal("get_ordering_availability is not registered")
}

// ---------------------------------------------------------------------------
// 7.6 update_my_order / cancel_my_order
// ---------------------------------------------------------------------------

func ordersOfTwoCustomers(b *fakeBackend) (own, others, paid string) {
	own, others, paid = uuid.NewString(), uuid.NewString(), uuid.NewString()
	now := time.Now().UTC()
	b.orders = []*fakeOrder{
		{ID: own, Phone: callerPhone, Type: dto.OrderTypeDelivery, CreatedAt: now.Add(-10 * time.Minute),
			Lines: []dto.OpenBillProductDetail{line("line-1", "Bandeja", 1, dto.CommandStatusCreated)}},
		{ID: others, Phone: otherPhone, Type: dto.OrderTypeDelivery, CreatedAt: now.Add(-5 * time.Minute)},
		{ID: paid, Phone: callerPhone, Type: dto.OrderTypeTakeaway, CreatedAt: now.Add(-time.Hour), Paid: true,
			Lines: []dto.OpenBillProductDetail{
				line("line-2", "Bandeja", 1, dto.CommandStatusCompleted),
				line("line-3", "Limonada", 1, dto.CommandStatusInProgress),
			}},
	}
	return own, others, paid
}

func TestUpdateMyOrder_OwnUnpaidOrder(t *testing.T) {
	backend := newFakeBackend(t)
	own, _, _ := ordersOfTwoCustomers(backend)
	cs := connectCustomer(t, backend, map[string]string{customerPhoneHeader: "573005551234"})

	text, isErr := call(t, cs, "update_my_order", map[string]any{"order_id": own, "lines": []map[string]any{
		{"product_id": "prod-Bandeja", "quantity": 1},
		{"product_id": "prod-Limonada", "quantity": 2},
	}})

	require.False(t, isErr, text)
	puts := backend.requestsTo(http.MethodPut, "/api/orders/"+own)
	require.Len(t, puts, 1)
	var sent dto.UpdateOrderRequest
	require.NoError(t, json.Unmarshal(puts[0].Body, &sent))
	require.Len(t, sent.Products, 2)
	assert.Equal(t, "line-1", sent.Products[0].OpenBillProductID, "an existing line keeps its id")
	assert.True(t, isUUIDv7(sent.Products[1].OpenBillProductID), "a new line gets a generated id")
	assert.Nil(t, sent.TemporalIdentifier)
	assert.Contains(t, text, paymentAwaiting)
}

func TestCancelMyOrder_OwnUnpaidOrder(t *testing.T) {
	backend := newFakeBackend(t)
	own, _, _ := ordersOfTwoCustomers(backend)
	cs := connectCustomer(t, backend, map[string]string{customerPhoneHeader: "573005551234"})

	text, isErr := call(t, cs, "cancel_my_order", map[string]any{"order_id": own})

	require.False(t, isErr, text)
	assert.Len(t, backend.requestsTo(http.MethodDelete, "/api/orders/"+own), 1)
}

func TestUpdateAndCancel_NotMyOrderOrPaidIsNotFound(t *testing.T) {
	backend := newFakeBackend(t)
	_, others, paid := ordersOfTwoCustomers(backend)
	cs := connectCustomer(t, backend, map[string]string{customerPhoneHeader: "573005551234"})

	var messages []string
	for _, id := range []string{others, paid, "not-a-real-id"} {
		text, isErr := call(t, cs, "cancel_my_order", map[string]any{"order_id": id})
		require.True(t, isErr)
		messages = append(messages, text)
		text, isErr = call(t, cs, "update_my_order", map[string]any{"order_id": id, "lines": []map[string]any{{"product_id": "p", "quantity": 1}}})
		require.True(t, isErr)
		messages = append(messages, text)
	}

	for _, m := range messages {
		assert.Equal(t, errOrderNotFound.Error(), m, "the same answer whether the order is another customer's, paid, or absent")
	}
	assert.Empty(t, backend.requestsTo(http.MethodPut, "/api/orders"))
	assert.Empty(t, backend.requestsTo(http.MethodDelete, "/api/orders"))
	for _, r := range backend.requestsTo(http.MethodGet, "/api/orders") {
		assert.Equal(t, "customer_phone=%2B573005551234", r.Query, "only the caller's list is read")
	}
}

// ---------------------------------------------------------------------------
// 7.7 get_my_orders
// ---------------------------------------------------------------------------

func TestGetMyOrders_OnlyTheCallersNonVoidedOrders(t *testing.T) {
	backend := newFakeBackend(t)
	own, others, paid := ordersOfTwoCustomers(backend)
	voided := uuid.NewString()
	backend.orders = append(backend.orders, &fakeOrder{ID: voided, Phone: callerPhone, Type: dto.OrderTypeDelivery, CreatedAt: time.Now().UTC(), Voided: true})
	cs := connectCustomer(t, backend, map[string]string{customerPhoneHeader: "573005551234"})

	text, isErr := call(t, cs, "get_my_orders", nil)

	require.False(t, isErr, text)
	var result myOrders
	require.NoError(t, json.Unmarshal([]byte(text), &result))
	require.Len(t, result.Orders, 2)
	assert.Equal(t, own, result.Orders[0].OrderID, "newest first")
	assert.Equal(t, paymentAwaiting, result.Orders[0].PaymentState)
	assert.Equal(t, paid, result.Orders[1].OrderID)
	assert.Equal(t, paymentPaid, result.Orders[1].PaymentState)
	require.Len(t, result.Orders[1].Lines, 2)
	assert.Equal(t, "completed", result.Orders[1].Lines[0].State)
	assert.Equal(t, "in_progress", result.Orders[1].Lines[1].State)

	for _, r := range backend.recorded() {
		assert.NotContains(t, r.Path, others, "another customer's order is never fetched")
		assert.NotContains(t, r.Path, voided, "a voided order is never fetched")
	}
}

func TestGetMyOrders_NoOrdersIsAnEmptyList(t *testing.T) {
	backend := newFakeBackend(t)
	cs := connectCustomer(t, backend, map[string]string{customerPhoneHeader: "573005551234"})

	text, isErr := call(t, cs, "get_my_orders", nil)

	require.False(t, isErr)
	assert.JSONEq(t, `{"orders":[]}`, text)
}
