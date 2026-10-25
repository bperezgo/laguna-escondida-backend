package service

import (
	"context"
	"errors"
	"fmt"
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
		Email:    "testuser@example.com",
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

func anaFound(t *testing.T, password string, active bool) *dto.UserInOrganization {
	t.Helper()
	u := activeUser(t, password, active)
	email := "ana@example.com"
	u.Email = &email
	return &dto.UserInOrganization{User: u, OrganizationID: testOrgA}
}

func cloudSignIn(f userServiceFixture) (*dto.SignInResponse, error) {
	return f.service.SignIn(context.Background(), &dto.SignInRequest{Email: "ana@example.com", Password: "password123"})
}

func TestUserService_SignIn_CloudSuccess(t *testing.T) {
	f := newUserServiceFixture(t, tenant.AnyOrganization())
	f.userRepo.On("FindByEmail", mock.Anything, "ana@example.com").Return(anaFound(t, "password123", true), nil)
	f.orgRepo.On("Get", mock.Anything, testOrgA).Return(lagunaOrg(dto.OrganizationStatusActive), nil)
	f.userRoleRepo.On("FindRolesByUserID", mock.MatchedBy(inOrg(testOrgA)), "bob-id").Return([]*dto.Role{createTestRole(permissions.RoleManager, "manager")}, nil)

	result, err := cloudSignIn(f)

	require.NoError(t, err)
	assert.Equal(t, "bob", result.Username)
	assert.Equal(t, permissions.RoleManager, result.Roles[0].ID)
	assert.NotEmpty(t, result.Permissions)
	claims, err := f.jwt.ValidateToken(result.Token)
	require.NoError(t, err)
	assert.Equal(t, testOrgA, claims.OrgID)
	assert.Equal(t, "bob-id", claims.UserID)
}

func TestUserService_SignIn_CloudNormalizesTheEmail(t *testing.T) {
	f := newUserServiceFixture(t, tenant.AnyOrganization())
	f.userRepo.On("FindByEmail", mock.Anything, "ana@example.com").Return(anaFound(t, "password123", true), nil)
	f.orgRepo.On("Get", mock.Anything, testOrgA).Return(lagunaOrg(dto.OrganizationStatusActive), nil)
	f.userRoleRepo.On("FindRolesByUserID", mock.Anything, "bob-id").Return([]*dto.Role{createTestRole(permissions.RoleManager, "manager")}, nil)

	result, err := f.service.SignIn(context.Background(), &dto.SignInRequest{Email: "  ANA@Example.com ", Password: "password123"})

	require.NoError(t, err)
	assert.NotEmpty(t, result.Token)
}

func TestUserService_SignIn_CloudAcceptsEveryCloudRole(t *testing.T) {
	for _, roleID := range permissions.CloudRoles {
		t.Run(fmt.Sprintf("role %d", roleID), func(t *testing.T) {
			f := newUserServiceFixture(t, tenant.AnyOrganization())
			f.userRepo.On("FindByEmail", mock.Anything, "ana@example.com").Return(anaFound(t, "password123", true), nil)
			f.orgRepo.On("Get", mock.Anything, testOrgA).Return(lagunaOrg(dto.OrganizationStatusActive), nil)
			f.userRoleRepo.On("FindRolesByUserID", mock.Anything, "bob-id").Return([]*dto.Role{createTestRole(roleID, "cloud")}, nil)

			_, err := cloudSignIn(f)

			require.NoError(t, err)
		})
	}
}

func TestUserService_SignIn_MissingCredential(t *testing.T) {
	cases := []struct {
		name  string
		scope tenant.Scope
		req   *dto.SignInRequest
		want  error
	}{
		{name: "cloud without email", scope: tenant.AnyOrganization(), req: &dto.SignInRequest{Username: "bob", Password: "password123"}, want: domainError.ErrSignInEmailRequired},
		{name: "cloud with a blank email", scope: tenant.AnyOrganization(), req: &dto.SignInRequest{Email: "  ", Password: "password123"}, want: domainError.ErrSignInEmailRequired},
		{name: "edge without username", scope: tenant.PinnedTo(testOrgA), req: &dto.SignInRequest{Email: "ana@example.com", Password: "password123"}, want: domainError.ErrSignInUsernameRequired},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newUserServiceFixture(t, tc.scope)

			result, err := f.service.SignIn(context.Background(), tc.req)

			assert.Nil(t, result)
			assert.ErrorIs(t, err, tc.want)
			assert.NotErrorIs(t, err, domainError.ErrInvalidCredentials)
		})
	}
}

