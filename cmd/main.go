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
	"laguna-escondida/backend/internal/platform/postgres/repository"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Database connection
	dsn := getDSN()
	db, err := repository.NewDatabase(dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize repositories
	productRepo := repository.NewProductRepository(db.DB)
	openBillRepo := repository.NewOpenBillRepository(db.DB)
	stockRepo := repository.NewStockRepository(db.DB)
	userRepo := repository.NewUserRepository(db.DB)
	roleRepo := repository.NewRoleRepository(db.DB)
	userRoleRepo := repository.NewUserRoleRepository(db.DB)
	electronicInvoiceClient := httpclient.NewElectronicInvoiceClient(cfg)
	billRepo := repository.NewBillRepository(db.DB, electronicInvoiceClient)
	invoiceService := service.NewInvoiceService(electronicInvoiceClient, productRepo, billRepo)

	// Initialize JWT service
	jwtService := service.NewJWTService(cfg.JWTSecret)

	// Initialize services
	orderService := service.NewOrderService(openBillRepo, productRepo, invoiceService)
	productService := service.NewProductService(productRepo)
	stockService := service.NewStockService(stockRepo, productRepo)
	userService := service.NewUserService(userRepo, roleRepo, userRoleRepo, jwtService)

	// Initialize handlers
	orderHandler := handler.NewOrderHandler(orderService)
	productHandler := handler.NewProductHandler(productService)
	stockHandler := handler.NewStockHandler(stockService)
	invoiceHandler := handler.NewInvoiceHandler(invoiceService)
	userHandler := handler.NewUserHandler(userService)

	// Setup routes
	router := gin.Default()

	// Apply CORS middleware globally
	router.Use(handler.CORSMiddleware())

	// Health check
	router.GET("/api/health", handler.HealthCheckHandler)

	// Auth routes (no authentication required)
	router.POST("/api/auth/signin", userHandler.SignInHandler)

	// User routes (protected with admin API key)
	router.POST("/api/users", handler.AdminAPIKeyMiddleware(cfg), userHandler.CreateUserHandler)

	// Protected routes (require JWT authentication)
	// Order routes
	router.POST("/api/orders", handler.JWTAuthMiddleware(jwtService), orderHandler.CreateOrderHandler)
	router.PUT("/api/orders/:id", handler.JWTAuthMiddleware(jwtService), orderHandler.UpdateOrderHandler)
	router.POST("/api/orders/:id/pay", handler.JWTAuthMiddleware(jwtService), orderHandler.PayOrderHandler)

	// Product routes
	router.POST("/api/products", handler.JWTAuthMiddleware(jwtService), productHandler.CreateProductHandler)
	router.GET("/api/products", handler.JWTAuthMiddleware(jwtService), productHandler.ListProductsHandler)
	router.GET("/api/products/:id", handler.JWTAuthMiddleware(jwtService), productHandler.GetProductByIDHandler)
	router.PUT("/api/products/:id", handler.JWTAuthMiddleware(jwtService), productHandler.UpdateProductHandler)
	router.DELETE("/api/products/:id", handler.JWTAuthMiddleware(jwtService), productHandler.DeleteProductHandler)

	// Invoice routes
	router.POST("/api/invoices", handler.JWTAuthMiddleware(jwtService), invoiceHandler.CreateElectronicInvoiceHandler)

	// Stock routes
	router.POST("/api/stock", handler.JWTAuthMiddleware(jwtService), stockHandler.CreateStockHandler)
	router.PUT("/api/stock/:product_id/add-or-decrease", handler.JWTAuthMiddleware(jwtService), stockHandler.AddOrDecreaseStockHandler)
	router.DELETE("/api/stock/:product_id", handler.JWTAuthMiddleware(jwtService), stockHandler.DeleteStockHandler)
	router.GET("/api/stock", handler.JWTAuthMiddleware(jwtService), stockHandler.GetAllStocksHandler)
	router.POST("/api/stock/bulk", handler.JWTAuthMiddleware(jwtService), stockHandler.BulkStockCreationOrUpdatingHandler)

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
