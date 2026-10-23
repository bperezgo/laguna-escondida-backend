package repository_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/handler"
	"laguna-escondida/backend/internal/platform/postgres"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func backendPID(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var pid int64
	require.NoError(t, db.Raw("SELECT pg_backend_pid()").Scan(&pid).Error)
	return pid
}

func orgSetting(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var org string
	require.NoError(t, db.Raw("SELECT COALESCE(current_setting('app.org_id', true), '')").Scan(&org).Error)
	return org
}

func TestSession_PinsOneConnectionWithOrg_Integration(t *testing.T) {
	db := newSyncTestDB(t)
	orgID := uuid.NewString()

	ctx, release := postgres.WithSession(tenant.WithOrg(context.Background(), orgID), db.DB)
	defer release()

	first := backendPID(t, postgres.GetTxOrDB(ctx, db.DB))
	assert.Equal(t, first, backendPID(t, postgres.GetTxOrDB(ctx, db.DB)))
	assert.Equal(t, orgID, orgSetting(t, postgres.GetTxOrDB(ctx, db.DB)))

	require.NoError(t, postgres.NewUnitOfWork(db.DB).Do(ctx, func(ctx context.Context) error {
		tx := postgres.GetTxOrDB(ctx, db.DB)
		assert.Equal(t, first, backendPID(t, tx), "the transaction begins on the pinned connection")
		assert.Equal(t, orgID, orgSetting(t, tx))
		return nil
	}))
}

func TestSession_ReleaseResetsOrgAndFallsBackToPool_Integration(t *testing.T) {
	db := newSyncTestDB(t)
	sqlDB, err := db.DB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	ctx, release := postgres.WithSession(tenant.WithOrg(context.Background(), uuid.NewString()), db.DB)
	pinned := backendPID(t, postgres.GetTxOrDB(ctx, db.DB))
	release()

	after := postgres.GetTxOrDB(ctx, db.DB)
	assert.Equal(t, pinned, backendPID(t, after), "with one pooled connection the released one is reused")
	assert.Empty(t, orgSetting(t, after), "the org must not survive on a pooled connection")
}

func TestSession_DetachLeavesThePinnedConnection_Integration(t *testing.T) {
	db := newSyncTestDB(t)

	ctx, release := postgres.WithSession(tenant.WithOrg(context.Background(), uuid.NewString()), db.DB)
	defer release()
	pinned := backendPID(t, postgres.GetTxOrDB(ctx, db.DB))

	detached := postgres.GetTxOrDB(postgres.Detach(ctx), db.DB)
	assert.NotEqual(t, pinned, backendPID(t, detached))
	assert.Empty(t, orgSetting(t, detached))
}

func TestSession_NoOrgPinsNothing_Integration(t *testing.T) {
	db := newSyncTestDB(t)

	ctx, release := postgres.WithSession(context.Background(), db.DB)
	defer release()

	assert.Empty(t, orgSetting(t, postgres.GetTxOrDB(ctx, db.DB)), "no org in the context, no app.org_id")

	orgID := uuid.NewString()
	withOrg := tenant.WithOrg(ctx, orgID)
	assert.Equal(t, orgID, orgSetting(t, postgres.GetTxOrDB(withOrg, db.DB)), "pins once the org is known")
}

func TestSession_RefusesASecondOrg_Integration(t *testing.T) {
	db := newSyncTestDB(t)

	ctx, release := postgres.WithSession(context.Background(), db.DB)
	defer release()
	_ = backendPID(t, postgres.GetTxOrDB(tenant.WithOrg(ctx, uuid.NewString()), db.DB))

	var pid int64
	err := postgres.GetTxOrDB(tenant.WithOrg(ctx, uuid.NewString()), db.DB).Raw("SELECT pg_backend_pid()").Scan(&pid).Error
	assert.ErrorContains(t, err, "pinned for organization")
}

// The session takes the org authentication put in the context, so each caller gets its own.
func TestDBSessionMiddleware_OrgComesFromTheToken_Integration(t *testing.T) {
	db := newSyncTestDB(t)
	gin.SetMode(gin.TestMode)
	jwtService := service.NewJWTService("test-secret", "")

	router := gin.New()
	router.Use(handler.DBSessionMiddleware(db.DB))
	probe := func(c *gin.Context) {
		c.String(http.StatusOK, orgSetting(t, postgres.GetTxOrDB(c.Request.Context(), db.DB)))
	}
	router.GET("/authed", handler.JWTAuthMiddleware(jwtService), probe)
	router.GET("/open", probe)

	call := func(path, orgID string) string {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if orgID != "" {
			token, err := jwtService.GenerateToken("user-1", orgID, "cajero1", []int{permissions.RoleServer})
			require.NoError(t, err)
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		return rec.Body.String()
	}

	orgA, orgB := uuid.NewString(), uuid.NewString()
	assert.Equal(t, orgA, call("/authed", orgA))
	assert.Equal(t, orgB, call("/authed", orgB))
	assert.Empty(t, call("/open", ""), "an unauthenticated request gets no org")
}
