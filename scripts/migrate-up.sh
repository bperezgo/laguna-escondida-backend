#!/bin/bash

# Script to run database migrations up
# Usage: ./scripts/migrate-up.sh

set -e

# Check if migrate CLI is installed
if ! command -v migrate &> /dev/null; then
    echo "Error: 'migrate' CLI tool is not installed"
    echo "Install it with: brew install golang-migrate (macOS) or see https://github.com/golang-migrate/migrate"
    exit 1
fi

MIGRATIONS_DIR="internal/platform/postgres/migrations"

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

# Check if migrations directory exists
if [ ! -d "$MIGRATIONS_DIR" ]; then
    echo "Error: Migrations directory not found: $MIGRATIONS_DIR"
    exit 1
fi

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

echo "Running migrations up..."
migrate -path "$MIGRATIONS_DIR" -database "$DATABASE_URL" up

echo "Migrations completed successfully!"

