# Walkscape Helper

CLI utility for Walkscape workflows.

Walkscape Wiki: https://wiki.walkscape.app/wiki/WalkScape:_Grind_by_walking!

## Current Status

Phase 1 baseline is implemented:

- basic CLI structure
- SQLite-backed guide configuration
- character import and storage in SQLite
- command-level and integration-style CLI tests for implemented functionality

Wiki ingestion/sync and full guide Q&A are intentionally deferred to later phases.

## Tech

- Golang CLI app (`cobra` + `viper`)
- SQLite for local state and persisted data
- provider-configurable model/embedding/vector setup (implemented incrementally)

## CLI Conventions

- Every command supports `--json` for machine/AI-friendly output.
- Secrets are never stored in SQLite.
- API credentials are env var only (for example: `OPENAI_API_KEY`).
- In `--json` mode, command output is structured JSON on stdout.

## Phase 1 Commands

### Guide Configuration

- `wsh guide setup`
- `wsh guide show`
- `wsh guide validate`

### Character Data

- `wsh character import --from-stdin`
- `wsh character import --raw-json '<json>'`
- `wsh character import --from-file <path>`
- `wsh character list`
- `wsh character show --id <id>`
- `wsh character show --latest`

Character data will usually be pasted in and imported via stdin or `--raw-json`.

## Run Locally

1. Install dependencies:

```bash
go mod tidy
```

2. Run tests:

```bash
go test ./...
```

3. Run the CLI without installing:

```bash
go run . version
```

4. Build a local binary:

```bash
go build -o bin/wsh .
./bin/wsh version
```

By default the SQLite database is created at `./wsh.db`. You can override it with `--db-path`.

## Quick Start

1. Configure guide settings:

```bash
wsh guide setup \
  --llm-provider openai \
  --llm-model gpt-4o-mini \
  --llm-api-key-env OPENAI_API_KEY \
  --embedding-provider openai \
  --embedding-model text-embedding-3-small \
  --vectordb-provider qdrant \
  --vectordb-url http://localhost:6333 \
  --vectordb-collection walkscape
```

2. Import character JSON from stdin:

```bash
cat character.json | wsh character import --from-stdin
```

3. Use machine-readable output:

```bash
wsh character list --json
```

## Later Phases

- local wiki fetch/create/update/sync
- vector indexing and retrieval
- guide `ask` command with retrieval and citations
- docker-compose local stack for full runtime services

See `NEXT_STEPS.md` for concrete phase-1 execution details and `FUTURE.md` for high-level roadmap items.
