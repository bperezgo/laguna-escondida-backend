package handler

import (
	"errors"
	"log"
	"net/http"

	"laguna-escondida/backend/internal/domain/command"
	"laguna-escondida/backend/internal/domain/dto"
	orderError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/service"
	orderdto "laguna-escondida/backend/internal/platform/dto/order"

	"github.com/gin-gonic/gin"
)

type OrderHandler struct {
	orderService *service.OrderService
}

func NewOrderHandler(orderService *service.OrderService) *OrderHandler {
	return &OrderHandler{
		orderService: orderService,
	}
}

func (h *OrderHandler) CreateOrderHandler(c *gin.Context) {
	var req dto.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error decoding request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	ctx := c.Request.Context()

	userID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Username not found in context"})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID is not a string"})
		return
	}

	openBill, err := h.orderService.CreateOrder(ctx, &req, dto.UserDomain{ID: userIDStr})
	if err != nil {
		log.Printf("Error creating order: %v", err)

		if errors.Is(err, orderError.ErrProductNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "One or more products not found"})
			return
		}
		if errors.Is(err, orderError.ErrOrderCreationFailed) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create order"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusCreated, openBill)
}

func (h *OrderHandler) UpdateOrderHandler(c *gin.Context) {
	openBillID := c.Param("id")
	if openBillID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order ID is required"})
		return
	}

	var req dto.UpdateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error decoding request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Validate request - allow empty products array (will clear the order)
	if req.Products == nil {
		req.Products = []dto.OrderProductItem{}
	}

	openBill, err := h.orderService.UpdateOrder(c.Request.Context(), openBillID, &req)
	if err != nil {
		log.Printf("Error updating order: %v", err)

		if errors.Is(err, orderError.ErrOrderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}
		if errors.Is(err, orderError.ErrProductNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "One or more products not found"})
			return
		}
		if errors.Is(err, orderError.ErrOrderUpdateFailed) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, openBill)
}

func (h *OrderHandler) PayOrderHandler(c *gin.Context) {
	var req orderdto.PayOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error decoding request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	err := h.orderService.PayOrder(c.Request.Context(), command.PayOrderCommand{
		OpenBillID:  req.OrderID,
		PaymentCode: req.PaymentType,
		Customer:    req.Customer,
	})
	if err != nil {
		log.Printf("Error paying order: %v", err)

		if errors.Is(err, orderError.ErrOrderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}
		if errors.Is(err, orderError.ErrOrderPaymentFailed) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to pay order"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Order paid successfully"})
}

func (h *OrderHandler) GetAllActiveOpenBillsHandler(c *gin.Context) {
	openBills, err := h.orderService.GetAllActiveOpenBills(c.Request.Context())
	if err != nil {
		log.Printf("Error getting active open bills: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve open bills"})
		return
	}

	c.JSON(http.StatusOK, openBills)
}

func (h *OrderHandler) GetOpenBillWithProductsHandler(c *gin.Context) {
	openBillID := c.Param("id")
	if openBillID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order ID is required"})
		return
	}

	openBill, err := h.orderService.GetOpenBillWithProducts(c.Request.Context(), openBillID)
	if err != nil {
		log.Printf("Error getting open bill with products: %v", err)

		if errors.Is(err, orderError.ErrOrderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, openBill)
}

func (h *OrderHandler) DeleteOrderHandler(c *gin.Context) {
	openBillID := c.Param("id")
	if openBillID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order ID is required"})
		return
	}

	err := h.orderService.DeleteOrder(c.Request.Context(), openBillID)
	if err != nil {
		log.Printf("Error deleting order: %v", err)

		if errors.Is(err, orderError.ErrOrderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}
		if errors.Is(err, orderError.ErrOrderDeletionFailed) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete order"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Order deleted successfully"})
}
