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

// BillingHandler serves an admin's subscription routes, behind a sign-in token and billing:manage,
// and the recovery routes, behind a setup token. Both put org_id and user_id in the context.
type BillingHandler struct {
	billing *service.BillingService
	logger  *slog.Logger
}

func NewBillingHandler(billing *service.BillingService, logger *slog.Logger) *BillingHandler {
	return &BillingHandler{billing: billing, logger: logger}
}

func (h *BillingHandler) ReadHandler(c *gin.Context) {
	subscription, err := h.billing.Read(c.Request.Context(), c.GetString("org_id"))
	if err != nil {
		h.respondError(c, "reading subscription", err)
		return
	}
	c.JSON(http.StatusOK, subscription)
}

func (h *BillingHandler) CardSetupHandler(c *gin.Context) {
	setup, err := h.billing.CardSetup(c.Request.Context())
	if err != nil {
		h.respondError(c, "reading card setup", err)
		return
	}
	c.JSON(http.StatusOK, setup)
}

func (h *BillingHandler) ChangeCardHandler(c *gin.Context) {
	var req dto.SaveCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	subscription, err := h.billing.ChangeCard(c.Request.Context(), c.GetString("org_id"), c.GetString("user_id"), &req)
	if err != nil {
		h.respondError(c, "changing card", err)
		return
	}
	h.logger.InfoContext(c.Request.Context(), "saved card changed", slog.String("organization_id", c.GetString("org_id")))
	c.JSON(http.StatusOK, subscription)
}

func (h *BillingHandler) CancelHandler(c *gin.Context) {
	subscription, err := h.billing.Cancel(c.Request.Context(), c.GetString("org_id"))
	if err != nil {
		h.respondError(c, "cancelling subscription", err)
		return
	}
	h.logger.InfoContext(c.Request.Context(), "subscription cancelled", slog.String("organization_id", c.GetString("org_id")))
	c.JSON(http.StatusOK, subscription)
}

func (h *BillingHandler) ResumeHandler(c *gin.Context) {
	subscription, err := h.billing.Resume(c.Request.Context(), c.GetString("org_id"))
	if err != nil {
		h.respondError(c, "resuming subscription", err)
		return
	}
	h.logger.InfoContext(c.Request.Context(), "subscription resumed", slog.String("organization_id", c.GetString("org_id")))
	c.JSON(http.StatusOK, subscription)
}

// RecoveryStateHandler answers a sign-in body once the recovery payment is approved: the page's
// polling is what signs the person in.
func (h *BillingHandler) RecoveryStateHandler(c *gin.Context) {
	state, signIn, err := h.billing.RecoveryState(c.Request.Context(), c.GetString("org_id"), c.GetString("user_id"))
	if err != nil {
		h.respondError(c, "reading recovery state", err)
		return
	}
	if signIn != nil {
		h.logger.InfoContext(c.Request.Context(), "organization recovered by a payment", slog.String("organization_id", c.GetString("org_id")))
		c.JSON(http.StatusOK, signIn)
		return
	}
	c.JSON(http.StatusOK, state)
}

func (h *BillingHandler) RecoverySelectionHandler(c *gin.Context) {
	var req dto.BillingSelectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	selection, err := h.billing.RecoverySelect(c.Request.Context(), c.GetString("org_id"), &req)
	if err != nil {
		h.respondError(c, "choosing plan and period", err)
		return
	}
	c.JSON(http.StatusOK, selection)
}

// RecoveryPayHandler takes an optional new card; an empty body pays with the saved one.
func (h *BillingHandler) RecoveryPayHandler(c *gin.Context) {
	var req dto.RecoveryPayRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}
	}
	paid, err := h.billing.RecoveryPay(c.Request.Context(), c.GetString("org_id"), c.GetString("user_id"), &req)
	if err != nil {
		h.respondError(c, "paying recovery", err)
		return
	}
	c.JSON(http.StatusAccepted, paid)
}

// The reason goes to the access log through c.Error; a provider's answer never reaches the body.
func (h *BillingHandler) respondError(c *gin.Context, action string, err error) {
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
	case errors.Is(err, domainError.ErrChargePending):
		c.JSON(http.StatusConflict, gin.H{"error": "A charge is pending", "code": "charge_pending"})
	case errors.Is(err, domainError.ErrNoSubscription):
		c.JSON(http.StatusConflict, gin.H{"error": "There is no saved card", "code": "no_subscription"})
	case errors.Is(err, domainError.ErrSubscriptionLapsed):
		c.JSON(http.StatusConflict, gin.H{"error": "The subscription has lapsed", "code": "subscription_lapsed"})
	case errors.Is(err, domainError.ErrOrganizationNotStopped):
		c.JSON(http.StatusConflict, notStoppedBody)
	case errors.Is(err, domainError.ErrCardEntryOff), errors.Is(err, domainError.ErrChargingOff):
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
	default:
		h.logger.ErrorContext(c.Request.Context(), "billing failed", slog.String("action", action), slog.String("error", err.Error()))
		RespondError(c, err)
	}
}
