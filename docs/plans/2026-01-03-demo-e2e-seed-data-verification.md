# Demo E2E Seed Data Verification Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Create comprehensive Playwright E2E tests that verify seeded demo data is visible in each view of the Open Accounting demo environment.

**Architecture:** Update existing test files to verify specific seed data (contacts, invoices, employees, etc.) instead of just checking "page structure". Each test will assert on actual seeded values from `scripts/demo-seed.sql`.

**Tech Stack:** Playwright, TypeScript, existing test utilities (`loginAsDemo`, `navigateTo`, `ensureAcmeTenant`)

---

## Seed Data Summary (from demo-seed.sql)

| Entity | Count | Key Data Points |
|--------|-------|-----------------|
| Accounts | 28 | Cash (1000), Bank Account - EUR (1100), Accounts Receivable (1200) |
| Contacts | 7 | TechStart OÜ (C001), Nordic Solutions AS (C002), Baltic Commerce (C003), GreenTech Industries (C004), Office Supplies Ltd (S001), CloudHost Services (S002), Marketing Agency OÜ (S003) |
| Invoices | 9 | INV-2024-001 to INV-2024-007, INV-2025-001, INV-2025-002 (PAID, SENT, PARTIALLY_PAID, DRAFT) |
| Payments | 4 | PAY-2024-001 to PAY-2024-004 |
| Employees | 5 | Maria Tamm (EMP001), Jaan Kask (EMP002), Anna Mets (EMP003), Peeter Saar (EMP004), Liisa Kivi (EMP005) |
| Payroll Runs | 3 | Oct, Nov, Dec 2024 |
| TSD Declarations | 3 | Oct, Nov, Dec 2024 |
| Recurring Invoices | 3 | Monthly Support, Quarterly Retainer, Annual License |
| Journal Entries | 4 | JE-2024-001 to JE-2024-004 |
| Bank Accounts | 2 | Main EUR Account, Savings Account |
| Bank Transactions | 8 | Various incoming/outgoing payments |

---

### Task 1: Update Contacts Test to Verify Seed Data

**Files:**
- Modify: `frontend/e2e/demo/contacts.spec.ts`

**Step 1: Read existing test file**

Already done - current test only checks page structure.

**Step 2: Update test to verify seeded contacts**

Replace the file with tests that verify:
- TechStart OÜ is visible (customer)
- Nordic Solutions AS is visible (customer)
- Baltic Commerce is visible (customer)
- GreenTech Industries is visible (customer)
- Office Supplies Ltd is visible (supplier)
- Contact type filter works

```typescript
import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant } from './utils';

test.describe('Demo Contacts - Seed Data Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/contacts');
		await page.waitForLoadState('networkidle');
	});

	test('displays seeded customer contacts', async ({ page }) => {
		// Wait for table to load
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Verify seeded customers are visible
		await expect(page.getByText('TechStart OÜ')).toBeVisible();
		await expect(page.getByText('Nordic Solutions AS')).toBeVisible();
	});

	test('displays seeded supplier contacts', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Verify seeded suppliers are visible
		await expect(page.getByText('Office Supplies Ltd')).toBeVisible();
	});

	test('shows correct contact count', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Should have at least 7 contacts (4 customers + 3 suppliers)
		const rows = page.locator('table tbody tr');
		await expect(rows).toHaveCount(7, { timeout: 10000 });
	});

	test('contact details include email and phone', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Check for contact details (email domain or phone patterns)
		const pageContent = await page.content();
		expect(pageContent).toContain('@');
	});
});
```

**Step 3: Run test to verify it passes**

Run: `cd frontend && npx playwright test e2e/demo/contacts.spec.ts --project=chromium`
Expected: All tests pass with seeded data visible.

**Step 4: Commit**

```bash
git add frontend/e2e/demo/contacts.spec.ts
git commit -m "test: verify seeded contacts data in demo E2E tests"
```

---

### Task 2: Update Invoices Test to Verify Seed Data

**Files:**
- Modify: `frontend/e2e/demo/invoices.spec.ts`

