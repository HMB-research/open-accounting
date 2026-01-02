# Phase 2 Features Implementation Plan

> **Status:** 🚧 IN PROGRESS

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement Phase 2 features: E2E Testing Completion, Dashboard Improvements, Report Export (PDF/Excel), and Mobile Responsiveness.

**Architecture:** Extends existing SvelteKit frontend with new components (PeriodSelector, ExportButton, MobileNav), adds analytics API endpoints in Go backend, and integrates Playwright for comprehensive E2E coverage.

**Tech Stack:** Go backend, PostgreSQL, SvelteKit 2 + Svelte 5 frontend, Playwright E2E, SheetJS (xlsx), Chart.js

---

## Ralph Loop Commands

### Quick Reference

```bash
# Feature 1: E2E Testing Completion
/ralph-loop "Complete E2E test coverage for core user flows.

WORKFLOW:
1. Run existing E2E tests: cd frontend && npx playwright test --project=chromium
2. Identify uncovered flows by reviewing e2e/ directory
3. Add missing tests for: invoices CRUD, contacts, recurring invoices, reports
4. Ensure each test has proper setup/teardown
5. Run full suite to verify all pass

SUCCESS CRITERIA:
- All core flows have E2E tests (auth, dashboard, invoices, contacts, reports)
- All E2E tests pass on chromium
- CI e2e job passes

When all E2E tests pass, output: <promise>E2E_COMPLETE</promise>" --max-iterations 25 --completion-promise "E2E_COMPLETE"

# Feature 2: Dashboard Improvements
/ralph-loop "Implement dashboard improvements with period selector and enhanced charts.

WORKFLOW:
1. Create PeriodSelector.svelte component
2. Add analytics API endpoints (cashflow, aging, activity)
3. Update dashboard to use period selector
4. Add cash flow chart and activity feed
5. Run tests and verify

SUCCESS CRITERIA:
- Period selector allows This Month, Last Month, This Quarter, This Year, Custom
- Cash flow chart displays correctly
- Activity feed shows last 10 transactions
- All existing tests pass

When dashboard is complete, output: <promise>DASHBOARD_IMPROVED</promise>" --max-iterations 30 --completion-promise "DASHBOARD_IMPROVED"

# Feature 3: Report Export
/ralph-loop "Add PDF and Excel export to reports.

WORKFLOW:
1. Install xlsx library: cd frontend && npm install xlsx
2. Create ExportButton.svelte component with PDF/Excel/CSV options
3. Implement Excel export using SheetJS
4. Implement PDF export using browser print API
5. Add export to Trial Balance, Balance Sheet, Income Statement
6. Test exports work correctly

SUCCESS CRITERIA:
- Excel export generates valid .xlsx file
- PDF export opens print dialog
- CSV export generates valid CSV
- All three report types have export buttons

When exports work, output: <promise>EXPORTS_READY</promise>" --max-iterations 20 --completion-promise "EXPORTS_READY"

# Feature 4: Mobile Responsiveness
/ralph-loop "Make all pages mobile-friendly.

WORKFLOW:
1. Create MobileNav.svelte with hamburger menu
2. Add viewport store for responsive breakpoints
3. Audit each route for mobile issues
4. Convert tables to card view on mobile
5. Ensure touch targets >= 44px
6. Run mobile E2E tests

SUCCESS CRITERIA:
- Hamburger menu works on mobile
- Tables display as cards on <768px viewport
- All forms are touch-friendly
- Mobile E2E tests pass

When mobile is ready, output: <promise>MOBILE_READY</promise>" --max-iterations 30 --completion-promise "MOBILE_READY"
```

---

## Feature 1: E2E Testing Completion

**Priority:** High | **Effort:** ~2 days

### Task 1.1: Audit Existing E2E Tests

**Files:**
- Review: `frontend/e2e/*.spec.ts`
- Review: `frontend/playwright.config.ts`

**Step 1: List existing E2E tests**

Run: `ls -la frontend/e2e/`
Expected: See list of existing test files

**Step 2: Run existing tests to establish baseline**

Run: `cd frontend && npx playwright test --project=chromium --reporter=list`
Expected: Note which tests pass/fail

**Step 3: Document coverage gaps**

Create checklist of missing tests:
- [ ] Invoice creation flow
- [ ] Invoice editing flow
- [ ] Contact CRUD operations
- [ ] Recurring invoice setup
- [ ] Report viewing and date filtering
- [ ] Mobile navigation

---

### Task 1.2: Add Invoice E2E Tests

**Files:**
- Create: `frontend/e2e/invoices.spec.ts`

**Step 1: Write invoice creation test**

```typescript
// frontend/e2e/invoices.spec.ts
import { test, expect } from '@playwright/test';

test.describe('Invoices', () => {
  test.beforeEach(async ({ page }) => {
    // Login and navigate to invoices
    await page.goto('/login');
    await page.fill('[data-testid="email"]', 'test@example.com');
    await page.fill('[data-testid="password"]', 'password123');
    await page.click('[data-testid="login-button"]');
    await page.waitForURL('/dashboard');
    await page.click('[data-testid="nav-invoices"]');
  });

  test('can create a new invoice', async ({ page }) => {
    await page.click('[data-testid="new-invoice-button"]');
    await page.fill('[data-testid="invoice-contact"]', 'Test Customer');
    await page.fill('[data-testid="invoice-description"]', 'Test Service');
    await page.fill('[data-testid="invoice-amount"]', '100.00');
    await page.click('[data-testid="save-invoice"]');

    await expect(page.locator('[data-testid="invoice-success"]')).toBeVisible();
  });

  test('can view invoice list', async ({ page }) => {
    await expect(page.locator('[data-testid="invoices-table"]')).toBeVisible();
  });

  test('can filter invoices by status', async ({ page }) => {
    await page.click('[data-testid="filter-draft"]');
    await expect(page.locator('[data-testid="invoice-row"]').first()).toContainText('DRAFT');
  });
});
```

