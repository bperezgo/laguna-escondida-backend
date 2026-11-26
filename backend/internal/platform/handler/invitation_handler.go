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

type InvitationHandler struct {
	invitations *service.InvitationService
	logger      *slog.Logger
}

func NewInvitationHandler(invitations *service.InvitationService, logger *slog.Logger) *InvitationHandler {
	return &InvitationHandler{invitations: invitations, logger: logger}
}

func (h *InvitationHandler) CreateHandler(c *gin.Context) {
	var req dto.CreateInvitationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	roleIDs, _ := c.Get("role_ids")
	inviterRoles, _ := roleIDs.([]int)
	created, err := h.invitations.Create(c.Request.Context(), dto.InviterPrincipal{UserID: c.GetString("user_id"), RoleIDs: inviterRoles}, &req)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusCreated, created)
}

func (h *InvitationHandler) ListHandler(c *gin.Context) {
	list, err := h.invitations.List(c.Request.Context())
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *InvitationHandler) NewLinkHandler(c *gin.Context) {
	renewed, err := h.invitations.NewLink(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, renewed)
}

func (h *InvitationHandler) RevokeHandler(c *gin.Context) {
	if err := h.invitations.Revoke(c.Request.Context(), c.Param("id")); err != nil {
		h.respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *InvitationHandler) LookupHandler(c *gin.Context) {
	invitation, err := h.invitations.Lookup(c.Request.Context(), c.Param("token"))
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, invitation)
}

func (h *InvitationHandler) AcceptHandler(c *gin.Context) {
	var req dto.AcceptInvitationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	resp, err := h.invitations.Accept(c.Request.Context(), c.Param("token"), &req)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

func (h *InvitationHandler) respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domainError.ErrInvitationInvalidOrExpired):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired link", "code": "invalid_or_expired_link"})
	case errors.Is(err, domainError.ErrOrganizationInactive):
		c.JSON(http.StatusForbidden, gin.H{"error": "Organization is not active", "code": "organization_inactive"})
	case errors.Is(err, domainError.ErrInvitationAdminOnly):
		c.JSON(http.StatusForbidden, gin.H{"error": "Only an admin can invite an admin"})
	case errors.Is(err, domainError.ErrInvitationNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Invitation not found"})
	case errors.Is(err, domainError.ErrUserEmailTaken):
		c.JSON(http.StatusConflict, gin.H{"error": "Email is already in use"})
	case errors.Is(err, domainError.ErrInvitationPending):
		c.JSON(http.StatusConflict, gin.H{"error": "This email already has a pending invitation; get a new link for it instead"})
	case errors.Is(err, domainError.ErrInvitationEmailMismatch):
		c.JSON(http.StatusBadRequest, gin.H{"error": "The Google account's email is not the invited one"})
	case errors.Is(err, domainError.ErrInvitationRoleNotAllowed), errors.Is(err, domainError.ErrInvalidRoleIDs),
		errors.Is(err, domainError.ErrEmailRequired), errors.Is(err, domainError.ErrInvitationNameInvalid),
		errors.Is(err, domainError.ErrInvitationCredentials), errors.Is(err, domainError.ErrGoogleNotConfigured),
		errors.Is(err, domainError.ErrRoleNotFound):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		h.logger.ErrorContext(c.Request.Context(), "invitation request failed", slog.String("error", err.Error()))
		RespondError(c, err)
	}
}
