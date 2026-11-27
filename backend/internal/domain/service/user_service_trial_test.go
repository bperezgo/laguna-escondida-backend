package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var signInNow = time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)

func trialOrg(endsIn time.Duration) *dto.Organization {
	ends := signInNow.Add(endsIn)
	return &dto.Organization{ID: testOrgA, Slug: "dona-rosa", Status: dto.OrganizationStatusTrial, Plan: "full", TrialEndsAt: &ends}
}

func trialFixture(t *testing.T, org *dto.Organization, password string) userServiceFixture {
	t.Helper()
	f := newUserServiceFixture(t, tenant.AnyOrganization())
	f.service.now = func() time.Time { return signInNow }
	f.userRepo.On("FindByEmail", mock.Anything, "ana@example.com").Return(anaFound(t, password, true), nil)
	f.orgRepo.On("Get", mock.Anything, testOrgA).Return(org, nil)
	f.userRoleRepo.On("FindRolesByUserID", mock.Anything, "bob-id").Return([]*dto.Role{createTestRole(permissions.RoleAdmin, "admin")}, nil).Maybe()
	return f
}

func TestUserService_SignIn_ExpiredTrialWithTheCorrectPassword(t *testing.T) {
	f := trialFixture(t, trialOrg(-24*time.Hour), "password123")

	result, err := cloudSignIn(f)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainError.ErrTrialExpired)
	assert.NotErrorIs(t, err, domainError.ErrInvalidCredentials)
}

func TestUserService_SignIn_ExpiredTrialWithAWrongPassword(t *testing.T) {
	f := trialFixture(t, trialOrg(-24*time.Hour), "other-password")

	_, err := cloudSignIn(f)

	assert.ErrorIs(t, err, domainError.ErrInvalidCredentials)
	assert.NotErrorIs(t, err, domainError.ErrTrialExpired)
}

func TestUserService_SignIn_TrialInProgress(t *testing.T) {
	f := trialFixture(t, trialOrg(9*24*time.Hour), "password123")

	result, err := cloudSignIn(f)

	require.NoError(t, err)
	assert.NotEmpty(t, result.Token)
}

func pendingOrg() *dto.Organization {
	return &dto.Organization{ID: testOrgA, Slug: "dona-rosa", Status: dto.OrganizationStatusPending, Plan: "einvoicing"}
}

func TestUserService_SignIn_PendingWithTheCorrectPassword(t *testing.T) {
	f := trialFixture(t, pendingOrg(), "password123")

	result, err := cloudSignIn(f)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainError.ErrOrganizationPending)
	assert.NotErrorIs(t, err, domainError.ErrInvalidCredentials)
	var pending *domainError.PendingApprovalError
	require.ErrorAs(t, err, &pending)
	claims, err := f.jwt.ValidateSetupToken(pending.SetupToken)
	require.NoError(t, err)
	assert.Equal(t, testOrgA, claims.OrgID)
	assert.Equal(t, "bob-id", claims.UserID)
	_, err = f.jwt.ValidateToken(pending.SetupToken)
	assert.ErrorIs(t, err, ErrInvalidToken, "the setup token is not a sign-in")
}

func TestUserService_SignIn_PendingManagerGetsNoSetupToken(t *testing.T) {
	f := newUserServiceFixture(t, tenant.AnyOrganization())
	f.userRepo.On("FindByEmail", mock.Anything, "ana@example.com").Return(anaFound(t, "password123", true), nil)
	f.orgRepo.On("Get", mock.Anything, testOrgA).Return(pendingOrg(), nil)
	f.userRoleRepo.On("FindRolesByUserID", mock.Anything, "bob-id").Return([]*dto.Role{createTestRole(permissions.RoleManager, "manager")}, nil)

	_, err := cloudSignIn(f)

	assert.ErrorIs(t, err, domainError.ErrOrganizationPending)
	var pending *domainError.PendingApprovalError
	assert.False(t, errors.As(err, &pending))
}

func TestUserService_SignIn_PendingWithAWrongPassword(t *testing.T) {
	f := trialFixture(t, pendingOrg(), "other-password")

	_, err := cloudSignIn(f)

	assert.ErrorIs(t, err, domainError.ErrInvalidCredentials)
	assert.NotErrorIs(t, err, domainError.ErrOrganizationPending)
}

func TestUserService_SignIn_PendingButNoCloudRoleStaysOpaque(t *testing.T) {
	f := newUserServiceFixture(t, tenant.AnyOrganization())
	f.userRepo.On("FindByEmail", mock.Anything, "ana@example.com").Return(anaFound(t, "password123", true), nil)
	f.orgRepo.On("Get", mock.Anything, testOrgA).Return(pendingOrg(), nil)
	f.userRoleRepo.On("FindRolesByUserID", mock.Anything, "bob-id").Return([]*dto.Role{createTestRole(permissions.RoleServer, "server")}, nil)

	_, err := cloudSignIn(f)

	assert.ErrorIs(t, err, domainError.ErrInvalidCredentials)
}

