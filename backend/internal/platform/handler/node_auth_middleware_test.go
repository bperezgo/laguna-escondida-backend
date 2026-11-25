package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testNodeID    = "019a0000-0000-7000-8000-0000000000e1"
	testNodeOrgID = "019a0000-0000-7000-8000-00000000000a"
	testRevokedID = "019a0000-0000-7000-8000-0000000000e2"
	testSuspendID = "019a0000-0000-7000-8000-0000000000e3"
	testOtherID   = "019a0000-0000-7000-8000-0000000000e4"
)

// registeredNodes registers testNodeID with nodeKey (none when empty). touched, when set,
// records the node ids whose last_seen_at was written.
func registeredNodes(t *testing.T, nodeKey string, touched *[]string) *service.NodeService {
	t.Helper()
	repo := mocks.NewMockNodeRepository(t)
	record := func(id string, mutate func(*dto.NodeAuthRecord)) *dto.NodeAuthRecord {
		r := &dto.NodeAuthRecord{
			Node:               dto.Node{ID: id, OrganizationID: testNodeOrgID},
			OrganizationStatus: dto.OrganizationStatusActive,
		}
		if nodeKey != "" {
			r.KeyHash = nodeKeyHash(nodeKey)
		}
		if mutate != nil {
			mutate(r)
		}
		return r
	}
	now := time.Now()
	repo.On("FindForAuth", mock.Anything, testNodeID).Return(record(testNodeID, nil), nil).Maybe()
	repo.On("FindForAuth", mock.Anything, testRevokedID).Return(record(testRevokedID, func(r *dto.NodeAuthRecord) { r.RevokedAt = &now }), nil).Maybe()
	repo.On("FindForAuth", mock.Anything, testSuspendID).Return(record(testSuspendID, func(r *dto.NodeAuthRecord) { r.OrganizationStatus = dto.OrganizationStatusSuspended }), nil).Maybe()
	repo.On("FindForAuth", mock.Anything, testOtherID).Return(record(testOtherID, func(r *dto.NodeAuthRecord) {
		r.KeyHash = nodeKeyHash("another-boxes-key")
	}), nil).Maybe()
	repo.On("FindForAuth", mock.Anything, mock.Anything).Return(nil, domainError.ErrNodeNotFound).Maybe()
	repo.On("TouchLastSeen", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			if touched != nil {
				*touched = append(*touched, args.String(1))
			}
		}).Return(nil).Maybe()
	return service.NewNodeService(repo, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), "")
}

