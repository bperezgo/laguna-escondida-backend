package handler

import (
	"encoding/json"
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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// pendingWithSetupToken marks a case whose body carries a setup token, which no fixed JSON can match.
const pendingWithSetupToken = "pending with setup token"

func assertSetupToken(t *testing.T, token string) {
	t.Helper()
	jwtService := service.NewJWTService("test-secret", tenant.AnyOrganization())
	claims, err := jwtService.ValidateSetupToken(token)
	require.NoError(t, err)
	assert.NotEmpty(t, claims.UserID)
	assert.NotEmpty(t, claims.OrgID)
	_, err = jwtService.ValidateToken(token)
	assert.Error(t, err, "a setup token is not a sign-in")
}

func assertPendingWithSetupToken(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "Approval pending", body["error"])
	assert.Equal(t, "pending_approval", body["code"])
	assert.NotContains(t, body, "token")
	assertSetupToken(t, body["setup_token"])
}

// trialExpiredWithSetupToken marks an expired-trial case for an admin, whose body carries a setup token.
const trialExpiredWithSetupToken = "trial expired with setup token"

func assertTrialExpiredWithSetupToken(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "Trial ended", body["error"])
	assert.Equal(t, "trial_expired", body["code"])
	assert.NotContains(t, body, "token")
	assertSetupToken(t, body["setup_token"])
}

type signupDeps struct {
	signInDeps
	google *mocks.MockGoogleIdentityVerifier
}

func newSignupDeps(t *testing.T) signupDeps {
	t.Helper()
	d := signupDeps{signInDeps: newSignInDeps(t, tenant.AnyOrganization()), google: mocks.NewMockGoogleIdentityVerifier(t)}
	users := service.NewUserService(d.userRepo, d.roleRepo, d.userRoleRepo, d.orgRepo, service.NewJWTService("test-secret", tenant.AnyOrganization()), immediateUnitOfWork{}, tenant.AnyOrganization())
	h := NewSignupHandler(service.NewSignupService(d.google, users, d.orgRepo, d.userRepo, d.userRoleRepo, immediateUnitOfWork{}), slog.New(slog.DiscardHandler))
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
		}, http.StatusForbidden, trialExpiredWithSetupToken},
		{"pending organization", func(t *testing.T, d signupDeps) {
			d.googleSays("ana@example.com")
			d.anaIs(t, &dto.Organization{ID: testOrganizationID, Status: "pending"}, permissions.RoleAdmin, true)
		}, http.StatusForbidden, pendingWithSetupToken},
		{"pending organization, not an admin", func(t *testing.T, d signupDeps) {
			d.googleSays("ana@example.com")
			d.anaIs(t, &dto.Organization{ID: testOrganizationID, Status: "pending"}, permissions.RoleManager, true)
		}, http.StatusForbidden, `{"error":"Approval pending","code":"pending_approval"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newSignupDeps(t)
			tc.setup(t, d)

			rec := d.do(http.MethodPost, "/api/auth/google", `{"id_token":"tok"}`)

			require.Equal(t, tc.code, rec.Code, rec.Body.String())
			if tc.want == pendingWithSetupToken {
				assertPendingWithSetupToken(t, rec)
				return
			}
			if tc.want == trialExpiredWithSetupToken {
				assertTrialExpiredWithSetupToken(t, rec)
				return
			}
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

	rec := d.do(http.MethodPost, "/api/signup/google", `{"id_token":"tok","restaurant_name":"Doña Rosa","plan":"starter"}`)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var body struct {
		Status           string `json:"status"`
		OrganizationName string `json:"organization_name"`
		Plan             string `json:"plan"`
		SetupToken       string `json:"setup_token"`
		Token            string `json:"token"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, []string{"pending", "Doña Rosa", "starter"}, []string{body.Status, body.OrganizationName, body.Plan})
	assert.Empty(t, body.Token, "no sign-in token")
	assertSetupToken(t, body.SetupToken)
	assert.Empty(t, rec.Header().Values("Set-Cookie"))
}

func TestSignupHandler_GoogleSignup_AgainWhilePending(t *testing.T) {
	d := newSignupDeps(t)
	d.googleSays("ana@example.com")
	d.anaIs(t, &dto.Organization{ID: testOrganizationID, Status: "pending"}, permissions.RoleAdmin, true)

	rec := d.do(http.MethodPost, "/api/signup/google", `{"id_token":"tok","restaurant_name":"Doña Rosa","plan":"starter"}`)

	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assertPendingWithSetupToken(t, rec)
	d.orgRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
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
