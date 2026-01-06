# Multiple Demo Users and Parallel Testing Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Enable parallel E2E test execution by creating 3 isolated demo users, each with their own tenant and seed data, reducing test runtime from 20+ minutes to ~7 minutes.

**Architecture:** Create 3 demo users (demo1@example.com, demo2@example.com, demo3@example.com) each linked to isolated tenants (tenant_demo1, tenant_demo2, tenant_demo3). Playwright workers will be assigned specific demo credentials based on worker index, ensuring test isolation and enabling parallel execution.

**Tech Stack:** Go (backend), PostgreSQL (multi-tenant schemas), Playwright (E2E testing), TypeScript (frontend tests)

---

## Task 1: Update Demo Reset Handler for Multiple Users

**Files:**
- Modify: `cmd/api/handlers.go:1044-1098` (DemoReset function)
- Modify: `cmd/api/handlers.go:1100-end` (getDemoSeedSQL function)

**Step 1: Update DemoReset to handle all 3 demo users/tenants**

Replace the current single-user cleanup with multi-user cleanup. Update the `DemoReset` function starting at line 1044:

```go
ctx := r.Context()

// Demo identifiers for 3 parallel test users
demoUsers := []struct {
    email string
    slug  string
    schema string
}{
    {"demo1@example.com", "demo1", "tenant_demo1"},
    {"demo2@example.com", "demo2", "tenant_demo2"},
    {"demo3@example.com", "demo3", "tenant_demo3"},
}

// Drop all demo tenant schemas
for _, demo := range demoUsers {
    log.Info().Str("schema", demo.schema).Msg("Demo reset: dropping tenant schema")
    _, err := h.pool.Exec(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", demo.schema))
    if err != nil {
        log.Error().Err(err).Str("schema", demo.schema).Msg("Demo reset failed: drop schema")
        respondError(w, http.StatusInternalServerError, "Failed to drop tenant schema: "+err.Error())
        return
    }
}

// Delete demo data from public tables
for _, demo := range demoUsers {
    log.Info().Str("slug", demo.slug).Msg("Demo reset: cleaning tenant_users by slug")
    _, err := h.pool.Exec(ctx, "DELETE FROM tenant_users WHERE tenant_id IN (SELECT id FROM tenants WHERE slug = $1)", demo.slug)
    if err != nil {
        log.Error().Err(err).Msg("Demo reset failed: clean tenant_users")
        respondError(w, http.StatusInternalServerError, "Failed to clean tenant_users: "+err.Error())
        return
    }

    _, err = h.pool.Exec(ctx, "DELETE FROM tenants WHERE slug = $1", demo.slug)
    if err != nil {
        log.Error().Err(err).Msg("Demo reset failed: clean tenants")
        respondError(w, http.StatusInternalServerError, "Failed to clean tenants: "+err.Error())
        return
    }

    _, err = h.pool.Exec(ctx, "DELETE FROM users WHERE email = $1", demo.email)
    if err != nil {
        log.Error().Err(err).Msg("Demo reset failed: clean users")
        respondError(w, http.StatusInternalServerError, "Failed to clean users: "+err.Error())
        return
    }
}

log.Info().Msg("Demo reset: seeding demo data for all users")
```

**Step 2: Run test to verify the changes compile**

```bash
cd /Users/tsopic/algo/open-accounting && go build ./cmd/api
```
Expected: Successful build with no errors

**Step 3: Commit**

```bash
git add cmd/api/handlers.go
git commit -m "refactor: update demo reset to handle multiple demo users"
```

---

## Task 2: Create Multi-User Demo Seed SQL

**Files:**
- Modify: `cmd/api/handlers.go:1100-end` (getDemoSeedSQL function)
- Modify: `scripts/demo-seed.sql` (keep in sync)

**Step 1: Create helper function for generating per-user seed SQL**

Add this new function after `getDemoSeedSQL()`:

```go
// generateDemoUserSeed generates seed SQL for a single demo user/tenant
// userNum: 1, 2, or 3 - determines UUIDs and identifiers
func generateDemoUserSeed(userNum int) string {
    // UUID patterns: use userNum in the UUID to ensure uniqueness
    // User:   a0000000-0000-0000-000{userNum}-000000000001
    // Tenant: b0000000-0000-0000-000{userNum}-000000000001
    // Schema: tenant_demo{userNum}

    email := fmt.Sprintf("demo%d@example.com", userNum)
    slug := fmt.Sprintf("demo%d", userNum)
    schema := fmt.Sprintf("tenant_demo%d", userNum)
    companyName := fmt.Sprintf("Demo Company %d", userNum)

    userID := fmt.Sprintf("a0000000-0000-0000-000%d-000000000001", userNum)
    tenantID := fmt.Sprintf("b0000000-0000-0000-000%d-000000000001", userNum)

    return fmt.Sprintf(`