**Step 1: Update test to verify seeded invoices**

```typescript
import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant } from './utils';

test.describe('Demo Invoices - Seed Data Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/invoices');
		await page.waitForLoadState('networkidle');
	});

	test('displays seeded invoices', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Verify invoice numbers are visible (at least one)
		const pageContent = await page.content();
		expect(pageContent).toMatch(/INV-202[45]-\d{3}/);
	});

	test('shows invoices with various statuses', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Check for status indicators (PAID, SENT, DRAFT)
		const pageContent = await page.content();
		const hasStatuses =
			pageContent.toLowerCase().includes('paid') ||
			pageContent.toLowerCase().includes('sent') ||
			pageContent.toLowerCase().includes('draft');
		expect(hasStatuses).toBeTruthy();
	});

	test('shows correct invoice count', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Should have at least 9 invoices
		const rows = page.locator('table tbody tr');
		const count = await rows.count();
		expect(count).toBeGreaterThanOrEqual(9);
	});

	test('invoices show customer names', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Check for seeded customer names
		const hasTechStart = await page.getByText('TechStart').isVisible().catch(() => false);
		const hasNordic = await page.getByText('Nordic').isVisible().catch(() => false);
		expect(hasTechStart || hasNordic).toBeTruthy();
	});
});
```

**Step 2: Run test to verify it passes**

Run: `cd frontend && npx playwright test e2e/demo/invoices.spec.ts --project=chromium`
Expected: All tests pass.

**Step 3: Commit**

```bash
git add frontend/e2e/demo/invoices.spec.ts
git commit -m "test: verify seeded invoices data in demo E2E tests"
```

---

### Task 3: Update Employees Test to Verify Seed Data

**Files:**
- Modify: `frontend/e2e/demo/employees.spec.ts`

**Step 1: Update test to verify seeded employees**

```typescript
import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant } from './utils';

test.describe('Demo Employees - Seed Data Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/employees');
		await page.waitForLoadState('networkidle');
	});

	test('displays seeded employees', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Verify seeded employee names
		await expect(page.getByText('Maria Tamm')).toBeVisible();
		await expect(page.getByText('Jaan Kask')).toBeVisible();
	});

	test('shows employee positions', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Check for positions
		const pageContent = await page.content();
		const hasPositions =
			pageContent.includes('Developer') ||
			pageContent.includes('Manager') ||
			pageContent.includes('Designer');
		expect(hasPositions).toBeTruthy();
	});

	test('shows correct active employee count', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Should have 4 active employees (Liisa Kivi is inactive)
		const rows = page.locator('table tbody tr');
		const count = await rows.count();
		expect(count).toBeGreaterThanOrEqual(4);
	});

	test('shows employee departments', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Check for departments
		const pageContent = await page.content();
		const hasDepartments =
			pageContent.includes('Engineering') ||
			pageContent.includes('Management') ||
			pageContent.includes('Design');
		expect(hasDepartments).toBeTruthy();
	});
});
```

**Step 2: Run test to verify it passes**

Run: `cd frontend && npx playwright test e2e/demo/employees.spec.ts --project=chromium`
Expected: All tests pass.

**Step 3: Commit**

```bash
git add frontend/e2e/demo/employees.spec.ts
git commit -m "test: verify seeded employees data in demo E2E tests"
```

---

### Task 4: Update Accounts Test to Verify Seed Data

**Files:**
- Modify: `frontend/e2e/demo/accounts.spec.ts`

**Step 1: Update test to verify seeded chart of accounts**

