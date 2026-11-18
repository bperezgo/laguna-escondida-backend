package repository_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestPosAppRole_CanUseDataButNotSchema_Integration(t *testing.T) {
	db := newSyncTestDB(t)

	var bypassRLS bool
	require.NoError(t, db.DB.Raw("SELECT rolbypassrls FROM pg_roles WHERE rolname = 'pos_app'").Scan(&bypassRLS).Error)
	assert.False(t, bypassRLS)

	asPosApp := func(t *testing.T, sql string) error {
		t.Helper()
		return db.DB.Transaction(func(tx *gorm.DB) error {
			require.NoError(t, tx.Exec("SET LOCAL ROLE pos_app").Error)
			if err := tx.Exec(sql).Error; err != nil {
				return err
			}
			return tx.Exec("SELECT 1/0").Error // never commit what the probe wrote
		})
	}

	assert.ErrorContains(t, asPosApp(t, "SELECT COUNT(*) FROM products"), "division by zero", "reads are granted")
	assert.ErrorContains(t, asPosApp(t, "UPDATE products SET name = name WHERE false"), "division by zero", "writes are granted")
	assert.ErrorContains(t, asPosApp(t, "CREATE TABLE pos_app_probe (id int)"), "permission denied")
	assert.ErrorContains(t, asPosApp(t, "SELECT * FROM schema_migrations"), "permission denied")
}
