#!/bin/bash

# Script to run database migrations down
# Usage: ./scripts/migrate-down.sh

set -e

# Check if migrate CLI is installed
if ! command -v migrate &> /dev/null; then
    echo "Error: 'migrate' CLI tool is not installed"
    echo "Install it with: brew install golang-migrate (macOS) or see https://github.com/golang-migrate/migrate"
    exit 1
fi

MIGRATIONS_DIR="internal/platform/postgres/migrations"
DATABASE_URL="postgres://postgres:postgres@localhost:5432/laguna_escondida?sslmode=disable"

# Check if migrations directory exists
if [ ! -d "$MIGRATIONS_DIR" ]; then
    echo "Error: Migrations directory not found: $MIGRATIONS_DIR"
    exit 1
fi

# Check if postgres is running (optional check)
if ! docker ps | grep -q laguna-escondida-postgres; then
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