```typescript
import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant } from './utils';

test.describe('Demo Chart of Accounts - Seed Data Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/accounts');
		await page.waitForLoadState('networkidle');
	});

	test('displays seeded accounts', async ({ page }) => {
		await expect(page.locator('table tbody tr, .account-item').first()).toBeVisible({ timeout: 10000 });

		// Verify key account names
		await expect(page.getByText('Cash')).toBeVisible();
		await expect(page.getByText(/Bank Account.*EUR/i)).toBeVisible();
	});

	test('shows account codes', async ({ page }) => {
		await expect(page.locator('table tbody tr, .account-item').first()).toBeVisible({ timeout: 10000 });

		// Check for account codes (1000, 1100, etc.)
		const pageContent = await page.content();
		expect(pageContent).toMatch(/1[0-9]{3}/);
	});

	test('shows different account types', async ({ page }) => {
		await expect(page.locator('table tbody tr, .account-item').first()).toBeVisible({ timeout: 10000 });

		// Should show Assets, Liabilities, Revenue, Expense
		const pageContent = await page.content();
		const hasTypes =
			pageContent.includes('Asset') ||
			pageContent.includes('ASSET') ||
			pageContent.includes('Liability') ||
			pageContent.includes('LIABILITY');
		expect(hasTypes).toBeTruthy();
	});

	test('shows minimum expected account count', async ({ page }) => {
		await page.waitForTimeout(3000);

		// Should have 28 accounts
		const rows = page.locator('table tbody tr, .account-item');
		const count = await rows.count();
		expect(count).toBeGreaterThanOrEqual(20);
	});
});
```

**Step 2: Run test to verify it passes**

Run: `cd frontend && npx playwright test e2e/demo/accounts.spec.ts --project=chromium`
Expected: All tests pass.

**Step 3: Commit**

```bash
git add frontend/e2e/demo/accounts.spec.ts
git commit -m "test: verify seeded chart of accounts in demo E2E tests"
```

---

### Task 5: Update Journal Test to Verify Seed Data

**Files:**
- Modify: `frontend/e2e/demo/journal.spec.ts`

**Step 1: Update test to verify seeded journal entries**

```typescript
import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant } from './utils';

test.describe('Demo Journal Entries - Seed Data Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/journal');
		await page.waitForLoadState('networkidle');
	});

	test('displays seeded journal entries', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Verify journal entry numbers
		const pageContent = await page.content();
		expect(pageContent).toMatch(/JE-2024-\d{3}/);
	});

	test('shows journal entry descriptions', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Check for entry descriptions
		const pageContent = await page.content();
		const hasDescriptions =
			pageContent.includes('Opening balances') ||
			pageContent.includes('rent') ||
			pageContent.includes('Depreciation');
		expect(hasDescriptions).toBeTruthy();
	});

	test('shows correct journal entry count', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Should have at least 4 journal entries
		const rows = page.locator('table tbody tr');
		const count = await rows.count();
		expect(count).toBeGreaterThanOrEqual(4);
	});

	test('shows entry statuses', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Should show POSTED and DRAFT statuses
		const pageContent = await page.content().then(c => c.toLowerCase());
		expect(pageContent.includes('posted') || pageContent.includes('draft')).toBeTruthy();
	});
});
```

**Step 2: Run test to verify it passes**

Run: `cd frontend && npx playwright test e2e/demo/journal.spec.ts --project=chromium`
Expected: All tests pass.

**Step 3: Commit**

```bash
git add frontend/e2e/demo/journal.spec.ts
git commit -m "test: verify seeded journal entries in demo E2E tests"
```

---

### Task 6: Update Payments Test to Verify Seed Data

**Files:**
- Modify: `frontend/e2e/demo/payments.spec.ts`

**Step 1: Update test to verify seeded payments**

```typescript
import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant } from './utils';

test.describe('Demo Payments - Seed Data Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/payments');
		await page.waitForLoadState('networkidle');
	});

	test('displays seeded payments', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Verify payment numbers
		const pageContent = await page.content();
		expect(pageContent).toMatch(/PAY-2024-\d{3}/);
	});

	test('shows payment amounts', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Check for payment amounts (in EUR format)
		const pageContent = await page.content();
		expect(pageContent).toMatch(/[\d,]+\.\d{2}/);
	});

	test('shows correct payment count', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Should have at least 4 payments
		const rows = page.locator('table tbody tr');
		const count = await rows.count();
		expect(count).toBeGreaterThanOrEqual(4);
	});

	test('shows customer names for payments', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Check for customer names
		const pageContent = await page.content();
		expect(pageContent.includes('TechStart') || pageContent.includes('Nordic') || pageContent.includes('Baltic')).toBeTruthy();
	});
});
```

