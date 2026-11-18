package dto

import "time"

type UserDomain struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	RoleIDs        []int  `json:"role_ids,omitempty"`
}

type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Name      string    `json:"name"`
	Email     *string   `json:"email"`
	Password  string    `json:"-"` // Never serialize password
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UserInOrganization struct {
	User           *User
	OrganizationID string
}

type Role struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type UserRole struct {
	ID        int       `json:"id"`
	UserID    string    `json:"user_id"`
	RoleID    int       `json:"role_id"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateUserRequest struct {
	// Organization is the slug, read only from an admin-key bootstrap; a signed-in creator's
	// own organization always wins.
	Organization string `json:"organization,omitempty"`
	Username     string `json:"username" validate:"required,min=3,max=255"`
	Name         string `json:"name" validate:"required,min=1,max=255"`
	Email        string `json:"email,omitempty" validate:"omitempty,email,max=255"`
	Password     string `json:"password" validate:"required,min=6"`
	RoleIDs      []int  `json:"role_ids" validate:"required,min=1,dive,min=1"`
}

type UpdateUserRequest struct {
	Name    *string `json:"name" validate:"omitempty,min=1,max=255"`
	Email   *string `json:"email" validate:"omitempty,email,max=255"`
	RoleIDs []int   `json:"role_ids" validate:"omitempty,min=1,dive,min=1"`
	Active  *bool   `json:"active"`
}

type ResetPasswordRequest struct {
	Password string `json:"password" validate:"required,min=6"`
}

type UserWithRoles struct {
	User  *User   `json:"user"`
	Roles []*Role `json:"roles"`
}

type UsersListResponse struct {
	Users []*UserWithRoles `json:"users"`
}

type RolesListResponse struct {
	Roles []*Role `json:"roles"`
}

// SignInRequest: the cloud requires Email, an edge box requires Username.
type SignInRequest struct {
	Email    string `json:"email,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password" validate:"required"`
}

type SignInMode string

const (
	SignInModeCloud SignInMode = "cloud"
	SignInModeEdge  SignInMode = "edge"
)

type SignInContextOrganization struct {
	Name string `json:"name"`
}

type SignInContextResponse struct {
	Mode         SignInMode                 `json:"mode"`
	Organization *SignInContextOrganization `json:"organization,omitempty"`
}

type SignInResponse struct {
	Token       string   `json:"token"`
	Username    string   `json:"username"`
	Roles       []*Role  `json:"roles"`
	Permissions []string `json:"permissions"`
}

type CurrentUserResponse struct {
	ID           string               `json:"id"`
	Username     string               `json:"username"`
	Name         string               `json:"name"`
	Roles        []*Role              `json:"roles"`
	Permissions  []string             `json:"permissions"`
	Organization *CurrentOrganization `json:"organization"`
}
