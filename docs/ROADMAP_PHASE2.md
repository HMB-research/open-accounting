# Phase 2 Roadmap: Dashboard, Reports, Mobile & E2E Testing

## Overview

This document outlines the implementation plan for four major enhancements to the Open Accounting platform:

1. **Dashboard Improvements** - Enhanced visualizations and financial insights
2. **Report Generation** - PDF/Excel export and additional report types
3. **Mobile Responsiveness** - Consistent mobile-first experience
4. **E2E Testing with GitHub Actions** - Playwright integration for automated testing

---

## Current State Analysis

### Dashboard (`frontend/src/routes/dashboard/+page.svelte`)
- 4 summary cards (Revenue, Expenses, Net Income, Receivables)
- Invoice status breakdown
- Chart.js revenue vs expenses chart
- Quick links grid
- Onboarding wizard integration

### Reports (`frontend/src/routes/reports/+page.svelte`)
- Trial Balance, Balance Sheet, Income Statement
- Print functionality only (no export)
- Date range filtering

### Mobile/CSS (`frontend/src/app.css`)
- Custom CSS with CSS variables (no Tailwind)
- Breakpoints: 480px, 768px, 1024px, 1280px
- Some responsive utilities exist but inconsistent usage

### Testing
- Vitest for unit/component tests
- No E2E framework installed
- No E2E tests in CI pipeline

---

## Implementation Plan

### Phase 2.1: Dashboard Improvements

**Priority: High | Estimated Effort: 3-5 days**

#### New Features

1. **Period Selector**
   - Add dropdown for: This Month, Last Month, This Quarter, This Year, Custom Range
   - Persist selection in URL params or localStorage

2. **Additional Summary Cards**
   - Outstanding Payables (amount owed to vendors)
   - Cash Flow (net change in bank accounts)
   - Invoice Count (sent this period)
   - Average Collection Days (DSO metric)

3. **Enhanced Charts**
   - Cash Flow Chart (line chart showing daily/weekly cash position)
   - Invoice Aging Chart (horizontal bar chart by 30/60/90+ days)
   - Revenue by Contact (pie/doughnut chart)

4. **Recent Activity Feed**
   - Show last 10 transactions/invoices/payments
   - Clickable to navigate to detail view

5. **Key Metrics Panel**
   - Gross Margin %
   - Current Ratio (if balance sheet data available)
   - Month-over-Month growth

#### Technical Tasks

| Task | File(s) | Notes |
|------|---------|-------|
| Add period selector component | `frontend/src/lib/components/PeriodSelector.svelte` | Reusable across dashboard/reports |
| Create dashboard API endpoints | `cmd/api/handlers_analytics.go` | Aggregate queries for metrics |
| Add cash flow calculation | `internal/analytics/cashflow.go` | Sum bank transactions by period |
| Create aging report query | `internal/analytics/aging.go` | Invoice age buckets |
| Add Chart.js plugins | `frontend/package.json` | chartjs-plugin-datalabels |
| Create activity feed component | `frontend/src/lib/components/ActivityFeed.svelte` | Recent items list |
| Add i18n keys | `frontend/messages/*.json` | All new labels |

#### New API Endpoints

```
GET /tenants/{id}/analytics/dashboard?period=THIS_MONTH
GET /tenants/{id}/analytics/cashflow?start_date=X&end_date=Y
GET /tenants/{id}/analytics/aging
GET /tenants/{id}/analytics/revenue-by-contact?period=THIS_MONTH
GET /tenants/{id}/activity?limit=10
```

---

### Phase 2.2: Report Generation

**Priority: High | Estimated Effort: 4-6 days**

#### New Features

1. **Export Formats**
   - PDF export (using browser print or jsPDF)
   - Excel export (using SheetJS/xlsx library)
   - CSV export (native implementation)

2. **Additional Report Types**
   - Profit & Loss (monthly breakdown)
   - Cash Flow Statement
   - Accounts Receivable Aging
   - Accounts Payable Aging
   - General Ledger (journal entries)
   - Tax Report (VAT summary)

