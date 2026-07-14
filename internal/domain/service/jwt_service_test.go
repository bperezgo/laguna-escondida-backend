package service

import (
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/permissions"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestJWTService() *JWTService {
	return NewJWTService("test-secret-key")
}

// Token expiration: admin gets 24 h, every other role gets 7 days.

func TestGenerateToken_AdminExpires24Hours(t *testing.T) {
	svc := createTestJWTService()

	before := time.Now()
	tokenStr, err := svc.GenerateToken("uid-1", "admin", []int{permissions.RoleAdmin})
	after := time.Now()

	require.NoError(t, err)

	claims, err := svc.ValidateToken(tokenStr)
	require.NoError(t, err)

	expiresAt := claims.ExpiresAt.Time
	assert.WithinDuration(t, before.Add(24*time.Hour), expiresAt, after.Sub(before)+time.Second)
}

func TestGenerateToken_NonAdminExpires7Days(t *testing.T) {
	nonAdminRoles := []struct {
		name    string
		roleIDs []int
	}{
		{"server", []int{permissions.RoleServer}},
		{"cooker", []int{permissions.RoleCooker}},
		{"manager", []int{permissions.RoleManager}},
		{"accountant", []int{permissions.RoleAccountant}},
		{"server+cooker", []int{permissions.RoleServer, permissions.RoleCooker}},
	}

	svc := createTestJWTService()

	for _, tc := range nonAdminRoles {
		t.Run(tc.name, func(t *testing.T) {
			before := time.Now()
			tokenStr, err := svc.GenerateToken("uid-1", tc.name, tc.roleIDs)
			after := time.Now()

			require.NoError(t, err)

			claims, err := svc.ValidateToken(tokenStr)
			require.NoError(t, err)

			expiresAt := claims.ExpiresAt.Time
			assert.WithinDuration(t, before.Add(7*24*time.Hour), expiresAt, after.Sub(before)+time.Second)
		})
	}
}

// Admin role wins even when combined with other roles.
func TestGenerateToken_AdminWithOtherRolesExpires24Hours(t *testing.T) {
	svc := createTestJWTService()

	before := time.Now()
	tokenStr, err := svc.GenerateToken("uid-1", "admin", []int{permissions.RoleAdmin, permissions.RoleManager})
	after := time.Now()

	require.NoError(t, err)

	claims, err := svc.ValidateToken(tokenStr)
	require.NoError(t, err)

	expiresAt := claims.ExpiresAt.Time
	assert.WithinDuration(t, before.Add(24*time.Hour), expiresAt, after.Sub(before)+time.Second)
}
