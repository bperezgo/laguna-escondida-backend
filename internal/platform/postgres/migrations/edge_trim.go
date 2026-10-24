package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
)

// TrimBlocker is a table holding rows that exist only on the box, which the edge trim would destroy.
type TrimBlocker struct {
	Table string
	Rows  int64
}

// EdgeTrimBlockers counts what only the box holds: expenses are never synced, nor are extra
// categories, and a purchase entry is safe to drop only once the cloud confirmed its push.
func EdgeTrimBlockers(ctx context.Context, db *sql.DB) ([]TrimBlocker, error) {
	// The category codes are the ones migration 000036 seeds on every database.
	checks := []struct{ table, query string }{
		{"expenses", `SELECT COUNT(*) FROM expenses`},
		{"expense_categories", `SELECT COUNT(*) FROM expense_categories
			WHERE code NOT IN ('indirect_cost', 'expense', 'investment', 'rent', 'service')`},
		{"purchase_entries", `SELECT COUNT(*) FROM sync_outbox WHERE entity_type = 'purchase_entry' AND synced_at IS NULL`},
	}
	var blockers []TrimBlocker
	for _, c := range checks {
		var rows int64
		if err := db.QueryRowContext(ctx, c.query).Scan(&rows); err != nil {
			return nil, fmt.Errorf("count %s: %w", c.table, err)
		}
		if rows > 0 {
			blockers = append(blockers, TrimBlocker{Table: c.table, Rows: rows})
		}
	}
	return blockers, nil
}

// UpEdge migrates the box. Past the shared history it runs the trim only when EdgeTrimBlockers
// finds nothing; otherwise it stays at SharedVersion and the box boots untrimmed. A guard in
// SQL would leave the migration dirty instead, and a box that won't boot can't take orders.
func UpEdge(ctx context.Context, m *migrate.Migrate, db *sql.DB, logger *slog.Logger) error {
	version, dirty, err := m.Version()
	switch {
	case errors.Is(err, migrate.ErrNilVersion):
		version = 0
	case err != nil:
		return fmt.Errorf("read schema version: %w", err)
	case dirty:
		return fmt.Errorf("schema version %d is dirty", version)
	}

	if version < SharedVersion {
		if err = m.Migrate(SharedVersion); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("migrate to the shared version: %w", err)
		}
		version = SharedVersion
	}

	if version == SharedVersion {
		blockers, checkErr := EdgeTrimBlockers(ctx, db)
		if checkErr != nil {
			logger.ErrorContext(ctx, "Edge schema trim skipped: pre-check failed", slog.Any("error", checkErr))
			return nil
		}
		if len(blockers) > 0 {
			for _, b := range blockers {
				logger.ErrorContext(ctx, "Edge schema trim blocked: rows exist only on this box",
					slog.String("table", b.Table), slog.Int64("rows", b.Rows))
			}
			return nil
		}
	}

	if err = m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run edge migrations: %w", err)
	}
	return nil
}
