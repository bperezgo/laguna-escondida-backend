package migration

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"testing"

	"laguna-escondida/backend/internal/platform/postgres/migrations"
	"laguna-escondida/backend/test/acceptance/testsupport"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A box migrates as the database owner, which cannot create roles, so migration 65 needs
// pos_app to exist beforehand. Without it UpEdge must stop before 65 with the schema clean.
//
//	RUN_ACCEPTANCE_TESTS=true go test ./test/acceptance/migration/...

const (
	edgeRolesDBName = "laguna_accept_edge_roles"
	boxOwner        = "laguna_accept_box_owner"
)

func TestEdgeMigrations_PosAppRole(t *testing.T) {
	if !gateEnabled() {
		t.Skip("Skipping acceptance test. Set RUN_ACCEPTANCE_TESTS=true to run (needs Docker or DB_HOST).")
	}

	ctx := context.Background()
	base, container := resolveConn(t, ctx)
	if container != nil {
		defer func() { _ = container.Terminate(ctx) }()
	}

	admin := openGorm(t, base.gormDSN("postgres"))
	defer closeGorm(admin)
	exec := func(query string) { require.NoError(t, admin.Exec(query).Error, query) }

	// Roles are cluster-wide: hide an existing pos_app (other test databases hold grants to it).
	var posAppExists bool
	require.NoError(t, admin.Raw(`SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'pos_app')`).Scan(&posAppExists).Error)
	if posAppExists {
		exec(`ALTER ROLE pos_app RENAME TO pos_app_hidden`)
	}
	exec(fmt.Sprintf(`CREATE ROLE %s LOGIN PASSWORD '%s' NOSUPERUSER NOCREATEROLE`, boxOwner, boxOwner))
	recreateDatabase(t, base, edgeRolesDBName)
	exec(fmt.Sprintf(`ALTER DATABASE %s OWNER TO %s`, edgeRolesDBName, boxOwner))
	t.Cleanup(func() {
		exec(fmt.Sprintf(`DROP DATABASE IF EXISTS %s WITH (FORCE)`, edgeRolesDBName))
		exec(`DROP ROLE IF EXISTS pos_app`)
		exec(`DROP ROLE IF EXISTS ` + boxOwner)
		if posAppExists {
			exec(`ALTER ROLE pos_app_hidden RENAME TO pos_app`)
		}
	})

	owner := base
	owner.user, owner.password = boxOwner, boxOwner
	m, err := testsupport.NewMigrator(migrations.Edge, owner.migrateURL(edgeRolesDBName))
	require.NoError(t, err)
	defer func() { _, _ = m.Close() }()
	require.NoError(t, m.Migrate(migrations.PosAppRoleVersion-1))
	sqlDB, err := sql.Open("pgx", owner.migrateURL(edgeRolesDBName))
	require.NoError(t, err)
	defer func() { _ = sqlDB.Close() }()
	logger := slog.New(slog.DiscardHandler)

	t.Run("without pos_app the box stops before 65 with a clean schema", func(t *testing.T) {
		err := migrations.UpEdge(ctx, m, sqlDB, logger)

		require.ErrorIs(t, err, migrations.ErrPosAppRoleMissing)
		version, dirty, err := m.Version()
		require.NoError(t, err)
		assert.Equal(t, uint(migrations.PosAppRoleVersion-1), version)
		assert.False(t, dirty)
	})

	t.Run("once provisioning creates pos_app the owner migrates through the trim", func(t *testing.T) {
		exec(`CREATE ROLE pos_app NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS`)

		require.NoError(t, migrations.UpEdge(ctx, m, sqlDB, logger))

		version, dirty, err := m.Version()
		require.NoError(t, err)
		assert.Equal(t, uint(trimVersion), version)
		assert.False(t, dirty)
	})
}
