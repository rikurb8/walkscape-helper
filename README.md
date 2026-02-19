# Walkscape Helper

`wsh` is a local CLI for Walkscape assistant workflows. It stores guide configuration and imported character snapshots in SQLite, with optional JSON output for automation.

Walkscape Wiki: https://wiki.walkscape.app/wiki/WalkScape:_Grind_by_walking!

## Features

- Local-first: guide and character state are stored in a local SQLite database.
- Automation-friendly: every command supports `--json` with stable response envelopes.
- Safe credential model: only environment variable names are stored, never raw API keys.
- Tested baseline: command and integration tests cover the current command set.

## Command Overview

### `wsh guide`

Manage guide runtime configuration.

- `setup`: save provider/model/persona configuration.
- `show`: display current saved configuration.
- `validate`: check whether saved configuration is complete.

### `wsh character`

Manage imported character JSON snapshots.

- `import`: import one character payload from stdin, file, or raw JSON.
- `list`: list imported characters.
- `show`: show one character by id or the most recently imported one.

### `wsh wiki`

Manage local wiki snapshots and incremental updates.

- `scrape full`: fetch a full snapshot (raw API payloads + normalized NDJSON) into `data/wiki/<snapshot_id>/`.
- `scrape update`: fetch recent changes since the stored cursor and write an update snapshot.
- `clean`: transform normalized wiki revisions into cleaned markdown with YAML frontmatter and domain taxonomy paths.
- `clean validate`: validate cleaned markdown coverage/integrity/quality gates.
- `export`: package a snapshot directory into a deterministic zip archive after checksum verification.
- `export verify`: verify snapshot checksums without creating an archive.
- `status`: show last snapshot metadata and incremental cursor readiness.

Common workflow:

```bash
# initial seed (creates cursor metadata for incremental updates)
./bin/wsh wiki scrape full --out ./data/wiki

# inspect local status
./bin/wsh wiki status --out ./data/wiki

# incremental refresh
./bin/wsh wiki scrape update --out ./data/wiki

# clean snapshot into markdown corpus
./bin/wsh wiki clean --snapshot ./data/wiki/<snapshot_id>

# validate cleaned corpus quality checks
./bin/wsh wiki clean validate --snapshot ./data/wiki/<snapshot_id>

# verify snapshot integrity before sharing
./bin/wsh wiki export verify --snapshot-id <snapshot_id> --wiki-root ./data/wiki

# create deterministic shareable archive
./bin/wsh wiki export --snapshot-id <snapshot_id> --wiki-root ./data/wiki --out ./dist/<snapshot_id>.zip
```

By default, wiki scraping includes namespace `0` and filters to the phase-1 focus categories (skills, activities, recipes, equipment, etc.).

### `wsh version`

Show CLI version/build information.

### `wsh completion`

Generate shell completion scripts for `bash`, `zsh`, `fish`, or `powershell`.

## Run Locally

For a fresh machine (or CI/agent environment), bootstrap dependencies and dev tools first:

```bash
make setup
```

This installs Go module dependencies and pinned developer tools (including `golangci-lint`).

```bash
make ci
make build
./bin/wsh version
```

By default, the SQLite database is created at `./wsh.db`.

## Shell Completion

`wsh` includes a completion command out of the box:

```bash
./bin/wsh completion --help
```

Generate and install completions per shell:

```bash
# bash
./bin/wsh completion bash > ~/.local/share/bash-completion/completions/wsh

# zsh
./bin/wsh completion zsh > ~/.zsh/completions/_wsh

# fish
./bin/wsh completion fish > ~/.config/fish/completions/wsh.fish

# powershell
./bin/wsh completion powershell > ./wsh.ps1
```

You can pass `--no-descriptions` to generate leaner completion scripts.

## LLM-Ready CLI Docs

The repo includes a Cobra doc generator at `internal/tools/docgen`.

Generate markdown command docs:

```bash
make docs-cli
```

Or run the generator directly:

```bash
go run ./internal/tools/docgen -out ./docs/cli -format markdown
```

Supported formats: `markdown`, `man`, `rest`.

Optional front matter for static sites:

```bash
go run ./internal/tools/docgen -out ./docs/cli -format markdown -frontmatter
```

## Docs

- `docs/NEXT_STEPS.md`: phase-1 implementation plan and scope.
- `docs/FUTURE.md`: high-level roadmap and future areas.
- `docs/cli/`: generated command reference docs (via `make docs-cli`).
