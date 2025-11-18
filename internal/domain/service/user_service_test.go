package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(ctx context.Context, user *dto.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) FindByUsername(ctx context.Context, username string) (*dto.User, error) {
	args := m.Called(ctx, username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.User), args.Error(1)
}

func (m *MockUserRepository) FindByID(ctx context.Context, id string) (*dto.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.User), args.Error(1)
}

type MockRoleRepository struct {
	mock.Mock
}

func (m *MockRoleRepository) FindByID(ctx context.Context, id int) (*dto.Role, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.Role), args.Error(1)
}

func (m *MockRoleRepository) FindByIDs(ctx context.Context, ids []int) ([]*dto.Role, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*dto.Role), args.Error(1)
}

func (m *MockRoleRepository) FindByName(ctx context.Context, name string) (*dto.Role, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.Role), args.Error(1)
}

type MockUserRoleRepository struct {
	mock.Mock
}

func (m *MockUserRoleRepository) Create(ctx context.Context, userRole *dto.UserRole) error {
	args := m.Called(ctx, userRole)
	return args.Error(0)
}

func (m *MockUserRoleRepository) FindByUserID(ctx context.Context, userID string) ([]*dto.UserRole, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*dto.UserRole), args.Error(1)
}

func (m *MockUserRoleRepository) FindRolesByUserID(ctx context.Context, userID string) ([]*dto.Role, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*dto.Role), args.Error(1)
}

func createTestUserService(userRepo *MockUserRepository, roleRepo *MockRoleRepository, userRoleRepo *MockUserRoleRepository) *UserService {
	return NewUserService(userRepo, roleRepo, userRoleRepo)
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
	userRepo := new(MockUserRepository)
	roleRepo := new(MockRoleRepository)
	userRoleRepo := new(MockUserRoleRepository)
	service := createTestUserService(userRepo, roleRepo, userRoleRepo)

	req := &dto.CreateUserRequest{
		Username: "testuser",
		Password: "password123",
		RoleIDs:  []int{1, 2},
	}

	roles := []*dto.Role{
		createTestRole(1, "admin"),
		createTestRole(2, "manager"),
	}

	userRepo.On("FindByUsername", ctx, req.Username).Return(nil, errors.New("not found"))
	roleRepo.On("FindByIDs", ctx, req.RoleIDs).Return(roles, nil)
	userRepo.On("Create", ctx, mock.AnythingOfType("*dto.User")).Run(func(args mock.Arguments) {
		user := args.Get(1).(*dto.User)
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

	userRepo.AssertExpectations(t)
	roleRepo.AssertExpectations(t)
	userRoleRepo.AssertExpectations(t)
}

// Error Cases
func TestCreateUser_UserAlreadyExists(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepository)
	roleRepo := new(MockRoleRepository)
	userRoleRepo := new(MockUserRoleRepository)
	service := createTestUserService(userRepo, roleRepo, userRoleRepo)

	req := &dto.CreateUserRequest{
		Username: "existinguser",
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

	userRepo.AssertExpectations(t)
	roleRepo.AssertNotCalled(t, "FindByIDs")
	userRoleRepo.AssertNotCalled(t, "Create")
}

func TestCreateUser_RoleNotFound(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepository)
	roleRepo := new(MockRoleRepository)
	userRoleRepo := new(MockUserRoleRepository)
	service := createTestUserService(userRepo, roleRepo, userRoleRepo)

	req := &dto.CreateUserRequest{
		Username: "testuser",
		Password: "password123",
		RoleIDs:  []int{999},
	}

	userRepo.On("FindByUsername", ctx, req.Username).Return(nil, errors.New("not found"))
	roleRepo.On("FindByIDs", ctx, req.RoleIDs).Return(nil, errors.New("role not found"))

	result, err := service.CreateUser(ctx, req)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, domainError.ErrRoleNotFound))

	userRepo.AssertExpectations(t)
	roleRepo.AssertExpectations(t)
	userRoleRepo.AssertNotCalled(t, "Create")
}

func TestCreateUser_InvalidRoleIDs(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepository)
	roleRepo := new(MockRoleRepository)
	userRoleRepo := new(MockUserRoleRepository)
	service := createTestUserService(userRepo, roleRepo, userRoleRepo)

	req := &dto.CreateUserRequest{
		Username: "testuser",
		Password: "password123",
		RoleIDs:  []int{1, 2},
	}

	roles := []*dto.Role{
		createTestRole(1, "admin"),
	}

	userRepo.On("FindByUsername", ctx, req.Username).Return(nil, errors.New("not found"))
	roleRepo.On("FindByIDs", ctx, req.RoleIDs).Return(roles, nil)

	result, err := service.CreateUser(ctx, req)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, domainError.ErrInvalidRoleIDs))

	userRepo.AssertExpectations(t)
	roleRepo.AssertExpectations(t)
	userRoleRepo.AssertNotCalled(t, "Create")
}

func TestCreateUser_UserCreationFailed(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepository)
	roleRepo := new(MockRoleRepository)
	userRoleRepo := new(MockUserRoleRepository)
	service := createTestUserService(userRepo, roleRepo, userRoleRepo)

	req := &dto.CreateUserRequest{
		Username: "testuser",
		Password: "password123",
		RoleIDs:  []int{1},
	}

	roles := []*dto.Role{
		createTestRole(1, "admin"),
	}

	userRepo.On("FindByUsername", ctx, req.Username).Return(nil, errors.New("not found"))
	roleRepo.On("FindByIDs", ctx, req.RoleIDs).Return(roles, nil)
	userRepo.On("Create", ctx, mock.AnythingOfType("*dto.User")).Return(errors.New("database error"))

	result, err := service.CreateUser(ctx, req)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, domainError.ErrUserCreationFailed))

	userRepo.AssertExpectations(t)
	roleRepo.AssertExpectations(t)
	userRoleRepo.AssertNotCalled(t, "Create")
}

func TestCreateUser_UserRoleAssignmentFailed(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepository)
	roleRepo := new(MockRoleRepository)
	userRoleRepo := new(MockUserRoleRepository)
	service := createTestUserService(userRepo, roleRepo, userRoleRepo)

	req := &dto.CreateUserRequest{
		Username: "testuser",
		Password: "password123",
		RoleIDs:  []int{1},
	}

	roles := []*dto.Role{
		createTestRole(1, "admin"),
	}

	userRepo.On("FindByUsername", ctx, req.Username).Return(nil, errors.New("not found"))
	roleRepo.On("FindByIDs", ctx, req.RoleIDs).Return(roles, nil)
	userRepo.On("Create", ctx, mock.AnythingOfType("*dto.User")).Run(func(args mock.Arguments) {
		user := args.Get(1).(*dto.User)
		user.ID = "test-user-id"
	}).Return(nil)
	userRoleRepo.On("Create", ctx, mock.AnythingOfType("*dto.UserRole")).Return(errors.New("database error"))

	result, err := service.CreateUser(ctx, req)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to assign role to user")

	userRepo.AssertExpectations(t)
	roleRepo.AssertExpectations(t)
	userRoleRepo.AssertExpectations(t)
}
