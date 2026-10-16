package handler

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
)

type DeliveryZoneHandler struct {
	zoneService *service.DeliveryZoneService
}

func NewDeliveryZoneHandler(zoneService *service.DeliveryZoneService) *DeliveryZoneHandler {
	return &DeliveryZoneHandler{zoneService: zoneService}
}

func (h *DeliveryZoneHandler) ListHandler(c *gin.Context) {
	zones, err := h.zoneService.List(c.Request.Context())
	if err != nil {
		h.respondError(c, "listing delivery zones", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"delivery_zones": zones})
}

func (h *DeliveryZoneHandler) CreateHandler(c *gin.Context) {
	var req dto.CreateDeliveryZoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	zone, err := h.zoneService.Create(c.Request.Context(), &req)
	if err != nil {
		h.respondError(c, "creating delivery zone", err)
		return
	}
	c.JSON(http.StatusCreated, zone)
}

func (h *DeliveryZoneHandler) UpdateHandler(c *gin.Context) {
	var req dto.UpdateDeliveryZoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	zone, err := h.zoneService.Update(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		h.respondError(c, "updating delivery zone", err)
		return
	}
	c.JSON(http.StatusOK, zone)
}

func (h *DeliveryZoneHandler) DeleteHandler(c *gin.Context) {
	if err := h.zoneService.Delete(c.Request.Context(), c.Param("id")); err != nil {
		h.respondError(c, "deleting delivery zone", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Delivery zone deleted successfully"})
}

func (h *DeliveryZoneHandler) CheckHandler(c *gin.Context) {
	lat, latErr := strconv.ParseFloat(c.Query("lat"), 64)
	lng, lngErr := strconv.ParseFloat(c.Query("lng"), 64)
	if latErr != nil || lngErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "lat and lng query parameters must be decimal degrees"})
		return
	}
	result, err := h.zoneService.Check(c.Request.Context(), lat, lng)
	if err != nil {
		h.respondError(c, "checking delivery location", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *DeliveryZoneHandler) respondError(c *gin.Context, action string, err error) {
	log.Printf("Error %s: %v", action, err)
	if errors.Is(err, domainError.ErrDeliveryZoneNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Delivery zone not found"})
		return
	}
	RespondError(c, err)
}