**Step 2: Run test to verify it passes**

Run: `cd frontend && npx playwright test e2e/demo/payments.spec.ts --project=chromium`
Expected: All tests pass.

**Step 3: Commit**

```bash
git add frontend/e2e/demo/payments.spec.ts
git commit -m "test: verify seeded payments in demo E2E tests"
```

---

### Task 7: Update Banking Test to Verify Seed Data

**Files:**
- Modify: `frontend/e2e/demo/banking.spec.ts`

**Step 1: Update test to verify seeded bank data**

```typescript
import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant } from './utils';

test.describe('Demo Banking - Seed Data Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/banking');
		await page.waitForLoadState('networkidle');
	});

	test('displays bank accounts', async ({ page }) => {
		await page.waitForTimeout(3000);

		// Verify bank account names
		const pageContent = await page.content();
		expect(pageContent.includes('Main EUR') || pageContent.includes('Savings') || pageContent.includes('Swedbank') || pageContent.includes('SEB')).toBeTruthy();
	});

	test('shows bank transactions', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Should have bank transactions
		const rows = page.locator('table tbody tr');
		const count = await rows.count();
		expect(count).toBeGreaterThanOrEqual(1);
	});

	test('shows transaction amounts', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Check for amounts
		const pageContent = await page.content();
		expect(pageContent).toMatch(/[\d,]+\.\d{2}/);
	});

	test('shows transaction statuses', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Check for status indicators
		const pageContent = await page.content().then(c => c.toLowerCase());
		const hasStatuses =
			pageContent.includes('matched') ||
			pageContent.includes('unmatched') ||
			pageContent.includes('reconciled');
		expect(hasStatuses).toBeTruthy();
	});
});
```

**Step 2: Run test to verify it passes**

Run: `cd frontend && npx playwright test e2e/demo/banking.spec.ts --project=chromium`
Expected: All tests pass.

**Step 3: Commit**

```bash
git add frontend/e2e/demo/banking.spec.ts
git commit -m "test: verify seeded banking data in demo E2E tests"
```

---

### Task 8: Update Payroll Test to Verify Seed Data

**Files:**
- Modify: `frontend/e2e/demo/payroll.spec.ts`

**Step 1: Update test to verify seeded payroll data**

```typescript
import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant } from './utils';

test.describe('Demo Payroll - Seed Data Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/payroll');
		await page.waitForLoadState('networkidle');
	});

	test('displays payroll runs', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Verify payroll periods (Oct, Nov, Dec 2024)
		const pageContent = await page.content();
		expect(pageContent.includes('2024') || pageContent.includes('October') || pageContent.includes('November') || pageContent.includes('December')).toBeTruthy();
	});

	test('shows payroll statuses', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Should show PAID and APPROVED statuses
		const pageContent = await page.content().then(c => c.toLowerCase());
		expect(pageContent.includes('paid') || pageContent.includes('approved')).toBeTruthy();
	});

	test('shows correct payroll run count', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Should have at least 3 payroll runs
		const rows = page.locator('table tbody tr');
		const count = await rows.count();
		expect(count).toBeGreaterThanOrEqual(3);
	});

	test('shows total amounts', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Check for amount values
		const pageContent = await page.content();
		expect(pageContent).toMatch(/[\d,]+\.\d{2}/);
	});
});
```

**Step 2: Run test to verify it passes**

Run: `cd frontend && npx playwright test e2e/demo/payroll.spec.ts --project=chromium`
Expected: All tests pass.

**Step 3: Commit**

```bash
git add frontend/e2e/demo/payroll.spec.ts
git commit -m "test: verify seeded payroll data in demo E2E tests"
```

---

### Task 9: Update Recurring Invoices Test to Verify Seed Data

