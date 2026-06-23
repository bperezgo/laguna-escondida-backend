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

type DeviceHandler struct {
	printService *service.PrintService
}

func NewDeviceHandler(printService *service.PrintService) *DeviceHandler {
	return &DeviceHandler{printService: printService}
}

// PrintTicketHandler renders and prints the receipt ("cuenta") for an open bill.
// The client sends only the bill id; the edge node loads the authoritative bill
// and drives the printer. Errors are actionable so the frontend can fall back to
// browser printing (409) or surface a not-found (404).
func (h *DeviceHandler) PrintTicketHandler(c *gin.Context) {
	var req dto.PrintTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error decoding print request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid request body"})
		return
	}

	if err := h.printService.PrintTicket(c.Request.Context(), &req); err != nil {
		log.Printf("Error printing ticket: %v", err)

		if errors.Is(err, domainError.ErrOpenBillNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "bill_not_found"})
			return
		}
		if errors.Is(err, domainError.ErrTicketPrintFailed) {
			c.JSON(http.StatusConflict, gin.H{
				"error":   "printer_unavailable",
				"message": "the printer is unavailable (offline, out of paper, or no device)",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"printed": true})
}
