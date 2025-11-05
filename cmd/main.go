package main

import (
	"log"
	"net/http"
	"os"

	"laguna-escondida/backend/internal/platform/handler"

	"github.com/gorilla/mux"
)

func main() {
	router := mux.NewRouter()

	router.HandleFunc("/api/health", handler.HealthCheckHandler).Methods("GET", "OPTIONS")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}