func TestUserService_SignIn_CloudRejectedOpaquely(t *testing.T) {
	cases := []struct {
		name   string
		email  string
		setup  func(t *testing.T, f userServiceFixture)
		reason error
	}{
		{
			name: "unknown email",
			setup: func(t *testing.T, f userServiceFixture) {
				f.userRepo.On("FindByEmail", mock.Anything, "ana@example.com").Return(nil, domainError.ErrUserNotFound)
			},
			reason: domainError.ErrUserNotFound,
		},
		{
			name:  "malformed email",
			email: "not-an-email",
		},
		{
			name: "suspended organization",
			setup: func(t *testing.T, f userServiceFixture) {
				f.userRepo.On("FindByEmail", mock.Anything, "ana@example.com").Return(anaFound(t, "password123", true), nil)
				f.orgRepo.On("Get", mock.Anything, testOrgA).Return(lagunaOrg(dto.OrganizationStatusSuspended), nil)
			},
		},
		{
			name: "wrong password",
			setup: func(t *testing.T, f userServiceFixture) {
				f.userRepo.On("FindByEmail", mock.Anything, "ana@example.com").Return(anaFound(t, "other-password", true), nil)
				f.orgRepo.On("Get", mock.Anything, testOrgA).Return(lagunaOrg(dto.OrganizationStatusActive), nil)
			},
		},
		{
			name: "inactive user",
			setup: func(t *testing.T, f userServiceFixture) {
				f.userRepo.On("FindByEmail", mock.Anything, "ana@example.com").Return(anaFound(t, "password123", false), nil)
				f.orgRepo.On("Get", mock.Anything, testOrgA).Return(lagunaOrg(dto.OrganizationStatusActive), nil)
			},
			reason: domainError.ErrUserInactive,
		},
		{
			name: "floor roles only",
			setup: func(t *testing.T, f userServiceFixture) {
				f.userRepo.On("FindByEmail", mock.Anything, "ana@example.com").Return(anaFound(t, "password123", true), nil)
				f.orgRepo.On("Get", mock.Anything, testOrgA).Return(lagunaOrg(dto.OrganizationStatusActive), nil)
				f.userRoleRepo.On("FindRolesByUserID", mock.Anything, "bob-id").Return([]*dto.Role{
					createTestRole(permissions.RoleServer, "server"), createTestRole(permissions.RoleCooker, "cooker"),
				}, nil)
			},
			reason: domainError.ErrCloudAccessDenied,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newUserServiceFixture(t, tenant.AnyOrganization())
			if tc.setup != nil {
				tc.setup(t, f)
			}
			email := tc.email
			if email == "" {
				email = "ana@example.com"
			}

			result, err := f.service.SignIn(context.Background(), &dto.SignInRequest{Email: email, Password: "password123"})

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
	repoErr := errors.New("connection refused")
	cases := []struct {
		name  string
		scope tenant.Scope
		req   *dto.SignInRequest
		setup func(f userServiceFixture)
	}{
		{
			name: "cloud", scope: tenant.AnyOrganization(), req: &dto.SignInRequest{Email: "ana@example.com", Password: "password123"},
			setup: func(f userServiceFixture) {
				f.userRepo.On("FindByEmail", mock.Anything, "ana@example.com").Return(nil, repoErr)
			},
		},
		{
			name: "edge", scope: tenant.PinnedTo(testOrgA), req: &dto.SignInRequest{Username: "bob", Password: "password123"},
			setup: func(f userServiceFixture) {
				f.orgRepo.On("Get", mock.Anything, testOrgA).Return(lagunaOrg(dto.OrganizationStatusActive), nil)
				f.userRepo.On("FindByUsername", mock.Anything, "bob").Return(nil, repoErr)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newUserServiceFixture(t, tc.scope)
			tc.setup(f)

			result, err := f.service.SignIn(context.Background(), tc.req)

			assert.Nil(t, result)
			assert.NotErrorIs(t, err, domainError.ErrInvalidCredentials)
			assert.ErrorIs(t, err, repoErr)
		})
	}
}

func TestUserService_SignIn_EdgeSuccess(t *testing.T) {
	for _, role := range []*dto.Role{createTestRole(permissions.RoleServer, "server"), createTestRole(permissions.RoleManager, "manager")} {
		t.Run(role.Name, func(t *testing.T) {
			f := newUserServiceFixture(t, tenant.PinnedTo(testOrgA))
			f.orgRepo.On("Get", mock.Anything, testOrgA).Return(lagunaOrg(dto.OrganizationStatusActive), nil)
			f.userRepo.On("FindByUsername", mock.MatchedBy(inOrg(testOrgA)), "bob").Return(activeUser(t, "password123", true), nil)
			f.userRoleRepo.On("FindRolesByUserID", mock.MatchedBy(inOrg(testOrgA)), "bob-id").Return([]*dto.Role{role}, nil)

			result, err := f.service.SignIn(context.Background(), &dto.SignInRequest{Username: "bob", Password: "password123"})

			require.NoError(t, err)
			claims, err := f.jwt.ValidateToken(result.Token)
			require.NoError(t, err)
			assert.Equal(t, testOrgA, claims.OrgID)
			f.userRepo.AssertNotCalled(t, "FindByEmail", mock.Anything, mock.Anything)
		})
	}
}

func TestUserService_SignIn_EdgeRejectedOpaquely(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(t *testing.T, f userServiceFixture)
		reason error
	}{
		{
			name: "username of another organization",
			setup: func(t *testing.T, f userServiceFixture) {
				f.orgRepo.On("Get", mock.Anything, testOrgA).Return(lagunaOrg(dto.OrganizationStatusActive), nil)
				f.userRepo.On("FindByUsername", mock.MatchedBy(inOrg(testOrgA)), "bob").Return(nil, domainError.ErrUserNotFound)
			},
			reason: domainError.ErrUserNotFound,
		},
		{
			name: "wrong password",
			setup: func(t *testing.T, f userServiceFixture) {
				f.orgRepo.On("Get", mock.Anything, testOrgA).Return(lagunaOrg(dto.OrganizationStatusActive), nil)
				f.userRepo.On("FindByUsername", mock.MatchedBy(inOrg(testOrgA)), "bob").Return(activeUser(t, "other-password", true), nil)
			},
		},
		{
			name: "inactive user",
			setup: func(t *testing.T, f userServiceFixture) {
				f.orgRepo.On("Get", mock.Anything, testOrgA).Return(lagunaOrg(dto.OrganizationStatusActive), nil)
				f.userRepo.On("FindByUsername", mock.MatchedBy(inOrg(testOrgA)), "bob").Return(activeUser(t, "password123", false), nil)
			},
			reason: domainError.ErrUserInactive,
		},
		{
			name: "suspended organization",
			setup: func(t *testing.T, f userServiceFixture) {
				f.orgRepo.On("Get", mock.Anything, testOrgA).Return(lagunaOrg(dto.OrganizationStatusSuspended), nil)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newUserServiceFixture(t, tenant.PinnedTo(testOrgA))
			tc.setup(t, f)

			result, err := f.service.SignIn(context.Background(), &dto.SignInRequest{Username: "bob", Password: "password123"})

			assert.Nil(t, result)
			assert.ErrorIs(t, err, domainError.ErrInvalidCredentials)
			if tc.reason != nil {
				assert.ErrorIs(t, err, tc.reason, "the reason is kept for the log")
			}
		})
	}
}

func TestUserService_SignInContext_Cloud(t *testing.T) {
	f := newUserServiceFixture(t, tenant.AnyOrganization())

	result, err := f.service.SignInContext(context.Background())

	require.NoError(t, err)
	assert.Equal(t, &dto.SignInContextResponse{Mode: dto.SignInModeCloud}, result)
	assert.Equal(t, dto.SignInModeCloud, f.service.SignInMode())
	f.orgRepo.AssertNotCalled(t, "Get", mock.Anything, mock.Anything)
}

func TestUserService_SignInContext_Edge(t *testing.T) {
	f := newUserServiceFixture(t, tenant.PinnedTo(testOrgA))
	org := lagunaOrg(dto.OrganizationStatusActive)
	org.Name = "Laguna Escondida"
	f.orgRepo.On("Get", mock.Anything, testOrgA).Return(org, nil)

	result, err := f.service.SignInContext(context.Background())

	require.NoError(t, err)
	assert.Equal(t, &dto.SignInContextResponse{
		Mode:         dto.SignInModeEdge,
		Organization: &dto.SignInContextOrganization{Name: "Laguna Escondida"},
	}, result)
	assert.Equal(t, dto.SignInModeEdge, f.service.SignInMode())
}

// The sign-in page shows the restaurant's brand, not its legal name.
func TestUserService_SignInContext_EdgePrefersTheDisplayName(t *testing.T) {
	f := newUserServiceFixture(t, tenant.PinnedTo(testOrgA))
	org := lagunaOrg(dto.OrganizationStatusActive)
	org.Name = "Laguna Escondida, Restaurante y Pesca S.A.S"
	displayName := "Laguna Escondida"
	org.DisplayName = &displayName
	f.orgRepo.On("Get", mock.Anything, testOrgA).Return(org, nil)

	result, err := f.service.SignInContext(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "Laguna Escondida", result.Organization.Name)
}

func TestUserService_SignInContext_EdgeOrganizationLoadFails(t *testing.T) {
	f := newUserServiceFixture(t, tenant.PinnedTo(testOrgA))
	repoErr := errors.New("connection refused")
	f.orgRepo.On("Get", mock.Anything, testOrgA).Return(nil, repoErr)

	result, err := f.service.SignInContext(context.Background())

	assert.Nil(t, result)
	assert.ErrorIs(t, err, repoErr)
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
	req := &dto.CreateUserRequest{Organization: "laguna-escondida", Username: "admin", Name: "A", Email: "admin@example.com", Password: "password123", RoleIDs: []int{permissions.RoleAdmin}}
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
	req := &dto.CreateUserRequest{Username: "admin", Name: "A", Email: "admin@example.com", Password: "password123", RoleIDs: []int{permissions.RoleAdmin}}
	f.userRepo.On("FindByUsername", mock.MatchedBy(inOrg(testOrgA)), "admin").Return(nil, domainError.ErrUserNotFound)
	f.roleRepo.On("FindByIDs", mock.Anything, req.RoleIDs).Return([]*dto.Role{createTestRole(permissions.RoleAdmin, "admin")}, nil)
	f.userRepo.On("Create", mock.MatchedBy(inOrg(testOrgA)), mock.AnythingOfType("*dto.User")).Return(nil)
	f.userRoleRepo.On("Create", mock.MatchedBy(inOrg(testOrgA)), mock.AnythingOfType("*dto.UserRole")).Return(nil)

	_, err := f.service.CreateUser(context.Background(), req)

	require.NoError(t, err)
}

// Email: unique across all organizations, stored lowercased; required for a cloud role.

func expectRoles(f userServiceFixture, roleIDs []int) {
	f.userRepo.On("FindByUsername", mock.Anything, mock.Anything).Return(nil, domainError.ErrUserNotFound)
	roles := make([]*dto.Role, len(roleIDs))
	for i, id := range roleIDs {
		roles[i] = createTestRole(id, "role")
	}
	f.roleRepo.On("FindByIDs", mock.Anything, roleIDs).Return(roles, nil)
}

func expectCreate(t *testing.T, f userServiceFixture, createErr error) *dto.User {
	created := &dto.User{}
	f.userRepo.On("Create", mock.Anything, mock.AnythingOfType("*dto.User")).Run(func(args mock.Arguments) {
		u, ok := args.Get(1).(*dto.User)
		require.True(t, ok)
		*created = *u
	}).Return(createErr)
	if createErr == nil {
		f.userRoleRepo.On("Create", mock.Anything, mock.AnythingOfType("*dto.UserRole")).Return(nil)
	}
	return created
}

func TestUserService_CreateUser_Email(t *testing.T) {
	cases := []struct {
		name      string
		email     string
		roleIDs   []int
		noCreate  bool
		createErr error
		wantEmail *string
		wantErr   error
	}{
		{name: "manager with a mixed-case email", email: " Ana@Example.com ", roleIDs: []int{permissions.RoleManager}, wantEmail: strPtr("ana@example.com")},
		{name: "floor staff without an email", roleIDs: []int{permissions.RoleServer}},
		{name: "floor staff with an email", email: "cook@example.com", roleIDs: []int{permissions.RoleCooker}, wantEmail: strPtr("cook@example.com")},
		{name: "cloud role without an email", roleIDs: []int{permissions.RoleServer, permissions.RoleAccountant}, noCreate: true, wantErr: domainError.ErrEmailRequired},
		{name: "email taken in another organization", email: "ANA@example.com", roleIDs: []int{permissions.RoleManager}, createErr: domainError.ErrUserEmailTaken, wantErr: domainError.ErrUserEmailTaken},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newUserServiceFixture(t, tenant.AnyOrganization())
			expectRoles(f, tc.roleIDs)
			created := &dto.User{}
			if !tc.noCreate {
				created = expectCreate(t, f, tc.createErr)
			}
			ctx := tenant.WithOrg(context.Background(), testOrgB)

			result, err := f.service.CreateUser(ctx, &dto.CreateUserRequest{
				Username: "ana", Name: "Ana", Email: tc.email, Password: "password123", RoleIDs: tc.roleIDs,
			})

			if tc.wantErr != nil {
				assert.Nil(t, result)
				assert.ErrorIs(t, err, tc.wantErr)
				if tc.noCreate {
					f.userRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
				}
				f.userRoleRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantEmail, created.Email)
			assert.Equal(t, tc.wantEmail, result.User.Email)
		})
	}
}

