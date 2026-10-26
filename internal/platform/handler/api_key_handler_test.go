package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type apiKeyRouter struct {
	router    *gin.Engine
	keys      *mocks.MockAPIKeyRepository
	orgs      *mocks.MockOrganizationRepository
	users     *mocks.MockUserRepository
	userRoles *mocks.MockUserRoleRepository
}

func newAPIKeyRouter(t *testing.T) *apiKeyRouter {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := &apiKeyRouter{
		keys:      mocks.NewMockAPIKeyRepository(t),
		orgs:      mocks.NewMockOrganizationRepository(t),
		users:     mocks.NewMockUserRepository(t),
		userRoles: mocks.NewMockUserRoleRepository(t),
	}
	h := NewAPIKeyHandler(service.NewAPIKeyService(r.keys, r.orgs, r.users, r.userRoles, slog.New(slog.NewTextHandler(io.Discard, nil))))
	r.router = gin.New()
	r.router.POST("/api/platform/organizations/:id/api-keys", h.IssueHandler)
	r.router.GET("/api/platform/organizations/:id/api-keys", h.ListHandler)
	r.router.DELETE("/api/platform/organizations/:id/api-keys/:keyId", h.RevokeHandler)
	return r
}

func (r *apiKeyRouter) activeOrg() {
	r.orgs.EXPECT().Get(mock.Anything, testOrganizationID).Return(&dto.Organization{ID: testOrganizationID, Status: dto.OrganizationStatusActive}, nil)
}

func (r *apiKeyRouter) userWithRole(roleID int) {
	r.users.EXPECT().FindByID(mock.Anything, apiKeyUserID).Return(&dto.User{ID: apiKeyUserID, Active: true}, nil)
	r.userRoles.EXPECT().FindByUserID(mock.Anything, apiKeyUserID).Return([]*dto.UserRole{{RoleID: roleID}}, nil)
}

const apiKeysPath = "/api/platform/organizations/" + testOrganizationID + "/api-keys"

func TestAPIKeyHandler_Issue_Returns201WithThePlaintextOnce(t *testing.T) {
	r := newAPIKeyRouter(t)
	r.activeOrg()
	r.userWithRole(permissions.RoleAgent)
	r.keys.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(nil)

	rec := serveOrganization(r.router, http.MethodPost, apiKeysPath, `{"user_id":"`+apiKeyUserID+`","name":"whatsapp-agent"}`)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	body := decodeBody(t, rec)
	var key string
	require.NoError(t, json.Unmarshal(body["key"], &key))
	assert.True(t, strings.HasPrefix(key, "lgk_"))
	for _, field := range []string{"id", "name", "user_id", "organization_id", "prefix", "created_at"} {
		assert.Contains(t, body, field)
	}
	assert.NotContains(t, body, "hash")
}

func TestAPIKeyHandler_Issue_StatusMapping(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		arrange   func(r *apiKeyRouter)
		wantCode  int
		wantField string
		created   bool
	}{
		{name: "malformed body", body: `{`, wantCode: http.StatusBadRequest},
		{name: "missing name", body: `{"user_id":"` + apiKeyUserID + `"}`, wantCode: http.StatusBadRequest, wantField: "name"},
		{
			name: "staff user", body: `{"user_id":"` + apiKeyUserID + `","name":"k"}`, wantCode: http.StatusBadRequest, wantField: "user_id",
			arrange: func(r *apiKeyRouter) { r.activeOrg(); r.userWithRole(permissions.RoleManager) },
		},
		{
			name: "user of another organization", body: `{"user_id":"` + apiKeyUserID + `","name":"k"}`, wantCode: http.StatusBadRequest, wantField: "user_id",
			arrange: func(r *apiKeyRouter) {
				r.activeOrg()
				r.users.EXPECT().FindByID(mock.Anything, apiKeyUserID).Return(nil, domainError.ErrUserNotFound)
			},
		},
		{
			name: "unknown organization", body: `{"user_id":"` + apiKeyUserID + `","name":"k"}`, wantCode: http.StatusNotFound,
			arrange: func(r *apiKeyRouter) {
				r.orgs.EXPECT().Get(mock.Anything, testOrganizationID).Return(nil, domainError.ErrOrganizationNotFound)
			},
		},
		{
			name: "suspended organization", body: `{"user_id":"` + apiKeyUserID + `","name":"k"}`, wantCode: http.StatusNotFound,
			arrange: func(r *apiKeyRouter) {
				r.orgs.EXPECT().Get(mock.Anything, testOrganizationID).Return(&dto.Organization{ID: testOrganizationID, Status: dto.OrganizationStatusSuspended}, nil)
			},
		},
		{
			name: "repository failure", body: `{"user_id":"` + apiKeyUserID + `","name":"k"}`, wantCode: http.StatusInternalServerError, created: true,
			arrange: func(r *apiKeyRouter) {
				r.activeOrg()
				r.userWithRole(permissions.RoleAgent)
				r.keys.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(errors.New("db down"))
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newAPIKeyRouter(t)
			if tc.arrange != nil {
				tc.arrange(r)
			}

			rec := serveOrganization(r.router, http.MethodPost, apiKeysPath, tc.body)

			assert.Equal(t, tc.wantCode, rec.Code, rec.Body.String())
			if tc.wantField != "" {
				var field string
				require.NoError(t, json.Unmarshal(decodeBody(t, rec)["field"], &field))
				assert.Equal(t, tc.wantField, field)
			}
			if !tc.created {
				r.keys.AssertNotCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)
			}
		})
	}
}

