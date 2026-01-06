# Phase 2 Feature Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement 12 high-priority features for Estonian accounting compliance and enhanced functionality

**Architecture:** Go backend with service/repository pattern, Svelte 5 frontend with runes, PostgreSQL multi-tenant schemas, TDD with mock repositories for unit tests and integration tests against real DB

**Tech Stack:** Go 1.21+, Svelte 5, PostgreSQL, Playwright E2E, testify/stretchr

---

## Phase Overview

| Phase | Features | Focus |
|-------|----------|-------|
| Phase 2A | VAT Returns UI, Cash Flow Statement | Estonian Compliance |
| Phase 2B | Leave Management, Salary Calculator | Payroll Enhancement |
| Phase 2C | Balance Confirmations, Payment Reminders | Year-End & Cash Flow |
| Phase 2D | Cost Centers | Reporting Enhancement |

---

## Phase 2A: Estonian Compliance (VAT & Cash Flow)

### Task 1: VAT Returns (KMD) Frontend

**Files:**
- Create: `frontend/src/routes/vat-returns/+page.svelte`
- Modify: `frontend/src/lib/api.ts:1-50` (add KMD types)
- Modify: `frontend/messages/en.json` (add translations)
- Modify: `frontend/messages/et.json` (add translations)
- Test: `frontend/e2e/demo/vat-returns.spec.ts`

**Context:** Backend KMD service exists at `internal/tax/service.go:42-99` with types at `internal/tax/types.go`. API handlers need to be wired to frontend.

**Step 1: Add KMD types to frontend API**

```typescript
// Add to frontend/src/lib/api.ts after other interfaces

export interface KMDRow {
  code: string;
  description: string;
  tax_base: string;
  tax_amount: string;
}

export interface KMDDeclaration {
  id: string;
  tenant_id: string;
  year: number;
  month: number;
  status: string;
  total_output_vat: string;
  total_input_vat: string;
  rows: KMDRow[];
  submitted_at?: string;
  created_at: string;
  updated_at: string;
}

export interface CreateKMDRequest {
  year: number;
  month: number;
}
```

**Step 2: Add KMD API methods**

```typescript
// Add to api object in frontend/src/lib/api.ts

async listKMD(tenantId: string): Promise<KMDDeclaration[]> {
  const response = await this.fetch(`/api/v1/tenants/${tenantId}/tax/kmd`);
  return response.json();
},

async getKMD(tenantId: string, year: number, month: number): Promise<KMDDeclaration> {
  const response = await this.fetch(`/api/v1/tenants/${tenantId}/tax/kmd/${year}/${month}`);
  return response.json();
},

async generateKMD(tenantId: string, req: CreateKMDRequest): Promise<KMDDeclaration> {
  const response = await this.fetch(`/api/v1/tenants/${tenantId}/tax/kmd`, {
    method: 'POST',
    body: JSON.stringify(req),
  });
  return response.json();
},

async exportKMD(tenantId: string, year: number, month: number, format: 'xml' | 'json'): Promise<Blob> {
  const response = await this.fetch(`/api/v1/tenants/${tenantId}/tax/kmd/${year}/${month}/export?format=${format}`);
  return response.blob();
},
```

**Step 3: Add i18n translations (English)**

```json
// Add to frontend/messages/en.json
"vat_title": "VAT Returns",
"vat_declarations": "Declarations",
"vat_generate": "Generate Declaration",
"vat_year": "Year",
"vat_month": "Month",
"vat_period": "Period",
"vat_status": "Status",
"vat_status_draft": "Draft",
"vat_status_submitted": "Submitted",
"vat_status_accepted": "Accepted",
"vat_output_vat": "Output VAT",
"vat_input_vat": "Input VAT",
"vat_payable": "VAT Payable",
"vat_refundable": "VAT Refundable",
"vat_export_xml": "Export XML",
"vat_export_json": "Export JSON",
"vat_row_code": "Code",
"vat_row_description": "Description",
"vat_row_tax_base": "Tax Base",
"vat_row_tax_amount": "Tax Amount",
"vat_kmd_1": "Standard rate (22%)",
"vat_kmd_2": "Reduced rate (9%)",
"vat_kmd_3": "Zero-rated exports",
"vat_kmd_4": "Input VAT",
"vat_submit_to_emta": "Submit to e-MTA",
"vat_no_declarations": "No VAT declarations found",
"vat_generating": "Generating..."
```

**Step 4: Add i18n translations (Estonian)**

```json
// Add to frontend/messages/et.json
"vat_title": "Käibedeklaratsioonid",
"vat_declarations": "Deklaratsioonid",
"vat_generate": "Genereeri deklaratsioon",
"vat_year": "Aasta",
"vat_month": "Kuu",
"vat_period": "Periood",
"vat_status": "Staatus",
"vat_status_draft": "Mustand",
"vat_status_submitted": "Esitatud",
"vat_status_accepted": "Kinnitatud",
"vat_output_vat": "Arvestatud käibemaks",
"vat_input_vat": "Sisendkäibemaks",
"vat_payable": "Tasumisele kuuluv käibemaks",
"vat_refundable": "Tagastatav käibemaks",
"vat_export_xml": "Ekspordi XML",
"vat_export_json": "Ekspordi JSON",
"vat_row_code": "Kood",
"vat_row_description": "Kirjeldus",
"vat_row_tax_base": "Maksustatav summa",
"vat_row_tax_amount": "Käibemaks",
"vat_kmd_1": "Standardmäär (22%)",
"vat_kmd_2": "Vähendatud määr (9%)",
"vat_kmd_3": "Nullmääraga eksport",
"vat_kmd_4": "Sisendkäibemaks",
"vat_submit_to_emta": "Esita e-MTAsse",
"vat_no_declarations": "Käibedeklaratsioone ei leitud",
"vat_generating": "Genereerin..."
```

**Step 5: Create VAT Returns page**

