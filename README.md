# dbmate-s3-docker

[![GitHub release](https://img.shields.io/github/v/release/tokuhirom/dbmate-s3-docker)](https://github.com/tokuhirom/dbmate-s3-docker/releases)
[![Docker Image](https://img.shields.io/badge/docker-ghcr.io-blue)](https://github.com/tokuhirom/dbmate-s3-docker/pkgs/container/dbmate-s3-docker)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Database migration tool using [dbmate](https://github.com/amacneil/dbmate) with version-based migration management via S3-compatible storage.

## Features

- 🐳 **Containerized**: Runs migrations in Docker container
- 📦 **Version Management**: Date-based version control with completion tracking
- 🔄 **Incremental**: Only applies unapplied versions
- 📝 **Result Logging**: Uploads detailed result logs to S3
- 🚀 **Simple**: Minimal configuration, focused on reliability

## Quick Start

Pull the latest Docker image from GitHub Container Registry:

```bash
docker pull ghcr.io/tokuhirom/dbmate-s3-docker:latest
```

Run migration:

```bash
docker run --rm \
  -e DATABASE_URL="postgres://user:pass@host:5432/db?sslmode=require" \
  -e S3_BUCKET="your-bucket" \
  -e S3_PATH_PREFIX="migrations/" \
  -e S3_ENDPOINT_URL="https://s3.isk01.sakurastorage.jp" \
  -e AWS_ACCESS_KEY_ID="your-access-key" \
  -e AWS_SECRET_ACCESS_KEY="your-secret-key" \
  ghcr.io/tokuhirom/dbmate-s3-docker:latest
```

## How It Works

### Version Management

Migrations are organized by versions in S3. Each version contains **all migration files** up to that point (cumulative):

```
s3://your-bucket/${S3_PATH_PREFIX}/
  20260121010000/           # Version: YYYYMMDDHHMMSS
    migrations/             # Migration SQL files (directory name is fixed)
      20260101000000_create_users.sql
      20260102000000_add_email.sql
    result.json            # Execution result (created after run)
  20260121020000/           # Newer version
    migrations/             # Directory name "migrations/" is fixed and cannot be changed
      20260101000000_create_users.sql      # Previous migrations included
      20260102000000_add_email.sql         # Previous migrations included
      20260103000000_add_posts.sql         # New migration
    # No result.json = unapplied version
```

**Important**: Each version directory must contain **all migration files** from the beginning, not just new ones. This ensures dbmate can properly track which migrations have been applied.

**S3 Path Structure**: `s3://${S3_BUCKET}/${S3_PATH_PREFIX}${VERSION}/migrations/`

**Note**: The `migrations/` directory name within each version is fixed and cannot be customized.

### Execution Flow

1. List all version directories from S3 (sorted numerically)
2. Find the first version without `result.json`
3. Download migrations from that version
4. Run `dbmate up` to apply migrations
5. Upload `result.json` with execution details (both success and failure)

**Key behavior**: The tool applies **one version at a time**, starting from the oldest unapplied version. A version is considered applied if `result.json` exists, regardless of success or failure status.

## Project Structure

```
your-project/
├── Dockerfile                # Dockerfile for migration job
└── .github/
    └── workflows/
        └── migrate.yml       # GitHub Actions workflow
```

**Note**: Migration files are NOT bundled in the Docker image. They are stored in S3 and downloaded at runtime.

## Migration Files

Migration files follow dbmate's naming convention:

```
YYYYMMDDHHMMSS_description.sql
```

Example:

```sql
-- migrate:up
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- migrate:down
DROP TABLE users;
```

## Setup

### 1. Prepare S3 Structure

Create a version directory in S3 with **all migration files** (cumulative):

```bash
# Example: Create version 20260121010000 with initial migrations
# Assuming S3_PATH_PREFIX="migrations/"
aws s3 sync db/migrations/ \
  s3://your-bucket/migrations/20260121010000/migrations/

# Later: Create version 20260121020000 with all migrations (old + new)
# Copy ALL migration files, not just new ones
aws s3 sync db/migrations/ \
  s3://your-bucket/migrations/20260121020000/migrations/
```

**Important**:
- Version naming: Use `YYYYMMDDHHMMSS` format (e.g., `20260121153000` for 2026-01-21 15:30:00)
- Each version must contain **ALL migration files** from the beginning (cumulative)
- The `migrations/` subdirectory within each version directory is required and cannot be changed
- Use `aws s3 sync` to upload all files at once

### 2. Configure GitHub Secrets

Add the following secrets to your GitHub repository:

**Database:**
- `DATABASE_URL`: PostgreSQL connection string (format: `postgres://user:pass@host:port/db?sslmode=require`)

**S3 Storage:**
- `S3_BUCKET`: S3 bucket name
- `S3_PATH_PREFIX`: S3 path prefix (e.g., `migrations/`)
- `S3_ENDPOINT_URL`: S3 endpoint URL (optional, for S3-compatible services like Sakura Cloud)
- `AWS_ACCESS_KEY_ID`: AWS/S3-compatible access key
- `AWS_SECRET_ACCESS_KEY`: AWS/S3-compatible secret key

### 3. Copy files to your project

Copy this file to your project:
- `Dockerfile`

### 4. Build and run

```bash
# Build the image
docker build -t dbmate-migration:latest .

# Run migration
docker run --rm \
  -e DATABASE_URL="postgres://user:pass@host:5432/db?sslmode=require" \
  -e S3_BUCKET="your-bucket" \
  -e S3_PATH_PREFIX="migrations/" \
  -e S3_ENDPOINT_URL="https://s3.isk01.sakurastorage.jp" \
  -e AWS_ACCESS_KEY_ID="your-access-key" \
  -e AWS_SECRET_ACCESS_KEY="your-secret-key" \
  dbmate-migration:latest
```

## Environment Variables

**Required:**
- `DATABASE_URL`: PostgreSQL connection string
- `S3_BUCKET`: S3 bucket name
- `S3_PATH_PREFIX`: S3 path prefix (must end with `/`)

**Optional:**
- `S3_ENDPOINT_URL`: S3 endpoint URL (required for S3-compatible services)
- `AWS_ACCESS_KEY_ID`: AWS access key
- `AWS_SECRET_ACCESS_KEY`: AWS secret key
- `AWS_DEFAULT_REGION`: AWS region (default: `us-east-1`)
- `METRICS_ADDR`: Prometheus metrics endpoint address (e.g., `:9090`). Metrics disabled if not set

## Result JSON

After execution, `result.json` is uploaded to S3:

**Success example** (`s3://bucket/migrations/20260121010000/result.json`):

```json
{
  "version": "20260121010000",
  "status": "success",
  "timestamp": "2026-01-21T01:00:00Z",
  "migrations_applied": 2,
  "log": "[2026-01-21 01:00:00 UTC] === Starting database migration ===\n..."
}
```

**Failure example**:

```json
{
  "version": "20260121010000",
  "status": "failed",
  "timestamp": "2026-01-21T01:00:00Z",
  "error": "Failed to download migrations from S3",
  "log": "[2026-01-21 01:00:00 UTC] ✗ Failed to download...\n..."
}
```

## Version Management

A version is considered applied if `result.json` exists in its directory. The tool checks for `result.json` existence using S3 HeadObject (lightweight operation) before applying a version.

**To retry a failed migration**: Delete the `result.json` file from S3 and run the tool again.

## Deployment

### Running as a Daemon

The tool is designed to run as a long-running daemon process that continuously polls S3 for new migration versions:

```bash
docker run -d \
  --name dbmate-s3-docker \
  --restart unless-stopped \
  -e DATABASE_URL="postgres://user:pass@host:5432/db?sslmode=require" \
  -e S3_BUCKET="your-bucket" \
  -e S3_PATH_PREFIX="migrations/" \
  -e S3_ENDPOINT_URL="https://s3.isk01.sakurastorage.jp" \
  -e AWS_ACCESS_KEY_ID="your-access-key" \
  -e AWS_SECRET_ACCESS_KEY="your-secret-key" \
  dbmate-s3-docker:latest
```

The daemon will:
1. Check S3 for unapplied versions every time it runs
2. Apply the oldest unapplied version
3. Upload `result.json` to S3
4. Exit (restart by orchestrator to check again)

**Note**: Configure your orchestrator (Docker, Kubernetes, systemd, etc.) to restart the container after it exits.

### GitHub Actions Integration

Use GitHub Actions to upload new migration versions to S3:

```yaml
name: Upload Migrations

on:
  push:
    branches: [main]
    paths:
      - 'db/migrations/**'

jobs:
  upload:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Upload migrations to S3
        env:
          AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
          AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
          S3_ENDPOINT_URL: ${{ secrets.S3_ENDPOINT_URL }}
        run: |
          # Generate version timestamp
          VERSION=$(date -u +%Y%m%d%H%M%S)

          # Upload migration files
          aws s3 sync db/migrations/ \
            s3://${{ secrets.S3_BUCKET }}/migrations/${VERSION}/migrations/ \
            --endpoint-url=$S3_ENDPOINT_URL

          echo "Uploaded migrations as version: ${VERSION}"

      - name: Wait for completion and notify
        if: always()
        env:
          AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
          AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
          S3_ENDPOINT_URL: ${{ secrets.S3_ENDPOINT_URL }}
          SLACK_WEBHOOK_URL: ${{ secrets.SLACK_WEBHOOK_URL }}
        run: |
          VERSION=$(date -u +%Y%m%d%H%M%S)

          # Wait for result.json (timeout after 10 minutes)
          for i in {1..120}; do
            if aws s3 ls s3://${{ secrets.S3_BUCKET }}/migrations/${VERSION}/result.json \
                --endpoint-url=$S3_ENDPOINT_URL >/dev/null 2>&1; then
              echo "Migration completed"
              break
            fi
            echo "Waiting for migration... ($i/120)"
            sleep 5
          done

          # Download and parse result
          aws s3 cp \
            s3://${{ secrets.S3_BUCKET }}/migrations/${VERSION}/result.json \
            result.json \
            --endpoint-url=$S3_ENDPOINT_URL

          STATUS=$(jq -r '.status' result.json)
          LOG=$(jq -r '.log' result.json)

          # Notify Slack
          if [ "$STATUS" = "success" ]; then
            COLOR="good"
            EMOJI="✅"
          else
            COLOR="danger"
            EMOJI="❌"
          fi

          curl -X POST "$SLACK_WEBHOOK_URL" -H 'Content-Type: application/json' -d @- <<EOF
          {
            "attachments": [{
              "color": "$COLOR",
              "title": "$EMOJI Migration $STATUS",
              "fields": [
                {"title": "Version", "value": "$VERSION", "short": true},
                {"title": "Status", "value": "$STATUS", "short": true}
              ],
              "text": "```\n${LOG:0:1000}\n```"
            }]
          }
          EOF
```

## Local Testing

This repository includes a test environment with docker-compose for development:

```bash
# Run test
make test

# Verify database
make verify

# Cleanup
make clean
```

The test environment uses LocalStack for S3 and PostgreSQL for the database.

## Prometheus Metrics

When `METRICS_ADDR` environment variable is set, the tool exposes Prometheus metrics:

**Endpoint**: `http://<METRICS_ADDR>/metrics`

**Available metrics**:

- `dbmate_migration_attempts_total{status}` - Total number of migration attempts (labels: `success`, `failed`)
- `dbmate_migration_duration_seconds` - Duration of migration execution in seconds (histogram)
- `dbmate_last_migration_timestamp` - Timestamp of the last migration (unix seconds)
- `dbmate_current_version{version}` - Current migration version (gauge with version label)

**Example usage**:

```bash
docker run --rm \
  -e DATABASE_URL="..." \
  -e S3_BUCKET="..." \
  -e S3_PATH_PREFIX="migrations/" \
  -e METRICS_ADDR=":9090" \
  -p 9090:9090 \
  dbmate-s3-docker:latest
```

Then access metrics at `http://localhost:9090/metrics`.

## Workflow Best Practices

1. **Version Naming**: Use `date +%Y%m%d%H%M%S` to generate version names
2. **Incremental Versions**: Create a new version for each migration batch
3. **Testing**: Test migrations locally before uploading to S3
4. **Rollback**: Use dbmate's `-- migrate:down` for rollback support
5. **Monitoring**: Use Prometheus metrics and parse `result.json` for alerting

## Architecture

```
┌─────────────────┐
│  Docker         │
│  Container      │
└────────┬────────┘
         │
    ┌────┴────┐
    ▼         ▼
┌────────┐ ┌────────────────────┐
│  DB    │ │   S3 (Versions)   │
│ Migrate│ │ - Download files   │
│        │ │ - Upload results   │
└────────┘ └────────────────────┘
```

## Differences from db-schema-sync

This tool is inspired by [db-schema-sync](https://github.com/tokuhirom/db-schema-sync) but differs in:

- **Tool**: Uses dbmate instead of psqldef
- **File Format**: Uses dbmate's migration format (with `-- migrate:up/down`)
- **Versioning**: Simple date-based versions (YYYYMMDDHHMMSS)
- **Result Format**: JSON result with detailed logs

## Development & Release

### Docker Image Publishing

Docker images are automatically built and published to GitHub Container Registry (ghcr.io) via GitHub Actions:

**Continuous Integration** (`.github/workflows/docker.yml`):
- **Pull Requests**: Test build only (no push)
- **Main branch**: Build and push development images with branch name and commit SHA tags

**Release** (`.github/workflows/tagpr.yml`):
- **Version tags**: Build and push release images with semantic version tags (e.g., `v1.0.0`, `1.0`, `1`, `latest`)
- Triggered automatically when tagpr creates a release tag

### Making Container Images Public

After the first release, make the container image public:

1. Go to **Packages** in your GitHub repository
2. Click on the `dbmate-s3-docker` package
3. Go to **Package settings**
4. Scroll down to **Danger Zone**
5. Click **Change visibility** → **Public**

### Local Development

```bash
# Build locally
docker build -t dbmate-s3-docker:dev .

# Run tests
make test

# Verify
make verify
```

## License

MIT

## Related Projects

- [dbmate](https://github.com/amacneil/dbmate) - Database migration tool
- [db-schema-sync](https://github.com/tokuhirom/db-schema-sync) - Schema synchronization tool
