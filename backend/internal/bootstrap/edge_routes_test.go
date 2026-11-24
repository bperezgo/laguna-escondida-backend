package bootstrap

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The box only pulls the catalog and users, and keeps no accounting: a handler here served
// on the edge would write data the next pull overwrites, or that never reaches the cloud.
var cloudOnlyHandlers = []string{
	"(*ProductHandler).CreateProductHandler",
	"(*ProductHandler).BulkCreateProductsHandler",
	"(*ProductHandler).UpdateProductHandler",
	"(*ProductHandler).DeleteProductHandler",
	"(*ProductHandler).CreateProductResponsibilityHandler",
	"(*ProductHandler).UpdateProductResponsibilityHandler",
	"(*ProductHandler).DeleteProductResponsibilityHandler",
	"(*ProductIngredientHandler).AddIngredientHandler",
	"(*ProductIngredientHandler).UpdateIngredientHandler",
	"(*ProductIngredientHandler).RemoveIngredientHandler",
	"(*PlateOptionSlotHandler).ReplaceHandler",
	"(*UserHandler).CreateUserHandler",
	"(*UserHandler).UpdateUserHandler",
	"(*UserHandler).ResetPasswordHandler",
	"(*UserHandler).DeleteUserHandler",
	"(*SupplierHandler).",
	"(*PurchaseEntryHandler).",
	"(*ExpenseHandler).",
	"(*FinancialHandler).",
}

func TestEdgeRoutes_ReachNoCloudOnlyHandler(t *testing.T) {
	edge, err := NewEdge(testEdgeConfig(), testInfra(t))
	require.NoError(t, err)
	cloud, err := NewCloud(testCloudConfig(), testInfra(t))
	require.NoError(t, err)

	for _, name := range cloudOnlyHandlers {
		for _, route := range edge.Router.Routes() {
			assert.NotContains(t, route.Handler, name, "%s %s is cloud-only", route.Method, route.Path)
		}
		assert.True(t, servesHandler(cloud, name), "the cloud still serves %s", name)
	}
}

func servesHandler(app *App, name string) bool {
	for _, route := range app.Router.Routes() {
		if strings.Contains(route.Handler, name) {
			return true
		}
	}
	return false
}

func TestEdgeRoutes_IgnoreTheAdminAPIKey(t *testing.T) {
	edge, err := NewEdge(testEdgeConfig(), testInfra(t))
	require.NoError(t, err)

	for _, route := range []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, "/api/products", http.StatusUnauthorized},
		{http.MethodPost, "/api/users", http.StatusNotFound},
		{http.MethodPost, "/api/stock/opening-balances", http.StatusNotFound},
	} {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			req := httptest.NewRequest(route.method, route.path, strings.NewReader("{}"))
			req.Header.Set("X-API-Key", "admin-key")
			rec := httptest.NewRecorder()
			edge.Router.ServeHTTP(rec, req)
			assert.Equal(t, route.want, rec.Code)
		})
	}
}
