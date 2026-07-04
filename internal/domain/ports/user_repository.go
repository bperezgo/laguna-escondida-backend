package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

type UserRepository interface {
	Create(ctx context.Context, user *dto.User) error
	FindByUsername(ctx context.Context, username string) (*dto.User, error)
	FindByID(ctx context.Context, id string) (*dto.User, error)
	FindAll(ctx context.Context) ([]*dto.User, error)
	Update(ctx context.Context, user *dto.User) error
	UpdatePassword(ctx context.Context, id, hashedPassword string) error
	SoftDelete(ctx context.Context, id string) error
}

type RoleRepository interface {
	FindByID(ctx context.Context, id int) (*dto.Role, error)
	FindByIDs(ctx context.Context, ids []int) ([]*dto.Role, error)
	FindByName(ctx context.Context, name string) (*dto.Role, error)
	FindAll(ctx context.Context) ([]*dto.Role, error)
}

type UserRoleRepository interface {
	Create(ctx context.Context, userRole *dto.UserRole) error
	FindByUserID(ctx context.Context, userID string) ([]*dto.UserRole, error)
	FindRolesByUserID(ctx context.Context, userID string) ([]*dto.Role, error)
	DeleteByUserID(ctx context.Context, userID string) error
	CountUsersByRoleID(ctx context.Context, roleID int) (int, error)
}
