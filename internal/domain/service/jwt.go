package service

import (
	"errors"
	"slices"
	"time"

	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token has expired")
)

type JWTClaims struct {
	UserID      string   `json:"user_id"`
	OrgID       string   `json:"org_id"`
	Username    string   `json:"username"`
	RoleIDs     []int    `json:"role_ids"`
	Permissions []string `json:"permissions"`
	jwt.RegisteredClaims
}

type JWTService struct {
	secret  []byte
	scope   tenant.Scope
	apiKeys *APIKeyService
}

// NewJWTService issues and validates tokens; a token whose organization is outside scope is invalid.
func NewJWTService(secret string, scope tenant.Scope) *JWTService {
	return &JWTService{
		secret: []byte(secret),
		scope:  scope,
	}
}

func (s *JWTService) GenerateToken(userID, orgID, username string, roleIDs []int) (string, error) {
	perms := permissions.GetPermissionsForRoles(roleIDs)
	permStrings := permissions.PermissionStrings(perms)

	expiration := 7 * 24 * time.Hour
	if slices.Contains(roleIDs, permissions.RoleAdmin) {
		expiration = 24 * time.Hour
	}

	claims := JWTClaims{
		UserID:      userID,
		OrgID:       orgID,
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

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}
	// Tokens issued before tenancy carry no org; their holders sign in again.
	if claims.OrgID == "" {
		return nil, ErrInvalidToken
	}
	if !s.scope.Admits(claims.OrgID) {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// AcceptAPIKeys lets the bearer check also take organization API keys. Only the cloud calls
// it, so an edge box refuses every key.
func (s *JWTService) AcceptAPIKeys(keys *APIKeyService) {
	s.apiKeys = keys
}

// APIKeys is nil where keys are not accepted.
func (s *JWTService) APIKeys() *APIKeyService {
	return s.apiKeys
}
