# Changelog

## [v0.0.7](https://github.com/tokuhirom/dbmate-deployer/compare/v0.0.6...v0.0.7) - 2026-01-22
- docs: Fix wait-and-notify option name from --version to --migration-version by @tokuhirom in https://github.com/tokuhirom/dbmate-deployer/pull/26
- refactor: Rename project from dbmate-s3-docker to dbmate-deployer by @tokuhirom in https://github.com/tokuhirom/dbmate-deployer/pull/28
- refactor: Rename daemon subcommand to watch by @tokuhirom in https://github.com/tokuhirom/dbmate-deployer/pull/29

## [v0.0.6](https://github.com/tokuhirom/dbmate-s3-docker/compare/v0.0.5...v0.0.6) - 2026-01-22
- feat: Add comprehensive Go test suite with testcontainers by @tokuhirom in https://github.com/tokuhirom/dbmate-s3-docker/pull/22
- refactor: Remove docker compose based tests in favor of Go tests by @tokuhirom in https://github.com/tokuhirom/dbmate-s3-docker/pull/25
- Bump golang.org/x/crypto from 0.43.0 to 0.45.0 by @dependabot[bot] in https://github.com/tokuhirom/dbmate-s3-docker/pull/24

## [v0.0.5](https://github.com/tokuhirom/dbmate-s3-docker/compare/v0.0.4...v0.0.5) - 2026-01-21
- refactor: Split main.go by subcommands into internal packages by @tokuhirom in https://github.com/tokuhirom/dbmate-s3-docker/pull/18

## [v0.0.4](https://github.com/tokuhirom/dbmate-s3-docker/compare/v0.0.3...v0.0.4) - 2026-01-21
- Improve webhook testing with payload verification by @tokuhirom in https://github.com/tokuhirom/dbmate-s3-docker/pull/13
- Add test for version subcommand by @tokuhirom in https://github.com/tokuhirom/dbmate-s3-docker/pull/15
- Add test for daemon mode by @tokuhirom in https://github.com/tokuhirom/dbmate-s3-docker/pull/16
- feat: Add push subcommand for uploading migrations to S3 by @tokuhirom in https://github.com/tokuhirom/dbmate-s3-docker/pull/17

## [v0.0.3](https://github.com/tokuhirom/dbmate-s3-docker/compare/v0.0.2...v0.0.3) - 2026-01-21
- Add wait-and-notify subcommand with optional Slack notification by @tokuhirom in https://github.com/tokuhirom/dbmate-s3-docker/pull/11

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
