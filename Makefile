.PHONY: build clean test test-coverage lint fmt vet ci install run version help

BINARY_NAME := wsh
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"
GO := go
GOFILES := $(shell find . -name '*.go' -type f)
GOENV_GOBIN := $(shell $(GO) env GOBIN)
GOENV_GOPATH := $(shell $(GO) env GOPATH)
INSTALL_DIR := $(if $(GOENV_GOBIN),$(GOENV_GOBIN),$(GOENV_GOPATH)/bin)

build:
	@echo "Building $(BINARY_NAME)..."
	$(GO) build $(LDFLAGS) -o bin/$(BINARY_NAME) .
	@echo "Built bin/$(BINARY_NAME)"

clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f *.db
	$(GO) clean

test:
	$(GO) test ./... -count=1

test-coverage:
	$(GO) test ./... -coverprofile=coverage.out -count=1
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

test-race:
	$(GO) test ./... -race -count=1

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

fmt:
	gofmt -w .

vet:
	$(GO) vet ./...

fmt-check:
	@files=$$(gofmt -l .); \
	if [ -n "$$files" ]; then \
		echo "Files need formatting:"; \
		echo "$$files"; \
		exit 1; \
	fi

ci: fmt-check vet test build
	@echo "CI checks passed"

install: build
	@echo "Installing $(BINARY_NAME) to $(INSTALL_DIR)..."
	mkdir -p $(INSTALL_DIR)
	cp bin/$(BINARY_NAME) $(INSTALL_DIR)/

run:
	$(GO) run . $(ARGS)

version:
	$(GO) run . version

deps:
	$(GO) mod download
	$(GO) mod tidy

tidy:
	$(GO) mod tidy

docker-build:
	docker build -t $(BINARY_NAME):$(VERSION) .

docker-run:
	docker run --rm $(BINARY_NAME):$(VERSION) version

help:
	@echo "Available targets:"
	@echo "  build         - Build the binary (bin/wsh)"
	@echo "  clean         - Remove build artifacts"
	@echo "  test          - Run all tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  test-race     - Run tests with race detector"
	@echo "  lint          - Run golangci-lint"
	@echo "  fmt           - Format source files"
	@echo "  fmt-check     - Check formatting without modifying"
	@echo "  vet           - Run go vet"
	@echo "  ci            - Run all CI checks (fmt-check, vet, test, build)"
	@echo "  install       - Install binary to GOBIN (or GOPATH/bin)"
	@echo "  run           - Run the CLI with ARGS"
	@echo "  version       - Print version info"
	@echo "  deps          - Download and tidy dependencies"
	@echo "  tidy          - Run go mod tidy"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run    - Run Docker container"
	@echo ""
	@echo "Variables:"
	@echo "  VERSION       - Version string (default: git tag or 0.1.0)"
	@echo "  COMMIT        - Git commit hash (default: git rev-parse)"
	@echo "  DATE          - Build date (default: current UTC)"
