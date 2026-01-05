#!/bin/bash
# Simple retry loop for demo E2E tests against existing environment
# Usage: ./scripts/test-loop.sh [max_retries] [test_filter]
#
# Examples:
#   ./scripts/test-loop.sh 10                    # Run all demo tests, max 10 retries
#   ./scripts/test-loop.sh 20 demo-env.spec.ts   # Run specific test file
#   ./scripts/test-loop.sh 5 "banking"           # Run tests matching "banking"
#
# Environment variables:
#   BASE_URL          - Frontend URL (default: http://localhost:5173)
#   PUBLIC_API_URL    - Backend URL (default: http://localhost:8080)
#   DEMO_RESET_SECRET - Secret for demo reset API (default: test-demo-secret)

set -e

MAX_RETRIES=${1:-50}
TEST_FILTER=${2:-""}
RETRY_DELAY=5
ATTEMPT=0
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FRONTEND_DIR="$PROJECT_ROOT/frontend"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
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

cd "$FRONTEND_DIR"

# Set default environment variables
export CI=${CI:-true}
export BASE_URL=${BASE_URL:-http://localhost:5173}
export PUBLIC_API_URL=${PUBLIC_API_URL:-http://localhost:8080}
export DEMO_RESET_SECRET=${DEMO_RESET_SECRET:-test-demo-secret}

log_info "Test configuration:"
echo -e "  ${CYAN}BASE_URL:${NC}          $BASE_URL"
echo -e "  ${CYAN}PUBLIC_API_URL:${NC}    $PUBLIC_API_URL"
echo -e "  ${CYAN}MAX_RETRIES:${NC}       $MAX_RETRIES"
echo -e "  ${CYAN}TEST_FILTER:${NC}       ${TEST_FILTER:-'(all tests)'}"
echo ""

# Build the test command
TEST_CMD="npx playwright test --config=playwright.demo.config.ts --project=demo-chromium"
if [ -n "$TEST_FILTER" ]; then
    TEST_CMD="$TEST_CMD $TEST_FILTER"
fi

# Run tests in a loop
while [ $ATTEMPT -lt $MAX_RETRIES ]; do
    ATTEMPT=$((ATTEMPT + 1))

    echo ""
    echo "========================================"
    echo -e "${BLUE}Attempt $ATTEMPT of $MAX_RETRIES${NC}"
    echo "========================================"
    echo ""

    if $TEST_CMD; then
        echo ""
        log_success "============================================"
        log_success "ALL TESTS PASSED on attempt $ATTEMPT!"
        log_success "============================================"
        exit 0
    else
        FAILED_TESTS=$?
        log_warn "Tests failed on attempt $ATTEMPT (exit code: $FAILED_TESTS)"

        if [ $ATTEMPT -lt $MAX_RETRIES ]; then
            # Re-seed demo data for next attempt
            if [ -n "$DEMO_RESET_SECRET" ]; then
                log_info "Re-seeding demo data..."
                curl -s -X POST "$PUBLIC_API_URL/api/demo/reset" \
                    -H "Content-Type: application/json" \
                    -H "X-Demo-Secret: $DEMO_RESET_SECRET" > /dev/null || true
            fi

            log_info "Waiting $RETRY_DELAY seconds before retry..."
            sleep $RETRY_DELAY
        fi
    fi
done

echo ""
log_error "============================================"
log_error "FAILED: Tests did not pass after $MAX_RETRIES attempts"
log_error "============================================"
exit 1
