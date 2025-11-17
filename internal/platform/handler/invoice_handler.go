package handler

import (
	"log"
	"net/http"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
)

type InvoiceHandler struct {
	invoiceService *service.InvoiceService
}

func NewInvoiceHandler(invoiceService *service.InvoiceService) *InvoiceHandler {
	return &InvoiceHandler{
		invoiceService: invoiceService,
	}
}

func (h *InvoiceHandler) CreateElectronicInvoiceHandler(c *gin.Context) {
	var invoice dto.ElectronicInvoice
	if err := c.ShouldBindJSON(&invoice); err != nil {
		log.Printf("Error decoding request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := h.invoiceService.CreateElectronicInvoice(c.Request.Context(), &invoice); err != nil {
		log.Printf("Error creating electronic invoice: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create electronic invoice"})
		return
	}

	c.Status(http.StatusCreated)
}
