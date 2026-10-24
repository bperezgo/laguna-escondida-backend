package bootstrap

import (
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/handler"

	"github.com/gin-gonic/gin"
)

// Route wiring lives outside main so tests can build the real router and check every
// route's guard (see agent_routes_test.go).

type apiHandlers struct {
	order             *handler.OrderHandler
	product           *handler.ProductHandler
	stock             *handler.StockHandler
	user              *handler.UserHandler
	billOwner         *handler.BillOwnerHandler
	productIngredient *handler.ProductIngredientHandler
	sse               *handler.SSEHandler
	edge              *handler.EdgeHandler
}

// registerAPIRoutes wires the routes served in both modes: the restaurant floor plus reads of
// the data the cloud owns. stockAuthoring adds the cloud's stock writes.
func registerAPIRoutes(router *gin.Engine, adminAPIKey string, jwtService *service.JWTService, h apiHandlers, stockAuthoring bool) {
	// Health check
	router.GET("/api/health", handler.HealthCheckHandler)

	// Edge/node status (mode, connectivity, sync lag, pending sync ops)
	router.GET("/api/edge/status", h.edge.GetStatusHandler)

	// Auth routes (no authentication required)
	router.POST("/api/auth/signin", h.user.SignInHandler)

	// Auth routes (protected with JWT)
	router.GET("/api/auth/me", handler.JWTAuthMiddleware(jwtService), h.user.GetCurrentUserHandler)

	// Admin user reads (protected with JWT + fine-grained user permissions); writes are cloud-only.
	adminUsers := router.Group("/api/admin", handler.JWTAuthMiddleware(jwtService))
	adminUsers.GET("/users", handler.RequirePermission(permissions.UsersRead), h.user.ListUsersHandler)
	adminUsers.GET("/users/:id", handler.RequirePermission(permissions.UsersRead), h.user.GetUserHandler)
	adminUsers.GET("/roles", handler.RequirePermission(permissions.UsersRead), h.user.ListRolesHandler)

	// Protected routes (require JWT authentication + permissions)
	handler.RegisterOrderRoutes(router, h.order, jwtService)

	// Catalog reads; writes are cloud-only (see registerCloudBackOfficeRoutes).
	router.GET("/api/products", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsRead), h.product.ListProductsHandler)
	router.GET("/api/products/categories", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsRead), h.product.ListCategoriesHandler)
	router.GET("/api/products/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsRead), h.product.GetProductByIDHandler)
	router.GET("/api/product-responsibilities/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsRead), h.product.GetProductResponsibilityByIDHandler)
	router.GET("/api/products/:id/ingredients", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsRead), h.productIngredient.GetIngredientsHandler)
	router.GET("/api/products/:id/side-dishes", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsRead), h.productIngredient.GetSideDishOptionsHandler)

	// Stock routes — reads in both modes, authoring cloud-only (see registerStockRoutes).
	registerStockRoutes(router, h.stock, jwtService, handler.AdminAPIKeyMiddleware(adminAPIKey), stockAuthoring)

	// Bill Owner routes
	router.GET("/api/bill-owners/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.BillOwnersRead), h.billOwner.GetByIDHandler)

	// SSE routes for real-time open bill product notifications
	router.GET("/api/sse/commands/:area", handler.SSEMiddleware(), handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SSECommandsRead), h.sse.StreamCommandsHandler)
	router.GET("/api/sse/open-bill-products/:area", handler.SSEMiddleware(), handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SSECommandItemsRead), h.sse.StreamOpenBillProductsHandler)
	router.GET("/api/open-bill-products/:area/pending", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.OrdersRead), h.sse.GetPendingOpenBillProductsHandler)
	router.GET("/api/open-bill-products/:area/completed", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.OrdersRead), h.sse.GetCompletedOpenBillProductsHandler)
}
