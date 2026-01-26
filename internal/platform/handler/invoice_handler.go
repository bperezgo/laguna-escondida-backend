package handler

import (
	"fmt"
	"log"
	"net/http"
	"time"

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

	c.JSON(http.StatusCreated, struct {
		Message string `json:"message"`
	}{
		Message: "Electronic invoice created successfully",
	})
}

func (h *InvoiceHandler) ListInvoicesHandler(c *gin.Context) {
	var req dto.ListInvoicesRequest

	if pageStr := c.Query("page"); pageStr != "" {
		var page int
		if _, err := fmt.Sscanf(pageStr, "%d", &page); err == nil {
			req.Page = page
		}
	}

	if pageSizeStr := c.Query("page_size"); pageSizeStr != "" {
		var pageSize int
		if _, err := fmt.Sscanf(pageSizeStr, "%d", &pageSize); err == nil {
			req.PageSize = pageSize
		}
	}

	if createdAtStartStr := c.Query("created_at_start"); createdAtStartStr != "" {
		if parsedTime, err := time.Parse(time.RFC3339, createdAtStartStr); err == nil {
			req.CreatedAtStart = &parsedTime
		}
	}

	if createdAtEndStr := c.Query("created_at_end"); createdAtEndStr != "" {
		if parsedTime, err := time.Parse(time.RFC3339, createdAtEndStr); err == nil {
			req.CreatedAtEnd = &parsedTime
		}
	}

	if nationalID := c.Query("national_identification"); nationalID != "" {
		req.NationalIdentification = &nationalID
	}

	response, err := h.invoiceService.ListInvoices(c.Request.Context(), &req)
	if err != nil {
		log.Printf("Error listing invoices: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list invoices"})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *InvoiceHandler) UpdateMissingDocumentURLsHandler(c *gin.Context) {
	response, err := h.invoiceService.UpdateMissingDocumentURLs(c.Request.Context())
	if err != nil {
		log.Printf("Error updating missing document URLs: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update missing document URLs"})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *InvoiceHandler) ExportInvoicesCSVHandler(c *gin.Context) {
	var req dto.ExportInvoicesRequest

	if createdAtStartStr := c.Query("created_at_start"); createdAtStartStr != "" {
		if parsedTime, err := time.Parse(time.RFC3339, createdAtStartStr); err == nil {
			req.CreatedAtStart = &parsedTime
		}
	}

	if createdAtEndStr := c.Query("created_at_end"); createdAtEndStr != "" {
		if parsedTime, err := time.Parse(time.RFC3339, createdAtEndStr); err == nil {
			req.CreatedAtEnd = &parsedTime
		}
	}

	if nationalID := c.Query("national_identification"); nationalID != "" {
		req.NationalIdentification = &nationalID
	}

	csvData, err := h.invoiceService.ExportInvoicesCSV(c.Request.Context(), &req)
	if err != nil {
		log.Printf("Error exporting invoices to CSV: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to export invoices"})
		return
	}

	filename := fmt.Sprintf("facturas_%s.csv", time.Now().Format("2006-01-02"))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Data(http.StatusOK, "text/csv; charset=utf-8", csvData)
}
