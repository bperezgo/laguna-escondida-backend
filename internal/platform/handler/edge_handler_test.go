package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/syncstatus"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const edgeHandlerNodeID = "33333333-3333-3333-3333-333333333333"

func serveEdgeStatus(t *testing.T, h *EdgeHandler) EdgeStatusResponse {
	t.Helper()
	router := gin.New()
	router.GET("/api/edge/status", h.GetStatusHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/edge/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp EdgeStatusResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

// Cloud mode is the source of truth: always online, nothing pending. It never consults
// the status service or tracker, so they may be nil.
func TestEdgeHandler_GetStatus_CloudModeIsTriviallyOnline(t *testing.T) {
	h := NewEdgeHandler(&config.Config{AppMode: config.ModeCloud}, nil, nil)

	resp := serveEdgeStatus(t, h)

	assert.Equal(t, config.ModeCloud, resp.Mode)
	assert.True(t, resp.Online)
	assert.Equal(t, 0, resp.SyncLagSeconds)
	assert.Equal(t, 0, resp.PendingSyncOps)
}

// Edge mode before any successful sync reports offline and surfaces the unsynced backlog.
func TestEdgeHandler_GetStatus_EdgeModeOfflineWithBacklog(t *testing.T) {
	outbox := mocks.NewMockSyncOutboxRepository(t)
	oldest := time.Now().Add(-30 * time.Second)
	outbox.EXPECT().
		PendingStats(mock.Anything, edgeHandlerNodeID).
		Return(&dto.SyncOutboxPendingStats{PendingCount: 2, OldestPendingAt: &oldest}, nil)

	statusService := service.NewEdgeStatusService(outbox, dto.SyncIdentity{NodeID: edgeHandlerNodeID})
	tracker := syncstatus.NewTracker(time.Minute) // no success recorded -> offline

	h := NewEdgeHandler(&config.Config{AppMode: config.ModeEdge}, statusService, tracker)

	resp := serveEdgeStatus(t, h)

	assert.Equal(t, config.ModeEdge, resp.Mode)
	assert.False(t, resp.Online)
	assert.Equal(t, 2, resp.PendingSyncOps)
	assert.InDelta(t, 30, resp.SyncLagSeconds, 2)
}

// Edge mode after a successful pull with a drained outbox reports online and caught up.
func TestEdgeHandler_GetStatus_EdgeModeOnlineCaughtUp(t *testing.T) {
	outbox := mocks.NewMockSyncOutboxRepository(t)
	outbox.EXPECT().
		PendingStats(mock.Anything, edgeHandlerNodeID).
		Return(&dto.SyncOutboxPendingStats{PendingCount: 0, OldestPendingAt: nil}, nil)

	statusService := service.NewEdgeStatusService(outbox, dto.SyncIdentity{NodeID: edgeHandlerNodeID})
	tracker := syncstatus.NewTracker(time.Minute)
	tracker.RecordSuccess()

	h := NewEdgeHandler(&config.Config{AppMode: config.ModeEdge}, statusService, tracker)

	resp := serveEdgeStatus(t, h)

	assert.Equal(t, config.ModeEdge, resp.Mode)
	assert.True(t, resp.Online)
	assert.Equal(t, 0, resp.PendingSyncOps)
	assert.Equal(t, 0, resp.SyncLagSeconds)
}
