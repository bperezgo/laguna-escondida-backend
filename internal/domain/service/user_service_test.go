package service

import (
	"context"
	"errors"
	"testing"
	"time"

	useragg "laguna-escondida/backend/internal/domain/aggregate/user"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// passthroughUnitOfWork runs the transactional callback inline, so tests exercise
// the same code path without a real database transaction.
type passthroughUnitOfWork struct{}

func (passthroughUnitOfWork) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func createTestUserService(t *testing.T) (*UserService, *mocks.MockUserRepository, *mocks.MockRoleRepository, *mocks.MockUserRoleRepository) {
	f := newUserServiceFixture(t, tenant.AnyOrganization())
	return f.service, f.userRepo, f.roleRepo, f.userRoleRepo
}

type userServiceFixture struct {
	service      *UserService
	userRepo     *mocks.MockUserRepository
	roleRepo     *mocks.MockRoleRepository
	userRoleRepo *mocks.MockUserRoleRepository
	orgRepo      *mocks.MockOrganizationRepository
	jwt          *JWTService
}

func newUserServiceFixture(t *testing.T, scope tenant.Scope) userServiceFixture {
	f := userServiceFixture{
		userRepo:     mocks.NewMockUserRepository(t),
		roleRepo:     mocks.NewMockRoleRepository(t),
		userRoleRepo: mocks.NewMockUserRoleRepository(t),
		orgRepo:      mocks.NewMockOrganizationRepository(t),
		jwt:          NewJWTService("test-secret-key-for-testing", scope),
	}
	f.service = NewUserService(f.userRepo, f.roleRepo, f.userRoleRepo, f.orgRepo, f.jwt, passthroughUnitOfWork{}, scope)
	return f
}

// inOrg reports whether a context acts for orgID; use it with mock.MatchedBy.
func inOrg(orgID string) func(ctx context.Context) bool {
	return func(ctx context.Context) bool {
		got, ok := tenant.OrgFrom(ctx)
		return ok && got == orgID
	}
}

func createTestRole(id int, name string) *dto.Role {
	return &dto.Role{
		ID:        id,
		Name:      name,
		CreatedAt: time.Now(),
	}
}

// Success Cases
func TestCreateUser_Success(t *testing.T) {
	ctx := tenant.WithOrg(context.Background(), testOrgA)
	service, userRepo, roleRepo, userRoleRepo := createTestUserService(t)

	req := &dto.CreateUserRequest{
		Username: "testuser",
		Name:     "Test User",
		Password: "password123",
		RoleIDs:  []int{1, 2},
	}

	roles := []*dto.Role{
		createTestRole(1, "admin"),
		createTestRole(2, "manager"),
	}

	userRepo.On("FindByUsername", ctx, req.Username).Return(nil, domainError.ErrUserNotFound)
	roleRepo.On("FindByIDs", ctx, req.RoleIDs).Return(roles, nil)
	userRepo.On("Create", ctx, mock.AnythingOfType("*dto.User")).Run(func(args mock.Arguments) {
		user, ok := args.Get(1).(*dto.User)
		if !ok {
			panic("user is not a *dto.User")
		}
		user.ID = "test-user-id"
	}).Return(nil)
	userRoleRepo.On("Create", ctx, mock.AnythingOfType("*dto.UserRole")).Return(nil).Times(2)

	result, err := service.CreateUser(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "testuser", result.User.Username)
	assert.Equal(t, 2, len(result.Roles))
	assert.Equal(t, "admin", result.Roles[0].Name)
	assert.Equal(t, "manager", result.Roles[1].Name)
	assert.NotEmpty(t, result.User.ID)
	assert.NotEmpty(t, result.User.Password)
	assert.NotEqual(t, req.Password, result.User.Password)

}

// Error Cases
func TestCreateUser_UserAlreadyExists(t *testing.T) {
	ctx := tenant.WithOrg(context.Background(), testOrgA)
	service, userRepo, roleRepo, userRoleRepo := createTestUserService(t)

	req := &dto.CreateUserRequest{
		Username: "existinguser",
		Name:     "Existing User",
		Password: "password123",
		RoleIDs:  []int{1},
	}

	existingUser := &dto.User{
		ID:       "existing-id",
		Username: "existinguser",
	}

	userRepo.On("FindByUsername", ctx, req.Username).Return(existingUser, nil)

	result, err := service.CreateUser(ctx, req)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, domainError.ErrUserAlreadyExists))

	roleRepo.AssertNotCalled(t, "FindByIDs")
	userRoleRepo.AssertNotCalled(t, "Create")
}

