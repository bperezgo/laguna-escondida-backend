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

type SupportDocumentHandler struct {
	supportDocumentService *service.SupportDocumentService
}

func NewSupportDocumentHandler(supportDocumentService *service.SupportDocumentService) *SupportDocumentHandler {
	return &SupportDocumentHandler{
		supportDocumentService: supportDocumentService,
	}
}

func (h *SupportDocumentHandler) CreateSupportDocumentHandler(c *gin.Context) {
	var doc dto.SupportDocument
	if err := c.ShouldBindJSON(&doc); err != nil {
		log.Printf("Error decoding request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if doc.Provider.DocumentNumber == "" || doc.Provider.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provider document number and name are required"})
		return
	}

	if err := h.supportDocumentService.CreateSupportDocument(c.Request.Context(), &doc); err != nil {
		log.Printf("Error creating support document: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create support document"})
		return
	}

	c.JSON(http.StatusCreated, struct {
		Message string `json:"message"`
	}{
		Message: "Support document created successfully",
	})
}

func (h *SupportDocumentHandler) ListSupportDocumentsHandler(c *gin.Context) {
	var req dto.ListSupportDocumentsRequest

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
		parsedTime, err := parseStartDate(createdAtStartStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid created_at_start format. Use YYYY-MM-DD or RFC3339"})
			return
		}
		req.CreatedAtStart = &parsedTime
	}

	if createdAtEndStr := c.Query("created_at_end"); createdAtEndStr != "" {
		parsedTime, err := parseEndDate(createdAtEndStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid created_at_end format. Use YYYY-MM-DD or RFC3339"})
			return
		}
		req.CreatedAtEnd = &parsedTime
	}

	if providerDoc := c.Query("provider_document_number"); providerDoc != "" {
		req.ProviderDocumentNumber = &providerDoc
	}

	response, err := h.supportDocumentService.ListSupportDocuments(c.Request.Context(), &req)
	if err != nil {
		log.Printf("Error listing support documents: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list support documents"})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *SupportDocumentHandler) ExportSupportDocumentsCSVHandler(c *gin.Context) {
	var req dto.ExportSupportDocumentsRequest

	if createdAtStartStr := c.Query("created_at_start"); createdAtStartStr != "" {
		parsedTime, err := parseStartDate(createdAtStartStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid created_at_start format. Use YYYY-MM-DD or RFC3339"})
			return
		}
		req.CreatedAtStart = &parsedTime
	}

	if createdAtEndStr := c.Query("created_at_end"); createdAtEndStr != "" {
		parsedTime, err := parseEndDate(createdAtEndStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid created_at_end format. Use YYYY-MM-DD or RFC3339"})
			return
		}
		req.CreatedAtEnd = &parsedTime
	}

	if providerDoc := c.Query("provider_document_number"); providerDoc != "" {
		req.ProviderDocumentNumber = &providerDoc
	}

	csvData, err := h.supportDocumentService.ExportSupportDocumentsCSV(c.Request.Context(), &req)
	if err != nil {
		log.Printf("Error exporting support documents to CSV: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to export support documents"})
		return
	}

	filename := fmt.Sprintf("documentos_soporte_%s.csv", time.Now().Format("2006-01-02"))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Data(http.StatusOK, "text/csv; charset=utf-8", csvData)
}

func (h *SupportDocumentHandler) UpdateMissingSupportDocumentURLsHandler(c *gin.Context) {
	response, err := h.supportDocumentService.UpdateMissingDocumentURLs(c.Request.Context())
	if err != nil {
		log.Printf("Error updating missing support document URLs: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update missing support document URLs"})
		return
	}

	c.JSON(http.StatusOK, response)
}
