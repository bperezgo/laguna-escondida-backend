#!/bin/bash

# Script to run database migrations down
# Usage: SET=cloud|edge ./scripts/migrate-down.sh   (default: cloud)

set -e

# Check if migrate CLI is installed
if ! command -v migrate &> /dev/null; then
    echo "Error: 'migrate' CLI tool is not installed"
    echo "Install it with: brew install golang-migrate (macOS) or see https://github.com/golang-migrate/migrate"
    exit 1
fi

SET="${SET:-cloud}"
SOURCE_ROOT="internal/platform/postgres/migrations"
if [ "$SET" != "cloud" ] && [ "$SET" != "edge" ]; then
    echo "Error: SET must be cloud or edge, got: $SET"
    exit 1
fi
if [ ! -d "$SOURCE_ROOT/shared" ] || [ ! -d "$SOURCE_ROOT/$SET" ]; then
    echo "Error: Migrations directory not found: $SOURCE_ROOT/{shared,$SET}"
    exit 1
fi
# The migrate CLI reads one directory; each binary runs shared/ plus its own set.
MIGRATIONS_DIR="$(mktemp -d)"
trap 'rm -rf "$MIGRATIONS_DIR"' EXIT
cp "$SOURCE_ROOT"/shared/*.sql "$MIGRATIONS_DIR"/
cp "$SOURCE_ROOT/$SET"/*.sql "$MIGRATIONS_DIR"/ 2>/dev/null || true

# Honour the DB_* environment (the dev container points them at its own postgres
# service); the defaults are the host docker-compose stack, so nothing changes for
# anyone running this on their machine.
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-postgres}"
DB_PASSWORD="${DB_PASSWORD:-postgres}"
DB_NAME="${DB_NAME:-laguna_escondida}"
DB_SSLMODE="${DB_SSLMODE:-disable}"
DATABASE_URL="${DATABASE_URL:-postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSLMODE}}"

# Check if postgres is running (optional check; only meaningful for the host stack)
if [ "$DB_HOST" = "localhost" ] && ! docker ps | grep -q laguna-escondida-postgres; then
    echo "Warning: PostgreSQL container 'laguna-escondida-postgres' doesn't seem to be running"
    echo "Start it with: docker-compose up -d"
    read -p "Continue anyway? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

echo "Running migrations down (rollback last migration)..."
migrate -path "$MIGRATIONS_DIR" -database "$DATABASE_URL" down 1

echo "Migration rollback completed successfully!"