```svelte
<!-- frontend/src/routes/vat-returns/+page.svelte -->
<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { api, type KMDDeclaration } from '$lib/api';
  import Decimal from 'decimal.js';
  import * as m from '$lib/paraglide/messages.js';
  import ExportButton from '$lib/components/ExportButton.svelte';

  let tenantId = $derived($page.url.searchParams.get('tenant') || '');
  let isLoading = $state(false);
  let isGenerating = $state(false);
  let error = $state('');

  let declarations = $state<KMDDeclaration[]>([]);
  let selectedDeclaration = $state<KMDDeclaration | null>(null);

  let selectedYear = $state(new Date().getFullYear());
  let selectedMonth = $state(new Date().getMonth() + 1);

  const months = [
    { value: 1, label: 'January / Jaanuar' },
    { value: 2, label: 'February / Veebruar' },
    { value: 3, label: 'March / Märts' },
    { value: 4, label: 'April / Aprill' },
    { value: 5, label: 'May / Mai' },
    { value: 6, label: 'June / Juuni' },
    { value: 7, label: 'July / Juuli' },
    { value: 8, label: 'August / August' },
    { value: 9, label: 'September / September' },
    { value: 10, label: 'October / Oktoober' },
    { value: 11, label: 'November / November' },
    { value: 12, label: 'December / Detsember' },
  ];

  onMount(() => {
    if (tenantId) {
      loadDeclarations();
    }
  });

  async function loadDeclarations() {
    isLoading = true;
    error = '';
    try {
      declarations = await api.listKMD(tenantId);
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to load declarations';
    } finally {
      isLoading = false;
    }
  }

  async function generateDeclaration() {
    isGenerating = true;
    error = '';
    try {
      const newDecl = await api.generateKMD(tenantId, {
        year: selectedYear,
        month: selectedMonth,
      });
      declarations = [newDecl, ...declarations];
      selectedDeclaration = newDecl;
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to generate declaration';
    } finally {
      isGenerating = false;
    }
  }

  function formatAmount(amount: string | undefined): string {
    if (!amount) return '0.00';
    return new Decimal(amount).toFixed(2);
  }

  function getStatusClass(status: string): string {
    switch (status) {
      case 'SUBMITTED': return 'status-submitted';
      case 'ACCEPTED': return 'status-accepted';
      default: return 'status-draft';
    }
  }

  function getStatusLabel(status: string): string {
    switch (status) {
      case 'SUBMITTED': return m.vat_status_submitted();
      case 'ACCEPTED': return m.vat_status_accepted();
      default: return m.vat_status_draft();
    }
  }

  function calculatePayable(decl: KMDDeclaration): Decimal {
    const output = new Decimal(decl.total_output_vat || 0);
    const input = new Decimal(decl.total_input_vat || 0);
    return output.minus(input);
  }

  async function exportXML() {
    if (!selectedDeclaration) return;
    const blob = await api.exportKMD(tenantId, selectedDeclaration.year, selectedDeclaration.month, 'xml');
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `KMD_${selectedDeclaration.year}_${String(selectedDeclaration.month).padStart(2, '0')}.xml`;
    a.click();
  }
</script>

<svelte:head>
  <title>{m.vat_title()} - Open Accounting</title>
</svelte:head>

<div class="container">
  <div class="header">
    <h1>{m.vat_title()}</h1>
  </div>

  {#if !tenantId}
    <div class="card empty-state">
      <p>Select a tenant from <a href="/dashboard">{m.nav_dashboard()}</a>.</p>
    </div>
  {:else}
    <div class="card generate-section">
      <h2>{m.vat_generate()}</h2>
      <div class="generate-form">
        <div class="form-group">
          <label class="label" for="year">{m.vat_year()}</label>
          <select class="input" id="year" bind:value={selectedYear}>
            {#each [2024, 2025, 2026] as year}
              <option value={year}>{year}</option>
            {/each}
          </select>
        </div>
        <div class="form-group">
          <label class="label" for="month">{m.vat_month()}</label>
          <select class="input" id="month" bind:value={selectedMonth}>
            {#each months as month}
              <option value={month.value}>{month.label}</option>
            {/each}
          </select>
        </div>
        <button class="btn btn-primary" onclick={generateDeclaration} disabled={isGenerating}>
          {isGenerating ? m.vat_generating() : m.vat_generate()}
        </button>
      </div>
    </div>

    {#if error}
      <div class="alert alert-error">{error}</div>
    {/if}

    <div class="declarations-grid">
      <div class="card declarations-list">
        <h2>{m.vat_declarations()}</h2>
        {#if isLoading}
          <p>Loading...</p>
        {:else if declarations.length === 0}
          <p class="empty-message">{m.vat_no_declarations()}</p>
        {:else}
          <table class="table">
            <thead>
              <tr>
                <th>{m.vat_period()}</th>
                <th>{m.vat_status()}</th>
                <th class="amount">{m.vat_payable()}</th>
              </tr>
            </thead>
            <tbody>
              {#each declarations as decl}
                <tr
                  class:selected={selectedDeclaration?.id === decl.id}
                  onclick={() => selectedDeclaration = decl}
                >
                  <td>{decl.year}-{String(decl.month).padStart(2, '0')}</td>
                  <td><span class="status {getStatusClass(decl.status)}">{getStatusLabel(decl.status)}</span></td>
                  <td class="amount">{formatAmount(calculatePayable(decl).toString())} EUR</td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </div>

      {#if selectedDeclaration}
        <div class="card declaration-detail">
          <div class="detail-header">
            <h2>KMD {selectedDeclaration.year}-{String(selectedDeclaration.month).padStart(2, '0')}</h2>
            <div class="detail-actions">
              <button class="btn btn-secondary" onclick={exportXML}>{m.vat_export_xml()}</button>
            </div>
          </div>

          <div class="summary-cards">
            <div class="summary-card">
              <span class="summary-label">{m.vat_output_vat()}</span>
              <span class="summary-value">{formatAmount(selectedDeclaration.total_output_vat)} EUR</span>
            </div>
            <div class="summary-card">
              <span class="summary-label">{m.vat_input_vat()}</span>
              <span class="summary-value">{formatAmount(selectedDeclaration.total_input_vat)} EUR</span>
            </div>
            <div class="summary-card highlight">
              <span class="summary-label">{calculatePayable(selectedDeclaration).greaterThanOrEqualTo(0) ? m.vat_payable() : m.vat_refundable()}</span>
              <span class="summary-value">{formatAmount(calculatePayable(selectedDeclaration).abs().toString())} EUR</span>
            </div>
          </div>

          <h3>KMD Rows</h3>
          <table class="table">
            <thead>
              <tr>
                <th>{m.vat_row_code()}</th>
                <th>{m.vat_row_description()}</th>
                <th class="amount">{m.vat_row_tax_base()}</th>
                <th class="amount">{m.vat_row_tax_amount()}</th>
              </tr>
            </thead>
            <tbody>
              {#each selectedDeclaration.rows as row}
                <tr>
                  <td>{row.code}</td>
                  <td>{row.description}</td>
                  <td class="amount">{formatAmount(row.tax_base)}</td>
                  <td class="amount">{formatAmount(row.tax_amount)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .header {
    margin-bottom: 1.5rem;
  }

  h1 {
    font-size: 1.75rem;
  }

  .generate-section {
    margin-bottom: 1.5rem;
  }

  .generate-section h2 {
    font-size: 1.1rem;
    margin-bottom: 1rem;
  }

  .generate-form {
    display: flex;
    gap: 1rem;
    align-items: flex-end;
    flex-wrap: wrap;
  }

  .generate-form .form-group {
    flex: 1;
    min-width: 150px;
    max-width: 200px;
  }

  .declarations-grid {
    display: grid;
    grid-template-columns: 1fr 2fr;
    gap: 1.5rem;
  }

  .declarations-list h2,
  .declaration-detail h2 {
    font-size: 1.1rem;
    margin-bottom: 1rem;
  }

  .declaration-detail h3 {
    font-size: 1rem;
    margin: 1.5rem 0 1rem;
  }

  .detail-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem;
  }

  .detail-actions {
    display: flex;
    gap: 0.5rem;
  }

  .summary-cards {
    display: flex;
    gap: 1rem;
    margin-bottom: 1.5rem;
  }

  .summary-card {
    flex: 1;
    padding: 1rem;
    background: var(--color-bg);
    border-radius: 8px;
    text-align: center;
  }

  .summary-card.highlight {
    background: var(--color-primary);
    color: white;
  }

  .summary-label {
    display: block;
    font-size: 0.75rem;
    text-transform: uppercase;
    margin-bottom: 0.5rem;
    opacity: 0.8;
  }

  .summary-value {
    display: block;
    font-size: 1.25rem;
    font-weight: 600;
  }

  .table {
    width: 100%;
    border-collapse: collapse;
  }

  .table th,
  .table td {
    padding: 0.75rem;
    text-align: left;
    border-bottom: 1px solid var(--color-border);
  }

  .table th {
    font-weight: 600;
    font-size: 0.75rem;
    text-transform: uppercase;
    color: var(--color-text-muted);
  }

  .table .amount {
    text-align: right;
    font-family: monospace;
  }

  .table tbody tr {
    cursor: pointer;
  }

  .table tbody tr:hover {
    background: var(--color-bg);
  }

  .table tbody tr.selected {
    background: var(--color-primary-light, #e0f2fe);
  }

  .status {
    display: inline-block;
    padding: 0.25rem 0.5rem;
    border-radius: 4px;
    font-size: 0.75rem;
    font-weight: 500;
  }

  .status-draft {
    background: var(--color-bg);
    color: var(--color-text-muted);
  }

  .status-submitted {
    background: #fef3c7;
    color: #92400e;
  }

  .status-accepted {
    background: #d1fae5;
    color: #065f46;
  }

  .empty-state,
  .empty-message {
    text-align: center;
    padding: 2rem;
    color: var(--color-text-muted);
  }

  @media (max-width: 768px) {
    .declarations-grid {
      grid-template-columns: 1fr;
    }

    .generate-form {
      flex-direction: column;
    }

    .generate-form .form-group {
      max-width: none;
      width: 100%;
    }

    .summary-cards {
      flex-direction: column;
    }
  }
</style>
```

