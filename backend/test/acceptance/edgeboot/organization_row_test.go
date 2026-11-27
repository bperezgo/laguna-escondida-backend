package edgeboot

import (
	"context"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/postgres/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A box enrolled to any restaurant but the first has no row for its organization after
// migrating, so the first pulled user broke users_organization_id_fkey and the box never got one.
func TestBox_OfANewOrganization_StoresItsFirstPulledUser(t *testing.T) {
	if !gateEnabled() {
		t.Skip("set RUN_ACCEPTANCE_TESTS=true")
	}
	box := boxes[otherBoxDBName]
	orgID := uuid.NewString()
	userID := uuid.NewString()
	t.Cleanup(func() {
		box.owner.Exec("DELETE FROM user_roles WHERE user_id = ?", userID)
		box.owner.Exec("DELETE FROM users WHERE id = ?", userID)
		box.owner.Exec("DELETE FROM organizations WHERE id = ?", orgID)
	})
	ctx := tenant.WithOrg(context.Background(), orgID)
	now := time.Now().UTC()
	pulled := []dto.UserSyncPayload{{ID: userID, Username: "caja", Name: "Caja", Password: "hash",
		RoleIDs: []int{permissions.RoleManager}, CreatedAt: now, UpdatedAt: now}}
	pulls := repository.NewSyncReferenceRepository(box.app)

	require.Error(t, pulls.UpsertUsers(ctx, pulled), "without its organization row the box cannot store the user")

	orgs := repository.NewBoxOrganizationRepository(box.app)
	require.NoError(t, orgs.Ensure(context.Background(), orgID), "the app role adds the row")
	require.NoError(t, orgs.Ensure(context.Background(), orgID), "a restart changes nothing")

	require.NoError(t, pulls.UpsertUsers(ctx, pulled))
	var stored int64
	require.NoError(t, box.owner.Raw("SELECT count(*) FROM users WHERE id = ? AND organization_id = ?", userID, orgID).Scan(&stored).Error)
	assert.Equal(t, int64(1), stored)
}
