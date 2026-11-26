package service

import (
	"context"
	"strings"
	"testing"
	"time"

	useragg "laguna-escondida/backend/internal/domain/aggregate/user"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var invitationNow = time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)

const invitationToken = "tok-abc"

type invitationFixture struct {
	invitations *mocks.MockInvitationRepository
	users       *mocks.MockUserRepository
	userRoles   *mocks.MockUserRoleRepository
	roles       *mocks.MockRoleRepository
	orgs        *mocks.MockOrganizationRepository
	access      *mocks.MockOrganizationAccessReader
	google      *mocks.MockGoogleIdentityVerifier
	jwt         *JWTService
	service     *InvitationService
}

func newInvitationFixture(t *testing.T, withGoogle bool) *invitationFixture {
	t.Helper()
	f := &invitationFixture{
		invitations: mocks.NewMockInvitationRepository(t), users: mocks.NewMockUserRepository(t),
		userRoles: mocks.NewMockUserRoleRepository(t), roles: mocks.NewMockRoleRepository(t),
		orgs: mocks.NewMockOrganizationRepository(t), access: mocks.NewMockOrganizationAccessReader(t),
		google: mocks.NewMockGoogleIdentityVerifier(t), jwt: NewJWTService("test-secret", tenant.AnyOrganization()),
	}
	signIns := NewUserService(f.users, f.roles, f.userRoles, f.orgs, f.jwt, passthroughUnitOfWork{}, tenant.AnyOrganization())
	access := NewOrganizationAccessService(f.access)
	access.now = func() time.Time { return invitationNow }
	var google ports.GoogleIdentityVerifier
	if withGoogle {
		google = f.google
	}
	f.service = NewInvitationService(f.invitations, f.users, f.userRoles, f.roles, f.orgs, access, signIns, google, markingUnitOfWork{}, "https://app.example.com/")
	f.service.now = func() time.Time { return invitationNow }
	f.service.newToken = func() (string, error) { return invitationToken, nil }
	return f
}

func (f *invitationFixture) rolesExist(ids ...int) {
	roles := make([]*dto.Role, len(ids))
	for i, id := range ids {
		roles[i] = createTestRole(id, "role")
	}
	f.roles.On("FindByIDs", mock.Anything, ids).Return(roles, nil)
}

var adminInviter = dto.InviterPrincipal{UserID: "admin-id", RoleIDs: []int{permissions.RoleAdmin}}

func orgCtx() context.Context { return tenant.WithOrg(context.Background(), testOrgA) }

func TestInvitationService_Create_AdminInvitesAnAccountant(t *testing.T) {
	f := newInvitationFixture(t, true)
	f.rolesExist(permissions.RoleAccountant)
	f.users.On("FindByEmail", mock.Anything, "contadora@example.com").Return(nil, domainError.ErrUserNotFound)
	var stored *dto.Invitation
	var storedHash string
	f.invitations.On("Create", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		stored, _ = args.Get(1).(*dto.Invitation)
		storedHash = args.String(2)
	}).Return(nil)

	got, err := f.service.Create(orgCtx(), adminInviter, &dto.CreateInvitationRequest{
		Email: " Contadora@Example.com ", Name: "Contadora", RoleIDs: []int{permissions.RoleAccountant},
	})

	require.NoError(t, err)
	assert.Equal(t, "https://app.example.com/invitacion/"+invitationToken, got.Link)
	assert.Equal(t, "contadora@example.com", stored.Email)
	assert.Equal(t, "admin-id", stored.InvitedBy)
	assert.Equal(t, invitationNow.Add(7*24*time.Hour), stored.ExpiresAt)
	assert.Equal(t, hashSecret(invitationToken), storedHash, "only the token's hash is stored")
	assert.NotContains(t, storedHash, invitationToken)
}

