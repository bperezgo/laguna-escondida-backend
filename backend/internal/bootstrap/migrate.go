package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"log/slog"

	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/postgres/migrations"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func runMigrations(ctx context.Context, dsn string, mode config.Mode, logger *slog.Logger) (err error) {
	migrationURL := convertDSNToURL(dsn)

	set := migrations.Cloud
	if mode == config.ModeEdge {
		set = migrations.Edge
	}
	source, err := iofs.New(set, ".")
	if err != nil {
		return fmt.Errorf("failed to open embedded migrations: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, migrationURL)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer func() {
		if sourceErr, databaseErr := m.Close(); sourceErr != nil || databaseErr != nil {
			err = fmt.Errorf("failed to close migrate instance: %w", errors.Join(sourceErr, databaseErr))
		}
	}()

	if mode == config.ModeEdge {
		db, openErr := sql.Open("pgx", migrationURL)
		if openErr != nil {
			return fmt.Errorf("failed to open the edge trim pre-check connection: %w", openErr)
		}
		defer func() { _ = db.Close() }()
		if err = migrations.UpEdge(ctx, m, db, logger); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}
		log.Println("Migrations completed")
		return nil
	}

	if err = m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	if err == migrate.ErrNoChange {
		log.Println("No new migrations to apply")
	} else {
		log.Println("Migrations completed successfully")
	}

	return nil
}

func convertDSNToURL(dsn string) string {
	// Parse DSN components
	var host, port, user, password, dbname, sslmode string

	// Simple parser for key=value format
	parts := make(map[string]string)
	for _, part := range splitDSN(dsn) {
		if kv := splitKeyValue(part); len(kv) == 2 {
			parts[kv[0]] = kv[1]
		}
	}

	host = parts["host"]
	port = parts["port"]
	user = parts["user"]
	password = parts["password"]
	dbname = parts["dbname"]
	sslmode = parts["sslmode"]

	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		user, password, host, port, dbname, sslmode)
}

func splitDSN(dsn string) []string {
	var parts []string
	var current string
	for _, char := range dsn {
		if char == ' ' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func splitKeyValue(kv string) []string {
	for i, char := range kv {
		if char == '=' {
			return []string{kv[:i], kv[i+1:]}
		}
	}
	return []string{kv}
}
