# Variables
APP_NAME := ytm
ENTRY := ./cmd/ytm-tui
BIN_DIR := bin
OUTPUT := $(BIN_DIR)/$(APP_NAME)
AIR_BIN := $(shell go env GOPATH)/bin/air
GOLANGCI_LINT_BIN := $(shell go env GOPATH)/bin/golangci-lint

PREFIX ?= $(HOME)/.local
SHARE_DIR := $(PREFIX)/share/go-ytm

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
CGO_ENABLED ?= 0
GO := go

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.date=$(DATE)
BUILD_FLAGS := -trimpath -ldflags="$(LDFLAGS)"

.PHONY: all build run test lint fmt tidy clean air api-stop install uninstall

all: tidy fmt lint test build

build:
	@echo "==> Building $(APP_NAME) $(VERSION) for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build $(BUILD_FLAGS) -o $(OUTPUT) $(ENTRY)

# Dev run: Go owns ytm-api; YTM_DEV points API home at ./ytm-api
run: build
	@chmod +x scripts/run.sh
	@./scripts/run.sh ./$(OUTPUT)

api-stop:
	@if [ -f "$${XDG_STATE_HOME:-$$HOME/.local/state}/go-ytm/ytm-api.pid" ]; then \
		pid=$$(cat "$${XDG_STATE_HOME:-$$HOME/.local/state}/go-ytm/ytm-api.pid"); \
		kill $$pid 2>/dev/null || true; \
		rm -f "$${XDG_STATE_HOME:-$$HOME/.local/state}/go-ytm/ytm-api.pid"; \
		echo "==> Stopped ytm-api (pid $$pid)"; \
	elif [ -f tmp/ytm-api.pid ]; then \
		pid=$$(cat tmp/ytm-api.pid); \
		kill $$pid 2>/dev/null || true; \
		rm -f tmp/ytm-api.pid; \
		echo "==> Stopped ytm-api (pid $$pid)"; \
	else \
		echo "==> No ytm-api pidfile found"; \
	fi

install: build
	@echo "==> Installing to $(PREFIX)..."
	@mkdir -p $(PREFIX)/bin $(SHARE_DIR)
	install -m 755 $(OUTPUT) $(PREFIX)/bin/$(APP_NAME)
	@rm -rf $(SHARE_DIR)/ytm-api
	@mkdir -p $(SHARE_DIR)/ytm-api
	@tar -C ytm-api \
		--exclude='venv' \
		--exclude='__pycache__' \
		--exclude='*.pyc' \
		--exclude='test_*.py' \
		-cf - . | tar -C $(SHARE_DIR)/ytm-api -xf -
	@echo "==> Installed $(PREFIX)/bin/$(APP_NAME)"
	@echo "==> API at $(SHARE_DIR)/ytm-api"
	@echo "    Run: ytm doctor && ytm"

uninstall:
	@echo "==> Removing $(PREFIX)/bin/$(APP_NAME) and $(SHARE_DIR)..."
	rm -f $(PREFIX)/bin/$(APP_NAME)
	rm -rf $(SHARE_DIR)

test:
	@echo "==> Running tests..."
	CGO_ENABLED=$(CGO_ENABLED) $(GO) test ./...

lint:
	@echo "==> Running golangci-lint..."
	@if [ ! -f $(GOLANGCI_LINT_BIN) ]; then \
		echo "Installing golangci-lint via go install..."; \
		CGO_ENABLED=0 $(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi
	$(GOLANGCI_LINT_BIN) run --timeout 5m ./...

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
	rm -f ytm ytm-tui
	rm -rf tmp/

air:
	@echo "==> Running air for live reloading..."
	@if [ ! -f $(AIR_BIN) ]; then \
		echo "Installing air for live reloading..."; \
		$(GO) install github.com/air-verse/air@latest; \
	fi
	$(AIR_BIN) -c .air.toml
