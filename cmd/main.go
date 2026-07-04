package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/cron"
	"laguna-escondida/backend/internal/platform/device"
	"laguna-escondida/backend/internal/platform/handler"
	"laguna-escondida/backend/internal/platform/httpclient"
	"laguna-escondida/backend/internal/platform/postgres"
	"laguna-escondida/backend/internal/platform/postgres/migrations"
	"laguna-escondida/backend/internal/platform/postgres/repository"
	"laguna-escondida/backend/internal/platform/sse"
	"laguna-escondida/backend/internal/platform/storage"
	"laguna-escondida/backend/internal/platform/syncstatus"
	"laguna-escondida/backend/pkg/eventbus"

	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Initialize Zap logger (production config uses JSON format)
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer func() {
		if errLogger := logger.Sync(); errLogger != nil {
			log.Printf("Failed to sync logger: %v", errLogger)
		}
	}()

	// Database connection
	dsn := getDSN()

	// Run migrations before connecting to database
	log.Println("Running database migrations...")
	if err = runMigrations(dsn); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	db, err := repository.NewDatabase(dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	httpClient := httpclient.NewClient(logger)

	// Initialize storage client
	spacesClient, err := storage.NewSpacesClient(cfg)
	if err != nil {
		log.Fatalf("Failed to create Spaces client: %v", err)
	}

	// Initialize repositories
	productRepo := repository.NewProductRepository(db.DB)
	openBillRepo := repository.NewOpenBillRepository(db.DB)
	stockRepo := repository.NewStockRepository(db.DB)
	userRepo := repository.NewUserRepository(db.DB)
	roleRepo := repository.NewRoleRepository(db.DB)
	userRoleRepo := repository.NewUserRoleRepository(db.DB)
	billOwnerRepo := repository.NewBillOwnerRepository(db.DB)
	supplierRepo := repository.NewSupplierRepository(db.DB)
	supplierCatalogRepo := repository.NewSupplierCatalogRepository(db.DB)
	purchaseEntryRepo := repository.NewPurchaseEntryRepository(db.DB)
	expenseCategoryRepo := repository.NewExpenseCategoryRepository(db.DB)
	expenseRepo := repository.NewExpenseRepository(db.DB)
	productIngredientRepo := repository.NewProductIngredientRepository(db.DB)
	electronicInvoiceClient := httpclient.NewElectronicInvoiceClient(cfg, httpClient)
	billRepo := repository.NewBillRepository(db.DB, cfg)
	pendingInvoiceRepo := repository.NewPendingInvoiceRepository(db.DB)
	invoiceService := service.NewInvoiceService(electronicInvoiceClient, productRepo, billRepo, spacesClient, cfg.OrganizationID)

	// Initialize SSE Hubs
	sseHub := sse.NewHub()
	openBillProductHub := sse.NewOpenBillProductHub(logger)

	// Initialize Watermill Event Bus for pub/sub messaging
	watermillLogger := eventbus.NewZapLoggerAdapter(logger)
	eventBusImpl := eventbus.NewGoChannelEventBus(watermillLogger)
	defer func() {
		if errEventBus := eventBusImpl.Close(); errEventBus != nil {
			log.Printf("Failed to close event bus: %v", errEventBus)
		}
	}()

	// Initialize JWT service
	jwtService := service.NewJWTService(cfg.JWTSecret)

	// Initialize slog logger for services
	slogLogger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Initialize services
	unitOfWork := postgres.NewUnitOfWork(db.DB)
	syncOutboxRepo := repository.NewSyncOutboxRepository(db.DB)
	syncInboxRepo := repository.NewSyncInboxRepository(db.DB)
	syncStateRepo := repository.NewSyncStateRepository(db.DB)
	// Reference repo is both sides of pull: the cloud reads changed rows, the edge upserts.
	syncReferenceRepo := repository.NewSyncReferenceRepository(db.DB)
	// Entity appliers turn a received op's payload snapshot into a local upsert (or a
	// soft-delete for a tombstone). The sync service dispatches on op.EntityType and
	// fails closed for any entity type without a registered applier.
	syncAppliers := map[dto.SyncEntityType]ports.SyncApplier{
		dto.SyncEntityOpenBill:      repository.NewOpenBillSyncApplier(db.DB),
		dto.SyncEntityPurchaseEntry: repository.NewPurchaseEntrySyncApplier(db.DB),
		dto.SyncEntityBill:          repository.NewBillSyncApplier(db.DB),
	}
	syncService := service.NewSyncService(unitOfWork, syncInboxRepo, syncAppliers, slogLogger)
	syncReferenceService := service.NewSyncReferenceService(syncReferenceRepo, slogLogger)
	// This install's sync identity, built once and injected as a unit into every
	// sync-participating component (order/purchase outbox writers + push/pull loops).
	syncIdentity := dto.SyncIdentity{NodeID: cfg.NodeID, CloudNodeID: cfg.CloudNodeID}
	// Edge status: pending-ops/lag come from this node's unsynced outbox (a domain use
	// case); connectivity comes from the sync scheduler via the in-memory tracker below.
	edgeStatusService := service.NewEdgeStatusService(syncOutboxRepo, syncIdentity)
	orderService := service.NewOrderServiceWithSSE(
		logger,
		openBillRepo,
		productRepo,
		billRepo,
		billOwnerRepo,
		invoiceService,
		unitOfWork,
		eventBusImpl,
		userRepo,
		openBillProductHub,
		syncOutboxRepo,
		syncIdentity,
	)
	productService := service.NewProductService(productRepo, supplierRepo, supplierCatalogRepo)
	stockService := service.NewStockService(stockRepo, productRepo)
	userService := service.NewUserService(userRepo, roleRepo, userRoleRepo, jwtService, unitOfWork)
	billOwnerService := service.NewBillOwnerService(billOwnerRepo)
	supplierService := service.NewSupplierService(supplierRepo, supplierCatalogRepo, productRepo)
	purchaseEntryService := service.NewPurchaseEntryService(purchaseEntryRepo, supplierRepo, supplierCatalogRepo, productRepo, spacesClient, eventBusImpl, unitOfWork, syncOutboxRepo, syncIdentity, slogLogger, cfg.OrganizationID)
	expenseService := service.NewExpenseService(expenseCategoryRepo, expenseRepo, supplierRepo, spacesClient, cfg.OrganizationID)
	productIngredientService := service.NewProductIngredientService(productIngredientRepo, productRepo)
	financialService := service.NewFinancialService(billRepo, expenseRepo, purchaseEntryRepo)
	supportDocRepo := repository.NewSupportDocumentRepository(db.DB, electronicInvoiceClient, cfg)
	supportDocService := service.NewSupportDocumentService(electronicInvoiceClient, supportDocRepo, spacesClient, cfg.OrganizationID)
	// Drains the pending_invoices queue: issues queued electronic invoices to the fiscal
	// provider out-of-band (so paying an order never blocks on it), stores the CUFE on the
	// bill, and replicates that result to the cloud. Runs in both modes — the queue only ever
	// has rows on the node that took the payment.
	invoiceSubmissionService := service.NewInvoiceSubmissionService(
		pendingInvoiceRepo,
		billRepo,
		electronicInvoiceClient,
		unitOfWork,
		syncOutboxRepo,
		syncIdentity,
		logger,
	)

	// Initialize Event Subscriber
	eventSubscriber, err := eventbus.NewGoChannelEventSubscriber(eventBusImpl.PubSub(), watermillLogger)
	if err != nil {
		log.Fatalf("Failed to create event subscriber: %v", err)
	}
	defer func() {
		if errEventSubscriber := eventSubscriber.Close(); errEventSubscriber != nil {
			log.Printf("Failed to close event subscriber: %v", errEventSubscriber)
		}
	}()

	// Register event handlers for SSE notifications
	orderCreatedHandler := eventbus.NewTypedEventHandler(
		dto.OrderCreatedEventName,
		orderService.HandleOrderCreatedSSE,
	)
	if err = eventSubscriber.Subscribe(orderCreatedHandler); err != nil {
		log.Fatalf("Failed to subscribe to order created events: %v", err)
	}

	orderDeletedHandler := eventbus.NewTypedEventHandler(
		dto.OrderDeletedEventName,
		orderService.HandleOrderDeletedSSE,
	)
	if err = eventSubscriber.Subscribe(orderDeletedHandler); err != nil {
		log.Fatalf("Failed to subscribe to order deleted events: %v", err)
	}

	orderUpdatedHandler := eventbus.NewTypedEventHandler(
		dto.OrderUpdatedEventName,
		orderService.HandleOrderUpdatedSSE,
	)
	if err = eventSubscriber.Subscribe(orderUpdatedHandler); err != nil {
		log.Fatalf("Failed to subscribe to order updated events: %v", err)
	}

	// Start event subscriber in background
	logger.Info("Starting SSE event subscriber for order events")
	go func() {
		logger.Info("SSE event subscriber goroutine started")
		if errEventSubscriber := eventSubscriber.Start(context.Background()); errEventSubscriber != nil {
			logger.Error("Event subscriber stopped", zap.Error(errEventSubscriber))
		} else {
			logger.Info("Event subscriber completed successfully")
		}
	}()

	// Initialize Stock Event Handler with Product Lock Manager
	productLockManager := eventbus.NewProductLockManager()
	stockEventHandler := service.NewStockEventHandler(
		stockRepo,
		productRepo,
		productIngredientRepo,
		productLockManager,
		slogLogger,
	)

	// Create separate subscriber for stock handlers
	stockSubscriber, err := eventbus.NewGoChannelEventSubscriber(eventBusImpl.PubSub(), watermillLogger)
	if err != nil {
		log.Fatalf("Failed to create stock subscriber: %v", err)
	}
	defer func() {
		if errStockSubscriber := stockSubscriber.Close(); errStockSubscriber != nil {
			log.Printf("Failed to close stock subscriber: %v", errStockSubscriber)
		}
	}()

	// Register stock handlers for order events
	stockOrderCreatedHandler := eventbus.NewTypedEventHandler(
		dto.OrderCreatedEventName,
		stockEventHandler.HandleOrderCreated,
	)
	if err = stockSubscriber.Subscribe(stockOrderCreatedHandler); err != nil {
		log.Fatalf("Failed to subscribe stock handler to order created events: %v", err)
	}

	stockOrderUpdatedHandler := eventbus.NewTypedEventHandler(
		dto.OrderUpdatedEventName,
		stockEventHandler.HandleOrderUpdated,
	)
	if err = stockSubscriber.Subscribe(stockOrderUpdatedHandler); err != nil {
		log.Fatalf("Failed to subscribe stock handler to order updated events: %v", err)
	}

	stockOrderDeletedHandler := eventbus.NewTypedEventHandler(
		dto.OrderDeletedEventName,
		stockEventHandler.HandleOrderDeleted,
	)
	if err = stockSubscriber.Subscribe(stockOrderDeletedHandler); err != nil {
		log.Fatalf("Failed to subscribe stock handler to order deleted events: %v", err)
	}

	// Register handler for purchase entry events
	purchaseEntryCreatedHandler := eventbus.NewTypedEventHandler(
		dto.PurchaseEntryCreatedEventName,
		stockEventHandler.HandlePurchaseEntryCreated,
	)
	if err = stockSubscriber.Subscribe(purchaseEntryCreatedHandler); err != nil {
		log.Fatalf("Failed to subscribe to purchase entry created events: %v", err)
	}

	// Start stock subscriber in background
	logger.Info("Starting stock event subscriber")
	go func() {
		logger.Info("Stock event subscriber goroutine started")
		if errStockSubscriber := stockSubscriber.Start(context.Background()); errStockSubscriber != nil {
			logger.Error("Stock subscriber stopped", zap.Error(errStockSubscriber))
		} else {
			logger.Info("Stock subscriber completed successfully")
		}
	}()

	// Initialize and start cron scheduler
	cronScheduler, err := cron.NewScheduler(invoiceService, supportDocService, invoiceSubmissionService, cfg.InvoiceURLCron, cfg.SupportDocumentURLCron, cfg.InvoiceSubmitCron, logger)
	if err != nil {
		log.Fatalf("Failed to create cron scheduler: %v", err)
	}
	if err := cronScheduler.Start(); err != nil {
		log.Fatalf("Failed to start cron scheduler: %v", err)
	}
	defer func() {
		if err := cronScheduler.Stop(); err != nil {
			log.Printf("Failed to stop cron scheduler: %v", err)
		}
	}()

	// Initialize handlers
	orderHandler := handler.NewOrderHandler(orderService)
	productHandler := handler.NewProductHandler(productService)
	stockHandler := handler.NewStockHandler(stockService)
	invoiceHandler := handler.NewInvoiceHandler(invoiceService)
	userHandler := handler.NewUserHandler(userService)
	billOwnerHandler := handler.NewBillOwnerHandler(billOwnerService)
	supplierHandler := handler.NewSupplierHandler(supplierService)
	purchaseEntryHandler := handler.NewPurchaseEntryHandler(purchaseEntryService)
	expenseHandler := handler.NewExpenseHandler(expenseService)
	productIngredientHandler := handler.NewProductIngredientHandler(productIngredientService)
	financialHandler := handler.NewFinancialHandler(financialService)
	supportDocHandler := handler.NewSupportDocumentHandler(supportDocService)
	sseHandler := handler.NewSSEHandler(sseHub, openBillProductHub, orderService, logger)
	// Online flips to offline if no successful pull lands within ~2.5 pull-cron intervals.
	// In edge mode the scheduler (below) records pull outcomes into this tracker; in cloud
	// mode it is never written and the status handler short-circuits to online.
	syncStatusTracker := syncstatus.NewTracker(150 * time.Second)
	edgeHandler := handler.NewEdgeHandler(cfg, edgeStatusService, syncStatusTracker)

	// Setup routes
	router := gin.Default()

	// Apply CORS middleware globally
	router.Use(handler.CORSMiddleware())

	// Apply Logger middleware globally
	router.Use(handler.LoggerMiddleware(logger))

	// Health check
	router.GET("/api/health", handler.HealthCheckHandler)

	// Edge/node status (mode, connectivity, sync lag, pending sync ops)
	router.GET("/api/edge/status", edgeHandler.GetStatusHandler)

	// Auth routes (no authentication required)
	router.POST("/api/auth/signin", userHandler.SignInHandler)

	// Auth routes (protected with JWT)
	router.GET("/api/auth/me", handler.JWTAuthMiddleware(jwtService), userHandler.GetCurrentUserHandler)

	// User routes (protected with admin API key) — kept for first-admin bootstrap via curl.
	router.POST("/api/users", handler.AdminAPIKeyMiddleware(cfg), userHandler.CreateUserHandler)

	// Admin user management (protected with JWT + fine-grained user permissions).
	adminUsers := router.Group("/api/admin", handler.JWTAuthMiddleware(jwtService))
	adminUsers.POST("/users", handler.RequirePermission(permissions.UsersCreate), userHandler.CreateUserHandler)
	adminUsers.GET("/users", handler.RequirePermission(permissions.UsersRead), userHandler.ListUsersHandler)
	adminUsers.GET("/users/:id", handler.RequirePermission(permissions.UsersRead), userHandler.GetUserHandler)
	adminUsers.PUT("/users/:id", handler.RequirePermission(permissions.UsersUpdate), userHandler.UpdateUserHandler)
	adminUsers.POST("/users/:id/reset-password", handler.RequirePermission(permissions.UsersUpdate), userHandler.ResetPasswordHandler)
	adminUsers.DELETE("/users/:id", handler.RequirePermission(permissions.UsersDelete), userHandler.DeleteUserHandler)
	adminUsers.GET("/roles", handler.RequirePermission(permissions.UsersRead), userHandler.ListRolesHandler)

	// Admin routes (protected with admin API key)
	router.POST("/api/invoices/update-missing-document-urls", handler.AdminAPIKeyMiddleware(cfg), invoiceHandler.UpdateMissingDocumentURLsHandler)
	router.POST("/api/support-documents/update-missing-document-urls", handler.AdminAPIKeyMiddleware(cfg), supportDocHandler.UpdateMissingSupportDocumentURLsHandler)

	// Protected routes (require JWT authentication + permissions)
	// Order routes
	router.POST("/api/orders", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.OrdersCreate), orderHandler.CreateOrderHandler)
	router.GET("/api/orders", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.OrdersRead), orderHandler.GetAllActiveOpenBillsHandler)
	router.GET("/api/orders/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.OrdersRead), orderHandler.GetOpenBillWithProductsHandler)
	router.PUT("/api/orders/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.OrdersUpdate), orderHandler.UpdateOrderHandler)
	router.DELETE("/api/orders/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.OrdersDelete), orderHandler.DeleteOrderHandler)
	router.POST("/api/orders/pay-order", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.OrdersPay), orderHandler.PayOrderHandler)

	// Order product status routes
	router.PATCH("/api/orders/:id/products/:open_bill_product_id/complete", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.OrdersCompleteProduct), orderHandler.CompleteOpenBillProductHandler)
	router.PATCH("/api/orders/:id/products/:open_bill_product_id/uncomplete", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.OrdersCompleteProduct), orderHandler.UncompleteOpenBillProductHandler)
	router.PATCH("/api/orders/:id/products/:open_bill_product_id/in-progress", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.OrdersCompleteProduct), orderHandler.SetOpenBillProductInProgressHandler)
	router.PATCH("/api/orders/:id/products/:open_bill_product_id/cancel", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.OrdersUpdate), orderHandler.CancelOpenBillProductHandler)

	// Product routes
	router.POST("/api/products", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsCreate), productHandler.CreateProductHandler)
	router.POST("/api/products/bulk", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsCreate), productHandler.BulkCreateProductsHandler)
	router.GET("/api/products", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsRead), productHandler.ListProductsHandler)
	router.GET("/api/products/categories", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsRead), productHandler.ListCategoriesHandler)
	router.GET("/api/products/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsRead), productHandler.GetProductByIDHandler)
	router.PUT("/api/products/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsUpdate), productHandler.UpdateProductHandler)
	router.DELETE("/api/products/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsDelete), productHandler.DeleteProductHandler)

	// Product responsibility routes
	router.POST("/api/product-responsibilities", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsUpdate), productHandler.CreateProductResponsibilityHandler)
	router.GET("/api/product-responsibilities/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsRead), productHandler.GetProductResponsibilityByIDHandler)
	router.PUT("/api/product-responsibilities/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsUpdate), productHandler.UpdateProductResponsibilityHandler)
	router.DELETE("/api/product-responsibilities/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsUpdate), productHandler.DeleteProductResponsibilityHandler)

	// Product ingredient routes
	router.POST("/api/products/:id/ingredients", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsUpdate), productIngredientHandler.AddIngredientHandler)
	router.GET("/api/products/:id/ingredients", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsRead), productIngredientHandler.GetIngredientsHandler)
	router.PUT("/api/products/:id/ingredients/:ingredient_id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsUpdate), productIngredientHandler.UpdateIngredientHandler)
	router.DELETE("/api/products/:id/ingredients/:ingredient_id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsUpdate), productIngredientHandler.RemoveIngredientHandler)

	// Invoice routes
	router.POST("/api/invoices", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.InvoicesCreate), invoiceHandler.CreateElectronicInvoiceHandler)
	router.GET("/api/invoices", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.InvoicesRead), invoiceHandler.ListInvoicesHandler)
	router.GET("/api/invoices/export", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.InvoicesExport), invoiceHandler.ExportInvoicesCSVHandler)

	// Support Document routes
	router.POST("/api/support-documents", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SupportDocumentsCreate), supportDocHandler.CreateSupportDocumentHandler)
	router.GET("/api/support-documents", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SupportDocumentsRead), supportDocHandler.ListSupportDocumentsHandler)
	router.GET("/api/support-documents/export", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SupportDocumentsExport), supportDocHandler.ExportSupportDocumentsCSVHandler)

	// Stock routes
	router.POST("/api/stock", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.StockCreate), stockHandler.CreateStockHandler)
	router.PUT("/api/stock/:product_id/add-or-decrease", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.StockUpdate), stockHandler.AddOrDecreaseStockHandler)
	router.DELETE("/api/stock/:product_id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.StockDelete), stockHandler.DeleteStockHandler)
	router.GET("/api/stock", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.StockRead), stockHandler.GetAllStocksHandler)
	router.POST("/api/stock/bulk", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.StockCreate), stockHandler.BulkStockCreationOrUpdatingHandler)

	// Bill Owner routes
	router.GET("/api/bill-owners/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.BillOwnersRead), billOwnerHandler.GetByIDHandler)

	// Supplier routes
	router.POST("/api/suppliers", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SuppliersCreate), supplierHandler.CreateSupplierHandler)
	router.GET("/api/suppliers", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SuppliersRead), supplierHandler.ListSuppliersHandler)
	router.GET("/api/suppliers/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SuppliersRead), supplierHandler.GetSupplierByIDHandler)
	router.PUT("/api/suppliers/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SuppliersUpdate), supplierHandler.UpdateSupplierHandler)
	router.DELETE("/api/suppliers/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SuppliersDelete), supplierHandler.DeleteSupplierHandler)

	// Supplier Catalog routes
	router.POST("/api/suppliers/:id/products", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SupplierCatalogCreate), supplierHandler.AddProductToSupplierHandler)
	router.PUT("/api/suppliers/:id/products/:product_id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SupplierCatalogUpdate), supplierHandler.UpdateSupplierCatalogHandler)
	router.DELETE("/api/suppliers/:id/products/:product_id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SupplierCatalogDelete), supplierHandler.RemoveProductFromSupplierHandler)
	router.GET("/api/suppliers/:id/products", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SupplierCatalogRead), supplierHandler.GetSupplierProductsHandler)
	router.GET("/api/products/:id/suppliers", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SupplierCatalogRead), supplierHandler.GetProductSuppliersHandler)

	// Purchase Entry routes
	router.POST("/api/purchase-entries", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.PurchaseEntriesCreate), purchaseEntryHandler.CreatePurchaseEntryHandler)
	router.GET("/api/purchase-entries", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.PurchaseEntriesRead), purchaseEntryHandler.ListPurchaseEntriesHandler)
	router.GET("/api/purchase-entries/export", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.PurchaseEntriesExport), purchaseEntryHandler.ExportPurchaseEntriesCSVHandler)
	router.GET("/api/purchase-entries/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.PurchaseEntriesRead), purchaseEntryHandler.GetPurchaseEntryByIDHandler)
	router.GET("/api/suppliers/:id/purchase-entries", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.PurchaseEntriesRead), purchaseEntryHandler.GetPurchaseEntriesBySupplierHandler)
	router.POST("/api/purchase-entries/:id/documents", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.PurchaseEntriesUpload), purchaseEntryHandler.UploadPurchaseEntryDocumentHandler)

	// Expense Category routes
	router.POST("/api/expense-categories", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ExpenseCategoriesCreate), expenseHandler.CreateCategoryHandler)
	router.GET("/api/expense-categories", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ExpenseCategoriesRead), expenseHandler.ListCategoriesHandler)
	router.GET("/api/expense-categories/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ExpenseCategoriesRead), expenseHandler.GetCategoryByIDHandler)
	router.PUT("/api/expense-categories/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ExpenseCategoriesUpdate), expenseHandler.UpdateCategoryHandler)

	// Expense routes
	router.POST("/api/expenses", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ExpensesCreate), expenseHandler.CreateExpenseHandler)
	router.GET("/api/expenses", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ExpensesRead), expenseHandler.ListExpensesHandler)
	router.GET("/api/expenses/export", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ExpensesExport), expenseHandler.ExportExpensesCSVHandler)
	router.GET("/api/expenses/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ExpensesRead), expenseHandler.GetExpenseByIDHandler)
	router.PUT("/api/expenses/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ExpensesUpdate), expenseHandler.UpdateExpenseHandler)
	router.DELETE("/api/expenses/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ExpensesDelete), expenseHandler.DeleteExpenseHandler)
	router.POST("/api/expenses/:id/documents", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ExpensesUpload), expenseHandler.UploadExpenseDocumentHandler)

	// Financial routes
	router.GET("/api/financial/summary", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.FinancialRead), financialHandler.GetFinancialSummaryHandler)

	// SSE routes for real-time open bill product notifications
	router.GET("/api/sse/commands/:area", handler.SSEMiddleware(), handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SSECommandsRead), sseHandler.StreamCommandsHandler)
	router.GET("/api/sse/open-bill-products/:area", handler.SSEMiddleware(), handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SSECommandItemsRead), sseHandler.StreamOpenBillProductsHandler)
	router.GET("/api/open-bill-products/:area/pending", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.OrdersRead), sseHandler.GetPendingOpenBillProductsHandler)
	router.GET("/api/open-bill-products/:area/completed", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.OrdersRead), sseHandler.GetCompletedOpenBillProductsHandler)

	// ---------------------------------------------------------------------
	// Mode-specific wiring seam (edge vs cloud).
	//
	// Single attachment point for components that only exist in one run mode.
	// Every shared dependency is already constructed above (cfg, logger, the
	// services, the unit of work, and the fully-routed gin engine), so later
	// steps wire here without restructuring main:
	//   - EDGE  (Step 8/9, §6): sync push/pull loops + pending-invoice retry.
	//   - CLOUD (Step 7):        POST /api/sync/push + NodeAuthMiddleware.
	// Both branches only log today so the seam is observable and lint-clean.
	switch cfg.AppMode {
	case config.ModeEdge:
		logger.Info("Running in EDGE mode", zap.String("app_mode", string(cfg.AppMode)))

		// Ticket printing (POST /api/device/print) — edge only. Build the printer
		// transport from PRINTER_* config; if it fails (e.g. windows transport on a
		// non-windows host, or an unreachable target), log and skip the route so the
		// node still boots and the frontend falls back to browser printing.
		deviceCfg := device.Config{
			Transport: cfg.PrinterTransport,
			Target:    cfg.PrinterTarget,
			WidthMM:   cfg.PrinterWidthMM,
			Codepage:  cfg.PrinterCodepage,
			Cut:       cfg.PrinterCut,
		}
		if receiptPrinter, printerErr := device.NewReceiptPrinterFromConfig(deviceCfg); printerErr != nil {
			logger.Error("Ticket printing disabled: failed to init printer transport", zap.Error(printerErr))
		} else {
			printService := service.NewPrintService(openBillRepo, receiptPrinter, dto.TicketBusinessInfo{
				Name:        cfg.BusinessName,
				NIT:         cfg.BusinessNIT,
				Address:     cfg.BusinessAddress,
				Footer:      cfg.TicketFooter,
				LegalNotice: cfg.TicketLegalNotice,
			})
			deviceHandler := handler.NewDeviceHandler(printService)
			router.POST("/api/device/print", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.OrdersRead), deviceHandler.PrintTicketHandler)
			logger.Info("Ticket printing enabled", zap.String("transport", deviceCfg.Transport))
		}

		if cfg.CloudSyncURL == "" || cfg.NodeSyncKey == "" {
			logger.Fatal("Edge sync push disabled: set CLOUD_SYNC_URL and NODE_SYNC_KEY to enable it")
			break
		}
		syncPushClient := httpclient.NewSyncPushClient(httpClient, cfg.CloudSyncURL, cfg.NodeSyncKey)
		syncPushService := service.NewSyncPushService(
			unitOfWork, syncOutboxRepo, syncStateRepo, syncPushClient,
			syncIdentity, 0, slogLogger,
		)
		syncPullClient := httpclient.NewSyncPullClient(httpClient, cfg.CloudSyncURL, cfg.NodeSyncKey)
		syncPullService := service.NewSyncPullService(
			unitOfWork, syncPullClient, syncReferenceRepo, syncStateRepo,
			syncIdentity, slogLogger,
		)
		edgeScheduler, edgeErr := cron.NewEdgeSyncScheduler(syncPushService, syncPullService, syncStatusTracker, cfg.SyncPushCron, cfg.SyncPullCron, logger)
		if edgeErr != nil {
			log.Fatalf("Failed to create edge sync scheduler: %v", edgeErr)
		}
		if edgeErr := edgeScheduler.Start(); edgeErr != nil {
			log.Fatalf("Failed to start edge sync scheduler: %v", edgeErr)
		}
		defer func() {
			if stopErr := edgeScheduler.Stop(); stopErr != nil {
				log.Printf("Failed to stop edge sync scheduler: %v", stopErr)
			}
		}()
	case config.ModeCloud:
		logger.Info("Running in CLOUD mode", zap.String("app_mode", string(cfg.AppMode)))
		// Cloud is the sync aggregate: accept pushes from edge nodes and serve them the
		// reference changes they pull. Both routes require the shared node key.
		syncHandler := handler.NewSyncHandler(syncService, syncReferenceService)
		router.POST("/api/sync/push", handler.NodeAuthMiddleware(cfg), syncHandler.PushHandler)
		router.GET("/api/sync/pull", handler.NodeAuthMiddleware(cfg), syncHandler.PullHandler)
	}

	port := os.Getenv("PORT")
	if port == "" {
		panic("PORT is not set")
	}

	// Create HTTP server
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Server starting on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Close SSE hubs to disconnect all SSE clients
	log.Println("Closing SSE connections...")
	sseHub.Close()
	openBillProductHub.Close()

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

