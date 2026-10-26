package handler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testAPIKey    = "lgk_tttttttttttttttttttttttttttttttttttttttttttt"
	apiKeyUserID  = "019a0000-0000-7000-8000-0000000000c1"
	apiKeyEntryID = "019a0000-0000-7000-8000-0000000000d1"
)

type seenRequest struct {
	org      string
	userID   string
	username string
	roleIDs  []int
	reached  bool
	logged   string
	status   int
	body     string
}

// cloudJWTService accepts API keys backed by keys, the way NewCloud wires it.
func cloudJWTService(t *testing.T) (*service.JWTService, *mocks.MockAPIKeyRepository) {
	t.Helper()
	keys := mocks.NewMockAPIKeyRepository(t)
	apiKeys := service.NewAPIKeyService(keys, mocks.NewMockOrganizationRepository(t), mocks.NewMockUserRepository(t),
		mocks.NewMockUserRoleRepository(t), slog.New(slog.NewTextHandler(io.Discard, nil)))
	jwt := service.NewJWTService("test-secret", tenant.AnyOrganization())
	jwt.AcceptAPIKeys(apiKeys)
	return jwt, keys
}

func serveBearer(t *testing.T, jwt *service.JWTService, bearer string, requiredRoles ...int) seenRequest {
	t.Helper()
	gin.SetMode(gin.TestMode)
	var seen seenRequest
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Next()
		seen.logged = c.Errors.String()
	})
	router.GET("/probe", JWTAuthMiddleware(jwt, requiredRoles...), func(c *gin.Context) {
		seen.reached = true
		seen.org, _ = tenant.OrgFrom(c.Request.Context())
		seen.userID = c.GetString("user_id")
		seen.username = c.GetString("username")
		seen.roleIDs, _ = c.MustGet("role_ids").([]int)
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.Header.Set("Authorization", "Bearer "+bearer)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	seen.status, seen.body = rec.Code, rec.Body.String()
	return seen
}

func activeAgentRecord() *dto.APIKeyRecord {
	return &dto.APIKeyRecord{
		KeyID: apiKeyEntryID, KeyOrganizationID: authOrgA,
		UserID: apiKeyUserID, Username: "agente", UserOrganizationID: authOrgA, UserActive: true,
		OrganizationStatus: dto.OrganizationStatusActive, RoleIDs: []int{permissions.RoleAgent},
	}
}

func TestJWTAuthMiddleware_APIKey_ActsAsTheKeysUserInTheKeysOrganization(t *testing.T) {
	jwt, keys := cloudJWTService(t)
	var lookupCtx context.Context
	keys.EXPECT().FindByHash(mock.Anything, mock.Anything).
		Run(func(ctx context.Context, _ string) { lookupCtx = ctx }).Return(activeAgentRecord(), nil)
	touched := make(chan struct{})
	keys.EXPECT().TouchLastUsed(mock.Anything, apiKeyEntryID, mock.Anything).
		Run(func(context.Context, string, time.Time) { close(touched) }).Return(nil)

	seen := serveBearer(t, jwt, testAPIKey)

	require.Equal(t, http.StatusOK, seen.status, seen.body)
	assert.Equal(t, authOrgA, seen.org)
	assert.Equal(t, apiKeyUserID, seen.userID)
	assert.Equal(t, "agente", seen.username)
	assert.Equal(t, []int{permissions.RoleAgent}, seen.roleIDs)
	<-touched

	// Guard for RLS (design D4): the key names the organization, so its lookup must not run
	// under one. A context with an org would pin a session to it and scope the lookup.
	_, hasOrg := tenant.OrgFrom(lookupCtx)
	assert.False(t, hasOrg, "the key lookup runs without an organization")
}

func TestJWTAuthMiddleware_APIKey_RequiredRolesUseTheUsersRoles(t *testing.T) {
	jwt, keys := cloudJWTService(t)
	keys.EXPECT().FindByHash(mock.Anything, mock.Anything).Return(activeAgentRecord(), nil)
	keys.EXPECT().TouchLastUsed(mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	seen := serveBearer(t, jwt, testAPIKey, permissions.RoleManager)

	assert.Equal(t, http.StatusForbidden, seen.status)
	assert.False(t, seen.reached)
}

func TestJWTAuthMiddleware_APIKey_EveryRefusalLooksTheSame(t *testing.T) {
	revokedAt := time.Now()
	cases := []struct {
		name       string
		record     func(r *dto.APIKeyRecord)
		notFound   bool
		wantLogged string
	}{
		{name: "unknown key", notFound: true, wantLogged: "unknown key"},
		{name: "revoked", record: func(r *dto.APIKeyRecord) { r.RevokedAt = &revokedAt }, wantLogged: "revoked"},
		{name: "user inactive", record: func(r *dto.APIKeyRecord) { r.UserActive = false }, wantLogged: "user inactive"},
		{name: "user deleted", record: func(r *dto.APIKeyRecord) { r.UserDeleted = true }, wantLogged: "user deleted"},
		{
			name: "organization suspended", wantLogged: "organization suspended",
			record: func(r *dto.APIKeyRecord) { r.OrganizationStatus = dto.OrganizationStatusSuspended },
		},
	}
	var bodies []string
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			jwt, keys := cloudJWTService(t)
			if tc.notFound {
				keys.EXPECT().FindByHash(mock.Anything, mock.Anything).Return(nil, domainError.ErrAPIKeyNotFound)
			} else {
				record := activeAgentRecord()
				tc.record(record)
				keys.EXPECT().FindByHash(mock.Anything, mock.Anything).Return(record, nil)
			}

			seen := serveBearer(t, jwt, testAPIKey)

			assert.Equal(t, http.StatusUnauthorized, seen.status)
			assert.False(t, seen.reached)
			assert.Contains(t, seen.logged, tc.wantLogged)
			assert.NotContains(t, seen.logged, testAPIKey[4:], "the log never carries the key")
			bodies = append(bodies, seen.body)
		})
	}
	invalidJWT := serveBearer(t, service.NewJWTService("test-secret", tenant.AnyOrganization()), "not-a-jwt")
	for _, body := range bodies {
		assert.JSONEq(t, invalidJWT.body, body, "a refused key reads like any invalid token")
	}
}

