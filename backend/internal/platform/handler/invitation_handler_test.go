package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
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

type invitationDeps struct {
	invitations *mocks.MockInvitationRepository
	users       *mocks.MockUserRepository
	userRoles   *mocks.MockUserRoleRepository
	roles       *mocks.MockRoleRepository
	orgs        *mocks.MockOrganizationRepository
	access      *mocks.MockOrganizationAccessReader
	router      *gin.Engine
}

func newInvitationDeps(t *testing.T, inviterRoles []int) invitationDeps {
	t.Helper()
	gin.SetMode(gin.TestMode)
	d := invitationDeps{
		invitations: mocks.NewMockInvitationRepository(t), users: mocks.NewMockUserRepository(t),
		userRoles: mocks.NewMockUserRoleRepository(t), roles: mocks.NewMockRoleRepository(t),
		orgs: mocks.NewMockOrganizationRepository(t), access: mocks.NewMockOrganizationAccessReader(t),
	}
	jwt := service.NewJWTService("test-secret", tenant.AnyOrganization())
	users := service.NewUserService(d.users, d.roles, d.userRoles, d.orgs, jwt, immediateUnitOfWork{}, tenant.AnyOrganization())
	svc := service.NewInvitationService(d.invitations, d.users, d.userRoles, d.roles, d.orgs, service.NewOrganizationAccessService(d.access),
		users, nil, immediateUnitOfWork{}, "https://app.example.com")
	h := NewInvitationHandler(svc, slog.New(slog.DiscardHandler))
	d.router = gin.New()
	signedIn := func(c *gin.Context) {
		c.Set("user_id", "inviter-id")
		c.Set("role_ids", inviterRoles)
		c.Request = c.Request.WithContext(tenant.WithOrg(c.Request.Context(), testOrganizationID))
	}
	d.router.POST("/api/admin/invitations", signedIn, h.CreateHandler)
	d.router.GET("/api/admin/invitations", signedIn, h.ListHandler)
	d.router.POST("/api/admin/invitations/:id/link", signedIn, h.NewLinkHandler)
	d.router.DELETE("/api/admin/invitations/:id", signedIn, h.RevokeHandler)
	d.router.GET("/api/invitations/:token", h.LookupHandler)
	d.router.POST("/api/invitations/:token/accept", h.AcceptHandler)
	return d
}

func (d invitationDeps) do(method, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	d.router.ServeHTTP(rec, req)
	return rec
}

func (d invitationDeps) roleExists(id int) {
	d.roles.EXPECT().FindByIDs(mock.Anything, []int{id}).Return([]*dto.Role{{ID: id, Name: "role"}}, nil).Maybe()
}

func pendingInvitation() *dto.Invitation {
	return &dto.Invitation{ID: "019a0000-0000-7000-8000-0000000000c1", OrganizationID: testOrganizationID, Email: "ana@example.com",
		Name: "Ana", RoleIDs: []int{permissions.RoleAccountant}, ExpiresAt: time.Now().Add(time.Hour), CreatedAt: time.Now()}
}

func linkToken(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body dto.InvitationWithLink
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, strings.HasPrefix(body.Link, "https://app.example.com/invitacion/"), body.Link)
	return strings.TrimPrefix(body.Link, "https://app.example.com/invitacion/")
}

func TestInvitationHandler_CreateReturnsTheLinkOnce(t *testing.T) {
	d := newInvitationDeps(t, []int{permissions.RoleAdmin})
	d.roleExists(permissions.RoleAccountant)
	d.users.EXPECT().FindByEmail(mock.Anything, "contadora@example.com").Return(nil, domainError.ErrUserNotFound)
	d.invitations.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(nil)
	d.invitations.EXPECT().ListPending(mock.Anything).Return([]*dto.Invitation{pendingInvitation()}, nil)

	rec := d.do(http.MethodPost, "/api/admin/invitations", `{"email":"contadora@example.com","name":"Contadora","role_ids":[5]}`)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	token := linkToken(t, rec)
	assert.NotEmpty(t, token)

	rec = d.do(http.MethodGet, "/api/admin/invitations", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), "link")
	assert.NotContains(t, rec.Body.String(), token)
}

func TestInvitationHandler_CreateRefusals(t *testing.T) {
	cases := []struct {
		name   string
		roles  []int
		body   string
		setup  func(d invitationDeps)
		status int
	}{
		{"manager invites an admin", []int{permissions.RoleManager}, `{"email":"a@example.com","name":"A","role_ids":[2]}`, nil, http.StatusForbidden},
		{"agent role", []int{permissions.RoleAdmin}, `{"email":"a@example.com","name":"A","role_ids":[6]}`, nil, http.StatusBadRequest},
		{"email has an account", []int{permissions.RoleAdmin}, `{"email":"a@example.com","name":"A","role_ids":[3]}`, func(d invitationDeps) {
			d.roleExists(permissions.RoleManager)
			d.users.EXPECT().FindByEmail(mock.Anything, "a@example.com").Return(&dto.UserInOrganization{User: &dto.User{}}, nil)
		}, http.StatusConflict},
		{"pending invitation", []int{permissions.RoleAdmin}, `{"email":"a@example.com","name":"A","role_ids":[3]}`, func(d invitationDeps) {
			d.roleExists(permissions.RoleManager)
			d.users.EXPECT().FindByEmail(mock.Anything, "a@example.com").Return(nil, domainError.ErrUserNotFound)
			d.invitations.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(domainError.ErrInvitationPending)
		}, http.StatusConflict},
		{"malformed body", []int{permissions.RoleAdmin}, `{`, nil, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newInvitationDeps(t, tc.roles)
			if tc.setup != nil {
				tc.setup(d)
			}
			assert.Equal(t, tc.status, d.do(http.MethodPost, "/api/admin/invitations", tc.body).Code)
		})
	}
}

