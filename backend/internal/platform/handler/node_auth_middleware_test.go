package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	testNodeID    = "019a0000-0000-7000-8000-0000000000e1"
	testNodeOrgID = "019a0000-0000-7000-8000-00000000000a"
)

// registeredNodes answers testNodeID with testNodeOrgID and any other id as unregistered.
func registeredNodes(t *testing.T) *service.NodeService {
	t.Helper()
	repo := mocks.NewMockNodeRepository(t)
	repo.On("FindByID", mock.Anything, testNodeID).Return(&dto.Node{ID: testNodeID, OrganizationID: testNodeOrgID}, nil).Maybe()
	repo.On("FindByID", mock.Anything, mock.Anything).Return(nil, domainError.ErrNodeNotFound).Maybe()
	return service.NewNodeService(repo)
}

// nodeAuthRouter serves POST /api/sync/push behind the middleware and reports the org the handler saw.
func nodeAuthRouter(t *testing.T, nodeSyncKey string, seen *string) *gin.Engine {
	t.Helper()
	router := gin.New()
	router.POST("/api/sync/push", PushNodeIDFromBody(), NodeAuthMiddleware(nodeSyncKey, registeredNodes(t)), func(c *gin.Context) {
		*seen, _ = tenant.OrgFrom(c.Request.Context())
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return router
}

func TestNodeAuthMiddleware(t *testing.T) {
	nodeSyncKey := "secret-key"

	tests := []struct {
		name       string
		key        string
		nodeID     string
		body       string
		wantStatus int
		wantOrg    string
	}{
		{name: "registered node", key: "secret-key", nodeID: testNodeID, wantStatus: http.StatusOK, wantOrg: testNodeOrgID},
		{name: "unregistered node", key: "secret-key", nodeID: "019a0000-0000-7000-8000-0000000000ff", wantStatus: http.StatusUnauthorized},
		{name: "no node id", key: "secret-key", wantStatus: http.StatusUnauthorized},
		{name: "node id only in the push body", key: "secret-key", body: `{"node_id":"` + testNodeID + `","ops":[]}`, wantStatus: http.StatusOK, wantOrg: testNodeOrgID},
		{name: "wrong key", key: "nope", nodeID: testNodeID, wantStatus: http.StatusUnauthorized},
		{name: "missing key", nodeID: testNodeID, wantStatus: http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var seen string
			router := nodeAuthRouter(t, nodeSyncKey, &seen)

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
		})
	}
}

// TestNodeAuthMiddleware_FailsClosedWhenUnconfigured proves an install with no node
// key configured rejects everything rather than allowing unauthenticated access.
func TestNodeAuthMiddleware_FailsClosedWhenUnconfigured(t *testing.T) {
	var seen string
	router := nodeAuthRouter(t, "", &seen)

	req := httptest.NewRequest(http.MethodPost, "/api/sync/push", nil)
	req.Header.Set("X-Node-Key", "anything")
	req.Header.Set(NodeIDHeader, testNodeID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
