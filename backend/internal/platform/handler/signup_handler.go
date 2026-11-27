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

type SignupHandler struct {
	signup *service.SignupService
	logger *slog.Logger
}

func NewSignupHandler(signup *service.SignupService, logger *slog.Logger) *SignupHandler {
	return &SignupHandler{signup: signup, logger: logger}
}

var googleSignInFailedBody = gin.H{"error": "Google sign-in failed"}

func (h *SignupHandler) GoogleSignInHandler(c *gin.Context) {
	var req dto.GoogleSignInRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.IDToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id_token is required"})
		return
	}
	resp, err := h.signup.SignInWithGoogle(c.Request.Context(), req.IDToken)
	if err != nil {
		h.respondGoogleError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *SignupHandler) GoogleSignupHandler(c *gin.Context) {
	var req dto.GoogleSignupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if req.IDToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id_token is required", "field": "id_token"})
		return
	}
	result, err := h.signup.SignUpWithGoogle(c.Request.Context(), &req)
	if err != nil {
		var validationErr *domainError.OrganizationValidationError
		if errors.As(err, &validationErr) {
			c.JSON(http.StatusBadRequest, gin.H{"error": validationErr.Error(), "field": validationErr.Field})
			return
		}
		h.respondGoogleError(c, err)
		return
	}
	if result.Created == nil {
		c.JSON(http.StatusOK, result.SignIn)
		return
	}
	h.logger.InfoContext(c.Request.Context(), "organization signed up", slog.String("plan", req.Plan))
	c.JSON(http.StatusCreated, result.Created)
}

// Opaque, so the response can't tell which emails or organizations exist; the reason goes to the log.
func (h *SignupHandler) respondGoogleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domainError.ErrTrialExpired):
		respondTrialExpired(c)
	case errors.Is(err, domainError.ErrOrganizationPending):
		respondPendingApproval(c, err)
	case errors.Is(err, domainError.ErrGoogleSignInFailed), errors.Is(err, domainError.ErrGoogleTokenInvalid):
		h.logger.WarnContext(c.Request.Context(), "google sign-in rejected", slog.String("reason", err.Error()))
		c.JSON(http.StatusUnauthorized, googleSignInFailedBody)
	default:
		h.logger.ErrorContext(c.Request.Context(), "google sign-in failed", slog.String("error", err.Error()))
		RespondError(c, err)
	}
}
