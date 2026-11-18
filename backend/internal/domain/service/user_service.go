package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/user"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/tenant"
)

type UserService struct {
	userRepo     ports.UserRepository
	roleRepo     ports.RoleRepository
	userRoleRepo ports.UserRoleRepository
	orgRepo      ports.OrganizationRepository
	jwtService   *JWTService
	unitOfWork   ports.UnitOfWork
	scope        tenant.Scope
}

func NewUserService(userRepo ports.UserRepository, roleRepo ports.RoleRepository, userRoleRepo ports.UserRoleRepository, orgRepo ports.OrganizationRepository, jwtService *JWTService, unitOfWork ports.UnitOfWork, scope tenant.Scope) *UserService {
	return &UserService{
		userRepo:     userRepo,
		roleRepo:     roleRepo,
		userRoleRepo: userRoleRepo,
		orgRepo:      orgRepo,
		jwtService:   jwtService,
		unitOfWork:   unitOfWork,
		scope:        scope,
	}
}

// CreateUser puts the user in the caller's organization. Only a caller without one (the
// admin-key bootstrap) names it, by slug in req.Organization.
func (s *UserService) CreateUser(ctx context.Context, req *dto.CreateUserRequest) (*dto.UserWithRoles, error) {
	if _, ok := tenant.OrgFrom(ctx); !ok {
		orgID, err := s.bootstrapOrganization(ctx, req.Organization)
		if err != nil {
			return nil, err
		}
		ctx = tenant.WithOrg(ctx, orgID)
	}

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

	if permissions.HasCloudRole(req.RoleIDs) && strings.TrimSpace(req.Email) == "" {
		return nil, domainError.ErrEmailRequired
	}

	userAggregate, err := user.NewAggregateFromCreateUserRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create user aggregate: %w", err)
	}

	userDTO := userAggregate.ToDTO()

	err = s.unitOfWork.Do(ctx, func(ctx context.Context) error {
		if createErr := s.userRepo.Create(ctx, userDTO); createErr != nil {
			return fmt.Errorf("%w: %w", domainError.ErrUserCreationFailed, createErr)
		}

		now := time.Now()
		for _, role := range roles {
			userRole := &dto.UserRole{
				UserID:    userDTO.ID,
				RoleID:    role.ID,
				CreatedAt: now,
			}
			if roleErr := s.userRoleRepo.Create(ctx, userRole); roleErr != nil {
				return fmt.Errorf("failed to assign role to user: %w", roleErr)
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

func (s *UserService) bootstrapOrganization(ctx context.Context, slug string) (string, error) {
	if slug == "" {
		if boxOrgID, pinned := s.scope.Pinned(); pinned {
			return boxOrgID, nil
		}
		return "", domainError.ErrOrganizationRequired
	}
	org, err := s.orgRepo.GetBySlug(ctx, slug)
	if err != nil {
		return "", err
	}
	if !s.scope.Admits(org.ID) {
		return "", domainError.ErrOrganizationMismatch
	}
	return org.ID, nil
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
		newRoles, err = s.roleRepo.FindByIDs(ctx, req.RoleIDs)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", domainError.ErrRoleNotFound, err)
		}
		if len(newRoles) != len(req.RoleIDs) {
			return nil, domainError.ErrInvalidRoleIDs
		}
		newRoleIDs = req.RoleIDs
	}

	if actingUserID == id && req.Active != nil && !*req.Active {
		return nil, domainError.ErrCannotDeactivateSelf
	}

	// Guard against demoting/deactivating the last remaining active admin.
	wasActiveAdmin := current.Active && containsInt(roleIDsOf(currentRoles), permissions.RoleAdmin)
	willBeActiveAdmin := newActive && containsInt(newRoleIDs, permissions.RoleAdmin)
	if wasActiveAdmin && !willBeActiveAdmin {
		count, countErr := s.userRoleRepo.CountUsersByRoleID(ctx, permissions.RoleAdmin)
		if countErr != nil {
			return nil, fmt.Errorf("failed to count admins: %w", countErr)
		}
		if count <= 1 {
			return nil, domainError.ErrCannotRemoveLastAdmin
		}
	}

	newEmail := current.Email
	if req.Email != nil {
		newEmail, err = user.NormalizeEmail(*req.Email)
		if err != nil {
			return nil, err
		}
	}
	if permissions.HasCloudRole(newRoleIDs) && newEmail == nil {
		return nil, domainError.ErrEmailRequired
	}

	if req.Name != nil {
		current.Name = *req.Name
	}
	current.Email = newEmail
	current.Active = newActive
	current.UpdatedAt = time.Now()

	err = s.unitOfWork.Do(ctx, func(ctx context.Context) error {
		if updateErr := s.userRepo.Update(ctx, current); updateErr != nil {
			return updateErr
		}
		if rolesChanged {
			if deleteErr := s.userRoleRepo.DeleteByUserID(ctx, id); deleteErr != nil {
				return fmt.Errorf("failed to clear user roles: %w", deleteErr)
			}
			now := time.Now()
			for _, role := range newRoles {
				userRole := &dto.UserRole{UserID: id, RoleID: role.ID, CreatedAt: now}
				if roleErr := s.userRoleRepo.Create(ctx, userRole); roleErr != nil {
					return fmt.Errorf("failed to assign role to user: %w", roleErr)
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

// SignIn: an edge box signs its own organization's users in by username; the cloud signs in by
// email, which names the organization. Every rejection wraps ErrInvalidCredentials so the
// response stays opaque.
func (s *UserService) SignIn(ctx context.Context, req *dto.SignInRequest) (*dto.SignInResponse, error) {
	boxOrgID, pinned := s.scope.Pinned()

	var userDTO *dto.User
	orgID := boxOrgID
	var err error
	if pinned {
		userDTO, err = s.edgeSignInUser(ctx, boxOrgID, req.Username)
	} else {
		userDTO, orgID, err = s.cloudSignInUser(ctx, req.Email)
	}
	if err != nil {
		return nil, err
	}
	ctx = tenant.WithOrg(ctx, orgID)

	userAggregate := user.NewAggregateFromDTO(userDTO)
	if err = userAggregate.ComparePassword(req.Password); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrInvalidCredentials, err)
	}

	if !userDTO.Active {
		return nil, fmt.Errorf("%w: %w", domainError.ErrInvalidCredentials, domainError.ErrUserInactive)
	}

	roles, err := s.userRoleRepo.FindRolesByUserID(ctx, userDTO.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user roles: %w", err)
	}
	roleIDs := roleIDsOf(roles)

	if !pinned && !permissions.HasCloudRole(roleIDs) {
		return nil, fmt.Errorf("%w: %w", domainError.ErrInvalidCredentials, domainError.ErrCloudAccessDenied)
	}

	token, err := s.jwtService.GenerateToken(userDTO.ID, orgID, userDTO.Username, roleIDs)
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

func (s *UserService) edgeSignInUser(ctx context.Context, boxOrgID, username string) (*dto.User, error) {
	if username == "" {
		return nil, domainError.ErrSignInUsernameRequired
	}
	org, err := s.orgRepo.Get(ctx, boxOrgID)
	if err != nil {
		return nil, fmt.Errorf("sign-in: failed to load the box's organization: %w", err)
	}
	if err = checkActive(org); err != nil {
		return nil, err
	}

	userDTO, err := s.userRepo.FindByUsername(tenant.WithOrg(ctx, boxOrgID), username)
	if err != nil {
		return nil, signInLookupError(err)
	}
	return userDTO, nil
}

func (s *UserService) cloudSignInUser(ctx context.Context, rawEmail string) (*dto.User, string, error) {
	if strings.TrimSpace(rawEmail) == "" {
		return nil, "", domainError.ErrSignInEmailRequired
	}
	email, err := user.NormalizeEmail(rawEmail)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", domainError.ErrInvalidCredentials, err)
	}

	found, err := s.userRepo.FindByEmail(ctx, *email)
	if err != nil {
		return nil, "", signInLookupError(err)
	}

	org, err := s.orgRepo.Get(ctx, found.OrganizationID)
	if err != nil {
		return nil, "", fmt.Errorf("sign-in: failed to load the user's organization: %w", err)
	}
	if err = checkActive(org); err != nil {
		return nil, "", err
	}
	return found.User, org.ID, nil
}

// An infrastructure error must not be masked as invalid credentials, so it surfaces as a 500.
func signInLookupError(err error) error {
	if errors.Is(err, domainError.ErrUserNotFound) {
		return fmt.Errorf("%w: %w", domainError.ErrInvalidCredentials, err)
	}
	return fmt.Errorf("sign-in: failed to look up user: %w", err)
}

func checkActive(org *dto.Organization) error {
	if org.Status == dto.OrganizationStatusSuspended {
		return fmt.Errorf("%w: organization %s is suspended", domainError.ErrInvalidCredentials, org.ID)
	}
	return nil
}

func (s *UserService) SignInMode() dto.SignInMode {
	if _, pinned := s.scope.Pinned(); pinned {
		return dto.SignInModeEdge
	}
	return dto.SignInModeCloud
}

// SignInContext tells an unauthenticated client which sign-in form to show. It names only the
// box's own organization, never any other.
func (s *UserService) SignInContext(ctx context.Context) (*dto.SignInContextResponse, error) {
	boxOrgID, pinned := s.scope.Pinned()
	if !pinned {
		return &dto.SignInContextResponse{Mode: dto.SignInModeCloud}, nil
	}
	org, err := s.orgRepo.Get(ctx, boxOrgID)
	if err != nil {
		return nil, fmt.Errorf("sign-in context: failed to load the box's organization: %w", err)
	}
	name := org.Name
	if org.DisplayName != nil && *org.DisplayName != "" {
		name = *org.DisplayName
	}
	return &dto.SignInContextResponse{
		Mode:         dto.SignInModeEdge,
		Organization: &dto.SignInContextOrganization{Name: name},
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
