package mcpserver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"laguna-escondida/backend/internal/domain/dto"
)

const (
	keyOfA     = "lgk_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	keyOfB     = "lgk_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	revokedKey = "lgk_rrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrr"
	staffJWT   = "eyJhbGciOiJIUzI1NiJ9.eyJ1c2VyX2lkIjoibWFuYWdlciJ9.c2ln"
)

// ---------------------------------------------------------------------------
// 4.2 The customer endpoint takes only organization API keys.
// ---------------------------------------------------------------------------

func TestRequireOrgAPIKey(t *testing.T) {
	cases := []struct {
		name      string
		header    string
		wantCode  int
		wantReach bool
	}{
		{name: "no header", wantCode: http.StatusUnauthorized},
		{name: "a staff JWT", header: "Bearer " + staffJWT, wantCode: http.StatusUnauthorized},
		{name: "the bare prefix", header: "Bearer lgk_", wantCode: http.StatusUnauthorized},
		{name: "not a bearer", header: "Basic " + keyOfA, wantCode: http.StatusUnauthorized},
		{name: "an organization key", header: "Bearer " + keyOfA, wantCode: http.StatusOK, wantReach: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var reached bool
			var info *auth.TokenInfo
			guarded := RequireOrgAPIKey(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reached = true
				info = auth.TokenInfoFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			}))
			req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()

			guarded.ServeHTTP(rec, req)

			assert.Equal(t, tc.wantCode, rec.Code)
			assert.Equal(t, tc.wantReach, reached)
			if tc.wantReach {
				require.NotNil(t, info, "the key's identity is on the request context")
				assert.NotEmpty(t, info.UserID)
				assert.NotContains(t, info.UserID, keyOfA[4:], "the session is bound to a digest, not the key")
			}
		})
	}
}

func TestRequireOrgAPIKey_BindsTheSessionToItsKey(t *testing.T) {
	verify := func(key string) string {
		info, err := verifyOrgAPIKeyShape(context.Background(), key, nil)
		require.NoError(t, err)
		return info.UserID
	}
	assert.Equal(t, verify(keyOfA), verify(keyOfA))
	assert.NotEqual(t, verify(keyOfA), verify(keyOfB))
}

// ---------------------------------------------------------------------------
// 4.3 The client sends the caller's key and nothing else.
// ---------------------------------------------------------------------------

type bearerRecorder struct {
	mu      sync.Mutex
	bearers []string
	signIns int
}

func (b *bearerRecorder) server(t *testing.T, status int) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b.mu.Lock()
		defer b.mu.Unlock()
		if r.URL.Path == "/api/auth/signin" {
			b.signIns++
			writeSignIn(w)
			return
		}
		b.bearers = append(b.bearers, r.Header.Get("Authorization"))
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"error":"Invalid or expired token"}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func customerClient(baseURL string) *Client {
	return NewClient(&Config{APIBaseURL: baseURL, Toolset: ToolsetCustomer})
}

func TestClient_EachContextReachesTheBackendWithItsOwnKey(t *testing.T) {
	rec := &bearerRecorder{}
	c := customerClient(rec.server(t, http.StatusOK).URL)

	var wg sync.WaitGroup
	for range 20 {
		for _, key := range []string{keyOfA, keyOfB} {
			wg.Go(func() {
				_, err := c.Get(withCallerKey(context.Background(), key), "/api/products", nil)
				assert.NoError(t, err)
			})
		}
	}
	wg.Wait()

	rec.mu.Lock()
	defer rec.mu.Unlock()
	counts := map[string]int{}
	for _, b := range rec.bearers {
		counts[b]++
	}
	assert.Equal(t, map[string]int{"Bearer " + keyOfA: 20, "Bearer " + keyOfB: 20}, counts)
	assert.Zero(t, rec.signIns, "the customer server never signs in")
}

func TestClient_RefusedKeyIsNotRetried(t *testing.T) {
	for name, c := range map[string]func(url string) *Client{
		"customer toolset":                  customerClient,
		"a key on a service-account client": testClient,
	} {
		t.Run(name, func(t *testing.T) {
			rec := &bearerRecorder{}
			client := c(rec.server(t, http.StatusUnauthorized).URL)

			_, err := client.Get(withCallerKey(context.Background(), revokedKey), "/api/orders", nil)

			assert.ErrorIs(t, err, ErrCredentialRefused)
			assert.Len(t, rec.bearers, 1, "no second request")
			assert.Zero(t, rec.signIns, "no other credential is tried")
		})
	}
}

