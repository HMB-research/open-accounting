# Demo E2E Testing - Ralph Loop Workflow

> "Me fail tests? That's unpossible!" - Ralph Wiggum
>
> The Ralph Loop keeps running tests until they pass. Simple. Persistent. Unstoppable.

This document describes how to run and verify demo data presence across all views using Playwright E2E tests in a persistent loop that doesn't give up until everything works.

## Overview

The **Ralph Loop** is a simple testing pattern:
1. Run tests
2. If tests fail → re-seed demo data → wait → retry
3. Repeat until ALL tests pass
4. Never give up (up to 50 attempts by default)

The demo E2E tests verify that:
1. All demo accounts (demo2, demo3, demo4) are properly seeded
2. Every view in the application displays actual data (not empty states)
3. Demo reset functionality works correctly and is idempotent

## Quick Start

### Prerequisites

1. Backend running with demo mode enabled:
   ```bash
   DEMO_MODE=true DEMO_RESET_SECRET=test-demo-secret ./api
   ```

2. Frontend dev server running:
   ```bash
   cd frontend && npm run dev
   ```

3. Environment variables set:
   ```bash
   export BASE_URL=http://localhost:5173
   export PUBLIC_API_URL=http://localhost:8080
   export DEMO_RESET_SECRET=test-demo-secret
   ```

### Run Data Verification Tests Once

```bash
cd frontend
npm run test:e2e:demo:verify
```

### Run the Ralph Loop (Retry Until Pass)

```bash
cd frontend
npm run test:e2e:demo:loop:verify
```

The Ralph Loop will:
1. Run the data verification tests
2. If tests fail → call `/api/demo/reset` to re-seed data
3. Wait 5 seconds
4. Retry (up to 50 times by default)
5. Exit with success ONLY when ALL tests pass

```
┌─────────────────────────────────────────┐
│           THE RALPH LOOP                │
│                                         │
│    ┌──────────┐                        │
│    │Run Tests │◄────────────┐          │
│    └────┬─────┘             │          │
│         │                   │          │
│         ▼                   │          │
│    ┌──────────┐      ┌──────┴─────┐    │
│    │ Pass?    │──No──►Re-seed Data│    │
│    └────┬─────┘      └────────────┘    │
│         │Yes                            │
│         ▼                               │
│    ┌──────────┐                        │
│    │ SUCCESS! │                        │
│    └──────────┘                        │
└─────────────────────────────────────────┘
```

## Test Structure

### Demo User Assignment

Tests run in parallel with 3 workers, each using a dedicated demo user:

| Worker | Email | Tenant ID |
|--------|-------|-----------|
| 0 | demo2@example.com | b0000000-0000-0000-0002-000000000001 |
| 1 | demo3@example.com | b0000000-0000-0000-0003-000000000001 |
| 2 | demo4@example.com | b0000000-0000-0000-0004-000000000001 |

**Note:** demo1@example.com is reserved for end users (README documentation).

### Test Files

| File | Purpose |
|------|---------|
| `e2e/demo/data-verification.spec.ts` | Strict tests that FAIL if any view is empty |
| `e2e/demo/reset.spec.ts` | Tests demo reset functionality |
| `e2e/demo/*.spec.ts` | Individual view tests |
| `e2e/demo-all-views.spec.ts` | Comprehensive page load tests |

## Data Verification Tests

The `data-verification.spec.ts` file contains strict tests that verify actual data presence:

### Views Tested

| View | Expected Data |
|------|---------------|
| Dashboard | Charts with data, summary cards |
| Accounts | 33+ chart of accounts entries |
| Journal | 4+ journal entries |
| Contacts | 7 contacts (TechStart, Nordic, etc.) |
| Invoices | 9 invoices with INV-* numbers |
| Payments | 4+ payments |
| Employees | 5 employees (Maria, Jaan, Anna, etc.) |
| Payroll | 3 payroll runs (2024-10, 11, 12) |
| Recurring | 3 recurring invoices |
| Banking | 2 bank accounts |
| Reports | Report type options |
| TSD | 3 TSD declarations |

### What Makes a Test Fail

Tests will FAIL if:
- A table shows 0 rows
- Empty state message is visible ("No entries", "Create first")
- Error messages appear
- Expected data identifiers are missing

## Ralph Loop Script

The `scripts/test-loop.sh` script implements the Ralph Loop:

### Usage

```bash
# Run all demo tests with up to 50 retries
./scripts/test-loop.sh 50

# Run specific test file
./scripts/test-loop.sh 20 data-verification

# Run tests matching pattern
./scripts/test-loop.sh 10 "banking"
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `BASE_URL` | http://localhost:5173 | Frontend URL |
| `PUBLIC_API_URL` | http://localhost:8080 | Backend API URL |
| `DEMO_RESET_SECRET` | test-demo-secret | Secret for demo reset API |

### How the Ralph Loop Works

1. Run Playwright tests
2. If ALL pass → Exit with success (exit code 0)
3. If ANY fail:
   - Call `/api/demo/reset` to re-seed demo data
   - Wait 5 seconds
   - Increment attempt counter
   - Go back to step 1
4. If max attempts reached → Exit with failure (exit code 1)

## Troubleshooting

### Tests Fail with "Login failed"

1. Verify backend is running with `DEMO_MODE=true`
2. Check demo users are seeded: `curl localhost:8080/api/demo/status?user=2`
3. Trigger manual reset: `curl -X POST localhost:8080/api/demo/reset -H "X-Demo-Secret: test-demo-secret"`

### Tests Fail with Empty Data

1. Check API status endpoint returns data counts > 0
2. Verify tenant ID is being passed in URL
3. Check browser console for API errors

### Tests Timeout

1. Increase timeout in `playwright.demo.config.ts`
2. Check network connectivity to backend
3. Verify frontend is building/serving correctly

## CI Integration

The CI workflow `e2e-demo-local` job:

1. Starts PostgreSQL
2. Builds and runs backend with `DEMO_MODE=true`
3. Seeds demo data via `/api/demo/reset`
4. Builds frontend
5. Runs demo E2E tests

See `.github/workflows/ci.yml` for the full configuration.

## Adding New Tests

When adding new data verification tests:

1. Add test to `e2e/demo/data-verification.spec.ts`
2. Use strict assertions that FAIL on empty data
3. Include meaningful error messages
4. Update `EXPECTED_DEMO_DATA` in `e2e/demo/api.ts` if counts change

Example:

```typescript
test('New view shows data (not empty)', async ({ page }, testInfo) => {
  await navigateTo(page, '/new-view', testInfo);

  const tableRows = page.locator('table tbody tr');
  const rowCount = await tableRows.count();
  expect(rowCount, 'New view must have rows (expected X)').toBeGreaterThanOrEqual(1);

  // Verify NOT showing empty state
  const emptyState = page.getByText(/no.*data|create.*first/i);
  const isEmpty = await emptyState.isVisible().catch(() => false);
  expect(isEmpty, 'Should NOT show empty state').toBeFalsy();
});
```
