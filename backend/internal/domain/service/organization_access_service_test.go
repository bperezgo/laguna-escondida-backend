package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var accessNow = time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)

func newAccessService(t *testing.T, access *dto.OrganizationAccess) *OrganizationAccessService {
	t.Helper()
	reader := mocks.NewMockOrganizationAccessReader(t)
	reader.EXPECT().GetAccess(mock.Anything, testOrgA).Return(access, nil).Maybe()
	s := NewOrganizationAccessService(reader)
	s.now = func() time.Time { return accessNow }
	return s
}

func accessAt(d time.Duration) *time.Time { v := accessNow.Add(d); return &v }

var accessManager = AccessPrincipal{RoleIDs: []int{permissions.RoleManager}}

func assertPlanRequired(t *testing.T, err error, plan string) {
	t.Helper()
	var planErr *domainError.PlanRequiredError
	require.ErrorAs(t, err, &planErr)
	assert.Equal(t, plan, planErr.RequiredPlan)
	assert.ErrorIs(t, err, domainError.ErrPlanRequired)
}

func TestOrganizationAccessService_Check_SuspendedComesBeforeExpired(t *testing.T) {
	s := newAccessService(t, &dto.OrganizationAccess{Status: "suspended", Plan: "starter", TrialEndsAt: accessAt(-time.Hour)})
	assert.ErrorIs(t, s.Check(context.Background(), testOrgA, accessManager, "/api/invoices"), domainError.ErrOrganizationSuspended)
}

func TestOrganizationAccessService_Check_PendingComesBeforePlan(t *testing.T) {
	s := newAccessService(t, &dto.OrganizationAccess{Status: "pending", Plan: "starter"})
	assert.ErrorIs(t, s.Check(context.Background(), testOrgA, accessManager, "/api/invoices"), domainError.ErrOrganizationPending)
	assert.ErrorIs(t, s.Check(context.Background(), testOrgA, accessManager, "/api/products"), domainError.ErrOrganizationPending)
	assert.ErrorIs(t, s.Check(context.Background(), testOrgA, AccessPrincipal{APIKey: true}, "/api/products"), domainError.ErrOrganizationPending)
}

func TestOrganizationAccessService_Check_SuspendedIsNotReportedAsPending(t *testing.T) {
	s := newAccessService(t, &dto.OrganizationAccess{Status: "suspended", Plan: "full"})
	err := s.Check(context.Background(), testOrgA, accessManager, "/api/products")
	assert.ErrorIs(t, err, domainError.ErrOrganizationSuspended)
	assert.NotErrorIs(t, err, domainError.ErrOrganizationPending)
}

func TestOrganizationAccessService_Check_ExpiredComesBeforePlan(t *testing.T) {
	s := newAccessService(t, &dto.OrganizationAccess{Status: "trial", Plan: "starter", TrialEndsAt: accessAt(-time.Hour)})
	assert.ErrorIs(t, s.Check(context.Background(), testOrgA, accessManager, "/api/invoices"), domainError.ErrTrialExpired)
	assert.ErrorIs(t, s.Check(context.Background(), testOrgA, accessManager, "/api/products"), domainError.ErrTrialExpired)
}

func TestOrganizationAccessService_Check_ExpiresExactlyAtTrialEnd(t *testing.T) {
	s := newAccessService(t, &dto.OrganizationAccess{Status: "trial", Plan: "full", TrialEndsAt: accessAt(0)})
	assert.ErrorIs(t, s.Check(context.Background(), testOrgA, accessManager, "/api/products"), domainError.ErrTrialExpired)
}

func TestOrganizationAccessService_Check_TrialInProgress(t *testing.T) {
	s := newAccessService(t, &dto.OrganizationAccess{Status: "trial", Plan: "full", TrialEndsAt: accessAt(24 * time.Hour)})
	assert.NoError(t, s.Check(context.Background(), testOrgA, accessManager, "/api/reservations"))
}

func TestOrganizationAccessService_Check_StarterRefusedOnPlanFeatures(t *testing.T) {
	s := newAccessService(t, &dto.OrganizationAccess{Status: "active", Plan: "starter"})
	assertPlanRequired(t, s.Check(context.Background(), testOrgA, accessManager, "/api/invoices"), "einvoicing")
	assertPlanRequired(t, s.Check(context.Background(), testOrgA, accessManager, "/api/reservations"), "full")
}

func TestOrganizationAccessService_Check_EInvoicingRefusedOnReservations(t *testing.T) {
	s := newAccessService(t, &dto.OrganizationAccess{Status: "active", Plan: "einvoicing"})
	assert.NoError(t, s.Check(context.Background(), testOrgA, accessManager, "/api/invoices/:id"))
	assertPlanRequired(t, s.Check(context.Background(), testOrgA, accessManager, "/api/reservations"), "full")
}

