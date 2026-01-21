# Test Specification

This document describes the testing approach and procedures for dbmate-s3-docker.

## Overview

The testing strategy consists of:
1. **Integration Tests**: End-to-end testing with real components (PostgreSQL, S3, dbmate)
2. **CI/CD Tests**: Automated testing in GitHub Actions
3. **Manual Testing**: Local development and verification

## Test Environment

### Local Testing Environment

The local test environment uses Docker Compose with the following services:

- **PostgreSQL**: Database server for migration testing
- **LocalStack**: S3-compatible storage emulator
- **S3 Setup**: Uploads test migration files to LocalStack
- **dbmate**: Migration execution container

Configuration: `docker-compose.yml`

## Integration Tests

### Test Setup

1. **Database**: PostgreSQL 16-alpine
   - User: `testuser`
   - Password: `testpass`
   - Database: `testdb`

2. **S3 Storage**: LocalStack 4.12.0
   - Endpoint: `http://localstack:4566`
   - Bucket: `migrations-bucket`
   - Path prefix: `migrations/`

3. **Test Migrations**: Located in `db/migrations/`
   - `20260120000001_create_users_table.sql`
   - `20260120000002_create_posts_table.sql`

### Test Execution

#### Running Tests Locally

```bash
# Build Docker image
make build

# Run full test suite
make test
```

The `make test` command performs:
1. Starts PostgreSQL and LocalStack
2. Creates S3 bucket and uploads migrations (version: 20260120000000)
3. Runs `dbmate once` to apply migrations
4. Verifies database schema

#### Manual Verification

```bash
# Verify database tables
make verify

# Check S3 bucket contents
make s3-check

# View service logs
make logs

# Access PostgreSQL shell
make psql
```

### Test Cases

#### TC-001: Migration File Upload to S3

**Objective**: Verify migration files are uploaded to S3 correctly

**Steps**:
1. Start LocalStack service
2. Run s3-setup container
3. Check bucket contents

**Expected Result**:
- Bucket `migrations-bucket` exists
- Path `migrations/20260120000000/migrations/` contains migration files
- Files: `20260120000001_create_users_table.sql`, `20260120000002_create_posts_table.sql`

**Verification**:
```bash
make s3-check
```

#### TC-002: Database Migration Execution

**Objective**: Verify dbmate successfully applies migrations

**Steps**:
1. Start PostgreSQL and LocalStack
2. Upload migrations to S3
3. Run `dbmate once`
4. Check database schema

**Expected Result**:
- `users` table created with columns: `id`, `email`, `created_at`
- `posts` table created with columns: `id`, `user_id`, `title`, `content`, `created_at`
- `schema_migrations` table contains migration records

**Verification**:
```bash
make verify
```

#### TC-003: Newest Version Detection

**Objective**: Verify tool applies only the newest version

**Steps**:
1. Upload version 20260120000000 to S3
2. Run `dbmate once` (applies version)
3. Upload version 20260121000000 to S3
4. Run `dbmate once` again

**Expected Result**:
- First run: Applies version 20260120000000, uploads `result.json`
- Second run: Detects and applies version 20260121000000
- Each version is applied only once

**Verification**:
- Check `result.json` exists for each version in S3
- Verify `schema_migrations` table for applied migrations

#### TC-004: Daemon Mode Polling

**Objective**: Verify daemon mode polls S3 periodically

**Steps**:
1. Start daemon with `POLL_INTERVAL=10s`
2. Upload new version after daemon starts
3. Wait for next poll cycle

**Expected Result**:
- Daemon continues running
- New version is detected and applied within poll interval
- `result.json` uploaded after successful migration

**Verification**:
```bash
docker logs dbmate-s3-docker
```

#### TC-005: One-shot Execution

**Objective**: Verify `once` subcommand exits after single execution

**Steps**:
1. Run `dbmate once` with unapplied version
2. Check container exit status

**Expected Result**:
- Container runs once and exits
- Exit code 0 if migration succeeds
- Non-zero exit code if migration fails

**Verification**:
```bash
docker compose run --rm dbmate once
echo $?
```

#### TC-006: Result JSON Format

**Objective**: Verify result.json contains correct information

**Steps**:
1. Run migration
2. Download `result.json` from S3
3. Parse JSON content

**Expected Result** (success):
```json
{
  "version": "20260120000000",
  "status": "success",
  "timestamp": "2026-01-21T01:00:00Z",
  "migrations_applied": 2,
  "log": "[timestamp] === Starting database migration ===\n..."
}
```

