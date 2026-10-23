package postgres_test

import (
	"context"
	"testing"

	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/postgres"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pgdriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type probeModel struct {
	postgres.TenantOwned
	ID   string `gorm:"column:id;primaryKey"`
	Name string `gorm:"column:name"`
}

func (probeModel) TableName() string { return "probes" }

// dryRunDB renders SQL without a server.
func dryRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(pgdriver.New(pgdriver.Config{DSN: "host=127.0.0.1 port=1"}), &gorm.Config{DryRun: true, DisableAutomaticPing: true, SkipDefaultTransaction: true})
	require.NoError(t, err)
	return db
}

func TestTenantDB_WithoutOrganization_Errors(t *testing.T) {
	_, err := postgres.TenantDB(context.Background(), dryRunDB(t))

	assert.ErrorIs(t, err, domainError.ErrNoOrganization)
}

func TestTenantDB_FiltersEveryStatementByTheOrganization(t *testing.T) {
	ctx := tenant.WithOrg(context.Background(), "org-a")
	scoped, err := postgres.TenantDB(ctx, dryRunDB(t))
	require.NoError(t, err)

	first := scoped.Where("name = ?", "x").Find(&[]probeModel{}).Statement
	assert.Equal(t, `SELECT * FROM "probes" WHERE "probes"."organization_id" = $1 AND name = $2`, first.SQL.String())
	assert.Equal(t, []interface{}{"org-a", "x"}, first.Vars)

	second := scoped.Joins("JOIN others ON others.probe_id = probes.id").Find(&[]probeModel{}).Statement
	assert.Equal(t, `SELECT "probes"."organization_id","probes"."id","probes"."name" FROM "probes" JOIN others ON others.probe_id = probes.id WHERE "probes"."organization_id" = $1`,
		second.SQL.String(), "reusing the scoped db neither keeps the first query's conditions nor is ambiguous in a join")
}

func TestTenantOwned_StampsTheContextsOrganization(t *testing.T) {
	model := &probeModel{ID: "p1", Name: "x"}
	model.OrganizationID = "org-from-a-dto"
	ctx := tenant.WithOrg(context.Background(), "org-a")

	stmt := dryRunDB(t).WithContext(ctx).Create(model).Statement

	assert.Equal(t, "org-a", model.OrganizationID)
	assert.Contains(t, stmt.Vars, "org-a")
}

func TestTenantOwned_WithoutOrganization_RefusesTheWrite(t *testing.T) {
	err := dryRunDB(t).Create(&probeModel{ID: "p1"}).Error

	assert.ErrorIs(t, err, domainError.ErrNoOrganization)
}
