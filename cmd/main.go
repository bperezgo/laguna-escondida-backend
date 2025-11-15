package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/handler"
	"laguna-escondida/backend/internal/platform/httpclient"
	"laguna-escondida/backend/internal/platform/postgres/repository"

	"github.com/gorilla/mux"
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
	router := mux.NewRouter()

	// Apply CORS middleware to routes
	healthMiddleware := handler.CORSMiddleware([]string{"GET", "OPTIONS"})
	orderMiddleware := handler.CORSMiddleware([]string{"POST", "OPTIONS"})
	updateOrderMiddleware := handler.CORSMiddleware([]string{"PUT", "OPTIONS"})
	payOrderMiddleware := handler.CORSMiddleware([]string{"POST", "OPTIONS"})
	productGetMiddleware := handler.CORSMiddleware([]string{"GET", "OPTIONS"})
	productPostMiddleware := handler.CORSMiddleware([]string{"POST", "OPTIONS"})
	productPutMiddleware := handler.CORSMiddleware([]string{"PUT", "OPTIONS"})
	productDeleteMiddleware := handler.CORSMiddleware([]string{"DELETE", "OPTIONS"})
	invoicePostMiddleware := handler.CORSMiddleware([]string{"POST", "OPTIONS"})

	router.HandleFunc("/api/health", healthMiddleware(http.HandlerFunc(handler.HealthCheckHandler)).ServeHTTP).Methods("GET", "OPTIONS")

	// Order routes
	router.HandleFunc("/api/orders", orderMiddleware(http.HandlerFunc(orderHandler.CreateOrderHandler)).ServeHTTP).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/orders/{id}", updateOrderMiddleware(http.HandlerFunc(orderHandler.UpdateOrderHandler)).ServeHTTP).Methods("PUT", "OPTIONS")
	router.HandleFunc("/api/orders/{id}/pay", payOrderMiddleware(http.HandlerFunc(orderHandler.PayOrderHandler)).ServeHTTP).Methods("POST", "OPTIONS")

	// Product routes
	router.HandleFunc("/api/products", productPostMiddleware(http.HandlerFunc(productHandler.CreateProductHandler)).ServeHTTP).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/products", productGetMiddleware(http.HandlerFunc(productHandler.ListProductsHandler)).ServeHTTP).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/products/{id}", productGetMiddleware(http.HandlerFunc(productHandler.GetProductByIDHandler)).ServeHTTP).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/products/{id}", productPutMiddleware(http.HandlerFunc(productHandler.UpdateProductHandler)).ServeHTTP).Methods("PUT", "OPTIONS")
	router.HandleFunc("/api/products/{id}", productDeleteMiddleware(http.HandlerFunc(productHandler.DeleteProductHandler)).ServeHTTP).Methods("DELETE", "OPTIONS")

	// Invoice routes
	router.HandleFunc("/api/invoices/electronic", invoicePostMiddleware(http.HandlerFunc(invoiceHandler.CreateElectronicInvoiceHandler)).ServeHTTP).Methods("POST", "OPTIONS")

	port := os.Getenv("PORT")
	if port == "" {
		panic("PORT is not set")
	}

	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
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
