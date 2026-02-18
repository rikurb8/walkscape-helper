# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Makefile with build, test, lint, and install targets
- golangci-lint configuration with comprehensive linters
- GitHub Actions CI workflow (build, test, lint, vet, format)
- Dockerfile with multi-stage build
- LICENSE file (MIT)
- Structured logging package (`internal/logging`)
- `--verbose` flag now enables debug logging
- SQLite connection pool configuration
- SQLite WAL mode and foreign keys support
- Additional storage tests for pragmas and connection pool

### Changed

- Improved error handling in output writers (no more ignored errors)
- Updated AGENTS.md with new Makefile commands

## [0.1.0] - 2024-01-01

### Added

- Initial release
- `wsh guide setup` - configure LLM/embedding/vector providers
- `wsh guide show` - display current configuration
- `wsh guide validate` - check configuration completeness
- `wsh character import` - import character JSON data
- `wsh character list` - list imported characters
- `wsh character show` - show character by ID or latest
- `wsh version` - print version info
- JSON output support via `--json` flag
- SQLite storage with migrations
- Viper for configuration management
