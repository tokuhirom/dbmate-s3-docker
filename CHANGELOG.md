# Changelog

## [v0.0.2](https://github.com/tokuhirom/dbmate-s3-docker/compare/v0.0.1...v0.0.2) - 2026-01-21
- Add GoReleaser for binary releases by @tokuhirom in https://github.com/tokuhirom/dbmate-s3-docker/pull/9

## [v0.0.1](https://github.com/tokuhirom/dbmate-s3-docker/commits/v0.0.1) - 2026-01-21
- Rewrite migration tool in Go with version-based management by @tokuhirom in https://github.com/tokuhirom/dbmate-s3-docker/pull/1
- Implement ticker-based daemon with subcommand support by @tokuhirom in https://github.com/tokuhirom/dbmate-s3-docker/pull/2
- Fix tagpr configuration and split release workflow by @tokuhirom in https://github.com/tokuhirom/dbmate-s3-docker/pull/3
- Fix tagpr configuration and split release workflow by @tokuhirom in https://github.com/tokuhirom/dbmate-s3-docker/pull/5
- Document branch protection in CLAUDE.md by @tokuhirom in https://github.com/tokuhirom/dbmate-s3-docker/pull/4

## [v0.0.1](https://github.com/tokuhirom/dbmate-s3-docker/commits/v0.0.1) - 2026-01-21
- Rewrite migration tool in Go with version-based management by @tokuhirom in https://github.com/tokuhirom/dbmate-s3-docker/pull/1
- Implement ticker-based daemon with subcommand support by @tokuhirom in https://github.com/tokuhirom/dbmate-s3-docker/pull/2
- Fix tagpr configuration and split release workflow by @tokuhirom in https://github.com/tokuhirom/dbmate-s3-docker/pull/3
- Fix tagpr configuration and split release workflow by @tokuhirom in https://github.com/tokuhirom/dbmate-s3-docker/pull/5
- Document branch protection in CLAUDE.md by @tokuhirom in https://github.com/tokuhirom/dbmate-s3-docker/pull/4

## [Unreleased]

- Initial release
- Go implementation with dbmate library
- Version-based migration management (YYYYMMDDHHMMSS format)
- S3 storage for migrations and results
- Prometheus metrics support
- result.json for execution tracking
