package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// newBilledSignInDeps is the cloud's sign-in: the user service also follows the billing rules.
func newBilledSignInDeps(t *testing.T, access *dto.OrganizationAccess) signupDeps {
	t.Helper()
	gin.SetMode(gin.TestMode)
	d := signupDeps{
		signInDeps: signInDeps{
			userRepo: mocks.NewMockUserRepository(t), roleRepo: mocks.NewMockRoleRepository(t),
			userRoleRepo: mocks.NewMockUserRoleRepository(t), orgRepo: mocks.NewMockOrganizationRepository(t),
		},
		google: mocks.NewMockGoogleIdentityVerifier(t),
	}
	reader := mocks.NewMockOrganizationAccessReader(t)
	reader.EXPECT().GetAccess(mock.Anything, testOrganizationID).Return(access, nil).Maybe()
	scope := tenant.AnyOrganization()
	users := service.NewUserService(d.userRepo, d.roleRepo, d.userRoleRepo, d.orgRepo, service.NewJWTService("test-secret", scope), immediateUnitOfWork{}, scope)
	users.CheckSubscriptions(service.NewOrganizationAccessService(reader))
	d.router = gin.New()
	d.router.POST("/api/auth/signin", NewUserHandler(users, nil, slog.New(slog.DiscardHandler)).SignInHandler)
	d.router.POST("/api/auth/google", NewSignupHandler(
		service.NewSignupService(d.google, users, d.orgRepo, d.userRepo, d.userRoleRepo, immediateUnitOfWork{}), slog.New(slog.DiscardHandler)).GoogleSignInHandler)
	return d
}

// The organization-sign-in delta: an ended trial and a missed payment are told to a person with
// the right password, and only an admin gets the setup token the recovery routes accept.
func TestUserHandler_SignIn_StoppedOrganization(t *testing.T) {
	day := 24 * time.Hour
	at := func(d time.Duration) *time.Time { v := time.Now().Add(d); return &v }
	expired := &dto.OrganizationAccess{Status: "trial", Plan: "full", TrialEndsAt: at(-day)}
	unpaid := &dto.OrganizationAccess{Status: "active", Plan: "full", PaidUntil: at(-4 * day), HasCard: true}
	inGrace := &dto.OrganizationAccess{Status: "trial", Plan: "full", TrialEndsAt: at(-day), HasCard: true}
	cancelled := &dto.OrganizationAccess{Status: "active", Plan: "full", PaidUntil: at(-day), HasCard: true, CancelledAt: at(-9 * day)}

	cases := []struct {
		name      string
		access    *dto.OrganizationAccess
		roleID    int
		password  string
		code      int
		wantCode  string
		wantError string
		wantToken bool
	}{
		{"expired trial, admin", expired, permissions.RoleAdmin, "password123", http.StatusForbidden, "trial_expired", "Trial ended", true},
		{"expired trial, manager", expired, permissions.RoleManager, "password123", http.StatusForbidden, "trial_expired", "Trial ended", false},
		{"expired trial, wrong password", expired, permissions.RoleAdmin, "wrong-password", http.StatusUnauthorized, "", "Invalid email or password", false},
		{"unpaid, admin", unpaid, permissions.RoleAdmin, "password123", http.StatusForbidden, "payment_required", "Payment required", true},
		{"unpaid, manager", unpaid, permissions.RoleManager, "password123", http.StatusForbidden, "payment_required", "Payment required", false},
		{"unpaid, wrong password", unpaid, permissions.RoleAdmin, "wrong-password", http.StatusUnauthorized, "", "Invalid email or password", false},
		{"cancelled and past its period, admin", cancelled, permissions.RoleAdmin, "password123", http.StatusForbidden, "payment_required", "Payment required", true},
		{"trial in grace with a saved card", inGrace, permissions.RoleAdmin, "password123", http.StatusOK, "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newBilledSignInDeps(t, tc.access)
			d.userRepo.EXPECT().FindByEmail(mock.Anything, "ana@example.com").Return(&dto.UserInOrganization{User: hashedUser(t), OrganizationID: testOrganizationID}, nil)
			d.orgRepo.EXPECT().Get(mock.Anything, testOrganizationID).Return(
				&dto.Organization{ID: testOrganizationID, Status: tc.access.Status, TrialEndsAt: tc.access.TrialEndsAt}, nil)
			d.userRoleRepo.EXPECT().FindRolesByUserID(mock.Anything, "ana-id").Return([]*dto.Role{{ID: tc.roleID, Name: "r"}}, nil).Maybe()

			rec := d.do(http.MethodPost, "/api/auth/signin", `{"email":"ana@example.com","password":"`+tc.password+`"}`)

			require.Equal(t, tc.code, rec.Code, rec.Body.String())
			got := decodeBody(t, rec)
			if tc.code == http.StatusOK {
				assert.Contains(t, got, "token")
				return
			}
			assert.NotContains(t, got, "token", "a refusal never carries a sign-in token")
			assert.JSONEq(t, `"`+tc.wantError+`"`, string(got["error"]))
			if tc.wantCode != "" {
				assert.JSONEq(t, `"`+tc.wantCode+`"`, string(got["code"]))
			}
			if !tc.wantToken {
				assert.NotContains(t, got, "setup_token")
				return
			}
			var token string
			require.Contains(t, got, "setup_token")
			require.NoError(t, json.Unmarshal(got["setup_token"], &token))
			assertSetupToken(t, token)
		})
	}
}

