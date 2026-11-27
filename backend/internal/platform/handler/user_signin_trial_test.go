package handler

import (
	"net/http"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestUserHandler_SignIn_Trial(t *testing.T) {
	past, future := time.Now().Add(-24*time.Hour), time.Now().Add(5*24*time.Hour)
	cases := []struct {
		name     string
		org      *dto.Organization
		password string
		code     int
		want     string
	}{
		{"expired trial, correct password", &dto.Organization{ID: testOrganizationID, Status: "trial", TrialEndsAt: &past}, "password123",
			http.StatusForbidden, `{"error":"Trial ended","code":"trial_expired"}`},
		{"expired trial, wrong password", &dto.Organization{ID: testOrganizationID, Status: "trial", TrialEndsAt: &past}, "wrong-password",
			http.StatusUnauthorized, `{"error":"Invalid email or password"}`},
		{"suspended", &dto.Organization{ID: testOrganizationID, Status: "suspended"}, "password123",
			http.StatusUnauthorized, `{"error":"Invalid email or password"}`},
		{"pending, correct password", &dto.Organization{ID: testOrganizationID, Status: "pending"}, "password123",
			http.StatusForbidden, pendingWithSetupToken},
		{"pending, wrong password", &dto.Organization{ID: testOrganizationID, Status: "pending"}, "wrong-password",
			http.StatusUnauthorized, `{"error":"Invalid email or password"}`},
		{"active", &dto.Organization{ID: testOrganizationID, Status: "active"}, "password123", http.StatusOK, ""},
		{"in trial", &dto.Organization{ID: testOrganizationID, Status: "trial", TrialEndsAt: &future}, "password123", http.StatusOK, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newSignInDeps(t, tenant.AnyOrganization())
			d.userRepo.EXPECT().FindByEmail(mock.Anything, "ana@example.com").Return(&dto.UserInOrganization{User: hashedUser(t), OrganizationID: testOrganizationID}, nil)
			d.orgRepo.EXPECT().Get(mock.Anything, testOrganizationID).Return(tc.org, nil)
			d.userRoleRepo.EXPECT().FindRolesByUserID(mock.Anything, "ana-id").Return([]*dto.Role{{ID: permissions.RoleAdmin, Name: "admin"}}, nil).Maybe()

			rec := d.do(http.MethodPost, "/api/auth/signin", `{"email":"ana@example.com","password":"`+tc.password+`"}`)

			assert.Equal(t, tc.code, rec.Code, rec.Body.String())
			if tc.want == pendingWithSetupToken {
				assertPendingWithSetupToken(t, rec)
			} else if tc.want != "" {
				assert.JSONEq(t, tc.want, rec.Body.String())
			} else {
				assert.Contains(t, decodeBody(t, rec), "token")
			}
		})
	}
}

func TestUserHandler_SignIn_PendingManagerGetsNoSetupToken(t *testing.T) {
	d := newSignInDeps(t, tenant.AnyOrganization())
	d.userRepo.EXPECT().FindByEmail(mock.Anything, "ana@example.com").Return(&dto.UserInOrganization{User: hashedUser(t), OrganizationID: testOrganizationID}, nil)
	d.orgRepo.EXPECT().Get(mock.Anything, testOrganizationID).Return(&dto.Organization{ID: testOrganizationID, Status: "pending"}, nil)
	d.userRoleRepo.EXPECT().FindRolesByUserID(mock.Anything, "ana-id").Return([]*dto.Role{{ID: permissions.RoleManager, Name: "manager"}}, nil)

	rec := d.do(http.MethodPost, "/api/auth/signin", `{"email":"ana@example.com","password":"password123"}`)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.JSONEq(t, `{"error":"Approval pending","code":"pending_approval"}`, rec.Body.String())
}
