#!/bin/bash
# Ralph Wiggum Loop - View Verification & Implementation
#
# Usage: ./scripts/ralph-loop-views.sh [--status-only] [--max-iterations N]
#
# This script iterates through all views, checking for:
# 1. Demo data availability
# 2. E2E test existence
# 3. Test passing status
#
# For views missing any of these, it generates a plan and attempts fixes.

set -e

MAX_ITERATIONS=${2:-50}
STATUS_ONLY=false
GENERATE_PROMPT=false
TARGET_ROUTE=""
COMPLETION_MARKER="RALPH_LOOP_COMPLETE"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --status-only)
            STATUS_ONLY=true
            shift
            ;;
        --max-iterations)
            MAX_ITERATIONS="$2"
            shift 2
            ;;
        --generate-prompt)
            GENERATE_PROMPT=true
            TARGET_ROUTE="$2"
            shift 2
            ;;
        *)
            shift
            ;;
    esac
done

# Views that need verification (route:name format)
VIEWS=(
    "/dashboard:Dashboard"
    "/accounts:Chart of Accounts"
    "/journal:Journal Entries"
    "/contacts:Contacts"
    "/invoices:Invoices"
    "/invoices/reminders:Payment Reminders"
    "/quotes:Quotes"
    "/orders:Orders"
    "/payments:Payments"
    "/payments/cash:Cash Payments"
    "/recurring:Recurring Invoices"
    "/employees:Employees"
    "/employees/absences:Absences"
    "/payroll:Payroll Runs"
    "/payroll/calculator:Salary Calculator"
    "/banking:Bank Accounts"
    "/banking/import:Bank Import"
    "/assets:Fixed Assets"
    "/inventory:Inventory"
    "/reports:Reports"
    "/reports/balance-confirmations:Balance Confirmations"
    "/reports/cash-flow:Cash Flow"
    "/tax:Tax Overview"
    "/vat-returns:VAT Returns"
    "/tsd:TSD Declarations"
    "/settings:Settings"
    "/settings/company:Company Settings"
    "/settings/email:Email Settings"
    "/settings/plugins:Plugins"
    "/settings/cost-centers:Cost Centers"
)

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

