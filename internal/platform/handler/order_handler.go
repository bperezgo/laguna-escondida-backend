package handler

import (
	"errors"
	"log"
	"net/http"

	openBillError "laguna-escondida/backend/internal/domain/aggregate/open_bill/error"
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
		if errors.Is(err, orderError.ErrDuplicateTemporalIdentifier) {
			c.JSON(http.StatusConflict, gin.H{"error": "An active order with this temporal identifier already exists"})
			return
		}
		if errors.Is(err, orderError.ErrOrderCreationFailed) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create order"})
			return
		}
		RespondError(c, err)
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
		RespondError(c, err)
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
		RespondError(c, err)
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

// GetClosedOpenBillsTodayHandler returns the orders closed today (soft-deleted open
// bills whose created_at is within the current local business day, America/Bogota).
// Powers the read-only "Órdenes cerradas hoy" view used to reprint a paid cuenta.
func (h *OrderHandler) GetClosedOpenBillsTodayHandler(c *gin.Context) {
	from, to := businessDayRange()

	openBills, err := h.orderService.GetClosedOpenBills(c.Request.Context(), from, to)
	if err != nil {
		log.Printf("Error getting closed open bills: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve closed open bills"})
		return
	}

	c.JSON(http.StatusOK, openBills)
}

// GetClosedOpenBillWithProductsHandler returns a single closed (soft-deleted) open bill
// with its products, for the closed-order detail view and cuenta reprint.
func (h *OrderHandler) GetClosedOpenBillWithProductsHandler(c *gin.Context) {
	openBillID := c.Param("id")
	if openBillID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order ID is required"})
		return
	}

	openBill, err := h.orderService.GetClosedOpenBillWithProducts(c.Request.Context(), openBillID)
	if err != nil {
		log.Printf("Error getting closed open bill with products: %v", err)

		if errors.Is(err, orderError.ErrOrderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}
		RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, openBill)
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
		RespondError(c, err)
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
		RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Order deleted successfully"})
}

func (h *OrderHandler) CompleteOpenBillProductHandler(c *gin.Context) {
	openBillID := c.Param("id")
	if openBillID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order ID is required"})
		return
	}

	openBillProductID := c.Param("open_bill_product_id")
	if openBillProductID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Product ID is required"})
		return
	}

	err := h.orderService.CompleteOpenBillProduct(c.Request.Context(), openBillID, openBillProductID)
	if err != nil {
		log.Printf("Error completing open bill product: %v", err)

		if errors.Is(err, orderError.ErrOrderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}
		if errors.Is(err, openBillError.ErrOpenBillProductNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found in order"})
			return
		}
		if errors.Is(err, openBillError.ErrProductAlreadyCompleted) {
			c.JSON(http.StatusConflict, gin.H{"error": "Product is already completed"})
			return
		}
		if errors.Is(err, openBillError.ErrCannotCompleteProduct) {
			c.JSON(http.StatusConflict, gin.H{"error": "Product cannot be completed from current status"})
			return
		}
		RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Product completed successfully"})
}

func (h *OrderHandler) UncompleteOpenBillProductHandler(c *gin.Context) {
	openBillID := c.Param("id")
	if openBillID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order ID is required"})
		return
	}

	openBillProductID := c.Param("open_bill_product_id")
	if openBillProductID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Product ID is required"})
		return
	}

	err := h.orderService.UncompleteOpenBillProduct(c.Request.Context(), openBillID, openBillProductID)
	if err != nil {
		log.Printf("Error uncompleting open bill product: %v", err)

		if errors.Is(err, orderError.ErrOrderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}
		if errors.Is(err, openBillError.ErrOpenBillProductNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found in order"})
			return
		}
		if errors.Is(err, openBillError.ErrProductNotCompleted) {
			c.JSON(http.StatusConflict, gin.H{"error": "Product is not completed"})
			return
		}
		RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Product uncompleted successfully"})
}

func (h *OrderHandler) SetOpenBillProductInProgressHandler(c *gin.Context) {
	openBillID := c.Param("id")
	if openBillID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order ID is required"})
		return
	}

	openBillProductID := c.Param("open_bill_product_id")
	if openBillProductID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Product ID is required"})
		return
	}

	err := h.orderService.SetOpenBillProductInProgress(c.Request.Context(), openBillID, openBillProductID)
	if err != nil {
		log.Printf("Error setting open bill product in progress: %v", err)

		if errors.Is(err, orderError.ErrOrderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}
		if errors.Is(err, openBillError.ErrOpenBillProductNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found in order"})
			return
		}
		if errors.Is(err, openBillError.ErrCannotSetInProgressFromCancelled) {
			c.JSON(http.StatusConflict, gin.H{"error": "Cannot set in progress from cancelled status"})
			return
		}
		RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Product set to in progress successfully"})
}

func (h *OrderHandler) CancelOpenBillProductHandler(c *gin.Context) {
	openBillID := c.Param("id")
	if openBillID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order ID is required"})
		return
	}

	openBillProductID := c.Param("open_bill_product_id")
	if openBillProductID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Product ID is required"})
		return
	}

	err := h.orderService.CancelOpenBillProduct(c.Request.Context(), openBillID, openBillProductID)
	if err != nil {
		log.Printf("Error cancelling open bill product: %v", err)

		if errors.Is(err, orderError.ErrOrderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}
		if errors.Is(err, openBillError.ErrOpenBillProductNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found in order"})
			return
		}
		if errors.Is(err, openBillError.ErrProductAlreadyCancelled) {
			c.JSON(http.StatusConflict, gin.H{"error": "Product is already cancelled"})
			return
		}
		if errors.Is(err, openBillError.ErrCannotCancelProduct) {
			c.JSON(http.StatusConflict, gin.H{"error": "Product cannot be cancelled from current status"})
			return
		}
		RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Product cancelled successfully"})
}
