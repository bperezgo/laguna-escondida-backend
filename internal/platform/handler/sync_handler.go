package handler

import (
	"net/http"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
)

type SyncHandler struct {
	syncService      *service.SyncService
	referenceService *service.SyncReferenceService
}

func NewSyncHandler(syncService *service.SyncService, referenceService *service.SyncReferenceService) *SyncHandler {
	return &SyncHandler{syncService: syncService, referenceService: referenceService}
}

// PushHandler applies a batch of ops pushed by an edge node and returns the acks.
func (h *SyncHandler) PushHandler(c *gin.Context) {
	var req dto.SyncPushRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	resp, err := h.syncService.ApplyPush(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// PullHandler returns the reference rows that changed after the `since` cursor (RFC3339Nano).
// An empty/absent `since` means "from the beginning of time" — the edge's first pull.
func (h *SyncHandler) PullHandler(c *gin.Context) {
	var since time.Time
	if raw := c.Query("since"); raw != "" {
		parsed, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid since timestamp"})
			return
		}
		since = parsed
	}

	resp, err := h.referenceService.ChangesSince(c.Request.Context(), since)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}
