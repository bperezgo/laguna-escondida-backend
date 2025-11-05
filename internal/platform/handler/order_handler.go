package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"laguna-escondida/backend/internal/domain/dto"
	orderError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gorilla/mux"
)

type OrderHandler struct {
	orderService *service.OrderService
}

func NewOrderHandler(orderService *service.OrderService) *OrderHandler {
	return &OrderHandler{
		orderService: orderService,
	}
}

func (h *OrderHandler) CreateOrderHandler(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Error decoding request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.ProductIDs == nil {
		req.ProductIDs = []string{} // Allow empty order
	}

	openBill, err := h.orderService.CreateOrder(r.Context(), &req)
	if err != nil {
		log.Printf("Error creating order: %v", err)

		if errors.Is(err, orderError.ErrProductNotFound) {
			http.Error(w, "One or more products not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, orderError.ErrOrderCreationFailed) {
			http.Error(w, "Failed to create order", http.StatusInternalServerError)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(openBill); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

func (h *OrderHandler) UpdateOrderHandler(w http.ResponseWriter, r *http.Request) {
	// Extract open_bill_id from URL path
	vars := mux.Vars(r)
	openBillID := vars["id"]
	if openBillID == "" {
		http.Error(w, "Order ID is required", http.StatusBadRequest)
		return
	}

	var req dto.UpdateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Error decoding request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request - allow empty products array (will clear the order)
	if req.Products == nil {
		req.Products = []dto.OrderProductItem{}
	}

	openBill, err := h.orderService.UpdateOrder(r.Context(), openBillID, &req)
	if err != nil {
		log.Printf("Error updating order: %v", err)

		if errors.Is(err, orderError.ErrOrderNotFound) {
			http.Error(w, "Order not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, orderError.ErrProductNotFound) {
			http.Error(w, "One or more products not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, orderError.ErrOrderUpdateFailed) {
			http.Error(w, "Failed to update order", http.StatusInternalServerError)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(openBill); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}
