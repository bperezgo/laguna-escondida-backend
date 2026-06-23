package handler

import (
	"net/http"

	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/syncstatus"

	"github.com/gin-gonic/gin"
)

// EdgeStatusResponse describes the runtime state of the node for the frontend. In edge
// mode Online comes from the sync scheduler's last pull outcome, while SyncLagSeconds and
// PendingSyncOps come from this node's unsynced outbox. In cloud mode the node is the
// source of truth, so it reports online with nothing pending.
type EdgeStatusResponse struct {
	Mode           config.Mode `json:"mode"`
	Online         bool        `json:"online"`
	SyncLagSeconds int         `json:"sync_lag_seconds"`
	PendingSyncOps int         `json:"pending_sync_ops"`
}

type EdgeHandler struct {
	cfg           *config.Config
	statusService *service.EdgeStatusService
	tracker       *syncstatus.Tracker
}

// NewEdgeHandler wires the edge status endpoint. statusService and tracker are only
// consulted in edge mode; in cloud mode the cloud reports a trivial always-online status
// and never touches them.
func NewEdgeHandler(cfg *config.Config, statusService *service.EdgeStatusService, tracker *syncstatus.Tracker) *EdgeHandler {
	return &EdgeHandler{cfg: cfg, statusService: statusService, tracker: tracker}
}

// GetStatusHandler reports the node's run mode, connectivity and sync backlog. In cloud
// mode it short-circuits to online with an empty backlog; in edge mode it reads the
// scheduler's connectivity tracker and this node's unsynced outbox.
func (h *EdgeHandler) GetStatusHandler(c *gin.Context) {
	resp := EdgeStatusResponse{
		Mode:   h.cfg.AppMode,
		Online: true,
	}

	if h.cfg.AppMode == config.ModeEdge {
		resp.Online = h.tracker.Online()

		health, err := h.statusService.GetSyncHealth(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "edge_status_error",
				"message": "failed to read sync health",
			})
			return
		}
		resp.SyncLagSeconds = health.SyncLagSeconds
		resp.PendingSyncOps = health.PendingOps
	}

	c.JSON(http.StatusOK, resp)
}
