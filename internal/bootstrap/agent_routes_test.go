package bootstrap

import (
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/handler"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every route the WhatsApp agent's user can get past the guards of. Any other route must
// refuse it with 401 or 403, so a new route reachable by the agent fails this test until it
// is added here on purpose.
var agentReachableRoutes = map[string]bool{
	// Public.
	"GET /api/health":       true,
	"GET /api/edge/status":  true,
	"POST /api/auth/signin": true,
	// Any signed-in user.
	"GET /api/auth/me": true,
	// products:read
	"GET /api/products":                     true,
	"GET /api/products/categories":          true,
	"GET /api/products/:id":                 true,
	"GET /api/product-responsibilities/:id": true,
	"GET /api/products/:id/ingredients":     true,
	"GET /api/products/:id/side-dishes":     true,
	// orders:read — includes every order and both kitchen boards, not only the agent's own.
	"GET /api/orders":                             true,
	"GET /api/orders/closed":                      true,
	"GET /api/orders/closed/:id":                  true,
	"GET /api/orders/:id":                         true,
	"GET /api/open-bill-products/:area/pending":   true,
	"GET /api/open-bill-products/:area/completed": true,
	// orders:create — the service further limits it to takeaway and delivery.
	"POST /api/orders":              true,
	"GET /api/delivery-zones/check": true,
	// orders:update / orders:delete — the service further limits them to the agent's own orders.
	"PUT /api/orders/:id": true,
	"PATCH /api/orders/:id/products/:open_bill_product_id/cancel": true,
	"DELETE /api/orders/:id": true,
}

func fullRouter(mode config.Mode, jwtService *service.JWTService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	// Recovery turns the nil services behind a reachable route into a 500: past the guards.
	router := gin.New()
	router.Use(gin.Recovery())
	cfg := &config.Config{AppMode: mode, AdminAPIKey: "admin-key", NodeSyncKey: "node-key"}

	registerAPIRoutes(router, cfg, jwtService, apiHandlers{
		order: &handler.OrderHandler{}, product: &handler.ProductHandler{}, stock: &handler.StockHandler{},
		user: &handler.UserHandler{}, billOwner: &handler.BillOwnerHandler{}, supplier: &handler.SupplierHandler{},
		purchaseEntry: &handler.PurchaseEntryHandler{}, expense: &handler.ExpenseHandler{},
		productIngredient: &handler.ProductIngredientHandler{}, financial: &handler.FinancialHandler{},
		sse: &handler.SSEHandler{}, edge: &handler.EdgeHandler{},
	}, mode == config.ModeCloud)
	switch mode {
	case config.ModeCloud:
		registerCloudRoutes(router, cfg, jwtService, cloudHandlers{
			invoice: &handler.InvoiceHandler{}, supportDocument: &handler.SupportDocumentHandler{},
			sync: &handler.SyncHandler{}, telemetry: &handler.TelemetryHandler{},
			organization: &handler.OrganizationHandler{}, deliveryZone: &handler.DeliveryZoneHandler{},
		})
	case config.ModeEdge:
		registerEdgeRoutes(router, jwtService, &handler.DeviceHandler{})
		registerEdgeUpdateRoutes(router, jwtService, &handler.EdgeUpdateHandler{})
	}
	return router
}

var pathParams = strings.NewReplacer(
	":open_bill_product_id", "019a0000-0000-7000-8000-000000000002",
	":ingredient_id", "019a0000-0000-7000-8000-000000000003",
	":product_id", "019a0000-0000-7000-8000-000000000004",
	":id", "019a0000-0000-7000-8000-000000000001",
	":area", "kitchen",
)

// testOrgID is the organization every test token belongs to.
const testOrgID = "bb0cad3e-be13-4bad-9a37-bba9525bfece"

func TestAgentRole_ReachesOnlyItsRoutes(t *testing.T) {
	jwtService := service.NewJWTService("test-secret", "")
	token, err := jwtService.GenerateToken(
		"019a0000-0000-7000-8000-0000000000aa", testOrgID, "whatsapp-agent", []int{permissions.RoleAgent})
	require.NoError(t, err)

	for _, mode := range []config.Mode{config.ModeCloud, config.ModeEdge} {
		t.Run(string(mode), func(t *testing.T) {
			router := fullRouter(mode, jwtService)
			routes := router.Routes()
			sort.Slice(routes, func(i, j int) bool { return routes[i].Path+routes[i].Method < routes[j].Path+routes[j].Method })
			require.NotEmpty(t, routes)

			for _, route := range routes {
				key := route.Method + " " + route.Path
				req := httptest.NewRequest(route.Method, pathParams.Replace(route.Path), strings.NewReader("{}"))
				req.Header.Set("Authorization", "Bearer "+token)
				req.Header.Set("Content-Type", "application/json")
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)

				refused := rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden
				if agentReachableRoutes[key] {
					assert.False(t, refused, "%s should be reachable by the agent, got %d", key, rec.Code)
				} else {
					assert.True(t, refused, "%s must refuse the agent, got %d", key, rec.Code)
				}
			}
		})
	}
}

// Keeps the list honest: an entry for a route that no longer exists is a stale promise.
func TestAgentRole_ReachableRoutesExist(t *testing.T) {
	registered := map[string]bool{}
	for _, mode := range []config.Mode{config.ModeCloud, config.ModeEdge} {
		for _, route := range fullRouter(mode, service.NewJWTService("test-secret", "")).Routes() {
			registered[route.Method+" "+route.Path] = true
		}
	}
	for key := range agentReachableRoutes {
		assert.True(t, registered[key], "%s is listed as agent-reachable but is not registered", key)
	}
}