func TestInvitationService_Create_Refusals(t *testing.T) {
	manager := dto.InviterPrincipal{UserID: "m", RoleIDs: []int{permissions.RoleManager}}
	cases := []struct {
		name    string
		inviter dto.InviterPrincipal
		req     dto.CreateInvitationRequest
		setup   func(f *invitationFixture)
		want    error
	}{
		{"manager invites an admin", manager, dto.CreateInvitationRequest{Email: "a@example.com", Name: "A", RoleIDs: []int{permissions.RoleAdmin}}, nil, domainError.ErrInvitationAdminOnly},
		{"agent role", adminInviter, dto.CreateInvitationRequest{Email: "a@example.com", Name: "A", RoleIDs: []int{permissions.RoleAgent}}, nil, domainError.ErrInvitationRoleNotAllowed},
		{"floor role", adminInviter, dto.CreateInvitationRequest{Email: "a@example.com", Name: "A", RoleIDs: []int{permissions.RoleManager, permissions.RoleServer}}, nil, domainError.ErrInvitationRoleNotAllowed},
		{"no email", adminInviter, dto.CreateInvitationRequest{Name: "A", RoleIDs: []int{permissions.RoleManager}}, nil, domainError.ErrEmailRequired},
		{"no name", adminInviter, dto.CreateInvitationRequest{Email: "a@example.com", Name: " ", RoleIDs: []int{permissions.RoleManager}}, nil, domainError.ErrInvitationNameInvalid},
		{"no roles", adminInviter, dto.CreateInvitationRequest{Email: "a@example.com", Name: "A"}, nil, domainError.ErrInvalidRoleIDs},
		{"email already has an account", adminInviter, dto.CreateInvitationRequest{Email: "ana@example.com", Name: "Ana", RoleIDs: []int{permissions.RoleManager}}, func(f *invitationFixture) {
			f.rolesExist(permissions.RoleManager)
			f.users.On("FindByEmail", mock.Anything, "ana@example.com").Return(&dto.UserInOrganization{User: &dto.User{ID: "x"}, OrganizationID: testOrgB}, nil)
		}, domainError.ErrUserEmailTaken},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newInvitationFixture(t, true)
			if tc.setup != nil {
				tc.setup(f)
			}
			_, err := f.service.Create(orgCtx(), tc.inviter, &tc.req)
			assert.ErrorIs(t, err, tc.want)
			f.invitations.AssertNotCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestInvitationService_Create_PendingInvitationIsAConflict(t *testing.T) {
	f := newInvitationFixture(t, true)
	f.rolesExist(permissions.RoleManager)
	f.users.On("FindByEmail", mock.Anything, "ana@example.com").Return(nil, domainError.ErrUserNotFound)
	f.invitations.On("Create", mock.Anything, mock.Anything, mock.Anything).Return(domainError.ErrInvitationPending)

	_, err := f.service.Create(orgCtx(), adminInviter, &dto.CreateInvitationRequest{Email: "ana@example.com", Name: "Ana", RoleIDs: []int{permissions.RoleManager}})

	assert.ErrorIs(t, err, domainError.ErrInvitationPending)
}

func TestInvitationService_ListNewLinkRevoke(t *testing.T) {
	f := newInvitationFixture(t, true)
	pending := &dto.Invitation{ID: "inv-1", Email: "ana@example.com", RoleIDs: []int{permissions.RoleManager}}
	f.rolesExist(permissions.RoleManager)
	f.invitations.On("ListPending", mock.Anything).Return([]*dto.Invitation{pending}, nil)
	f.invitations.On("FindPending", mock.Anything, "inv-1").Return(pending, nil)
	f.invitations.On("ReplaceToken", mock.Anything, "inv-1", hashSecret(invitationToken), invitationNow.Add(InvitationLifetime)).Return(nil)
	f.invitations.On("Revoke", mock.Anything, "inv-1", invitationNow).Return(nil)
	f.invitations.On("Revoke", mock.Anything, "other-org", invitationNow).Return(domainError.ErrInvitationNotFound)

	list, err := f.service.List(orgCtx())
	require.NoError(t, err)
	require.Len(t, list.Invitations, 1)
	assert.Len(t, list.Invitations[0].Roles, 1)

	renewed, err := f.service.NewLink(orgCtx(), "inv-1")
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(renewed.Link, "/invitacion/"+invitationToken))

	require.NoError(t, f.service.Revoke(orgCtx(), "inv-1"))
	assert.ErrorIs(t, f.service.Revoke(orgCtx(), "other-org"), domainError.ErrInvitationNotFound)
}

func validInvitation(mutate func(*dto.Invitation)) *dto.Invitation {
	inv := &dto.Invitation{
		ID: "inv-1", OrganizationID: testOrgA, Email: "contadora@example.com", Name: "Contadora",
		RoleIDs: []int{permissions.RoleAccountant}, ExpiresAt: invitationNow.Add(time.Hour), CreatedAt: invitationNow.Add(-time.Hour),
	}
	if mutate != nil {
		mutate(inv)
	}
	return inv
}

func TestInvitationService_Lookup(t *testing.T) {
	f := newInvitationFixture(t, true)
	display := "Doña Rosa"
	f.invitations.On("FindByTokenHash", mock.Anything, hashSecret(invitationToken), false).Return(validInvitation(nil), nil)
	f.orgs.On("Get", mock.Anything, testOrgA).Return(&dto.Organization{ID: testOrgA, Name: "Doña Rosa S.A.S", DisplayName: &display}, nil)
	f.rolesExist(permissions.RoleAccountant)

	got, err := f.service.Lookup(context.Background(), invitationToken)

	require.NoError(t, err)
	assert.Equal(t, "Doña Rosa", got.OrganizationName)
	assert.Equal(t, "contadora@example.com", got.Email)
	assert.Len(t, got.Roles, 1)
}

func TestInvitationService_InvalidLinks(t *testing.T) {
	past := invitationNow.Add(-time.Minute)
	cases := map[string]struct {
		inv *dto.Invitation
		err error
	}{
		"unknown":          {err: domainError.ErrInvitationNotFound},
		"expired (8 days)": {inv: validInvitation(func(i *dto.Invitation) { i.ExpiresAt = invitationNow.Add(-24 * time.Hour) })},
		"revoked":          {inv: validInvitation(func(i *dto.Invitation) { i.RevokedAt = &past })},
		"accepted":         {inv: validInvitation(func(i *dto.Invitation) { i.AcceptedAt = &past })},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newInvitationFixture(t, true)
			f.invitations.On("FindByTokenHash", mock.Anything, mock.Anything, mock.Anything).Return(tc.inv, tc.err)

			_, err := f.service.Lookup(context.Background(), invitationToken)
			assert.ErrorIs(t, err, domainError.ErrInvitationInvalidOrExpired)

			_, err = f.service.Accept(context.Background(), invitationToken, &dto.AcceptInvitationRequest{Password: "password123"})
			assert.ErrorIs(t, err, domainError.ErrInvitationInvalidOrExpired)
			f.users.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
		})
	}
}

