package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/handler"
	"laguna-escondida/backend/internal/platform/postgres/repository"

	"github.com/gorilla/mux"
)

func main() {
	// Database connection
	dsn := getDSN()
	db, err := repository.NewDatabase(dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Initialize repositories
	productRepo := repository.NewProductRepository(db.DB)
	openBillRepo := repository.NewOpenBillRepository(db.DB)

	// Initialize services
	orderService := service.NewOrderService(openBillRepo, productRepo)

	// Initialize handlers
	orderHandler := handler.NewOrderHandler(orderService)

	// Setup routes
	router := mux.NewRouter()

	// Apply CORS middleware to routes
	healthMiddleware := handler.CORSMiddleware([]string{"GET", "OPTIONS"})
	orderMiddleware := handler.CORSMiddleware([]string{"POST", "OPTIONS"})
	updateOrderMiddleware := handler.CORSMiddleware([]string{"PUT", "OPTIONS"})
	payOrderMiddleware := handler.CORSMiddleware([]string{"POST", "OPTIONS"})

	router.HandleFunc("/api/health", healthMiddleware(http.HandlerFunc(handler.HealthCheckHandler)).ServeHTTP).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/orders", orderMiddleware(http.HandlerFunc(orderHandler.CreateOrderHandler)).ServeHTTP).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/orders/{id}", updateOrderMiddleware(http.HandlerFunc(orderHandler.UpdateOrderHandler)).ServeHTTP).Methods("PUT", "OPTIONS")
	router.HandleFunc("/api/orders/{id}/pay", payOrderMiddleware(http.HandlerFunc(orderHandler.PayOrderHandler)).ServeHTTP).Methods("POST", "OPTIONS")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
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
