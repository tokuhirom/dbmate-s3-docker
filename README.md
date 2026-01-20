# dbmate-s3-docker

Dockerized database migration tool using [dbmate](https://github.com/amacneil/dbmate) with S3-compatible storage support.

## Features

- 🐳 **Containerized**: No installation required, runs anywhere Docker is available
- 📦 **S3 Integration**: Sync migration files from S3-compatible storage (AWS S3, MinIO, LocalStack, etc.)
- 🔄 **Automatic Sync**: Automatically downloads latest migrations before applying
- 🧪 **Testable**: Includes complete local testing environment with docker-compose
- 🚀 **Production Ready**: Can be used with any container orchestrator
- 📝 **Simple**: Single entrypoint script, minimal configuration

## Quick Start

### Local Testing

1. Clone the repository:
```bash
git clone https://github.com/tokuhirom/dbmate-s3-docker.git
cd dbmate-s3-docker
```

2. Run the test:
```bash
make test
```

This will:
- Start PostgreSQL and LocalStack (S3-compatible storage)
- Upload test migrations to S3
- Run dbmate to sync and apply migrations
- Verify the results

### Production Usage

#### 1. Build the Docker image

```bash
docker build -t dbmate-s3:latest .
```

#### 2. Run migrations

```bash
docker run --rm \
  -e DATABASE_URL="postgres://user:pass@host:5432/db?sslmode=require" \
  -e S3_BUCKET="my-migrations-bucket" \
  -e S3_ENDPOINT_URL="https://s3.amazonaws.com" \
  -e AWS_ACCESS_KEY_ID="your-access-key" \
  -e AWS_SECRET_ACCESS_KEY="your-secret-key" \
  dbmate-s3:latest
```

#### 3. Or use with docker-compose

Create a `docker-compose.yml`:

```yaml
services:
  dbmate:
    image: dbmate-s3:latest
    environment:
      DATABASE_URL: postgres://user:pass@db:5432/mydb?sslmode=require
      S3_BUCKET: my-migrations-bucket
      S3_ENDPOINT_URL: https://s3.amazonaws.com
      AWS_ACCESS_KEY_ID: ${AWS_ACCESS_KEY_ID}
      AWS_SECRET_ACCESS_KEY: ${AWS_SECRET_ACCESS_KEY}
      MIGRATIONS_PATH: migrations/
```

Run with:
```bash
docker compose run --rm dbmate
```

## Environment Variables

### Required

- `DATABASE_URL`: PostgreSQL connection string (format: `postgres://user:pass@host:port/db`)
- `S3_BUCKET`: S3 bucket name where migrations are stored
- `AWS_ACCESS_KEY_ID`: AWS access key ID
- `AWS_SECRET_ACCESS_KEY`: AWS secret access key

### Optional

- `S3_ENDPOINT_URL`: S3 endpoint URL (required for S3-compatible services like MinIO, Sakura Cloud)
- `MIGRATIONS_PATH`: Path in S3 bucket (default: `migrations/`)
- `MIGRATIONS_DIR`: Local directory for migrations (default: `/migrations`)
- `DBMATE_NO_DUMP_SCHEMA`: Skip schema dump (default: `true`)
- `AWS_DEFAULT_REGION`: AWS region (default: `us-east-1`)

## Migration File Format

Migration files should follow dbmate's naming convention:

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

## Makefile Commands

```bash
make help      # Show all available commands
make build     # Build the Docker image
make up        # Start test environment
make test      # Run complete migration test
make verify    # Verify migrations were applied
make clean     # Clean up everything
make logs      # Show logs
make shell     # Open shell in dbmate container
make psql      # Open PostgreSQL shell
make s3-ls     # List files in S3 bucket
```

## Testing Workflow

The test environment includes:

1. **PostgreSQL 16**: Target database
2. **LocalStack**: S3-compatible storage for testing
3. **dbmate**: Migration runner

Test workflow:
```bash
# 1. Start services
make up

# 2. Verify S3 has migrations
make s3-ls

# 3. Run migrations
make test

# 4. Check database
make verify

# 5. Cleanup
make clean
```

## CI/CD Integration

See [.github/workflows/test.yml](.github/workflows/test.yml) for GitHub Actions example.

## Architecture

```
┌─────────────────┐
│   S3 Bucket     │
│  (migrations/)  │
└────────┬────────┘
         │ sync
         ▼
┌─────────────────┐
│  dbmate         │
│  Container      │
└────────┬────────┘
         │ apply
         ▼
┌─────────────────┐
│  PostgreSQL     │
│  Database       │
└─────────────────┘
```

## License

MIT

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
