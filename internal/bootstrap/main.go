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
func Main(mode config.Mode, assemble func(cfg *config.Config, infra Infra) (*App, error)) {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Before anything touches the database, so a box running the wrong binary changes nothing.
	cfg, err := config.NewConfig(mode)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Installed before the logger so the slog OTLP bridge picks up the global logger provider.
	providers, err := observability.Init(context.Background(), observability.InitConfig{
		Enabled:          cfg.ObservabilityEnabled,
		ServiceName:      cfg.ServiceName,
		ServiceVersion:   cfg.ServiceVersion,
		Environment:      cfg.Environment,
		OTLPEndpoint:     cfg.OTLPEndpoint,
		TraceSampleRatio: cfg.TraceSampleRatio,
		NodeID:           cfg.NodeID,
		OrganizationID:   cfg.OrganizationID,
		Mode:             string(mode),
	})
	if err != nil {
		log.Fatalf("Failed to initialize observability: %v", err)
	}
	logger := observability.NewSlogLogger(cfg.ObservabilityEnabled)

	// Migrations run as the schema owner; the app may connect as a pos_app member, which can't run DDL.
	log.Println("Running database migrations...")
	if err = runMigrations(getMigrationDSN()); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	db, err := repository.NewDatabase(getDSN(getEnv("DB_USER", "postgres"), getEnv("DB_PASSWORD", "postgres")))
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	infra, err := NewInfra(cfg, db.DB, logger)
	if err != nil {
		log.Fatalf("Failed to build infrastructure: %v", err)
	}
	app, err := assemble(cfg, infra)
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
