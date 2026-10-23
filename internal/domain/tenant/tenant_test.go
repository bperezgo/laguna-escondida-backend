package tenant_test

import (
	"context"
	"testing"

	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTenant_OrgFrom(t *testing.T) {
	_, ok := tenant.OrgFrom(context.Background())
	assert.False(t, ok, "no org")

	_, ok = tenant.OrgFrom(tenant.WithOrg(context.Background(), ""))
	assert.False(t, ok, "an empty org is no org")

	got, ok := tenant.OrgFrom(tenant.WithOrg(context.Background(), "org-a"))
	require.True(t, ok)
	assert.Equal(t, "org-a", got)
}

func TestTenant_RequireOrg(t *testing.T) {
	_, err := tenant.RequireOrg(context.Background())
	assert.ErrorIs(t, err, domainError.ErrNoOrganization)

	got, err := tenant.RequireOrg(tenant.WithOrg(context.Background(), "org-a"))
	require.NoError(t, err)
	assert.Equal(t, "org-a", got)
}
