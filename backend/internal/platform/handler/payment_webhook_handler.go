package handler

import (
	"io"
	"log/slog"
	"net/http"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// wompiEventMaxBytes is far above any transaction event; a bigger body is not Wompi's.
const wompiEventMaxBytes = 1 << 20

type PaymentWebhookHandler struct {
	webhooks *service.PaymentWebhookService
	logger   *slog.Logger
}

func NewPaymentWebhookHandler(webhooks *service.PaymentWebhookService, logger *slog.Logger) *PaymentWebhookHandler {
	return &PaymentWebhookHandler{webhooks: webhooks, logger: logger}
}

// WompiHandler is public: the event's checksum, not a credential, proves it is Wompi's. Anything
// but 200 makes Wompi send the event again, so only a verified event that failed to apply is a 500.
func (h *PaymentWebhookHandler) WompiHandler(c *gin.Context) {
	orgID := c.Param("organization_id")
	if _, err := uuid.Parse(orgID); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid event"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, wompiEventMaxBytes))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unreadable body"})
		return
	}
	outcome, err := h.webhooks.Handle(c.Request.Context(), orgID, body)
	if err != nil {
		h.logger.ErrorContext(c.Request.Context(), "Wompi event not applied; Wompi will retry", slog.String("organization_id", orgID), slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "event not applied"})
		return
	}
	if outcome == dto.PaymentWebhookRefused {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid event"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
