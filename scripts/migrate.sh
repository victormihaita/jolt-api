#!/bin/bash

# Database migration script for Zalt backend
# Usage: ./scripts/migrate.sh [up|down|status]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="$(dirname "$SCRIPT_DIR")"
MIGRATIONS_DIR="$BACKEND_DIR/internal/database/migrations"

# Load .env file if it exists (only DATABASE_URL)
if [ -f "$BACKEND_DIR/.env" ]; then
    DATABASE_URL=$(grep '^DATABASE_URL=' "$BACKEND_DIR/.env" | cut -d'=' -f2-)
    export DATABASE_URL
fi

# Check DATABASE_URL is set
if [ -z "$DATABASE_URL" ]; then
    echo "Error: DATABASE_URL environment variable is not set"
    exit 1
fi

# Function to run a SQL file
run_sql_file() {
    local file=$1
    echo "Running: $(basename "$file")"
    psql "$DATABASE_URL" -f "$file"
}

# Function to run migrations up
migrate_up() {
    echo "Running migrations UP..."

    # Create migrations tracking table if not exists
    psql "$DATABASE_URL" -c "
        CREATE TABLE IF NOT EXISTS schema_migrations (
            version VARCHAR(255) PRIMARY KEY,
            applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
        );
    " 2>/dev/null || true

    # Get applied migrations
    applied=$(psql "$DATABASE_URL" -t -c "SELECT version FROM schema_migrations ORDER BY version;" 2>/dev/null | tr -d ' ')

    # Run each up migration in order
    for file in "$MIGRATIONS_DIR"/*.up.sql; do
        if [ -f "$file" ]; then
            version=$(basename "$file" | cut -d'_' -f1)

            if echo "$applied" | grep -q "^${version}$"; then
                echo "Skipping: $(basename "$file") (already applied)"
            else
                run_sql_file "$file"
                psql "$DATABASE_URL" -c "INSERT INTO schema_migrations (version) VALUES ('$version');" 2>/dev/null
                echo "Applied: $(basename "$file")"
            fi
        fi
    done

    echo "Migrations complete!"
}

# Function to run migrations down (rollback last)
migrate_down() {
    echo "Rolling back last migration..."

    # Get last applied migration
    last=$(psql "$DATABASE_URL" -t -c "SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1;" 2>/dev/null | tr -d ' ')

    if [ -z "$last" ]; then
        echo "No migrations to rollback"
        exit 0
    fi

    # Find and run the down migration
    for file in "$MIGRATIONS_DIR"/${last}_*.down.sql; do
        if [ -f "$file" ]; then
            run_sql_file "$file"
            psql "$DATABASE_URL" -c "DELETE FROM schema_migrations WHERE version = '$last';" 2>/dev/null
            echo "Rolled back: $(basename "$file")"
        fi
    done
}

# Function to show migration status
migrate_status() {
    echo "Migration Status:"
    echo "================="

    # Check if tracking table exists
    exists=$(psql "$DATABASE_URL" -t -c "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'schema_migrations');" 2>/dev/null | tr -d ' ')

    if [ "$exists" != "t" ]; then
        echo "No migrations have been run yet"
        return
    fi

    echo ""
    echo "Applied migrations:"
    psql "$DATABASE_URL" -c "SELECT version, applied_at FROM schema_migrations ORDER BY version;" 2>/dev/null

    echo ""
    echo "Available migrations:"
    for file in "$MIGRATIONS_DIR"/*.up.sql; do
        if [ -f "$file" ]; then
            echo "  - $(basename "$file")"
        fi
    done
}

# Main
case "${1:-up}" in
    up)
        migrate_up
        ;;
    down)
        migrate_down
        ;;
    status)
        migrate_status
        ;;
    *)
        echo "Usage: $0 [up|down|status]"
        exit 1
        ;;
esac
