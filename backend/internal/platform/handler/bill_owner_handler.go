package handler

import (
	"errors"
	"log"
	"net/http"

	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
)

type BillOwnerHandler struct {
	billOwnerService *service.BillOwnerService
}

func NewBillOwnerHandler(billOwnerService *service.BillOwnerService) *BillOwnerHandler {
	return &BillOwnerHandler{
		billOwnerService: billOwnerService,
	}
}

func (h *BillOwnerHandler) GetByIDHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID is required"})
		return
	}

	billOwner, err := h.billOwnerService.GetByID(c.Request.Context(), id)
	if err != nil {
		log.Printf("Error getting bill owner by ID: %v", err)

		if errors.Is(err, domainError.ErrBillOwnerNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bill owner not found"})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, billOwner)
}
