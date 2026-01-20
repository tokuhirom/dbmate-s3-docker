#!/bin/bash
set -euo pipefail

# Required environment variables:
# - DATABASE_URL: PostgreSQL connection string
# - S3_BUCKET: S3 bucket name
# - S3_ENDPOINT_URL: S3 endpoint URL (optional for AWS, required for compatible services)
# - AWS_ACCESS_KEY_ID: AWS access key
# - AWS_SECRET_ACCESS_KEY: AWS secret key
# - MIGRATIONS_PATH: Path in S3 bucket (default: migrations/)

MIGRATIONS_PATH="${MIGRATIONS_PATH:-migrations/}"
MIGRATIONS_DIR="${MIGRATIONS_DIR:-/migrations}"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"
}

error_exit() {
    log "ERROR: $*" >&2
    exit 1
}

# Validate required environment variables
: "${DATABASE_URL:?ERROR: DATABASE_URL is required}"
: "${S3_BUCKET:?ERROR: S3_BUCKET is required}"
: "${AWS_ACCESS_KEY_ID:?ERROR: AWS_ACCESS_KEY_ID is required}"
: "${AWS_SECRET_ACCESS_KEY:?ERROR: AWS_SECRET_ACCESS_KEY is required}"

log "Starting dbmate migration process"
log "Database: ${DATABASE_URL%%\?*}"  # Hide password/query params
log "S3 Bucket: s3://${S3_BUCKET}/${MIGRATIONS_PATH}"

# Build AWS CLI command
AWS_CMD="aws s3 sync"
if [ -n "${S3_ENDPOINT_URL:-}" ]; then
    AWS_CMD="$AWS_CMD --endpoint-url=${S3_ENDPOINT_URL}"
    log "Using S3-compatible endpoint: ${S3_ENDPOINT_URL}"
fi

# Sync migrations from S3
log "Syncing migrations from S3..."
if ! $AWS_CMD "s3://${S3_BUCKET}/${MIGRATIONS_PATH}" "${MIGRATIONS_DIR}/" --delete; then
    error_exit "Failed to sync migrations from S3"
fi

# Count migration files
MIGRATION_COUNT=$(find "${MIGRATIONS_DIR}" -type f -name "*.sql" 2>/dev/null | wc -l)
log "Found ${MIGRATION_COUNT} migration file(s)"

if [ "$MIGRATION_COUNT" -eq 0 ]; then
    log "No migration files found, exiting"
    exit 0
fi

# List migrations
log "Migration files:"
find "${MIGRATIONS_DIR}" -type f -name "*.sql" -exec basename {} \; | sort

# Run dbmate migrations
log "Running dbmate migrations..."
export DBMATE_MIGRATIONS_DIR="${MIGRATIONS_DIR}"
export DBMATE_NO_DUMP_SCHEMA="${DBMATE_NO_DUMP_SCHEMA:-true}"

if dbmate up; then
    log "✓ Migration completed successfully"
    exit 0
else
    error_exit "Migration failed"
fi
