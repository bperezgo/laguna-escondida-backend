package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
)

// BillingSetupHandler serves the routes a setup token opens. SetupAuthMiddleware puts org_id and user_id in the context.
type BillingSetupHandler struct {
	setup  *service.BillingSetupService
	logger *slog.Logger
}

func NewBillingSetupHandler(setup *service.BillingSetupService, logger *slog.Logger) *BillingSetupHandler {
	return &BillingSetupHandler{setup: setup, logger: logger}
}

func (h *BillingSetupHandler) StateHandler(c *gin.Context) {
	state, err := h.setup.State(c.Request.Context(), c.GetString("org_id"))
	if err != nil {
		h.respondError(c, "reading billing setup", err)
		return
	}
	c.JSON(http.StatusOK, state)
}

func (h *BillingSetupHandler) SelectionHandler(c *gin.Context) {
	var req dto.BillingSelectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	selection, err := h.setup.Select(c.Request.Context(), c.GetString("org_id"), &req)
	if err != nil {
		h.respondError(c, "choosing plan and period", err)
		return
	}
	c.JSON(http.StatusOK, selection)
}

// SaveCardHandler answers a sign-in body: leaving a card is what signs the person in.
func (h *BillingSetupHandler) SaveCardHandler(c *gin.Context) {
	var req dto.SaveCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	signIn, err := h.setup.SaveCard(c.Request.Context(), c.GetString("org_id"), c.GetString("user_id"), &req)
	if err != nil {
		h.respondError(c, "saving card", err)
		return
	}
	h.logger.InfoContext(c.Request.Context(), "trial started by a saved card", slog.String("organization_id", c.GetString("org_id")))
	c.JSON(http.StatusOK, signIn)
}

// The reason goes to the access log through c.Error; a provider's answer never reaches the body.
func (h *BillingSetupHandler) respondError(c *gin.Context, action string, err error) {
	var validationErr *domainError.BillingValidationError
	switch {
	case errors.As(err, &validationErr):
		c.JSON(http.StatusBadRequest, gin.H{"error": validationErr.Error(), "field": validationErr.Field})
	case errors.Is(err, domainError.ErrCardRefused):
		_ = c.Error(err)
		c.JSON(http.StatusPaymentRequired, gin.H{"error": "Card refused", "code": "card_refused"})
	case errors.Is(err, domainError.ErrPaymentProviderUnavailable):
		_ = c.Error(err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Payment provider unavailable", "code": "payment_provider_unavailable"})
	case errors.Is(err, domainError.ErrOrganizationNotPending):
		c.JSON(http.StatusConflict, notPendingBody)
	case errors.Is(err, domainError.ErrCardEntryOff):
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
	default:
		h.logger.ErrorContext(c.Request.Context(), "billing setup failed", slog.String("action", action), slog.String("error", err.Error()))
		RespondError(c, err)
	}
}