3. **Report Customization**
   - Date range presets (YTD, Last Year, Custom)
   - Account filtering
   - Comparison periods (vs previous period/year)
   - Column selection

4. **Scheduled Reports**
   - Email reports on schedule (monthly/quarterly)
   - Integrate with existing email service

#### Technical Tasks

| Task | File(s) | Notes |
|------|---------|-------|
| Add xlsx library | `frontend/package.json` | `bun add xlsx` |
| Create ExportButton component | `frontend/src/lib/components/ExportButton.svelte` | PDF/Excel/CSV options |
| Add P&L API endpoint | `internal/analytics/pnl.go` | Monthly breakdown |
| Add Cash Flow Statement API | `internal/analytics/cashflow_statement.go` | Operating/Investing/Financing |
| Create aging report components | `frontend/src/routes/reports/aging/+page.svelte` | AR/AP aging |
| Add General Ledger page | `frontend/src/routes/reports/ledger/+page.svelte` | Journal entries |
| Create VAT report | `frontend/src/routes/reports/vat/+page.svelte` | Estonian VAT support |
| Add report scheduler | `internal/scheduler/reports.go` | Cron-based email |

#### New API Endpoints

```
GET /tenants/{id}/reports/pnl?start_date=X&end_date=Y&compare=PREVIOUS_YEAR
GET /tenants/{id}/reports/cashflow-statement?start_date=X&end_date=Y
GET /tenants/{id}/reports/ar-aging?as_of_date=X
GET /tenants/{id}/reports/ap-aging?as_of_date=X
GET /tenants/{id}/reports/general-ledger?start_date=X&end_date=Y&account_id=Z
GET /tenants/{id}/reports/vat?period=2025-Q1
POST /tenants/{id}/reports/schedule (create scheduled report)
```

---

### Phase 2.3: Mobile Responsiveness

**Priority: Medium | Estimated Effort: 3-4 days**

#### Audit & Fix Areas

1. **Navigation**
   - Hamburger menu for mobile
   - Bottom navigation bar option
   - Collapsible sidebar

2. **Tables**
   - Card view for mobile (existing `.table-mobile-cards` needs consistent usage)
   - Horizontal scroll indicators
   - Priority columns (hide less important on mobile)

3. **Forms**
   - Full-width inputs on mobile
   - Stacked form rows
   - Touch-friendly input sizes (min 44px tap targets)

4. **Modals**
   - Full-screen on mobile
   - Slide-up animation
   - Fixed action buttons at bottom

5. **Dashboard**
   - Single column card layout
   - Swipeable charts
   - Collapsible sections

6. **Reports**
   - Simplified mobile view
   - Export buttons prominent
   - Horizontal scroll for data tables

#### Technical Tasks

| Task | File(s) | Notes |
|------|---------|-------|
| Create MobileNav component | `frontend/src/lib/components/MobileNav.svelte` | Hamburger + drawer |
| Add responsive table mixin | `frontend/src/app.css` | Card transform utility |
| Audit all pages for mobile | All route files | Apply consistent patterns |
| Add touch gestures | `frontend/src/lib/utils/touch.ts` | Swipe detection |
| Create responsive breakpoint hook | `frontend/src/lib/stores/viewport.ts` | Reactive viewport size |
| Test on real devices | Manual | iOS Safari, Android Chrome |

#### CSS Additions

```css
/* Mobile-first approach additions to app.css */

/* Viewport store integration */
.mobile-only { display: none; }
.desktop-only { display: block; }

@media (max-width: 768px) {
  .mobile-only { display: block; }
  .desktop-only { display: none; }

  /* Bottom navigation */
  .bottom-nav {
    position: fixed;
    bottom: 0;
    left: 0;
    right: 0;
    background: var(--color-surface);
    border-top: 1px solid var(--color-border);
    display: flex;
    justify-content: space-around;
    padding: 0.5rem;
    z-index: 100;
  }

  /* Content padding for bottom nav */
  .main-content {
    padding-bottom: 70px;
  }
}
```

---