**Step 2: Run new test**

Run: `cd frontend && npx playwright test invoices.spec.ts --project=chromium`
Expected: Test runs (may fail if selectors need adjustment)

**Step 3: Adjust selectors as needed**

Review actual DOM and update data-testid attributes in components.

**Step 4: Commit**

```bash
git add frontend/e2e/invoices.spec.ts
git commit -m "test(e2e): add invoice CRUD tests"
```

---

### Task 1.3: Add Contacts E2E Tests

**Files:**
- Create: `frontend/e2e/contacts.spec.ts`

**Step 1: Write contacts tests**

```typescript
// frontend/e2e/contacts.spec.ts
import { test, expect } from '@playwright/test';

test.describe('Contacts', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/login');
    await page.fill('[data-testid="email"]', 'test@example.com');
    await page.fill('[data-testid="password"]', 'password123');
    await page.click('[data-testid="login-button"]');
    await page.waitForURL('/dashboard');
    await page.click('[data-testid="nav-contacts"]');
  });

  test('can create a new contact', async ({ page }) => {
    await page.click('[data-testid="new-contact-button"]');
    await page.fill('[data-testid="contact-name"]', 'Test Company OÜ');
    await page.fill('[data-testid="contact-email"]', 'test@company.ee');
    await page.selectOption('[data-testid="contact-type"]', 'CUSTOMER');
    await page.click('[data-testid="save-contact"]');

    await expect(page.locator('[data-testid="contact-success"]')).toBeVisible();
  });

  test('can search contacts', async ({ page }) => {
    await page.fill('[data-testid="contact-search"]', 'Test');
    await expect(page.locator('[data-testid="contact-row"]')).toContainText('Test');
  });

  test('can edit a contact', async ({ page }) => {
    await page.click('[data-testid="contact-row"]');
    await page.click('[data-testid="edit-contact"]');
    await page.fill('[data-testid="contact-name"]', 'Updated Company OÜ');
    await page.click('[data-testid="save-contact"]');

    await expect(page.locator('[data-testid="contact-success"]')).toBeVisible();
  });
});
```

**Step 2: Run and adjust**

Run: `cd frontend && npx playwright test contacts.spec.ts --project=chromium`

**Step 3: Commit**

```bash
git add frontend/e2e/contacts.spec.ts
git commit -m "test(e2e): add contact management tests"
```

---

### Task 1.4: Add Reports E2E Tests

**Files:**
- Create: `frontend/e2e/reports.spec.ts`

**Step 1: Write reports tests**

```typescript
// frontend/e2e/reports.spec.ts
import { test, expect } from '@playwright/test';

test.describe('Reports', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/login');
    await page.fill('[data-testid="email"]', 'test@example.com');
    await page.fill('[data-testid="password"]', 'password123');
    await page.click('[data-testid="login-button"]');
    await page.waitForURL('/dashboard');
    await page.click('[data-testid="nav-reports"]');
  });

  test('can view trial balance', async ({ page }) => {
    await page.click('[data-testid="report-trial-balance"]');
    await expect(page.locator('[data-testid="report-title"]')).toContainText('Trial Balance');
    await expect(page.locator('[data-testid="report-table"]')).toBeVisible();
  });

  test('can view balance sheet', async ({ page }) => {
    await page.click('[data-testid="report-balance-sheet"]');
    await expect(page.locator('[data-testid="report-title"]')).toContainText('Balance Sheet');
  });

  test('can view income statement', async ({ page }) => {
    await page.click('[data-testid="report-income-statement"]');
    await expect(page.locator('[data-testid="report-title"]')).toContainText('Income Statement');
  });

  test('can filter by date range', async ({ page }) => {
    await page.click('[data-testid="report-trial-balance"]');
    await page.fill('[data-testid="date-from"]', '2025-01-01');
    await page.fill('[data-testid="date-to"]', '2025-12-31');
    await page.click('[data-testid="apply-filter"]');

    await expect(page.locator('[data-testid="report-table"]')).toBeVisible();
  });
});
```

**Step 2: Run and commit**

```bash
cd frontend && npx playwright test reports.spec.ts --project=chromium
git add frontend/e2e/reports.spec.ts
git commit -m "test(e2e): add financial reports tests"
```

---

### Task 1.5: Add Mobile E2E Tests

**Files:**
- Create: `frontend/e2e/mobile.spec.ts`

**Step 1: Write mobile viewport tests**

