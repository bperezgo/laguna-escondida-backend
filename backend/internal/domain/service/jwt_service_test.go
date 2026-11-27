package service

import (
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testOrgA = "019a0000-0000-7000-8000-00000000000a"
	testOrgB = "019a0000-0000-7000-8000-00000000000b"
)

func createTestJWTService() *JWTService {
	return NewJWTService("test-secret-key", tenant.AnyOrganization())
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
	box := NewJWTService("test-secret-key", tenant.PinnedTo(testOrgA))

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

func TestJWTService_SetupToken_IsNotASignInToken(t *testing.T) {
	svc := NewJWTService("test-secret", tenant.AnyOrganization())
	setup, err := svc.GenerateSetupToken("user-1", "org-1")
	require.NoError(t, err)

	_, err = svc.ValidateToken(setup)
	assert.ErrorIs(t, err, ErrInvalidToken)

	claims, err := svc.ValidateSetupToken(setup)
	require.NoError(t, err)
	assert.Equal(t, "user-1", claims.UserID)
	assert.Equal(t, "org-1", claims.OrgID)
	assert.Empty(t, claims.RoleIDs)
	assert.Empty(t, claims.Permissions)
}

func TestJWTService_ValidateSetupToken_RefusesASignInToken(t *testing.T) {
	svc := NewJWTService("test-secret", tenant.AnyOrganization())
	signIn, err := svc.GenerateToken("user-1", "org-1", "rosa", []int{permissions.RoleAdmin})
	require.NoError(t, err)

	_, err = svc.ValidateSetupToken(signIn)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestJWTService_SetupToken_ExpiresAfterAnHour(t *testing.T) {
	svc := NewJWTService("test-secret", tenant.AnyOrganization())
	issued := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return issued }
	setup, err := svc.GenerateSetupToken("user-1", "org-1")
	require.NoError(t, err)

	svc.now = func() time.Time { return issued.Add(59 * time.Minute) }
	_, err = svc.ValidateSetupToken(setup)
	require.NoError(t, err)

	svc.now = func() time.Time { return issued.Add(61 * time.Minute) }
	_, err = svc.ValidateSetupToken(setup)
	assert.ErrorIs(t, err, ErrExpiredToken)
}

func TestJWTService_SetupToken_OutsideTheScopeIsInvalid(t *testing.T) {
	issuer := NewJWTService("test-secret", tenant.AnyOrganization())
	setup, err := issuer.GenerateSetupToken("user-1", "org-1")
	require.NoError(t, err)

	_, err = NewJWTService("test-secret", tenant.PinnedTo("org-2")).ValidateSetupToken(setup)
	assert.ErrorIs(t, err, ErrInvalidToken)
}