func TestUserService_CreateUser_InvalidEmail(t *testing.T) {
	f := newUserServiceFixture(t, tenant.AnyOrganization())
	f.userRepo.On("FindByUsername", mock.Anything, "ana").Return(nil, domainError.ErrUserNotFound)
	f.roleRepo.On("FindByIDs", mock.Anything, []int{permissions.RoleManager}).Return([]*dto.Role{createTestRole(permissions.RoleManager, "manager")}, nil)

	result, err := f.service.CreateUser(tenant.WithOrg(context.Background(), testOrgA), &dto.CreateUserRequest{
		Username: "ana", Name: "Ana", Email: "Ana <ana@example.com>", Password: "password123", RoleIDs: []int{permissions.RoleManager},
	})

	assert.Nil(t, result)
	assert.ErrorContains(t, err, "email is not a valid address")
	f.userRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestUserService_UpdateUser_Email(t *testing.T) {
	cases := []struct {
		name         string
		currentEmail *string
		currentRole  int
		req          *dto.UpdateUserRequest
		updateErr    error
		wantEmail    *string
		wantErr      error
	}{
		{name: "promote floor staff with an email", currentRole: permissions.RoleServer, req: &dto.UpdateUserRequest{RoleIDs: []int{permissions.RoleManager}, Email: strPtr("ANA@example.com")}, wantEmail: strPtr("ana@example.com")},
		{name: "promote floor staff without an email", currentRole: permissions.RoleServer, req: &dto.UpdateUserRequest{RoleIDs: []int{permissions.RoleManager}}, wantErr: domainError.ErrEmailRequired},
		{name: "clear a manager's email", currentEmail: strPtr("ana@example.com"), currentRole: permissions.RoleManager, req: &dto.UpdateUserRequest{Email: strPtr("")}, wantErr: domainError.ErrEmailRequired},
		{name: "rename a manager who has no email yet", currentRole: permissions.RoleManager, req: &dto.UpdateUserRequest{Name: strPtr("Ana")}, wantErr: domainError.ErrEmailRequired},
		{name: "demote and clear the email together", currentEmail: strPtr("ana@example.com"), currentRole: permissions.RoleManager, req: &dto.UpdateUserRequest{RoleIDs: []int{permissions.RoleServer}, Email: strPtr("")}},
		{name: "keep the email when the request omits it", currentEmail: strPtr("ana@example.com"), currentRole: permissions.RoleManager, req: &dto.UpdateUserRequest{Name: strPtr("Ana")}, wantEmail: strPtr("ana@example.com")},
		{name: "email taken in another organization", currentRole: permissions.RoleServer, req: &dto.UpdateUserRequest{Email: strPtr("ana@example.com")}, updateErr: domainError.ErrUserEmailTaken, wantErr: domainError.ErrUserEmailTaken},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newUserServiceFixture(t, tenant.AnyOrganization())
			ctx := tenant.WithOrg(context.Background(), testOrgA)
			current := &dto.User{ID: "ana-id", Username: "ana", Active: true, Email: tc.currentEmail}
			f.userRepo.On("FindByID", ctx, "ana-id").Return(current, nil)
			f.userRoleRepo.On("FindRolesByUserID", ctx, "ana-id").Return([]*dto.Role{createTestRole(tc.currentRole, "current")}, nil)
			if len(tc.req.RoleIDs) > 0 {
				roles := []*dto.Role{createTestRole(tc.req.RoleIDs[0], "new")}
				f.roleRepo.On("FindByIDs", ctx, tc.req.RoleIDs).Return(roles, nil)
				f.userRoleRepo.On("DeleteByUserID", ctx, "ana-id").Return(nil).Maybe()
				f.userRoleRepo.On("Create", ctx, mock.AnythingOfType("*dto.UserRole")).Return(nil).Maybe()
			}
			if tc.wantErr == nil || tc.updateErr != nil {
				f.userRepo.On("Update", ctx, mock.AnythingOfType("*dto.User")).Return(tc.updateErr)
			}

			result, err := f.service.UpdateUser(ctx, "acting-id", "ana-id", tc.req)

			if tc.wantErr != nil {
				assert.Nil(t, result)
				assert.ErrorIs(t, err, tc.wantErr)
				if tc.updateErr == nil {
					f.userRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantEmail, result.User.Email)
		})
	}
}