**Step 6: Write E2E test for VAT Returns page**

```typescript
// frontend/e2e/demo/vat-returns.spec.ts
import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureDemoTenant } from './utils';

test.describe('Demo VAT Returns - Page Structure Verification', () => {
  test.beforeEach(async ({ page }, testInfo) => {
    await loginAsDemo(page, testInfo);
    await ensureDemoTenant(page, testInfo);
    await navigateTo(page, '/vat-returns', testInfo);
    await page.waitForLoadState('networkidle');
  });

  test('displays VAT returns page heading', async ({ page }) => {
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible({ timeout: 10000 });
  });

  test('shows generate declaration section', async ({ page }) => {
    await expect(page.getByRole('button', { name: /generate/i })).toBeVisible({ timeout: 10000 });
  });

  test('has year and month dropdowns', async ({ page }) => {
    const yearSelect = page.locator('select#year');
    const monthSelect = page.locator('select#month');
    await expect(yearSelect).toBeVisible({ timeout: 10000 });
    await expect(monthSelect).toBeVisible({ timeout: 10000 });
  });

  test('can generate a KMD declaration', async ({ page }) => {
    // Select 2024 January
    await page.locator('select#year').selectOption('2024');
    await page.locator('select#month').selectOption('1');

    // Click generate
    await page.getByRole('button', { name: /generate/i }).click();
    await page.waitForLoadState('networkidle');

    // Should show the declaration in the list
    await expect(page.getByText('2024-01')).toBeVisible({ timeout: 10000 });
  });

  test('shows declaration detail when selected', async ({ page }) => {
    // Generate a declaration first
    await page.locator('select#year').selectOption('2024');
    await page.locator('select#month').selectOption('2');
    await page.getByRole('button', { name: /generate/i }).click();
    await page.waitForLoadState('networkidle');

    // Click on the declaration row
    await page.getByText('2024-02').click();

    // Should show summary cards
    await expect(page.getByText(/Output VAT|Arvestatud käibemaks/i)).toBeVisible({ timeout: 10000 });
    await expect(page.getByText(/Input VAT|Sisendkäibemaks/i)).toBeVisible({ timeout: 10000 });
  });

  test('has export XML button', async ({ page }) => {
    // Generate and select a declaration
    await page.locator('select#year').selectOption('2024');
    await page.locator('select#month').selectOption('3');
    await page.getByRole('button', { name: /generate/i }).click();
    await page.waitForLoadState('networkidle');
    await page.getByText('2024-03').click();

    // Should show export button
    await expect(page.getByRole('button', { name: /export xml/i })).toBeVisible({ timeout: 10000 });
  });
});
```

**Step 7: Run E2E test to verify**

Run: `cd frontend && npx playwright test e2e/demo/vat-returns.spec.ts --headed`
Expected: Tests should pass once backend routes are wired

**Step 8: Commit**

```bash
git add frontend/src/routes/vat-returns/ frontend/src/lib/api.ts frontend/messages/*.json frontend/e2e/demo/vat-returns.spec.ts
git commit -m "feat(vat): add VAT returns (KMD) frontend with E2E tests

- Add KMD types and API methods to frontend
- Create /vat-returns page with declaration list and detail view
- Add i18n translations (en/et)
- Add E2E tests for demo mode"
```

---

### Task 2: Cash Flow Statement Report

**Files:**
- Create: `internal/reports/cashflow.go`
- Create: `internal/reports/types.go`
- Create: `internal/reports/service.go`
- Create: `internal/reports/repository.go`
- Create: `internal/reports/service_test.go`
- Modify: `cmd/api/handlers.go` (add handler)
- Create: `frontend/src/routes/reports/cash-flow/+page.svelte`
- Test: `internal/reports/service_test.go`
- Test: `frontend/e2e/demo/cash-flow.spec.ts`

**Step 1: Write failing test for Cash Flow service**

