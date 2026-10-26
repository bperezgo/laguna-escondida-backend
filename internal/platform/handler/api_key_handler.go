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

type APIKeyHandler struct {
	apiKeys *service.APIKeyService
}

func NewAPIKeyHandler(apiKeys *service.APIKeyService) *APIKeyHandler {
	return &APIKeyHandler{apiKeys: apiKeys}
}

func (h *APIKeyHandler) IssueHandler(c *gin.Context) {
	var req dto.IssueAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	issued, err := h.apiKeys.Issue(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		h.respondError(c, "issuing api key", err)
		return
	}
	c.JSON(http.StatusCreated, issued)
}

func (h *APIKeyHandler) ListHandler(c *gin.Context) {
	keys, err := h.apiKeys.List(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.respondError(c, "listing api keys", err)
		return
	}
	c.JSON(http.StatusOK, dto.APIKeyListResponse{APIKeys: keys})
}

func (h *APIKeyHandler) RevokeHandler(c *gin.Context) {
	if err := h.apiKeys.Revoke(c.Request.Context(), c.Param("id"), c.Param("keyId")); err != nil {
		h.respondError(c, "revoking api key", err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *APIKeyHandler) respondError(c *gin.Context, action string, err error) {
	switch {
	case errors.Is(err, domainError.ErrOrganizationNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
	case errors.Is(err, domainError.ErrAPIKeyNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
	case errors.Is(err, domainError.ErrAPIKeyUserNotEligible):
		c.JSON(http.StatusBadRequest, gin.H{"error": domainError.ErrAPIKeyUserNotEligible.Error(), "field": "user_id"})
	case errors.Is(err, domainError.ErrAPIKeyNameRequired), errors.Is(err, domainError.ErrAPIKeyNameTooLong):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "field": "name"})
	default:
		log.Printf("Error %s: %v", action, err)
		RespondError(c, err)
	}
}