```typescript
// frontend/e2e/mobile.spec.ts
import { test, expect, devices } from '@playwright/test';

test.describe('Mobile Responsiveness', () => {
  test.use({ ...devices['iPhone 12'] });

  test('navigation menu works on mobile', async ({ page }) => {
    await page.goto('/dashboard');

    // Should see hamburger menu
    await expect(page.locator('[data-testid="mobile-menu-button"]')).toBeVisible();

    // Click to open
    await page.click('[data-testid="mobile-menu-button"]');
    await expect(page.locator('[data-testid="mobile-nav"]')).toBeVisible();

    // Can navigate
    await page.click('[data-testid="nav-invoices"]');
    await expect(page).toHaveURL(/\/invoices/);
  });

  test('tables are scrollable on mobile', async ({ page }) => {
    await page.goto('/invoices');

    const table = page.locator('[data-testid="invoices-table"]');
    await expect(table).toBeVisible();

    // Should be horizontally scrollable or card view
    const tableStyles = await table.evaluate((el) => {
      const styles = window.getComputedStyle(el);
      return { overflowX: styles.overflowX };
    });

    expect(['auto', 'scroll']).toContain(tableStyles.overflowX);
  });

  test('forms are usable on mobile', async ({ page }) => {
    await page.goto('/contacts/new');

    // Check input sizes are touch-friendly
    const input = page.locator('[data-testid="contact-name"]');
    const box = await input.boundingBox();
    expect(box?.height).toBeGreaterThanOrEqual(44);
  });
});
```

**Step 2: Run mobile tests**

Run: `cd frontend && npx playwright test mobile.spec.ts`

**Step 3: Commit**

```bash
git add frontend/e2e/mobile.spec.ts
git commit -m "test(e2e): add mobile responsiveness tests"
```

---

### Task 1.6: Run Full E2E Suite and Fix Issues

**Step 1: Run complete E2E suite**

Run: `cd frontend && npx playwright test --project=chromium`

**Step 2: Fix any failures**

Review playwright-report for failures and fix issues.

**Step 3: Verify CI passes**

```bash
git push origin feature/e2e-completion
gh pr checks
```

**Step 4: Final commit**

```bash
git add -A
git commit -m "test(e2e): complete E2E test coverage for core flows"
```

---

## Feature 2: Dashboard Improvements

**Priority:** High | **Effort:** ~4 days

### Task 2.1: Create PeriodSelector Component

**Files:**
- Create: `frontend/src/lib/components/PeriodSelector.svelte`
- Test: `frontend/src/tests/components/PeriodSelector.test.ts`

**Step 1: Write the component**

```svelte
<!-- frontend/src/lib/components/PeriodSelector.svelte -->
<script lang="ts">
  import * as m from '$lib/paraglide/messages.js';

  type Period = 'THIS_MONTH' | 'LAST_MONTH' | 'THIS_QUARTER' | 'THIS_YEAR' | 'CUSTOM';

  interface Props {
    value?: Period;
    startDate?: string;
    endDate?: string;
    onchange?: (period: Period, start: string, end: string) => void;
  }

  let { value = $bindable('THIS_MONTH'), startDate = $bindable(''), endDate = $bindable(''), onchange }: Props = $props();

  let showCustom = $derived(value === 'CUSTOM');

  function calculateDates(period: Period): { start: string; end: string } {
    const now = new Date();
    const year = now.getFullYear();
    const month = now.getMonth();

    switch (period) {
      case 'THIS_MONTH':
        return {
          start: new Date(year, month, 1).toISOString().slice(0, 10),
          end: new Date(year, month + 1, 0).toISOString().slice(0, 10)
        };
      case 'LAST_MONTH':
        return {
          start: new Date(year, month - 1, 1).toISOString().slice(0, 10),
          end: new Date(year, month, 0).toISOString().slice(0, 10)
        };
      case 'THIS_QUARTER':
        const quarterStart = Math.floor(month / 3) * 3;
        return {
          start: new Date(year, quarterStart, 1).toISOString().slice(0, 10),
          end: new Date(year, quarterStart + 3, 0).toISOString().slice(0, 10)
        };
      case 'THIS_YEAR':
        return {
          start: new Date(year, 0, 1).toISOString().slice(0, 10),
          end: new Date(year, 11, 31).toISOString().slice(0, 10)
        };
      default:
        return { start: startDate, end: endDate };
    }
  }

  function handlePeriodChange(newPeriod: Period) {
    value = newPeriod;
    if (newPeriod !== 'CUSTOM') {
      const dates = calculateDates(newPeriod);
      startDate = dates.start;
      endDate = dates.end;
    }
    onchange?.(value, startDate, endDate);
  }

  function handleDateChange() {
    onchange?.(value, startDate, endDate);
  }
</script>

<div class="period-selector" data-testid="period-selector">
  <select
    bind:value
    onchange={(e) => handlePeriodChange(e.currentTarget.value as Period)}
    class="period-select"
    data-testid="period-select"
  >
    <option value="THIS_MONTH">{m.dashboard_thisMonth()}</option>
    <option value="LAST_MONTH">{m.dashboard_lastMonth()}</option>
    <option value="THIS_QUARTER">{m.dashboard_thisQuarter()}</option>
    <option value="THIS_YEAR">{m.dashboard_thisYear()}</option>
    <option value="CUSTOM">{m.dashboard_custom()}</option>
  </select>

  {#if showCustom}
    <div class="custom-dates" data-testid="custom-dates">
      <input
        type="date"
        bind:value={startDate}
        onchange={handleDateChange}
        class="date-input"
        data-testid="date-start"
      />
      <span class="date-separator">—</span>
      <input
        type="date"
        bind:value={endDate}
        onchange={handleDateChange}
        class="date-input"
        data-testid="date-end"
      />
    </div>
  {/if}
</div>

<style>
  .period-selector {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    flex-wrap: wrap;
  }

  .period-select {
    padding: 0.5rem 1rem;
    border: 1px solid var(--color-border);
    border-radius: 0.5rem;
    background: var(--color-surface);
    font-size: 0.875rem;
  }

  .custom-dates {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .date-input {
    padding: 0.5rem;
    border: 1px solid var(--color-border);
    border-radius: 0.5rem;
    font-size: 0.875rem;
  }

  .date-separator {
    color: var(--color-text-muted);
  }
</style>
```