```go
// internal/reports/service_test.go
package reports

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateCashFlowStatement(t *testing.T) {
	ctx := context.Background()
	mockRepo := NewMockRepository()
	svc := NewServiceWithRepository(mockRepo)

	// Setup mock data
	mockRepo.JournalEntries = []JournalEntryWithLines{
		{
			ID:          "je-1",
			EntryDate:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			Description: "Cash sale",
			Lines: []JournalLine{
				{AccountCode: "1000", AccountType: "ASSET", Debit: decimal.NewFromFloat(1000), Credit: decimal.Zero},
				{AccountCode: "4000", AccountType: "REVENUE", Debit: decimal.Zero, Credit: decimal.NewFromFloat(1000)},
			},
		},
		{
			ID:          "je-2",
			EntryDate:   time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC),
			Description: "Supplier payment",
			Lines: []JournalLine{
				{AccountCode: "2000", AccountType: "LIABILITY", Debit: decimal.NewFromFloat(500), Credit: decimal.Zero},
				{AccountCode: "1000", AccountType: "ASSET", Debit: decimal.Zero, Credit: decimal.NewFromFloat(500)},
			},
		},
	}

	req := &CashFlowRequest{
		StartDate: "2024-01-01",
		EndDate:   "2024-01-31",
	}

	result, err := svc.GenerateCashFlowStatement(ctx, "tenant-1", "schema_tenant1", req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "2024-01-01", result.StartDate)
	assert.Equal(t, "2024-01-31", result.EndDate)

	// Operating activities should show net cash from sales and payments
	assert.NotEmpty(t, result.OperatingActivities)
}

func TestCashFlowOperatingActivities(t *testing.T) {
	ctx := context.Background()
	mockRepo := NewMockRepository()
	svc := NewServiceWithRepository(mockRepo)

	// Cash received from customers
	mockRepo.JournalEntries = []JournalEntryWithLines{
		{
			ID:          "je-1",
			EntryDate:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			Description: "Cash from customer",
			Lines: []JournalLine{
				{AccountCode: "1000", AccountType: "ASSET", AccountName: "Cash", Debit: decimal.NewFromFloat(5000), Credit: decimal.Zero},
				{AccountCode: "1200", AccountType: "ASSET", AccountName: "Accounts Receivable", Debit: decimal.Zero, Credit: decimal.NewFromFloat(5000)},
			},
		},
	}

	req := &CashFlowRequest{StartDate: "2024-01-01", EndDate: "2024-01-31"}
	result, err := svc.GenerateCashFlowStatement(ctx, "tenant-1", "schema_tenant1", req)

	require.NoError(t, err)

	// Should have cash received from customers
	var cashFromCustomers decimal.Decimal
	for _, item := range result.OperatingActivities {
		if item.Code == "CF_OPER_RECEIPTS" {
			cashFromCustomers = item.Amount
			break
		}
	}
	assert.True(t, cashFromCustomers.GreaterThan(decimal.Zero), "Should have positive cash from customers")
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/tsopic/algo/open-accounting && go test ./internal/reports/... -v -run TestGenerateCashFlowStatement`
Expected: FAIL - package does not exist

**Step 3: Create reports types**

```go
// internal/reports/types.go
package reports

import (
	"time"

	"github.com/shopspring/decimal"
)

// CashFlowStatement represents an Estonian-standard cash flow statement
type CashFlowStatement struct {
	TenantID            string              `json:"tenant_id"`
	StartDate           string              `json:"start_date"`
	EndDate             string              `json:"end_date"`
	OperatingActivities []CashFlowItem      `json:"operating_activities"`
	InvestingActivities []CashFlowItem      `json:"investing_activities"`
	FinancingActivities []CashFlowItem      `json:"financing_activities"`
	TotalOperating      decimal.Decimal     `json:"total_operating"`
	TotalInvesting      decimal.Decimal     `json:"total_investing"`
	TotalFinancing      decimal.Decimal     `json:"total_financing"`
	NetCashChange       decimal.Decimal     `json:"net_cash_change"`
	OpeningCash         decimal.Decimal     `json:"opening_cash"`
	ClosingCash         decimal.Decimal     `json:"closing_cash"`
	GeneratedAt         time.Time           `json:"generated_at"`
}

// CashFlowItem represents a line item in the cash flow statement
type CashFlowItem struct {
	Code        string          `json:"code"`
	Description string          `json:"description"`
	DescriptionET string        `json:"description_et"`
	Amount      decimal.Decimal `json:"amount"`
	IsSubtotal  bool            `json:"is_subtotal"`
}

// CashFlowRequest represents a request to generate cash flow statement
type CashFlowRequest struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	CompareType string `json:"compare_type,omitempty"` // "", "previous_months", "previous_years", "quarters", "objects"
}

// JournalEntryWithLines represents a journal entry with its lines for reporting
type JournalEntryWithLines struct {
	ID          string        `json:"id"`
	EntryDate   time.Time     `json:"entry_date"`
	Description string        `json:"description"`
	Lines       []JournalLine `json:"lines"`
}

// JournalLine represents a single journal line
type JournalLine struct {
	AccountCode string          `json:"account_code"`
	AccountName string          `json:"account_name"`
	AccountType string          `json:"account_type"`
	Debit       decimal.Decimal `json:"debit"`
	Credit      decimal.Decimal `json:"credit"`
}

// Estonian cash flow codes
const (
	// Operating Activities - Rahavood äritegevusest
	CFOperReceipts    = "CF_OPER_RECEIPTS"    // Kaupade või teenuste müügist laekunud raha
	CFOperPayments    = "CF_OPER_PAYMENTS"    // Kaupade, materjalide ja teenuste eest makstud raha
	CFOperWages       = "CF_OPER_WAGES"       // Makstud palgad
	CFOperTaxes       = "CF_OPER_TAXES"       // Makstud tulumaks
	CFOperInterestPd  = "CF_OPER_INTEREST_PD" // Makstud intressid
	CFOperOther       = "CF_OPER_OTHER"       // Muud rahavood äritegevusest
	CFOperTotal       = "CF_OPER_TOTAL"       // Rahavood äritegevusest kokku

	// Investing Activities - Rahavood investeerimistegevusest
	CFInvFixedAssets  = "CF_INV_FIXED_ASSETS" // Materiaalse põhivara ost/müük
	CFInvProperty     = "CF_INV_PROPERTY"     // Kinnisvarainvesteeringud
	CFInvSubsidiaries = "CF_INV_SUBSIDIARIES" // Tütar- ja sidusettevõtted
	CFInvLoansGiven   = "CF_INV_LOANS_GIVEN"  // Teistele osapooltele antud laenud
	CFInvLoansRcvd    = "CF_INV_LOANS_RCVD"   // Antud laenude laekumised
	CFInvDividends    = "CF_INV_DIVIDENDS"    // Saadud intressid ja dividendid
	CFInvTotal        = "CF_INV_TOTAL"        // Rahavood investeerimistegevusest kokku

	// Financing Activities - Rahavood finantseerimistegevusest
	CFFinLoansRcvd    = "CF_FIN_LOANS_RCVD"   // Laenude saamine
	CFFinLoansRepaid  = "CF_FIN_LOANS_REPAID" // Saadud laenude tagasimaksmine
	CFFinLease        = "CF_FIN_LEASE"        // Kapitalirendi maksed
	CFFinShares       = "CF_FIN_SHARES"       // Aktsiate emiteerimine
	CFFinDividendsPd  = "CF_FIN_DIVIDENDS_PD" // Dividendide maksmine
	CFFinTotal        = "CF_FIN_TOTAL"        // Rahavood finantseerimistegevusest kokku
)
```

**Step 4: Create mock repository**