func TestCreateUser_FindByUsernameError(t *testing.T) {
	ctx := tenant.WithOrg(context.Background(), testOrgA)
	service, userRepo, roleRepo, userRoleRepo := createTestUserService(t)

	req := &dto.CreateUserRequest{
		Username: "testuser",
		Name:     "Test User",
		Password: "password123",
		RoleIDs:  []int{1},
	}

	userRepo.On("FindByUsername", ctx, req.Username).Return(nil, errors.New("connection refused"))

	result, err := service.CreateUser(ctx, req)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check existing user")
	assert.False(t, errors.Is(err, domainError.ErrUserAlreadyExists))

	roleRepo.AssertNotCalled(t, "FindByIDs")
	userRepo.AssertNotCalled(t, "Create")
	userRoleRepo.AssertNotCalled(t, "Create")
}

func TestCreateUser_MissingName(t *testing.T) {
	ctx := tenant.WithOrg(context.Background(), testOrgA)
	service, userRepo, roleRepo, userRoleRepo := createTestUserService(t)

	req := &dto.CreateUserRequest{
		Username: "testuser",
		Name:     "",
		Password: "password123",
		RoleIDs:  []int{1},
	}

	roles := []*dto.Role{
		createTestRole(1, "admin"),
	}

	userRepo.On("FindByUsername", ctx, req.Username).Return(nil, domainError.ErrUserNotFound)
	roleRepo.On("FindByIDs", ctx, req.RoleIDs).Return(roles, nil)

	result, err := service.CreateUser(ctx, req)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")

	userRepo.AssertNotCalled(t, "Create")
	userRoleRepo.AssertNotCalled(t, "Create")
}

func TestCreateUser_RoleNotFound(t *testing.T) {
	ctx := tenant.WithOrg(context.Background(), testOrgA)
	service, userRepo, roleRepo, userRoleRepo := createTestUserService(t)

	req := &dto.CreateUserRequest{
		Username: "testuser",
		Name:     "Test User",
		Password: "password123",
		RoleIDs:  []int{999},
	}

	userRepo.On("FindByUsername", ctx, req.Username).Return(nil, domainError.ErrUserNotFound)
	roleRepo.On("FindByIDs", ctx, req.RoleIDs).Return(nil, errors.New("role not found"))

	result, err := service.CreateUser(ctx, req)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, domainError.ErrRoleNotFound))

	roleRepo.AssertExpectations(t)
	userRoleRepo.AssertNotCalled(t, "Create")
}

func TestCreateUser_InvalidRoleIDs(t *testing.T) {
	ctx := tenant.WithOrg(context.Background(), testOrgA)
	service, userRepo, roleRepo, userRoleRepo := createTestUserService(t)

	req := &dto.CreateUserRequest{
		Username: "testuser",
		Name:     "Test User",
		Password: "password123",
		RoleIDs:  []int{1, 2},
	}

	roles := []*dto.Role{
		createTestRole(1, "admin"),
	}

	userRepo.On("FindByUsername", ctx, req.Username).Return(nil, domainError.ErrUserNotFound)
	roleRepo.On("FindByIDs", ctx, req.RoleIDs).Return(roles, nil)

	result, err := service.CreateUser(ctx, req)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, domainError.ErrInvalidRoleIDs))

	roleRepo.AssertExpectations(t)
	userRoleRepo.AssertNotCalled(t, "Create")
}

func TestCreateUser_UserCreationFailed(t *testing.T) {
	ctx := tenant.WithOrg(context.Background(), testOrgA)
	service, userRepo, roleRepo, userRoleRepo := createTestUserService(t)

	req := &dto.CreateUserRequest{
		Username: "testuser",
		Name:     "Test User",
		Password: "password123",
		RoleIDs:  []int{1},
	}

	roles := []*dto.Role{
		createTestRole(1, "admin"),
	}

	userRepo.On("FindByUsername", ctx, req.Username).Return(nil, domainError.ErrUserNotFound)
	roleRepo.On("FindByIDs", ctx, req.RoleIDs).Return(roles, nil)
	userRepo.On("Create", ctx, mock.AnythingOfType("*dto.User")).Return(errors.New("database error"))

	result, err := service.CreateUser(ctx, req)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, domainError.ErrUserCreationFailed))

	roleRepo.AssertExpectations(t)
	userRoleRepo.AssertNotCalled(t, "Create")
}

