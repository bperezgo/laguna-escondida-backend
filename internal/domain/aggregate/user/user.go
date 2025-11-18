package user

import (
	userError "laguna-escondida/backend/internal/domain/aggregate/user/error"
	"laguna-escondida/backend/internal/domain/dto"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Aggregate struct {
	id        string
	username  string
	password  string
	createdAt time.Time
	updatedAt time.Time
}

func NewAggregateFromCreateUserRequest(req *dto.CreateUserRequest) (*Aggregate, error) {
	if req == nil {
		return nil, userError.NewInvalidRequestError("request cannot be nil")
	}

	if req.Username == "" {
		return nil, userError.NewMissingUsernameError()
	}

	if req.Password == "" {
		return nil, userError.NewMissingPasswordError()
	}

	if len(req.Password) < 6 {
		return nil, userError.NewInvalidPasswordError("password must be at least 6 characters")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, userError.NewPasswordHashingFailedError(err)
	}

	now := time.Now()
	return &Aggregate{
		id:        uuid.New().String(),
		username:  req.Username,
		password:  string(hashedPassword),
		createdAt: now,
		updatedAt: now,
	}, nil
}

func (a *Aggregate) ToDTO() *dto.User {
	return &dto.User{
		ID:        a.id,
		Username:  a.username,
		Password:  a.password,
		CreatedAt: a.createdAt,
		UpdatedAt: a.updatedAt,
	}
}