```go
// internal/reports/repository.go
package reports

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// Repository defines the interface for report data access
type Repository interface {
	// GetJournalEntriesForPeriod retrieves journal entries within a date range
	GetJournalEntriesForPeriod(ctx context.Context, schemaName, tenantID string, startDate, endDate time.Time) ([]JournalEntryWithLines, error)

	// GetCashAccountBalance gets balance of cash accounts at a specific date
	GetCashAccountBalance(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) (decimal.Decimal, error)
}

// MockRepository for testing
type MockRepository struct {
	JournalEntries     []JournalEntryWithLines
	CashBalance        decimal.Decimal
	GetEntriesErr      error
	GetCashBalanceErr  error
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		JournalEntries: make([]JournalEntryWithLines, 0),
		CashBalance:    decimal.Zero,
	}
}

func (m *MockRepository) GetJournalEntriesForPeriod(ctx context.Context, schemaName, tenantID string, startDate, endDate time.Time) ([]JournalEntryWithLines, error) {
	if m.GetEntriesErr != nil {
		return nil, m.GetEntriesErr
	}

	// Filter by date range
	var result []JournalEntryWithLines
	for _, entry := range m.JournalEntries {
		if (entry.EntryDate.Equal(startDate) || entry.EntryDate.After(startDate)) &&
			(entry.EntryDate.Equal(endDate) || entry.EntryDate.Before(endDate)) {
			result = append(result, entry)
		}
	}
	return result, nil
}

func (m *MockRepository) GetCashAccountBalance(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) (decimal.Decimal, error) {
	if m.GetCashBalanceErr != nil {
		return decimal.Zero, m.GetCashBalanceErr
	}
	return m.CashBalance, nil
}
```

**Step 5: Create service implementation**

```go
// internal/reports/service.go
package reports

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Service provides financial report operations
type Service struct {
	repo Repository
}

// NewService creates a new reports service with a PostgreSQL repository
func NewService(db *pgxpool.Pool) *Service {
	return &Service{repo: NewPostgresRepository(db)}
}

// NewServiceWithRepository creates a new reports service with an injected repository
func NewServiceWithRepository(repo Repository) *Service {
	return &Service{repo: repo}
}

// GenerateCashFlowStatement generates a cash flow statement for the given period
func (s *Service) GenerateCashFlowStatement(ctx context.Context, tenantID, schemaName string, req *CashFlowRequest) (*CashFlowStatement, error) {
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return nil, fmt.Errorf("invalid start date: %w", err)
	}
	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		return nil, fmt.Errorf("invalid end date: %w", err)
	}

	// Get journal entries for the period
	entries, err := s.repo.GetJournalEntriesForPeriod(ctx, schemaName, tenantID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("get journal entries: %w", err)
	}

	// Get opening cash balance
	openingCash, err := s.repo.GetCashAccountBalance(ctx, schemaName, tenantID, startDate.AddDate(0, 0, -1))
	if err != nil {
		return nil, fmt.Errorf("get opening cash: %w", err)
	}

	// Classify and aggregate cash flows
	operating := s.classifyOperatingActivities(entries)
	investing := s.classifyInvestingActivities(entries)
	financing := s.classifyFinancingActivities(entries)

	totalOperating := sumCashFlowItems(operating)
	totalInvesting := sumCashFlowItems(investing)
	totalFinancing := sumCashFlowItems(financing)
	netChange := totalOperating.Add(totalInvesting).Add(totalFinancing)

	// Add subtotals
	operating = append(operating, CashFlowItem{
		Code:        CFOperTotal,
		Description: "Net cash from operating activities",
		DescriptionET: "Rahavood äritegevusest kokku",
		Amount:      totalOperating,
		IsSubtotal:  true,
	})

	investing = append(investing, CashFlowItem{
		Code:        CFInvTotal,
		Description: "Net cash from investing activities",
		DescriptionET: "Rahavood investeerimistegevusest kokku",
		Amount:      totalInvesting,
		IsSubtotal:  true,
	})

	financing = append(financing, CashFlowItem{
		Code:        CFFinTotal,
		Description: "Net cash from financing activities",
		DescriptionET: "Rahavood finantseerimistegevusest kokku",
		Amount:      totalFinancing,
		IsSubtotal:  true,
	})

	return &CashFlowStatement{
		TenantID:            tenantID,
		StartDate:           req.StartDate,
		EndDate:             req.EndDate,
		OperatingActivities: operating,
		InvestingActivities: investing,
		FinancingActivities: financing,
		TotalOperating:      totalOperating,
		TotalInvesting:      totalInvesting,
		TotalFinancing:      totalFinancing,
		NetCashChange:       netChange,
		OpeningCash:         openingCash,
		ClosingCash:         openingCash.Add(netChange),
		GeneratedAt:         time.Now(),
	}, nil
}

func (s *Service) classifyOperatingActivities(entries []JournalEntryWithLines) []CashFlowItem {
	var receipts, payments, wages, taxes decimal.Decimal

	for _, entry := range entries {
		// Look for cash account movements
		var cashMovement decimal.Decimal
		var counterpartyType string

		for _, line := range entry.Lines {
			if isCashAccount(line.AccountCode) {
				cashMovement = line.Debit.Sub(line.Credit)
			} else {
				counterpartyType = line.AccountType
			}
		}

		if cashMovement.IsZero() {
			continue
		}

		// Classify based on counterparty account
		switch counterpartyType {
		case "REVENUE", "ASSET": // Receivables
			if cashMovement.GreaterThan(decimal.Zero) {
				receipts = receipts.Add(cashMovement)
			}
		case "EXPENSE":
			if cashMovement.LessThan(decimal.Zero) {
				payments = payments.Add(cashMovement.Abs())
			}
		case "LIABILITY":
			if cashMovement.LessThan(decimal.Zero) {
				payments = payments.Add(cashMovement.Abs())
			}
		}
	}

	items := []CashFlowItem{}
	if !receipts.IsZero() {
		items = append(items, CashFlowItem{
			Code:          CFOperReceipts,
			Description:   "Cash received from customers",
			DescriptionET: "Kaupade või teenuste müügist laekunud raha",
			Amount:        receipts,
		})
	}
	if !payments.IsZero() {
		items = append(items, CashFlowItem{
			Code:          CFOperPayments,
			Description:   "Cash paid to suppliers",
			DescriptionET: "Kaupade, materjalide ja teenuste eest makstud raha",
			Amount:        payments.Neg(),
		})
	}

	return items
}

func (s *Service) classifyInvestingActivities(entries []JournalEntryWithLines) []CashFlowItem {
	// Simplified - look for fixed asset related cash movements
	var fixedAssets decimal.Decimal

	for _, entry := range entries {
		var cashMovement decimal.Decimal
		var isFixedAsset bool

		for _, line := range entry.Lines {
			if isCashAccount(line.AccountCode) {
				cashMovement = line.Debit.Sub(line.Credit)
			}
			if isFixedAssetAccount(line.AccountCode) {
				isFixedAsset = true
			}
		}

		if isFixedAsset && !cashMovement.IsZero() {
			fixedAssets = fixedAssets.Add(cashMovement)
		}
	}

	items := []CashFlowItem{}
	if !fixedAssets.IsZero() {
		items = append(items, CashFlowItem{
			Code:          CFInvFixedAssets,
			Description:   "Purchase/sale of fixed assets",
			DescriptionET: "Materiaalse ja immateriaalse põhivara ost ja müük",
			Amount:        fixedAssets,
		})
	}

	return items
}

func (s *Service) classifyFinancingActivities(entries []JournalEntryWithLines) []CashFlowItem {
	// Simplified - look for loan and equity related cash movements
	var loans, dividends decimal.Decimal

	for _, entry := range entries {
		var cashMovement decimal.Decimal
		var isLoan, isDividend bool

		for _, line := range entry.Lines {
			if isCashAccount(line.AccountCode) {
				cashMovement = line.Debit.Sub(line.Credit)
			}
			if isLoanAccount(line.AccountCode) {
				isLoan = true
			}
			if isDividendAccount(line.AccountCode) {
				isDividend = true
			}
		}

		if isLoan && !cashMovement.IsZero() {
			loans = loans.Add(cashMovement)
		}
		if isDividend && !cashMovement.IsZero() {
			dividends = dividends.Add(cashMovement)
		}
	}

	items := []CashFlowItem{}
	if !loans.IsZero() {
		if loans.GreaterThan(decimal.Zero) {
			items = append(items, CashFlowItem{
				Code:          CFFinLoansRcvd,
				Description:   "Proceeds from loans",
				DescriptionET: "Laenude saamine",
				Amount:        loans,
			})
		} else {
			items = append(items, CashFlowItem{
				Code:          CFFinLoansRepaid,
				Description:   "Repayment of loans",
				DescriptionET: "Saadud laenude tagasimaksmine",
				Amount:        loans,
			})
		}
	}
	if !dividends.IsZero() {
		items = append(items, CashFlowItem{
			Code:          CFFinDividendsPd,
			Description:   "Dividends paid",
			DescriptionET: "Dividendide maksmine",
			Amount:        dividends,
		})
	}

	return items
}

func sumCashFlowItems(items []CashFlowItem) decimal.Decimal {
	sum := decimal.Zero
	for _, item := range items {
		if !item.IsSubtotal {
			sum = sum.Add(item.Amount)
		}
	}
	return sum
}

func isCashAccount(code string) bool {
	// Estonian chart of accounts: 1000-1099 are typically cash accounts
	return len(code) >= 4 && code[:2] == "10"
}

func isFixedAssetAccount(code string) bool {
	// Estonian chart of accounts: 1500-1599 are fixed assets
	return len(code) >= 4 && code[:2] == "15"
}

func isLoanAccount(code string) bool {
	// Estonian chart of accounts: 2000-2099 are short-term loans, 2500+ long-term
	return len(code) >= 4 && (code[:2] == "20" || code[:2] == "25")
}

func isDividendAccount(code string) bool {
	// Estonian chart of accounts: 3xxx are equity, dividends declared would be here
	return len(code) >= 4 && code[:1] == "3"
}
```