**Step 2: Add i18n keys**

Add to `frontend/messages/en.json`:
```json
{
  "dashboard_thisMonth": "This Month",
  "dashboard_lastMonth": "Last Month",
  "dashboard_thisQuarter": "This Quarter",
  "dashboard_thisYear": "This Year",
  "dashboard_custom": "Custom Range"
}
```

Add to `frontend/messages/et.json`:
```json
{
  "dashboard_thisMonth": "See kuu",
  "dashboard_lastMonth": "Eelmine kuu",
  "dashboard_thisQuarter": "See kvartal",
  "dashboard_thisYear": "See aasta",
  "dashboard_custom": "Kohandatud"
}
```

**Step 3: Build paraglide**

Run: `cd frontend && npm run paraglide`

**Step 4: Commit**

```bash
git add frontend/src/lib/components/PeriodSelector.svelte frontend/messages/*.json
git commit -m "feat(frontend): add PeriodSelector component"
```

---

### Task 2.2: Add Analytics API Endpoints

**Files:**
- Create: `internal/analytics/cashflow.go`
- Create: `internal/analytics/activity.go`
- Modify: `cmd/api/handlers_analytics.go`

**Step 1: Create cashflow service**

```go
// internal/analytics/cashflow.go
package analytics

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

type CashFlowPoint struct {
	Date    string          `json:"date"`
	Inflow  decimal.Decimal `json:"inflow"`
	Outflow decimal.Decimal `json:"outflow"`
	Balance decimal.Decimal `json:"balance"`
}

type CashFlowResponse struct {
	Points       []CashFlowPoint `json:"points"`
	TotalInflow  decimal.Decimal `json:"total_inflow"`
	TotalOutflow decimal.Decimal `json:"total_outflow"`
	NetChange    decimal.Decimal `json:"net_change"`
}

func (s *Service) GetCashFlow(ctx context.Context, schemaName, tenantID string, startDate, endDate time.Time) (*CashFlowResponse, error) {
	query := fmt.Sprintf(`
		SELECT
			DATE_TRUNC('day', bt.transaction_date) as date,
			SUM(CASE WHEN bt.amount > 0 THEN bt.amount ELSE 0 END) as inflow,
			SUM(CASE WHEN bt.amount < 0 THEN ABS(bt.amount) ELSE 0 END) as outflow
		FROM %s.bank_transactions bt
		JOIN %s.bank_accounts ba ON bt.bank_account_id = ba.id
		WHERE ba.tenant_id = $1
			AND bt.transaction_date >= $2
			AND bt.transaction_date <= $3
		GROUP BY DATE_TRUNC('day', bt.transaction_date)
		ORDER BY date
	`, schemaName, schemaName)

	rows, err := s.db.Query(ctx, query, tenantID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("query cash flow: %w", err)
	}
	defer rows.Close()

	var points []CashFlowPoint
	var totalInflow, totalOutflow decimal.Decimal
	var runningBalance decimal.Decimal

	for rows.Next() {
		var point CashFlowPoint
		var date time.Time
		if err := rows.Scan(&date, &point.Inflow, &point.Outflow); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		point.Date = date.Format("2006-01-02")
		runningBalance = runningBalance.Add(point.Inflow).Sub(point.Outflow)
		point.Balance = runningBalance

		totalInflow = totalInflow.Add(point.Inflow)
		totalOutflow = totalOutflow.Add(point.Outflow)
		points = append(points, point)
	}

	return &CashFlowResponse{
		Points:       points,
		TotalInflow:  totalInflow,
		TotalOutflow: totalOutflow,
		NetChange:    totalInflow.Sub(totalOutflow),
	}, nil
}
```

**Step 2: Create activity feed service**

```go
// internal/analytics/activity.go
package analytics

import (
	"context"
	"fmt"
	"time"
)

type ActivityItem struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"` // INVOICE, PAYMENT, ENTRY, CONTACT
	Action      string    `json:"action"` // CREATED, UPDATED, POSTED, PAID
	Description string    `json:"description"`
	Amount      *string   `json:"amount,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

func (s *Service) GetRecentActivity(ctx context.Context, schemaName, tenantID string, limit int) ([]ActivityItem, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	query := fmt.Sprintf(`
		(SELECT
			id, 'INVOICE' as type,
			CASE status WHEN 'POSTED' THEN 'POSTED' ELSE 'CREATED' END as action,
			'Invoice ' || invoice_number as description,
			total::text as amount,
			created_at
		FROM %s.invoices WHERE tenant_id = $1
		ORDER BY created_at DESC LIMIT $2)

		UNION ALL

		(SELECT
			id, 'PAYMENT' as type, 'CREATED' as action,
			'Payment received' as description,
			amount::text as amount,
			created_at
		FROM %s.payments WHERE tenant_id = $1
		ORDER BY created_at DESC LIMIT $2)

		UNION ALL

		(SELECT
			id, 'ENTRY' as type,
			CASE status WHEN 'POSTED' THEN 'POSTED' ELSE 'CREATED' END as action,
			'Journal entry ' || entry_number as description,
			NULL as amount,
			created_at
		FROM %s.journal_entries WHERE tenant_id = $1
		ORDER BY created_at DESC LIMIT $2)

		ORDER BY created_at DESC
		LIMIT $2
	`, schemaName, schemaName, schemaName)

	rows, err := s.db.Query(ctx, query, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("query activity: %w", err)
	}
	defer rows.Close()

	var items []ActivityItem
	for rows.Next() {
		var item ActivityItem
		if err := rows.Scan(&item.ID, &item.Type, &item.Action, &item.Description, &item.Amount, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		items = append(items, item)
	}

	return items, nil
}
```

