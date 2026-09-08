// Package testsupport provides shared infrastructure for the acceptance suites: it boots
// throwaway backing services (currently Postgres) via testcontainers so the tests need no
// manually-managed local database. It is imported only by acceptance *_test.go files.
package testsupport

import (
	"context"
	"fmt"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// PostgresContainer is a running throwaway Postgres plus the connection parameters the
// acceptance harness needs to reach it. Host/Port are the docker-mapped values.
type PostgresContainer struct {
	Host     string
	Port     string
	User     string
	Password string
	SSLMode  string

	terminate func(context.Context) error
}

// Terminate stops and removes the container. Safe to call on a nil receiver.
func (c *PostgresContainer) Terminate(ctx context.Context) error {
	if c == nil || c.terminate == nil {
		return nil
	}
	return c.terminate(ctx)
}

// StartPostgres boots a throwaway postgres:16-alpine container (matching docker-compose.yml)
// with an admin `postgres` superuser and waits until it accepts connections. The superuser
// lets the harness CREATE/DROP the per-run acceptance databases exactly as it does against a
// local Postgres. Requires a reachable Docker daemon.
func StartPostgres(ctx context.Context) (*PostgresContainer, error) {
	const user, password, database = "postgres", "postgres", "postgres"

	ctr, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase(database),
		postgres.WithUsername(user),
		postgres.WithPassword(password),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(90*time.Second),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("start postgres container: %w", err)
	}

	host, err := ctr.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("container host: %w", err)
	}
	mapped, err := ctr.MappedPort(ctx, "5432/tcp")
	if err != nil {
		return nil, fmt.Errorf("container mapped port: %w", err)
	}

	return &PostgresContainer{
		Host:      host,
		Port:      mapped.Port(),
		User:      user,
		Password:  password,
		SSLMode:   "disable",
		terminate: func(ctx context.Context) error { return ctr.Terminate(ctx) },
	}, nil
}
