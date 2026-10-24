package bootstrap

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	lagunaOrganizationID = "bb0cad3e-be13-4bad-9a37-bba9525bfece"
	otherOrganizationID  = "019a0000-0000-7000-8000-000000000999"
)

// Existing cloud outbox rows carry this origin; new ones must keep it.
func TestCloudOriginNodeID_MatchesLagunasCloudPeerKey(t *testing.T) {
	assert.Equal(t, config.CloudNodeIDFor(lagunaOrganizationID), cloudOriginNodeID)
}

func setBootSharedEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ADMIN_API_KEY", "admin-key")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("STORAGE_BUCKET", "bucket")
	t.Setenv("METRICS_PORT", "0")
}

func serveMe(t *testing.T, app *App, orgID string) int {
	t.Helper()
	token, err := service.NewJWTService("test-secret", tenant.AnyOrganization()).GenerateToken("019a0000-0000-7000-8000-0000000000aa", orgID, "b-user", nil)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	app.Router.ServeHTTP(rec, req)
	return rec.Code
}

// A leftover ORGANIZATION_ID must not pin the cloud to one org: a user of another org still gets
// past authentication, so their requests run scoped to their own org. The edge, which is pinned,
// is the control.
func TestCloudBoot_IgnoresALeftoverOrganizationID(t *testing.T) {
	setBootSharedEnv(t)
	t.Setenv("ELECTRONIC_INVOICE_URL", "https://invoice.example.com")
	t.Setenv("ELECTRONIC_INVOICE_USER", "user")
	t.Setenv("ELECTRONIC_INVOICE_PASSWORD", "password")
	t.Setenv("ORGANIZATION_ID", lagunaOrganizationID)
	t.Setenv("NODE_ID", "")

	boot, err := CloudBoot()
	require.NoError(t, err)
	assert.Empty(t, boot.NodeID)
	assert.Empty(t, boot.OrganizationID)
	app, err := boot.Assemble(testInfra(t))
	require.NoError(t, err)

	assert.NotEqual(t, http.StatusUnauthorized, serveMe(t, app, otherOrganizationID))
}

func TestEdgeBoot_PinsItsOrganization(t *testing.T) {
	setBootSharedEnv(t)
	t.Setenv("ORGANIZATION_ID", lagunaOrganizationID)
	t.Setenv("NODE_ID", "ce6ebef8-37cf-576b-b7a8-15aaaaae44a1")
	t.Setenv("NODE_SYNC_KEY", "node-key")
	t.Setenv("CLOUD_SYNC_URL", "http://127.0.0.1:1")

	boot, err := EdgeBoot()
	require.NoError(t, err)
	assert.Equal(t, "ce6ebef8-37cf-576b-b7a8-15aaaaae44a1", boot.NodeID)
	assert.Equal(t, lagunaOrganizationID, boot.OrganizationID)
	app, err := boot.Assemble(testInfra(t))
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnauthorized, serveMe(t, app, otherOrganizationID))
}