func TestOrganizationAccessService_Check_APIKeyAndAgentNeedFull(t *testing.T) {
	s := newAccessService(t, &dto.OrganizationAccess{Status: "active", Plan: "einvoicing"})
	assertPlanRequired(t, s.Check(context.Background(), testOrgA, AccessPrincipal{APIKey: true, RoleIDs: []int{permissions.RoleAgent}}, "/api/products"), "full")
	assertPlanRequired(t, s.Check(context.Background(), testOrgA, AccessPrincipal{RoleIDs: []int{permissions.RoleAgent}}, "/api/products"), "full")
}

func TestOrganizationAccessService_Check_CoreOnEveryPlan(t *testing.T) {
	for _, plan := range []string{"starter", "einvoicing", "full"} {
		t.Run(plan, func(t *testing.T) {
			s := newAccessService(t, &dto.OrganizationAccess{Status: "active", Plan: plan})
			for _, route := range []string{"/api/products", "/api/orders/:id/pay", "/api/admin/users", "/api/organization/settings", "/api/edge/enrollment-codes"} {
				assert.NoError(t, s.Check(context.Background(), testOrgA, accessManager, route), route)
			}
		})
	}
}

func TestOrganizationAccessService_Check_FullPassesEverything(t *testing.T) {
	s := newAccessService(t, &dto.OrganizationAccess{Status: "active", Plan: "full", TrialEndsAt: accessAt(-48 * time.Hour)})
	for _, route := range []string{"/api/invoices", "/api/reservations", "/api/sse/reservations", "/api/organization/integrations", "/api/organization/api-keys", "/api/products"} {
		assert.NoError(t, s.Check(context.Background(), testOrgA, accessManager, route), route)
	}
	assert.NoError(t, s.Check(context.Background(), testOrgA, AccessPrincipal{APIKey: true, RoleIDs: []int{permissions.RoleAgent}}, "/api/orders"))
}

func TestOrganizationAccessService_Check_ReaderError(t *testing.T) {
	reader := mocks.NewMockOrganizationAccessReader(t)
	reader.EXPECT().GetAccess(mock.Anything, testOrgA).Return(nil, errors.New("db down"))
	err := NewOrganizationAccessService(reader).Check(context.Background(), testOrgA, accessManager, "/api/products")
	assert.Error(t, err)
	assert.NotErrorIs(t, err, domainError.ErrPlanRequired)
}

func TestOrganizationAccessService_Usable(t *testing.T) {
	cases := []struct {
		name   string
		access *dto.OrganizationAccess
		want   error
	}{
		{"active", &dto.OrganizationAccess{Status: "active", Plan: "starter"}, nil},
		{"trial in progress", &dto.OrganizationAccess{Status: "trial", Plan: "starter", TrialEndsAt: accessAt(time.Hour)}, nil},
		{"trial expired", &dto.OrganizationAccess{Status: "trial", Plan: "starter", TrialEndsAt: accessAt(-time.Hour)}, domainError.ErrOrganizationInactive},
		{"suspended", &dto.OrganizationAccess{Status: "suspended", Plan: "full"}, domainError.ErrOrganizationInactive},
		{"pending", &dto.OrganizationAccess{Status: "pending", Plan: "full"}, domainError.ErrOrganizationInactive},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := newAccessService(t, tc.access).Usable(context.Background(), testOrgA)
			if tc.want == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, tc.want)
			}
		})
	}
}