func TestClient_RefusedKeyOnUploadIsNotRetried(t *testing.T) {
	rec := &bearerRecorder{}
	c := customerClient(rec.server(t, http.StatusUnauthorized).URL)
	file := t.TempDir() + "/invoice.pdf"
	require.NoError(t, os.WriteFile(file, []byte("%PDF-1.4"), 0o600))

	_, err := c.UploadFile(withCallerKey(context.Background(), revokedKey), "/api/expenses/x/documents", nil, "file", file)

	assert.ErrorIs(t, err, ErrCredentialRefused)
	assert.Len(t, rec.bearers, 1)
}

func TestClient_CustomerToolsetWithoutAKeySendsNothing(t *testing.T) {
	rec := &bearerRecorder{}
	c := customerClient(rec.server(t, http.StatusOK).URL)

	_, err := c.Get(context.Background(), "/api/products", nil)

	assert.ErrorIs(t, err, errNoCallerKey)
	assert.Empty(t, rec.bearers)
	assert.Zero(t, rec.signIns)
}

// ---------------------------------------------------------------------------
// 4.4 One customer server, two restaurants, end to end over HTTP.
// ---------------------------------------------------------------------------

type tenantRequest struct {
	Bearer string
	Method string
	Path   string
	Body   []byte
}

// twoTenantBackend answers as the backend would for two organizations, told apart by the bearer.
type twoTenantBackend struct {
	mu       sync.Mutex
	requests []tenantRequest
	menus    map[string][]*dto.Product
	server   *httptest.Server
}

func newTwoTenantBackend(t *testing.T) *twoTenantBackend {
	b := &twoTenantBackend{menus: map[string][]*dto.Product{
		"Bearer " + keyOfA: {{ID: "prod-a", Name: "Mojarra frita", ProductType: dto.ProductTypeSellable, TotalPriceWithTaxes: decimal.NewFromInt(32000)}},
		"Bearer " + keyOfB: {{ID: "prod-b", Name: "Bandeja paisa", ProductType: dto.ProductTypeSellable, TotalPriceWithTaxes: decimal.NewFromInt(28000)}},
	}}
	b.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bearer := r.Header.Get("Authorization")
		b.mu.Lock()
		b.requests = append(b.requests, tenantRequest{Bearer: bearer, Method: r.Method, Path: r.URL.Path, Body: body})
		menu, known := b.menus[bearer]
		b.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch {
		case !known:
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"Invalid or expired token"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/products":
			_ = json.NewEncoder(w).Encode(dto.ProductListResponse{Products: menu})
		case r.Method == http.MethodPost && r.URL.Path == "/api/orders":
			var req dto.CreateOrderRequest
			_ = json.Unmarshal(body, &req)
			order := dto.OpenBill{ID: req.OpenBillID, TotalAmount: decimal.NewFromInt(32000)}
			order.OrderType = req.OrderType
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(order)
		default:
			_ = json.NewEncoder(w).Encode(dto.OpenBillListResponse{})
		}
	}))
	t.Cleanup(b.server.Close)
	return b
}

func (b *twoTenantBackend) recorded() []tenantRequest {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]tenantRequest(nil), b.requests...)
}

// customerDeployment is one customer MCP server as production runs it: guard, transport, tools.
func customerDeployment(t *testing.T, backendURL string) string {
	t.Helper()
	cfg := &Config{APIBaseURL: backendURL, Toolset: ToolsetCustomer}
	server := NewMCPServer(NewClient(cfg), cfg.Toolset)
	mcpHTTP := httptest.NewServer(RequireOrgAPIKey(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)))
	t.Cleanup(mcpHTTP.Close)
	return mcpHTTP.URL
}

func connectWith(t *testing.T, endpoint string, headers map[string]string) (*mcp.ClientSession, error) {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "agent-runtime", Version: "test"}, nil)
	cs, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:   endpoint,
		HTTPClient: &http.Client{Transport: headerTransport{headers: headers, next: http.DefaultTransport}},
	}, nil)
	if err == nil {
		t.Cleanup(func() { _ = cs.Close() })
	}
	return cs, err
}

func agentHeaders(key string) map[string]string {
	return map[string]string{
		"Authorization":        "Bearer " + key,
		customerPhoneHeader:    callerPhone,
		customerLocationHeader: insidePin,
	}
}

func TestCustomerDeployment_TwoRestaurantsOnOneServer(t *testing.T) {
	backend := newTwoTenantBackend(t)
	endpoint := customerDeployment(t, backend.server.URL)
	sessionA, err := connectWith(t, endpoint, agentHeaders(keyOfA))
	require.NoError(t, err)
	sessionB, err := connectWith(t, endpoint, agentHeaders(keyOfB))
	require.NoError(t, err)

	var wg sync.WaitGroup
	menus := make([]string, 2)
	for i, cs := range []*mcp.ClientSession{sessionA, sessionB} {
		wg.Go(func() {
			text, isErr := call(t, cs, "list_menu", nil)
			assert.False(t, isErr, text)
			menus[i] = text
		})
	}
	wg.Wait()

	assert.Contains(t, menus[0], "Mojarra frita")
	assert.NotContains(t, menus[0], "Bandeja paisa")
	assert.Contains(t, menus[1], "Bandeja paisa")
	assert.NotContains(t, menus[1], "Mojarra frita")

	for _, r := range backend.recorded() {
		assert.Contains(t, []string{"Bearer " + keyOfA, "Bearer " + keyOfB}, r.Bearer, "%s %s carried a caller's key", r.Method, r.Path)
	}
}

