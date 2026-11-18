package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/edgeupdate"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	edgeUpdateUserID = "019a0000-0000-7000-8000-0000000000cc"
	stagedV101       = `{"version":"v1.0.1","current":"v1.0.0","components":{"edge-node":"v1.1.19","next":"v1.0.3"}}`
)

// newEdgeUpdateRouter serves the handler over dir; an empty dir means LAGUNA_STATE_DIR is unset.
func newEdgeUpdateRouter(dir string) *gin.Engine {
	var updateService *service.EdgeUpdateService
	if dir != "" {
		store := edgeupdate.NewStateStore(dir, slog.New(slog.NewTextHandler(io.Discard, nil)))
		updateService = service.NewEdgeUpdateService(store)
	}
	h := NewEdgeUpdateHandler(updateService)

	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("user_id", edgeUpdateUserID) })
	router.GET("/api/edge/update", h.GetUpdateHandler)
	router.POST("/api/edge/update", h.RequestUpdateHandler)
	return router
}

func serveEdgeUpdate(router *gin.Engine, method, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/api/edge/update", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func writeStateFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
}

func TestEdgeUpdateHandler_Disabled(t *testing.T) {
	router := newEdgeUpdateRouter("")

	get := serveEdgeUpdate(router, http.MethodGet, "")
	assert.Equal(t, http.StatusOK, get.Code)
	assert.JSONEq(t, `{"enabled":false}`, get.Body.String())

	post := serveEdgeUpdate(router, http.MethodPost, `{"version":"v1.0.1"}`)
	assert.Equal(t, http.StatusNotFound, post.Code)
}

func TestEdgeUpdateHandler_Get_EmptyDirIsAllNull(t *testing.T) {
	w := serveEdgeUpdate(newEdgeUpdateRouter(t.TempDir()), http.MethodGet, "")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"enabled":true,"installed":null,"available":null,"requested":null,"in_progress":null,"last_result":null}`, w.Body.String())
}

func TestEdgeUpdateHandler_Get_CorruptFileIsNullAndBOMParses(t *testing.T) {
	dir := t.TempDir()
	writeStateFile(t, dir, "update-result.json", `{"outcome":`)
	writeStateFile(t, dir, "installed.json", "\xEF\xBB\xBF"+`{"version":"v1.0.0","applied_at":"2026-10-19T15:02:11-05:00"}`)
	writeStateFile(t, dir, "update-available.json", stagedV101)

	w := serveEdgeUpdate(newEdgeUpdateRouter(dir), http.MethodGet, "")

	require.Equal(t, http.StatusOK, w.Code)
	var resp EdgeUpdateResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Enabled)
	assert.Nil(t, resp.LastResult)
	require.NotNil(t, resp.Installed)
	assert.Equal(t, "v1.0.0", resp.Installed.Version)
	require.NotNil(t, resp.Available)
	assert.Equal(t, "v1.1.19", resp.Available.Components["edge-node"])
}

func TestEdgeUpdateHandler_Post_WritesRequest(t *testing.T) {
	dir := t.TempDir()
	writeStateFile(t, dir, "update-available.json", stagedV101)
	router := newEdgeUpdateRouter(dir)

	w := serveEdgeUpdate(router, http.MethodPost, `{"version":"v1.0.1"}`)

	require.Equal(t, http.StatusAccepted, w.Code)
	data, err := os.ReadFile(filepath.Join(dir, "update-requested.json")) //nolint:gosec // test temp dir
	require.NoError(t, err)
	assert.JSONEq(t, w.Body.String(), string(data))
	var written struct {
		Version     string `json:"version"`
		RequestedBy string `json:"requested_by"`
		RequestedAt string `json:"requested_at"`
	}
	require.NoError(t, json.Unmarshal(data, &written))
	assert.Equal(t, "v1.0.1", written.Version)
	assert.Equal(t, edgeUpdateUserID, written.RequestedBy)
	assert.NotEmpty(t, written.RequestedAt)
	assert.NoFileExists(t, filepath.Join(dir, "update-requested.json.tmp"))

	again := serveEdgeUpdate(router, http.MethodPost, `{"version":"v1.0.1"}`)
	assert.Equal(t, http.StatusConflict, again.Code, "the first request is still pending")
}

func TestEdgeUpdateHandler_Post_Conflicts(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		body  string
		code  string
	}{
		{"nothing staged", nil, `{"version":"v1.0.1"}`, "no_staged_update"},
		{"corrupt staged file", map[string]string{"update-available.json": "{"}, `{"version":"v1.0.1"}`, "no_staged_update"},
		{"version mismatch", map[string]string{"update-available.json": stagedV101}, `{"version":"v1.0.2"}`, "version_mismatch"},
		{"request pending", map[string]string{"update-available.json": stagedV101, "update-requested.json": `{"version":"v1.0.1"}`}, `{"version":"v1.0.1"}`, "update_pending"},
		{"apply in progress", map[string]string{"update-available.json": stagedV101, "update-progress.json": `{"phase":"applying"}`}, `{"version":"v1.0.1"}`, "update_pending"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tt.files {
				writeStateFile(t, dir, name, content)
			}

			w := serveEdgeUpdate(newEdgeUpdateRouter(dir), http.MethodPost, tt.body)

			assert.Equal(t, http.StatusConflict, w.Code)
			assert.Contains(t, w.Body.String(), tt.code)
		})
	}
}

func TestEdgeUpdateHandler_Post_BadBody(t *testing.T) {
	dir := t.TempDir()
	writeStateFile(t, dir, "update-available.json", stagedV101)
	router := newEdgeUpdateRouter(dir)

	for _, body := range []string{``, `not json`, `{}`, `{"version":""}`, `{"version":"  "}`, `{"version":101}`} {
		w := serveEdgeUpdate(router, http.MethodPost, body)
		assert.Equal(t, http.StatusBadRequest, w.Code, "body %q", body)
	}
	assert.NoFileExists(t, filepath.Join(dir, "update-requested.json"))
}
