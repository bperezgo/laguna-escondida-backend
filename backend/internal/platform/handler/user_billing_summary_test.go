package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
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

type stubSummarizer struct {
	summary *dto.BillingSummary
	err     error
	calls   int
}

func (s *stubSummarizer) Summary(context.Context, string) (*dto.BillingSummary, error) {
	s.calls++
	return s.summary, s.err
}

// serveCurrentUserAs calls /auth/me for a user with one role; billing is nil on an edge box.
func serveCurrentUserAs(t *testing.T, roleID int, billing *stubSummarizer) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	userRepo, userRoleRepo, orgRepo := mocks.NewMockUserRepository(t), mocks.NewMockUserRoleRepository(t), mocks.NewMockOrganizationRepository(t)
	userRepo.EXPECT().FindByID(mock.Anything, testCurrentUserID).Return(&dto.User{ID: testCurrentUserID, Username: "rosa", Name: "Rosa"}, nil)
	userRoleRepo.EXPECT().FindRolesByUserID(mock.Anything, testCurrentUserID).Return([]*dto.Role{{ID: roleID, Name: "r"}}, nil)
	ends := time.Date(2026, 12, 15, 14, 0, 0, 0, time.UTC)
	orgRepo.EXPECT().Get(mock.Anything, testOrganizationID).Return(
		&dto.Organization{ID: testOrganizationID, Name: "Doña Rosa", Status: "trial", Plan: "einvoicing", TrialEndsAt: &ends}, nil)

	scope := tenant.AnyOrganization()
	users := service.NewUserService(userRepo, mocks.NewMockRoleRepository(t), userRoleRepo, orgRepo, service.NewJWTService("test-secret", scope), immediateUnitOfWork{}, scope)
	h := NewUserHandler(users, service.NewOrganizationService(orgRepo, mocks.NewMockEInvoicingSettingsRepository(t), immediateUnitOfWork{}), slog.Default())
	if billing != nil {
		h.ShowBilling(billing)
	}
	router := gin.New()
	router.GET("/api/auth/me", func(c *gin.Context) {
		c.Set("user_id", testCurrentUserID)
		c.Set("org_id", testOrganizationID)
		h.GetCurrentUserHandler(c)
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/auth/me", nil))
	return rec
}

func trialSummary() *dto.BillingSummary {
	due := time.Date(2026, 12, 15, 14, 0, 0, 0, time.UTC)
	ends := due.Add(72 * time.Hour)
	amount := 40000
	return &dto.BillingSummary{Period: "monthly", HasCard: true, CardEntry: true, DueDate: &due, NextChargeAmount: &amount, AccessEndsAt: &ends}
}

func TestUserHandler_GetCurrentUser_AnAdminGetsTheBillingSummary(t *testing.T) {
	billing := &stubSummarizer{summary: trialSummary()}

	rec := serveCurrentUserAs(t, permissions.RoleAdmin, billing)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := decodeBody(t, rec)
	require.Contains(t, got, "billing")
	assert.JSONEq(t, `{"period":"monthly","has_card":true,"card_entry":true,"cancelled":false,
		"due_date":"2026-12-15T14:00:00Z","next_charge_amount":40000,"payment_failed":false,
		"access_ends_at":"2026-12-18T14:00:00Z"}`, string(got["billing"]))
	assert.Contains(t, got, "organization")
}

// A manager gets only what the trial's days-left notice uses today: the organization's status and trial end.
func TestUserHandler_GetCurrentUser_AManagerGetsNoBilling(t *testing.T) {
	for _, role := range []int{permissions.RoleManager, permissions.RoleAccountant, permissions.RoleServer} {
		billing := &stubSummarizer{summary: trialSummary()}

		rec := serveCurrentUserAs(t, role, billing)

		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		got := decodeBody(t, rec)
		assert.NotContains(t, got, "billing", "role %d", role)
		assert.Zero(t, billing.calls, "billing is not even read")
		var org dto.CurrentOrganization
		require.NoError(t, json.Unmarshal(got["organization"], &org))
		assert.Equal(t, "trial", org.Status)
		require.NotNil(t, org.TrialEndsAt)
	}
}

func TestUserHandler_GetCurrentUser_AnEdgeBoxHasNoBilling(t *testing.T) {
	rec := serveCurrentUserAs(t, permissions.RoleAdmin, nil)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, decodeBody(t, rec), "billing")
}

func TestUserHandler_GetCurrentUser_ABillingFailureIs500(t *testing.T) {
	rec := serveCurrentUserAs(t, permissions.RoleAdmin, &stubSummarizer{err: errors.New("db down")})

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
