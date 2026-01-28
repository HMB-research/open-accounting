# CI Test Reliability Improvements

## Problem Statement

Two CI jobs fail intermittently but are marked as non-blocking (`continue-on-error: true`):

1. **Integration tests** - Asset tests use invalid UUID strings for account IDs
2. **E2E demo local** - Timing issues cause tests to fail before backend is ready

## Solution Overview

### 1. Integration Test Fixtures

Create real GL accounts in test setup instead of using invalid placeholder strings.

**New file:** `internal/testutil/fixtures.go`

```go
type TestAccounts struct {
    AssetAccountID               string
    DepreciationExpenseAccountID string
    AccumulatedDepreciationAcctID string
}

func CreateTestAccounts(t *testing.T, db *pgxpool.Pool, schemaName string) *TestAccounts
```

**Changes:** Update `internal/assets/repository_integration_test.go` to use real account IDs.

### 2. E2E Health Checks

Wait for backend readiness before running tests.

**Addition to:** `frontend/e2e/demo/utils.ts`

```typescript
export async function waitForBackendReady(baseUrl: string, maxWaitMs = 30000): Promise<boolean>
```

### 3. E2E Wait Helpers

Standardize wait patterns for dynamic content.

**Addition to:** `frontend/e2e/demo/utils.ts`

```typescript
export async function waitForTableData(page: Page, minRows?: number, timeout?: number)
export async function waitForModalReady(page: Page, timeout?: number)
export async function waitForFormSubmission(page: Page, timeout?: number)
```

## Implementation Order

1. Create `fixtures.go` with account creation helper
2. Update assets integration tests
3. Verify integration tests pass locally
4. Add E2E wait helpers
5. Update invoice E2E tests
6. Verify E2E tests pass locally
7. Push and verify CI

## Success Criteria

- Integration tests pass without `continue-on-error`
- E2E demo local tests pass reliably
- No increase in test execution time > 10%
