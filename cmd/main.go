package main

import (
	"fmt"
	"log"
	"os"

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
	electronicInvoiceClient := httpclient.NewElectronicInvoiceClient(cfg)
	billRepo := repository.NewBillRepository(db.DB, electronicInvoiceClient)
	invoiceService := service.NewInvoiceService(electronicInvoiceClient, productRepo, billRepo)

	// Initialize services
	orderService := service.NewOrderService(openBillRepo, productRepo, invoiceService)
	productService := service.NewProductService(productRepo)

	// Initialize handlers
	orderHandler := handler.NewOrderHandler(orderService)
	productHandler := handler.NewProductHandler(productService)
	invoiceHandler := handler.NewInvoiceHandler(invoiceService)

	// Setup routes
	router := gin.Default()

	// Apply CORS middleware globally
	router.Use(handler.CORSMiddleware())

	// Health check
	router.GET("/api/health", handler.HealthCheckHandler)

	// Order routes
	router.POST("/api/orders", orderHandler.CreateOrderHandler)
	router.PUT("/api/orders/:id", orderHandler.UpdateOrderHandler)
	router.POST("/api/orders/:id/pay", orderHandler.PayOrderHandler)

	// Product routes
	router.POST("/api/products", productHandler.CreateProductHandler)
	router.GET("/api/products", productHandler.ListProductsHandler)
	router.GET("/api/products/:id", productHandler.GetProductByIDHandler)
	router.PUT("/api/products/:id", productHandler.UpdateProductHandler)
	router.DELETE("/api/products/:id", productHandler.DeleteProductHandler)

	// Invoice routes
	router.POST("/api/invoices", invoiceHandler.CreateElectronicInvoiceHandler)

	port := os.Getenv("PORT")
	if port == "" {
		panic("PORT is not set")
	}

	log.Printf("Server starting on port %s", port)
	log.Fatal(router.Run(":" + port))
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
