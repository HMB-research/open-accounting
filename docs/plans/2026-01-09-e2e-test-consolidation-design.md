# E2E Test Consolidation Design

> **Date:** 2026-01-09
> **Status:** Implemented

## Problem Statement

The E2E test suite had two separate configurations causing reliability issues:

1. **Regular E2E tests** (`playwright.config.ts`) - Used `test@example.com` without tenant data seeding
2. **Demo E2E tests** (`playwright.demo.config.ts`) - Used demo users with proper data seeding

This caused:
- Regular E2E tests failing because the test user had no data (pages showed "Loading..." forever)
- Duplicated test files testing the same features differently
- Maintenance burden with two configurations
- CI failures in the `e2e` job while `e2e-demo-local` passed

## Root Cause Analysis

| Issue | Cause | Impact |
|-------|-------|--------|
| Missing `testInfo` parameter | Demo tests didn't pass `testInfo` to `navigateTo()` | No tenant ID in URL, data not loaded |
| Nil slice JSON serialization | Go repositories returned `null` instead of `[]` | Frontend showed "Loading..." on empty results |
| No data for test user | `test@example.com` had no seeded tenant data | All data-dependent tests failed |

## Solution: Consolidate to Demo Configuration

### Decision

All E2E tests now use the demo configuration (`playwright.demo.config.ts`) as the single source of truth.

### Rationale

1. **Demo tests have working data seeding** - The demo reset endpoint seeds consistent test data
2. **Reduced maintenance burden** - One config, one data setup approach
3. **Production-like testing** - Demo environment mirrors production behavior
4. **Parallel execution** - 3 workers with isolated demo users (demo1-demo4)

## Changes Made

### 1. CI Workflow (`.github/workflows/ci.yml`)

- Removed the old `e2e` job that used the regular config
- Renamed `e2e-demo-local` to `e2e` (now the primary E2E job)
- Updated artifact naming for consistency

### 2. Test Files

**Deleted duplicate tests** (had demo equivalents):
- `frontend/e2e/contacts.spec.ts`
- `frontend/e2e/dashboard.spec.ts`
- `frontend/e2e/invoices.spec.ts`
- `frontend/e2e/recurring.spec.ts`
- `frontend/e2e/reports.spec.ts`
- `frontend/e2e/mobile.spec.ts`

**Created new demo test:**
- `frontend/e2e/demo/mobile.spec.ts` - Mobile viewport tests with demo auth

**Kept as-is:**
- `frontend/e2e/auth.spec.ts` - Tests login page (no auth required)

### 3. Playwright Configuration

Updated `playwright.demo.config.ts`:
- Added `auth.spec.ts` to `testMatch` pattern

### 4. Package.json Scripts

Simplified scripts to use demo config as default:
```json
{
  "test:e2e": "playwright test --config=playwright.demo.config.ts",
  "test:e2e:ui": "playwright test --config=playwright.demo.config.ts --ui",
  "test:e2e:debug": "playwright test --config=playwright.demo.config.ts --debug",
  "test:e2e:headed": "playwright test --config=playwright.demo.config.ts --headed"
}
```

### 5. Cleanup

**Files to be removed:**
- `frontend/e2e/auth.setup.ts` - No longer needed (demo tests handle auth)
- `frontend/playwright.config.ts` - Deprecated (demo config is now primary)

## Test Architecture After Changes

```
frontend/e2e/
├── demo/                    # All demo tests (with data seeding)
│   ├── accounts.spec.ts
│   ├── contacts.spec.ts
│   ├── dashboard.spec.ts
│   ├── invoices.spec.ts
│   ├── mobile.spec.ts      # NEW: Mobile viewport tests
│   ├── payments.spec.ts
│   └── ... (30+ test files)
├── auth.spec.ts            # Login page tests (no auth needed)
├── demo-env.spec.ts        # Environment verification
└── demo-all-views.spec.ts  # View completeness tests
```

## Running E2E Tests

### Local Development

```bash
# Start backend and database
docker compose up -d

# Seed demo data
curl -X POST http://localhost:8080/api/demo/reset \
  -H "X-Demo-Secret: your-secret"

# Run all E2E tests
cd frontend
npm run test:e2e

# Run with UI for debugging
npm run test:e2e:ui

# Run specific test file
npx playwright test --config=playwright.demo.config.ts e2e/demo/invoices.spec.ts
```

### CI/CD

E2E tests run automatically on:
- Push to `main` or `develop`
- Pull requests to `main`

The CI workflow:
1. Starts PostgreSQL service
2. Builds and starts the API server
3. Seeds demo data via `/api/demo/reset`
4. Runs Playwright tests with 3 parallel workers

## Success Criteria

- [x] Single E2E job in CI (`e2e` using demo config)
- [x] All tests use demo data seeding
- [x] No duplicate test files
- [x] Mobile viewport tests work with demo auth
- [x] Auth tests (login page) included in test suite
- [x] CI passes consistently

## Future Considerations

1. **Visual regression testing** - Add screenshot comparison for UI consistency
2. **API contract testing** - Verify API responses match expected schemas
3. **Performance benchmarks** - Track page load times in E2E tests
