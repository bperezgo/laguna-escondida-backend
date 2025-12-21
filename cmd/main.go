package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/handler"
	"laguna-escondida/backend/internal/platform/httpclient"
	"laguna-escondida/backend/internal/platform/postgres"
	"laguna-escondida/backend/internal/platform/postgres/repository"

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
	defer logger.Sync()

	// Database connection
	dsn := getDSN()

	// Run migrations before connecting to database
	log.Println("Running database migrations...")
	if err := runMigrations(dsn); err != nil {
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

	// Initialize repositories
	productRepo := repository.NewProductRepository(db.DB)
	openBillRepo := repository.NewOpenBillRepository(db.DB)
	stockRepo := repository.NewStockRepository(db.DB)
	userRepo := repository.NewUserRepository(db.DB)
	roleRepo := repository.NewRoleRepository(db.DB)
	userRoleRepo := repository.NewUserRoleRepository(db.DB)
	billOwnerRepo := repository.NewBillOwnerRepository(db.DB)
	electronicInvoiceClient := httpclient.NewElectronicInvoiceClient(cfg, httpClient)
	billRepo := repository.NewBillRepository(db.DB, electronicInvoiceClient, cfg)
	invoiceService := service.NewInvoiceService(electronicInvoiceClient, productRepo, billRepo)

	// Initialize JWT service
	jwtService := service.NewJWTService(cfg.JWTSecret)

	// Initialize services
	unitOfWork := postgres.NewUnitOfWork(db.DB)
	orderService := service.NewOrderService(openBillRepo, productRepo, billRepo, billOwnerRepo, invoiceService, unitOfWork)
	productService := service.NewProductService(productRepo)
	stockService := service.NewStockService(stockRepo, productRepo)
	userService := service.NewUserService(userRepo, roleRepo, userRoleRepo, jwtService)
	billOwnerService := service.NewBillOwnerService(billOwnerRepo)

	// Initialize handlers
	orderHandler := handler.NewOrderHandler(orderService)
	productHandler := handler.NewProductHandler(productService)
	stockHandler := handler.NewStockHandler(stockService)
	invoiceHandler := handler.NewInvoiceHandler(invoiceService)
	userHandler := handler.NewUserHandler(userService)
	billOwnerHandler := handler.NewBillOwnerHandler(billOwnerService)

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

	// User routes (protected with admin API key)
	router.POST("/api/users", handler.AdminAPIKeyMiddleware(cfg), userHandler.CreateUserHandler)

	// Admin routes (protected with admin API key)
	router.POST("/api/invoices/update-missing-document-urls", handler.AdminAPIKeyMiddleware(cfg), invoiceHandler.UpdateMissingDocumentURLsHandler)

	// Protected routes (require JWT authentication)
	// Order routes
	router.POST("/api/orders", handler.JWTAuthMiddleware(jwtService), orderHandler.CreateOrderHandler)
	router.GET("/api/orders", handler.JWTAuthMiddleware(jwtService), orderHandler.GetAllActiveOpenBillsHandler)
	router.GET("/api/orders/:id", handler.JWTAuthMiddleware(jwtService), orderHandler.GetOpenBillWithProductsHandler)
	router.PUT("/api/orders/:id", handler.JWTAuthMiddleware(jwtService), orderHandler.UpdateOrderHandler)
	router.POST("/api/orders/pay-order", handler.JWTAuthMiddleware(jwtService), orderHandler.PayOrderHandler)

	// Product routes
	router.POST("/api/products", handler.JWTAuthMiddleware(jwtService), productHandler.CreateProductHandler)
	router.GET("/api/products", handler.JWTAuthMiddleware(jwtService), productHandler.ListProductsHandler)
	router.GET("/api/products/:id", handler.JWTAuthMiddleware(jwtService), productHandler.GetProductByIDHandler)
	router.PUT("/api/products/:id", handler.JWTAuthMiddleware(jwtService), productHandler.UpdateProductHandler)
	router.DELETE("/api/products/:id", handler.JWTAuthMiddleware(jwtService), productHandler.DeleteProductHandler)

	// Invoice routes
	router.POST("/api/invoices", handler.JWTAuthMiddleware(jwtService), invoiceHandler.CreateElectronicInvoiceHandler)
	router.GET("/api/invoices", handler.JWTAuthMiddleware(jwtService), invoiceHandler.ListInvoicesHandler)

	// Stock routes
	router.POST("/api/stock", handler.JWTAuthMiddleware(jwtService), stockHandler.CreateStockHandler)
	router.PUT("/api/stock/:product_id/add-or-decrease", handler.JWTAuthMiddleware(jwtService), stockHandler.AddOrDecreaseStockHandler)
	router.DELETE("/api/stock/:product_id", handler.JWTAuthMiddleware(jwtService), stockHandler.DeleteStockHandler)
	router.GET("/api/stock", handler.JWTAuthMiddleware(jwtService), stockHandler.GetAllStocksHandler)
	router.POST("/api/stock/bulk", handler.JWTAuthMiddleware(jwtService), stockHandler.BulkStockCreationOrUpdatingHandler)

	// Bill Owner routes
	router.GET("/api/bill-owners/:id", handler.JWTAuthMiddleware(jwtService), billOwnerHandler.GetByIDHandler)

	port := os.Getenv("PORT")
	if port == "" {
		panic("PORT is not set")
	}

	// Create HTTP server
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
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

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

func runMigrations(dsn string) error {
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
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
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
