package service

import (
	"context"
	"errors"
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
	unitOfWork   ports.UnitOfWork
}

func NewUserService(userRepo ports.UserRepository, roleRepo ports.RoleRepository, userRoleRepo ports.UserRoleRepository, jwtService *JWTService, unitOfWork ports.UnitOfWork) *UserService {
	return &UserService{
		userRepo:     userRepo,
		roleRepo:     roleRepo,
		userRoleRepo: userRoleRepo,
		jwtService:   jwtService,
		unitOfWork:   unitOfWork,
	}
}

func (s *UserService) CreateUser(ctx context.Context, req *dto.CreateUserRequest) (*dto.UserWithRoles, error) {
	existingUser, err := s.userRepo.FindByUsername(ctx, req.Username)
	if err != nil && !errors.Is(err, domainError.ErrUserNotFound) {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}
	if existingUser != nil {
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

	err = s.unitOfWork.Do(ctx, func(ctx context.Context) error {
		if err := s.userRepo.Create(ctx, userDTO); err != nil {
			return fmt.Errorf("%w: %w", domainError.ErrUserCreationFailed, err)
		}

		now := time.Now()
		for _, role := range roles {
			userRole := &dto.UserRole{
				UserID:    userDTO.ID,
				RoleID:    role.ID,
				CreatedAt: now,
			}
			if err := s.userRoleRepo.Create(ctx, userRole); err != nil {
				return fmt.Errorf("failed to assign role to user: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &dto.UserWithRoles{
		User:  userDTO,
		Roles: roles,
	}, nil
}

func (s *UserService) ListUsers(ctx context.Context) (*dto.UsersListResponse, error) {
	users, err := s.userRepo.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	result := make([]*dto.UserWithRoles, 0, len(users))
	for _, u := range users {
		roles, err := s.userRoleRepo.FindRolesByUserID(ctx, u.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get user roles: %w", err)
		}
		result = append(result, &dto.UserWithRoles{User: u, Roles: roles})
	}

	return &dto.UsersListResponse{Users: result}, nil
}

func (s *UserService) GetUser(ctx context.Context, id string) (*dto.UserWithRoles, error) {
	userDTO, err := s.userRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	roles, err := s.userRoleRepo.FindRolesByUserID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get user roles: %w", err)
	}

	return &dto.UserWithRoles{User: userDTO, Roles: roles}, nil
}

func (s *UserService) UpdateUser(ctx context.Context, actingUserID, id string, req *dto.UpdateUserRequest) (*dto.UserWithRoles, error) {
	current, err := s.userRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	currentRoles, err := s.userRoleRepo.FindRolesByUserID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get user roles: %w", err)
	}

	// Resolve the resulting state after applying the (partial) update.
	newActive := current.Active
	if req.Active != nil {
		newActive = *req.Active
	}

	rolesChanged := len(req.RoleIDs) > 0
	newRoleIDs := roleIDsOf(currentRoles)
	var newRoles []*dto.Role
	if rolesChanged {
		roles, err := s.roleRepo.FindByIDs(ctx, req.RoleIDs)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", domainError.ErrRoleNotFound, err)
		}
		if len(roles) != len(req.RoleIDs) {
			return nil, domainError.ErrInvalidRoleIDs
		}
		newRoles = roles
		newRoleIDs = req.RoleIDs
	}

	if actingUserID == id && req.Active != nil && !*req.Active {
		return nil, domainError.ErrCannotDeactivateSelf
	}

	// Guard against demoting/deactivating the last remaining active admin.
	wasActiveAdmin := current.Active && containsInt(roleIDsOf(currentRoles), permissions.RoleAdmin)
	willBeActiveAdmin := newActive && containsInt(newRoleIDs, permissions.RoleAdmin)
	if wasActiveAdmin && !willBeActiveAdmin {
		count, err := s.userRoleRepo.CountUsersByRoleID(ctx, permissions.RoleAdmin)
		if err != nil {
			return nil, fmt.Errorf("failed to count admins: %w", err)
		}
		if count <= 1 {
			return nil, domainError.ErrCannotRemoveLastAdmin
		}
	}

	if req.Name != nil {
		current.Name = *req.Name
	}
	current.Active = newActive
	current.UpdatedAt = time.Now()

	err = s.unitOfWork.Do(ctx, func(ctx context.Context) error {
		if err := s.userRepo.Update(ctx, current); err != nil {
			return err
		}
		if rolesChanged {
			if err := s.userRoleRepo.DeleteByUserID(ctx, id); err != nil {
				return fmt.Errorf("failed to clear user roles: %w", err)
			}
			now := time.Now()
			for _, role := range newRoles {
				userRole := &dto.UserRole{UserID: id, RoleID: role.ID, CreatedAt: now}
				if err := s.userRoleRepo.Create(ctx, userRole); err != nil {
					return fmt.Errorf("failed to assign role to user: %w", err)
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.GetUser(ctx, id)
}

func (s *UserService) ResetPassword(ctx context.Context, id, newPassword string) error {
	if _, err := s.userRepo.FindByID(ctx, id); err != nil {
		return err
	}

	hashedPassword, err := user.HashPassword(newPassword)
	if err != nil {
		return err
	}

	return s.userRepo.UpdatePassword(ctx, id, hashedPassword)
}

func (s *UserService) DeleteUser(ctx context.Context, actingUserID, id string) error {
	if actingUserID == id {
		return domainError.ErrCannotDeleteSelf
	}

	current, err := s.userRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	roles, err := s.userRoleRepo.FindRolesByUserID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get user roles: %w", err)
	}

	if current.Active && containsInt(roleIDsOf(roles), permissions.RoleAdmin) {
		count, err := s.userRoleRepo.CountUsersByRoleID(ctx, permissions.RoleAdmin)
		if err != nil {
			return fmt.Errorf("failed to count admins: %w", err)
		}
		if count <= 1 {
			return domainError.ErrCannotRemoveLastAdmin
		}
	}

	return s.userRepo.SoftDelete(ctx, id)
}

func (s *UserService) ListRoles(ctx context.Context) (*dto.RolesListResponse, error) {
	roles, err := s.roleRepo.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list roles: %w", err)
	}

	return &dto.RolesListResponse{Roles: roles}, nil
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

	if !userDTO.Active {
		return nil, domainError.ErrUserInactive
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

func containsInt(ids []int, target int) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

func roleIDsOf(roles []*dto.Role) []int {
	ids := make([]int, len(roles))
	for i, role := range roles {
		ids[i] = role.ID
	}
	return ids
}
