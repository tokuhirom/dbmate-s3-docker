# Claude Code Guidelines for dbmate-s3-docker

## Commit Messages

- **All commit messages must be written in English**
- Use conventional commit format when appropriate (e.g., `feat:`, `fix:`, `docs:`, `refactor:`)
- Keep commit messages concise and descriptive

## Documentation

- README.md should be written in English for broader accessibility
- Code comments should be in English
- User-facing documentation should be in English
- Do not include Kubernetes examples in README - keep deployment examples Docker-focused
- Focus on simplicity and common use cases

## Code Style

- Follow Go best practices and idiomatic Go code
- Use meaningful variable and function names
- Keep functions focused and single-purpose

## Testing

- Test locally using the docker-compose setup
- Verify S3 integration with LocalStack
- Ensure PostgreSQL migrations work correctly

## Migration Files (dbmate reference)

This project uses dbmate for migrations. Migration files follow dbmate's naming convention:

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

For more details, see [dbmate documentation](https://github.com/amacneil/dbmate).

## Release Process

### Automated Release with tagpr

- This project uses [tagpr](https://github.com/Songmu/tagpr) for automated versioning
- On each merge to `main`, tagpr creates/updates a release PR
- When the release PR is merged, a git tag is automatically created
- Docker images are automatically built and pushed to ghcr.io with semantic version tags

### Docker Image Publishing

Docker images are automatically built and published to GitHub Container Registry (ghcr.io) via GitHub Actions.

**GitHub Actions workflows**:
- `.github/workflows/docker.yml` - CI builds (PRs and main branch)
  - **Pull Requests**: Test build only (no push)
  - **Main branch**: Build and push development images with branch name and commit SHA tags
- `.github/workflows/tagpr.yml` - Version management
  - Creates/updates release PR on each merge to main
  - Creates git tag when release PR is merged
- `.github/workflows/release.yml` - Release builds (version tags)
  - **Version tags**: Build and push release images with semantic version tags
  - Triggered automatically when tagpr creates a release tag

**Image tags**:
- `latest` - Latest stable release
- `v1.2.3` - Specific version
- `1.2`, `1` - Major/minor version aliases
- `main-<sha>` - Development builds from main branch

### Making Container Images Public

After the first release, make the container image public:

1. Go to **Packages** in your GitHub repository
2. Click on the `dbmate-s3-docker` package
3. Go to **Package settings**
4. Scroll down to **Danger Zone**
5. Click **Change visibility** → **Public**
