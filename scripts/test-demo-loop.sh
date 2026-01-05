#!/bin/bash
# Retry loop for demo E2E tests - runs until all tests pass
# Usage: ./scripts/test-demo-loop.sh [max_retries]

set -e

MAX_RETRIES=${1:-50}
RETRY_DELAY=10
ATTEMPT=0
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FRONTEND_DIR="$PROJECT_ROOT/frontend"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

cleanup() {
    log_info "Cleaning up..."
    if [ -n "$API_PID" ]; then
        kill $API_PID 2>/dev/null || true
    fi
    cd "$PROJECT_ROOT"
    docker-compose down 2>/dev/null || true
}

trap cleanup EXIT

# Check if docker is running
if ! docker info > /dev/null 2>&1; then
    log_error "Docker is not running. Please start Docker first."
    exit 1
fi

cd "$PROJECT_ROOT"

log_info "Starting local demo test environment..."

# Start PostgreSQL
log_info "Starting PostgreSQL via docker-compose..."
docker-compose up -d db
sleep 5

# Wait for PostgreSQL to be ready
log_info "Waiting for PostgreSQL to be healthy..."
for i in {1..30}; do
    if docker-compose exec -T db pg_isready -U openaccounting -d openaccounting > /dev/null 2>&1; then
        log_success "PostgreSQL is ready"
        break
    fi
    if [ $i -eq 30 ]; then
        log_error "PostgreSQL failed to start"
        exit 1
    fi
    sleep 1
done

# Build backend
log_info "Building backend..."
go build -o bin/api ./cmd/api
go build -o bin/migrate ./cmd/migrate

# Run migrations
log_info "Running migrations..."
./bin/migrate -db "postgres://openaccounting:openaccounting@localhost:5432/openaccounting?sslmode=disable"

# Start backend in demo mode
log_info "Starting backend in demo mode..."
export DATABASE_URL="postgres://openaccounting:openaccounting@localhost:5432/openaccounting?sslmode=disable"
export JWT_SECRET="test-secret-key-for-local"
export PORT=8080
export DEMO_MODE=true
export DEMO_RESET_SECRET="test-demo-secret"
export ALLOWED_ORIGINS="http://localhost:5173,http://localhost:3000"

./bin/api &
API_PID=$!
sleep 5

# Verify backend is running
if ! curl -s http://localhost:8080/health > /dev/null; then
    log_error "Backend failed to start"
    exit 1
fi
log_success "Backend is running"

# Seed demo data
log_info "Seeding demo data via API..."
response=$(curl -s -w "\n%{http_code}" -X POST http://localhost:8080/api/demo/reset \
    -H "Content-Type: application/json" \
    -H "X-Demo-Secret: test-demo-secret")
http_code=$(echo "$response" | tail -1)
body=$(echo "$response" | head -n -1)

if [ "$http_code" != "200" ]; then
    log_error "Demo data seeding failed with status $http_code: $body"
    exit 1
fi
log_success "Demo data seeded successfully"

# Install frontend dependencies if needed
cd "$FRONTEND_DIR"
if [ ! -d "node_modules" ]; then
    log_info "Installing frontend dependencies..."
    npm ci
fi

# Install Playwright browsers if needed
if ! npx playwright --version > /dev/null 2>&1; then
    log_info "Installing Playwright browsers..."
    npx playwright install chromium
fi

# Run tests in a loop
log_info "Starting test loop (max $MAX_RETRIES attempts)..."
echo ""

while [ $ATTEMPT -lt $MAX_RETRIES ]; do
    ATTEMPT=$((ATTEMPT + 1))

    echo ""
    echo "========================================"
    echo -e "${BLUE}Attempt $ATTEMPT of $MAX_RETRIES${NC}"
    echo "========================================"
    echo ""

    # Run tests
    export CI=true
    export BASE_URL=http://localhost:5173
    export PUBLIC_API_URL=http://localhost:8080
    export DEMO_RESET_SECRET=test-demo-secret

    # Start dev server in background for this attempt
    npm run dev &
    DEV_PID=$!
    sleep 5

    if npx playwright test --config=playwright.demo.config.ts --project=demo-chromium; then
        kill $DEV_PID 2>/dev/null || true
        echo ""
        log_success "============================================"
        log_success "ALL TESTS PASSED on attempt $ATTEMPT!"
        log_success "============================================"
        exit 0
    else
        kill $DEV_PID 2>/dev/null || true
        FAILED_TESTS=$?
        log_warn "Tests failed on attempt $ATTEMPT (exit code: $FAILED_TESTS)"

        if [ $ATTEMPT -lt $MAX_RETRIES ]; then
            log_info "Waiting $RETRY_DELAY seconds before retry..."

            # Re-seed demo data for next attempt
            log_info "Re-seeding demo data..."
            curl -s -X POST http://localhost:8080/api/demo/reset \
                -H "Content-Type: application/json" \
                -H "X-Demo-Secret: test-demo-secret" > /dev/null

            sleep $RETRY_DELAY
        fi
    fi
done

echo ""
log_error "============================================"
log_error "FAILED: Tests did not pass after $MAX_RETRIES attempts"
log_error "============================================"
exit 1