func (f *invitationFixture) acceptable(access *dto.OrganizationAccess) {
	f.invitations.On("FindByTokenHash", mock.MatchedBy(inUnitOfWork), hashSecret(invitationToken), true).Return(validInvitation(nil), nil)
	f.access.On("GetAccess", mock.Anything, testOrgA).Return(access, nil)
}

func (f *invitationFixture) expectUserCreated(created **dto.User) {
	f.users.On("Create", mock.MatchedBy(inOrg(testOrgA)), mock.Anything).Run(func(args mock.Arguments) {
		*created, _ = args.Get(1).(*dto.User)
	}).Return(nil)
	f.userRoles.On("Create", mock.MatchedBy(inOrg(testOrgA)), mock.MatchedBy(func(r *dto.UserRole) bool { return r.RoleID == permissions.RoleAccountant })).Return(nil)
	f.rolesExist(permissions.RoleAccountant)
	f.invitations.On("MarkAccepted", mock.MatchedBy(inUnitOfWork), "inv-1", invitationNow).Return(nil)
}

var activeAccess = &dto.OrganizationAccess{Status: "active", Plan: "full"}

func TestInvitationService_Accept_WithAPassword(t *testing.T) {
	f := newInvitationFixture(t, true)
	f.acceptable(activeAccess)
	var created *dto.User
	f.expectUserCreated(&created)

	resp, err := f.service.Accept(context.Background(), invitationToken, &dto.AcceptInvitationRequest{Password: "secreto123"})

	require.NoError(t, err)
	assert.Equal(t, "contadora@example.com", created.Username)
	assert.Equal(t, "contadora@example.com", *created.Email)
	assert.Equal(t, "Contadora", created.Name)
	assert.NoError(t, useragg.NewAggregateFromDTO(created).ComparePassword("secreto123"))
	claims, err := f.jwt.ValidateToken(resp.Token)
	require.NoError(t, err)
	assert.Equal(t, testOrgA, claims.OrgID)
	assert.Equal(t, created.ID, claims.UserID)
}

