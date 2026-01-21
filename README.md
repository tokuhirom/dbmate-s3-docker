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

The daemon mode (default):
1. Polls S3 every `POLL_INTERVAL` (default: 30 seconds)
2. Checks the newest version
3. Applies the newest version (if unapplied)
4. Uploads `result.json` to S3
5. Continues polling indefinitely

**One-shot execution**: To run once and exit, use the `once` subcommand:

```bash
docker run --rm \
  -e DATABASE_URL="..." \
  -e S3_BUCKET="..." \
  -e S3_PATH_PREFIX="migrations/" \
  ghcr.io/tokuhirom/dbmate-s3-docker:latest once
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
            s3://${{ secrets.S3_BUCKET }}/${{ secrets.S3_PATH_PREFIX }}${VERSION}/migrations/ \
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
            if aws s3 ls s3://${{ secrets.S3_BUCKET }}/${{ secrets.S3_PATH_PREFIX }}${VERSION}/result.json \
                --endpoint-url=$S3_ENDPOINT_URL >/dev/null 2>&1; then
              echo "Migration completed"
              break
            fi
            echo "Waiting for migration... ($i/120)"
            sleep 5
          done

          # Download and parse result
          aws s3 cp \
            s3://${{ secrets.S3_BUCKET }}/${{ secrets.S3_PATH_PREFIX }}${VERSION}/result.json \
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
              "text": "\`\`\`\n${LOG:0:1000}\n\`\`\`"
            }]
          }
          EOF
```

This workflow:
- Triggers manually via `workflow_dispatch`
- Generates a version timestamp (YYYYMMDDHHMMSS format)
- Uploads all migration files to S3 under the new version
- Waits for the daemon to apply migrations
- Notifies Slack with the result (optional)

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

This repository includes a test environment with docker-compose for development:

```bash
# Build locally
docker build -t dbmate-s3-docker:dev .

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