-- Demo User %d (password: demo12345)
INSERT INTO users (id, email, password_hash, name, is_active)
VALUES (
    '%s'::uuid,
    '%s',
    '$2a$10$NDz5VvAjksvnHzAq1p892.rZedeCGsy08iEiYzMUWcudFe7XH08pi',
    'Demo User %d',
    true
) ON CONFLICT (email) DO NOTHING;

-- Demo Tenant %d
INSERT INTO tenants (id, name, slug, schema_name, settings, is_active)
VALUES (
    '%s'::uuid,
    '%s',
    '%s',
    '%s',
    '{
        "reg_code": "1234567%d",
        "vat_number": "EE12345678%d",
        "address": "Demo Street %d, Tallinn",
        "email": "info@demo%d.example.com",
        "phone": "+372 5123 456%d",
        "bank_details": "Swedbank EE12345678901234567%d",
        "invoice_prefix": "INV-%d-",
        "invoice_footer": "Thank you for your business!",
        "default_payment_terms": 14,
        "pdf_primary_color": "#4f46e5"
    }'::jsonb,
    true
) ON CONFLICT (slug) DO NOTHING;

-- Mark onboarding as complete
DO $$ BEGIN
    UPDATE tenants SET onboarding_completed = true WHERE id = '%s'::uuid;
EXCEPTION WHEN undefined_column THEN
    NULL;
END $$;

-- Link demo user to tenant
INSERT INTO tenant_users (tenant_id, user_id, role, is_default)
VALUES (
    '%s'::uuid,
    '%s'::uuid,
    'admin',
    true
) ON CONFLICT (tenant_id, user_id) DO NOTHING;

-- Create tenant schema with all tables
SELECT create_tenant_schema('%s');

-- Add tables from later migrations
SELECT add_recurring_tables_to_schema('%s');
SELECT add_email_tables_to_schema('%s');
SELECT add_reconciliation_tables_to_schema('%s');
SELECT add_payroll_tables('%s');
SELECT add_recurring_email_fields_to_schema('%s');
`,
        userNum, userID, email, userNum,
        userNum, tenantID, companyName, slug, schema,
        userNum, userNum, userNum, userNum, userNum, userNum, userNum,
        tenantID, tenantID, userID,
        schema, schema, schema, schema, schema, schema)
}
```

**Step 2: Update getDemoSeedSQL to use the generator for all 3 users**

```go
func getDemoSeedSQL() string {
    var sql strings.Builder

    // Generate seed data for all 3 demo users
    for userNum := 1; userNum <= 3; userNum++ {
        sql.WriteString(generateDemoUserSeed(userNum))
        sql.WriteString(generateDemoTenantData(userNum))
    }

    return sql.String()
}
```

**Step 3: Create generateDemoTenantData function for seed data**

```go
// generateDemoTenantData generates the accounts, contacts, invoices, etc. for a demo tenant
func generateDemoTenantData(userNum int) string {
    schema := fmt.Sprintf("tenant_demo%d", userNum)
    tenantID := fmt.Sprintf("b0000000-0000-0000-000%d-000000000001", userNum)
    userID := fmt.Sprintf("a0000000-0000-0000-000%d-000000000001", userNum)

    // Account IDs: c{userNum}000000-0000-0000-000X-00000000000Y
    // Contact IDs: d{userNum}000000-0000-0000-000X-00000000000Y
    // Invoice IDs: e{userNum}000000-0000-0000-000X-00000000000Y
    // ... etc

    return fmt.Sprintf(`
-- Chart of Accounts for tenant %d
INSERT INTO %s.accounts (id, tenant_id, code, name, account_type, is_system) VALUES
('c%d000000-0000-0000-0001-000000000001'::uuid, '%s'::uuid, '1000', 'Cash', 'ASSET', true),
('c%d000000-0000-0000-0001-000000000002'::uuid, '%s'::uuid, '1100', 'Bank Account - EUR', 'ASSET', true),
('c%d000000-0000-0000-0001-000000000003'::uuid, '%s'::uuid, '1200', 'Accounts Receivable', 'ASSET', true),
-- ... (continue with all accounts, adapting UUIDs)
ON CONFLICT DO NOTHING;

