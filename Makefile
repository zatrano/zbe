##
## ZBE — ZATRANO Backend
## Usage: make <target>
##

BINARY    := zbe
CMD_DIR   := ./cmd/server
BUILD_DIR := ./bin
MAIN      := $(CMD_DIR)/main.go

# Go build flags
LDFLAGS   := -ldflags="-s -w -X main.version=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev)"
GOFLAGS   := -trimpath

.PHONY: all build run dev test lint clean migrate seed docker-build docker-run help

all: build  ## Default: build the binary

## ── Build ─────────────────────────────────────────────────────────────────────

build:  ## Build the production binary
	@echo "→ Building $(BINARY)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) $(CMD_DIR)
	@echo "✓ Binary: $(BUILD_DIR)/$(BINARY)"

build-linux:  ## Cross-compile for Linux/amd64
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(GOFLAGS) $(LDFLAGS) \
	    -o $(BUILD_DIR)/$(BINARY)-linux-amd64 $(CMD_DIR)

## ── Run ───────────────────────────────────────────────────────────────────────

run: build  ## Build and run the server
	$(BUILD_DIR)/$(BINARY)

dev:  ## Run with hot reload using air (install: go install github.com/air-verse/air@latest)
	@which air > /dev/null 2>&1 || (echo "air not found. Install: go install github.com/air-verse/air@latest" && exit 1)
	air

## ── Dependencies ──────────────────────────────────────────────────────────────

deps:  ## Download and tidy dependencies
	go mod download
	go mod tidy

## ── Testing ───────────────────────────────────────────────────────────────────

test:  ## Run all tests
	go test ./... -v -race -count=1

test-cover:  ## Run tests with coverage report
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report: coverage.html"

## ── Code quality ──────────────────────────────────────────────────────────────

lint:  ## Run golangci-lint (install: https://golangci-lint.run/usage/install/)
	@which golangci-lint > /dev/null 2>&1 || (echo "golangci-lint not found" && exit 1)
	golangci-lint run ./...

fmt:  ## Format Go source files
	gofmt -w -s .
	@echo "✓ Formatted"

vet:  ## Run go vet
	go vet ./...

## ── Database ──────────────────────────────────────────────────────────────────

migrate:  ## Run database migrations (server auto-runs on start; use this to check)
	@echo "Migrations run automatically on server startup."
	@echo "To run manually, start the server with APP_ENV=development."

db-create:  ## Create the database (requires psql)
	@echo "→ Creating database..."
	psql -h $${DB_HOST:-localhost} -U $${DB_USER:-postgres} \
	    -c "CREATE DATABASE $${DB_NAME:-zatrano};" || true

db-drop:  ## Drop the database (DESTRUCTIVE)
	@echo "⚠️  Dropping database $${DB_NAME:-zatrano}..."
	psql -h $${DB_HOST:-localhost} -U $${DB_USER:-postgres} \
	    -c "DROP DATABASE IF EXISTS $${DB_NAME:-zatrano};"

## ── Docker ────────────────────────────────────────────────────────────────────

docker-build:  ## Build Docker image
	docker build -t zatrano/zbe:latest .

docker-run:  ## Run with Docker Compose
	docker-compose up -d

docker-down:  ## Stop Docker Compose
	docker-compose down

## ── Utilities ─────────────────────────────────────────────────────────────────

clean:  ## Remove build artifacts
	rm -rf $(BUILD_DIR) coverage.out coverage.html
	@echo "✓ Cleaned"

setup: deps  ## First-time setup: copy .env and install dependencies
	@[ -f .env ] || cp .env.example .env && echo "✓ Copied .env.example → .env (edit it!)"
	@echo "✓ Setup complete. Edit .env then run: make run"

help:  ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
	    awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'
