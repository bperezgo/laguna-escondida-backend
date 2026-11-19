package repository

import (
	"context"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedIntegrationOrg(t *testing.T, db *Database) string {
	t.Helper()
	id := uuid.Must(uuid.NewV7()).String()
	require.NoError(t, db.DB.Exec("INSERT INTO organizations (id, name, slug) VALUES (?, 'Org', ?)", id, "org-"+id).Error)
	t.Cleanup(func() {
		db.DB.Exec("DELETE FROM organization_integrations WHERE organization_id = ?", id)
		db.DB.Exec("DELETE FROM organizations WHERE id = ?", id)
	})
	return id
}

func wompiRecord(secret string, at time.Time) *dto.OrganizationIntegrationRecord {
	return &dto.OrganizationIntegrationRecord{
		ID: uuid.Must(uuid.NewV7()).String(), Provider: dto.IntegrationProviderWompi, Environment: dto.IntegrationEnvironmentSandbox,
		Secrets: []byte(secret), SecretHints: map[string]string{"private_key": secret[len(secret)-4:]}, CreatedAt: at, UpdatedAt: at,
	}
}

func TestOrganizationIntegrationRepository_UpsertReplaces_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewOrganizationIntegrationRepository(db.DB)
	ctx := tenant.WithOrg(context.Background(), seedIntegrationOrg(t, db))
	first := time.Now().UTC().Truncate(time.Microsecond)

	created, err := repo.Upsert(ctx, wompiRecord("cipher-one-aaaa", first))
	require.NoError(t, err)
	replaced, err := repo.Upsert(ctx, wompiRecord("cipher-two-bbbb", first.Add(time.Hour)))
	require.NoError(t, err)

	assert.Equal(t, created.ID, replaced.ID, "one row per organization and provider")
	assert.True(t, first.Equal(replaced.CreatedAt), "created_at kept")
	assert.True(t, first.Add(time.Hour).Equal(replaced.UpdatedAt))
	assert.Equal(t, []byte("cipher-two-bbbb"), replaced.Secrets)
	assert.Equal(t, "bbbb", replaced.SecretHints["private_key"])
	assert.Equal(t, dto.IntegrationEnvironmentSandbox, replaced.Environment)

	list, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
}

func TestOrganizationIntegrationRepository_GetListDelete_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewOrganizationIntegrationRepository(db.DB)
	orgID := seedIntegrationOrg(t, db)
	ctx := tenant.WithOrg(context.Background(), orgID)

	_, found, err := repo.Get(ctx, dto.IntegrationProviderWompi)
	require.NoError(t, err)
	assert.False(t, found)
	list, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, list)

	_, err = repo.Upsert(ctx, wompiRecord("cipher-one-aaaa", time.Now().UTC()))
	require.NoError(t, err)
	got, found, err := repo.Get(ctx, dto.IntegrationProviderWompi)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, orgID, got.OrganizationID)

	deleted, err := repo.Delete(ctx, dto.IntegrationProviderWompi)
	require.NoError(t, err)
	assert.True(t, deleted)
	deleted, err = repo.Delete(ctx, dto.IntegrationProviderWompi)
	require.NoError(t, err)
	assert.False(t, deleted)
}

func TestOrganizationIntegrationRepository_AnotherOrganizationsRowIsInvisible_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewOrganizationIntegrationRepository(db.DB)
	ctxA := tenant.WithOrg(context.Background(), seedIntegrationOrg(t, db))
	ctxB := tenant.WithOrg(context.Background(), seedIntegrationOrg(t, db))
	_, err := repo.Upsert(ctxA, wompiRecord("cipher-of-a-aaaa", time.Now().UTC()))
	require.NoError(t, err)

	_, found, err := repo.Get(ctxB, dto.IntegrationProviderWompi)
	require.NoError(t, err)
	assert.False(t, found)
	list, err := repo.List(ctxB)
	require.NoError(t, err)
	assert.Empty(t, list)
	deleted, err := repo.Delete(ctxB, dto.IntegrationProviderWompi)
	require.NoError(t, err)
	assert.False(t, deleted)

	stored, err := repo.Upsert(ctxB, wompiRecord("cipher-of-b-bbbb", time.Now().UTC()))
	require.NoError(t, err)
	assert.Equal(t, []byte("cipher-of-b-bbbb"), stored.Secrets)
	a, found, err := repo.Get(ctxA, dto.IntegrationProviderWompi)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, []byte("cipher-of-a-aaaa"), a.Secrets, "B's upsert never touches A's row")
}

func TestOrganizationIntegrationRepository_RequiresAnOrganization_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewOrganizationIntegrationRepository(db.DB)

	_, _, err := repo.Get(context.Background(), dto.IntegrationProviderWompi)

	require.Error(t, err)
}
