package service

import (
	"context"
	"errors"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const testOrgSuspended = "019a0000-0000-7000-8000-00000000000c"

func runnerOrgs() []*dto.Organization {
	return []*dto.Organization{
		{ID: testOrgA, Status: dto.OrganizationStatusActive, EInvoicing: &dto.EInvoicingSettings{InvoicePrefix: "LAG"}},
		{ID: testOrgB, Status: dto.OrganizationStatusActive},
		{ID: testOrgSuspended, Status: dto.OrganizationStatusSuspended, EInvoicing: &dto.EInvoicingSettings{InvoicePrefix: "SUS"}},
	}
}

// newRunner counts opened and released sessions so a test can see each run got its own.
func newRunner(t *testing.T, orgs []*dto.Organization) (*OrganizationRunner, *int, *int) {
	t.Helper()
	orgRepo := mocks.NewMockOrganizationRepository(t)
	orgRepo.On("List", mock.Anything).Return(orgs, nil)
	sessions := mocks.NewMockSessionOpener(t)
	opened, released := 0, 0
	sessions.On("Open", mock.Anything).Return(
		func(ctx context.Context) context.Context { opened++; return ctx },
		func(context.Context) func() { return func() { released++ } },
	).Maybe()
	return NewOrganizationRunner(orgRepo, sessions), &opened, &released
}

func TestOrganizationRunner_ForEach_RunsEachActiveOrganizationInItsOwnContext(t *testing.T) {
	runner, opened, released := newRunner(t, runnerOrgs())
	var ran []string

	failures, err := runner.ForEach(context.Background(), AllActiveOrganizations, func(ctx context.Context) error {
		orgID, ok := tenant.OrgFrom(ctx)
		require.True(t, ok)
		ran = append(ran, orgID)
		return nil
	})

	require.NoError(t, err)
	assert.Empty(t, failures)
	assert.Equal(t, []string{testOrgA, testOrgB}, ran, "the suspended organization is skipped")
	assert.Equal(t, 2, *opened)
	assert.Equal(t, 2, *released)
}

func TestOrganizationRunner_ForEach_OneFailureDoesNotStopTheOthers(t *testing.T) {
	runner, _, released := newRunner(t, runnerOrgs())
	boom := errors.New("provider down")
	var ran []string

	failures, err := runner.ForEach(context.Background(), AllActiveOrganizations, func(ctx context.Context) error {
		orgID, _ := tenant.OrgFrom(ctx)
		ran = append(ran, orgID)
		if orgID == testOrgA {
			return boom
		}
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, []string{testOrgA, testOrgB}, ran)
	require.Len(t, failures, 1)
	assert.Equal(t, testOrgA, failures[0].OrganizationID)
	assert.ErrorIs(t, failures[0].Err, boom)
	assert.Equal(t, 2, *released, "a failed run still releases its session")
}

func TestOrganizationRunner_ForEach_EInvoicingScope(t *testing.T) {
	runner, _, _ := newRunner(t, runnerOrgs())
	var ran []string

	_, err := runner.ForEach(context.Background(), EInvoicingOrganizations, func(ctx context.Context) error {
		orgID, _ := tenant.OrgFrom(ctx)
		ran = append(ran, orgID)
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, []string{testOrgA}, ran, "only active organizations with e-invoicing configured")
}

func TestOrganizationRunner_ForEach_ListFails(t *testing.T) {
	orgRepo := mocks.NewMockOrganizationRepository(t)
	orgRepo.On("List", mock.Anything).Return(nil, errors.New("db down"))
	runner := NewOrganizationRunner(orgRepo, mocks.NewMockSessionOpener(t))

	_, err := runner.ForEach(context.Background(), AllActiveOrganizations, func(context.Context) error {
		t.Fatal("no job runs without the organizations")
		return nil
	})

	assert.Error(t, err)
}
