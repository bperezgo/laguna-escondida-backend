package main

import (
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/config"
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
	supplier          *handler.SupplierHandler
	purchaseEntry     *handler.PurchaseEntryHandler
	expense           *handler.ExpenseHandler
	productIngredient *handler.ProductIngredientHandler
	financial         *handler.FinancialHandler
	deliveryZone      *handler.DeliveryZoneHandler
	sse               *handler.SSEHandler
	edge              *handler.EdgeHandler
}

// cloudHandlers back the cloud-only routes; telemetry is nil while the ingest is disabled.
type cloudHandlers struct {
	invoice         *handler.InvoiceHandler
	supportDocument *handler.SupportDocumentHandler
	sync            *handler.SyncHandler
	telemetry       *handler.TelemetryHandler
	organization    *handler.OrganizationHandler
	nodes           *service.NodeService
}

// registerAPIRoutes wires the routes served in both modes.
func registerAPIRoutes(router *gin.Engine, cfg *config.Config, jwtService *service.JWTService, h apiHandlers) {
	// Health check
	router.GET("/api/health", handler.HealthCheckHandler)

	// Edge/node status (mode, connectivity, sync lag, pending sync ops)
	router.GET("/api/edge/status", h.edge.GetStatusHandler)

	// Auth routes (no authentication required)
	router.POST("/api/auth/signin", h.user.SignInHandler)

	// Auth routes (protected with JWT)
	router.GET("/api/auth/me", handler.JWTAuthMiddleware(jwtService), h.user.GetCurrentUserHandler)

	// User routes (protected with admin API key) — kept for first-admin bootstrap via curl.
	router.POST("/api/users", handler.AdminAPIKeyMiddleware(cfg), h.user.CreateUserHandler)

	// Admin user management (protected with JWT + fine-grained user permissions).
	adminUsers := router.Group("/api/admin", handler.JWTAuthMiddleware(jwtService))
	adminUsers.POST("/users", handler.RequirePermission(permissions.UsersCreate), h.user.CreateUserHandler)
	adminUsers.GET("/users", handler.RequirePermission(permissions.UsersRead), h.user.ListUsersHandler)
	adminUsers.GET("/users/:id", handler.RequirePermission(permissions.UsersRead), h.user.GetUserHandler)
	adminUsers.PUT("/users/:id", handler.RequirePermission(permissions.UsersUpdate), h.user.UpdateUserHandler)
	adminUsers.POST("/users/:id/reset-password", handler.RequirePermission(permissions.UsersUpdate), h.user.ResetPasswordHandler)
	adminUsers.DELETE("/users/:id", handler.RequirePermission(permissions.UsersDelete), h.user.DeleteUserHandler)
	adminUsers.GET("/roles", handler.RequirePermission(permissions.UsersRead), h.user.ListRolesHandler)

	// Protected routes (require JWT authentication + permissions)
	handler.RegisterOrderRoutes(router, h.order, jwtService)

	// Product routes
	router.POST("/api/products", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsCreate), h.product.CreateProductHandler)
	router.POST("/api/products/bulk", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsCreate), h.product.BulkCreateProductsHandler)
	router.GET("/api/products", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsRead), h.product.ListProductsHandler)
	router.GET("/api/products/categories", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsRead), h.product.ListCategoriesHandler)
	router.GET("/api/products/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsRead), h.product.GetProductByIDHandler)
	router.PUT("/api/products/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsUpdate), h.product.UpdateProductHandler)
	router.DELETE("/api/products/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsDelete), h.product.DeleteProductHandler)

	// Product responsibility routes
	router.POST("/api/product-responsibilities", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsUpdate), h.product.CreateProductResponsibilityHandler)
	router.GET("/api/product-responsibilities/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsRead), h.product.GetProductResponsibilityByIDHandler)
	router.PUT("/api/product-responsibilities/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsUpdate), h.product.UpdateProductResponsibilityHandler)
	router.DELETE("/api/product-responsibilities/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsUpdate), h.product.DeleteProductResponsibilityHandler)

	// Product ingredient routes
	router.POST("/api/products/:id/ingredients", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsUpdate), h.productIngredient.AddIngredientHandler)
	router.GET("/api/products/:id/ingredients", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsRead), h.productIngredient.GetIngredientsHandler)
	router.PUT("/api/products/:id/ingredients/:ingredient_id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsUpdate), h.productIngredient.UpdateIngredientHandler)
	router.DELETE("/api/products/:id/ingredients/:ingredient_id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsUpdate), h.productIngredient.RemoveIngredientHandler)

	// Side-dish configuration routes
	router.GET("/api/products/:id/side-dishes", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsRead), h.productIngredient.GetSideDishOptionsHandler)
	router.PUT("/api/products/:id/ingredients/:ingredient_id/side-dish", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsUpdate), h.productIngredient.ConfigureSideDishHandler)
	router.DELETE("/api/products/:id/ingredients/:ingredient_id/side-dish", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsUpdate), h.productIngredient.ClearSideDishHandler)

	// Stock routes — reads in both modes, authoring cloud-only (see registerStockRoutes).
	registerStockRoutes(router, cfg, h.stock, jwtService)

	// Delivery zones — cloud-only (see RegisterDeliveryZoneRoutes).
	handler.RegisterDeliveryZoneRoutes(router, cfg, h.deliveryZone, jwtService)

	// Bill Owner routes
	router.GET("/api/bill-owners/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.BillOwnersRead), h.billOwner.GetByIDHandler)

	// Supplier routes
	router.POST("/api/suppliers", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SuppliersCreate), h.supplier.CreateSupplierHandler)
	router.GET("/api/suppliers", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SuppliersRead), h.supplier.ListSuppliersHandler)
	router.GET("/api/suppliers/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SuppliersRead), h.supplier.GetSupplierByIDHandler)
	router.PUT("/api/suppliers/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SuppliersUpdate), h.supplier.UpdateSupplierHandler)
	router.DELETE("/api/suppliers/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SuppliersDelete), h.supplier.DeleteSupplierHandler)

	// Supplier Catalog routes
	router.POST("/api/suppliers/:id/products", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SupplierCatalogCreate), h.supplier.AddProductToSupplierHandler)
	router.PUT("/api/suppliers/:id/products/:product_id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SupplierCatalogUpdate), h.supplier.UpdateSupplierCatalogHandler)
	router.DELETE("/api/suppliers/:id/products/:product_id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SupplierCatalogDelete), h.supplier.RemoveProductFromSupplierHandler)
	router.GET("/api/suppliers/:id/products", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SupplierCatalogRead), h.supplier.GetSupplierProductsHandler)
	router.GET("/api/products/:id/suppliers", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SupplierCatalogRead), h.supplier.GetProductSuppliersHandler)

	// Purchase Entry routes
	router.POST("/api/purchase-entries", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.PurchaseEntriesCreate), h.purchaseEntry.CreatePurchaseEntryHandler)
	router.GET("/api/purchase-entries", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.PurchaseEntriesRead), h.purchaseEntry.ListPurchaseEntriesHandler)
	router.GET("/api/purchase-entries/export", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.PurchaseEntriesExport), h.purchaseEntry.ExportPurchaseEntriesCSVHandler)
	router.GET("/api/purchase-entries/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.PurchaseEntriesRead), h.purchaseEntry.GetPurchaseEntryByIDHandler)
	router.GET("/api/suppliers/:id/purchase-entries", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.PurchaseEntriesRead), h.purchaseEntry.GetPurchaseEntriesBySupplierHandler)
	router.POST("/api/purchase-entries/:id/documents", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.PurchaseEntriesUpload), h.purchaseEntry.UploadPurchaseEntryDocumentHandler)

	// Expense Category routes
	router.POST("/api/expense-categories", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ExpenseCategoriesCreate), h.expense.CreateCategoryHandler)
	router.GET("/api/expense-categories", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ExpenseCategoriesRead), h.expense.ListCategoriesHandler)
	router.GET("/api/expense-categories/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ExpenseCategoriesRead), h.expense.GetCategoryByIDHandler)
	router.PUT("/api/expense-categories/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ExpenseCategoriesUpdate), h.expense.UpdateCategoryHandler)

	// Expense routes
	router.POST("/api/expenses", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ExpensesCreate), h.expense.CreateExpenseHandler)
	router.GET("/api/expenses", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ExpensesRead), h.expense.ListExpensesHandler)
	router.GET("/api/expenses/export", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ExpensesExport), h.expense.ExportExpensesCSVHandler)
	router.GET("/api/expenses/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ExpensesRead), h.expense.GetExpenseByIDHandler)
	router.PUT("/api/expenses/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ExpensesUpdate), h.expense.UpdateExpenseHandler)
	router.DELETE("/api/expenses/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ExpensesDelete), h.expense.DeleteExpenseHandler)
	router.POST("/api/expenses/:id/documents", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ExpensesUpload), h.expense.UploadExpenseDocumentHandler)

	// Financial routes
	router.GET("/api/financial/summary", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.FinancialRead), h.financial.GetFinancialSummaryHandler)

	// Reports routes
	router.GET("/api/reports/daily-close", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.FinancialRead), h.financial.GetDailyCloseHandler)

	// SSE routes for real-time open bill product notifications
	router.GET("/api/sse/commands/:area", handler.SSEMiddleware(), handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SSECommandsRead), h.sse.StreamCommandsHandler)
	router.GET("/api/sse/open-bill-products/:area", handler.SSEMiddleware(), handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SSECommandItemsRead), h.sse.StreamOpenBillProductsHandler)
	router.GET("/api/open-bill-products/:area/pending", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.OrdersRead), h.sse.GetPendingOpenBillProductsHandler)
	router.GET("/api/open-bill-products/:area/completed", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.OrdersRead), h.sse.GetCompletedOpenBillProductsHandler)
}

