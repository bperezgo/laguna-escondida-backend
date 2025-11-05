#!/bin/bash

# Script to create a new migration file
# Usage: ./scripts/new-migration.sh migration_name

set -e

if [ -z "$1" ]; then
    echo "Error: Migration name is required"
    echo "Usage: ./scripts/new-migration.sh migration_name"
    exit 1
fi

MIGRATION_NAME="$1"
MIGRATIONS_DIR="internal/platform/postgres/migrations"

# Find the highest migration version number
LAST_MIGRATION=$(ls -1 "$MIGRATIONS_DIR"/*.up.sql 2>/dev/null | sort -r | head -n 1)

if [ -z "$LAST_MIGRATION" ]; then
    # No migrations exist, start with 000001
    NEXT_VERSION="000001"
else
    # Extract the version number from the last migration
    LAST_VERSION=$(basename "$LAST_MIGRATION" | cut -d'_' -f1)
    # Increment the version number
    NEXT_VERSION=$(printf "%06d" $((10#$LAST_VERSION + 1)))
fi

# Create migration file names
UP_FILE="${MIGRATIONS_DIR}/${NEXT_VERSION}_${MIGRATION_NAME}.up.sql"
DOWN_FILE="${MIGRATIONS_DIR}/${NEXT_VERSION}_${MIGRATION_NAME}.down.sql"

# Create the up migration file
cat > "$UP_FILE" << EOF
-- Migration: ${MIGRATION_NAME}
-- Version: ${NEXT_VERSION}

EOF

# Create the down migration file
cat > "$DOWN_FILE" << EOF
-- Migration: ${MIGRATION_NAME}
-- Version: ${NEXT_VERSION}

EOF

echo "Created migration files:"
echo "  Up:   $UP_FILE"
echo "  Down: $DOWN_FILE"

