# Next Steps (Phase 1)

This phase builds the project foundation before wiki scraping and full guide answering.

Status: baseline implementation complete.

## Goals

- ship a working CLI skeleton
- persist guide configuration in SQLite
- import and store character JSON in SQLite
- support `--json` output on all commands
- fully test all shipped CLI functionality

## Scope

### In Scope

- root CLI setup and shared flags (`--json`, `--verbose`, `--db-path`)
- guide configuration commands (`setup`, `show`, `validate`)
- character commands (`import`, `list`, `show`)
- SQLite schema + migrations for guide and character data
- command and integration tests

### Out of Scope

- wiki scraping/indexing/sync
- vector DB ingestion/search
- `guide ask` answer generation
- runtime docker-compose stack for retrieval/LLM services

## Concrete Implementation Plan

1. Bootstrap CLI project
   - initialize Go module
   - add cobra root command and command packages
   - add shared output and error handling helpers

2. Add configuration layer
   - use viper for config file + env + flags
   - enforce env-var-only credentials
   - store env var names in config, never secret values

3. Implement SQLite foundation
   - create DB initialization package
   - add migration runner
   - create initial tables: `guide_config`, `characters`, `app_meta`

4. Implement guide configuration commands
   - `guide setup` for provider/model/persona/env-var references
   - `guide show` for current config
   - `guide validate` for completeness checks

5. Implement character commands
   - import from `--from-stdin`, `--raw-json`, `--from-file`
   - validate JSON object payload
   - store raw JSON and metadata (hash, source, timestamp)
   - list and show entries

6. Standardize command output
   - human-readable output defaults
   - stable JSON envelope under `--json`
   - consistent non-zero exit codes for errors

7. Test all CLI functionality
   - unit tests for validation, storage helpers, and output formatting
   - command tests for success/failure in human + JSON modes
   - integration tests with temp SQLite database

## Deliverables

- working `wsh` binary with phase-1 commands
- migration-backed SQLite schema
- passing automated tests for all implemented commands
- updated docs and examples

## Acceptance Criteria

- all implemented commands accept `--json`
- character import supports paste-first workflows (`stdin` and raw JSON)
- guide configuration is persisted and retrievable from SQLite
- no API keys are stored in SQLite
- tests pass in CI for the full phase-1 command set
