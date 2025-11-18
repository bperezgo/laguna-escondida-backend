package dto

import "time"

type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Password  string    `json:"-"` // Never serialize password
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
	Username string `json:"username" validate:"required,min=3,max=255"`
	Password string `json:"password" validate:"required,min=6"`
	RoleIDs  []int  `json:"role_ids" validate:"required,min=1,dive,min=1"`
}

type UserWithRoles struct {
	User  *User   `json:"user"`
	Roles []*Role `json:"roles"`
}

type SignInRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type SignInResponse struct {
	Token    string  `json:"token"`
	Username string  `json:"username"`
	Roles    []*Role `json:"roles"`
}