**Step 3: Add API handlers**

```go
// In cmd/api/handlers_analytics.go - add handlers:

func (h *Handlers) HandleGetCashFlow(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")

	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")

	start, _ := time.Parse("2006-01-02", startDate)
	end, _ := time.Parse("2006-01-02", endDate)

	schemaName, err := h.tenantService.GetSchemaName(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Tenant not found")
		return
	}

	result, err := h.analyticsService.GetCashFlow(r.Context(), schemaName, tenantID, start, end)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

func (h *Handlers) HandleGetActivity(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	schemaName, err := h.tenantService.GetSchemaName(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Tenant not found")
		return
	}

	items, err := h.analyticsService.GetRecentActivity(r.Context(), schemaName, tenantID, limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, items)
}
```

**Step 4: Add routes**

```go
// In cmd/api/main.go - add routes:
r.Get("/tenants/{tenantID}/analytics/cashflow", h.HandleGetCashFlow)
r.Get("/tenants/{tenantID}/analytics/activity", h.HandleGetActivity)
```

**Step 5: Test and commit**

```bash
go test ./internal/analytics/... -v
go build ./...
git add internal/analytics/*.go cmd/api/handlers_analytics.go
git commit -m "feat(api): add cashflow and activity analytics endpoints"
```

---

### Task 2.3: Create ActivityFeed Component

**Files:**
- Create: `frontend/src/lib/components/ActivityFeed.svelte`

**Step 1: Write the component**

```svelte
<!-- frontend/src/lib/components/ActivityFeed.svelte -->
<script lang="ts">
  import * as m from '$lib/paraglide/messages.js';
  import { formatDistanceToNow } from 'date-fns';

  interface ActivityItem {
    id: string;
    type: 'INVOICE' | 'PAYMENT' | 'ENTRY' | 'CONTACT';
    action: string;
    description: string;
    amount?: string;
    created_at: string;
  }

  interface Props {
    items: ActivityItem[];
    loading?: boolean;
  }

  let { items = [], loading = false }: Props = $props();

  function getIcon(type: string): string {
    switch (type) {
      case 'INVOICE': return '📄';
      case 'PAYMENT': return '💰';
      case 'ENTRY': return '📝';
      case 'CONTACT': return '👤';
      default: return '📌';
    }
  }

  function formatTime(dateStr: string): string {
    return formatDistanceToNow(new Date(dateStr), { addSuffix: true });
  }

  function formatAmount(amount: string | undefined): string {
    if (!amount) return '';
    return new Intl.NumberFormat('et-EE', {
      style: 'currency',
      currency: 'EUR'
    }).format(parseFloat(amount));
  }
</script>

<div class="activity-feed" data-testid="activity-feed">
  <h3 class="feed-title">{m.dashboard_recentActivity()}</h3>

  {#if loading}
    <div class="loading">
      <div class="spinner"></div>
    </div>
  {:else if items.length === 0}
    <p class="empty">{m.dashboard_noRecentActivity()}</p>
  {:else}
    <ul class="activity-list">
      {#each items as item}
        <li class="activity-item" data-testid="activity-item">
          <span class="activity-icon">{getIcon(item.type)}</span>
          <div class="activity-content">
            <p class="activity-description">{item.description}</p>
            <div class="activity-meta">
              <span class="activity-time">{formatTime(item.created_at)}</span>
              {#if item.amount}
                <span class="activity-amount">{formatAmount(item.amount)}</span>
              {/if}
            </div>
          </div>
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .activity-feed {
    background: var(--color-surface);
    border-radius: 0.5rem;
    padding: 1rem;
  }

  .feed-title {
    font-size: 1rem;
    font-weight: 600;
    margin-bottom: 1rem;
    color: var(--color-text);
  }

  .activity-list {
    list-style: none;
    padding: 0;
    margin: 0;
  }

  .activity-item {
    display: flex;
    gap: 0.75rem;
    padding: 0.75rem 0;
    border-bottom: 1px solid var(--color-border);
  }

  .activity-item:last-child {
    border-bottom: none;
  }

  .activity-icon {
    font-size: 1.25rem;
  }

  .activity-content {
    flex: 1;
  }

  .activity-description {
    margin: 0;
    font-size: 0.875rem;
    color: var(--color-text);
  }

  .activity-meta {
    display: flex;
    justify-content: space-between;
    margin-top: 0.25rem;
    font-size: 0.75rem;
    color: var(--color-text-muted);
  }

  .activity-amount {
    font-weight: 500;
    color: var(--color-success);
  }

  .loading {
    display: flex;
    justify-content: center;
    padding: 2rem;
  }

  .spinner {
    width: 1.5rem;
    height: 1.5rem;
    border: 2px solid var(--color-border);
    border-top-color: var(--color-primary);
    border-radius: 50%;
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .empty {
    text-align: center;
    color: var(--color-text-muted);
    padding: 2rem;
  }
</style>
```

**Step 2: Add i18n keys**

