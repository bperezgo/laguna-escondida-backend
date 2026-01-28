package service

import (
	"context"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/user"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/ports"
)

type UserService struct {
	userRepo     ports.UserRepository
	roleRepo     ports.RoleRepository
	userRoleRepo ports.UserRoleRepository
	jwtService   *JWTService
}

func NewUserService(userRepo ports.UserRepository, roleRepo ports.RoleRepository, userRoleRepo ports.UserRoleRepository, jwtService *JWTService) *UserService {
	return &UserService{
		userRepo:     userRepo,
		roleRepo:     roleRepo,
		userRoleRepo: userRoleRepo,
		jwtService:   jwtService,
	}
}

func (s *UserService) CreateUser(ctx context.Context, req *dto.CreateUserRequest) (*dto.UserWithRoles, error) {
	existingUser, err := s.userRepo.FindByUsername(ctx, req.Username)
	if err == nil && existingUser != nil {
		return nil, domainError.ErrUserAlreadyExists
	}

	roles, err := s.roleRepo.FindByIDs(ctx, req.RoleIDs)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrRoleNotFound, err)
	}

	if len(roles) != len(req.RoleIDs) {
		return nil, domainError.ErrInvalidRoleIDs
	}

	userAggregate, err := user.NewAggregateFromCreateUserRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create user aggregate: %w", err)
	}

	userDTO := userAggregate.ToDTO()
	if err := s.userRepo.Create(ctx, userDTO); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrUserCreationFailed, err)
	}

	now := time.Now()
	for _, role := range roles {
		userRole := &dto.UserRole{
			UserID:    userDTO.ID,
			RoleID:    role.ID,
			CreatedAt: now,
		}
		if err := s.userRoleRepo.Create(ctx, userRole); err != nil {
			return nil, fmt.Errorf("failed to assign role to user: %w", err)
		}
	}

	return &dto.UserWithRoles{
		User:  userDTO,
		Roles: roles,
	}, nil
}

func (s *UserService) SignIn(ctx context.Context, req *dto.SignInRequest) (*dto.SignInResponse, error) {
	userDTO, err := s.userRepo.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, domainError.ErrInvalidCredentials
	}

	userAggregate := user.NewAggregateFromDTO(userDTO)
	if err = userAggregate.ComparePassword(req.Password); err != nil {
		return nil, domainError.ErrInvalidCredentials
	}

	roles, err := s.userRoleRepo.FindRolesByUserID(ctx, userDTO.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user roles: %w", err)
	}

	roleIDs := make([]int, len(roles))
	for i, role := range roles {
		roleIDs[i] = role.ID
	}

	token, err := s.jwtService.GenerateToken(userDTO.ID, userDTO.Username, roleIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	perms := permissions.GetPermissionsForRoles(roleIDs)
	permStrings := permissions.PermissionStrings(perms)

	return &dto.SignInResponse{
		Token:       token,
		Username:    userDTO.Username,
		Roles:       roles,
		Permissions: permStrings,
	}, nil
}

func (s *UserService) GetCurrentUser(ctx context.Context, userID string) (*dto.CurrentUserResponse, error) {
	userDTO, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	roles, err := s.userRoleRepo.FindRolesByUserID(ctx, userDTO.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user roles: %w", err)
	}

	roleIDs := make([]int, len(roles))
	for i, role := range roles {
		roleIDs[i] = role.ID
	}

	perms := permissions.GetPermissionsForRoles(roleIDs)
	permStrings := permissions.PermissionStrings(perms)

	return &dto.CurrentUserResponse{
		ID:          userDTO.ID,
		Username:    userDTO.Username,
		Name:        userDTO.Name,
		Roles:       roles,
		Permissions: permStrings,
	}, nil
}