func runMigrations(dsn string) (err error) {
	migrationURL := convertDSNToURL(dsn)

	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("failed to open embedded migrations: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, migrationURL)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer func() {
		if sourceErr, databaseErr := m.Close(); sourceErr != nil || databaseErr != nil {
			err = fmt.Errorf("failed to close migrate instance: %w", sourceErr)
			err = fmt.Errorf("failed to close migrate instance: %w", databaseErr)
		}
	}()

	if err = m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	if err == migrate.ErrNoChange {
		log.Println("No new migrations to apply")
	} else {
		log.Println("Migrations completed successfully")
	}

	return nil
}

func convertDSNToURL(dsn string) string {
	// Parse DSN components
	var host, port, user, password, dbname, sslmode string

	// Simple parser for key=value format
	parts := make(map[string]string)
	for _, part := range splitDSN(dsn) {
		if kv := splitKeyValue(part); len(kv) == 2 {
			parts[kv[0]] = kv[1]
		}
	}

	host = parts["host"]
	port = parts["port"]
	user = parts["user"]
	password = parts["password"]
	dbname = parts["dbname"]
	sslmode = parts["sslmode"]

	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		user, password, host, port, dbname, sslmode)
}

func splitDSN(dsn string) []string {
	var parts []string
	var current string
	for _, char := range dsn {
		if char == ' ' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func splitKeyValue(kv string) []string {
	for i, char := range kv {
		if char == '=' {
			return []string{kv[:i], kv[i+1:]}
		}
	}
	return []string{kv}
}

func getDSN() string {
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "postgres")
	password := getEnv("DB_PASSWORD", "postgres")
	dbname := getEnv("DB_NAME", "laguna_escondida")
	sslmode := getEnv("DB_SSLMODE", "disable")

	// TimeZone=UTC pins the connection's session timezone so timestamptz values
	// are read back and serialized in UTC (…Z) consistently, regardless of the
	// host's local timezone. (convertDSNToURL ignores this key, so the migrator
	// URL is unaffected.)
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=UTC",
		host, port, user, password, dbname, sslmode)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
