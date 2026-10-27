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
	baseError "laguna-escondida/backend/internal/platform/shared/domain/error"

	"github.com/gin-gonic/gin"
)

type OrderHandler struct {
	orderService *service.OrderService
	idempotency  *service.OrderIdempotencyService
}

func NewOrderHandler(orderService *service.OrderService, idempotency *service.OrderIdempotencyService) *OrderHandler {
	return &OrderHandler{
		orderService: orderService,
		idempotency:  idempotency,
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

	user, ok := currentUser(c)
	if !ok {
		return
	}

	var (
		openBill *dto.OpenBill
		replayed bool
		err      error
	)
	if key := c.GetHeader(IdempotencyKeyHeader); key != "" {
		openBill, replayed, err = h.idempotency.CreateOrder(ctx, key, &req, user)
	} else {
		openBill, err = h.orderService.CreateOrder(ctx, &req, user)
	}
	if err != nil {
		log.Printf("Error creating order: %v", err)

		if respondAgentScopeError(c, err) {
			return
		}
		if respondIdempotencyError(c, err) {
			return
		}

		if errors.Is(err, orderError.ErrProductNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "One or more products not found"})
			return
		}
		if errors.Is(err, orderError.ErrDuplicateTemporalIdentifier) {
			c.JSON(http.StatusConflict, gin.H{"error": "An active order with this temporal identifier already exists"})
			return
		}
		if errors.Is(err, orderError.ErrDeliveryCloudOnly) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Deliveries are taken in the cloud", "code": "DELIVERY_CLOUD_ONLY"})
			return
		}
		if errors.Is(err, orderError.ErrOutsideDeliveryZone) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "The delivery location is outside every delivery zone", "code": "OUTSIDE_DELIVERY_ZONE"})
			return
		}
		if respondOrderingUnavailable(c, err) {
			return
		}
		if errors.Is(err, orderError.ErrInvalidSideDishSelection) || errors.Is(err, orderError.ErrSideDishQuantityOutOfBounds) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if isValidationError(err) {
			RespondError(c, err)
			return
		}
		if errors.Is(err, orderError.ErrOrderCreationFailed) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create order"})
			return
		}
		RespondError(c, err)
		return
	}

	if replayed {
		c.Header(IdempotentReplayedHeader, "true")
	}
	c.JSON(http.StatusCreated, openBill)
}

