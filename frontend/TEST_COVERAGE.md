# Frontend Test Coverage Status

> Last Updated: 2026-06-14
> Unit Tests: 572 tests across 32 files
> E2E Tests: 33 demo spec files plus 1 blocking smoke spec

## Quick Stats

| Metric | Current | Target |
|--------|---------|--------|
| Unit Test Files | 32 | Keep current with new shared UI and API-client behavior |
| Unit Tests | 572 | Keep increasing with every frontend workflow change |
| Component Inventory | 20/20 shared components tested | Maintain |
| Utility Coverage | dates, formatting, tenant tested | Maintain |
| Store Coverage | auth store tested | Maintain |

## Unit Test Coverage

### Components

| Component | Status | Notes |
|-----------|--------|-------|
| `ActivityFeed` | Tested | Loading, empty, relative time, and amount display |
| `AccountantPortfolioPanel` | Tested | Cross-tenant review rollup and empty state |
| `AccountantReviewPanel` | Tested | Review queues, follow-up updates, reminders |
| `ContactFormModal` | Tested | Create, edit, API errors, and cancel close |
| `DateRangeFilter` | Tested | Presets, manual range edits, clear action |
| `DocumentManager` | Tested | Upload, download, delete, evidence approval |
| `DocumentReviewQueuePanel` | Tested | Review queue, filters, retention metadata |
| `ErrorAlert` | Tested | Display logic and accessibility attributes |
| `ExportButton` | Tested | Menu behavior, CSV escaping, PDF print action |
| `FormModal` | Tested | Dialog content, footer, backdrop, and Escape close |
| `LanguageSelector` | Tested | Locale switching and labels |
| `LineItemsEditor` | Tested | Add/remove lines, totals, discounts, and immutable edits |
| `MigrationWorkbench` | Tested | Bundle assembly, planning, saved-run monitoring, and resume execution |
| `OnboardingWizard` | Tested | Required fields, setup steps, completion |
| `PeriodSelector` | Tested | Shared period ranges and custom date edits |
| `SetupCenter` | Tested | Setup progress and guided setup action |
| `StatusBadge` | Tested | Configured labels, fallback status, sizes |
| `TenantSelector` | Tested | Tenant data, URL params, loading/error state |
| `WorkflowHero` | Tested | Rendered action surface |
| `YearEndClosePanel` | Tested | Ready, complete, and blocked close states |

All tracked shared components currently have focused component tests.

### Utilities, Stores, And API Client

| Area | Test File |
|------|-----------|
| API client | `src/tests/lib/api.test.ts` |
| API retry behavior | `src/tests/lib/api-retry.test.ts` |
| Plugin manager | `src/tests/lib/plugins.test.ts` |
| Auth store | `src/tests/stores/auth.test.ts` |
| Date utilities | `src/tests/utils/dates.test.ts` |
| Formatting utilities | `src/tests/utils/formatting.test.ts` |
| Tenant utilities | `src/tests/utils/tenant.test.ts` |
| List-page composable | `src/tests/composables/useListPage.test.ts` |

### i18n And Configuration

| Area | Test File |
|------|-----------|
| Translation completeness | `src/tests/i18n/translation-completeness.test.ts` |
| Message behavior | `src/tests/i18n/messages.test.ts` |
| Recurring email config labels | `src/tests/recurring/email-config.test.ts` |

## E2E Test Coverage

The local seeded demo suite is the authoritative UI workflow gate. Current inventory:

- 33 files under `frontend/e2e/demo/*.spec.ts`
- 1 blocking smoke file under `frontend/e2e/smoke/*.spec.ts`
- `auth.setup.ts` prepares reusable demo-user auth state for demo and smoke projects
- The full demo project is sharded across 4 CI workers

Covered demo workflow files include:

`absences`, `accounts`, `balance-confirmations`, `bank-import`, `banking`, `cash-flow`, `cash-payments`, `contacts`, `cost-centers`, `dashboard`, `data-verification`, `documents`, `email-settings`, `employees`, `fixed-assets`, `inventory`, `invoices`, `journal`, `mobile`, `orders`, `payment-reminders`, `payments`, `payroll`, `plugins-settings`, `quotes`, `recurring`, `reports`, `reset`, `salary-calculator`, `settings`, `tax-overview`, `tsd`, and `vat-returns`.

## Running Tests

```bash
cd frontend

# One-time generated localization for prepared gates
bun run paraglide

# Fast focused inner loop
bun run test:prepared -- src/tests/components/<file>.test.ts

# Frontend stage closeout
bun run lint
bun run check:prepared
bun run test:prepared
bun run build:prepared

# Demo E2E gates
bun run test:e2e:smoke
bunx playwright test --config=playwright.demo.config.ts --project=demo-chromium --shard=1/4
```

Run Paraglide/SvelteKit-writing gates serially. `check`, `test`, and `build` can touch generated `src/lib/paraglide` or `.svelte-kit` files.

## Progress Log

| Date | Change | Tests Added |
|------|--------|-------------|
| 2026-06-10 | Completed shared component inventory coverage for activity feed, contact modal, generic modal, and line-item editor; removed non-reactive line-item bindings | 15 tests |
| 2026-06-10 | Added shared UI control coverage for date ranges, periods, export, and status badges; fixed local-date range formatting | 10 tests |
| 2026-06-10 | Refreshed frontend test inventory from the current tree | 0 |
| 2026-06-14 | Added migration workbench coverage for historical cutover launch and monitoring controls | 3 tests |
| 2026-01-23 | Migrated test commands from npm to bun | 0 |
| 2026-01-10 | Initial tracking document created | 0 |