```json
// en.json
{
  "dashboard_recentActivity": "Recent Activity",
  "dashboard_noRecentActivity": "No recent activity"
}

// et.json
{
  "dashboard_recentActivity": "Viimased tegevused",
  "dashboard_noRecentActivity": "Viimased tegevused puuduvad"
}
```

**Step 3: Commit**

```bash
git add frontend/src/lib/components/ActivityFeed.svelte frontend/messages/*.json
git commit -m "feat(frontend): add ActivityFeed component"
```

---

### Task 2.4: Update Dashboard with New Components

**Files:**
- Modify: `frontend/src/routes/dashboard/+page.svelte`

**Step 1: Integrate PeriodSelector and ActivityFeed**

Update the dashboard to use the new components and fetch analytics data.

**Step 2: Add cash flow chart**

Use Chart.js to render the cash flow line chart.

**Step 3: Test dashboard**

Run: `cd frontend && npm run dev`
Verify: Period selector works, activity feed displays, cash flow chart renders

**Step 4: Commit**

```bash
git add frontend/src/routes/dashboard/+page.svelte
git commit -m "feat(dashboard): integrate period selector, activity feed, and cash flow chart"
```

---

## Feature 3: Report Export (PDF/Excel)

**Priority:** High | **Effort:** ~5 days

### Task 3.1: Install xlsx Library

**Step 1: Add dependency**

Run: `cd frontend && npm install xlsx`

**Step 2: Commit**

```bash
git add frontend/package.json frontend/package-lock.json
git commit -m "chore(deps): add xlsx library for Excel export"
```

---

### Task 3.2: Create ExportButton Component

**Files:**
- Create: `frontend/src/lib/components/ExportButton.svelte`

**Step 1: Write the component**

```svelte
<!-- frontend/src/lib/components/ExportButton.svelte -->
<script lang="ts">
  import * as m from '$lib/paraglide/messages.js';
  import * as XLSX from 'xlsx';

  type ExportFormat = 'pdf' | 'excel' | 'csv';

  interface Props {
    data: Record<string, unknown>[];
    columns: { key: string; label: string }[];
    filename?: string;
    title?: string;
  }

  let { data, columns, filename = 'export', title = 'Report' }: Props = $props();
  let showMenu = $state(false);

  function exportToExcel() {
    const ws = XLSX.utils.json_to_sheet(
      data.map(row => {
        const obj: Record<string, unknown> = {};
        columns.forEach(col => {
          obj[col.label] = row[col.key];
        });
        return obj;
      })
    );
    const wb = XLSX.utils.book_new();
    XLSX.utils.book_append_sheet(wb, ws, 'Report');
    XLSX.writeFile(wb, `${filename}.xlsx`);
    showMenu = false;
  }

  function exportToCsv() {
    const headers = columns.map(c => c.label).join(',');
    const rows = data.map(row =>
      columns.map(col => {
        const val = row[col.key];
        if (typeof val === 'string' && val.includes(',')) {
          return `"${val}"`;
        }
        return val;
      }).join(',')
    );

    const csv = [headers, ...rows].join('\n');
    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${filename}.csv`;
    a.click();
    URL.revokeObjectURL(url);
    showMenu = false;
  }

  function exportToPdf() {
    window.print();
    showMenu = false;
  }
</script>

