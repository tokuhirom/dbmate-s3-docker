# Claude Code Guidelines for dbmate-s3-docker

## Commit Messages

- **All commit messages must be written in English**
- Use conventional commit format when appropriate (e.g., `feat:`, `fix:`, `docs:`, `refactor:`)
- Keep commit messages concise and descriptive

## Documentation

- README.md should be written in English for broader accessibility
- Code comments should be in English
- User-facing documentation should be in English

## Code Style

- Follow Go best practices and idiomatic Go code
- Use meaningful variable and function names
- Keep functions focused and single-purpose

## Testing

- Test locally using the docker-compose setup
- Verify S3 integration with LocalStack
- Ensure PostgreSQL migrations work correctly

## Release Process

### Automated Release with tagpr

- This project uses [tagpr](https://github.com/Songmu/tagpr) for automated versioning
- On each merge to `main`, tagpr creates/updates a release PR
- When the release PR is merged, a git tag is automatically created
- Docker images are automatically built and pushed to ghcr.io with semantic version tags

### Docker Image Publishing

**GitHub Actions workflows**:
- `.github/workflows/docker.yml` - CI builds (PRs and main branch)
- `.github/workflows/tagpr.yml` - Release builds (version tags)

**Image tags**:
- `latest` - Latest stable release
- `v1.2.3` - Specific version
- `1.2`, `1` - Major/minor version aliases
- `main-<sha>` - Development builds from main branch

### First Release Setup

After the first release, make the container package public:
1. Go to GitHub repository → Packages → dbmate-s3-docker
2. Package settings → Change visibility → Public
