package handler

import (
	"errors"
	"log"
	"net/http"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
)

type DeliveryHandler struct {
	deliveries *service.DeliveryService
}

func NewDeliveryHandler(deliveries *service.DeliveryService) *DeliveryHandler {
	return &DeliveryHandler{deliveries: deliveries}
}

func (h *DeliveryHandler) ListHandler(c *gin.Context) {
	from, _ := businessDayRange()
	filter := dto.DeliveryListFilter{Status: dto.DeliveryStatus(c.Query("status")), OrderType: dto.OrderType(c.Query("order_type"))}
	result, err := h.deliveries.List(c.Request.Context(), filter, from)
	if err != nil {
		respondDeliveryError(c, "listing deliveries", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *DeliveryHandler) HistoryHandler(c *gin.Context) {
	result, err := h.deliveries.History(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondDeliveryError(c, "reading delivery history", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *DeliveryHandler) CouriersHandler(c *gin.Context) {
	result, err := h.deliveries.Couriers(c.Request.Context())
	if err != nil {
		respondDeliveryError(c, "listing couriers", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *DeliveryHandler) MineHandler(c *gin.Context) {
	result, err := h.deliveries.Mine(c.Request.Context(), c.GetString("user_id"))
	if err != nil {
		respondDeliveryError(c, "listing courier deliveries", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *DeliveryHandler) DispatchHandler(c *gin.Context) {
	var req dto.DispatchDeliveryRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}
	}
	result, err := h.deliveries.Dispatch(c.Request.Context(), c.Param("id"), deliveryActor(c), req)
	h.respondChange(c, "dispatching delivery", result, err)
}

func (h *DeliveryHandler) DeliverHandler(c *gin.Context) {
	result, err := h.deliveries.Deliver(c.Request.Context(), c.Param("id"), deliveryActor(c))
	h.respondChange(c, "delivering order", result, err)
}

func (h *DeliveryHandler) FailHandler(c *gin.Context) {
	var req dto.FailDeliveryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	result, err := h.deliveries.Fail(c.Request.Context(), c.Param("id"), deliveryActor(c), req)
	h.respondChange(c, "failing delivery", result, err)
}

func (h *DeliveryHandler) respondChange(c *gin.Context, action string, result *dto.DeliveryOrder, err error) {
	if err != nil {
		respondDeliveryError(c, action, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// deliveryActor: the route lets manage or carry through; without manage the caller is a courier.
func deliveryActor(c *gin.Context) service.DeliveryActor {
	return service.DeliveryActor{
		UserID: c.GetString("user_id"),
		Manage: permissions.HasPermission(callerRoles(c), permissions.DeliveriesManage),
	}
}

func respondDeliveryError(c *gin.Context, action string, err error) {
	var validation *domainError.ValidationError
	var transition *domainError.DeliveryTransitionError
	switch {
	case errors.As(err, &validation):
		c.JSON(http.StatusBadRequest, gin.H{"error": validation.Error(), "code": "VALIDATION_ERROR", "field": validation.Field})
	case errors.As(err, &transition):
		var current *string
		if transition.Current != "" {
			current = &transition.Current
		}
		c.JSON(http.StatusConflict, gin.H{"error": transition.Error(), "code": "INVALID_DELIVERY_TRANSITION", "current_status": current})
	case errors.Is(err, domainError.ErrDeliveryForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": "You may not make that delivery change", "code": "DELIVERY_FORBIDDEN"})
	case errors.Is(err, domainError.ErrDeliveryOrderNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found", "code": "ORDER_NOT_FOUND"})
	default:
		log.Printf("Error %s: %v", action, err)
		RespondError(c, err)
	}
}