-- Contacts for tenant %d
INSERT INTO %s.contacts (id, tenant_id, code, name, contact_type, reg_code, vat_number, email, phone, address_line1, city, postal_code, country_code, payment_terms_days) VALUES
('d%d000000-0000-0000-0001-000000000001'::uuid, '%s'::uuid, 'C001', 'TechStart Demo%d OÜ', 'CUSTOMER', '1456789%d', 'EE14567890%d', 'info@techstart%d.ee', '+372 5234 5678', 'Pärnu mnt 15', 'Tallinn', '10141', 'EE', 14)
-- ... (continue with all contacts)
ON CONFLICT DO NOTHING;

-- ... (invoices, payments, etc. with adapted UUIDs)
`, userNum, schema,
   userNum, tenantID,
   userNum, tenantID,
   userNum, tenantID,
   userNum, schema,
   userNum, tenantID, userNum, userNum, userNum, userNum)
}
```

**Step 4: Add strings import if not present**

```go
import (
    // existing imports...
    "strings"
)
```

**Step 5: Run test to verify changes compile**

```bash
cd /Users/tsopic/algo/open-accounting && go build ./cmd/api
```

**Step 6: Commit**

```bash
git add cmd/api/handlers.go
git commit -m "feat: generate seed data for 3 demo users/tenants"
```

---

## Task 3: Update Playwright Demo Config for Parallel Workers

**Files:**
- Modify: `frontend/playwright.demo.config.ts`

**Step 1: Update config to enable parallel testing**

```typescript
import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright configuration for Demo Environment E2E tests
 *
 * Run with: npx playwright test --config=playwright.demo.config.ts
 * Or: npm run test:e2e:demo
 *
 * These tests run against the live demo environment:
 * - Frontend: https://open-accounting.up.railway.app
 * - API: https://open-accounting-api.up.railway.app
 *
 * Parallel testing is enabled with 3 workers, each using a dedicated demo user:
 * - Worker 0: demo1@example.com / tenant_demo1
 * - Worker 1: demo2@example.com / tenant_demo2
 * - Worker 2: demo3@example.com / tenant_demo3
 */
export default defineConfig({
    testDir: './e2e',
    testMatch: ['**/demo/*.spec.ts', 'demo-env.spec.ts', 'demo-all-views.spec.ts'],
    fullyParallel: true, // Enable parallel execution
    forbidOnly: !!process.env.CI,
    retries: 2,
    workers: 3, // 3 workers for 3 demo users
    reporter: [
        ['html', { outputFolder: 'playwright-report-demo' }],
        ['list'],
        ['json', { outputFile: 'demo-test-results.json' }]
    ],
    timeout: 60000,

    use: {
        baseURL: 'https://open-accounting.up.railway.app',
        trace: 'on-first-retry',
        screenshot: 'only-on-failure',
        video: 'retain-on-failure',
        actionTimeout: 15000,
        navigationTimeout: 30000
    },

    projects: [
        {
            name: 'demo-chromium',
            use: {
                ...devices['Desktop Chrome'],
                storageState: { cookies: [], origins: [] }
            }
        }
    ]
});
```

**Step 2: Run Playwright config validation**

```bash
cd /Users/tsopic/algo/open-accounting/frontend && npx playwright test --config=playwright.demo.config.ts --list
```
Expected: Lists test files without errors

**Step 3: Commit**

```bash
git add frontend/playwright.demo.config.ts
git commit -m "feat: enable parallel E2E testing with 3 workers"
```

---

## Task 4: Update Demo Test Utilities for Worker-Based Credentials

**Files:**
- Modify: `frontend/e2e/demo/utils.ts`

**Step 1: Update utils to support worker-indexed credentials**