// The organization-plans delta: a trial with a live subscription gets three days of grace, and an
// active organization with a paid period stops when it lapses.
func TestOrganizationAccessService_Check_BillingScenarios(t *testing.T) {
	day := 24 * time.Hour
	cases := []struct {
		name   string
		access dto.OrganizationAccess
		want   error
	}{
		{"day 13 of a trial", dto.OrganizationAccess{Status: "trial", TrialEndsAt: accessAt(day)}, nil},
		{"day 15 without a saved card", dto.OrganizationAccess{Status: "trial", TrialEndsAt: accessAt(-day)}, domainError.ErrTrialExpired},
		{"day 15 with a saved card and a declined charge", dto.OrganizationAccess{Status: "trial", TrialEndsAt: accessAt(-day), HasCard: true}, nil},
		{"one second before the trial's grace ends", dto.OrganizationAccess{Status: "trial", TrialEndsAt: accessAt(-3*day + time.Second), HasCard: true}, nil},
		{"exactly at the end of the trial's grace", dto.OrganizationAccess{Status: "trial", TrialEndsAt: accessAt(-3 * day), HasCard: true}, domainError.ErrTrialExpired},
		{"day 18 with a saved card and no approved charge", dto.OrganizationAccess{Status: "trial", TrialEndsAt: accessAt(-4 * day), HasCard: true}, domainError.ErrTrialExpired},
		{"cancelled trial on day 15", dto.OrganizationAccess{Status: "trial", TrialEndsAt: accessAt(-day), HasCard: true, CancelledAt: accessAt(-5 * day)}, domainError.ErrTrialExpired},
		{"cancelled trial before its end", dto.OrganizationAccess{Status: "trial", TrialEndsAt: accessAt(day), HasCard: true, CancelledAt: accessAt(-5 * day)}, nil},

		{"paid and current", dto.OrganizationAccess{Status: "active", PaidUntil: accessAt(10 * day), HasCard: true}, nil},
		{"renewal declined, within grace", dto.OrganizationAccess{Status: "active", PaidUntil: accessAt(-2 * day), HasCard: true}, nil},
		{"grace over", dto.OrganizationAccess{Status: "active", PaidUntil: accessAt(-4 * day), HasCard: true}, domainError.ErrPaymentRequired},
		{"exactly at the end of the paid grace", dto.OrganizationAccess{Status: "active", PaidUntil: accessAt(-3 * day), HasCard: true}, domainError.ErrPaymentRequired},
		{"cancelled subscription a day after its period", dto.OrganizationAccess{Status: "active", PaidUntil: accessAt(-day), HasCard: true, CancelledAt: accessAt(-13 * day)}, domainError.ErrPaymentRequired},
		{"cancelled subscription inside its period", dto.OrganizationAccess{Status: "active", PaidUntil: accessAt(day), HasCard: true, CancelledAt: accessAt(-13 * day)}, nil},

		{"active by hand with a trial end long past", dto.OrganizationAccess{Status: "active", TrialEndsAt: accessAt(-400 * day)}, nil},
		{"active by hand with a saved card and no paid period", dto.OrganizationAccess{Status: "active", TrialEndsAt: accessAt(-400 * day), HasCard: true}, nil},
		{"active by hand after a cancellation", dto.OrganizationAccess{Status: "active", HasCard: true, CancelledAt: accessAt(-400 * day)}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			access := tc.access
			access.Plan = "full"
			s := newAccessService(t, &access)

			err := s.Check(context.Background(), testOrgA, accessManager, "/api/products")
			stopped := s.Stopped(context.Background(), testOrgA)
			usable := s.Usable(context.Background(), testOrgA)

			if tc.want == nil {
				require.NoError(t, err)
				require.NoError(t, stopped)
				require.NoError(t, usable)
				return
			}
			assert.ErrorIs(t, err, tc.want)
			assert.ErrorIs(t, stopped, tc.want)
			assert.ErrorIs(t, usable, domainError.ErrOrganizationInactive)
			assert.ErrorIs(t, s.Check(context.Background(), testOrgA, AccessPrincipal{APIKey: true}, "/api/products"), tc.want,
				"an API key is refused with the same code")
		})
	}
}

func TestOrganizationAccessService_Check_SuspendedComesBeforeUnpaid(t *testing.T) {
	s := newAccessService(t, &dto.OrganizationAccess{Status: "suspended", Plan: "full", PaidUntil: accessAt(-240 * time.Hour), HasCard: true})
	assert.ErrorIs(t, s.Check(context.Background(), testOrgA, accessManager, "/api/products"), domainError.ErrOrganizationSuspended)
	require.NoError(t, s.Stopped(context.Background(), testOrgA), "a suspended organization is not one that can pay its way back")
}

func TestOrganizationAccessService_Check_UnpaidComesBeforePlan(t *testing.T) {
	s := newAccessService(t, &dto.OrganizationAccess{Status: "active", Plan: "starter", PaidUntil: accessAt(-240 * time.Hour), HasCard: true})
	assert.ErrorIs(t, s.Check(context.Background(), testOrgA, accessManager, "/api/invoices"), domainError.ErrPaymentRequired)
}

func TestOrganizationAccessService_SetupState(t *testing.T) {
	cases := []struct {
		name   string
		access dto.OrganizationAccess
		want   SetupState
	}{
		{"pending", dto.OrganizationAccess{Status: "pending"}, SetupStatePending},
		{"suspended", dto.OrganizationAccess{Status: "suspended", PaidUntil: accessAt(-240 * time.Hour)}, SetupStateClosed},
		{"expired trial", dto.OrganizationAccess{Status: "trial", TrialEndsAt: accessAt(-time.Hour)}, SetupStateStopped},
		{"unpaid", dto.OrganizationAccess{Status: "active", PaidUntil: accessAt(-240 * time.Hour), HasCard: true}, SetupStateStopped},
		{"trial in grace", dto.OrganizationAccess{Status: "trial", TrialEndsAt: accessAt(-time.Hour), HasCard: true}, SetupStateWorking},
		{"active", dto.OrganizationAccess{Status: "active"}, SetupStateWorking},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			access := tc.access
			got, err := newAccessService(t, &access).SetupState(context.Background(), testOrgA)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
