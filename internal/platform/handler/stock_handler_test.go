package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newFreshnessRouter(t *testing.T) (*gin.Engine, *mocks.MockStockReconciliationRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := mocks.NewMockStockReconciliationRepository(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	stockHandler := NewStockHandler(nil, service.NewStockReconciliationService(repo, nil, nil, nil, logger))

	router := gin.New()
	router.GET("/api/stock/sync-freshness", stockHandler.GetStockSyncFreshnessHandler)
	return router, repo
}

func readFreshness(t *testing.T, router *gin.Engine) (*httptest.ResponseRecorder, dto.StockSyncFreshness) {
	t.Helper()
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/stock/sync-freshness", nil))

	var body dto.StockSyncFreshness
	if rec.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	}
	return rec, body
}

func TestStockHandler_GetStockSyncFreshness_FreshEdge(t *testing.T) {
	router, repo := newFreshnessRouter(t)
	appliedAt := time.Now().Add(-30 * time.Second)
	repo.EXPECT().LastEdgeMovementAppliedAt(mock.Anything).Return(&appliedAt, nil).Once()

	rec, body := readFreshness(t, router)

	assert.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, body.StalenessSeconds)
	assert.InDelta(t, 30, *body.StalenessSeconds, 5)
	assert.NotNil(t, body.LastMovementAppliedAt)
}

func TestStockHandler_GetStockSyncFreshness_StaleEdge(t *testing.T) {
	router, repo := newFreshnessRouter(t)
	appliedAt := time.Now().Add(-6 * time.Hour)
	repo.EXPECT().LastEdgeMovementAppliedAt(mock.Anything).Return(&appliedAt, nil).Once()

	rec, body := readFreshness(t, router)

	assert.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, body.StalenessSeconds)
	assert.InDelta(t, 6*60*60, *body.StalenessSeconds, 5)
}

// A node that has never pushed reads as unknown, not as fresh: the counting screen must be
// able to tell "nothing outstanding" apart from "no idea".
func TestStockHandler_GetStockSyncFreshness_NodeHasNeverPushed(t *testing.T) {
	router, repo := newFreshnessRouter(t)
	repo.EXPECT().LastEdgeMovementAppliedAt(mock.Anything).Return(nil, nil).Once()

	rec, body := readFreshness(t, router)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Nil(t, body.LastMovementAppliedAt)
	assert.Nil(t, body.StalenessSeconds)
	assert.JSONEq(t, `{"last_movement_applied_at":null,"staleness_seconds":null}`, rec.Body.String())
}

func TestStockHandler_GetStockSyncFreshness_RepositoryFailure(t *testing.T) {
	router, repo := newFreshnessRouter(t)
	repo.EXPECT().LastEdgeMovementAppliedAt(mock.Anything).Return(nil, assert.AnError).Once()

	rec, _ := readFreshness(t, router)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
