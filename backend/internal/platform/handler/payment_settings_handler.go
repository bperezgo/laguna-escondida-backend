package handler

import (
	"net/http"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
)

type PaymentSettingsHandler struct {
	settings *service.PaymentSettingsService
}

func NewPaymentSettingsHandler(settings *service.PaymentSettingsService) *PaymentSettingsHandler {
	return &PaymentSettingsHandler{settings: settings}
}

func (h *PaymentSettingsHandler) GetHandler(c *gin.Context) {
	settings, err := h.settings.Get(c.Request.Context())
	if err != nil {
		respondIntegrationError(c, "reading payment settings", err)
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (h *PaymentSettingsHandler) PutHandler(c *gin.Context) {
	var req dto.PaymentSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	settings, err := h.settings.Save(c.Request.Context(), req)
	if err != nil {
		respondIntegrationError(c, "saving payment settings", err)
		return
	}
	c.JSON(http.StatusOK, settings)
}
