package handler

import (
	"io"
	"log/slog"
	"net/http"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
)

type BillingWebhookHandler struct {
	webhooks *service.BillingWebhookService
	logger   *slog.Logger
}

func NewBillingWebhookHandler(webhooks *service.BillingWebhookService, logger *slog.Logger) *BillingWebhookHandler {
	return &BillingWebhookHandler{webhooks: webhooks, logger: logger}
}

// WompiHandler is public: the event's checksum, not a credential, proves it is Wompi's. Anything
// but 200 makes Wompi send the event again, so only a verified event that failed to apply is a 500.
func (h *BillingWebhookHandler) WompiHandler(c *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, wompiEventMaxBytes))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unreadable body"})
		return
	}
	outcome, err := h.webhooks.Handle(c.Request.Context(), body)
	if err != nil {
		h.logger.ErrorContext(c.Request.Context(), "Wompi billing event not applied; Wompi will retry", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "event not applied"})
		return
	}
	if outcome == dto.PaymentWebhookRefused {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid event"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
