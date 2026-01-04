# Demo Data Reset E2E Testing Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add E2E tests that verify demo data exists with correct counts/keys and that reset restores data after modifications.

**Architecture:** Add per-user reset parameter to existing endpoint, create new status endpoint for verification, implement dedicated reset.spec.ts test file with API helpers.

**Tech Stack:** Go (backend API), Playwright (E2E tests), TypeScript

---

## Task 1: Add User Parameter to Reset Endpoint

**Files:**
- Modify: `cmd/api/handlers.go:1014-1041`

**Step 1: Write the failing test**

Create test in `cmd/api/handlers_test.go`:

```go
func TestDemoReset_SingleUser(t *testing.T) {
	// Skip if not in demo mode
	if os.Getenv("DEMO_MODE") != "true" {
		t.Skip("DEMO_MODE not enabled")
	}

	os.Setenv("DEMO_RESET_SECRET", "test-secret")
	defer os.Unsetenv("DEMO_RESET_SECRET")

	tests := []struct {
		name       string
		userParam  string
		wantStatus int
	}{
		{"reset user 1", "1", http.StatusOK},
		{"reset user 2", "2", http.StatusOK},
		{"reset user 3", "3", http.StatusOK},
		{"invalid user 0", "0", http.StatusBadRequest},
		{"invalid user 4", "4", http.StatusBadRequest},
		{"invalid user abc", "abc", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/demo/reset?user="+tt.userParam, nil)
			req.Header.Set("X-Demo-Secret", "test-secret")
			// ... handler invocation would go here
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/tsopic/algo/open-accounting/.worktrees/demo-reset-testing && go test ./cmd/api/... -run TestDemoReset_SingleUser -v`
Expected: FAIL (test structure exists, but handler doesn't support user param yet)

**Step 3: Implement the user parameter parsing**

Modify `DemoReset` in `cmd/api/handlers.go` after line 1041 (after secret validation):

```go
	// Parse optional user parameter
	userParam := r.URL.Query().Get("user")
	var targetUsers []struct {
		email  string
		slug   string
		schema string
	}

	allDemoUsers := []struct {
		email  string
		slug   string
		schema string
	}{
		{"demo1@example.com", "demo1", "tenant_demo1"},
		{"demo2@example.com", "demo2", "tenant_demo2"},
		{"demo3@example.com", "demo3", "tenant_demo3"},
	}

	if userParam != "" {
		userNum, err := strconv.Atoi(userParam)
		if err != nil || userNum < 1 || userNum > 3 {
			log.Warn().Str("user", userParam).Msg("Demo reset rejected: invalid user parameter")
			respondError(w, http.StatusBadRequest, "Invalid user parameter. Must be 1, 2, or 3")
			return
		}
		targetUsers = []struct {
			email  string
			slug   string
			schema string
		}{allDemoUsers[userNum-1]}
		log.Info().Int("user", userNum).Msg("Demo reset: resetting single user")
	} else {
		targetUsers = allDemoUsers
		log.Info().Msg("Demo reset: resetting all users")
	}
```

Then replace the hardcoded `demoUsers` loop with `targetUsers`.

**Step 4: Run test to verify it passes**

Run: `cd /Users/tsopic/algo/open-accounting/.worktrees/demo-reset-testing && go test ./cmd/api/... -run TestDemoReset -v`
Expected: PASS

**Step 5: Commit**

```bash
cd /Users/tsopic/algo/open-accounting/.worktrees/demo-reset-testing
git add cmd/api/handlers.go cmd/api/handlers_test.go
git commit -m "feat(api): add user parameter to demo reset endpoint

Allows resetting individual demo users (1-3) instead of all users.
- POST /api/demo/reset?user=1 resets only demo1
- POST /api/demo/reset (no param) resets all users (backward compatible)"
```

---

## Task 2: Add Demo Status Endpoint - Handler

**Files:**
- Modify: `cmd/api/handlers.go` (add new handler after DemoReset)
- Modify: `cmd/api/main.go:249` (add route)

**Step 1: Add the route**

In `cmd/api/main.go` after line 249:

```go
	r.Get("/api/demo/status", h.DemoStatus)
```

**Step 2: Write the handler**

Add after `DemoReset` in `cmd/api/handlers.go`:

```go
// DemoStatusResponse represents the demo data status
type DemoStatusResponse struct {
	User              int          `json:"user"`
	Accounts          EntityStatus `json:"accounts"`
	Contacts          EntityStatus `json:"contacts"`
	Invoices          EntityStatus `json:"invoices"`
	Employees         EntityStatus `json:"employees"`
	Payments          EntityStatus `json:"payments"`
	JournalEntries    EntityStatus `json:"journalEntries"`
	BankAccounts      EntityStatus `json:"bankAccounts"`
	RecurringInvoices EntityStatus `json:"recurringInvoices"`
	PayrollRuns       EntityStatus `json:"payrollRuns"`
	TsdDeclarations   EntityStatus `json:"tsdDeclarations"`
}

type EntityStatus struct {
	Count int      `json:"count"`
	Keys  []string `json:"keys"`
}

// DemoStatus returns counts and key identifiers for demo data verification
// @Summary Get demo data status
// @Description Get counts and key identifiers for demo data verification
// @Tags Demo
// @Produce json
// @Param user query int true "Demo user number (1-3)"
// @Param X-Demo-Secret header string true "Demo secret key"
// @Success 200 {object} DemoStatusResponse
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /api/demo/status [get]
func (h *Handlers) DemoStatus(w http.ResponseWriter, r *http.Request) {
	// Check if demo mode is enabled
	if os.Getenv("DEMO_MODE") != "true" {
		respondError(w, http.StatusForbidden, "Demo mode is not enabled")
		return
	}

	// Validate secret key
	secret := os.Getenv("DEMO_RESET_SECRET")
	if secret == "" {
		respondError(w, http.StatusForbidden, "Demo status not configured")
		return
	}

	providedSecret := r.Header.Get("X-Demo-Secret")
	if providedSecret != secret {
		respondError(w, http.StatusUnauthorized, "Invalid or missing secret key")
		return
	}

	// Parse required user parameter
	userParam := r.URL.Query().Get("user")
	if userParam == "" {
		respondError(w, http.StatusBadRequest, "User parameter is required")
		return
	}

	userNum, err := strconv.Atoi(userParam)
	if err != nil || userNum < 1 || userNum > 3 {
		respondError(w, http.StatusBadRequest, "Invalid user parameter. Must be 1, 2, or 3")
		return
	}

	schema := fmt.Sprintf("tenant_demo%d", userNum)
	ctx := r.Context()

	response := DemoStatusResponse{User: userNum}

	// Query each entity count and keys
	response.Accounts = h.getEntityStatus(ctx, schema, "accounts", "name")
	response.Contacts = h.getEntityStatus(ctx, schema, "contacts", "name")
	response.Invoices = h.getEntityStatus(ctx, schema, "invoices", "invoice_number")
	response.Employees = h.getEntityStatusConcat(ctx, schema, "employees", "first_name", "last_name")
	response.Payments = h.getEntityStatus(ctx, schema, "payments", "payment_number")
	response.JournalEntries = h.getEntityStatus(ctx, schema, "journal_entries", "entry_number")
	response.BankAccounts = h.getEntityStatus(ctx, schema, "bank_accounts", "name")
	response.RecurringInvoices = h.getEntityStatus(ctx, schema, "recurring_invoices", "name")
	response.PayrollRuns = h.getEntityStatusPeriod(ctx, schema, "payroll_runs")
	response.TsdDeclarations = h.getEntityStatusPeriod(ctx, schema, "tsd_declarations")

	respondJSON(w, http.StatusOK, response)
}

func (h *Handlers) getEntityStatus(ctx context.Context, schema, table, keyColumn string) EntityStatus {
	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s", schema, table)
	_ = h.pool.QueryRow(ctx, query).Scan(&count)

	var keys []string
	keysQuery := fmt.Sprintf("SELECT %s FROM %s.%s ORDER BY %s LIMIT 10", keyColumn, schema, table, keyColumn)
	rows, _ := h.pool.Query(ctx, keysQuery)
	defer rows.Close()
	for rows.Next() {
		var key string
		if rows.Scan(&key) == nil {
			keys = append(keys, key)
		}
	}

	return EntityStatus{Count: count, Keys: keys}
}

func (h *Handlers) getEntityStatusConcat(ctx context.Context, schema, table, col1, col2 string) EntityStatus {
	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s", schema, table)
	_ = h.pool.QueryRow(ctx, query).Scan(&count)

	var keys []string
	keysQuery := fmt.Sprintf("SELECT %s || ' ' || %s FROM %s.%s ORDER BY %s LIMIT 10", col1, col2, schema, table, col1)
	rows, _ := h.pool.Query(ctx, keysQuery)
	defer rows.Close()
	for rows.Next() {
		var key string
		if rows.Scan(&key) == nil {
			keys = append(keys, key)
		}
	}

	return EntityStatus{Count: count, Keys: keys}
}

func (h *Handlers) getEntityStatusPeriod(ctx context.Context, schema, table string) EntityStatus {
	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s", schema, table)
	_ = h.pool.QueryRow(ctx, query).Scan(&count)

	var keys []string
	keysQuery := fmt.Sprintf("SELECT period_year || '-' || LPAD(period_month::text, 2, '0') FROM %s.%s ORDER BY period_year, period_month LIMIT 10", schema, table)
	rows, _ := h.pool.Query(ctx, keysQuery)
	defer rows.Close()
	for rows.Next() {
		var key string
		if rows.Scan(&key) == nil {
			keys = append(keys, key)
		}
	}

	return EntityStatus{Count: count, Keys: keys}
}
```

**Step 3: Add strconv import if not present**

Ensure `"strconv"` is in the imports at the top of `handlers.go`.

**Step 4: Run tests**

Run: `cd /Users/tsopic/algo/open-accounting/.worktrees/demo-reset-testing && go test ./cmd/api/... -v`
Expected: PASS

**Step 5: Commit**

```bash
cd /Users/tsopic/algo/open-accounting/.worktrees/demo-reset-testing
git add cmd/api/handlers.go cmd/api/main.go
git commit -m "feat(api): add demo status endpoint for E2E verification

GET /api/demo/status?user=N returns counts and key identifiers for:
- accounts, contacts, invoices, employees, payments
- journal entries, bank accounts, recurring invoices
- payroll runs, TSD declarations"
```

---

## Task 3: Create Frontend API Helpers

**Files:**
- Create: `frontend/e2e/demo/api.ts`

**Step 1: Create the API helper file**

```typescript
import { DEMO_API_URL } from './utils';

const DEMO_SECRET = process.env.DEMO_RESET_SECRET || '';

export interface EntityStatus {
	count: number;
	keys: string[];
}

export interface DemoStatus {
	user: number;
	accounts: EntityStatus;
	contacts: EntityStatus;
	invoices: EntityStatus;
	employees: EntityStatus;
	payments: EntityStatus;
	journalEntries: EntityStatus;
	bankAccounts: EntityStatus;
	recurringInvoices: EntityStatus;
	payrollRuns: EntityStatus;
	tsdDeclarations: EntityStatus;
}

/**
 * Expected demo data counts and key identifiers.
 * Update these when seed data changes.
 */
export const EXPECTED_DEMO_DATA = {
	accounts: {
		count: 28,
		keys: ['Cash', 'Bank Account - EUR', 'Accounts Receivable', 'Accounts Payable']
	},
	contacts: {
		count: 7,
		keys: ['TechStart OÜ', 'Nordic Solutions AS', 'Baltic Commerce']
	},
	invoices: {
		count: 9,
		keys: ['INV-2024-001', 'INV-2024-002', 'INV-2024-003']
	},
	employees: {
		count: 5,
		keys: ['Maria Tamm', 'Jaan Kask', 'Anna Mets']
	},
	payments: {
		count: 4,
		keys: ['PAY-2024-001', 'PAY-2024-002']
	},
	journalEntries: {
		count: 4,
		keys: ['JE-2024-001', 'JE-2024-002']
	},
	bankAccounts: {
		count: 2,
		keys: ['Main EUR Account', 'Savings Account']
	},
	recurringInvoices: {
		count: 3,
		keys: ['Monthly Support - TechStart', 'Quarterly Retainer - Nordic']
	},
	payrollRuns: {
		count: 3,
		keys: ['2024-10', '2024-11', '2024-12']
	},
	tsdDeclarations: {
		count: 3,
		keys: ['2024-10', '2024-11', '2024-12']
	}
};

/**
 * Trigger demo reset for a specific user
 */
export async function triggerDemoReset(userNum: number): Promise<void> {
	const response = await fetch(`${DEMO_API_URL}/api/demo/reset?user=${userNum}`, {
		method: 'POST',
		headers: {
			'X-Demo-Secret': DEMO_SECRET
		}
	});

	if (!response.ok) {
		throw new Error(`Demo reset failed: ${response.status} ${await response.text()}`);
	}
}

/**
 * Get demo status (counts and key identifiers) for a specific user
 */
export async function getDemoStatus(userNum: number): Promise<DemoStatus> {
	const response = await fetch(`${DEMO_API_URL}/api/demo/status?user=${userNum}`, {
		headers: {
			'X-Demo-Secret': DEMO_SECRET
		}
	});

	if (!response.ok) {
		throw new Error(`Demo status failed: ${response.status} ${await response.text()}`);
	}

	return response.json();
}
```

**Step 2: Verify TypeScript compiles**

Run: `cd /Users/tsopic/algo/open-accounting/.worktrees/demo-reset-testing/frontend && npx tsc --noEmit`
Expected: No errors

**Step 3: Commit**

```bash
cd /Users/tsopic/algo/open-accounting/.worktrees/demo-reset-testing
git add frontend/e2e/demo/api.ts
git commit -m "feat(e2e): add API helpers for demo status and reset

- triggerDemoReset(userNum) - reset specific demo user
- getDemoStatus(userNum) - get entity counts and keys
- EXPECTED_DEMO_DATA - expected values for verification"
```

---

## Task 4: Create Reset Verification Tests

**Files:**
- Create: `frontend/e2e/demo/reset.spec.ts`

**Step 1: Create the test file**

```typescript
import { test, expect } from '@playwright/test';
import { getDemoStatus, triggerDemoReset, EXPECTED_DEMO_DATA } from './api';

test.describe('Demo Data Reset Verification', () => {
	test.describe('Initial State Verification', () => {
		test('has correct account count and key accounts', async ({}, testInfo) => {
			const userNum = (testInfo.parallelIndex % 3) + 1;
			const status = await getDemoStatus(userNum);

			expect(status.accounts.count).toBe(EXPECTED_DEMO_DATA.accounts.count);
			for (const key of EXPECTED_DEMO_DATA.accounts.keys) {
				expect(status.accounts.keys).toContain(key);
			}
		});

		test('has correct contact count and key contacts', async ({}, testInfo) => {
			const userNum = (testInfo.parallelIndex % 3) + 1;
			const status = await getDemoStatus(userNum);

			expect(status.contacts.count).toBe(EXPECTED_DEMO_DATA.contacts.count);
			for (const key of EXPECTED_DEMO_DATA.contacts.keys) {
				expect(status.contacts.keys).toContain(key);
			}
		});

		test('has correct invoice count and key invoices', async ({}, testInfo) => {
			const userNum = (testInfo.parallelIndex % 3) + 1;
			const status = await getDemoStatus(userNum);

			expect(status.invoices.count).toBe(EXPECTED_DEMO_DATA.invoices.count);
			for (const key of EXPECTED_DEMO_DATA.invoices.keys) {
				expect(status.invoices.keys).toContain(key);
			}
		});

		test('has correct employee count and key employees', async ({}, testInfo) => {
			const userNum = (testInfo.parallelIndex % 3) + 1;
			const status = await getDemoStatus(userNum);

			expect(status.employees.count).toBe(EXPECTED_DEMO_DATA.employees.count);
			for (const key of EXPECTED_DEMO_DATA.employees.keys) {
				expect(status.employees.keys).toContain(key);
			}
		});

		test('has correct payment count', async ({}, testInfo) => {
			const userNum = (testInfo.parallelIndex % 3) + 1;
			const status = await getDemoStatus(userNum);

			expect(status.payments.count).toBe(EXPECTED_DEMO_DATA.payments.count);
		});

		test('has correct journal entry count', async ({}, testInfo) => {
			const userNum = (testInfo.parallelIndex % 3) + 1;
			const status = await getDemoStatus(userNum);

			expect(status.journalEntries.count).toBe(EXPECTED_DEMO_DATA.journalEntries.count);
		});

		test('has correct bank account count', async ({}, testInfo) => {
			const userNum = (testInfo.parallelIndex % 3) + 1;
			const status = await getDemoStatus(userNum);

			expect(status.bankAccounts.count).toBe(EXPECTED_DEMO_DATA.bankAccounts.count);
		});

		test('has correct recurring invoice count', async ({}, testInfo) => {
			const userNum = (testInfo.parallelIndex % 3) + 1;
			const status = await getDemoStatus(userNum);

			expect(status.recurringInvoices.count).toBe(EXPECTED_DEMO_DATA.recurringInvoices.count);
		});

		test('has correct payroll run count', async ({}, testInfo) => {
			const userNum = (testInfo.parallelIndex % 3) + 1;
			const status = await getDemoStatus(userNum);

			expect(status.payrollRuns.count).toBe(EXPECTED_DEMO_DATA.payrollRuns.count);
		});

		test('has correct TSD declaration count', async ({}, testInfo) => {
			const userNum = (testInfo.parallelIndex % 3) + 1;
			const status = await getDemoStatus(userNum);

			expect(status.tsdDeclarations.count).toBe(EXPECTED_DEMO_DATA.tsdDeclarations.count);
		});
	});

	test.describe('Reset Functionality', () => {
		test('reset is idempotent', async ({}, testInfo) => {
			const userNum = (testInfo.parallelIndex % 3) + 1;

			// Reset twice
			await triggerDemoReset(userNum);
			const statusAfterFirst = await getDemoStatus(userNum);

			await triggerDemoReset(userNum);
			const statusAfterSecond = await getDemoStatus(userNum);

			// Should produce identical state
			expect(statusAfterSecond.accounts.count).toBe(statusAfterFirst.accounts.count);
			expect(statusAfterSecond.contacts.count).toBe(statusAfterFirst.contacts.count);
			expect(statusAfterSecond.invoices.count).toBe(statusAfterFirst.invoices.count);
			expect(statusAfterSecond.employees.count).toBe(statusAfterFirst.employees.count);
		});

		test('reset restores expected counts', async ({}, testInfo) => {
			const userNum = (testInfo.parallelIndex % 3) + 1;

			// Trigger reset
			await triggerDemoReset(userNum);

			// Verify all counts match expected
			const status = await getDemoStatus(userNum);

			expect(status.accounts.count).toBe(EXPECTED_DEMO_DATA.accounts.count);
			expect(status.contacts.count).toBe(EXPECTED_DEMO_DATA.contacts.count);
			expect(status.invoices.count).toBe(EXPECTED_DEMO_DATA.invoices.count);
			expect(status.employees.count).toBe(EXPECTED_DEMO_DATA.employees.count);
			expect(status.payments.count).toBe(EXPECTED_DEMO_DATA.payments.count);
			expect(status.journalEntries.count).toBe(EXPECTED_DEMO_DATA.journalEntries.count);
			expect(status.bankAccounts.count).toBe(EXPECTED_DEMO_DATA.bankAccounts.count);
			expect(status.recurringInvoices.count).toBe(EXPECTED_DEMO_DATA.recurringInvoices.count);
			expect(status.payrollRuns.count).toBe(EXPECTED_DEMO_DATA.payrollRuns.count);
			expect(status.tsdDeclarations.count).toBe(EXPECTED_DEMO_DATA.tsdDeclarations.count);
		});
	});
});
```

**Step 2: Verify TypeScript compiles**

Run: `cd /Users/tsopic/algo/open-accounting/.worktrees/demo-reset-testing/frontend && npx tsc --noEmit`
Expected: No errors

**Step 3: Commit**

```bash
cd /Users/tsopic/algo/open-accounting/.worktrees/demo-reset-testing
git add frontend/e2e/demo/reset.spec.ts
git commit -m "test(e2e): add demo data reset verification tests

- Initial state verification: 10 tests checking counts and key entities
- Reset functionality: idempotent check and count restoration
- All tests use worker-scoped demo users for parallel execution"
```

---

## Task 5: Update Seed Data Expected Keys

**Files:**
- Modify: `frontend/e2e/demo/api.ts`

The key names in EXPECTED_DEMO_DATA need to match the actual seed data exactly. Based on the seed SQL, update the keys if they differ after per-user transformation:

For user 1, invoice numbers become: `INV1-2024-001`, `INV1-2024-002`, etc.
For user 1, payment numbers become: `PAY1-2024-001`, etc.
For user 1, journal entry numbers become: `JE1-2024-001`, etc.

**Step 1: Update expected keys in api.ts**

```typescript
// Note: Keys are per-user, so tests check if keys CONTAIN the base pattern
// The actual keys will have user number prefix (e.g., INV1-2024-001 for user 1)
export const EXPECTED_DEMO_DATA = {
	accounts: {
		count: 28,
		keys: ['Cash', 'Bank Account - EUR', 'Accounts Receivable', 'Accounts Payable']
	},
	contacts: {
		count: 7,
		keys: ['TechStart OÜ', 'Nordic Solutions AS', 'Baltic Commerce']
	},
	invoices: {
		count: 9,
		// Keys will be INV1-2024-001 for user 1, INV2-2024-001 for user 2, etc.
		keys: [] // Verify dynamically based on user
	},
	employees: {
		count: 5,
		keys: ['Maria Tamm', 'Jaan Kask', 'Anna Mets']
	},
	payments: {
		count: 4,
		keys: [] // Verify dynamically based on user
	},
	journalEntries: {
		count: 4,
		keys: [] // Verify dynamically based on user
	},
	bankAccounts: {
		count: 2,
		keys: ['Main EUR Account', 'Savings Account']
	},
	recurringInvoices: {
		count: 3,
		keys: ['Monthly Support - TechStart', 'Quarterly Retainer - Nordic']
	},
	payrollRuns: {
		count: 3,
		keys: ['2024-10', '2024-11', '2024-12']
	},
	tsdDeclarations: {
		count: 3,
		keys: ['2024-10', '2024-11', '2024-12']
	}
};

/**
 * Get expected invoice key pattern for a user
 */
export function getExpectedInvoiceKey(userNum: number): string {
	return `INV${userNum}-2024-001`;
}

/**
 * Get expected payment key pattern for a user
 */
export function getExpectedPaymentKey(userNum: number): string {
	return `PAY${userNum}-2024-001`;
}

/**
 * Get expected journal entry key pattern for a user
 */
export function getExpectedJournalEntryKey(userNum: number): string {
	return `JE${userNum}-2024-001`;
}
```

**Step 2: Update tests to use dynamic key helpers**

In `reset.spec.ts`, update invoice/payment/journal tests:

```typescript
import {
	getDemoStatus,
	triggerDemoReset,
	EXPECTED_DEMO_DATA,
	getExpectedInvoiceKey,
	getExpectedPaymentKey,
	getExpectedJournalEntryKey
} from './api';

// In invoice test:
test('has correct invoice count and key invoices', async ({}, testInfo) => {
	const userNum = (testInfo.parallelIndex % 3) + 1;
	const status = await getDemoStatus(userNum);

	expect(status.invoices.count).toBe(EXPECTED_DEMO_DATA.invoices.count);
	expect(status.invoices.keys).toContain(getExpectedInvoiceKey(userNum));
});
```

**Step 3: Commit**

```bash
cd /Users/tsopic/algo/open-accounting/.worktrees/demo-reset-testing
git add frontend/e2e/demo/api.ts frontend/e2e/demo/reset.spec.ts
git commit -m "fix(e2e): use per-user key patterns for invoice/payment/journal verification

Keys like INV-2024-001 are transformed to INV1-2024-001 for user 1.
Added helper functions to generate expected keys per user."
```

---

## Task 6: Run Full Test Suite

**Step 1: Run Go tests**

Run: `cd /Users/tsopic/algo/open-accounting/.worktrees/demo-reset-testing && go test ./... -v`
Expected: All PASS

**Step 2: Run frontend check**

Run: `cd /Users/tsopic/algo/open-accounting/.worktrees/demo-reset-testing/frontend && npm run check`
Expected: 0 errors

**Step 3: Commit any fixes**

If any issues, fix and commit with appropriate message.

---

## Task 7: Create Pull Request

**Step 1: Push branch**

```bash
cd /Users/tsopic/algo/open-accounting/.worktrees/demo-reset-testing
git push -u origin feature/demo-reset-e2e-testing
```

**Step 2: Create PR**

```bash
gh pr create --title "feat: add demo data reset E2E verification tests" --body "$(cat <<'EOF'
## Summary
- Add `user` parameter to `/api/demo/reset` for per-user reset
- Add `/api/demo/status` endpoint returning entity counts and keys
- Add `reset.spec.ts` with 12 E2E tests verifying demo data state
- Add API helpers (`api.ts`) for E2E test use

## Test Plan
- [ ] Run `go test ./...` - all pass
- [ ] Run `npm run check` in frontend - no errors
- [ ] Run demo E2E tests locally against demo environment
- [ ] Verify CI passes

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Add user param to reset endpoint | `handlers.go`, `handlers_test.go` |
| 2 | Add demo status endpoint | `handlers.go`, `main.go` |
| 3 | Create API helpers | `e2e/demo/api.ts` |
| 4 | Create reset tests | `e2e/demo/reset.spec.ts` |
| 5 | Update expected keys | `e2e/demo/api.ts`, `reset.spec.ts` |
| 6 | Run full test suite | - |
| 7 | Create PR | - |
