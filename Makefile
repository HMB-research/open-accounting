.PHONY: all build run test clean docker-build docker-up docker-down migrate help

# Variables
BINARY_API=api
BINARY_MIGRATE=migrate
GO=go
DOCKER_COMPOSE=docker-compose

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
	$(GO) test -v -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

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
	rm -f coverage.out coverage.html

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
	~/go/bin/swag init -g cmd/api/main.go -o docs --parseDependency
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