func TestAPIKeyHandler_List_NeverCarriesThePlaintextOrHash(t *testing.T) {
	r := newAPIKeyRouter(t)
	r.activeOrg()
	r.keys.EXPECT().List(mock.Anything).Return([]*dto.APIKey{{ID: apiKeyEntryID, Name: "whatsapp-agent", Prefix: "AbCdEfGh"}}, nil)

	rec := serveOrganization(r.router, http.MethodGet, apiKeysPath, "")

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		APIKeys []map[string]json.RawMessage `json:"api_keys"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.APIKeys, 1)
	assert.NotContains(t, resp.APIKeys[0], "key")
	assert.NotContains(t, resp.APIKeys[0], "hash")
	for _, field := range []string{"id", "name", "user_id", "prefix", "created_at", "last_used_at", "revoked_at"} {
		assert.Contains(t, resp.APIKeys[0], field)
	}
}

func TestAPIKeyHandler_List_UnknownOrganization_Returns404(t *testing.T) {
	r := newAPIKeyRouter(t)
	r.orgs.EXPECT().Get(mock.Anything, testOrganizationID).Return(nil, domainError.ErrOrganizationNotFound)

	rec := serveOrganization(r.router, http.MethodGet, apiKeysPath, "")

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAPIKeyHandler_Revoke_StatusMapping(t *testing.T) {
	cases := []struct {
		name     string
		keyID    string
		arrange  func(r *apiKeyRouter)
		wantCode int
	}{
		{
			name: "own key", keyID: apiKeyEntryID, wantCode: http.StatusNoContent,
			arrange: func(r *apiKeyRouter) {
				r.activeOrg()
				r.keys.EXPECT().Revoke(mock.Anything, apiKeyEntryID, mock.Anything).Return(nil)
			},
		},
		{
			name: "another organization's key", keyID: apiKeyEntryID, wantCode: http.StatusNotFound,
			arrange: func(r *apiKeyRouter) {
				r.activeOrg()
				r.keys.EXPECT().Revoke(mock.Anything, apiKeyEntryID, mock.Anything).Return(domainError.ErrAPIKeyNotFound)
			},
		},
		{name: "malformed key id", keyID: "nope", wantCode: http.StatusNotFound, arrange: func(r *apiKeyRouter) { r.activeOrg() }},
		{
			name: "unknown organization", keyID: apiKeyEntryID, wantCode: http.StatusNotFound,
			arrange: func(r *apiKeyRouter) {
				r.orgs.EXPECT().Get(mock.Anything, testOrganizationID).Return(nil, domainError.ErrOrganizationNotFound)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newAPIKeyRouter(t)
			tc.arrange(r)

			rec := serveOrganization(r.router, http.MethodDelete, apiKeysPath+"/"+tc.keyID, "")

			assert.Equal(t, tc.wantCode, rec.Code, rec.Body.String())
		})
	}
}
