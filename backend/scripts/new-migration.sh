#!/bin/bash

# Script to create a new migration file
# Usage: ./scripts/new-migration.sh migration_name cloud|edge
#
# shared/ (v1..v71) is frozen. A change to a table both binaries keep goes in both sets,
# with the same SQL; TestEdgeCloudSharedSchemaParity fails when one twin is missing.

set -e

if [ -z "$1" ] || { [ "$2" != "cloud" ] && [ "$2" != "edge" ]; }; then
    echo "Error: Migration name and set are required"
    echo "Usage: ./scripts/new-migration.sh migration_name cloud|edge"
    exit 1
fi

MIGRATION_NAME="$1"
SET="$2"
SOURCE_ROOT="internal/platform/postgres/migrations"
MIGRATIONS_DIR="${SOURCE_ROOT}/${SET}"

# Versions continue across shared/ and the chosen set; cloud/ and edge/ may reuse a number.
LAST_MIGRATION=$(ls -1 "$SOURCE_ROOT"/shared/*.up.sql "$MIGRATIONS_DIR"/*.up.sql 2>/dev/null | xargs -n1 basename | sort -r | head -n 1)

if [ -z "$LAST_MIGRATION" ]; then
    NEXT_VERSION="000001"
else
    LAST_VERSION=$(echo "$LAST_MIGRATION" | cut -d'_' -f1)
    NEXT_VERSION=$(printf "%06d" $((10#$LAST_VERSION + 1)))
fi

UP_FILE="${MIGRATIONS_DIR}/${NEXT_VERSION}_${MIGRATION_NAME}.up.sql"
DOWN_FILE="${MIGRATIONS_DIR}/${NEXT_VERSION}_${MIGRATION_NAME}.down.sql"

cat > "$UP_FILE" << EOT
-- Migration: ${MIGRATION_NAME}
-- Version: ${NEXT_VERSION}

EOT

cat > "$DOWN_FILE" << EOT
-- Migration: ${MIGRATION_NAME}
-- Version: ${NEXT_VERSION}

EOT

echo "Created migration files:"
echo "  Up:   $UP_FILE"
echo "  Down: $DOWN_FILE"
