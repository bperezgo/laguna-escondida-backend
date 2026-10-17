package handler

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
)

type EdgeUpdateResponse struct {
	Enabled    bool                      `json:"enabled"`
	Installed  *dto.EdgeInstalledRelease `json:"installed"`
	Available  *dto.EdgeUpdateAvailable  `json:"available"`
	Requested  *dto.EdgeUpdateRequest    `json:"requested"`
	InProgress *dto.EdgeUpdateProgress   `json:"in_progress"`
	LastResult *dto.EdgeUpdateResult     `json:"last_result"`
}

type requestEdgeUpdateBody struct {
	Version string `json:"version"`
}

type EdgeUpdateHandler struct {
	updateService *service.EdgeUpdateService
}

// NewEdgeUpdateHandler takes a nil service when LAGUNA_STATE_DIR is unset: the feature is off.
func NewEdgeUpdateHandler(updateService *service.EdgeUpdateService) *EdgeUpdateHandler {
	return &EdgeUpdateHandler{updateService: updateService}
}

func (h *EdgeUpdateHandler) GetUpdateHandler(c *gin.Context) {
	if h.updateService == nil {
		c.JSON(http.StatusOK, gin.H{"enabled": false})
		return
	}

	state := h.updateService.GetState(c.Request.Context())
	c.JSON(http.StatusOK, EdgeUpdateResponse{
		Enabled:    true,
		Installed:  state.Installed,
		Available:  state.Available,
		Requested:  state.Requested,
		InProgress: state.InProgress,
		LastResult: state.LastResult,
	})
}

func (h *EdgeUpdateHandler) RequestUpdateHandler(c *gin.Context) {
	if h.updateService == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "edge_update_disabled"})
		return
	}

	var body requestEdgeUpdateBody
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Version) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "version is required"})
		return
	}

	user, ok := currentUser(c)
	if !ok {
		return
	}

	req, err := h.updateService.RequestUpdate(c.Request.Context(), body.Version, user.ID)
	if err != nil {
		switch {
		case errors.Is(err, domainError.ErrNoStagedUpdate):
			c.JSON(http.StatusConflict, gin.H{"error": "no_staged_update", "message": "no update is staged"})
		case errors.Is(err, domainError.ErrUpdateVersionMismatch):
			c.JSON(http.StatusConflict, gin.H{"error": "version_mismatch", "message": "version is not the staged update"})
		case errors.Is(err, domainError.ErrUpdateAlreadyPending):
			c.JSON(http.StatusConflict, gin.H{"error": "update_pending", "message": "an update is already requested or applying"})
		default:
			log.Printf("Error requesting edge update: %v", err)
			RespondError(c, err)
		}
		return
	}

	c.JSON(http.StatusAccepted, req)
}
