package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/handler"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Snapshots the single binary's route table per mode before the split, so the split binaries
// can be checked against it. Telemetry ingest on; printer routes absent (they depend on hardware).
func TestWriteRouteGolden(t *testing.T) {
	if os.Getenv("WRITE_ROUTE_GOLDEN") == "" {
		t.Skip("set WRITE_ROUTE_GOLDEN=1 to regenerate internal/bootstrap/testdata/routes_*.golden")
	}
	jwtService := service.NewJWTService("test-secret", "")
	for _, mode := range []config.Mode{config.ModeCloud, config.ModeEdge} {
		gin.SetMode(gin.TestMode)
		router := gin.New()
		cfg := &config.Config{AppMode: mode, AdminAPIKey: "admin-key", NodeSyncKey: "node-key"}
		registerAPIRoutes(router, cfg, jwtService, apiHandlers{
			order: &handler.OrderHandler{}, product: &handler.ProductHandler{}, stock: &handler.StockHandler{},
			user: &handler.UserHandler{}, billOwner: &handler.BillOwnerHandler{}, supplier: &handler.SupplierHandler{},
			purchaseEntry: &handler.PurchaseEntryHandler{}, expense: &handler.ExpenseHandler{},
			productIngredient: &handler.ProductIngredientHandler{}, financial: &handler.FinancialHandler{},
			deliveryZone: &handler.DeliveryZoneHandler{}, sse: &handler.SSEHandler{}, edge: &handler.EdgeHandler{},
		})
		switch mode {
		case config.ModeCloud:
			registerCloudRoutes(router, cfg, jwtService, cloudHandlers{
				invoice: &handler.InvoiceHandler{}, supportDocument: &handler.SupportDocumentHandler{},
				sync: &handler.SyncHandler{}, telemetry: &handler.TelemetryHandler{},
				organization: &handler.OrganizationHandler{},
			})
		case config.ModeEdge:
			registerEdgeUpdateRoutes(router, jwtService, &handler.EdgeUpdateHandler{})
		}

		path := filepath.Join("..", "internal", "bootstrap", "testdata", "routes_"+string(mode)+".golden")
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		require.NoError(t, os.WriteFile(path, []byte(routeTable(router)), 0o600))
	}
}

func routeTable(router *gin.Engine) string {
	routes := router.Routes()
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path != routes[j].Path {
			return routes[i].Path < routes[j].Path
		}
		return routes[i].Method < routes[j].Method
	})
	var b strings.Builder
	for _, r := range routes {
		fmt.Fprintf(&b, "%s %s %s\n", r.Method, r.Path, r.Handler)
	}
	return b.String()
}
