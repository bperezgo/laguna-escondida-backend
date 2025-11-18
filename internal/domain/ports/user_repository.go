package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

type UserRepository interface {
	Create(ctx context.Context, user *dto.User) error
	FindByUsername(ctx context.Context, username string) (*dto.User, error)
	FindByID(ctx context.Context, id string) (*dto.User, error)
}

type RoleRepository interface {
	FindByID(ctx context.Context, id int) (*dto.Role, error)
	FindByIDs(ctx context.Context, ids []int) ([]*dto.Role, error)
	FindByName(ctx context.Context, name string) (*dto.Role, error)
}

type UserRoleRepository interface {
	Create(ctx context.Context, userRole *dto.UserRole) error
	FindByUserID(ctx context.Context, userID string) ([]*dto.UserRole, error)
	FindRolesByUserID(ctx context.Context, userID string) ([]*dto.Role, error)
}