func TestCreateUser_UserRoleAssignmentFailed(t *testing.T) {
	ctx := tenant.WithOrg(context.Background(), testOrgA)
	service, userRepo, roleRepo, userRoleRepo := createTestUserService(t)

	req := &dto.CreateUserRequest{
		Username: "testuser",
		Name:     "Test User",
		Password: "password123",
		RoleIDs:  []int{1},
	}

	roles := []*dto.Role{
		createTestRole(1, "admin"),
	}

	userRepo.On("FindByUsername", ctx, req.Username).Return(nil, domainError.ErrUserNotFound)
	roleRepo.On("FindByIDs", ctx, req.RoleIDs).Return(roles, nil)
	userRepo.On("Create", ctx, mock.AnythingOfType("*dto.User")).Run(func(args mock.Arguments) {
		user, ok := args.Get(1).(*dto.User)
		if !ok {
			panic("user is not a *dto.User")
		}
		user.ID = "test-user-id"
	}).Return(nil)
	userRoleRepo.On("Create", ctx, mock.AnythingOfType("*dto.UserRole")).Return(errors.New("database error"))

	result, err := service.CreateUser(ctx, req)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to assign role to user")

}

// Guard: an admin cannot delete their own user.
func TestDeleteUser_CannotDeleteSelf(t *testing.T) {
	ctx := context.Background()
	service, userRepo, _, _ := createTestUserService(t)

	err := service.DeleteUser(ctx, "same-id", "same-id")

	assert.True(t, errors.Is(err, domainError.ErrCannotDeleteSelf))
	userRepo.AssertNotCalled(t, "FindByID")
	userRepo.AssertNotCalled(t, "SoftDelete")
}

// Guard: deleting the last active admin is rejected.
func TestDeleteUser_CannotRemoveLastAdmin(t *testing.T) {
	ctx := context.Background()
	service, userRepo, _, userRoleRepo := createTestUserService(t)

	userRepo.On("FindByID", ctx, "admin-id").Return(&dto.User{ID: "admin-id", Active: true}, nil)
	userRoleRepo.On("FindRolesByUserID", ctx, "admin-id").Return([]*dto.Role{createTestRole(permissions.RoleAdmin, "admin")}, nil)
	userRoleRepo.On("CountUsersByRoleID", ctx, permissions.RoleAdmin).Return(1, nil)

	err := service.DeleteUser(ctx, "acting-id", "admin-id")

	assert.True(t, errors.Is(err, domainError.ErrCannotRemoveLastAdmin))
	userRepo.AssertNotCalled(t, "SoftDelete")
}

func TestDeleteUser_Success(t *testing.T) {
	ctx := context.Background()
	service, userRepo, _, userRoleRepo := createTestUserService(t)

	userRepo.On("FindByID", ctx, "user-id").Return(&dto.User{ID: "user-id", Active: true}, nil)
	userRoleRepo.On("FindRolesByUserID", ctx, "user-id").Return([]*dto.Role{createTestRole(permissions.RoleServer, "server")}, nil)
	userRepo.On("SoftDelete", ctx, "user-id").Return(nil)

	err := service.DeleteUser(ctx, "acting-id", "user-id")

	require.NoError(t, err)
}

// Guard: removing the admin role from the last active admin is rejected.
func TestUpdateUser_CannotRemoveLastAdminRole(t *testing.T) {
	ctx := context.Background()
	service, userRepo, roleRepo, userRoleRepo := createTestUserService(t)

	userRepo.On("FindByID", ctx, "admin-id").Return(&dto.User{ID: "admin-id", Active: true, Name: "Admin"}, nil)
	userRoleRepo.On("FindRolesByUserID", ctx, "admin-id").Return([]*dto.Role{createTestRole(permissions.RoleAdmin, "admin")}, nil)
	roleRepo.On("FindByIDs", ctx, []int{permissions.RoleServer}).Return([]*dto.Role{createTestRole(permissions.RoleServer, "server")}, nil)
	userRoleRepo.On("CountUsersByRoleID", ctx, permissions.RoleAdmin).Return(1, nil)

	req := &dto.UpdateUserRequest{RoleIDs: []int{permissions.RoleServer}}
	result, err := service.UpdateUser(ctx, "acting-id", "admin-id", req)

	assert.Nil(t, result)
	assert.True(t, errors.Is(err, domainError.ErrCannotRemoveLastAdmin))
	userRepo.AssertNotCalled(t, "Update")
}

