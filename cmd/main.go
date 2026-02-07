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
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/cron"
	"laguna-escondida/backend/internal/platform/handler"
	"laguna-escondida/backend/internal/platform/httpclient"
	"laguna-escondida/backend/internal/platform/postgres"
	"laguna-escondida/backend/internal/platform/postgres/repository"
	"laguna-escondida/backend/internal/platform/sse"
	"laguna-escondida/backend/internal/platform/storage"
	"laguna-escondida/backend/pkg/eventbus"

	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
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
	billRepo := repository.NewBillRepository(db.DB, electronicInvoiceClient, cfg)
	invoiceService := service.NewInvoiceService(electronicInvoiceClient, productRepo, billRepo, spacesClient, cfg.OrganizationID)

	// Initialize SSE Hubs
	sseHub := sse.NewHub()
	openBillProductHub := sse.NewOpenBillProductHub()

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
	)
	productService := service.NewProductService(productRepo)
	stockService := service.NewStockService(stockRepo, productRepo)
	userService := service.NewUserService(userRepo, roleRepo, userRoleRepo, jwtService)
	billOwnerService := service.NewBillOwnerService(billOwnerRepo)
	supplierService := service.NewSupplierService(supplierRepo, supplierCatalogRepo, productRepo)
	purchaseEntryService := service.NewPurchaseEntryService(purchaseEntryRepo, supplierRepo, supplierCatalogRepo, productRepo, spacesClient, eventBusImpl, slogLogger, cfg.OrganizationID)
	expenseService := service.NewExpenseService(expenseCategoryRepo, expenseRepo, supplierRepo, spacesClient, cfg.OrganizationID)
	productIngredientService := service.NewProductIngredientService(productIngredientRepo, productRepo)

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
	go func() {
		if errEventSubscriber := eventSubscriber.Start(context.Background()); errEventSubscriber != nil {
			log.Printf("Event subscriber stopped: %v", errEventSubscriber)
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
	go func() {
		if errStockSubscriber := stockSubscriber.Start(context.Background()); errStockSubscriber != nil {
			log.Printf("Stock subscriber stopped: %v", errStockSubscriber)
		}
	}()

	// Initialize and start cron scheduler
	cronScheduler, err := cron.NewScheduler(invoiceService, logger)
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
	sseHandler := handler.NewSSEHandler(sseHub, openBillProductHub, orderService, logger)

	// Setup routes
	router := gin.Default()

	// Apply CORS middleware globally
	router.Use(handler.CORSMiddleware())

	// Apply Logger middleware globally
	router.Use(handler.LoggerMiddleware(logger))

	// Health check
	router.GET("/api/health", handler.HealthCheckHandler)

	// Auth routes (no authentication required)
	router.POST("/api/auth/signin", userHandler.SignInHandler)

	// Auth routes (protected with JWT)
	router.GET("/api/auth/me", handler.JWTAuthMiddleware(jwtService), userHandler.GetCurrentUserHandler)

	// User routes (protected with admin API key)
	router.POST("/api/users", handler.AdminAPIKeyMiddleware(cfg), userHandler.CreateUserHandler)

	// Admin routes (protected with admin API key)
	router.POST("/api/invoices/update-missing-document-urls", handler.AdminAPIKeyMiddleware(cfg), invoiceHandler.UpdateMissingDocumentURLsHandler)

	// Protected routes (require JWT authentication + permissions)
	// Order routes
	router.POST("/api/orders", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.OrdersCreate), orderHandler.CreateOrderHandler)
	router.GET("/api/orders", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.OrdersRead), orderHandler.GetAllActiveOpenBillsHandler)
	router.GET("/api/orders/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.OrdersRead), orderHandler.GetOpenBillWithProductsHandler)
	router.PUT("/api/orders/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.OrdersUpdate), orderHandler.UpdateOrderHandler)
	router.DELETE("/api/orders/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.OrdersDelete), orderHandler.DeleteOrderHandler)
	router.POST("/api/orders/pay-order", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.OrdersUpdate), orderHandler.PayOrderHandler)

	// Order product status routes
	router.PATCH("/api/orders/:id/products/:open_bill_product_id/complete", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.OrdersUpdate), orderHandler.CompleteOpenBillProductHandler)
	router.PATCH("/api/orders/:id/products/:open_bill_product_id/in-progress", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.OrdersUpdate), orderHandler.SetOpenBillProductInProgressHandler)
	router.PATCH("/api/orders/:id/products/:open_bill_product_id/cancel", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.OrdersUpdate), orderHandler.CancelOpenBillProductHandler)

	// Product routes
	router.POST("/api/products", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ProductsCreate), productHandler.CreateProductHandler)
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
	router.GET("/api/expenses/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ExpensesRead), expenseHandler.GetExpenseByIDHandler)
	router.PUT("/api/expenses/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ExpensesUpdate), expenseHandler.UpdateExpenseHandler)
	router.DELETE("/api/expenses/:id", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ExpensesDelete), expenseHandler.DeleteExpenseHandler)
	router.POST("/api/expenses/:id/documents", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.ExpensesUpload), expenseHandler.UploadExpenseDocumentHandler)

	// SSE routes for real-time open bill product notifications
	router.GET("/api/sse/commands/:area", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SSECommandsRead), sseHandler.StreamCommandsHandler)
	router.GET("/api/sse/open-bill-products/:area", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SSECommandItemsRead), sseHandler.StreamOpenBillProductsHandler)
	router.GET("/api/open-bill-products/:area/pending", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.OrdersRead), sseHandler.GetPendingOpenBillProductsHandler)

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
	migrationsPath, err := getMigrationsPath()
	if err != nil {
		return fmt.Errorf("failed to locate migrations directory: %w", err)
	}

	log.Printf("Using migrations from: %s", migrationsPath)

	m, err := migrate.New(
		migrationsPath,
		migrationURL,
	)
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

func getMigrationsPath() (string, error) {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	// Try relative path from current directory (works for both local and Docker)
	relativePath := "internal/platform/postgres/migrations"
	fullPath := fmt.Sprintf("%s/%s", cwd, relativePath)

	// Check if migrations directory exists
	if _, err := os.Stat(fullPath); err == nil {
		return fmt.Sprintf("file://%s", fullPath), nil
	}

	// If not found, return error with helpful message
	return "", fmt.Errorf("migrations directory not found at %s", fullPath)
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

	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
