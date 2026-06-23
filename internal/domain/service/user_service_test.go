package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func createTestUserService(t *testing.T) (*UserService, *mocks.MockUserRepository, *mocks.MockRoleRepository, *mocks.MockUserRoleRepository) {
	mockUserRepo := mocks.NewMockUserRepository(t)
	mockRoleRepo := mocks.NewMockRoleRepository(t)
	mockUserRoleRepo := mocks.NewMockUserRoleRepository(t)
	jwtService := NewJWTService("test-secret-key-for-testing")
	return NewUserService(mockUserRepo, mockRoleRepo, mockUserRoleRepo, jwtService), mockUserRepo, mockRoleRepo, mockUserRoleRepo
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
