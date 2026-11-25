package sync

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/cache/memory"
	"laguna-escondida/backend/internal/platform/handler"
	"laguna-escondida/backend/internal/platform/postgres"
	"laguna-escondida/backend/internal/platform/postgres/repository"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const refusalCacheWindow = 300 * time.Millisecond

// newRefusalRig is the enrollment rig with the cloud's refusal cache (window shortened) and Retry-After.
func newRefusalRig(t *testing.T) *enrollmentRig {
	t.Helper()
	if !gateEnabled() {
		t.Skip("Skipping acceptance test. Set RUN_ACCEPTANCE_TESTS=true to run (needs Docker or DB_HOST).")
	}
	truncateAll(t, cloudGDB)

	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	jwtService := service.NewJWTService("accept-secret", tenant.AnyOrganization())
	var router *gin.Engine
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) { router.ServeHTTP(w, req) }))
	t.Cleanup(server.Close)
	nodes := service.NewNodeService(repository.NewNodeRepository(cloudGDB), repository.NewOrganizationRepository(cloudGDB),
		postgres.NewUnitOfWork(cloudGDB), logger, server.URL,
		service.WithNodeRefusalCache(memory.New(memory.WithMaxEntries(100)), refusalCacheWindow))
	syncHandler := handler.NewSyncHandler(nil, service.NewSyncReferenceService(repository.NewSyncReferenceRepository(cloudGDB), logger))

	router = gin.New()
	router.Use(handler.DBSessionMiddleware(cloudGDB))
	router.POST("/api/edge/enroll", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.EdgeEnroll),
		handler.NewEdgeEnrollHandler(nodes).EnrollHandler)
	router.GET("/api/sync/pull", handler.NodeAuthMiddleware(nodes, handler.WithRefusalRetryAfter(6*time.Hour)), syncHandler.PullHandler)
	return &enrollmentRig{t: t, server: server, jwt: jwtService}
}

func (r *enrollmentRig) pullResponse(nodeID, key string) (int, http.Header) {
	r.t.Helper()
	req, err := http.NewRequest(http.MethodGet, r.server.URL+"/api/sync/pull", nil)
	require.NoError(r.t, err)
	req.Header.Set("X-Node-ID", nodeID)
	req.Header.Set("X-Node-Key", key)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(r.t, err)
	_ = resp.Body.Close()
	return resp.StatusCode, resp.Header
}

func setOrganizationStatus(t *testing.T, orgID, status string) {
	t.Helper()
	require.NoError(t, cloudGDB.Exec(`UPDATE organizations SET status = ? WHERE id = ?`, status, orgID).Error)
}

func TestRefusalBackoff_SuspendedBoxIsRefusedWithRetryAfterUntilReactivated(t *testing.T) {
	r := newRefusalRig(t)
	status, body := r.enroll(lagunaOrgID, `{"name":"Caja principal"}`)
	require.Equal(t, http.StatusCreated, status)
	nodeID, key := str(t, body["node_id"]), str(t, body["node_key"])
	setOrganizationStatus(t, lagunaOrgID, "suspended")
	t.Cleanup(func() { setOrganizationStatus(t, lagunaOrgID, "active") })

	for i := 0; i < 2; i++ {
		refused, header := r.pullResponse(nodeID, key)
		assert.Equal(t, http.StatusForbidden, refused, "call %d", i+1)
		assert.Equal(t, "21600", header.Get("Retry-After"), "call %d", i+1)
	}

	setOrganizationStatus(t, lagunaOrgID, "active")
	status, _ = r.pullResponse(nodeID, key)
	assert.Equal(t, http.StatusForbidden, status, "the cached refusal still answers inside its window")

	time.Sleep(refusalCacheWindow + 100*time.Millisecond)
	status, header := r.pullResponse(nodeID, key)
	assert.Equal(t, http.StatusOK, status, "accepted once the window has passed")
	assert.Empty(t, header.Get("Retry-After"))
}
