package handler

import (
	"errors"
	"log"
	"net/http"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
)

type PlateOptionSlotHandler struct {
	slotService *service.PlateOptionSlotService
}

func NewPlateOptionSlotHandler(slotService *service.PlateOptionSlotService) *PlateOptionSlotHandler {
	return &PlateOptionSlotHandler{slotService: slotService}
}

func (h *PlateOptionSlotHandler) GetHandler(c *gin.Context) {
	slots, err := h.slotService.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		log.Printf("Error getting plate option slots: %v", err)
		if errors.Is(err, domainError.ErrProductNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get plate option slots"})
		return
	}
	c.JSON(http.StatusOK, slotList(slots))
}

func (h *PlateOptionSlotHandler) ReplaceHandler(c *gin.Context) {
	var req dto.ReplacePlateOptionSlotsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	slots, err := h.slotService.Replace(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		log.Printf("Error replacing plate option slots: %v", err)
		switch {
		case errors.Is(err, domainError.ErrBundleIngredientNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": domainError.ErrBundleIngredientNotFound.Error()})
		case errors.Is(err, domainError.ErrProductNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		case errors.Is(err, domainError.ErrInvalidPlateOptionSlots):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, domainError.ErrProductNotComposite), errors.Is(err, domainError.ErrIngredientCannotBeSelf):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update plate option slots"})
		}
		return
	}
	c.JSON(http.StatusOK, slotList(slots))
}

func slotList(slots []dto.PlateOptionSlot) dto.PlateOptionSlotListResponse {
	if slots == nil {
		slots = []dto.PlateOptionSlot{}
	}
	return dto.PlateOptionSlotListResponse{Slots: slots}
}
