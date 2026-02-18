# Contributing to walkscape-helper

Thank you for your interest in contributing to walkscape-helper! This document provides guidelines and instructions for contributing.

## Development Setup

### Prerequisites

- Go 1.24 or later
- Make (optional, but recommended)

### Getting Started

1. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/walkscape-helper.git
   cd walkscape-helper
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Build the project:
   ```bash
   make build
   # or
   go build -o bin/wsh .
   ```

4. Run tests:
   ```bash
   make test
   # or
   go test ./...
   ```

## Development Workflow

### Before Submitting a PR

1. **Format your code:**
   ```bash
   make fmt
   # or
   gofmt -w .
   ```

2. **Run the linter:**
   ```bash
   make lint
   # or
   golangci-lint run
   ```

3. **Run all tests:**
   ```bash
   make test
   # or
   go test ./... -count=1
   ```

4. **Run the full CI suite:**
   ```bash
   make ci
   ```

### Code Style

- Follow standard Go conventions
- Use `gofmt` for formatting
- Group imports: standard library, internal packages, third-party
- Avoid comments for obvious code; add comments only for non-obvious behavior
- Use early returns over deep nesting

### Testing

- Write tests for new functionality
- Use table-driven tests for multiple cases
- Use `t.Parallel()` for independent tests
- Use `t.TempDir()` for temporary files
- Aim for high test coverage on new code

### Commit Messages

- Use present tense ("Add feature" not "Added feature")
- Keep the first line under 72 characters
- Reference issues and pull requests where appropriate

## Project Structure

```
walkscape-helper/
├── main.go              # Entry point
├── docs/                # Planning and roadmap docs
├── internal/
│   ├── cli/             # Command definitions (cobra)
│   ├── storage/         # Database operations (SQLite)
│   ├── output/          # Error types and output helpers
│   └── logging/         # Structured logging
├── Makefile             # Build automation
├── .golangci.yml        # Linter configuration
└── .github/
    └── workflows/
        └── ci.yml       # CI pipeline
```

## Documentation

- Keep `README.md` concise and command-oriented.
- Put phase plans and future roadmap items under `docs/`.
  - `docs/NEXT_STEPS.md`: near-term phase planning
  - `docs/FUTURE.md`: long-term roadmap

## Adding a New Command

1. Create a new file in `internal/cli/` (e.g., `feature.go`)
2. Define the command with `newFeatureCmd()` returning `*cobra.Command`
3. Use `RunE` for error handling
4. Support both JSON and human output modes
5. Add tests in `internal/cli/cli_test.go`
6. Update `README.md` with usage documentation

## Questions?

Open an issue for bugs, feature requests, or questions.