### Phase 2.4: E2E Testing with GitHub Actions

**Priority: High | Estimated Effort: 4-5 days**

#### Setup Tasks

1. **Install Playwright**
   ```bash
   cd frontend
   npm init playwright@latest
   ```

2. **Configure Playwright**
   - Test against dev server
   - Multiple browsers (Chromium, Firefox, WebKit)
   - Mobile viewports
   - Screenshot on failure

3. **Test Structure**
   ```
   frontend/
   ├── e2e/
   │   ├── auth.spec.ts         # Login/logout flows
   │   ├── dashboard.spec.ts    # Dashboard functionality
   │   ├── invoices.spec.ts     # Invoice CRUD
   │   ├── contacts.spec.ts     # Contact management
   │   ├── recurring.spec.ts    # Recurring invoices
   │   ├── reports.spec.ts      # Report generation
   │   └── mobile.spec.ts       # Mobile-specific tests
   ├── playwright.config.ts
   └── package.json
   ```

4. **GitHub Actions Integration**
   - Add E2E job to CI workflow
   - Run on PR and main branch
   - Parallel test execution
   - Artifact upload for failed tests

#### E2E Test Scenarios

| Test File | Scenarios |
|-----------|-----------|
| `auth.spec.ts` | Login with valid credentials, Invalid login, Logout, Session persistence |
| `dashboard.spec.ts` | Load dashboard, Summary cards display, Chart renders, Quick links work |
| `invoices.spec.ts` | Create invoice, Edit invoice, Delete invoice, Generate PDF, Send email |
| `contacts.spec.ts` | Create contact, Edit contact, Search contacts, Import contacts |
| `recurring.spec.ts` | Create recurring, Generate invoice, Pause/resume, Email configuration |
| `reports.spec.ts` | Trial balance, Balance sheet, Income statement, Export PDF/Excel |
| `mobile.spec.ts` | Navigation menu, Table responsiveness, Form usability, Modal behavior |

#### GitHub Actions Workflow Addition

```yaml
# Add to .github/workflows/ci.yml

e2e:
  runs-on: ubuntu-latest
  needs: [frontend]
  if: needs.changes.outputs.frontend == 'true'

  services:
    postgres:
      image: postgres:16-alpine
      env:
        POSTGRES_USER: test
        POSTGRES_PASSWORD: test
        POSTGRES_DB: openaccounting_test
      ports:
        - 5432:5432
      options: >-
        --health-cmd pg_isready
        --health-interval 10s
        --health-timeout 5s
        --health-retries 5

  steps:
    - uses: actions/checkout@v4

    - name: Setup Go
      uses: actions/setup-go@v5
      with:
        go-version: '1.24'

    - name: Setup Bun
      uses: oven-sh/setup-bun@v2
      with:
        bun-version: latest

    - name: Install dependencies
      run: |
        cd frontend && bun install
        bunx playwright install --with-deps

    - name: Build backend
      run: go build -o api ./cmd/api

    - name: Run migrations
      run: |
        go build -o migrate ./cmd/migrate
        ./migrate -dsn "postgres://test:test@localhost:5432/openaccounting_test?sslmode=disable"

    - name: Start backend
      run: |
        ./api &
        sleep 5
      env:
        DATABASE_URL: postgres://test:test@localhost:5432/openaccounting_test?sslmode=disable
        JWT_SECRET: test-secret-key

    - name: Build frontend
      run: cd frontend && bun run build

    - name: Run E2E tests
      run: cd frontend && bunx playwright test
      env:
        BASE_URL: http://localhost:3000

    - name: Upload test results
      uses: actions/upload-artifact@v4
      if: failure()
      with:
        name: playwright-report
        path: frontend/playwright-report/
        retention-days: 7
```

#### Playwright Configuration

