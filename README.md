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
- 🔔 **Wait & Notify**: Built-in command to wait for migration completion and send Slack notifications
- 🚀 **Simple**: Minimal configuration, focused on reliability

## How It Works

### Upload Phase (GitHub Actions)

```mermaid
sequenceDiagram
    participant Dev as Developer
    participant GitHub as GitHub
    participant GHA as GitHub Actions
    participant S3 as S3 Storage
    participant Slack as Slack

    Dev->>GitHub: Trigger workflow_dispatch
    GitHub->>GHA: Start workflow
    GHA->>GHA: Generate version (YYYYMMDDHHMMSS)
    GHA->>S3: Upload all migration files<br/>s3://bucket/PREFIX/VERSION/migrations/*.sql
    Note over S3: Version created<br/>(no result.json yet)

    loop Wait for completion (max 10 minutes)
        GHA->>S3: Check result.json exists?
        alt result.json found
            S3-->>GHA: Found
            GHA->>S3: Download result.json
            S3-->>GHA: Return result data
            GHA->>GHA: Parse status and log
            GHA->>Slack: Notify result (success/failure)
        else timeout
            Note over GHA: Continue waiting...
        end
    end
```

### Execution Phase (Daemon Container)

```mermaid
sequenceDiagram
    participant Daemon as dbmate-s3-docker
    participant S3 as S3 Storage
    participant DB as PostgreSQL

    Note over Daemon: Daemon mode (ticker-based polling)

    loop Every POLL_INTERVAL (default: 30s)
        Daemon->>S3: List versions
        S3-->>Daemon: Return version directories
        Daemon->>S3: Check result.json for newest version (HeadObject)

        alt Newest version unapplied
            S3-->>Daemon: Found unapplied version: 20260121010000
            Daemon->>S3: Download migrations/*.sql
            S3-->>Daemon: Return migration files
            Daemon->>DB: Apply migrations (dbmate up)
            DB-->>Daemon: Success
            Daemon->>S3: Upload result.json
            Note over S3: Version marked as applied
        else Newest version already applied
            S3-->>Daemon: Newest version already applied
            Note over Daemon: Wait for next tick
        end
    end
```

**Key Points:**
- **GitHub Actions**: Uploads new migration versions to S3 (triggered by workflow_dispatch)
- **Daemon Container**: Polls S3 periodically (ticker-based), applies the newest version if unapplied
- **S3 Storage**: Central repository for versioned migrations and execution results
- **PostgreSQL**: Target database where migrations are applied
- **Version Tracking**: `result.json` existence indicates applied version (checked via HeadObject)

## Setup

### 1. Deploy Daemon Container

Run the container as a long-running daemon that polls S3 periodically:

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
  -e POLL_INTERVAL="30s" \
  ghcr.io/tokuhirom/dbmate-s3-docker:latest daemon
```

### 2. Configure GitHub Actions

#### 2.1 GitHub Secrets

Add the following secrets to your GitHub repository:

**Required secrets:**
- `S3_BUCKET`: S3 bucket name
- `S3_PATH_PREFIX`: S3 path prefix (e.g., `migrations/`)
- `S3_ENDPOINT_URL`: S3 endpoint URL (optional, for S3-compatible services like Sakura Cloud)
- `AWS_ACCESS_KEY_ID`: AWS/S3-compatible access key
- `AWS_SECRET_ACCESS_KEY`: AWS/S3-compatible secret key

#### 2.2 Workflow Setup

Create `.github/workflows/upload-migrations.yml` in your repository:

```yaml
name: Upload Migrations

on:
  workflow_dispatch:

jobs:
  upload:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Generate migration version
        run: |
          VERSION=$(date -u +%Y%m%d%H%M%S)
          echo "MIGRATION_VERSION=$VERSION" >> $GITHUB_ENV
          echo "Generated version: $VERSION"

      - name: Download dbmate-s3-docker
        run: |
          curl -LO https://github.com/tokuhirom/dbmate-s3-docker/releases/latest/download/dbmate-s3-docker_linux_amd64.tar.gz
          tar -xzf dbmate-s3-docker_linux_amd64.tar.gz
          chmod +x dbmate-s3-docker

      - name: Upload migrations to S3
        env:
          AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
          AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
          S3_ENDPOINT_URL: ${{ secrets.S3_ENDPOINT_URL }}
          S3_BUCKET: ${{ secrets.S3_BUCKET }}
          S3_PATH_PREFIX: ${{ secrets.S3_PATH_PREFIX }}
        run: |
          ./dbmate-s3-docker push \
            --migrations-dir=db/migrations \
            --version=${{ env.MIGRATION_VERSION }}

      - name: Wait for completion and notify
        if: always()
        env:
          S3_BUCKET: ${{ secrets.S3_BUCKET }}
          S3_PATH_PREFIX: ${{ secrets.S3_PATH_PREFIX }}
          S3_ENDPOINT_URL: ${{ secrets.S3_ENDPOINT_URL }}
          AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
          AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
          SLACK_INCOMING_WEBHOOK: ${{ secrets.SLACK_WEBHOOK_URL }}
        run: |
          ./dbmate-s3-docker wait-and-notify --version=${{ env.MIGRATION_VERSION }}
```

This workflow:
- Triggers manually via `workflow_dispatch`
- Generates a version timestamp (YYYYMMDDHHMMSS format)
- Downloads the dbmate-s3-docker binary
- Uses the `push` command to upload migrations with the generated version
- Uses `wait-and-notify` command to wait for daemon to apply migrations
- Sends Slack notification with migration result (if `SLACK_WEBHOOK_URL` secret is configured)

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
2. Check the newest version for `result.json`
3. If the newest version is unapplied, download migrations from that version
4. Run `dbmate up` to apply migrations
5. Upload `result.json` with execution details (both success and failure)

**Key behavior**: The tool applies the **newest version**. If the newest version is already applied, no action is taken. A version is considered applied if `result.json` exists, regardless of success or failure status.

## Commands

### daemon (default)

Runs as a long-running daemon that polls S3 periodically for new migration versions.

```bash
docker run -d \
  -e DATABASE_URL="postgres://user:pass@host:5432/db" \
  -e S3_BUCKET="your-bucket" \
  -e S3_PATH_PREFIX="migrations/" \
  -e POLL_INTERVAL="30s" \
  ghcr.io/tokuhirom/dbmate-s3-docker:latest daemon
```

### once

Runs once and exits. Useful for one-time migrations or debugging.

```bash
docker run --rm \
  -e DATABASE_URL="postgres://user:pass@host:5432/db" \
  -e S3_BUCKET="your-bucket" \
  -e S3_PATH_PREFIX="migrations/" \
  ghcr.io/tokuhirom/dbmate-s3-docker:latest once
```

### push

Uploads migration files to S3. This eliminates the need for AWS CLI in your CI/CD pipeline.

**Usage:**

```bash
# Generate version
VERSION=$(date -u +%Y%m%d%H%M%S)

# Upload migrations
./dbmate-s3-docker push \
  --migrations-dir=db/migrations \
  --s3-bucket=your-bucket \
  --s3-path-prefix=migrations/ \
  --version=$VERSION

# Use same version for wait-and-notify
./dbmate-s3-docker wait-and-notify --version=$VERSION
```

**Dry run (preview without uploading):**

```bash
VERSION=$(date -u +%Y%m%d%H%M%S)
./dbmate-s3-docker push \
  --migrations-dir=db/migrations \
  --version=$VERSION \
  --dry-run
```

**Flags:**

- `--migrations-dir, -m` (required): Local directory containing migration files
- `--s3-bucket` (required): S3 bucket name (also via `S3_BUCKET` env var)
- `--s3-path-prefix` (required): S3 path prefix (also via `S3_PATH_PREFIX` env var)
- `--version, -v` (required): Version timestamp (YYYYMMDDHHMMSS)
- `--dry-run`: Show what would be uploaded without uploading
- `--validate`: Validate migration files before upload (default: true)

### wait-and-notify

Waits for a specific migration version to complete and optionally sends a Slack notification. This command simplifies GitHub Actions workflows by replacing shell scripts that poll S3 and parse results.

**Usage:**

```bash
./dbmate-s3-docker wait-and-notify \
  --version=20260121010000 \
  --timeout=10m \
  --poll-interval=5s