func TestInvitationService_Accept_WithGoogle(t *testing.T) {
	f := newInvitationFixture(t, true)
	f.google.EXPECT().Verify(mock.Anything, "google-token").Return(&dto.GoogleIdentity{Email: "Contadora@example.com"}, nil)
	f.acceptable(activeAccess)
	var created *dto.User
	f.expectUserCreated(&created)

	_, err := f.service.Accept(context.Background(), invitationToken, &dto.AcceptInvitationRequest{IDToken: "google-token"})

	require.NoError(t, err)
	assert.NotEmpty(t, created.Password, "an unusable password, so only Google signs in")
}

func TestInvitationService_Accept_WithADifferentGoogleAccount(t *testing.T) {
	f := newInvitationFixture(t, true)
	f.google.EXPECT().Verify(mock.Anything, "google-token").Return(&dto.GoogleIdentity{Email: "otra@example.com"}, nil)
	f.invitations.On("FindByTokenHash", mock.Anything, hashSecret(invitationToken), true).Return(validInvitation(nil), nil)

	_, err := f.service.Accept(context.Background(), invitationToken, &dto.AcceptInvitationRequest{IDToken: "google-token"})

	assert.ErrorIs(t, err, domainError.ErrInvitationEmailMismatch)
	f.users.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestInvitationService_Accept_OrganizationInactive(t *testing.T) {
	past := invitationNow.Add(-time.Hour)
	for name, access := range map[string]*dto.OrganizationAccess{
		"trial expired": {Status: "trial", Plan: "full", TrialEndsAt: &past},
		"suspended":     {Status: "suspended", Plan: "full"},
	} {
		t.Run(name, func(t *testing.T) {
			f := newInvitationFixture(t, true)
			f.acceptable(access)

			_, err := f.service.Accept(context.Background(), invitationToken, &dto.AcceptInvitationRequest{Password: "secreto123"})

			assert.ErrorIs(t, err, domainError.ErrOrganizationInactive)
			f.users.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
		})
	}
}

func TestInvitationService_Accept_EmailTakenSinceCreation(t *testing.T) {
	f := newInvitationFixture(t, true)
	f.acceptable(activeAccess)
	f.users.On("Create", mock.Anything, mock.Anything).Return(domainError.ErrUserEmailTaken)

	_, err := f.service.Accept(context.Background(), invitationToken, &dto.AcceptInvitationRequest{Password: "secreto123"})

	assert.ErrorIs(t, err, domainError.ErrUserEmailTaken)
	f.invitations.AssertNotCalled(t, "MarkAccepted", mock.Anything, mock.Anything, mock.Anything)
}

func TestInvitationService_Accept_BadCredentials(t *testing.T) {
	cases := map[string]struct {
		google bool
		req    dto.AcceptInvitationRequest
		want   error
	}{
		"short password":             {true, dto.AcceptInvitationRequest{Password: "1234567"}, domainError.ErrInvitationCredentials},
		"nothing":                    {true, dto.AcceptInvitationRequest{}, domainError.ErrInvitationCredentials},
		"both":                       {true, dto.AcceptInvitationRequest{Password: "secreto123", IDToken: "t"}, domainError.ErrInvitationCredentials},
		"google without a client id": {false, dto.AcceptInvitationRequest{IDToken: "t"}, domainError.ErrGoogleNotConfigured},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newInvitationFixture(t, tc.google)
			_, err := f.service.Accept(context.Background(), invitationToken, &tc.req)
			assert.ErrorIs(t, err, tc.want)
		})
	}
}
