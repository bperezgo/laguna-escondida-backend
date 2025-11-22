package handler

import (
	"errors"
	"log"
	"net/http"

	"laguna-escondida/backend/internal/domain/dto"
	orderError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/service"

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

	userID, ok := ctx.Value("user_id").(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Username not found in context"})
		return
	}

	openBill, err := h.orderService.CreateOrder(ctx, &req, dto.UserDomain{ID: userID})
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
	openBillID := c.Param("id")
	if openBillID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order ID is required"})
		return
	}

	bill, err := h.orderService.PayOrder(c.Request.Context(), openBillID)
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

	c.JSON(http.StatusCreated, bill)
}