```typescript
import { Page, expect, TestInfo } from '@playwright/test';

export const DEMO_URL = 'https://open-accounting.up.railway.app';
export const DEMO_API_URL = 'https://open-accounting-api.up.railway.app';

// Demo credentials mapped by worker index (0-2)
export const DEMO_CREDENTIALS = [
    { email: 'demo1@example.com', password: 'demo12345', tenantSlug: 'demo1' },
    { email: 'demo2@example.com', password: 'demo12345', tenantSlug: 'demo2' },
    { email: 'demo3@example.com', password: 'demo12345', tenantSlug: 'demo3' },
] as const;

/**
 * Get demo credentials for the current worker
 * @param testInfo - Playwright TestInfo object containing parallelIndex
 */
export function getDemoCredentials(testInfo: TestInfo) {
    const workerIndex = testInfo.parallelIndex % DEMO_CREDENTIALS.length;
    return DEMO_CREDENTIALS[workerIndex];
}

/**
 * Login as the demo user assigned to this worker
 */
export async function loginAsDemo(page: Page, testInfo: TestInfo): Promise<void> {
    const creds = getDemoCredentials(testInfo);

    await page.goto(`${DEMO_URL}/login`);
    await page.waitForLoadState('networkidle');
    await page.getByLabel(/email/i).fill(creds.email);
    await page.getByLabel(/password/i).fill(creds.password);
    await page.getByRole('button', { name: /sign in|login/i }).click();
    await page.waitForURL(/dashboard/, { timeout: 30000 });
    await page.waitForLoadState('networkidle');
}

export async function navigateTo(page: Page, path: string): Promise<void> {
    await page.goto(`${DEMO_URL}${path}`);
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(500);
}

/**
 * Ensure the correct demo tenant is selected for this worker
 */
export async function ensureDemoTenant(page: Page, testInfo: TestInfo): Promise<void> {
    const creds = getDemoCredentials(testInfo);
    const selector = page.locator('select').first();

    if (await selector.isVisible()) {
        const options = await selector.locator('option').all();
        for (const option of options) {
            const text = await option.textContent();
            if (text && text.toLowerCase().includes(creds.tenantSlug)) {
                const value = await option.getAttribute('value');
                if (value) {
                    await selector.selectOption(value);
                    break;
                }
            }
        }
        await page.waitForLoadState('networkidle');
    }
}

// Keep backward-compatible functions for gradual migration
export const DEMO_EMAIL = 'demo1@example.com';
export const DEMO_PASSWORD = 'demo12345';

export async function ensureAcmeTenant(page: Page): Promise<void> {
    // Deprecated: use ensureDemoTenant instead
    const selector = page.locator('select').first();
    if (await selector.isVisible()) {
        const currentValue = await selector.inputValue();
        if (!currentValue.includes('demo')) {
            const options = await selector.locator('option').all();
            for (const option of options) {
                const text = await option.textContent();
                if (text && /demo/i.test(text)) {
                    const value = await option.getAttribute('value');
                    if (value) {
                        await selector.selectOption(value);
                        break;
                    }
                }
            }
            await page.waitForLoadState('networkidle');
        }
    }
}

export async function assertTableRowCount(page: Page, minRows: number): Promise<void> {
    const rows = page.locator('table tbody tr');
    const count = await rows.count();
    expect(count).toBeGreaterThanOrEqual(minRows);
}

export async function assertTextVisible(page: Page, text: string | RegExp): Promise<void> {
    await expect(page.getByText(text).first()).toBeVisible({ timeout: 10000 });
}
```

**Step 2: Run TypeScript check**

```bash
cd /Users/tsopic/algo/open-accounting/frontend && npm run check
```

**Step 3: Commit**

```bash
git add frontend/e2e/demo/utils.ts
git commit -m "feat: add worker-based demo credentials for parallel testing"
```

---

## Task 5: Update Demo Test Files to Use Worker Credentials

**Files:**
- Modify: `frontend/e2e/demo/dashboard.spec.ts`
- Modify: `frontend/e2e/demo/accounts.spec.ts`
- Modify: `frontend/e2e/demo/contacts.spec.ts`
- Modify: `frontend/e2e/demo/invoices.spec.ts`
- Modify: `frontend/e2e/demo/payments.spec.ts`
- Modify: `frontend/e2e/demo/employees.spec.ts`
- Modify: `frontend/e2e/demo/payroll.spec.ts`
- Modify: `frontend/e2e/demo/bank-accounts.spec.ts`
- Modify: `frontend/e2e/demo/fiscal-years.spec.ts`
- Modify: `frontend/e2e/demo/tsd.spec.ts`

**Step 1: Update each test file's beforeEach to pass testInfo**

Example pattern for each test file:

```typescript
import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureDemoTenant } from './utils';

test.describe('Demo [Feature] - Seed Data Verification', () => {
    test.beforeEach(async ({ page }, testInfo) => {
        await loginAsDemo(page, testInfo);
        await ensureDemoTenant(page, testInfo);
        await navigateTo(page, '/[path]');
        await page.waitForLoadState('networkidle');
    });

    // ... tests remain unchanged
});
```