**Expected Result** (failure):
```json
{
  "version": "20260120000000",
  "status": "failed",
  "timestamp": "2026-01-21T01:00:00Z",
  "error": "Error description",
  "log": "[timestamp] ✗ Failed to...\n..."
}
```

#### TC-007: Already Applied Version

**Objective**: Verify tool skips already applied versions

**Steps**:
1. Apply version 20260120000000
2. Run `dbmate once` again with same version

**Expected Result**:
- Second run detects `result.json` exists
- No migration is executed
- Log message: "Newest version already applied"

**Verification**:
```bash
docker compose logs dbmate
```

#### TC-008: Missing S3 Credentials

**Objective**: Verify proper error handling for authentication failures

**Steps**:
1. Run dbmate with invalid AWS credentials
2. Check error message

**Expected Result**:
- Migration fails with clear error message
- `result.json` contains error details
- Exit code non-zero

#### TC-009: Database Connection Failure

**Objective**: Verify proper error handling for database failures

**Steps**:
1. Run dbmate with invalid `DATABASE_URL`
2. Check error message

**Expected Result**:
- Migration fails with connection error
- `result.json` uploaded with error status
- Log contains database error details

#### TC-010: Migration SQL Error

**Objective**: Verify handling of malformed migration files

**Steps**:
1. Upload migration with SQL syntax error
2. Run `dbmate once`
3. Check result

**Expected Result**:
- Migration fails with SQL error
- `result.json` status: "failed"
- Error field contains SQL error message
- Database state is rolled back (if supported by dbmate)

## CI/CD Tests

### GitHub Actions Workflow

File: `.github/workflows/test.yml`

**Trigger**:
- Push to `main` branch
- Pull requests to `main` branch

**Test Steps**:
1. Checkout code
2. Build dbmate Docker image
3. Start PostgreSQL and LocalStack
4. Wait for services (10 seconds)
5. Verify S3 setup logs
6. Run migrations (`dbmate once`)
7. Verify database schema
8. Check users table structure
9. Check posts table structure
10. Cleanup (always runs)

**Success Criteria**:
- All steps complete without errors
- Database tables match expected schema
- Migration records exist in `schema_migrations`

### Docker Build Tests

File: `.github/workflows/docker.yml`

**Test Cases**:
- **Pull Request**: Build Docker image (no push)
- **Main Branch**: Build and push to ghcr.io with development tags

## Performance Tests

### Migration Performance

**Objective**: Ensure migrations complete within acceptable time

**Metrics**:
- Time to download migrations from S3
- Time to execute dbmate migrations
- Total execution time

**Acceptance Criteria**:
- Small migrations (<10 files): < 30 seconds
- Medium migrations (10-100 files): < 2 minutes

### Polling Performance

**Objective**: Verify daemon doesn't consume excessive resources

**Metrics**:
- CPU usage during idle polling
- Memory usage over 24 hours
- Network traffic per poll

**Acceptance Criteria**:
- CPU usage < 5% during idle
- Memory usage remains stable
- Network traffic minimal (HeadObject requests only)

## Manual Testing Checklist

Before release, manually verify:

- [ ] Local test suite passes (`make test`)
- [ ] CI tests pass on GitHub Actions
- [ ] Docker image builds successfully
- [ ] Daemon mode runs continuously without errors
- [ ] One-shot mode exits after execution
- [ ] Version command displays correct version
- [ ] Newest version detection works correctly
- [ ] Result JSON format is valid
- [ ] Error handling works for common failures
- [ ] S3 endpoint customization works (Sakura Cloud, etc.)
- [ ] PostgreSQL SSL mode works
- [ ] Prometheus metrics (if enabled) expose correctly

## Test Data Cleanup

After testing:

```bash
# Stop and remove all containers
make clean

# Or manually
docker compose down -v
```

## Known Limitations

1. **LocalStack Version**: Pinned to 4.12.0 for x-amz-trailer header support
2. **Database**: Tests only cover PostgreSQL (dbmate supports other databases)
3. **S3 Operations**: Tests use LocalStack, not real S3 (different behavior possible)
4. **Concurrency**: Tests don't cover multiple daemon instances

## Future Test Improvements

- Add unit tests for individual functions
- Test with real S3 (integration test environment)
- Add stress tests (many migrations, large files)
- Test concurrent daemon instances
- Add MySQL/SQLite compatibility tests
- Test with real Sakura Cloud Object Storage
- Add chaos engineering tests (network failures, service restarts)
