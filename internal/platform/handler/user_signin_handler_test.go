package handler

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	useragg "laguna-escondida/backend/internal/domain/aggregate/user"
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

type signInDeps struct {
	userRepo     *mocks.MockUserRepository
	roleRepo     *mocks.MockRoleRepository
	userRoleRepo *mocks.MockUserRoleRepository
	orgRepo      *mocks.MockOrganizationRepository
	router       *gin.Engine
}

func newSignInDeps(t *testing.T, scope tenant.Scope) signInDeps {
	t.Helper()
	gin.SetMode(gin.TestMode)
	d := signInDeps{
		userRepo:     mocks.NewMockUserRepository(t),
		roleRepo:     mocks.NewMockRoleRepository(t),
		userRoleRepo: mocks.NewMockUserRoleRepository(t),
		orgRepo:      mocks.NewMockOrganizationRepository(t),
	}
	userService := service.NewUserService(d.userRepo, d.roleRepo, d.userRoleRepo, d.orgRepo, service.NewJWTService("test-secret", scope), immediateUnitOfWork{}, scope)
	h := NewUserHandler(userService, nil, slog.New(slog.DiscardHandler))
	d.router = gin.New()
	d.router.POST("/api/auth/signin", h.SignInHandler)
	d.router.GET("/api/auth/context", h.SignInContextHandler)
	d.router.POST("/api/admin/users", func(c *gin.Context) {
		c.Request = c.Request.WithContext(tenant.WithOrg(c.Request.Context(), testOrganizationID))
		h.CreateUserHandler(c)
	})
	return d
}

func (d signInDeps) do(method, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	d.router.ServeHTTP(rec, req)
	return rec
}

func activeOrg() *dto.Organization {
	return &dto.Organization{ID: testOrganizationID, Name: "Laguna Escondida", Status: dto.OrganizationStatusActive}
}

func hashedUser(t *testing.T) *dto.User {
	t.Helper()
	hashed, err := useragg.HashPassword("password123")
	require.NoError(t, err)
	return &dto.User{ID: "ana-id", Username: "ana", Password: hashed, Active: true}
}

func TestUserHandler_SignIn_CloudByEmail(t *testing.T) {
	d := newSignInDeps(t, tenant.AnyOrganization())
	d.userRepo.EXPECT().FindByEmail(mock.Anything, "ana@example.com").Return(&dto.UserInOrganization{User: hashedUser(t), OrganizationID: testOrganizationID}, nil)
	d.orgRepo.EXPECT().Get(mock.Anything, testOrganizationID).Return(activeOrg(), nil)
	d.userRoleRepo.EXPECT().FindRolesByUserID(mock.Anything, "ana-id").Return([]*dto.Role{{ID: permissions.RoleManager, Name: "manager"}}, nil)

	rec := d.do(http.MethodPost, "/api/auth/signin", `{"email": "ANA@Example.com ", "password": "password123"}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	body := decodeBody(t, rec)
	for _, field := range []string{"token", "username", "roles", "permissions"} {
		assert.Contains(t, body, field)
	}
}

func TestUserHandler_SignIn_CloudRejections(t *testing.T) {
	cases := []struct {
		name  string
		body  string
		setup func(t *testing.T, d signInDeps)
		code  int
		want  string
	}{
		{name: "missing email", body: `{"username": "ana", "password": "password123"}`, code: http.StatusBadRequest, want: `{"error": "email is required"}`},
		{
			name: "unknown email", body: `{"email": "nobody@example.com", "password": "password123"}`,
			setup: func(t *testing.T, d signInDeps) {
				d.userRepo.EXPECT().FindByEmail(mock.Anything, "nobody@example.com").Return(nil, domainError.ErrUserNotFound)
			},
			code: http.StatusUnauthorized, want: `{"error": "Invalid email or password"}`,
		},
		{
			name: "server signs in to the cloud", body: `{"email": "ana@example.com", "password": "password123"}`,
			setup: func(t *testing.T, d signInDeps) {
				d.userRepo.EXPECT().FindByEmail(mock.Anything, "ana@example.com").Return(&dto.UserInOrganization{User: hashedUser(t), OrganizationID: testOrganizationID}, nil)
				d.orgRepo.EXPECT().Get(mock.Anything, testOrganizationID).Return(activeOrg(), nil)
				d.userRoleRepo.EXPECT().FindRolesByUserID(mock.Anything, "ana-id").Return([]*dto.Role{{ID: permissions.RoleServer, Name: "server"}}, nil)
			},
			code: http.StatusUnauthorized, want: `{"error": "Invalid email or password"}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newSignInDeps(t, tenant.AnyOrganization())
			if tc.setup != nil {
				tc.setup(t, d)
			}

			rec := d.do(http.MethodPost, "/api/auth/signin", tc.body)

			assert.Equal(t, tc.code, rec.Code)
			assert.JSONEq(t, tc.want, rec.Body.String())
		})
	}
}

