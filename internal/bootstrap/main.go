package bootstrap

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/observability"
	"laguna-escondida/backend/internal/platform/postgres/repository"

	"github.com/joho/godotenv"
)

// Main boots a binary of the given mode and serves it until SIGINT or SIGTERM.
func Main(mode config.Mode, load func() (Boot, error)) {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Before anything touches the database, so a box running the wrong binary changes nothing.
	boot, err := load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Installed before the logger so the slog OTLP bridge picks up the global logger provider.
	obs := boot.Shared.Observability
	providers, err := observability.Init(context.Background(), observability.InitConfig{
		Enabled:          obs.Enabled,
		ServiceName:      obs.ServiceName,
		ServiceVersion:   obs.ServiceVersion,
		Environment:      obs.Environment,
		OTLPEndpoint:     obs.OTLPEndpoint,
		TraceSampleRatio: obs.TraceSampleRatio,
		NodeID:           boot.NodeID,
		OrganizationID:   boot.OrganizationID,
		Mode:             string(mode),
	})
	if err != nil {
		log.Fatalf("Failed to initialize observability: %v", err)
	}
	logger := observability.NewSlogLogger(obs.Enabled)

	// Migrations run as the schema owner; the app may connect as a pos_app member, which can't run DDL.
	log.Println("Running database migrations...")
	if err = runMigrations(context.Background(), getMigrationDSN(), mode, logger); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	db, err := repository.NewDatabase(getDSN(getEnv("DB_USER", "postgres"), getEnv("DB_PASSWORD", "postgres")))
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	infra, err := NewInfra(boot.Shared, db.DB, logger)
	if err != nil {
		log.Fatalf("Failed to build infrastructure: %v", err)
	}
	app, err := boot.Assemble(infra)
	if err != nil {
		log.Fatalf("Failed to assemble the %s app: %v", mode, err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	runErr := app.Run(ctx)

	// Flush batched spans/logs last so nothing logged during shutdown is dropped.
	flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := providers.Shutdown(flushCtx); err != nil {
		log.Printf("observability shutdown: %v", err)
	}
	if runErr != nil {
		log.Fatalf("Server stopped: %v", runErr)
	}
	log.Println("Server exited")
}
