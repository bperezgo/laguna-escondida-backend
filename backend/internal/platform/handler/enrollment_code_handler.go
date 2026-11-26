package handler

import (
	"errors"
	"net/http"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
)

type EnrollmentCodeHandler struct {
	codes *service.EnrollmentCodeService
}

func NewEnrollmentCodeHandler(codes *service.EnrollmentCodeService) *EnrollmentCodeHandler {
	return &EnrollmentCodeHandler{codes: codes}
}

func (h *EnrollmentCodeHandler) IssueHandler(c *gin.Context) {
	issued, err := h.codes.Issue(c.Request.Context(), c.GetString("user_id"))
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusCreated, issued)
}

func (h *EnrollmentCodeHandler) EnrollHandler(c *gin.Context) {
	var req dto.EnrollWithCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	enrolled, err := h.codes.EnrollWithCode(c.Request.Context(), &req)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusCreated, enrolled)
}

// Unknown, expired and used codes answer alike, so a guess can't tell which.
func (h *EnrollmentCodeHandler) respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domainError.ErrEnrollmentCodeInvalid):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired code"})
	case errors.Is(err, domainError.ErrOrganizationInactive):
		c.JSON(http.StatusForbidden, gin.H{"error": "Organization is not active", "code": "organization_inactive"})
	default:
		respondEnrollError(c, err)
	}
}