```typescript
// frontend/playwright.config.ts
import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: 'html',

  use: {
    baseURL: process.env.BASE_URL || 'http://localhost:5173',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
    {
      name: 'firefox',
      use: { ...devices['Desktop Firefox'] },
    },
    {
      name: 'webkit',
      use: { ...devices['Desktop Safari'] },
    },
    {
      name: 'Mobile Chrome',
      use: { ...devices['Pixel 5'] },
    },
    {
      name: 'Mobile Safari',
      use: { ...devices['iPhone 12'] },
    },
  ],

  webServer: {
    command: 'bun run dev',
    url: 'http://localhost:5173',
    reuseExistingServer: !process.env.CI,
  },
});
```

---

## Implementation Order

| Phase | Name | Priority | Dependencies | Est. Days |
|-------|------|----------|--------------|-----------|
| 2.4 | E2E Testing Setup | High | None | 2 |
| 2.1 | Dashboard Improvements | High | None | 4 |
| 2.2 | Report Generation | High | 2.1 (period selector) | 5 |
| 2.3 | Mobile Responsiveness | Medium | 2.1, 2.2 | 3 |
| 2.4+ | E2E Test Coverage | High | 2.1, 2.2, 2.3 | 3 |

**Recommended Order:**
1. Set up E2E testing infrastructure first (enables testing all subsequent work)
2. Dashboard improvements (most visible user value)
3. Report generation (extends analytics capabilities)
4. Mobile responsiveness (polish existing features)
5. Comprehensive E2E test coverage for all new features

---

## Success Criteria

### Dashboard
- [ ] Period selector works and persists selection
- [ ] All new summary cards display correct data
- [ ] Charts render without errors
- [ ] Activity feed shows recent items
- [ ] All metrics calculated correctly

### Reports
- [ ] PDF export generates valid document
- [ ] Excel export opens correctly
- [ ] All report types render accurately
- [ ] Date range filtering works
- [ ] Comparison data shows correctly

### Mobile
- [ ] All pages usable on 375px viewport
- [ ] Navigation accessible via hamburger menu
- [ ] Tables transform to card view
- [ ] Forms are touch-friendly
- [ ] No horizontal scroll on main content

### E2E Testing
- [ ] Playwright configured and running locally
- [ ] Core user flows covered (auth, CRUD, reports)
- [ ] CI pipeline runs E2E tests on PRs
- [ ] Failed tests produce useful artifacts
- [ ] Tests pass in < 10 minutes

---

## Files to Create/Modify

### New Files
```
frontend/e2e/                           # E2E test directory
frontend/playwright.config.ts           # Playwright configuration
frontend/src/lib/components/PeriodSelector.svelte
frontend/src/lib/components/ExportButton.svelte
frontend/src/lib/components/ActivityFeed.svelte
frontend/src/lib/components/MobileNav.svelte
frontend/src/lib/stores/viewport.ts
frontend/src/routes/reports/aging/+page.svelte
frontend/src/routes/reports/ledger/+page.svelte
frontend/src/routes/reports/vat/+page.svelte
internal/analytics/cashflow.go
internal/analytics/aging.go
internal/analytics/pnl.go
cmd/api/handlers_analytics.go
```

### Modified Files
```
.github/workflows/ci.yml                # Add E2E job
frontend/package.json                   # Add Playwright, xlsx
frontend/src/app.css                    # Mobile improvements
frontend/src/routes/+layout.svelte      # Mobile nav integration
frontend/src/routes/dashboard/+page.svelte
frontend/src/routes/reports/+page.svelte
frontend/messages/en.json               # New i18n keys
frontend/messages/et.json               # Estonian translations
```

---

## Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|------------|
| E2E tests flaky in CI | Medium | Use retries, proper waits, stable selectors |
| PDF generation browser-dependent | Low | Test across browsers, fallback to server-side |
| Mobile CSS conflicts | Medium | Use BEM naming, scope styles properly |
| API performance for analytics | High | Add database indexes, caching layer |
| Chart.js bundle size | Low | Dynamic imports, tree shaking |

---

## Definition of Done

Each feature is complete when:
1. Code implemented and working locally
2. Unit tests added for new logic
3. E2E tests covering happy path
4. i18n keys added for both languages
5. Mobile responsive
6. Code reviewed and approved
7. CI pipeline passing
8. Documentation updated
