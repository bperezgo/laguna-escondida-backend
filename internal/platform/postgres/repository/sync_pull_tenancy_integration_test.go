package repository

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/handler"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The pull a node gets is its organization's alone, while another organization's reference
// data sits in the same tables; an unregistered node gets nothing.
func TestSyncPull_ReturnsOnlyTheCallingNodesOrganization_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	orgB := seedOrganization(t, db)
	asB := tenant.WithOrg(lagunaCtx(), orgB)

	lagunaProduct := seedProduct(t, db)
	lagunaUser := seedUser(t, db)
	bProduct := uuid.NewString()
	require.NoError(t, db.DB.WithContext(asB).Create(&productModel{ID: bProduct, Name: "B product", Category: "test",
		ProductType: "SELLABLE", UnitOfMeasure: "unit", Version: 1, SKU: "B-" + bProduct[:8]}).Error)
	t.Cleanup(func() { db.DB.Exec("DELETE FROM products WHERE id = ?", bProduct) })
	bUser := uuid.NewString()
	require.NoError(t, db.DB.WithContext(asB).Create(&userModel{ID: bUser, Username: "b-" + bUser[:8], Name: "B", Password: "x"}).Error)
	t.Cleanup(func() { db.DB.Exec("DELETE FROM users WHERE id = ?", bUser) })

	lagunaNode := uuid.NewString()
	require.NoError(t, db.DB.Exec("INSERT INTO nodes (id, name, kind, organization_id) VALUES (?, 'test edge', 'edge', ?)", lagunaNode, lagunaOrgID).Error)
	t.Cleanup(func() { db.DB.Exec("DELETE FROM nodes WHERE id = ?", lagunaNode) })

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(handler.DBSessionMiddleware(db.DB))
	syncHandler := handler.NewSyncHandler(nil, service.NewSyncReferenceService(NewSyncReferenceRepository(db.DB), slog.Default()))
	router.GET("/api/sync/pull", handler.NodeAuthMiddleware(&config.Config{NodeSyncKey: "key"},
		service.NewNodeService(NewNodeRepository(db.DB))), syncHandler.PullHandler)

	pull := func(nodeID string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/sync/pull", nil)
		req.Header.Set("X-Node-Key", "key")
		req.Header.Set(handler.NodeIDHeader, nodeID)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	rec := pull(lagunaNode)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp dto.SyncPullResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	productIDs := map[string]bool{}
	for _, p := range resp.Products {
		productIDs[p.ID] = true
	}
	userIDs := map[string]bool{}
	for _, u := range resp.Users {
		userIDs[u.ID] = true
	}
	assert.True(t, productIDs[lagunaProduct])
	assert.True(t, userIDs[lagunaUser])
	assert.False(t, productIDs[bProduct], "B's product is not in Laguna's pull")
	assert.False(t, userIDs[bUser], "B's user is not in Laguna's pull")

	assert.Equal(t, http.StatusUnauthorized, pull(uuid.NewString()).Code, "an unregistered node")
}