**Step 6: Run tests to verify implementation**

Run: `cd /Users/tsopic/algo/open-accounting && go test ./internal/reports/... -v`
Expected: PASS

**Step 7: Commit**

```bash
git add internal/reports/
git commit -m "feat(reports): add cash flow statement service with TDD

- Add CashFlowStatement type with Estonian-standard sections
- Implement service with operating/investing/financing classification
- Add mock repository for testing
- Include bilingual descriptions (EN/ET)"
```

---

### Task 3: Cash Flow Statement Frontend

**Files:**
- Create: `frontend/src/routes/reports/cash-flow/+page.svelte`
- Modify: `frontend/src/lib/api.ts` (add CashFlow types)
- Modify: `frontend/messages/en.json` (add translations)
- Modify: `frontend/messages/et.json` (add translations)
- Test: `frontend/e2e/demo/cash-flow.spec.ts`

**Step 1: Add CashFlow types to API**

```typescript
// Add to frontend/src/lib/api.ts

export interface CashFlowItem {
  code: string;
  description: string;
  description_et: string;
  amount: string;
  is_subtotal: boolean;
}

export interface CashFlowStatement {
  tenant_id: string;
  start_date: string;
  end_date: string;
  operating_activities: CashFlowItem[];
  investing_activities: CashFlowItem[];
  financing_activities: CashFlowItem[];
  total_operating: string;
  total_investing: string;
  total_financing: string;
  net_cash_change: string;
  opening_cash: string;
  closing_cash: string;
  generated_at: string;
}

export interface CashFlowRequest {
  start_date: string;
  end_date: string;
  compare_type?: string;
}
```

**Step 2: Add API method**

```typescript
// Add to api object

async getCashFlowStatement(tenantId: string, startDate: string, endDate: string): Promise<CashFlowStatement> {
  const response = await this.fetch(
    `/api/v1/tenants/${tenantId}/reports/cash-flow?start_date=${startDate}&end_date=${endDate}`
  );
  return response.json();
},
```

**Step 3: Add i18n translations**

```json
// Add to frontend/messages/en.json
"cashflow_title": "Cash Flow Statement",
"cashflow_generate": "Generate Report",
"cashflow_operating": "Operating Activities",
"cashflow_investing": "Investing Activities",
"cashflow_financing": "Financing Activities",
"cashflow_net_change": "Net Change in Cash",
"cashflow_opening": "Opening Cash Balance",
"cashflow_closing": "Closing Cash Balance",
"cashflow_total_operating": "Net Cash from Operating",
"cashflow_total_investing": "Net Cash from Investing",
"cashflow_total_financing": "Net Cash from Financing"
```

```json
// Add to frontend/messages/et.json
"cashflow_title": "Rahavoogude aruanne",
"cashflow_generate": "Genereeri aruanne",
"cashflow_operating": "Rahavood äritegevusest",
"cashflow_investing": "Rahavood investeerimistegevusest",
"cashflow_financing": "Rahavood finantseerimistegevusest",
"cashflow_net_change": "Raha muutus kokku",
"cashflow_opening": "Raha perioodi alguses",
"cashflow_closing": "Raha perioodi lõpus",
"cashflow_total_operating": "Äritegevuse rahavood kokku",
"cashflow_total_investing": "Investeerimistegevuse rahavood kokku",
"cashflow_total_financing": "Finantseerimistegevuse rahavood kokku"
```

**Step 4: Create Cash Flow page**