<div class="export-wrapper" data-testid="export-button">
  <button
    class="export-btn"
    onclick={() => showMenu = !showMenu}
    aria-expanded={showMenu}
  >
    {m.reports_export()}
    <svg class="icon" viewBox="0 0 20 20" fill="currentColor">
      <path fill-rule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clip-rule="evenodd" />
    </svg>
  </button>

  {#if showMenu}
    <div class="export-menu" role="menu">
      <button class="menu-item" onclick={exportToPdf} role="menuitem">
        📄 {m.reports_exportPdf()}
      </button>
      <button class="menu-item" onclick={exportToExcel} role="menuitem">
        📊 {m.reports_exportExcel()}
      </button>
      <button class="menu-item" onclick={exportToCsv} role="menuitem">
        📋 {m.reports_exportCsv()}
      </button>
    </div>
  {/if}
</div>

<style>
  .export-wrapper {
    position: relative;
    display: inline-block;
  }

  .export-btn {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.5rem 1rem;
    background: var(--color-primary);
    color: white;
    border: none;
    border-radius: 0.5rem;
    cursor: pointer;
    font-size: 0.875rem;
  }

  .export-btn:hover {
    background: var(--color-primary-dark);
  }

  .icon {
    width: 1rem;
    height: 1rem;
  }

  .export-menu {
    position: absolute;
    top: 100%;
    right: 0;
    margin-top: 0.25rem;
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: 0.5rem;
    box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
    min-width: 150px;
    z-index: 10;
  }

  .menu-item {
    display: block;
    width: 100%;
    padding: 0.75rem 1rem;
    text-align: left;
    background: none;
    border: none;
    cursor: pointer;
    font-size: 0.875rem;
  }

  .menu-item:hover {
    background: var(--color-hover);
  }

  .menu-item:first-child {
    border-radius: 0.5rem 0.5rem 0 0;
  }

  .menu-item:last-child {
    border-radius: 0 0 0.5rem 0.5rem;
  }

  @media print {
    .export-wrapper {
      display: none;
    }
  }
</style>
```

**Step 2: Add i18n keys**

```json
// en.json
{
  "reports_export": "Export",
  "reports_exportPdf": "Export as PDF",
  "reports_exportExcel": "Export as Excel",
  "reports_exportCsv": "Export as CSV"
}

// et.json
{
  "reports_export": "Ekspordi",
  "reports_exportPdf": "Ekspordi PDF-ina",
  "reports_exportExcel": "Ekspordi Excelina",
  "reports_exportCsv": "Ekspordi CSV-na"
}
```

**Step 3: Commit**

```bash
git add frontend/src/lib/components/ExportButton.svelte frontend/messages/*.json
git commit -m "feat(frontend): add ExportButton component with PDF/Excel/CSV support"
```

---

### Task 3.3: Add Export to Report Pages

**Files:**
- Modify: `frontend/src/routes/reports/+page.svelte`
- Modify: Report sub-pages (trial-balance, balance-sheet, income-statement)

**Step 1: Import and use ExportButton**

Add the ExportButton to each report page with appropriate data.

**Step 2: Test exports**

- Verify Excel opens in spreadsheet app
- Verify CSV downloads correctly
- Verify PDF opens print dialog

**Step 3: Commit**

```bash
git add frontend/src/routes/reports/*.svelte
git commit -m "feat(reports): add export functionality to all report pages"
```

---

## Feature 4: Mobile Responsiveness

**Priority:** Medium | **Effort:** ~3 days

### Task 4.1: Create MobileNav Component

**Files:**
- Create: `frontend/src/lib/components/MobileNav.svelte`

**Step 1: Write hamburger menu component**

```svelte
<!-- frontend/src/lib/components/MobileNav.svelte -->
<script lang="ts">
  import * as m from '$lib/paraglide/messages.js';
  import { page } from '$app/stores';

  interface Props {
    links: { href: string; label: string; icon?: string }[];
  }

  let { links }: Props = $props();
  let isOpen = $state(false);

  function toggleMenu() {
    isOpen = !isOpen;
  }

  function closeMenu() {
    isOpen = false;
  }

  // Close on navigation
  $effect(() => {
    $page.url;
    isOpen = false;
  });
</script>

<div class="mobile-nav" data-testid="mobile-nav">
  <button
    class="hamburger"
    onclick={toggleMenu}
    aria-label={isOpen ? 'Close menu' : 'Open menu'}
    aria-expanded={isOpen}
    data-testid="mobile-menu-button"
  >
    <span class="hamburger-line" class:open={isOpen}></span>
    <span class="hamburger-line" class:open={isOpen}></span>
    <span class="hamburger-line" class:open={isOpen}></span>
  </button>

  {#if isOpen}
    <div class="overlay" onclick={closeMenu}></div>
    <nav class="drawer" data-testid="mobile-drawer">
      <div class="drawer-header">
        <h2>{m.common_menu()}</h2>
        <button class="close-btn" onclick={closeMenu}>✕</button>
      </div>
      <ul class="nav-list">
        {#each links as link}
          <li>
            <a
              href={link.href}
              class="nav-link"
              class:active={$page.url.pathname === link.href}
              data-testid="nav-{link.href.replace('/', '')}"
            >
              {#if link.icon}
                <span class="nav-icon">{link.icon}</span>
              {/if}
              {link.label}
            </a>
          </li>
        {/each}
      </ul>
    </nav>
  {/if}
</div>

<style>
  .mobile-nav {
    display: none;
  }

  @media (max-width: 768px) {
    .mobile-nav {
      display: block;
    }
  }

  .hamburger {
    display: flex;
    flex-direction: column;
    gap: 5px;
    background: none;
    border: none;
    padding: 0.5rem;
    cursor: pointer;
  }

  .hamburger-line {
    width: 24px;
    height: 2px;
    background: var(--color-text);
    transition: all 0.3s ease;
  }

  .hamburger-line.open:nth-child(1) {
    transform: rotate(45deg) translate(5px, 5px);
  }

  .hamburger-line.open:nth-child(2) {
    opacity: 0;
  }

  .hamburger-line.open:nth-child(3) {
    transform: rotate(-45deg) translate(5px, -5px);
  }

  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    z-index: 40;
  }

  .drawer {
    position: fixed;
    top: 0;
    left: 0;
    bottom: 0;
    width: 280px;
    background: var(--color-surface);
    z-index: 50;
    animation: slideIn 0.3s ease;
    overflow-y: auto;
  }

  @keyframes slideIn {
    from { transform: translateX(-100%); }
    to { transform: translateX(0); }
  }

  .drawer-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 1rem;
    border-bottom: 1px solid var(--color-border);
  }

  .drawer-header h2 {
    font-size: 1.125rem;
    font-weight: 600;
    margin: 0;
  }

  .close-btn {
    background: none;
    border: none;
    font-size: 1.25rem;
    cursor: pointer;
    padding: 0.25rem;
    color: var(--color-text-muted);
  }

  .nav-list {
    list-style: none;
    padding: 0;
    margin: 0;
  }

  .nav-link {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 1rem;
    text-decoration: none;
    color: var(--color-text);
    border-bottom: 1px solid var(--color-border);
    min-height: 48px; /* Touch target */
  }

  .nav-link:hover,
  .nav-link.active {
    background: var(--color-hover);
    color: var(--color-primary);
  }

  .nav-icon {
    font-size: 1.25rem;
  }
</style>
```

**Step 2: Add i18n keys**

```json
// en.json
{ "common_menu": "Menu" }

