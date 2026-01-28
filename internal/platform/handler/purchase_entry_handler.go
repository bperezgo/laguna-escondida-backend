package handler

import (
	"errors"
	"io"
	"log"
	"net/http"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/fileutil"

	"github.com/gin-gonic/gin"
)

type PurchaseEntryHandler struct {
	purchaseEntryService *service.PurchaseEntryService
}

func NewPurchaseEntryHandler(purchaseEntryService *service.PurchaseEntryService) *PurchaseEntryHandler {
	return &PurchaseEntryHandler{
		purchaseEntryService: purchaseEntryService,
	}
}

func (h *PurchaseEntryHandler) CreatePurchaseEntryHandler(c *gin.Context) {
	var req dto.CreatePurchaseEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error decoding request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	entry, err := h.purchaseEntryService.CreatePurchaseEntry(c.Request.Context(), &req)
	if err != nil {
		log.Printf("Error creating purchase entry: %v", err)

		if errors.Is(err, domainError.ErrSupplierNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Supplier not found"})
			return
		}
		if errors.Is(err, domainError.ErrProductNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "One or more products not found"})
			return
		}
		if errors.Is(err, domainError.ErrPurchaseEntryCreationFailed) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create purchase entry"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, entry)
}

func (h *PurchaseEntryHandler) GetPurchaseEntryByIDHandler(c *gin.Context) {
	entryID := c.Param("id")
	if entryID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Purchase entry ID is required"})
		return
	}

	entry, err := h.purchaseEntryService.GetPurchaseEntryByID(c.Request.Context(), entryID)
	if err != nil {
		log.Printf("Error getting purchase entry: %v", err)

		if errors.Is(err, domainError.ErrPurchaseEntryNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Purchase entry not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, entry)
}

func (h *PurchaseEntryHandler) ListPurchaseEntriesHandler(c *gin.Context) {
	entries, err := h.purchaseEntryService.ListPurchaseEntries(c.Request.Context())
	if err != nil {
		log.Printf("Error listing purchase entries: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list purchase entries"})
		return
	}

	total := len(entries)
	response := dto.PurchaseEntryListResponse{
		Entries: entries,
		Total:   &total,
	}

	c.JSON(http.StatusOK, response)
}

func (h *PurchaseEntryHandler) GetPurchaseEntriesBySupplierHandler(c *gin.Context) {
	supplierID := c.Param("id")
	if supplierID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Supplier ID is required"})
		return
	}

	entries, err := h.purchaseEntryService.GetPurchaseEntriesBySupplier(c.Request.Context(), supplierID)
	if err != nil {
		log.Printf("Error getting supplier purchase entries: %v", err)

		if errors.Is(err, domainError.ErrSupplierNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Supplier not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get supplier purchase entries"})
		return
	}

	total := len(entries)
	response := dto.PurchaseEntryListResponse{
		Entries: entries,
		Total:   &total,
	}

	c.JSON(http.StatusOK, response)
}

func (h *PurchaseEntryHandler) UploadPurchaseEntryDocumentHandler(c *gin.Context) {
	purchaseEntryID := c.Param("id")
	if purchaseEntryID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Purchase entry ID is required"})
		return
	}

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		log.Printf("Error getting file from request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "File is required"})
		return
	}
	defer func() {
		_ = file.Close()
	}()

	fileData, err := io.ReadAll(file)
	if err != nil {
		log.Printf("Error reading file: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file"})
		return
	}

	detectedType := fileutil.DetectFileType(fileData)

	// If ZIP file is detected, extract and upload both PDF and XML
	if detectedType == "zip" {
		extractedFiles, extractErr := fileutil.ValidateAndExtractZip(fileData)
		if extractErr != nil {
			log.Printf("Error extracting ZIP file: %v", extractErr)
			c.JSON(http.StatusBadRequest, gin.H{"error": extractErr.Error()})
			return
		}

		result, uploadErr := h.purchaseEntryService.UploadPurchaseEntryDocuments(c.Request.Context(), purchaseEntryID, extractedFiles.PDFData, extractedFiles.XMLData)
		if uploadErr != nil {
			log.Printf("Error uploading purchase entry documents from ZIP: %v", uploadErr)

			if errors.Is(uploadErr, domainError.ErrPurchaseEntryNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Purchase entry not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload documents"})
			return
		}

		c.JSON(http.StatusOK, result)
		return
	}

	// For single PDF or XML files, require the file_type query parameter
	fileType := c.Query("file_type")
	if fileType != "pdf" && fileType != "xml" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File type must be 'pdf' or 'xml' for single file uploads"})
		return
	}

	result, err := h.purchaseEntryService.UploadPurchaseEntryDocument(c.Request.Context(), purchaseEntryID, fileData, fileType)
	if err != nil {
		log.Printf("Error uploading purchase entry document: %v", err)

		if errors.Is(err, domainError.ErrPurchaseEntryNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Purchase entry not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload document"})
		return
	}

	c.JSON(http.StatusOK, result)
}