func TestUserService_SignIn_ExpiredTrialButNoCloudRoleStaysOpaque(t *testing.T) {
	f := newUserServiceFixture(t, tenant.AnyOrganization())
	f.service.now = func() time.Time { return signInNow }
	f.userRepo.On("FindByEmail", mock.Anything, "ana@example.com").Return(anaFound(t, "password123", true), nil)
	f.orgRepo.On("Get", mock.Anything, testOrgA).Return(trialOrg(-time.Hour), nil)
	f.userRoleRepo.On("FindRolesByUserID", mock.Anything, "bob-id").Return([]*dto.Role{createTestRole(permissions.RoleServer, "server")}, nil)

	_, err := cloudSignIn(f)

	assert.ErrorIs(t, err, domainError.ErrInvalidCredentials)
}

func TestUserService_SignInWithGoogle(t *testing.T) {
	t.Run("signs in the user with that email", func(t *testing.T) {
		f := trialFixture(t, lagunaOrg(dto.OrganizationStatusActive), "irrelevant")
		result, err := f.service.SignInWithGoogle(context.Background(), "Ana@Example.com")
		require.NoError(t, err)
		claims, err := f.jwt.ValidateToken(result.Token)
		require.NoError(t, err)
		assert.Equal(t, testOrgA, claims.OrgID)
	})
	t.Run("expired trial", func(t *testing.T) {
		f := trialFixture(t, trialOrg(-time.Hour), "irrelevant")
		_, err := f.service.SignInWithGoogle(context.Background(), "ana@example.com")
		assert.ErrorIs(t, err, domainError.ErrTrialExpired)
	})
	t.Run("pending organization", func(t *testing.T) {
		f := trialFixture(t, pendingOrg(), "irrelevant")
		_, err := f.service.SignInWithGoogle(context.Background(), "ana@example.com")
		assert.ErrorIs(t, err, domainError.ErrOrganizationPending)
		assert.NotErrorIs(t, err, domainError.ErrGoogleSignInFailed)
		var pending *domainError.PendingApprovalError
		require.ErrorAs(t, err, &pending)
		assert.NotEmpty(t, pending.SetupToken)
	})
	t.Run("suspended organization is opaque", func(t *testing.T) {
		f := trialFixture(t, lagunaOrg(dto.OrganizationStatusSuspended), "irrelevant")
		_, err := f.service.SignInWithGoogle(context.Background(), "ana@example.com")
		assert.ErrorIs(t, err, domainError.ErrGoogleSignInFailed)
	})
	t.Run("no account is opaque", func(t *testing.T) {
		f := newUserServiceFixture(t, tenant.AnyOrganization())
		f.userRepo.On("FindByEmail", mock.Anything, "nobody@example.com").Return(nil, domainError.ErrUserNotFound)
		_, err := f.service.SignInWithGoogle(context.Background(), "nobody@example.com")
		assert.ErrorIs(t, err, domainError.ErrGoogleSignInFailed)
	})
	t.Run("inactive user is opaque", func(t *testing.T) {
		f := newUserServiceFixture(t, tenant.AnyOrganization())
		f.userRepo.On("FindByEmail", mock.Anything, "ana@example.com").Return(anaFound(t, "irrelevant", false), nil)
		f.orgRepo.On("Get", mock.Anything, testOrgA).Return(lagunaOrg(dto.OrganizationStatusActive), nil)
		_, err := f.service.SignInWithGoogle(context.Background(), "ana@example.com")
		assert.ErrorIs(t, err, domainError.ErrGoogleSignInFailed)
	})
	t.Run("floor-only user is opaque", func(t *testing.T) {
		f := newUserServiceFixture(t, tenant.AnyOrganization())
		f.userRepo.On("FindByEmail", mock.Anything, "ana@example.com").Return(anaFound(t, "irrelevant", true), nil)
		f.orgRepo.On("Get", mock.Anything, testOrgA).Return(lagunaOrg(dto.OrganizationStatusActive), nil)
		f.userRoleRepo.On("FindRolesByUserID", mock.Anything, "bob-id").Return([]*dto.Role{createTestRole(permissions.RoleServer, "server")}, nil)
		_, err := f.service.SignInWithGoogle(context.Background(), "ana@example.com")
		assert.ErrorIs(t, err, domainError.ErrGoogleSignInFailed)
	})
	t.Run("an edge box refuses it", func(t *testing.T) {
		f := newUserServiceFixture(t, tenant.PinnedTo(testOrgA))
		_, err := f.service.SignInWithGoogle(context.Background(), "ana@example.com")
		assert.ErrorIs(t, err, domainError.ErrGoogleSignInFailed)
	})
}