check_e2e_test() {
    local route=$1
    # Convert route to test file name pattern
    local name=$(echo "$route" | sed 's/^\///' | sed 's/\//-/g')

    # Check for exact match test file
    if [[ -f "$PROJECT_ROOT/frontend/e2e/demo/${name}.spec.ts" ]]; then
        return 0
    fi

    # Check for partial match in test files
    for file in "$PROJECT_ROOT"/frontend/e2e/demo/*.spec.ts; do
        if grep -q "navigateTo.*'$route'" "$file" 2>/dev/null; then
            return 0
        fi
    done

    # Check data-verification.spec.ts specifically
    if grep -q "navigateTo.*'$route'" "$PROJECT_ROOT/frontend/e2e/demo/data-verification.spec.ts" 2>/dev/null; then
        return 0
    fi

    return 1
}

check_demo_data() {
    local route=$1
    local handlers_file="$PROJECT_ROOT/cmd/api/handlers.go"

    # Map routes to required table inserts
    case "$route" in
        "/dashboard") return 0 ;;  # Uses aggregate data from other tables
        "/accounts") grep -q "INSERT INTO tenant_acme.accounts" "$handlers_file" && return 0 ;;
        "/journal") grep -q "INSERT INTO tenant_acme.journal_entries" "$handlers_file" && return 0 ;;
        "/contacts") grep -q "INSERT INTO tenant_acme.contacts" "$handlers_file" && return 0 ;;
        "/invoices") grep -q "INSERT INTO tenant_acme.invoices" "$handlers_file" && return 0 ;;
        "/invoices/reminders") grep -q "INSERT INTO tenant_acme.invoices" "$handlers_file" && return 0 ;;
        "/quotes") grep -q "INSERT INTO tenant_acme.quotes" "$handlers_file" && return 0 ;;
        "/orders") grep -q "INSERT INTO tenant_acme.orders" "$handlers_file" && return 0 ;;
        "/payments") grep -q "INSERT INTO tenant_acme.payments" "$handlers_file" && return 0 ;;
        "/payments/cash") grep -q "INSERT INTO tenant_acme.payments" "$handlers_file" && return 0 ;;
        "/recurring") grep -q "INSERT INTO tenant_acme.recurring_invoices" "$handlers_file" && return 0 ;;
        "/employees") grep -q "INSERT INTO tenant_acme.employees" "$handlers_file" && return 0 ;;
        "/employees/absences") grep -q "INSERT INTO tenant_acme.leave_records" "$handlers_file" && return 0 ;;
        "/payroll") grep -q "INSERT INTO tenant_acme.payroll_runs" "$handlers_file" && return 0 ;;
        "/payroll/calculator") return 0 ;;  # Calculator doesn't need seeded data
        "/banking") grep -q "INSERT INTO tenant_acme.bank_accounts" "$handlers_file" && return 0 ;;
        "/banking/import") grep -q "INSERT INTO tenant_acme.bank_accounts" "$handlers_file" && return 0 ;;
        "/assets") grep -q "INSERT INTO tenant_acme.fixed_assets" "$handlers_file" && return 0 ;;
        "/inventory") grep -q "INSERT INTO tenant_acme.inventory_items" "$handlers_file" && return 0 ;;
        "/reports") return 0 ;;  # Reports use aggregate data
        "/reports/balance-confirmations") return 0 ;;
        "/reports/cash-flow") return 0 ;;
        "/tax") return 0 ;;  # Tax overview uses aggregate data
        "/vat-returns") return 0 ;;  # Uses invoice VAT data
        "/tsd") grep -q "INSERT INTO tenant_acme.tsd_declarations" "$handlers_file" && return 0 ;;
        "/settings") return 0 ;;  # Settings don't need demo data
        "/settings/company") return 0 ;;
        "/settings/email") return 0 ;;
        "/settings/plugins") return 0 ;;
        "/settings/cost-centers") return 0 ;;
        "/login") return 0 ;;  # Login doesn't need demo data
        "/admin/plugins") return 0 ;;  # Admin plugins don't need demo data
        *) return 1 ;;
    esac
    return 1
}

generate_prompt() {
    local route=$1
    local name=$2

    cat << EOF
# Ralph Wiggum Loop - Fix View: $name

## Context
The **$name** view at route \`$route\` needs to be made functional with demo data.

## Current State
$(check_demo_data "$route" && echo "- Demo data: YES" || echo "- Demo data: NO")
$(check_e2e_test "$route" && echo "- E2E test: YES" || echo "- E2E test: NO")

## Tasks

### 1. Add Demo Data (if missing)
Location: \`cmd/api/handlers.go\` in the demo reset SQL section

Add INSERT statements following this pattern:
\`\`\`sql
INSERT INTO tenant_acme.[table_name] (id, tenant_id, ..., created_by) VALUES
('[uuid]'::uuid, 'b0000000-0000-0000-0000-000000000001'::uuid, ..., 'a0000000-0000-0000-0000-000000000001'::uuid);
\`\`\`

### 2. Create E2E Test (if missing)
Location: \`frontend/e2e/demo/$(echo "$route" | sed 's/^\///' | sed 's/\//-/g').spec.ts\`

Follow existing patterns from quotes.spec.ts, orders.spec.ts, or fixed-assets.spec.ts.

### 3. Run and Verify
\`\`\`bash
cd frontend
bun run test:e2e:demo -- --grep "$(echo "$name" | tr ' ' '-' | tr '[:upper:]' '[:lower:]')"
\`\`\`

## Completion Signal
When complete, output: RALPH_VIEW_COMPLETE: $route
EOF
}

# Handle --generate-prompt
if $GENERATE_PROMPT && [[ -n "$TARGET_ROUTE" ]]; then
    for view_entry in "${VIEWS[@]}"; do
        IFS=':' read -r route name <<< "$view_entry"
        if [[ "$route" == "$TARGET_ROUTE" ]]; then
            generate_prompt "$route" "$name"
            exit 0
        fi
    done
    echo "Route not found: $TARGET_ROUTE" >&2
    exit 1
fi

echo "=========================================="
echo "  RALPH WIGGUM LOOP - View Verification"
echo "=========================================="
echo ""
echo "\"Me fail tests? That's unpossible!\""
echo ""

# Status check
NEEDS_WORK=()
DONE_COUNT=0
TOTAL=${#VIEWS[@]}

for view_entry in "${VIEWS[@]}"; do
    IFS=':' read -r route name <<< "$view_entry"

    has_data=false
    has_test=false

    if check_demo_data "$route"; then
        has_data=true
    fi

    if check_e2e_test "$route"; then
        has_test=true
    fi

    if $has_data && $has_test; then
        echo -e "${GREEN}[DONE]${NC} $name ($route)"
        ((DONE_COUNT++))
    else
        status=""
        [[ "$has_data" == "false" ]] && status+="${RED}needs-data${NC} "
        [[ "$has_test" == "false" ]] && status+="${YELLOW}needs-test${NC}"
        echo -e "${BLUE}[TODO]${NC} $name ($route) - $status"
        NEEDS_WORK+=("$view_entry")
    fi
done

echo ""
echo "=========================================="
echo -e "  Status: ${GREEN}$DONE_COUNT${NC} / $TOTAL views complete"
echo "=========================================="

if [[ ${#NEEDS_WORK[@]} -eq 0 ]]; then
    echo ""
    echo -e "${GREEN}$COMPLETION_MARKER${NC}"
    echo "All views have demo data and E2E tests!"
    exit 0
fi

if $STATUS_ONLY; then
    echo ""
    echo "Views needing work:"
    for view in "${NEEDS_WORK[@]}"; do
        IFS=':' read -r route name <<< "$view"
        echo "  - $name ($route)"
    done
    echo ""
    echo "To fix a specific view, run:"
    echo "  ./scripts/ralph-loop-views.sh --generate-prompt /route | claude"
    exit 0
fi

# Implementation loop
echo ""
echo "Starting Ralph Loop..."
echo "Max iterations: $MAX_ITERATIONS"
echo ""

ITERATION=0
while [[ $ITERATION -lt $MAX_ITERATIONS && ${#NEEDS_WORK[@]} -gt 0 ]]; do
    ((ITERATION++))
    echo ""
    echo "=========================================="
    echo "  Iteration $ITERATION / $MAX_ITERATIONS"
    echo "=========================================="

    # Pick first view that needs work
    view_entry="${NEEDS_WORK[0]}"
    IFS=':' read -r route name <<< "$view_entry"

    echo "Working on: $name ($route)"
    echo ""

    # Generate and display prompt
    echo "--- PROMPT FOR CLAUDE ---"
    generate_prompt "$route" "$name"
    echo "--- END PROMPT ---"
    echo ""

    echo "To fix this view, copy the prompt above to Claude."
    echo "Or pipe directly: ./scripts/ralph-loop-views.sh --generate-prompt $route | claude"
    echo ""
    echo "Press Enter when fixed, or Ctrl+C to exit..."
    read -r

    # Re-check status after potential fix
    if check_demo_data "$route" && check_e2e_test "$route"; then
        echo -e "${GREEN}[FIXED]${NC} $name"
        NEEDS_WORK=("${NEEDS_WORK[@]:1}")
        ((DONE_COUNT++))
    else
        echo -e "${YELLOW}[STILL NEEDS WORK]${NC} $name"
    fi
done

if [[ ${#NEEDS_WORK[@]} -eq 0 ]]; then
    echo ""
    echo -e "${GREEN}$COMPLETION_MARKER${NC}"
    echo "All views complete!"
    exit 0
else
    echo ""
    echo -e "${RED}Loop ended. ${#NEEDS_WORK[@]} views still need work.${NC}"
    exit 1
fi
