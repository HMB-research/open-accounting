.PHONY: all build run test test-coverage test-backend-coverage test-integration test-integration-coverage test-cli-coverage verify-cli-coverage clean docker-build docker-up docker-down migrate help

# Variables
BINARY_API=api
BINARY_MIGRATE=migrate
GO=go
DOCKER_COMPOSE=docker-compose
COVERAGE_PROFILE ?= coverage.out
CLI_COVERAGE_PROFILE ?= coverage-cli.out

INTEGRATION_PACKAGE_LIST = git grep -l '^//go:build .*integration' -- '*_test.go' | while IFS= read -r file; do dirname "$$file"; done | sort -u | sed 's|^|./|'
INTEGRATION_PACKAGE_SHARD = awk -v shard="$(INTEGRATION_SHARD)" -v shards="$(INTEGRATION_SHARDS)" 'BEGIN { if ((shard == "") != (shards == "")) { print "INTEGRATION_SHARD and INTEGRATION_SHARDS must be set together" > "/dev/stderr"; exit 2 } if (shards != "" && (shard < 1 || shards < 1 || shard > shards)) { print "invalid integration shard " shard "/" shards > "/dev/stderr"; exit 2 } } shards == "" || (((NR - 1) % shards) + 1 == shard) { print }'

# Default target
all: build

# Build all binaries
build:
	@echo "Building..."
	$(GO) build -o bin/$(BINARY_API) ./cmd/api
	$(GO) build -o bin/$(BINARY_MIGRATE) ./cmd/migrate

# Run the API server locally
run:
	$(GO) run ./cmd/api

# Run tests
test:
	$(GO) test -v ./...

# Run tests with coverage
test-coverage:
	$(GO) test -v -coverprofile=$(COVERAGE_PROFILE) ./...
	$(GO) tool cover -html=$(COVERAGE_PROFILE) -o coverage.html

# Run the backend gate once and enforce the operator CLI coverage invariant
# from the same profile. This avoids rerunning cmd/oa after the full test pass.
test-backend-coverage:
	$(GO) test -v -race -coverprofile=$(COVERAGE_PROFILE) ./...
	scripts/verify-cli-coverage.sh $(COVERAGE_PROFILE)

# Run DB-backed integration tests. Packages are discovered from tracked build-tagged tests
# so local and CI runs do not duplicate every unit-only package. Set
# INTEGRATION_SHARD and INTEGRATION_SHARDS to run one shard of the same package set.
test-integration:
	@packages="$$( $(INTEGRATION_PACKAGE_LIST) | $(INTEGRATION_PACKAGE_SHARD) )"; \
	status=$$?; \
	if [ $$status -ne 0 ]; then exit $$status; fi; \
	if [ -z "$$packages" ]; then echo "No integration test packages found"; exit 0; fi; \
	$(GO) test -p 1 -v -race -tags=integration $$packages

test-integration-coverage:
	@packages="$$( $(INTEGRATION_PACKAGE_LIST) | $(INTEGRATION_PACKAGE_SHARD) )"; \
	status=$$?; \
	if [ $$status -ne 0 ]; then exit $$status; fi; \
	if [ -z "$$packages" ]; then echo "No integration test packages found"; exit 0; fi; \
	$(GO) test -p 1 -v -race -tags=integration -coverprofile=coverage-integration.out $$packages

# Verify the operator CLI stays fully covered.
test-cli-coverage:
	$(GO) test ./cmd/oa -coverprofile=$(CLI_COVERAGE_PROFILE) -count=1
	scripts/verify-cli-coverage.sh $(CLI_COVERAGE_PROFILE)

verify-cli-coverage:
	scripts/verify-cli-coverage.sh $(COVERAGE_PROFILE)

# Lint code
lint:
	golangci-lint run

# Format code
fmt:
	$(GO) fmt ./...
	gofumpt -l -w .

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html coverage-cli.out coverage-integration.out

# Docker commands
docker-build:
	$(DOCKER_COMPOSE) build

docker-up:
	$(DOCKER_COMPOSE) up -d

docker-down:
	$(DOCKER_COMPOSE) down

docker-logs:
	$(DOCKER_COMPOSE) logs -f

docker-restart:
	$(DOCKER_COMPOSE) restart

# Database commands
migrate-up:
	$(DOCKER_COMPOSE) run --rm migrate

migrate-down:
	$(DOCKER_COMPOSE) run --rm migrate /app/migrate -db "$$DATABASE_URL" -path /app/migrations -direction down

migrate-create:
	@read -p "Migration name: " name; \
	timestamp=$$(date +%Y%m%d%H%M%S); \
	touch migrations/$${timestamp}_$${name}.up.sql; \
	touch migrations/$${timestamp}_$${name}.down.sql; \
	echo "Created migrations/$${timestamp}_$${name}.up.sql"; \
	echo "Created migrations/$${timestamp}_$${name}.down.sql"

# Development helpers
dev: docker-up
	@echo "Development environment started"
	@echo "API: http://localhost:8080"
	@echo "DB: localhost:5432"

dev-stop:
	$(DOCKER_COMPOSE) down

dev-reset:
	$(DOCKER_COMPOSE) down -v
	$(DOCKER_COMPOSE) up -d
	sleep 5
	$(DOCKER_COMPOSE) run --rm migrate

# Database shell
db-shell:
	$(DOCKER_COMPOSE) exec db psql -U openaccounting -d openaccounting

# Production deployment
deploy-prod:
	$(DOCKER_COMPOSE) -f deploy/docker/docker-compose.prod.yml up -d

deploy-prod-down:
	$(DOCKER_COMPOSE) -f deploy/docker/docker-compose.prod.yml down

# Generate API documentation
swagger:
	@echo "Generating Swagger documentation..."
	GOROOT="$$(go env GOROOT)" ~/go/bin/swag init -g cmd/api/main.go -o docs --parseDependency
	@echo "Swagger docs generated in docs/"

docs: swagger
	@echo "API documentation available at /swagger/"

# Install development tools
install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install mvdan.cc/gofumpt@latest
	go install github.com/swaggo/swag/cmd/swag@latest

# Help
help:
	@echo "Open Accounting - Makefile Commands"
	@echo "===================================="
	@echo ""
	@echo "Build & Run:"
	@echo "  make build          - Build all binaries"
	@echo "  make run            - Run API server locally"
	@echo "  make test           - Run tests"
	@echo "  make test-coverage  - Run tests with coverage report"
	@echo "  make test-backend-coverage - Run backend tests and enforce CLI coverage"
	@echo "  make test-cli-coverage - Enforce 100% cmd/oa coverage"
	@echo "  make lint           - Run linter"
	@echo "  make fmt            - Format code"
	@echo "  make clean          - Clean build artifacts"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-build   - Build Docker images"
	@echo "  make docker-up      - Start Docker containers"
	@echo "  make docker-down    - Stop Docker containers"
	@echo "  make docker-logs    - View Docker logs"
	@echo "  make docker-restart - Restart Docker containers"
	@echo ""
	@echo "Database:"
	@echo "  make migrate-up     - Run migrations up"
	@echo "  make migrate-down   - Rollback last migration"
	@echo "  make migrate-create - Create new migration files"
	@echo "  make db-shell       - Open database shell"
	@echo ""
	@echo "Development:"
	@echo "  make dev            - Start development environment"
	@echo "  make dev-stop       - Stop development environment"
	@echo "  make dev-reset      - Reset development environment"
	@echo "  make install-tools  - Install development tools"
	@echo ""
	@echo "Production:"
	@echo "  make deploy-prod    - Deploy to production"
	@echo "  make deploy-prod-down - Stop production deployment"
