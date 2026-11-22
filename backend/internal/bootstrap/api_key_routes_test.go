package bootstrap

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/handler"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const orgAPIKeysPath = "/api/organization/api-keys" //nolint:gosec // G101: a route path, not a credential.

type passthroughUnitOfWork struct{}

func (passthroughUnitOfWork) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

type orgAPIKeyRoutes struct {
	router *gin.Engine
	jwt    *service.JWTService
	keys   *mocks.MockAPIKeyRepository
	users  *mocks.MockUserRepository
}

func newOrgAPIKeyRoutes(t *testing.T) *orgAPIKeyRoutes {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := &orgAPIKeyRoutes{keys: mocks.NewMockAPIKeyRepository(t), users: mocks.NewMockUserRepository(t)}
	orgs := mocks.NewMockOrganizationRepository(t)
	orgs.EXPECT().Get(mock.Anything, testOrgID).Return(&dto.Organization{ID: testOrgID, Status: dto.OrganizationStatusActive}, nil).Maybe()
	userRoles := mocks.NewMockUserRoleRepository(t)
	userRoles.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Maybe()
	r.users.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Maybe()
	r.keys.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	r.keys.EXPECT().FindByHash(mock.Anything, mock.Anything).Return(&dto.APIKeyRecord{
		KeyID: "019a0000-0000-7000-8000-0000000000d1", KeyOrganizationID: testOrgID,
		UserID: "019a0000-0000-7000-8000-0000000000c1", Username: "agente", UserOrganizationID: testOrgID, UserActive: true,
		OrganizationStatus: dto.OrganizationStatusActive, RoleIDs: []int{permissions.RoleAgent},
	}, nil).Maybe()
	r.keys.EXPECT().TouchLastUsed(mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(context.Context, string, time.Time) error { return nil }).Maybe()

	apiKeys := service.NewAPIKeyService(r.keys, orgs, r.users, userRoles, passthroughUnitOfWork{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	r.jwt = service.NewJWTService("test-secret", tenant.AnyOrganization())
	r.jwt.AcceptAPIKeys(apiKeys)
	r.router = gin.New()
	RegisterOrganizationAPIKeyRoutes(r.router, handler.NewAPIKeyHandler(apiKeys), r.jwt)
	return r
}

func (r *orgAPIKeyRoutes) serve(t *testing.T, method, path, bearer, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.router.ServeHTTP(rec, req)
	return rec
}

func (r *orgAPIKeyRoutes) token(t *testing.T, role int) string {
	t.Helper()
	token, err := r.jwt.GenerateToken("019a0000-0000-7000-8000-0000000000aa", testOrgID, "staff", []int{role})
	require.NoError(t, err)
	return token
}

func TestOrganizationAPIKeyRoutes_AdminIssues(t *testing.T) {
	r := newOrgAPIKeyRoutes(t)

	rec := r.serve(t, http.MethodPost, orgAPIKeysPath, r.token(t, permissions.RoleAdmin), `{"name":"console-local"}`)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"key":"lgk_`)
}

func TestOrganizationAPIKeyRoutes_OnlyAdminGetsIn(t *testing.T) {
	r := newOrgAPIKeyRoutes(t)
	callers := map[string]string{
		"manager":    r.token(t, permissions.RoleManager),
		"accountant": r.token(t, permissions.RoleAccountant),
		"server":     r.token(t, permissions.RoleServer),
		"agent key":  agentAPIKey,
	}
	for name, bearer := range callers {
		for _, req := range [][3]string{
			{http.MethodPost, orgAPIKeysPath, `{"name":"k"}`},
			{http.MethodGet, orgAPIKeysPath, ""},
			{http.MethodDelete, orgAPIKeysPath + "/019a0000-0000-7000-8000-0000000000d1", ""},
		} {
			rec := r.serve(t, req[0], req[1], bearer, req[2])
			assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s %s", name, req[0], req[1])
		}
	}
	r.users.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	r.keys.AssertNotCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)
}

// An unauthenticated request stops at 401 when the route exists and 404 when it does not.
func TestOrganizationAPIKeyRoutes_OnlyInTheCloud(t *testing.T) {
	for mode, want := range map[config.Mode]int{config.ModeEdge: http.StatusNotFound, config.ModeCloud: http.StatusUnauthorized} {
		router := fullRouter(mode, service.NewJWTService("test-secret", tenant.AnyOrganization()))
		for _, req := range [][2]string{
			{http.MethodPost, orgAPIKeysPath},
			{http.MethodGet, orgAPIKeysPath},
			{http.MethodDelete, orgAPIKeysPath + "/019a0000-0000-7000-8000-0000000000d1"},
		} {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(req[0], req[1], nil))
			assert.Equal(t, want, rec.Code, "%s %s %s", mode, req[0], req[1])
		}
	}
}