func TestCustomerDeployment_OrdersCarryTheCallersKeyAndChannelHeaders(t *testing.T) {
	backend := newTwoTenantBackend(t)
	endpoint := customerDeployment(t, backend.server.URL)
	sessionB, err := connectWith(t, endpoint, agentHeaders(keyOfB))
	require.NoError(t, err)

	text, isErr := call(t, sessionB, "create_delivery_order", map[string]any{
		"order_type":       "delivery",
		"delivery_address": "Calle 10 # 5-20, apto 301",
		"lines":            []map[string]any{{"product_id": "prod-b", "quantity": 2}},
	})
	require.False(t, isErr, text)

	var posted []tenantRequest
	for _, r := range backend.recorded() {
		if r.Method == http.MethodPost && r.Path == "/api/orders" {
			posted = append(posted, r)
		}
	}
	require.Len(t, posted, 1)
	assert.Equal(t, "Bearer "+keyOfB, posted[0].Bearer, "the order is created with B's key")
	var req dto.CreateOrderRequest
	require.NoError(t, json.Unmarshal(posted[0].Body, &req))
	require.NotNil(t, req.CustomerPhone)
	assert.Equal(t, callerPhone, *req.CustomerPhone, "the phone still comes from the channel header")
	require.NotNil(t, req.DeliveryLat)
	require.NotNil(t, req.DeliveryLng)
	assert.Equal(t, insidePin, *req.DeliveryLat+","+*req.DeliveryLng, "and the pin from the location header")
}

func TestCustomerDeployment_RefusesCallersWithoutAKey(t *testing.T) {
	for name, headers := range map[string]map[string]string{
		"no key":        {customerPhoneHeader: callerPhone},
		"a staff token": {"Authorization": "Bearer " + staffJWT},
	} {
		t.Run(name, func(t *testing.T) {
			backend := newTwoTenantBackend(t)
			endpoint := customerDeployment(t, backend.server.URL)

			_, err := connectWith(t, endpoint, headers)
			require.Error(t, err)

			req, reqErr := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(`{}`))
			require.NoError(t, reqErr)
			for k, v := range headers {
				req.Header.Set(k, v)
			}
			resp, reqErr := http.DefaultClient.Do(req)
			require.NoError(t, reqErr)
			_ = resp.Body.Close()
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
			assert.Empty(t, backend.recorded(), "no backend request is made")
		})
	}
}

func TestCustomerDeployment_RevokedKeyIsACredentialRefusedToolError(t *testing.T) {
	backend := newTwoTenantBackend(t)
	endpoint := customerDeployment(t, backend.server.URL)
	session, err := connectWith(t, endpoint, agentHeaders(revokedKey))
	require.NoError(t, err)

	text, isErr := call(t, session, "get_my_orders", nil)

	assert.True(t, isErr)
	assert.Contains(t, text, "credential refused")
	requests := backend.recorded()
	require.Len(t, requests, 1, "no retry under another credential")
	assert.Equal(t, "Bearer "+revokedKey, requests[0].Bearer)
}

// switchableKey sends whichever key is current, so a test can change it mid-session.
type switchableKey struct {
	key  atomic.Value
	next http.RoundTripper
}

func (s *switchableKey) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	key, _ := s.key.Load().(string)
	r.Header.Set("Authorization", "Bearer "+key)
	r.Header.Set(customerPhoneHeader, callerPhone)
	return s.next.RoundTrip(r)
}

func TestCustomerDeployment_ASessionCannotSwitchKeys(t *testing.T) {
	backend := newTwoTenantBackend(t)
	endpoint := customerDeployment(t, backend.server.URL)
	transport := &switchableKey{next: http.DefaultTransport}
	transport.key.Store(keyOfA)
	session, err := mcp.NewClient(&mcp.Implementation{Name: "agent-runtime", Version: "test"}, nil).
		Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: endpoint, HTTPClient: &http.Client{Transport: transport}}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })

	transport.key.Store(keyOfB)
	_, err = session.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_menu"})

	assert.Error(t, err, "A's session refuses B's key")
	for _, r := range backend.recorded() {
		assert.NotEqual(t, "Bearer "+keyOfB, r.Bearer)
	}
}