func TestSignupHandler_GoogleSignIn_StoppedOrganization(t *testing.T) {
	longAgo := time.Now().Add(-30 * 24 * time.Hour)
	unpaid := &dto.OrganizationAccess{Status: "active", Plan: "full", PaidUntil: &longAgo, HasCard: true}
	cases := []struct {
		name      string
		roleID    int
		wantToken bool
	}{
		{"admin", permissions.RoleAdmin, true},
		{"manager", permissions.RoleManager, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newBilledSignInDeps(t, unpaid)
			d.googleSays("ana@example.com")
			d.anaIs(t, &dto.Organization{ID: testOrganizationID, Status: "active"}, tc.roleID, true)

			rec := d.do(http.MethodPost, "/api/auth/google", `{"id_token":"tok"}`)

			require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
			got := decodeBody(t, rec)
			assert.JSONEq(t, `"payment_required"`, string(got["code"]))
			assert.NotContains(t, got, "token")
			assert.Equal(t, tc.wantToken, got["setup_token"] != nil)
		})
	}
}

func TestJWTAuthMiddleware_OrganizationAccess_PaymentRequired(t *testing.T) {
	day := 24 * time.Hour
	at := func(d time.Duration) *time.Time { v := time.Now().Add(d); return &v }
	cases := []struct {
		name     string
		access   dto.OrganizationAccess
		wantCode string
	}{
		{"renewal declined, within grace", dto.OrganizationAccess{Status: "active", Plan: "full", PaidUntil: at(-2 * day), HasCard: true}, ""},
		{"grace over", dto.OrganizationAccess{Status: "active", Plan: "full", PaidUntil: at(-4 * day), HasCard: true}, "payment_required"},
		{"cancelled, a day after the period", dto.OrganizationAccess{Status: "active", Plan: "full", PaidUntil: at(-day), HasCard: true, CancelledAt: at(-13 * day)}, "payment_required"},
		{"trial with a card on day 15", dto.OrganizationAccess{Status: "trial", Plan: "full", TrialEndsAt: at(-day), HasCard: true}, ""},
		{"trial with a card on day 18", dto.OrganizationAccess{Status: "trial", Plan: "full", TrialEndsAt: at(-4 * day), HasCard: true}, "trial_expired"},
		{"active by hand never stops", dto.OrganizationAccess{Status: "active", Plan: "full", TrialEndsAt: at(-400 * day)}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			access := tc.access
			jwt, _ := cloudJWT(t, &accessStub{access: &access})

			rec := serveRoute(t, jwt, "/api/products", managerToken(t))

			if tc.wantCode == "" {
				require.Equal(t, http.StatusOK, rec.Code)
				return
			}
			require.Equal(t, http.StatusForbidden, rec.Code)
			assert.Equal(t, tc.wantCode, body(t, rec)["code"])
		})
	}
}