// Guard: an admin cannot deactivate their own user.
func TestUpdateUser_CannotDeactivateSelf(t *testing.T) {
	ctx := context.Background()
	service, userRepo, _, userRoleRepo := createTestUserService(t)

	userRepo.On("FindByID", ctx, "me").Return(&dto.User{ID: "me", Active: true}, nil)
	userRoleRepo.On("FindRolesByUserID", ctx, "me").Return([]*dto.Role{createTestRole(permissions.RoleAdmin, "admin")}, nil)

	inactive := false
	req := &dto.UpdateUserRequest{Active: &inactive}
	result, err := service.UpdateUser(ctx, "me", "me", req)

	assert.Nil(t, result)
	assert.True(t, errors.Is(err, domainError.ErrCannotDeactivateSelf))
	userRepo.AssertNotCalled(t, "Update")
}

// Sign-in: every rejection is an opaque ErrInvalidCredentials (the handler answers one 401 for
// all of them) while still wrapping the specific reason for the server-side log.

func lagunaOrg(status string) *dto.Organization {
	return &dto.Organization{ID: testOrgA, Slug: "laguna-escondida", Status: status}
}

func activeUser(t *testing.T, password string, active bool) *dto.User {
	t.Helper()
	hashed, err := useragg.HashPassword(password)
	require.NoError(t, err)
	return &dto.User{ID: "bob-id", Username: "bob", Password: hashed, Active: active}
}

func TestUserService_SignIn_CloudSuccess(t *testing.T) {
	f := newUserServiceFixture(t, tenant.AnyOrganization())
	f.orgRepo.On("GetBySlug", mock.Anything, "laguna-escondida").Return(lagunaOrg(dto.OrganizationStatusActive), nil)
	f.userRepo.On("FindByUsername", mock.MatchedBy(inOrg(testOrgA)), "bob").Return(activeUser(t, "password123", true), nil)
	f.userRoleRepo.On("FindRolesByUserID", mock.MatchedBy(inOrg(testOrgA)), "bob-id").Return([]*dto.Role{createTestRole(permissions.RoleServer, "server")}, nil)

	result, err := f.service.SignIn(context.Background(), &dto.SignInRequest{Organization: "laguna-escondida", Username: "bob", Password: "password123"})

	require.NoError(t, err)
	assert.Equal(t, "bob", result.Username)
	claims, err := f.jwt.ValidateToken(result.Token)
	require.NoError(t, err)
	assert.Equal(t, testOrgA, claims.OrgID)
	assert.Equal(t, "bob-id", claims.UserID)
}

func TestUserService_SignIn_CloudWithoutOrganization(t *testing.T) {
	f := newUserServiceFixture(t, tenant.AnyOrganization())

	result, err := f.service.SignIn(context.Background(), &dto.SignInRequest{Username: "bob", Password: "password123"})

	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainError.ErrOrganizationRequired)
}

func TestUserService_SignIn_RejectedOpaquely(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(t *testing.T, f userServiceFixture)
		reason error
	}{
		{
			name: "unknown organization",
			setup: func(t *testing.T, f userServiceFixture) {
				f.orgRepo.On("GetBySlug", mock.Anything, "laguna-escondida").Return(nil, domainError.ErrOrganizationNotFound)
			},
			reason: domainError.ErrOrganizationNotFound,
		},
		{
			name: "suspended organization",
			setup: func(t *testing.T, f userServiceFixture) {
				f.orgRepo.On("GetBySlug", mock.Anything, "laguna-escondida").Return(lagunaOrg(dto.OrganizationStatusSuspended), nil)
			},
		},
		{
			name: "username of another organization",
			setup: func(t *testing.T, f userServiceFixture) {
				f.orgRepo.On("GetBySlug", mock.Anything, "laguna-escondida").Return(lagunaOrg(dto.OrganizationStatusActive), nil)
				f.userRepo.On("FindByUsername", mock.MatchedBy(inOrg(testOrgA)), "bob").Return(nil, domainError.ErrUserNotFound)
			},
			reason: domainError.ErrUserNotFound,
		},
		{
			name: "wrong password",
			setup: func(t *testing.T, f userServiceFixture) {
				f.orgRepo.On("GetBySlug", mock.Anything, "laguna-escondida").Return(lagunaOrg(dto.OrganizationStatusActive), nil)
				f.userRepo.On("FindByUsername", mock.MatchedBy(inOrg(testOrgA)), "bob").Return(activeUser(t, "other-password", true), nil)
			},
		},
		{
			name: "inactive user",
			setup: func(t *testing.T, f userServiceFixture) {
				f.orgRepo.On("GetBySlug", mock.Anything, "laguna-escondida").Return(lagunaOrg(dto.OrganizationStatusActive), nil)
				f.userRepo.On("FindByUsername", mock.MatchedBy(inOrg(testOrgA)), "bob").Return(activeUser(t, "password123", false), nil)
			},
			reason: domainError.ErrUserInactive,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newUserServiceFixture(t, tenant.AnyOrganization())
			tc.setup(t, f)

			result, err := f.service.SignIn(context.Background(), &dto.SignInRequest{Organization: "laguna-escondida", Username: "bob", Password: "password123"})

			assert.Nil(t, result)
			assert.ErrorIs(t, err, domainError.ErrInvalidCredentials)
			if tc.reason != nil {
				assert.ErrorIs(t, err, tc.reason, "the reason is kept for the log")
			}
		})
	}
}

