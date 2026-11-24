package sync

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	gosync "sync"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/handler"
	"laguna-escondida/backend/internal/platform/postgres"
	"laguna-escondida/backend/internal/platform/postgres/repository"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type enrollmentRig struct {
	t      *testing.T
	server *httptest.Server
	jwt    *service.JWTService
}

func newEnrollmentRig(t *testing.T) *enrollmentRig {
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
	publicURL := server.URL
	nodes := service.NewNodeService(repository.NewNodeRepository(cloudGDB), repository.NewOrganizationRepository(cloudGDB),
		postgres.NewUnitOfWork(cloudGDB), logger, publicURL)
	ref := service.NewSyncReferenceService(repository.NewSyncReferenceRepository(cloudGDB), logger)
	syncHandler := handler.NewSyncHandler(nil, ref)

	router = gin.New()
	router.Use(handler.DBSessionMiddleware(cloudGDB))
	router.POST("/api/edge/enroll", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.EdgeEnroll),
		handler.NewEdgeEnrollHandler(nodes).EnrollHandler)
	router.GET("/api/sync/pull", handler.NodeAuthMiddleware(nodes), syncHandler.PullHandler)
	return &enrollmentRig{t: t, server: server, jwt: jwtService}
}

func (r *enrollmentRig) enroll(orgID, body string) (int, map[string]json.RawMessage) {
	r.t.Helper()
	token, err := r.jwt.GenerateToken(uuid.NewString(), orgID, "admin", []int{permissions.RoleAdmin})
	require.NoError(r.t, err)
	req, err := http.NewRequest(http.MethodPost, r.server.URL+"/api/edge/enroll", strings.NewReader(body))
	require.NoError(r.t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(r.t, err)
	defer func() { _ = resp.Body.Close() }()
	var out map[string]json.RawMessage
	require.NoError(r.t, json.NewDecoder(resp.Body).Decode(&out))
	return resp.StatusCode, out
}

func (r *enrollmentRig) pull(nodeID, key string) (int, map[string]json.RawMessage) {
	r.t.Helper()
	return r.pullFrom(r.server.URL, nodeID, key)
}

func (r *enrollmentRig) pullFrom(base, nodeID, key string) (int, map[string]json.RawMessage) {
	r.t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/sync/pull", nil)
	require.NoError(r.t, err)
	req.Header.Set("X-Node-ID", nodeID)
	req.Header.Set("X-Node-Key", key)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(r.t, err)
	defer func() { _ = resp.Body.Close() }()
	var out map[string]json.RawMessage
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func str(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var s string
	require.NoError(t, json.Unmarshal(raw, &s))
	return s
}

func seedOtherOrganization(t *testing.T) string {
	t.Helper()
	id := uuid.NewString()
	require.NoError(t, cloudGDB.Exec(`INSERT INTO organizations (id, name, slug) VALUES (?, 'Other', ?)`, id, "other-"+id[:8]).Error)
	t.Cleanup(func() {
		cloudGDB.Exec(`DELETE FROM nodes WHERE organization_id = ?`, id)
		cloudGDB.Exec(`DELETE FROM organizations WHERE id = ?`, id)
	})
	return id
}

func TestEnrollment_EnrollThenPullWithTheReturnedKey(t *testing.T) {
	r := newEnrollmentRig(t)

	status, body := r.enroll(lagunaOrgID, `{"name":"Caja principal"}`)
	require.Equal(t, http.StatusCreated, status)

	pullStatus, _ := r.pullFrom(str(t, body["cloud_sync_url"]), str(t, body["node_id"]), str(t, body["node_key"]))
	assert.Equal(t, http.StatusOK, pullStatus, "the box syncs against cloud_sync_url as returned")
	assert.Equal(t, r.server.URL, str(t, body["cloud_sync_url"]))

	var stored string
	require.NoError(t, cloudGDB.Raw(`SELECT key_hash FROM nodes WHERE id = ?`, str(t, body["node_id"])).Scan(&stored).Error)
	assert.Len(t, stored, 64)
	assert.NotContains(t, stored, str(t, body["node_key"]))
}

func TestEnrollment_SecondBoxWithoutReplaceIsRefusedAndFirstKeepsSyncing(t *testing.T) {
	r := newEnrollmentRig(t)
	_, first := r.enroll(lagunaOrgID, `{"name":"Caja principal"}`)

	status, body := r.enroll(lagunaOrgID, `{"name":"PC nuevo"}`)

	assert.Equal(t, http.StatusConflict, status)
	assert.Equal(t, "box_already_enrolled", str(t, body["code"]))
	pullStatus, _ := r.pull(str(t, first["node_id"]), str(t, first["node_key"]))
	assert.Equal(t, http.StatusOK, pullStatus)
}

func TestEnrollment_ConcurrentEnrollmentsGiveOneWinner(t *testing.T) {
	r := newEnrollmentRig(t)

	const callers = 6
	statuses := make([]int, callers)
	var wg gosync.WaitGroup
	for i := range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses[i], _ = r.enroll(lagunaOrgID, `{"name":"Caja"}`)
		}()
	}
	wg.Wait()

	created, conflicts := 0, 0
	for _, s := range statuses {
		switch s {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			conflicts++
		}
	}
	assert.Equal(t, 1, created, "statuses: %v", statuses)
	assert.Equal(t, callers-1, conflicts, "statuses: %v", statuses)
}

