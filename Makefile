# Variables
APP_NAME := ytm-tui
ENTRY := cmd/ytm-tui/main.go
BIN_DIR := bin
OUTPUT := $(BIN_DIR)/$(APP_NAME)
AIR_BIN := $(shell go env GOPATH)/bin/air
GOLANGCI_LINT_BIN := $(shell go env GOPATH)/bin/golangci-lint

# Go environment variables
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
# Require pure Go, no cgo for SQLite cross-compilation (modernc.org/sqlite)
CGO_ENABLED ?= 0
GO := go
YTM_API_PORT ?= 8000

# Build flags
LDFLAGS := -s -w
BUILD_FLAGS := -trimpath -ldflags="$(LDFLAGS)"

.PHONY: all build run test lint fmt tidy clean air api-stop

# Default target
all: tidy fmt lint test build

build:
	@echo "==> Building $(APP_NAME) for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build $(BUILD_FLAGS) -o $(OUTPUT) $(ENTRY)

# Bootstraps ytm-api (Python), verifies mpv, then runs the TUI.
# mpv itself is started by the Go app over IPC — not as a separate Makefile step.
run: build
	@chmod +x scripts/run.sh
	@YTM_API_PORT=$(YTM_API_PORT) ./scripts/run.sh ./$(OUTPUT)

# Stop a leftover ytm-api from a previous run (if any).
api-stop:
	@if [ -f tmp/ytm-api.pid ]; then \
		pid=$$(cat tmp/ytm-api.pid); \
		kill $$pid 2>/dev/null || true; \
		rm -f tmp/ytm-api.pid; \
		echo "==> Stopped ytm-api (pid $$pid)"; \
	else \
		echo "==> No ytm-api pidfile found"; \
	fi

test:
	@echo "==> Running tests..."
	CGO_ENABLED=$(CGO_ENABLED) $(GO) test -v ./...

lint:
	@echo "==> Running golangci-lint..."
	@if [ ! -f $(GOLANGCI_LINT_BIN) ]; then \
		echo "Installing golangci-lint..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin v1.59.1; \
	fi
	$(GOLANGCI_LINT_BIN) run ./...

fmt:
	@echo "==> Formatting code..."
	$(GO) fmt ./...

tidy:
	@echo "==> Tidy and verify dependencies..."
	$(GO) mod tidy
	$(GO) mod verify

clean:
	@echo "==> Cleaning build artifacts..."
	rm -rf $(BIN_DIR)
	rm -f $(APP_NAME)
	rm -rf tmp/

air:
	@echo "==> Running air for live reloading..."
	@if [ ! -f $(AIR_BIN) ]; then \
		echo "Installing air for live reloading..."; \
		$(GO) install github.com/air-verse/air@latest; \
	fi
	$(AIR_BIN) -c .air.toml
