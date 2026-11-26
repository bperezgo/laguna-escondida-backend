package handler

import (
	"log/slog"
	"net/http"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type signupDeps struct {
	signInDeps
	google *mocks.MockGoogleIdentityVerifier
}

func newSignupDeps(t *testing.T) signupDeps {
	t.Helper()
	d := signupDeps{signInDeps: newSignInDeps(t, tenant.AnyOrganization()), google: mocks.NewMockGoogleIdentityVerifier(t)}
	users := service.NewUserService(d.userRepo, d.roleRepo, d.userRoleRepo, d.orgRepo, service.NewJWTService("test-secret", tenant.AnyOrganization()), immediateUnitOfWork{}, tenant.AnyOrganization())
	h := NewSignupHandler(service.NewSignupService(d.google, users, d.orgRepo, d.userRepo, d.roleRepo, d.userRoleRepo, immediateUnitOfWork{}), slog.New(slog.DiscardHandler))
	d.router.POST("/api/auth/google", h.GoogleSignInHandler)
	d.router.POST("/api/signup/google", h.GoogleSignupHandler)
	return d
}

func (d signupDeps) googleSays(email string) {
	d.google.EXPECT().Verify(mock.Anything, "tok").Return(&dto.GoogleIdentity{Email: email, Name: "Ana"}, nil)
}

func (d signupDeps) anaIs(t *testing.T, org *dto.Organization, roleID int, active bool) {
	u := hashedUser(t)
	u.Active = active
	d.userRepo.EXPECT().FindByEmail(mock.Anything, "ana@example.com").Return(&dto.UserInOrganization{User: u, OrganizationID: testOrganizationID}, nil)
	d.orgRepo.EXPECT().Get(mock.Anything, testOrganizationID).Return(org, nil)
	d.userRoleRepo.EXPECT().FindRolesByUserID(mock.Anything, "ana-id").Return([]*dto.Role{{ID: roleID, Name: "r"}}, nil).Maybe()
}

func TestSignupHandler_GoogleSignIn(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	failed := `{"error":"Google sign-in failed"}`
	cases := []struct {
		name  string
		setup func(t *testing.T, d signupDeps)
		code  int
		want  string
	}{
		{"google signup admin", func(t *testing.T, d signupDeps) {
			d.googleSays("ana@example.com")
			d.anaIs(t, activeOrg(), permissions.RoleAdmin, true)
		}, http.StatusOK, ""},
		{"password user with the same email", func(t *testing.T, d signupDeps) {
			d.googleSays("ANA@example.com")
			d.anaIs(t, activeOrg(), permissions.RoleManager, true)
		}, http.StatusOK, ""},
		{"unverified or invalid token", func(t *testing.T, d signupDeps) {
			d.google.EXPECT().Verify(mock.Anything, "tok").Return(nil, domainError.ErrGoogleTokenInvalid)
		}, http.StatusUnauthorized, failed},
		{"no account", func(t *testing.T, d signupDeps) {
			d.googleSays("nobody@example.com")
			d.userRepo.EXPECT().FindByEmail(mock.Anything, "nobody@example.com").Return(nil, domainError.ErrUserNotFound)
		}, http.StatusUnauthorized, failed},
		{"suspended organization", func(t *testing.T, d signupDeps) {
			d.googleSays("ana@example.com")
			d.anaIs(t, &dto.Organization{ID: testOrganizationID, Status: "suspended"}, permissions.RoleAdmin, true)
		}, http.StatusUnauthorized, failed},
		{"inactive user", func(t *testing.T, d signupDeps) {
			d.googleSays("ana@example.com")
			d.anaIs(t, activeOrg(), permissions.RoleAdmin, false)
		}, http.StatusUnauthorized, failed},
		{"no cloud role", func(t *testing.T, d signupDeps) {
			d.googleSays("ana@example.com")
			d.anaIs(t, activeOrg(), permissions.RoleServer, true)
		}, http.StatusUnauthorized, failed},
		{"expired trial", func(t *testing.T, d signupDeps) {
			d.googleSays("ana@example.com")
			d.anaIs(t, &dto.Organization{ID: testOrganizationID, Status: "trial", TrialEndsAt: &past}, permissions.RoleAdmin, true)
		}, http.StatusForbidden, `{"error":"Trial ended","code":"trial_expired"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newSignupDeps(t)
			tc.setup(t, d)

			rec := d.do(http.MethodPost, "/api/auth/google", `{"id_token":"tok"}`)

			require.Equal(t, tc.code, rec.Code, rec.Body.String())
			if tc.want != "" {
				assert.JSONEq(t, tc.want, rec.Body.String())
				return
			}
			for _, field := range []string{"token", "username", "roles", "permissions"} {
				assert.Contains(t, decodeBody(t, rec), field)
			}
		})
	}
}

func TestSignupHandler_GoogleSignIn_MissingToken(t *testing.T) {
	assert.Equal(t, http.StatusBadRequest, newSignupDeps(t).do(http.MethodPost, "/api/auth/google", `{}`).Code)
}

func TestSignupHandler_GoogleSignup_Created(t *testing.T) {
	d := newSignupDeps(t)
	d.googleSays("rosa@example.com")
	d.userRepo.EXPECT().FindByEmail(mock.Anything, "rosa@example.com").Return(nil, domainError.ErrUserNotFound)
	d.orgRepo.EXPECT().GetBySlug(mock.Anything, "dona-rosa").Return(nil, domainError.ErrOrganizationNotFound)
	d.orgRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)
	d.userRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Times(2)
	d.userRoleRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Times(2)
	d.roleRepo.EXPECT().FindByIDs(mock.Anything, []int{permissions.RoleAdmin}).Return([]*dto.Role{{ID: permissions.RoleAdmin, Name: "admin"}}, nil)

	rec := d.do(http.MethodPost, "/api/signup/google", `{"id_token":"tok","restaurant_name":"Doña Rosa","plan":"starter"}`)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Contains(t, decodeBody(t, rec), "token")
}

func TestSignupHandler_GoogleSignup_InvalidField(t *testing.T) {
	d := newSignupDeps(t)

	rec := d.do(http.MethodPost, "/api/signup/google", `{"id_token":"tok","restaurant_name":"Casa","plan":"premium"}`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `"plan"`, string(decodeBody(t, rec)["field"]))
	rec = d.do(http.MethodPost, "/api/signup/google", `{"id_token":"tok","restaurant_name":"  ","plan":"full"}`)
	assert.JSONEq(t, `"restaurant_name"`, string(decodeBody(t, rec)["field"]))
}

func TestSignupHandler_GoogleSignup_BadTokenIs401(t *testing.T) {
	d := newSignupDeps(t)
	d.google.EXPECT().Verify(mock.Anything, "tok").Return(nil, domainError.ErrGoogleTokenInvalid)

	rec := d.do(http.MethodPost, "/api/signup/google", `{"id_token":"tok","restaurant_name":"Casa","plan":"full"}`)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	d.orgRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}