**Step 2: Update dashboard.spec.ts**

```typescript
import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureDemoTenant, getDemoCredentials } from './utils';

test.describe('Demo Dashboard - Seed Data Verification', () => {
    test.beforeEach(async ({ page }, testInfo) => {
        await loginAsDemo(page, testInfo);
        await ensureDemoTenant(page, testInfo);
        await navigateTo(page, '/dashboard');
        await page.waitForLoadState('networkidle');
    });

    // Tests remain the same - they verify seed data exists
    test('displays revenue metrics', async ({ page }) => {
        // ...
    });
});
```

**Step 3: Apply same pattern to all other test files**

Repeat for: accounts.spec.ts, contacts.spec.ts, invoices.spec.ts, payments.spec.ts, employees.spec.ts, payroll.spec.ts, bank-accounts.spec.ts, fiscal-years.spec.ts, tsd.spec.ts

**Step 4: Run TypeScript check on all test files**

```bash
cd /Users/tsopic/algo/open-accounting/frontend && npm run check
```

**Step 5: Commit**

```bash
git add frontend/e2e/demo/*.spec.ts
git commit -m "refactor: update demo tests to use worker-based credentials"
```

---

## Task 6: Update Documentation

**Files:**
- Modify: `docs/DEPLOYMENT.md`

**Step 1: Add section about demo users**

Add after line 310 (Demo Mode section):

```markdown
### Demo Users for Parallel Testing

The demo environment includes 3 demo users for parallel E2E testing:

| User | Email | Password | Tenant |
|------|-------|----------|--------|
| Demo 1 | demo1@example.com | demo12345 | Demo Company 1 |
| Demo 2 | demo2@example.com | demo12345 | Demo Company 2 |
| Demo 3 | demo3@example.com | demo12345 | Demo Company 3 |

Each demo user has:
- Isolated tenant schema (tenant_demo1, tenant_demo2, tenant_demo3)
- Complete seed data (accounts, contacts, invoices, payments, employees, payroll)
- Admin access to their respective tenant

**Playwright Test Workers:**
- E2E tests run with 3 parallel workers
- Each worker uses a dedicated demo user (based on `parallelIndex`)
- This reduces test execution time from ~20 minutes to ~7 minutes

**Manual Testing:**
For manual testing, you can use any of the demo credentials above.
```

**Step 2: Commit**

```bash
git add docs/DEPLOYMENT.md
git commit -m "docs: add demo users documentation for parallel testing"
```

---

## Task 7: Sync demo-seed.sql Script

**Files:**
- Modify: `scripts/demo-seed.sql`

**Step 1: Update demo-seed.sql to match handlers.go**

The standalone script should generate the same 3 demo users/tenants. Since the handler generates this dynamically, extract the generated SQL and save it:

```bash
# After implementing the Go code, generate the SQL and update the script
# This ensures scripts/demo-seed.sql stays in sync with the handler
```

**Step 2: Commit**

```bash
git add scripts/demo-seed.sql
git commit -m "sync: update demo-seed.sql for 3 parallel demo users"
```

---

## Task 8: Test Locally and Deploy

**Step 1: Run backend tests**

```bash
cd /Users/tsopic/algo/open-accounting && go test ./...
```

**Step 2: Build and verify**

```bash
go build ./cmd/api && go build ./cmd/migrate
```

**Step 3: Push to trigger Railway deployment**

```bash
git push origin main
```

**Step 4: Trigger demo reset after deployment**

```bash
curl -X POST https://open-accounting-api.up.railway.app/api/demo/reset \
  -H "Content-Type: application/json" \
  -H "X-Demo-Secret: demo-reset-key"
```

**Step 5: Run demo E2E tests locally to verify parallel execution**

```bash
cd /Users/tsopic/algo/open-accounting/frontend && \
npx playwright test --config=playwright.demo.config.ts --project=demo-chromium
```

Expected: Tests complete in ~7 minutes with 3 workers

---

## Summary

| Task | Description | Estimated Effort |
|------|-------------|------------------|
| 1 | Update DemoReset for multiple users | Small |
| 2 | Create multi-user seed SQL generator | Medium |
| 3 | Update Playwright config for parallel | Small |
| 4 | Update demo test utilities | Small |
| 5 | Update all test files | Medium |
| 6 | Update documentation | Small |
| 7 | Sync demo-seed.sql | Small |
| 8 | Test and deploy | Small |

**Total:** 8 tasks, reducing test time from 20+ minutes to ~7 minutes.
