.PHONY: all dev build web-build test test-go test-web clean install-deps lint lint-go lint-web verify fmt help install run migrate

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
BINARY_SERVER=dist/loom-server
SERVER_DIR=./cmd/server

# Colors for output
GREEN=\033[0;32m
NC=\033[0m # No Color

# Version information (injected at build time)
VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILT_AT ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LD_FLAGS = -ldflags="-X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuiltAt=$(BUILT_AT)"

all: web-build build

# Development mode — run Go server + Vite dev server
dev:
	@echo "$(GREEN)Starting Loom development mode...$(NC)"
	@$(GOBUILD) -o $(BINARY_SERVER) $(SERVER_DIR) && ./$(BINARY_SERVER) &
	@cd web && npm run dev

# Build Go server binary (pure Go, no CGO needed — modernc.org/sqlite)
build: $(BINARY_SERVER)

$(BINARY_SERVER):
	@mkdir -p dist
	@echo "$(GREEN)Building loom-server...$(NC)"
	CGO_ENABLED=1 $(GOBUILD) $(LD_FLAGS) -o $(BINARY_SERVER) $(SERVER_DIR)

# Build web frontend
web-build:
	@echo "$(GREEN)Building web frontend...$(NC)"
	@cd web && npm install --silent && npm run build

# Run all tests
test: test-go test-web

# Go backend tests
test-go:
	@echo "$(GREEN)Running Go tests...$(NC)"
	$(GOTEST) -v -race -cover ./...

# React frontend tests
test-web:
	@echo "$(GREEN)Running React tests...$(NC)"
	cd web && npm ci && npm run test -- --run

# Clean build artifacts
clean:
	@echo "$(GREEN)Cleaning...$(NC)"
	rm -rf dist/
	rm -rf web/dist/

# Install all dependencies
install-deps:
	@echo "$(GREEN)Installing Go dependencies...$(NC)"
	$(GOMOD) download
	@echo "$(GREEN)Installing frontend dependencies...$(NC)"
	cd web && npm install

# Format code
fmt:
	$(GOCMD) fmt ./...

# Run all linters
lint: lint-go lint-web

# Go-specific linting
lint-go:
	@echo "$(GREEN)Linting Go code...$(NC)"
	golangci-lint run ./...

# React-specific linting
lint-web:
	@echo "$(GREEN)Linting React code...$(NC)"
	cd web && npm run lint

# Unified verification target
verify: fmt lint test all
	@echo "$(GREEN)All systems go. Loom is ready for commit.$(NC)"

# Run server locally
run:
	$(GOCMD) run $(SERVER_DIR)

# Migrations (run on startup, but explicit target for clarity)
migrate:
	@echo "Migrations run automatically on server startup"

# Installer script (local dev testing)
install:
	bash scripts/install-server.sh

# Show help
help:
	@echo "Loom — Agent-First JIT Kanban Board"
	@echo ""
	@echo " make all        - Build everything (web + server)"
	@echo " make dev        - Start development (server + Vite)"
	@echo " make build      - Build server binary"
	@echo " make web-build  - Build web frontend only"
	@echo " make test       - Run all tests"
	@echo " make test-go    - Run Go tests"
	@echo " make test-web   - Run React tests"
	@echo " make clean      - Clean build artifacts"
	@echo " make install-deps - Install all dependencies"
	@echo " make fmt        - Format Go code"
	@echo " make lint       - Run all linters"
	@echo " make lint-go    - Run Go linter only"
	@echo " make lint-web   - Run React linter only"
	@echo " make verify     - Run fmt, lint, test, and all builds"
	@echo " make run        - Run server locally"
	@echo " make install    - Run installer script locally"
	@echo ""
