package handler

import (
	"net/http"
)

// CORSMiddleware handles CORS headers and OPTIONS requests
func CORSMiddleware(allowedMethods []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Access-Control-Allow-Origin", "*")

			// Build allowed methods string
			methodsStr := ""
			for i, method := range allowedMethods {
				if i > 0 {
					methodsStr += ", "
				}
				methodsStr += method
			}
			w.Header().Set("Access-Control-Allow-Methods", methodsStr)
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

			// Handle OPTIONS preflight requests
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			// Validate allowed methods
			if !contains(allowedMethods, r.Method) {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// contains checks if a slice contains a string
func contains(slice []string, str string) bool {
	for _, v := range slice {
		if v == str {
			return true
		}
	}
	return false
}
