package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gorilla/mux"
)

type ProductHandler struct {
	productService *service.ProductService
}

func NewProductHandler(productService *service.ProductService) *ProductHandler {
	return &ProductHandler{
		productService: productService,
	}
}

func (h *ProductHandler) CreateProductHandler(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Error decoding request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	product, err := h.productService.CreateProduct(r.Context(), &req)
	if err != nil {
		log.Printf("Error creating product: %v", err)

		if errors.Is(err, domainError.ErrProductCreationFailed) {
			http.Error(w, "Failed to create product", http.StatusInternalServerError)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(product); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

func (h *ProductHandler) UpdateProductHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	productID := vars["id"]
	if productID == "" {
		http.Error(w, "Product ID is required", http.StatusBadRequest)
		return
	}

	var req dto.UpdateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Error decoding request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	product, err := h.productService.UpdateProduct(r.Context(), productID, &req)
	if err != nil {
		log.Printf("Error updating product: %v", err)

		if errors.Is(err, domainError.ErrProductNotFound) {
			http.Error(w, "Product not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, domainError.ErrProductUpdateFailed) {
			http.Error(w, "Failed to update product", http.StatusInternalServerError)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(product); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

func (h *ProductHandler) DeleteProductHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	productID := vars["id"]
	if productID == "" {
		http.Error(w, "Product ID is required", http.StatusBadRequest)
		return
	}

	err := h.productService.DeleteProduct(r.Context(), productID)
	if err != nil {
		log.Printf("Error deleting product: %v", err)

		if errors.Is(err, domainError.ErrProductNotFound) {
			http.Error(w, "Product not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, domainError.ErrProductDeleteFailed) {
			http.Error(w, "Failed to delete product", http.StatusInternalServerError)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ProductHandler) ListProductsHandler(w http.ResponseWriter, r *http.Request) {
	products, err := h.productService.ListProducts(r.Context())
	if err != nil {
		log.Printf("Error listing products: %v", err)
		http.Error(w, "Failed to list products", http.StatusInternalServerError)
		return
	}

	total := len(products)
	response := dto.ProductListResponse{
		Products: products,
		Total:    &total,
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

func (h *ProductHandler) GetProductByIDHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	productID := vars["id"]
	if productID == "" {
		http.Error(w, "Product ID is required", http.StatusBadRequest)
		return
	}

	product, err := h.productService.GetProductByID(r.Context(), productID)
	if err != nil {
		log.Printf("Error getting product: %v", err)

		if errors.Is(err, domainError.ErrProductNotFound) {
			http.Error(w, "Product not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(product); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}
