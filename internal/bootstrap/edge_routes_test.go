package bootstrap

import (
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
	"(*ProductIngredientHandler).ConfigureSideDishHandler",
	"(*ProductIngredientHandler).ClearSideDishHandler",
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