func TestUserHandler_SignIn_EdgeByUsername(t *testing.T) {
	d := newSignInDeps(t, tenant.PinnedTo(testOrganizationID))
	d.orgRepo.EXPECT().Get(mock.Anything, testOrganizationID).Return(activeOrg(), nil)
	d.userRepo.EXPECT().FindByUsername(mock.Anything, "ana").Return(hashedUser(t), nil)
	d.userRoleRepo.EXPECT().FindRolesByUserID(mock.Anything, "ana-id").Return([]*dto.Role{{ID: permissions.RoleServer, Name: "server"}}, nil)

	rec := d.do(http.MethodPost, "/api/auth/signin", `{"username": "ana", "password": "password123"}`)

	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestUserHandler_SignIn_EdgeRejections(t *testing.T) {
	cases := []struct {
		name  string
		body  string
		setup func(d signInDeps)
		code  int
		want  string
	}{
		{name: "missing username", body: `{"email": "ana@example.com", "password": "password123"}`, code: http.StatusBadRequest, want: `{"error": "username is required"}`},
		{
			name: "username of another organization", body: `{"username": "ana", "password": "password123"}`,
			setup: func(d signInDeps) {
				d.orgRepo.EXPECT().Get(mock.Anything, testOrganizationID).Return(activeOrg(), nil)
				d.userRepo.EXPECT().FindByUsername(mock.Anything, "ana").Return(nil, domainError.ErrUserNotFound)
			},
			code: http.StatusUnauthorized, want: `{"error": "Invalid username or password"}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newSignInDeps(t, tenant.PinnedTo(testOrganizationID))
			if tc.setup != nil {
				tc.setup(d)
			}

			rec := d.do(http.MethodPost, "/api/auth/signin", tc.body)

			assert.Equal(t, tc.code, rec.Code)
			assert.JSONEq(t, tc.want, rec.Body.String())
		})
	}
}

func TestUserHandler_SignInContext(t *testing.T) {
	t.Run("cloud", func(t *testing.T) {
		d := newSignInDeps(t, tenant.AnyOrganization())

		rec := d.do(http.MethodGet, "/api/auth/context", "")

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"mode": "cloud"}`, rec.Body.String())
	})
	t.Run("edge", func(t *testing.T) {
		d := newSignInDeps(t, tenant.PinnedTo(testOrganizationID))
		d.orgRepo.EXPECT().Get(mock.Anything, testOrganizationID).Return(activeOrg(), nil)

		rec := d.do(http.MethodGet, "/api/auth/context", "")

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"mode": "edge", "organization": {"name": "Laguna Escondida"}}`, rec.Body.String())
	})
}

func TestUserHandler_CreateUser_EmailErrors(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		createErr error
		code      int
		want      string
	}{
		{
			name: "manager without an email",
			body: `{"username": "ana", "name": "Ana", "password": "password123", "role_ids": [3]}`,
			code: http.StatusBadRequest, want: `{"error": "email is required for a user with a cloud role"}`,
		},
		{
			name:      "email taken",
			body:      `{"username": "ana", "name": "Ana", "email": "ana@example.com", "password": "password123", "role_ids": [3]}`,
			createErr: domainError.ErrUserEmailTaken,
			code:      http.StatusConflict, want: `{"error": "Email is already in use"}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newSignInDeps(t, tenant.AnyOrganization())
			d.userRepo.EXPECT().FindByUsername(mock.Anything, "ana").Return(nil, domainError.ErrUserNotFound)
			d.roleRepo.EXPECT().FindByIDs(mock.Anything, []int{permissions.RoleManager}).Return([]*dto.Role{{ID: permissions.RoleManager, Name: "manager"}}, nil)
			if tc.createErr != nil {
				d.userRepo.EXPECT().Create(mock.Anything, mock.AnythingOfType("*dto.User")).Return(tc.createErr)
			}

			rec := d.do(http.MethodPost, "/api/admin/users", tc.body)

			assert.Equal(t, tc.code, rec.Code)
			assert.JSONEq(t, tc.want, rec.Body.String())
		})
	}
}