// et.json
{ "common_menu": "Menüü" }
```

**Step 3: Commit**

```bash
git add frontend/src/lib/components/MobileNav.svelte frontend/messages/*.json
git commit -m "feat(frontend): add MobileNav component with hamburger menu"
```

---

### Task 4.2: Create Viewport Store

**Files:**
- Create: `frontend/src/lib/stores/viewport.ts`

**Step 1: Write viewport store**

```typescript
// frontend/src/lib/stores/viewport.ts
import { browser } from '$app/environment';
import { readable } from 'svelte/store';

export type Breakpoint = 'mobile' | 'tablet' | 'desktop' | 'wide';

interface ViewportState {
  width: number;
  height: number;
  breakpoint: Breakpoint;
  isMobile: boolean;
  isTablet: boolean;
  isDesktop: boolean;
}

function getBreakpoint(width: number): Breakpoint {
  if (width < 480) return 'mobile';
  if (width < 768) return 'tablet';
  if (width < 1280) return 'desktop';
  return 'wide';
}

function getState(width: number, height: number): ViewportState {
  const breakpoint = getBreakpoint(width);
  return {
    width,
    height,
    breakpoint,
    isMobile: breakpoint === 'mobile',
    isTablet: breakpoint === 'tablet',
    isDesktop: breakpoint === 'desktop' || breakpoint === 'wide'
  };
}

export const viewport = readable<ViewportState>(
  getState(browser ? window.innerWidth : 1024, browser ? window.innerHeight : 768),
  (set) => {
    if (!browser) return;

    function update() {
      set(getState(window.innerWidth, window.innerHeight));
    }

    window.addEventListener('resize', update);
    return () => window.removeEventListener('resize', update);
  }
);
```

**Step 2: Commit**

```bash
git add frontend/src/lib/stores/viewport.ts
git commit -m "feat(frontend): add viewport store for responsive breakpoints"
```

---

### Task 4.3: Add Mobile CSS Utilities

**Files:**
- Modify: `frontend/src/app.css`

**Step 1: Add mobile utilities**

```css
/* Add to frontend/src/app.css */

/* Mobile visibility utilities */
.mobile-only {
  display: none;
}

.desktop-only {
  display: block;
}

@media (max-width: 768px) {
  .mobile-only {
    display: block;
  }

  .desktop-only {
    display: none;
  }

  /* Mobile table as cards */
  .table-mobile-cards tbody {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .table-mobile-cards thead {
    display: none;
  }

  .table-mobile-cards tr {
    display: flex;
    flex-direction: column;
    background: var(--color-surface);
    border-radius: 0.5rem;
    padding: 1rem;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  }

  .table-mobile-cards td {
    display: flex;
    justify-content: space-between;
    padding: 0.5rem 0;
    border-bottom: 1px solid var(--color-border);
  }

  .table-mobile-cards td:last-child {
    border-bottom: none;
  }

  .table-mobile-cards td::before {
    content: attr(data-label);
    font-weight: 600;
    color: var(--color-text-muted);
  }

  /* Touch-friendly inputs */
  input, select, textarea, button {
    min-height: 44px;
  }

  /* Full-width forms */
  .form-group input,
  .form-group select,
  .form-group textarea {
    width: 100%;
  }

  /* Stack form rows */
  .form-row {
    flex-direction: column;
  }

  /* Main content padding for bottom nav */
  .main-content {
    padding-bottom: 70px;
  }

  /* Modal fullscreen on mobile */
  .modal-content {
    position: fixed;
    inset: 0;
    max-width: none;
    max-height: none;
    border-radius: 0;
  }
}
```

**Step 2: Commit**

```bash
git add frontend/src/app.css
git commit -m "style(css): add mobile responsiveness utilities"
```

---

### Task 4.4: Integrate MobileNav in Layout

**Files:**
- Modify: `frontend/src/routes/+layout.svelte`

**Step 1: Add MobileNav to layout**

Import and use MobileNav component with navigation links.

**Step 2: Test on mobile viewport**

Run: `cd frontend && npm run dev`
Open DevTools and test at 375px width.

**Step 3: Commit**

```bash
git add frontend/src/routes/+layout.svelte
git commit -m "feat(layout): integrate MobileNav for mobile responsiveness"
```

---

### Task 4.5: Audit and Fix Mobile Issues

**Step 1: Test each route on mobile**

Routes to test:
- [ ] Dashboard
- [ ] Invoices
- [ ] Contacts
- [ ] Accounts
- [ ] Reports
- [ ] Settings

**Step 2: Fix identified issues**

Apply mobile CSS classes and adjustments.

**Step 3: Run mobile E2E tests**

Run: `cd frontend && npx playwright test mobile.spec.ts`

**Step 4: Final commit**

```bash
git add -A
git commit -m "feat: complete mobile responsiveness for all pages"
```

---

## Summary

| Feature | Tasks | Estimated Effort |
|---------|-------|-----------------|
| E2E Testing Completion | 6 | 2 days |
| Dashboard Improvements | 4 | 4 days |
| Report Export (PDF/Excel) | 3 | 5 days |
| Mobile Responsiveness | 5 | 3 days |

**Total tasks:** 18
**Total estimated effort:** 14 days

## Ralph Loop Execution Order

1. **First:** E2E Testing (establishes test coverage for all subsequent work)
2. **Second:** Dashboard Improvements (highest user visibility)
3. **Third:** Report Export (extends analytics functionality)
4. **Fourth:** Mobile Responsiveness (polish across all features)

## Success Verification

After completing all features, run:

```bash
# Backend tests
go test -race -cover ./...

# Frontend tests
cd frontend && npm test -- --run

# E2E tests
cd frontend && npx playwright test

# Build
go build ./... && cd frontend && npm run build
```

All tests should pass and build should succeed.
