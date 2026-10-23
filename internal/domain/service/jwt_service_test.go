package service

import (
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/permissions"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testOrgA = "019a0000-0000-7000-8000-00000000000a"
	testOrgB = "019a0000-0000-7000-8000-00000000000b"
)

func createTestJWTService() *JWTService {
	return NewJWTService("test-secret-key", "")
}

// Token expiration: admin gets 24 h, every other role gets 7 days.

func TestGenerateToken_AdminExpires24Hours(t *testing.T) {
	svc := createTestJWTService()

	before := time.Now()
	tokenStr, err := svc.GenerateToken("uid-1", testOrgA, "admin", []int{permissions.RoleAdmin})
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
			tokenStr, err := svc.GenerateToken("uid-1", testOrgA, tc.name, tc.roleIDs)
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
	tokenStr, err := svc.GenerateToken("uid-1", testOrgA, "admin", []int{permissions.RoleAdmin, permissions.RoleManager})
	after := time.Now()

	require.NoError(t, err)

	claims, err := svc.ValidateToken(tokenStr)
	require.NoError(t, err)

	expiresAt := claims.ExpiresAt.Time
	assert.WithinDuration(t, before.Add(24*time.Hour), expiresAt, after.Sub(before)+time.Second)
}

func TestValidateToken_CarriesOrganization(t *testing.T) {
	svc := createTestJWTService()
	tokenStr, err := svc.GenerateToken("uid-1", testOrgA, "cajero1", []int{permissions.RoleServer})
	require.NoError(t, err)

	claims, err := svc.ValidateToken(tokenStr)

	require.NoError(t, err)
	assert.Equal(t, testOrgA, claims.OrgID)
}

// Tokens issued before tenancy have no org_id; their holders must sign in again.
func TestValidateToken_WithoutOrganization_IsRejected(t *testing.T) {
	svc := createTestJWTService()
	tokenStr, err := svc.GenerateToken("uid-1", "", "cajero1", []int{permissions.RoleServer})
	require.NoError(t, err)

	_, err = svc.ValidateToken(tokenStr)

	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestValidateToken_EdgeBox_OnlyItsOwnOrganization(t *testing.T) {
	cloud := createTestJWTService()
	box := NewJWTService("test-secret-key", testOrgA)

	own, err := cloud.GenerateToken("uid-1", testOrgA, "cajero1", []int{permissions.RoleServer})
	require.NoError(t, err)
	other, err := cloud.GenerateToken("uid-2", testOrgB, "cajero1", []int{permissions.RoleServer})
	require.NoError(t, err)

	_, err = box.ValidateToken(own)
	assert.NoError(t, err)
	_, err = box.ValidateToken(other)
	assert.ErrorIs(t, err, ErrInvalidToken, "a validly signed token of another organization")
	_, err = cloud.ValidateToken(other)
	assert.NoError(t, err, "the cloud serves every organization")
}
