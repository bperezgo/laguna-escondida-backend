package handler

import (
	"net/http"

	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/buildinfo"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/syncstatus"

	"github.com/gin-gonic/gin"
)

// EdgeStatusResponse describes the runtime state of the node for the frontend. In edge
// mode Online comes from the sync scheduler's last pull outcome, while SyncLagSeconds and
// PendingSyncOps come from this node's unsynced outbox. In cloud mode the node is the
// source of truth, so it reports online with nothing pending. Version is the release tag
// stamped into the binary at build time ("dev" when unstamped).
type EdgeStatusResponse struct {
	Mode           config.Mode `json:"mode"`
	Version        string      `json:"version"`
	Online         bool        `json:"online"`
	SyncLagSeconds int         `json:"sync_lag_seconds"`
	PendingSyncOps int         `json:"pending_sync_ops"`
}

type EdgeHandler struct {
	mode           config.Mode
	organizationID string
	statusService  *service.EdgeStatusService
	tracker        *syncstatus.Tracker
}

// NewEdgeHandler wires the edge status endpoint. statusService and tracker are only
// consulted in edge mode; in cloud mode the cloud reports a trivial always-online status
// and never touches them.
func NewEdgeHandler(mode config.Mode, organizationID string, statusService *service.EdgeStatusService, tracker *syncstatus.Tracker) *EdgeHandler {
	return &EdgeHandler{mode: mode, organizationID: organizationID, statusService: statusService, tracker: tracker}
}

// GetStatusHandler reports the node's run mode, connectivity and sync backlog. In cloud
// mode it short-circuits to online with an empty backlog; in edge mode it reads the
// scheduler's connectivity tracker and this node's unsynced outbox.
func (h *EdgeHandler) GetStatusHandler(c *gin.Context) {
	resp := EdgeStatusResponse{
		Mode:    h.mode,
		Version: buildinfo.Version,
		Online:  true,
	}

	if h.mode == config.ModeEdge {
		resp.Online = h.tracker.Online()

		// Unauthenticated, but it reads only the box's own outbox, so it acts for the box's org.
		health, err := h.statusService.GetSyncHealth(tenant.WithOrg(c.Request.Context(), h.organizationID))
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