func TestEnrollment_ReplaceMakesTheOldKeyAnswerNodeRevoked(t *testing.T) {
	r := newEnrollmentRig(t)
	_, old := r.enroll(lagunaOrgID, `{"name":"Caja principal"}`)

	status, replacement := r.enroll(lagunaOrgID, `{"name":"PC nuevo","replace":true}`)
	require.Equal(t, http.StatusCreated, status)
	assert.NotEqual(t, str(t, old["node_id"]), str(t, replacement["node_id"]))

	oldStatus, oldBody := r.pull(str(t, old["node_id"]), str(t, old["node_key"]))
	assert.Equal(t, http.StatusUnauthorized, oldStatus)
	assert.Equal(t, "node_revoked", str(t, oldBody["code"]))
	newStatus, _ := r.pull(str(t, replacement["node_id"]), str(t, replacement["node_key"]))
	assert.Equal(t, http.StatusOK, newStatus)

	var rows int
	require.NoError(t, cloudGDB.Raw(`SELECT count(*) FROM nodes WHERE organization_id = ?`, lagunaOrgID).Scan(&rows).Error)
	assert.Equal(t, 2, rows, "the revoked box's record is kept")
}

func TestEnrollment_AnotherOrganizationsNodeIDWithMyKeyIsRefused(t *testing.T) {
	r := newEnrollmentRig(t)
	orgB := seedOtherOrganization(t)
	_, a := r.enroll(lagunaOrgID, `{"name":"A"}`)
	_, b := r.enroll(orgB, `{"name":"B"}`)

	status, _ := r.pull(str(t, b["node_id"]), str(t, a["node_key"]))

	assert.Equal(t, http.StatusUnauthorized, status)
}

func TestEnrollment_SuspendedOrganizationAnswers403ThenRecovers(t *testing.T) {
	r := newEnrollmentRig(t)
	orgB := seedOtherOrganization(t)
	_, b := r.enroll(orgB, `{"name":"B"}`)
	setStatus := func(s string) {
		require.NoError(t, cloudGDB.Exec(`UPDATE organizations SET status = ? WHERE id = ?`, s, orgB).Error)
	}

	setStatus(dto.OrganizationStatusSuspended)
	status, body := r.pull(str(t, b["node_id"]), str(t, b["node_key"]))
	assert.Equal(t, http.StatusForbidden, status)
	assert.Equal(t, "organization_suspended", str(t, body["code"]))

	setStatus(dto.OrganizationStatusActive)
	status, _ = r.pull(str(t, b["node_id"]), str(t, b["node_key"]))
	assert.Equal(t, http.StatusOK, status)
}

func TestEnrollment_BoxRegisteredBeforeKeysIsAdoptedKeepingItsNodeID(t *testing.T) {
	r := newEnrollmentRig(t)
	legacy := uuid.NewString()
	require.NoError(t, cloudGDB.Exec(
		`INSERT INTO nodes (id, name, kind, organization_id) VALUES (?, 'Laguna Escondida edge', 'edge', ?)`, legacy, lagunaOrgID).Error)

	status, _ := r.pull(legacy, "old-shared-key")
	assert.Equal(t, http.StatusUnauthorized, status, "refused until it enrolls")

	enrollStatus, body := r.enroll(lagunaOrgID, `{"name":"Caja principal"}`)
	require.Equal(t, http.StatusCreated, enrollStatus)
	assert.Equal(t, legacy, str(t, body["node_id"]))
	status, _ = r.pull(legacy, str(t, body["node_key"]))
	assert.Equal(t, http.StatusOK, status)
}

func TestEnrollment_LastSeenAdvancesOnlyOnAcceptedRequests(t *testing.T) {
	r := newEnrollmentRig(t)
	_, box := r.enroll(lagunaOrgID, `{"name":"Caja"}`)
	id := str(t, box["node_id"])
	lastSeen := func() *string {
		var v *string
		require.NoError(t, cloudGDB.Raw(`SELECT last_seen_at::text FROM nodes WHERE id = ?`, id).Scan(&v).Error)
		return v
	}
	require.Nil(t, lastSeen())

	r.pull(id, "wrong")
	assert.Nil(t, lastSeen(), "a refused request does not count")

	r.pull(id, str(t, box["node_key"]))
	assert.NotNil(t, lastSeen())
}
