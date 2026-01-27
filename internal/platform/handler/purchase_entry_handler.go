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