func TestJWTAuthMiddleware_APIKey_EdgeBoxRefusesEveryKey(t *testing.T) {
	box := service.NewJWTService("test-secret", tenant.PinnedTo(authOrgA))

	seen := serveBearer(t, box, testAPIKey)

	assert.Equal(t, http.StatusUnauthorized, seen.status)
	assert.JSONEq(t, `{"error":"Invalid or expired token"}`, seen.body)
	assert.Contains(t, seen.logged, "not accepted here")
	assert.False(t, seen.reached)
}

func TestJWTAuthMiddleware_APIKey_LookupFailureIs500(t *testing.T) {
	jwt, keys := cloudJWTService(t)
	keys.EXPECT().FindByHash(mock.Anything, mock.Anything).Return(nil, errors.New("db down"))

	seen := serveBearer(t, jwt, testAPIKey)

	assert.Equal(t, http.StatusInternalServerError, seen.status)
	assert.False(t, seen.reached)
}

func TestJWTAuthMiddleware_JWTPathUnchangedWhenKeysAreAccepted(t *testing.T) {
	jwt, keys := cloudJWTService(t)

	seen := serveBearer(t, jwt, issue(t, authOrgB))

	require.Equal(t, http.StatusOK, seen.status)
	assert.Equal(t, authOrgB, seen.org)
	assert.Equal(t, "user-1", seen.userID)
	keys.AssertNotCalled(t, "FindByHash", mock.Anything, mock.Anything)
}
