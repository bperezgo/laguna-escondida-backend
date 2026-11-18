package testsupport

import (
	"fmt"
	"io/fs"

	migrate "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// NewMigrator opens a migrator over one binary's embedded set (migrations.Cloud or migrations.Edge).
func NewMigrator(set fs.FS, databaseURL string) (*migrate.Migrate, error) {
	source, err := iofs.New(set, ".")
	if err != nil {
		return nil, fmt.Errorf("open migration set: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", source, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open migrator: %w", err)
	}
	return m, nil
}
