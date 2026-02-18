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

### `wsh version`

Show CLI version/build information.

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

## Docs

- `docs/NEXT_STEPS.md`: phase-1 implementation plan and scope.
- `docs/FUTURE.md`: high-level roadmap and future areas.
