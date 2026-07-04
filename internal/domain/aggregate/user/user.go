package user

import (
	"time"

	userError "laguna-escondida/backend/internal/domain/aggregate/user/error"
	"laguna-escondida/backend/internal/domain/dto"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Aggregate struct {
	id        string
	username  string
	name      string
	password  string
	active    bool
	createdAt time.Time
	updatedAt time.Time
}

// HashPassword validates and bcrypt-hashes a plaintext password. It is the single
// place password rules live, shared by user creation and admin password resets.
func HashPassword(plainPassword string) (string, error) {
	if plainPassword == "" {
		return "", userError.NewMissingPasswordError()
	}

	if len(plainPassword) < 6 {
		return "", userError.NewInvalidPasswordError("password must be at least 6 characters")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
	if err != nil {
		return "", userError.NewPasswordHashingFailedError(err)
	}

	return string(hashedPassword), nil
}

func NewAggregateFromCreateUserRequest(req *dto.CreateUserRequest) (*Aggregate, error) {
	if req == nil {
		return nil, userError.NewInvalidRequestError("request cannot be nil")
	}

	if req.Username == "" {
		return nil, userError.NewMissingUsernameError()
	}

	if req.Name == "" {
		return nil, userError.NewMissingNameError()
	}

	hashedPassword, err := HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	return &Aggregate{
		id:        uuid.Must(uuid.NewV7()).String(),
		username:  req.Username,
		name:      req.Name,
		password:  hashedPassword,
		active:    true,
		createdAt: now,
		updatedAt: now,
	}, nil
}

func (a *Aggregate) ToDTO() *dto.User {
	return &dto.User{
		ID:        a.id,
		Username:  a.username,
		Name:      a.name,
		Password:  a.password,
		Active:    a.active,
		CreatedAt: a.createdAt,
		UpdatedAt: a.updatedAt,
	}
}

func (a *Aggregate) ComparePassword(password string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(a.password), []byte(password)); err != nil {
		return userError.NewInvalidPasswordError("password does not match")
	}
	return nil
}

func NewAggregateFromDTO(user *dto.User) *Aggregate {
	return &Aggregate{
		id:        user.ID,
		username:  user.Username,
		name:      user.Name,
		password:  user.Password,
		active:    user.Active,
		createdAt: user.CreatedAt,
		updatedAt: user.UpdatedAt,
	}
}