```svelte
<!-- frontend/src/routes/reports/cash-flow/+page.svelte -->
<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { api, type CashFlowStatement, type CashFlowItem } from '$lib/api';
  import Decimal from 'decimal.js';
  import * as m from '$lib/paraglide/messages.js';
  import ExportButton from '$lib/components/ExportButton.svelte';
  import { languageTag } from '$lib/paraglide/runtime';

  let tenantId = $derived($page.url.searchParams.get('tenant') || '');
  let isLoading = $state(false);
  let error = $state('');

  let startDate = $state(new Date(new Date().getFullYear(), 0, 1).toISOString().split('T')[0]);
  let endDate = $state(new Date().toISOString().split('T')[0]);

  let report = $state<CashFlowStatement | null>(null);

  async function loadReport() {
    if (!tenantId) return;

    isLoading = true;
    error = '';
    try {
      report = await api.getCashFlowStatement(tenantId, startDate, endDate);
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to load report';
    } finally {
      isLoading = false;
    }
  }

  function formatAmount(amount: string | undefined): string {
    if (!amount) return '0.00';
    const dec = new Decimal(amount);
    return dec.toFixed(2);
  }

  function getDescription(item: CashFlowItem): string {
    const lang = languageTag();
    return lang === 'et' ? item.description_et : item.description;
  }

  function getAmountClass(amount: string): string {
    const dec = new Decimal(amount || 0);
    if (dec.isZero()) return '';
    return dec.greaterThan(0) ? 'positive' : 'negative';
  }

  function getExportData() {
    if (!report) return { data: [[]], headers: [[]] };

    const rows: Record<string, unknown>[] = [];

    // Operating
    rows.push({ section: m.cashflow_operating(), amount: '' });
    for (const item of report.operating_activities) {
      rows.push({
        section: item.is_subtotal ? `  ${getDescription(item)}` : `    ${getDescription(item)}`,
        amount: formatAmount(item.amount)
      });
    }

    // Investing
    rows.push({ section: '', amount: '' });
    rows.push({ section: m.cashflow_investing(), amount: '' });
    for (const item of report.investing_activities) {
      rows.push({
        section: item.is_subtotal ? `  ${getDescription(item)}` : `    ${getDescription(item)}`,
        amount: formatAmount(item.amount)
      });
    }

    // Financing
    rows.push({ section: '', amount: '' });
    rows.push({ section: m.cashflow_financing(), amount: '' });
    for (const item of report.financing_activities) {
      rows.push({
        section: item.is_subtotal ? `  ${getDescription(item)}` : `    ${getDescription(item)}`,
        amount: formatAmount(item.amount)
      });
    }

    // Summary
    rows.push({ section: '', amount: '' });
    rows.push({ section: m.cashflow_net_change(), amount: formatAmount(report.net_cash_change) });
    rows.push({ section: m.cashflow_opening(), amount: formatAmount(report.opening_cash) });
    rows.push({ section: m.cashflow_closing(), amount: formatAmount(report.closing_cash) });

    return {
      data: [rows],
      headers: [['Description', 'Amount EUR']]
    };
  }
</script>

<svelte:head>
  <title>{m.cashflow_title()} - Open Accounting</title>
</svelte:head>

<div class="container">
  <div class="header">
    <h1>{m.cashflow_title()}</h1>
    {#if report}
      <ExportButton
        data={getExportData().data}
        headers={getExportData().headers}
        filename={`cash-flow-${startDate}-${endDate}`}
      />
    {/if}
  </div>

  {#if !tenantId}
    <div class="card empty-state">
      <p>Select a tenant from <a href="/dashboard">{m.nav_dashboard()}</a>.</p>
    </div>
  {:else}
    <div class="card controls">
      <div class="control-row">
        <div class="form-group">
          <label class="label" for="startDate">{m.reports_startDate()}</label>
          <input class="input" type="date" id="startDate" bind:value={startDate} />
        </div>
        <div class="form-group">
          <label class="label" for="endDate">{m.reports_endDate()}</label>
          <input class="input" type="date" id="endDate" bind:value={endDate} />
        </div>
        <button class="btn btn-primary" onclick={loadReport} disabled={isLoading}>
          {isLoading ? m.reports_generating() : m.cashflow_generate()}
        </button>
      </div>
    </div>

    {#if error}
      <div class="alert alert-error">{error}</div>
    {/if}

    {#if report}
      <div class="card report">
        <div class="report-header">
          <h2>{m.cashflow_title()}</h2>
          <p class="report-period">{report.start_date} - {report.end_date}</p>
        </div>

        <!-- Operating Activities -->
        <section class="cf-section">
          <h3>{m.cashflow_operating()}</h3>
          <table class="cf-table">
            <tbody>
              {#each report.operating_activities as item}
                <tr class:subtotal={item.is_subtotal}>
                  <td>{getDescription(item)}</td>
                  <td class="amount {getAmountClass(item.amount)}">{formatAmount(item.amount)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </section>

        <!-- Investing Activities -->
        <section class="cf-section">
          <h3>{m.cashflow_investing()}</h3>
          <table class="cf-table">
            <tbody>
              {#each report.investing_activities as item}
                <tr class:subtotal={item.is_subtotal}>
                  <td>{getDescription(item)}</td>
                  <td class="amount {getAmountClass(item.amount)}">{formatAmount(item.amount)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </section>

        <!-- Financing Activities -->
        <section class="cf-section">
          <h3>{m.cashflow_financing()}</h3>
          <table class="cf-table">
            <tbody>
              {#each report.financing_activities as item}
                <tr class:subtotal={item.is_subtotal}>
                  <td>{getDescription(item)}</td>
                  <td class="amount {getAmountClass(item.amount)}">{formatAmount(item.amount)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </section>

        <!-- Summary -->
        <section class="cf-summary">
          <table class="cf-table">
            <tbody>
              <tr class="summary-row">
                <td>{m.cashflow_net_change()}</td>
                <td class="amount {getAmountClass(report.net_cash_change)}">{formatAmount(report.net_cash_change)}</td>
              </tr>
              <tr>
                <td>{m.cashflow_opening()}</td>
                <td class="amount">{formatAmount(report.opening_cash)}</td>
              </tr>
              <tr class="closing-row">
                <td><strong>{m.cashflow_closing()}</strong></td>
                <td class="amount"><strong>{formatAmount(report.closing_cash)}</strong></td>
              </tr>
            </tbody>
          </table>
        </section>

        <p class="report-footer">
          {m.reports_generatedOn()} {new Date(report.generated_at).toLocaleString()}
        </p>
      </div>
    {/if}
  {/if}
</div>

<style>
  .header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1.5rem;
  }

  h1 {
    font-size: 1.75rem;
  }

  .controls {
    margin-bottom: 1.5rem;
  }

  .control-row {
    display: flex;
    gap: 1rem;
    align-items: flex-end;
    flex-wrap: wrap;
  }

  .control-row .form-group {
    flex: 1;
    min-width: 150px;
    max-width: 200px;
  }

  .report {
    padding: 2rem;
  }

  .report-header {
    text-align: center;
    margin-bottom: 2rem;
    padding-bottom: 1rem;
    border-bottom: 2px solid var(--color-border);
  }

  .report-header h2 {
    font-size: 1.5rem;
    margin-bottom: 0.5rem;
  }

  .report-period {
    color: var(--color-text-muted);
  }

  .cf-section {
    margin-bottom: 2rem;
  }

  .cf-section h3 {
    font-size: 1.1rem;
    margin-bottom: 1rem;
    padding-bottom: 0.5rem;
    border-bottom: 1px solid var(--color-border);
    color: var(--color-primary);
  }

  .cf-table {
    width: 100%;
    border-collapse: collapse;
  }

  .cf-table td {
    padding: 0.5rem 1rem;
  }

  .cf-table .amount {
    text-align: right;
    font-family: monospace;
    width: 150px;
  }

  .cf-table tr.subtotal {
    font-weight: 600;
    border-top: 1px solid var(--color-border);
  }

  .cf-table tr.subtotal td {
    padding-top: 1rem;
  }

  .cf-summary {
    margin-top: 2rem;
    padding-top: 1rem;
    border-top: 2px solid var(--color-border);
  }

  .summary-row {
    font-weight: 600;
  }

  .closing-row {
    border-top: 2px solid var(--color-border);
  }

  .closing-row td {
    padding-top: 1rem;
  }

  .positive {
    color: var(--color-success, #10b981);
  }

  .negative {
    color: var(--color-error, #ef4444);
  }

  .report-footer {
    margin-top: 2rem;
    text-align: center;
    font-size: 0.75rem;
    color: var(--color-text-muted);
  }

  .empty-state {
    text-align: center;
    padding: 3rem;
  }

  @media (max-width: 768px) {
    .control-row {
      flex-direction: column;
    }

    .control-row .form-group {
      max-width: none;
      width: 100%;
    }

    .report {
      padding: 1rem;
    }

    .cf-table .amount {
      width: auto;
    }
  }
</style>
```

