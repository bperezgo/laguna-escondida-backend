package bootstrap

import (
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/handler"

	"github.com/gin-gonic/gin"
)

// cloudHandlers back the cloud-only routes; telemetry is nil while the ingest is disabled.
type cloudHandlers struct {
	invoice         *handler.InvoiceHandler
	supportDocument *handler.SupportDocumentHandler
	sync            *handler.SyncHandler
	telemetry       *handler.TelemetryHandler
	organization    *handler.OrganizationHandler
	deliveryZone    *handler.DeliveryZoneHandler
	nodes           *service.NodeService
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

	RegisterDeliveryZoneRoutes(router, h.deliveryZone, jwtService)
}

// RegisterDeliveryZoneRoutes is cloud-only: deliveries are taken in the cloud, and the edge's
// empty zone table keeps a delivery from being created there by accident.
func RegisterDeliveryZoneRoutes(router *gin.Engine, zoneHandler *handler.DeliveryZoneHandler, jwtService *service.JWTService) {
	zones := router.Group("/api/delivery-zones", handler.JWTAuthMiddleware(jwtService))
	zones.GET("", handler.RequirePermission(permissions.DeliveryZonesRead), zoneHandler.ListHandler)
	zones.GET("/check", handler.RequirePermission(permissions.OrdersCreate), zoneHandler.CheckHandler)
	zones.POST("", handler.RequirePermission(permissions.DeliveryZonesManage), zoneHandler.CreateHandler)
	zones.PUT("/:id", handler.RequirePermission(permissions.DeliveryZonesManage), zoneHandler.UpdateHandler)
	zones.DELETE("/:id", handler.RequirePermission(permissions.DeliveryZonesManage), zoneHandler.DeleteHandler)
}

// backOfficeHandlers back the cloud's accounting and the writes to data the box only pulls.
type backOfficeHandlers struct {
	product           *handler.ProductHandler
	productIngredient *handler.ProductIngredientHandler
	user              *handler.UserHandler
	supplier          *handler.SupplierHandler
	purchaseEntry     *handler.PurchaseEntryHandler
	expense           *handler.ExpenseHandler
	financial         *handler.FinancialHandler
}

// registerCloudBackOfficeRoutes is cloud-only: a write on the box would be overwritten by its
// next pull, and expenses or purchases recorded there would never reach the cloud.
func registerCloudBackOfficeRoutes(router *gin.Engine, cfg *config.Config, jwtService *service.JWTService, h backOfficeHandlers) {
	// User routes (protected with admin API key) — kept for first-admin bootstrap via curl.
	router.POST("/api/users", handler.AdminAPIKeyMiddleware(cfg), h.user.CreateUserHandler)

	adminUsers := router.Group("/api/admin", handler.JWTAuthMiddleware(jwtService))
	adminUsers.POST("/users", handler.RequirePermission(permissions.UsersCreate), h.user.CreateUserHandler)
	adminUsers.PUT("/users/:id", handler.RequirePermission(permissions.UsersUpdate), h.user.UpdateUserHandler)
	adminUsers.POST("/users/:id/reset-password", handler.RequirePermission(permissions.UsersUpdate), h.user.ResetPasswordHandler)
	adminUsers.DELETE("/users/:id", handler.RequirePermission(permissions.UsersDelete), h.user.DeleteUserHandler)

	// Product routes
	router.POST("/api/products", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsCreate), h.product.CreateProductHandler)
	router.POST("/api/products/bulk", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsCreate), h.product.BulkCreateProductsHandler)
	router.PUT("/api/products/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsUpdate), h.product.UpdateProductHandler)
	router.DELETE("/api/products/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsDelete), h.product.DeleteProductHandler)

	// Product responsibility routes
	router.POST("/api/product-responsibilities", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsUpdate), h.product.CreateProductResponsibilityHandler)
	router.PUT("/api/product-responsibilities/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsUpdate), h.product.UpdateProductResponsibilityHandler)
	router.DELETE("/api/product-responsibilities/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsUpdate), h.product.DeleteProductResponsibilityHandler)

	// Product ingredient routes
	router.POST("/api/products/:id/ingredients", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsUpdate), h.productIngredient.AddIngredientHandler)
	router.PUT("/api/products/:id/ingredients/:ingredient_id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsUpdate), h.productIngredient.UpdateIngredientHandler)
	router.DELETE("/api/products/:id/ingredients/:ingredient_id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsUpdate), h.productIngredient.RemoveIngredientHandler)

	// Side-dish configuration routes
	router.PUT("/api/products/:id/ingredients/:ingredient_id/side-dish", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsUpdate), h.productIngredient.ConfigureSideDishHandler)
	router.DELETE("/api/products/:id/ingredients/:ingredient_id/side-dish", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsUpdate), h.productIngredient.ClearSideDishHandler)

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
}