func respondIdempotencyError(c *gin.Context, err error) bool {
	switch {
	case errors.Is(err, orderError.ErrIdempotencyKeyInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "INVALID_IDEMPOTENCY_KEY"})
	case errors.Is(err, orderError.ErrIdempotencyKeyReused):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error(), "code": "IDEMPOTENCY_KEY_REUSED"})
	case errors.Is(err, orderError.ErrIdempotencyRequestInProgress):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "IDEMPOTENCY_REQUEST_IN_PROGRESS"})
	default:
		return false
	}
	return true
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

	user, ok := currentUser(c)
	if !ok {
		return
	}

	openBill, err := h.orderService.UpdateOrder(c.Request.Context(), openBillID, &req, user)
	if err != nil {
		log.Printf("Error updating order: %v", err)

		if respondAgentScopeError(c, err) {
			return
		}

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

func orderListFilter(c *gin.Context) dto.OrderListFilter {
	return dto.OrderListFilter{
		OrderType:     dto.OrderType(c.Query("order_type")),
		CustomerPhone: c.Query("customer_phone"),
	}
}

func (h *OrderHandler) GetAllActiveOpenBillsHandler(c *gin.Context) {
	openBills, err := h.orderService.GetAllActiveOpenBills(c.Request.Context(), orderListFilter(c))
	if err != nil {
		log.Printf("Error getting active open bills: %v", err)
		if isValidationError(err) {
			RespondError(c, err)
			return
		}
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

	openBills, err := h.orderService.GetClosedOpenBills(c.Request.Context(), from, to, orderListFilter(c))
	if err != nil {
		log.Printf("Error getting closed open bills: %v", err)
		if isValidationError(err) {
			RespondError(c, err)
			return
		}
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

	user, ok := currentUser(c)
	if !ok {
		return
	}

	err := h.orderService.DeleteOrder(c.Request.Context(), openBillID, user)
	if err != nil {
		log.Printf("Error deleting order: %v", err)

		if respondAgentScopeError(c, err) {
			return
		}

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
		if errors.Is(err, openBillError.ErrOrderAwaitingPayment) {
			respondAwaitingPayment(c)
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
		if errors.Is(err, openBillError.ErrOrderAwaitingPayment) {
			respondAwaitingPayment(c)
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

	user, ok := currentUser(c)
	if !ok {
		return
	}

	err := h.orderService.CancelOpenBillProduct(c.Request.Context(), openBillID, openBillProductID, user)
	if err != nil {
		log.Printf("Error cancelling open bill product: %v", err)

		if respondAgentScopeError(c, err) {
			return
		}

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
		if errors.Is(err, openBillError.ErrCannotCancelPaidOrderLine) {
			c.JSON(http.StatusConflict, gin.H{"error": "A line of a paid order cannot be cancelled"})
			return
		}
		RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Product cancelled successfully"})
}

func respondAwaitingPayment(c *gin.Context) {
	c.JSON(http.StatusConflict, gin.H{"error": "The order must be paid before the kitchen can prepare it", "code": "ORDER_AWAITING_PAYMENT"})
}

func isValidationError(err error) bool {
	var baseErr *baseError.BaseError
	return errors.As(err, &baseErr)
}

// currentUser reads the caller set by JWTAuthMiddleware, answering 401 itself when it is missing.
func currentUser(c *gin.Context) (dto.UserDomain, bool) {
	userID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Username not found in context"})
		return dto.UserDomain{}, false
	}
	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID is not a string"})
		return dto.UserDomain{}, false
	}
	roleIDs, _ := c.Get("role_ids")
	roles, _ := roleIDs.([]int)
	return dto.UserDomain{ID: userIDStr, OrganizationID: c.GetString("org_id"), RoleIDs: roles}, true
}

func respondAgentScopeError(c *gin.Context, err error) bool {
	switch {
	case errors.Is(err, orderError.ErrAgentOrderTypeNotAllowed):
		c.JSON(http.StatusForbidden, gin.H{"error": "The agent can only take takeaway and delivery orders", "code": "AGENT_ORDER_TYPE_NOT_ALLOWED"})
	case errors.Is(err, orderError.ErrOrderNotOwnedByAgent):
		c.JSON(http.StatusForbidden, gin.H{"error": "The agent can only change takeaway and delivery orders it created", "code": "ORDER_NOT_OWNED"})
	default:
		return false
	}
	return true
}

// respondOrderingUnavailable puts the reason or the next opening in the body, so the caller can relay it.
func respondOrderingUnavailable(c *gin.Context, err error) bool {
	var paused *orderError.OrderingPausedError
	if errors.As(err, &paused) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":         orderTypeLabel(paused.OrderType) + " orders are paused",
			"code":          "ORDERING_PAUSED",
			"order_type":    paused.OrderType,
			"paused_reason": paused.PausedReason,
		})
		return true
	}
	var outside *orderError.OutsideOrderingHoursError
	if errors.As(err, &outside) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":         orderTypeLabel(outside.OrderType) + " orders are outside their hours",
			"code":          "OUTSIDE_ORDERING_HOURS",
			"order_type":    outside.OrderType,
			"next_opens_at": outside.NextOpensAt,
		})
		return true
	}
	return false
}

func orderTypeLabel(orderType string) string {
	if orderType == string(dto.OrderTypeDelivery) {
		return "Delivery"
	}
	return "Takeaway"
}