func nodeAuthRouter(t *testing.T, nodeKey string, seen *string, touched *[]string) *gin.Engine {
	t.Helper()
	router := gin.New()
	router.POST("/api/sync/push", PushNodeIDFromBody(), NodeAuthMiddleware(registeredNodes(t, nodeKey, touched)), func(c *gin.Context) {
		*seen, _ = tenant.OrgFrom(c.Request.Context())
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return router
}

func TestNodeAuthMiddleware(t *testing.T) {
	const key = "lgn_secret"
	const plainBody = `{"error":"invalid node credentials"}`

	tests := []struct {
		name       string
		key        string
		nodeID     string
		body       string
		wantStatus int
		wantOrg    string
		wantCode   string
		wantTouch  bool
	}{
		{name: "box with its own key", key: key, nodeID: testNodeID, wantStatus: http.StatusOK, wantOrg: testNodeOrgID, wantTouch: true},
		{name: "node id only in the push body", key: key, body: `{"node_id":"` + testNodeID + `","ops":[]}`, wantStatus: http.StatusOK, wantOrg: testNodeOrgID, wantTouch: true},
		{name: "unregistered node", key: key, nodeID: "019a0000-0000-7000-8000-0000000000ff", wantStatus: http.StatusUnauthorized},
		{name: "no node id", key: key, wantStatus: http.StatusUnauthorized},
		{name: "wrong key", key: "nope", nodeID: testNodeID, wantStatus: http.StatusUnauthorized},
		{name: "missing key", nodeID: testNodeID, wantStatus: http.StatusUnauthorized},
		{name: "key of another box", key: key, nodeID: testOtherID, wantStatus: http.StatusUnauthorized},
		{name: "revoked box with its key", key: key, nodeID: testRevokedID, wantStatus: http.StatusUnauthorized, wantCode: "node_revoked"},
		{name: "revoked box with a wrong key", key: "nope", nodeID: testRevokedID, wantStatus: http.StatusUnauthorized},
		{name: "suspended organization", key: key, nodeID: testSuspendID, wantStatus: http.StatusForbidden, wantCode: "organization_suspended"},
		{name: "suspended organization with a wrong key", key: "nope", nodeID: testSuspendID, wantStatus: http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var seen string
			var touched []string
			router := nodeAuthRouter(t, key, &seen, &touched)

			req := httptest.NewRequest(http.MethodPost, "/api/sync/push", strings.NewReader(tt.body))
			if tt.key != "" {
				req.Header.Set("X-Node-Key", tt.key)
			}
			if tt.nodeID != "" {
				req.Header.Set(NodeIDHeader, tt.nodeID)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
			assert.Equal(t, tt.wantOrg, seen)
			assert.Equal(t, tt.wantTouch, len(touched) > 0, "last_seen_at is written for accepted requests only")
			if tt.wantStatus == http.StatusUnauthorized && tt.wantCode == "" {
				assert.JSONEq(t, plainBody, w.Body.String())
			}
			if tt.wantCode != "" {
				var body map[string]string
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
				assert.Equal(t, tt.wantCode, body["code"])
			}
		})
	}
}

// A box registered before node keys has no hash; it must be refused whatever it sends.
func TestNodeAuthMiddleware_NodeWithoutKeyIsRefused(t *testing.T) {
	var seen string
	router := nodeAuthRouter(t, "", &seen, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/sync/push", nil)
	req.Header.Set("X-Node-Key", "anything")
	req.Header.Set(NodeIDHeader, testNodeID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func nodeKeyHash(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func TestNodeAuthMiddleware_RefusalRetryAfter(t *testing.T) {
	const key = "lgn_secret"
	refusals := []struct {
		name       string
		key        string
		nodeID     string
		syncStatus int
		body       string
	}{
		{"wrong key", "nope", testNodeID, http.StatusUnauthorized, `{"error":"invalid node credentials"}`},
		{"revoked", key, testRevokedID, http.StatusUnauthorized, `{"error":"node revoked","code":"node_revoked"}`},
		{"suspended", key, testSuspendID, http.StatusForbidden, `{"error":"organization suspended","code":"organization_suspended"}`},
	}
	routes := []struct {
		name string
		opts []NodeAuthOption
		want func(syncStatus int) int
	}{
		{"sync", []NodeAuthOption{WithRefusalRetryAfter(6 * time.Hour)}, func(s int) int { return s }},
		{"telemetry", []NodeAuthOption{WithRefusalRetryAfter(6 * time.Hour), WithRefusalStatus(http.StatusTooManyRequests)},
			func(int) int { return http.StatusTooManyRequests }},
	}
	for _, route := range routes {
		for _, tc := range refusals {
			t.Run(route.name+"/"+tc.name, func(t *testing.T) {
				router := gin.New()
				router.POST("/x", NodeAuthMiddleware(registeredNodes(t, key, nil), route.opts...), func(c *gin.Context) { c.Status(http.StatusOK) })
				req := httptest.NewRequest(http.MethodPost, "/x", nil)
				req.Header.Set("X-Node-Key", tc.key)
				req.Header.Set(NodeIDHeader, tc.nodeID)
				w := httptest.NewRecorder()

				router.ServeHTTP(w, req)

				assert.Equal(t, route.want(tc.syncStatus), w.Code)
				assert.Equal(t, "21600", w.Header().Get("Retry-After"))
				assert.JSONEq(t, tc.body, w.Body.String())
			})
		}
	}
}

func TestNodeAuthMiddleware_AcceptedRequestHasNoRetryAfter(t *testing.T) {
	const key = "lgn_secret"
	router := gin.New()
	router.POST("/x", NodeAuthMiddleware(registeredNodes(t, key, nil), WithRefusalRetryAfter(time.Hour), WithRefusalStatus(http.StatusTooManyRequests)),
		func(c *gin.Context) { c.Status(http.StatusOK) })
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.Header.Set("X-Node-Key", key)
	req.Header.Set(NodeIDHeader, testNodeID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("Retry-After"))
}