**Step 5: Write E2E test**

```typescript
// frontend/e2e/demo/cash-flow.spec.ts
import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureDemoTenant } from './utils';

test.describe('Demo Cash Flow Statement', () => {
  test.beforeEach(async ({ page }, testInfo) => {
    await loginAsDemo(page, testInfo);
    await ensureDemoTenant(page, testInfo);
    await navigateTo(page, '/reports/cash-flow', testInfo);
    await page.waitForLoadState('networkidle');
  });

  test('displays cash flow page heading', async ({ page }) => {
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible({ timeout: 10000 });
  });

  test('has date range inputs', async ({ page }) => {
    await expect(page.locator('input#startDate')).toBeVisible({ timeout: 10000 });
    await expect(page.locator('input#endDate')).toBeVisible({ timeout: 10000 });
  });

  test('can generate report', async ({ page }) => {
    await page.locator('input#startDate').fill('2024-01-01');
    await page.locator('input#endDate').fill('2024-12-31');
    await page.getByRole('button', { name: /generate/i }).click();
    await page.waitForLoadState('networkidle');

    // Should show three sections
    await expect(page.getByText(/Operating Activities|Rahavood äritegevusest/i)).toBeVisible({ timeout: 10000 });
    await expect(page.getByText(/Investing Activities|Rahavood investeerimistegevusest/i)).toBeVisible({ timeout: 10000 });
    await expect(page.getByText(/Financing Activities|Rahavood finantseerimistegevusest/i)).toBeVisible({ timeout: 10000 });
  });

  test('shows cash summary', async ({ page }) => {
    await page.locator('input#startDate').fill('2024-01-01');
    await page.locator('input#endDate').fill('2024-12-31');
    await page.getByRole('button', { name: /generate/i }).click();
    await page.waitForLoadState('networkidle');

    await expect(page.getByText(/Net Change|Raha muutus/i)).toBeVisible({ timeout: 10000 });
    await expect(page.getByText(/Opening.*Balance|Raha perioodi alguses/i)).toBeVisible({ timeout: 10000 });
    await expect(page.getByText(/Closing.*Balance|Raha perioodi lõpus/i)).toBeVisible({ timeout: 10000 });
  });
});
```

**Step 6: Run tests**

Run: `cd frontend && npx playwright test e2e/demo/cash-flow.spec.ts --headed`
Expected: Tests should pass

**Step 7: Commit**

```bash
git add frontend/src/routes/reports/cash-flow/ frontend/src/lib/api.ts frontend/messages/*.json frontend/e2e/demo/cash-flow.spec.ts
git commit -m "feat(reports): add cash flow statement frontend with E2E tests

- Add CashFlowStatement types and API
- Create /reports/cash-flow page with Estonian sections
- Add bilingual i18n (en/et)
- Add E2E tests for demo mode"
```

---

## Phase 2B: Payroll Enhancement (Tasks 4-7)

### Task 4: Leave Management Database Schema

**Files:**
- Create: `migrations/017_leave_management.up.sql`
- Create: `migrations/017_leave_management.down.sql`
- Create: `scripts/demo-seed-leave.sql`

(Continue with detailed steps...)

### Task 5: Leave Management Backend

**Files:**
- Create: `internal/payroll/absence_types.go`
- Create: `internal/payroll/absence_service.go`
- Create: `internal/payroll/absence_repository.go`
- Create: `internal/payroll/absence_service_test.go`
- Modify: `cmd/api/handlers.go`

(Continue with detailed steps...)

### Task 6: Leave Management Frontend

**Files:**
- Create: `frontend/src/routes/employees/absences/+page.svelte`
- Modify: `frontend/src/lib/api.ts`
- Test: `frontend/e2e/demo/absences.spec.ts`

(Continue with detailed steps...)

### Task 7: Salary Calculator

**Files:**
- Create: `frontend/src/routes/payroll/calculator/+page.svelte`
- Create: `internal/payroll/calculator.go`
- Create: `internal/payroll/calculator_test.go`
- Test: `frontend/e2e/demo/salary-calculator.spec.ts`

(Continue with detailed steps...)

---

## Phase 2C: Year-End & Cash Flow (Tasks 8-11)

### Task 8: Balance Confirmations

**Files:**
- Create: `internal/reports/balance_confirmation.go`
- Create: `frontend/src/routes/reports/balance-confirmation/+page.svelte`

### Task 9: Payment Reminders

**Files:**
- Create: `internal/invoicing/reminder_service.go`
- Create: `frontend/src/routes/invoices/reminders/+page.svelte`

---

## Phase 2D: Reporting Enhancement (Tasks 10-12)

### Task 10: Cost Centers

**Files:**
- Create: `migrations/018_cost_centers.up.sql`
- Create: `internal/accounting/cost_centers.go`
- Create: `frontend/src/routes/settings/cost-centers/+page.svelte`

---

## Quick Reference: Commands

### Run all Go tests
```bash
cd /Users/tsopic/algo/open-accounting && go test ./... -v
```

### Run specific package tests
```bash
go test ./internal/reports/... -v
```

### Run frontend type check
```bash
cd frontend && npm run check
```

### Run E2E tests
```bash
cd frontend && npx playwright test e2e/demo/ --headed
```

### Build and verify
```bash
go build ./... && cd frontend && npm run build
```

---

## Completion Checklist

### Phase 2A
- [ ] Task 1: VAT Returns Frontend - tests passing
- [ ] Task 2: Cash Flow Statement Backend - tests passing
- [ ] Task 3: Cash Flow Statement Frontend - tests passing

### Phase 2B
- [ ] Task 4: Leave Management Schema - migrated
- [ ] Task 5: Leave Management Backend - tests passing
- [ ] Task 6: Leave Management Frontend - tests passing
- [ ] Task 7: Salary Calculator - tests passing

### Phase 2C
- [ ] Task 8: Balance Confirmations - tests passing
- [ ] Task 9: Payment Reminders - tests passing

### Phase 2D
- [ ] Task 10: Cost Centers - tests passing

---

## Notes for Implementation

1. **Test Pattern**: This codebase uses mock repositories for unit tests. See `internal/payroll/service_test.go` for the pattern.

2. **E2E Tests**: All E2E tests use the demo mode utilities from `frontend/e2e/demo/utils.ts`. Use `loginAsDemo()` and `ensureDemoTenant()`.

3. **i18n**: Always add both English and Estonian translations. Estonian is required for compliance.

4. **Svelte 5**: Use runes (`$state`, `$derived`, `$effect`) not the old reactive syntax.

5. **Decimal handling**: Use `github.com/shopspring/decimal` in Go and `decimal.js` in frontend.
