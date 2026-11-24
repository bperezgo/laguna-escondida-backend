// Package edgeboot boots the edge assembly the way a box does: as the app role, on a database the
// box migrated itself, with none of the cloud's secrets in its environment.
//
//	RUN_ACCEPTANCE_TESTS=true go test ./test/acceptance/edgeboot/...
package edgeboot

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"laguna-escondida/backend/internal/bootstrap"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/httpclient"
	"laguna-escondida/backend/internal/platform/postgres/migrations"
	"laguna-escondida/backend/pkg/eventbus"
	"laguna-escondida/backend/test/acceptance/testsupport"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/gin-gonic/gin"
	migrate "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	gormpg "gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const (
	appRole        = "laguna_accept_box_app"
	appRolePass    = "laguna_accept_box_app"
	boxOrgID       = "bb0cad3e-be13-4bad-9a37-bba9525bfece"
	boxUsername    = "mesero"
	boxPassword    = "correct horse"
	boxNodeID      = "ce6ebef8-37cf-576b-b7a8-15aaaaae44a1"
	firstBoxDBName = "laguna_accept_edgeboot_a"
	otherBoxDBName = "laguna_accept_edgeboot_b"
)

var (
	boxes       = map[string]boxDB{}
	pgContainer *testsupport.PostgresContainer
)

// boxDB is one box's database: owner is the migrating user, app is the connection the binary serves with.
type boxDB struct {
	owner, app *gorm.DB
}

func gateEnabled() bool { return os.Getenv("RUN_ACCEPTANCE_TESTS") == "true" }

func TestMain(m *testing.M) {
	if !gateEnabled() {
		os.Exit(m.Run())
	}
	if err := provision(); err != nil {
		fmt.Fprintln(os.Stderr, "acceptance DB provisioning failed:", err)
		os.Exit(1)
	}
	code := m.Run()
	if pgContainer != nil {
		_ = pgContainer.Terminate(context.Background())
	}
	os.Exit(code)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func provision() error {
	host, port, user, password, sslmode := os.Getenv("DB_HOST"), envOr("DB_PORT", "5432"),
		envOr("DB_USER", "postgres"), envOr("DB_PASSWORD", "postgres"), envOr("DB_SSLMODE", "disable")
	if host == "" {
		ctr, err := testsupport.StartPostgres(context.Background())
		if err != nil {
			return fmt.Errorf("boot postgres testcontainer (is Docker running?): %w", err)
		}
		pgContainer = ctr
		host, port, user, password, sslmode = ctr.Host, ctr.Port, ctr.User, ctr.Password, ctr.SSLMode
	}
	dsn := func(name, who, pass string) string {
		return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", host, port, who, pass, name, sslmode)
	}

	admin, err := openGorm(dsn("postgres", user, password))
	if err != nil {
		return err
	}
	defer closeGorm(admin)
	for _, name := range []string{firstBoxDBName, otherBoxDBName} {
		if err = admin.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", name)).Error; err != nil {
			return err
		}
	}
	// What the box's provisioning does: pos_app first, then a login role that belongs to it.
	if err = admin.Exec(`DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'pos_app') THEN
			CREATE ROLE pos_app NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS;
		END IF; END $$`).Error; err != nil {
		return err
	}
	if err = admin.Exec("DROP ROLE IF EXISTS " + appRole).Error; err != nil {
		return err
	}
	if err = admin.Exec(fmt.Sprintf("CREATE ROLE %s LOGIN PASSWORD '%s' NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS IN ROLE pos_app", appRole, appRolePass)).Error; err != nil {
		return err
	}

	for _, name := range []string{firstBoxDBName, otherBoxDBName} {
		if err = admin.Exec(fmt.Sprintf("CREATE DATABASE %s", name)).Error; err != nil {
			return err
		}
		migrator, migrateErr := testsupport.NewMigrator(migrations.Edge,
			fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, password, host, port, name, sslmode))
		if migrateErr != nil {
			return migrateErr
		}
		upErr := migrator.Up()
		_, _ = migrator.Close()
		if upErr != nil && upErr != migrate.ErrNoChange {
			return upErr
		}
		owner, ownerErr := openGorm(dsn(name, user, password))
		if ownerErr != nil {
			return ownerErr
		}
		app, appErr := openGorm(dsn(name, appRole, appRolePass))
		if appErr != nil {
			return appErr
		}
		boxes[name] = boxDB{owner: owner, app: app}
		if err = seedBox(owner); err != nil {
			return err
		}
	}
	return nil
}

func seedBox(owner *gorm.DB) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(boxPassword), bcrypt.MinCost)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if err = owner.Exec(`INSERT INTO organizations (id, name, slug, status) VALUES (?, 'Laguna', 'laguna', 'active')
		ON CONFLICT (id) DO NOTHING`, boxOrgID).Error; err != nil {
		return err
	}
	return owner.Exec(`INSERT INTO users (id, organization_id, username, name, password, active, created_at, updated_at)
		VALUES (?, ?, ?, 'Mesero', ?, true, ?, ?)`, uuid.NewString(), boxOrgID, boxUsername, string(hash), now, now).Error
}

func openGorm(dsn string) (*gorm.DB, error) {
	return gorm.Open(gormpg.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
}

func closeGorm(db *gorm.DB) {
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
}

// startBox assembles the edge binary on db as the app role, the way a restart would.
func startBox(t *testing.T, db boxDB, jwtSecret string) (*httptest.Server, *logBuffer) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	logs := &logBuffer{}
	logger := slog.New(slog.NewTextHandler(io.MultiWriter(logs, io.Discard), nil))
	bus := eventbus.NewGoChannelEventBus(watermill.NopLogger{})
	t.Cleanup(func() { _ = bus.Close() })
	cfg := &config.Edge{
		Shared: config.Shared{
			JWTSecret: jwtSecret, PhoneHomeRegion: "CO",
			Observability: config.Observability{ServiceName: "laguna-backend", MetricsPort: "0"},
		},
		OrganizationID: boxOrgID, NodeID: boxNodeID, CloudNodeID: config.CloudNodeIDFor(boxOrgID),
		CloudSyncURL: "http://127.0.0.1:1", NodeSyncKey: "node-key",
	}
	app, err := bootstrap.NewEdge(cfg, bootstrap.Infra{
		DB: db.app, Logger: logger, HTTPClient: httpclient.NewClient(logger), EventBus: bus,
	})
	require.NoError(t, err)
	server := httptest.NewServer(app.Router)
	t.Cleanup(server.Close)
	return server, logs
}
