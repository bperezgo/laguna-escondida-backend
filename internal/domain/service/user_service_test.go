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
	mockUserRepo := mocks.NewMockUserRepository(t)
	mockRoleRepo := mocks.NewMockRoleRepository(t)
	mockUserRoleRepo := mocks.NewMockUserRoleRepository(t)
	jwtService := NewJWTService("test-secret-key-for-testing")
	return NewUserService(mockUserRepo, mockRoleRepo, mockUserRoleRepo, jwtService, passthroughUnitOfWork{}), mockUserRepo, mockRoleRepo, mockUserRoleRepo
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
	ctx := context.Background()
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
	ctx := context.Background()
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
	ctx := context.Background()
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
	ctx := context.Background()
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
	ctx := context.Background()
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
	ctx := context.Background()
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
	ctx := context.Background()
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
	ctx := context.Background()
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

// An inactive user cannot sign in even with the correct password.
func TestSignIn_InactiveUser(t *testing.T) {
	ctx := context.Background()
	service, userRepo, _, _ := createTestUserService(t)

	hashed, err := useragg.HashPassword("password123")
	require.NoError(t, err)

	userRepo.On("FindByUsername", ctx, "bob").Return(&dto.User{
		ID:       "bob-id",
		Username: "bob",
		Password: hashed,
		Active:   false,
	}, nil)

	result, err := service.SignIn(ctx, &dto.SignInRequest{Username: "bob", Password: "password123"})

	assert.Nil(t, result)
	assert.True(t, errors.Is(err, domainError.ErrUserInactive))
}