// An infrastructure error must not be masked as invalid credentials, so it surfaces as a 500.
func TestUserService_SignIn_RepositoryError(t *testing.T) {
	f := newUserServiceFixture(t, tenant.AnyOrganization())
	repoErr := errors.New("connection refused")
	f.orgRepo.On("GetBySlug", mock.Anything, "laguna-escondida").Return(lagunaOrg(dto.OrganizationStatusActive), nil)
	f.userRepo.On("FindByUsername", mock.Anything, "bob").Return(nil, repoErr)

	result, err := f.service.SignIn(context.Background(), &dto.SignInRequest{Organization: "laguna-escondida", Username: "bob", Password: "password123"})

	assert.Nil(t, result)
	assert.NotErrorIs(t, err, domainError.ErrInvalidCredentials)
	assert.ErrorIs(t, err, repoErr)
}

func TestUserService_SignIn_EdgeWithoutOrganization(t *testing.T) {
	f := newUserServiceFixture(t, tenant.PinnedTo(testOrgA))
	f.orgRepo.On("Get", mock.Anything, testOrgA).Return(lagunaOrg(dto.OrganizationStatusActive), nil)
	f.userRepo.On("FindByUsername", mock.MatchedBy(inOrg(testOrgA)), "bob").Return(activeUser(t, "password123", true), nil)
	f.userRoleRepo.On("FindRolesByUserID", mock.MatchedBy(inOrg(testOrgA)), "bob-id").Return([]*dto.Role{createTestRole(permissions.RoleServer, "server")}, nil)

	result, err := f.service.SignIn(context.Background(), &dto.SignInRequest{Username: "bob", Password: "password123"})

	require.NoError(t, err)
	claims, err := f.jwt.ValidateToken(result.Token)
	require.NoError(t, err)
	assert.Equal(t, testOrgA, claims.OrgID)
}

func TestUserService_SignIn_EdgeNamingAnotherOrganization(t *testing.T) {
	f := newUserServiceFixture(t, tenant.PinnedTo(testOrgA))
	f.orgRepo.On("GetBySlug", mock.Anything, "other-org").Return(&dto.Organization{ID: testOrgB, Slug: "other-org", Status: dto.OrganizationStatusActive}, nil)

	result, err := f.service.SignIn(context.Background(), &dto.SignInRequest{Organization: "other-org", Username: "bob", Password: "password123"})

	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainError.ErrInvalidCredentials)
	f.userRepo.AssertNotCalled(t, "FindByUsername", mock.Anything, mock.Anything)
}

// User creation: a signed-in creator's organization always wins; only the admin-key
// bootstrap (no organization in the context) names one.

func TestUserService_CreateUser_IgnoresOrganizationInBodyWhenSignedIn(t *testing.T) {
	f := newUserServiceFixture(t, tenant.AnyOrganization())
	ctx := tenant.WithOrg(context.Background(), testOrgA)
	req := &dto.CreateUserRequest{Organization: "other-org", Username: "cajero1", Name: "C", Password: "password123", RoleIDs: []int{permissions.RoleServer}}
	f.userRepo.On("FindByUsername", mock.MatchedBy(inOrg(testOrgA)), "cajero1").Return(nil, domainError.ErrUserNotFound)
	f.roleRepo.On("FindByIDs", mock.Anything, req.RoleIDs).Return([]*dto.Role{createTestRole(permissions.RoleServer, "server")}, nil)
	f.userRepo.On("Create", mock.MatchedBy(inOrg(testOrgA)), mock.AnythingOfType("*dto.User")).Return(nil)
	f.userRoleRepo.On("Create", mock.MatchedBy(inOrg(testOrgA)), mock.AnythingOfType("*dto.UserRole")).Return(nil)

	_, err := f.service.CreateUser(ctx, req)

	require.NoError(t, err)
	f.orgRepo.AssertNotCalled(t, "GetBySlug", mock.Anything, mock.Anything)
}

