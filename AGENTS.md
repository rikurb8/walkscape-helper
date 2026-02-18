# AGENTS.md

Guidance for autonomous coding agents working in `walkscape-helper`.

## Scope and priority

- This file applies to the whole repository.
- Follow direct user instructions first, then this file, then existing code conventions.
- Keep edits minimal, focused, and consistent with current patterns.
- Do not introduce broad refactors unless explicitly requested.

## Repository snapshot

- Language: Go (`go 1.24.0` in `go.mod`).
- App type: CLI (`cobra` + `viper`) with SQLite (`modernc.org/sqlite`).
- Entry point: `main.go`.
- Core packages: `internal/cli`, `internal/storage`, `internal/output`, `internal/logging`.

## Rule files check (Cursor/Copilot)

- `.cursorrules`: not present.
- `.cursor/rules/`: not present.
- `.github/copilot-instructions.md`: not present.
- If these files are added later, treat them as additional constraints and merge their guidance into this file.

## Build, test, and lint commands

Run commands from repo root: `walkscape-helper`.

### Using Make (recommended)

- `make build` - build binary to `bin/wsh`
- `make test` - run all tests
- `make test-coverage` - run tests with HTML coverage report
- `make test-race` - run tests with race detector
- `make lint` - run golangci-lint
- `make fmt` - format source files
- `make vet` - run go vet
- `make ci` - run all CI checks (fmt-check, vet, test, build)
- `make clean` - remove build artifacts

### Dependency/setup

- `go mod tidy` - ensure module graph is tidy.
- `make deps` - download and tidy dependencies.

### Build

- `go build ./...` - compile all packages.
- `go build -o bin/wsh .` - build CLI binary.
- `go run . version` - quick runtime smoke check.

### Tests

- `go test ./...` - run all tests.
- `go test ./... -count=1` - bypass test cache.

### Run a single test (important)

- Single test in one package:
  - `go test ./internal/cli -run '^TestGuideSetupShowValidateJSON$' -count=1`
- Multiple tests by regex in one package:
  - `go test ./internal/storage -run 'Test(OpenHonorsCancelledContext|InsertCharacterInvalidJSON)$' -count=1`
- Verbose single-test output:
  - `go test ./internal/output -run '^TestExitCode$' -v -count=1`

### Lint/format/static checks

Project uses `golangci-lint` with configuration in `.golangci.yml`.

- `make lint` - run golangci-lint.
- `gofmt -w .` - format source files.
- `go vet ./...` - static analysis.

Recommended pre-PR quality gate:

1. `make ci` (runs fmt-check, vet, test, build)

## Architecture and implementation conventions

### Command layer (`internal/cli`)

- Construct commands with `newXCmd()` helpers returning `*cobra.Command`.
- Use `RunE` and return errors instead of exiting directly.
- Obtain runtime options via `contextFromCommand(cmd)`.
- Support both human and JSON output paths.
- In JSON mode, write machine-readable response objects with `output.WriteJSONSuccess` / `output.WriteJSONError`.
- Prefer validating flags/input early before opening DB connections.

### Output and errors (`internal/output`)

- Use `output.AppError` for expected domain errors.
- Use stable error codes (`validation_error`, `not_found`, `storage_error`, etc.).
- Include actionable `Details` when helpful (usually a `reason` field).
- Convert errors to exit codes through `output.ExitCode(err)`.
- In JSON mode, wrap handled errors with `output.MarkHandled(err)` to avoid duplicate stderr text.

### Storage layer (`internal/storage`)

- Accept `context.Context` in DB APIs.
- Pass `*sql.DB` into storage functions; do not rely on package globals.
- Keep SQL near usage site with raw query strings.
- Handle `sql.ErrNoRows` explicitly and return `(value, found, error)` where appropriate.
- Keep migration steps idempotent (`CREATE TABLE IF NOT EXISTS`, `INSERT OR IGNORE`).

## Code style guidelines

### Formatting and file layout

- Always run `gofmt`; do not hand-format.
- Keep functions focused and prefer early returns over deep nesting.
- Keep package-level vars limited to true globals (version metadata is one existing example).
- Avoid comments for obvious code; add brief comments only for non-obvious behavior.

### Imports

- Group imports in Go standard style:
  1. standard library
  2. internal module imports (e.g., `walkscape-helper/internal/...`)
  3. third-party imports
- Let `gofmt` manage grouping/order.
- Keep blank-import side effects explicit (e.g., sqlite driver registration).

### Types and data structures

- Prefer concrete structs for API/storage models (`GuideConfig`, `Character`).
- Use `map[string]any` only for flexible JSON envelopes/metadata.
- Keep JSON tags snake_case to match existing CLI JSON output.
- Preserve response envelope shape: `{ "ok": bool, "data"|"error": ..., "meta": ... }`.

### Naming

- Exported identifiers: `PascalCase` with clear domain naming.
- Unexported locals/helpers: `camelCase`.
- Command constructors follow `new<Domain><Action>Cmd` pattern.
- Validation helpers use `validateX` naming.
- Avoid cryptic abbreviations unless already established (`cfg`, `ctx`, `db`, `cmd`).

### Error handling

- Never ignore returned errors unless intentionally best-effort.
- Wrap storage/internal failures into `output.NewError("storage_error", ..., details)` at command boundary.
- Use `validation_error` for bad flags/input; include failing field when possible.
- For not-found domain states, return `not_found` rather than generic internal errors.
- Do not panic for recoverable runtime issues.

### Testing conventions

- Use table-driven tests where multiple cases share behavior (`TestExitCode` pattern).
- Use `t.Parallel()` for independent tests and `t.TempDir()` for per-test DB files.
- Prefer black-box checks through `Execute(...)` for CLI flows.
- Assert exit codes and JSON schema content for machine-mode behavior.

## Change safety for agents

- Preserve CLI compatibility unless explicitly asked to change UX/flags/output.
- Preserve JSON field names and exit-code mappings; downstream tooling may depend on them.
- Do not store secrets in SQLite; keep API keys as env-var names only.
- Avoid schema changes unless required; if needed, update migration logic safely and idempotently.

## When adding new functionality

- Add/extend command tests in `internal/cli/cli_test.go` (or nearby package tests).
- Add storage unit tests when changing DB behavior.
- Keep human output concise and JSON output structured.
- Update `README.md` command docs when user-facing CLI behavior changes.

## Quick checklist before finishing

- Formatting applied (`gofmt -w .`).
- Static checks pass (`go vet ./...`).
- Relevant tests pass (at least targeted package; ideally `go test ./... -count=1`).
- Build succeeds (`go build ./...`).
- Any user-visible behavior updates documented in `README.md`.