func TestInvitationHandler_NewLinkAndRevoke(t *testing.T) {
	d := newInvitationDeps(t, []int{permissions.RoleAdmin})
	inv := pendingInvitation()
	d.roleExists(permissions.RoleAccountant)
	d.invitations.EXPECT().FindPending(mock.Anything, inv.ID).Return(inv, nil)
	d.invitations.EXPECT().ReplaceToken(mock.Anything, inv.ID, mock.Anything, mock.Anything).Return(nil)
	d.invitations.EXPECT().FindPending(mock.Anything, "019a0000-0000-7000-8000-0000000000ff").Return(nil, domainError.ErrInvitationNotFound)
	d.invitations.EXPECT().Revoke(mock.Anything, inv.ID, mock.Anything).Return(nil)
	d.invitations.EXPECT().Revoke(mock.Anything, "019a0000-0000-7000-8000-0000000000ff", mock.Anything).Return(domainError.ErrInvitationNotFound)

	rec := d.do(http.MethodPost, "/api/admin/invitations/"+inv.ID+"/link", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotEmpty(t, linkToken(t, rec))
	assert.Equal(t, http.StatusNotFound, d.do(http.MethodPost, "/api/admin/invitations/019a0000-0000-7000-8000-0000000000ff/link", "").Code)
	assert.Equal(t, http.StatusNoContent, d.do(http.MethodDelete, "/api/admin/invitations/"+inv.ID, "").Code)
	assert.Equal(t, http.StatusNotFound, d.do(http.MethodDelete, "/api/admin/invitations/019a0000-0000-7000-8000-0000000000ff", "").Code)
}

func TestInvitationHandler_LookupAndAccept(t *testing.T) {
	d := newInvitationDeps(t, nil)
	inv := pendingInvitation()
	d.invitations.EXPECT().FindByTokenHash(mock.Anything, mock.Anything, mock.Anything).Return(inv, nil)
	d.orgs.EXPECT().Get(mock.Anything, testOrganizationID).Return(&dto.Organization{ID: testOrganizationID, Name: "Doña Rosa"}, nil)
	d.roleExists(permissions.RoleAccountant)
	d.access.EXPECT().GetAccess(mock.Anything, testOrganizationID).Return(&dto.OrganizationAccess{Status: "active", Plan: "full"}, nil)
	d.users.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)
	d.userRoles.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)
	d.invitations.EXPECT().MarkAccepted(mock.Anything, inv.ID, mock.Anything).Return(nil)

	rec := d.do(http.MethodGet, "/api/invitations/tok", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `"Doña Rosa"`, string(decodeBody(t, rec)["organization_name"]))
	assert.NotContains(t, rec.Body.String(), "link")

	rec = d.do(http.MethodPost, "/api/invitations/tok/accept", `{"password":"secreto123"}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Contains(t, decodeBody(t, rec), "token")
}

func TestInvitationHandler_AcceptRefusals(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	cases := []struct {
		name   string
		body   string
		setup  func(d invitationDeps)
		status int
		code   string
	}{
		{"expired link", `{"password":"secreto123"}`, func(d invitationDeps) {
			inv := pendingInvitation()
			inv.ExpiresAt = past
			d.invitations.EXPECT().FindByTokenHash(mock.Anything, mock.Anything, true).Return(inv, nil)
		}, http.StatusBadRequest, "invalid_or_expired_link"},
		{"organization trial ended", `{"password":"secreto123"}`, func(d invitationDeps) {
			d.invitations.EXPECT().FindByTokenHash(mock.Anything, mock.Anything, true).Return(pendingInvitation(), nil)
			d.access.EXPECT().GetAccess(mock.Anything, testOrganizationID).Return(&dto.OrganizationAccess{Status: "trial", TrialEndsAt: &past}, nil)
		}, http.StatusForbidden, "organization_inactive"},
		{"email taken since", `{"password":"secreto123"}`, func(d invitationDeps) {
			d.invitations.EXPECT().FindByTokenHash(mock.Anything, mock.Anything, true).Return(pendingInvitation(), nil)
			d.access.EXPECT().GetAccess(mock.Anything, testOrganizationID).Return(&dto.OrganizationAccess{Status: "active", Plan: "full"}, nil)
			d.users.EXPECT().Create(mock.Anything, mock.Anything).Return(domainError.ErrUserEmailTaken)
		}, http.StatusConflict, ""},
		{"short password", `{"password":"short"}`, nil, http.StatusBadRequest, ""},
		{"google without a client id", `{"id_token":"t"}`, nil, http.StatusBadRequest, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newInvitationDeps(t, nil)
			if tc.setup != nil {
				tc.setup(d)
			}
			rec := d.do(http.MethodPost, "/api/invitations/tok/accept", tc.body)
			require.Equal(t, tc.status, rec.Code, rec.Body.String())
			if tc.code != "" {
				assert.JSONEq(t, `"`+tc.code+`"`, string(decodeBody(t, rec)["code"]))
			}
		})
	}
}