```

**With Slack notification:**

```bash
./dbmate-s3-docker wait-and-notify \
  --version=20260121010000 \
  --slack-incoming-webhook=https://hooks.slack.com/services/YOUR/WEBHOOK/URL
```

**Flags:**

- `--version` (required): Migration version to wait for (YYYYMMDDHHMMSS format)
- `--slack-incoming-webhook`: Slack incoming webhook URL (optional, also via `SLACK_INCOMING_WEBHOOK` env var)
- `--timeout`: Maximum wait time (default: `10m`)
- `--poll-interval`: Polling interval for checking result.json (default: `5s`)

**Behavior:**

1. Polls S3 for `result.json` at the specified version
2. Returns immediately if result already exists (optimization)
3. Downloads and parses the result when found
4. Sends Slack notification if webhook URL provided (with color-coded status, emoji, and log excerpt)
5. Exits with code 0 if migration succeeded, 1 if failed or timed out
6. Slack notification failures are logged but don't fail the command

**Slack Notification Format:**

The notification includes:
- Color: green (success) or red (failure)
- Emoji: ✅ (success) or ❌ (failure)
- Fields: Version and Status
- Log excerpt: First 1000 characters of migration log

**Example in GitHub Actions:**

See the [workflow example above](#22-workflow-setup) for usage in CI/CD pipelines.

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
- `POLL_INTERVAL`: Polling interval for daemon mode (default: `30s`). Examples: `10s`, `1m`, `5m`
- `METRICS_ADDR`: Prometheus metrics endpoint address (e.g., `:9090`). Metrics disabled if not set
- `SLACK_INCOMING_WEBHOOK`: Slack incoming webhook URL for `wait-and-notify` command (optional)

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

## Local Testing

### Go Test Suite

The project includes a comprehensive Go test suite with unit and integration tests.

**Run all tests:**

```bash
# Run all tests (unit + integration)
make test

# Or directly with go test
go test -v ./...
```

**Run unit tests only (fast, no Docker required):**

```bash
# Run unit tests only (skips integration tests)
make test-unit

# Or with go test -short
go test -v -short ./...
```

**Run integration tests only (requires Docker for testcontainers):**

```bash
# Run integration tests
make test-integration

# Or with go test
go test -v -run Integration ./...
```

**Test organization:**
- **Unit tests**: Fast tests using mocks, no external dependencies
  - Migration validation tests
  - Slack notification tests (using httptest)
  - S3 operations tests (using mock S3 client)
- **Integration tests**: End-to-end tests using [testcontainers-go](https://golang.testcontainers.org/)
  - Requires Docker for running PostgreSQL and LocalStack containers
  - Containers are automatically started and stopped during tests
  - Tests are named with `Integration` suffix (e.g., `TestOnceIntegration`)

**Continuous Integration:**

The project runs both unit and integration tests automatically on every push and pull request via GitHub Actions. See `.github/workflows/go-tests.yml` for the CI configuration.

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

## Differences from db-schema-sync

This tool is inspired by [db-schema-sync](https://github.com/tokuhirom/db-schema-sync) but differs in:

- **Tool**: Uses dbmate instead of psqldef
- **File Format**: Uses dbmate's migration format (with `-- migrate:up/down`)
- **Versioning**: Simple date-based versions (YYYYMMDDHHMMSS)
- **Result Format**: JSON result with detailed logs

## License

MIT

## Related Projects

- [dbmate](https://github.com/amacneil/dbmate) - Database migration tool
- [db-schema-sync](https://github.com/tokuhirom/db-schema-sync) - Schema synchronization tool
