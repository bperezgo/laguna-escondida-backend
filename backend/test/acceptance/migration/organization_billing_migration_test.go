package migration

import (
	"context"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/platform/postgres/migrations"
	"laguna-escondida/backend/internal/platform/postgres/repository"
	"laguna-escondida/backend/test/acceptance/testsupport"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Migration 000098 (signup-card-step): one billing row per organization, with a period of
// monthly or annual, and a down step that drops it.
func TestMigration_OrganizationBilling(t *testing.T) {
	if !gateEnabled() {
		t.Skip("Skipping acceptance test. Set RUN_ACCEPTANCE_TESTS=true to run (needs Docker or DB_HOST).")
	}

	ctx := context.Background()
	base, container := resolveConn(t, ctx)
	if container != nil {
		defer func() { _ = container.Terminate(ctx) }()
	}
	const dbName = "laguna_accept_organization_billing"
	recreateDatabase(t, base, dbName)
	m, err := testsupport.NewMigrator(migrations.Cloud, base.migrateURL(dbName))
	require.NoError(t, err)
	defer func() { _, _ = m.Close() }()
	require.NoError(t, m.Migrate(98))

	db := openGorm(t, base.gormDSN(dbName))
	defer closeGorm(db)
	orgID := uuid.NewString()
	require.NoError(t, db.Exec(`INSERT INTO organizations (id, name, slug, status, plan) VALUES (?, 'Org', ?, 'pending', 'starter')`,
		orgID, "org-"+orgID[:8]).Error)
	repo := repository.NewBillingRepository(db)

	_, found, err := repo.Get(ctx, orgID)
	require.NoError(t, err)
	assert.False(t, found)

	require.NoError(t, repo.SetPeriod(ctx, orgID, "annual"))
	row, found, err := repo.Get(ctx, orgID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "annual", row.Period)
	assert.False(t, row.HasCard())

	assert.Error(t, repo.SetPeriod(ctx, orgID, "weekly"), "only monthly and annual are periods")

	acceptedAt := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, repo.SaveCard(ctx, orgID, dto.BillingCard{Provider: "wompi", PaymentSourceID: "3891", Brand: "VISA", LastFour: "4242", TermsAcceptedAt: acceptedAt}))
	row, _, err = repo.Get(ctx, orgID)
	require.NoError(t, err)
	assert.Equal(t, "annual", row.Period, "saving a card keeps the chosen period")
	assert.True(t, row.HasCard())
	assert.Equal(t, []string{"wompi", "3891", "VISA", "4242"}, []string{*row.Provider, *row.PaymentSourceID, *row.CardBrand, *row.CardLastFour})
	require.NotNil(t, row.TermsAcceptedAt)
	assert.WithinDuration(t, acceptedAt, *row.TermsAcceptedAt, time.Second)

	require.NoError(t, repo.SetPeriod(ctx, orgID, "monthly"))
	many, err := repo.GetMany(ctx, []string{orgID, uuid.NewString()})
	require.NoError(t, err)
	require.Len(t, many, 1)
	assert.Equal(t, "monthly", many[orgID].Period)
	assert.True(t, many[orgID].HasCard(), "changing the period keeps the card")

	require.NoError(t, m.Migrate(97))
	var exists bool
	require.NoError(t, db.Raw(`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'organization_billing')`).Scan(&exists).Error)
	assert.False(t, exists)
}
