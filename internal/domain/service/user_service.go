package service

import (
	"context"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/user"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
)

type UserService struct {
	userRepo     ports.UserRepository
	roleRepo     ports.RoleRepository
	userRoleRepo ports.UserRoleRepository
}

func NewUserService(userRepo ports.UserRepository, roleRepo ports.RoleRepository, userRoleRepo ports.UserRoleRepository) *UserService {
	return &UserService{
		userRepo:     userRepo,
		roleRepo:     roleRepo,
		userRoleRepo: userRoleRepo,
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