func registerCloudRoutes(router *gin.Engine, cfg *config.Config, jwtService *service.JWTService, h cloudHandlers) {
	// Admin routes (protected with admin API key)
	router.POST("/api/invoices/update-missing-document-urls", handler.AdminAPIKeyMiddleware(cfg), h.invoice.UpdateMissingDocumentURLsHandler)
	router.POST("/api/support-documents/update-missing-document-urls", handler.AdminAPIKeyMiddleware(cfg), h.supportDocument.UpdateMissingSupportDocumentURLsHandler)

	// Platform organization management: cross-org, so the admin API key until platform_admin exists.
	// No DELETE on purpose: an organization's invoices are legal records; retire it with status=suspended.
	organizations := router.Group("/api/platform/organizations", handler.AdminAPIKeyMiddleware(cfg))
	organizations.POST("", h.organization.CreateHandler)
	organizations.GET("", h.organization.ListHandler)
	organizations.GET("/:id", h.organization.GetHandler)
	organizations.PATCH("/:id", h.organization.UpdateHandler)
	organizations.PUT("/:id/einvoicing", h.organization.SetEInvoicingHandler)

	// Invoice routes
	router.POST("/api/invoices", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.InvoicesCreate), h.invoice.CreateElectronicInvoiceHandler)
	router.GET("/api/invoices", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.InvoicesRead), h.invoice.ListInvoicesHandler)
	router.GET("/api/invoices/export", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.InvoicesExport), h.invoice.ExportInvoicesCSVHandler)

	// Support Document routes
	router.POST("/api/support-documents", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SupportDocumentsCreate), h.supportDocument.CreateSupportDocumentHandler)
	router.GET("/api/support-documents", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SupportDocumentsRead), h.supportDocument.ListSupportDocumentsHandler)
	router.GET("/api/support-documents/export", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SupportDocumentsExport), h.supportDocument.ExportSupportDocumentsCSVHandler)

	// Cloud sync aggregate: accept pushes from edge nodes and serve reference changes.
	// Each acts for the organization the calling node is registered to.
	nodeAuth := handler.NodeAuthMiddleware(cfg, h.nodes)
	router.POST("/api/sync/push", handler.PushNodeIDFromBody(), nodeAuth, h.sync.PushHandler)
	router.GET("/api/sync/pull", nodeAuth, h.sync.PullHandler)
	router.GET("/api/sync/pull/stock", nodeAuth, h.sync.PullStockHandler)

	if h.telemetry != nil {
		router.POST("/api/telemetry/v1/logs", nodeAuth, h.telemetry.IngestLogsHandler)
		router.POST("/api/telemetry/v1/traces", nodeAuth, h.telemetry.IngestTracesHandler)
		router.POST("/api/telemetry/v1/metrics", nodeAuth, h.telemetry.IngestMetricsHandler)
	}
}

// registerEdgeRoutes wires ticket printing, only once the printer transport is up.
func registerEdgeRoutes(router *gin.Engine, jwtService *service.JWTService, device *handler.DeviceHandler) {
	router.POST("/api/device/print", handler.JWTAuthMiddleware(jwtService, permissions.RoleAdmin, permissions.RoleManager), device.PrintTicketHandler)
}

// registerEdgeUpdateRoutes wires owner-triggered appliance updates (edge only).
func registerEdgeUpdateRoutes(router *gin.Engine, jwtService *service.JWTService, update *handler.EdgeUpdateHandler) {
	edgeUpdate := router.Group("/api/edge/update", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.EdgeUpdate))
	edgeUpdate.GET("", update.GetUpdateHandler)
	edgeUpdate.POST("", update.RequestUpdateHandler)
}