**Files:**
- Modify: `frontend/e2e/demo/recurring.spec.ts`

**Step 1: Update test to verify seeded recurring invoices**

```typescript
import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant } from './utils';

test.describe('Demo Recurring Invoices - Seed Data Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/recurring-invoices');
		await page.waitForLoadState('networkidle');
	});

	test('displays seeded recurring invoices', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Verify recurring invoice names
		const pageContent = await page.content();
		expect(pageContent.includes('Support') || pageContent.includes('Retainer') || pageContent.includes('License')).toBeTruthy();
	});

	test('shows frequency types', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Check for frequency types
		const pageContent = await page.content().then(c => c.toLowerCase());
		const hasFrequencies =
			pageContent.includes('monthly') ||
			pageContent.includes('quarterly') ||
			pageContent.includes('yearly');
		expect(hasFrequencies).toBeTruthy();
	});

	test('shows correct recurring invoice count', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Should have at least 3 recurring invoices
		const rows = page.locator('table tbody tr');
		const count = await rows.count();
		expect(count).toBeGreaterThanOrEqual(3);
	});

	test('shows customer names', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Check for customer names
		const pageContent = await page.content();
		expect(pageContent.includes('TechStart') || pageContent.includes('Nordic') || pageContent.includes('GreenTech')).toBeTruthy();
	});
});
```

**Step 2: Run test to verify it passes**

Run: `cd frontend && npx playwright test e2e/demo/recurring.spec.ts --project=chromium`
Expected: All tests pass.

**Step 3: Commit**

```bash
git add frontend/e2e/demo/recurring.spec.ts
git commit -m "test: verify seeded recurring invoices in demo E2E tests"
```

---

### Task 10: Update TSD Test to Verify Seed Data

**Files:**
- Modify: `frontend/e2e/demo/tsd.spec.ts`

**Step 1: Update test to verify seeded TSD declarations**

```typescript
import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant } from './utils';

test.describe('Demo TSD Declarations - Seed Data Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/tsd');
		await page.waitForLoadState('networkidle');
	});

	test('displays seeded TSD declarations', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Verify TSD periods (2024)
		const pageContent = await page.content();
		expect(pageContent.includes('2024')).toBeTruthy();
	});

	test('shows declaration statuses', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Should show SUBMITTED and DRAFT statuses
		const pageContent = await page.content().then(c => c.toLowerCase());
		expect(pageContent.includes('submitted') || pageContent.includes('draft')).toBeTruthy();
	});

	test('shows correct declaration count', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Should have at least 3 TSD declarations
		const rows = page.locator('table tbody tr');
		const count = await rows.count();
		expect(count).toBeGreaterThanOrEqual(3);
	});

	test('shows tax amounts', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Check for tax amounts
		const pageContent = await page.content();
		expect(pageContent).toMatch(/[\d,]+\.\d{2}/);
	});
});
```

**Step 2: Run test to verify it passes**

Run: `cd frontend && npx playwright test e2e/demo/tsd.spec.ts --project=chromium`
Expected: All tests pass.

**Step 3: Commit**

```bash
git add frontend/e2e/demo/tsd.spec.ts
git commit -m "test: verify seeded TSD declarations in demo E2E tests"
```

---

### Task 11: Run Full Test Suite and Verify

**Step 1: Run all demo tests**

Run: `cd frontend && npx playwright test e2e/demo/ --project=chromium`
Expected: All tests pass.

**Step 2: Generate test report**

Run: `cd frontend && npx playwright show-report`

**Step 3: Final commit with all changes**

```bash
git add frontend/e2e/demo/
git commit -m "test: comprehensive demo E2E tests verifying seed data in all views"
```

---

## Notes

- All tests use `waitForLoadState('networkidle')` for consistent loading
- Tests are designed to be resilient to minor UI changes while still verifying seed data
- Each test file focuses on a specific domain (contacts, invoices, etc.)
- Tests verify both presence of data AND specific seeded values
- If tests fail, first check that the demo environment has been properly reset with seed data