func TestUserService_CreateUser_BootstrapNamesOrganization(t *testing.T) {
	f := newUserServiceFixture(t, tenant.AnyOrganization())
	req := &dto.CreateUserRequest{Organization: "laguna-escondida", Username: "admin", Name: "A", Password: "password123", RoleIDs: []int{permissions.RoleAdmin}}
	f.orgRepo.On("GetBySlug", mock.Anything, "laguna-escondida").Return(lagunaOrg(dto.OrganizationStatusActive), nil)
	f.userRepo.On("FindByUsername", mock.MatchedBy(inOrg(testOrgA)), "admin").Return(nil, domainError.ErrUserNotFound)
	f.roleRepo.On("FindByIDs", mock.Anything, req.RoleIDs).Return([]*dto.Role{createTestRole(permissions.RoleAdmin, "admin")}, nil)
	f.userRepo.On("Create", mock.MatchedBy(inOrg(testOrgA)), mock.AnythingOfType("*dto.User")).Return(nil)
	f.userRoleRepo.On("Create", mock.MatchedBy(inOrg(testOrgA)), mock.AnythingOfType("*dto.UserRole")).Return(nil)

	_, err := f.service.CreateUser(context.Background(), req)

	require.NoError(t, err)
}

func TestUserService_CreateUser_BootstrapRejected(t *testing.T) {
	cases := []struct {
		name  string
		scope tenant.Scope
		slug  string
		setup func(f userServiceFixture)
		want  error
	}{
		{name: "cloud without organization", scope: tenant.AnyOrganization(), want: domainError.ErrOrganizationRequired},
		{
			name: "cloud with unknown organization", scope: tenant.AnyOrganization(), slug: "nope",
			setup: func(f userServiceFixture) {
				f.orgRepo.On("GetBySlug", mock.Anything, "nope").Return(nil, domainError.ErrOrganizationNotFound)
			},
			want: domainError.ErrOrganizationNotFound,
		},
		{
			name: "edge naming another organization", scope: tenant.PinnedTo(testOrgA), slug: "other-org",
			setup: func(f userServiceFixture) {
				f.orgRepo.On("GetBySlug", mock.Anything, "other-org").Return(&dto.Organization{ID: testOrgB, Slug: "other-org"}, nil)
			},
			want: domainError.ErrOrganizationMismatch,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newUserServiceFixture(t, tc.scope)
			if tc.setup != nil {
				tc.setup(f)
			}

			result, err := f.service.CreateUser(context.Background(), &dto.CreateUserRequest{
				Organization: tc.slug, Username: "admin", Name: "A", Password: "password123", RoleIDs: []int{permissions.RoleAdmin},
			})

			assert.Nil(t, result)
			assert.ErrorIs(t, err, tc.want)
			f.userRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
		})
	}
}

func TestUserService_CreateUser_EdgeBootstrapUsesTheBoxOrganization(t *testing.T) {
	f := newUserServiceFixture(t, tenant.PinnedTo(testOrgA))
	req := &dto.CreateUserRequest{Username: "admin", Name: "A", Password: "password123", RoleIDs: []int{permissions.RoleAdmin}}
	f.userRepo.On("FindByUsername", mock.MatchedBy(inOrg(testOrgA)), "admin").Return(nil, domainError.ErrUserNotFound)
	f.roleRepo.On("FindByIDs", mock.Anything, req.RoleIDs).Return([]*dto.Role{createTestRole(permissions.RoleAdmin, "admin")}, nil)
	f.userRepo.On("Create", mock.MatchedBy(inOrg(testOrgA)), mock.AnythingOfType("*dto.User")).Return(nil)
	f.userRoleRepo.On("Create", mock.MatchedBy(inOrg(testOrgA)), mock.AnythingOfType("*dto.UserRole")).Return(nil)

	_, err := f.service.CreateUser(context.Background(), req)

	require.NoError(t, err)
}
