package service

import (
	"errors"
	"slices"
	"time"

	"laguna-escondida/backend/internal/domain/permissions"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token has expired")
)

type JWTClaims struct {
	UserID      string   `json:"user_id"`
	Username    string   `json:"username"`
	RoleIDs     []int    `json:"role_ids"`
	Permissions []string `json:"permissions"`
	jwt.RegisteredClaims
}

type JWTService struct {
	secret []byte
}

func NewJWTService(secret string) *JWTService {
	return &JWTService{
		secret: []byte(secret),
	}
}

func (s *JWTService) GenerateToken(userID string, username string, roleIDs []int) (string, error) {
	perms := permissions.GetPermissionsForRoles(roleIDs)
	permStrings := permissions.PermissionStrings(perms)

	expiration := 7 * 24 * time.Hour
	if slices.Contains(roleIDs, permissions.RoleAdmin) {
		expiration = 24 * time.Hour
	}

	claims := JWTClaims{
		UserID:      userID,
		Username:    username,
		RoleIDs:     roleIDs,
		Permissions: permStrings,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

func (s *JWTService) ValidateToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return s.secret, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}
