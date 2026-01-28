# Frontend Test Coverage Status

> Last Updated: 2026-01-23
> Unit Tests: 563 passing (41 with known issues)
> E2E Tests: 32 spec files (demo configuration)

**Known Issues**:
- SvelteKit `$env/dynamic/public` module not available in Bun test environment
- Some i18n message tests need updating for changed message content

## Quick Stats

| Metric | Current | Target |
|--------|---------|--------|
| Unit Test Files | 20 | 21 |
| Unit Tests | 584 | 600+ |
| Component Coverage | 83% (10/12) | 90%+ |
| Utility Coverage | 100% (3/3) | 100% |
| Store Coverage | 100% (1/1) | 100% |

---

## Unit Test Coverage

### Libraries (Well Tested)

| File | Tests | Status | Notes |
|------|-------|--------|-------|
| `lib/api.ts` | 134 | ✅ Complete | Comprehensive endpoint coverage |
| `lib/plugins/manager.ts` | 27 | ✅ Complete | Full plugin manager testing |

### i18n/Localization (Well Tested)

| File | Tests | Status | Notes |
|------|-------|--------|-------|
| Translation Completeness | 17 | ✅ Complete | EN/ET key matching |
| Messages | 23 | ✅ Complete | Message validation |
| Recurring Email Config | 31 | ✅ Complete | Email configuration labels |

### Components (Excellent Progress)

| Component | Tests | Status | Priority |
|-----------|-------|--------|----------|
| LanguageSelector | 8 | ✅ Complete | - |
| TenantSelector | 20+ | ✅ Complete | - |
| ErrorAlert | 16 | ✅ Complete | - |
| DateRangeFilter | 55 | ✅ Complete | - |
| LineItemsEditor | 45 | ✅ Complete | - |
| ContactFormModal | 35 | ✅ Complete | - |
| PeriodSelector | 55 | ✅ Complete | - |
| FormModal | 47 | ✅ Complete | - |
| ExportButton | 50 | ✅ Complete | - |
| ActivityFeed | 57 | ✅ Complete | - |
| OnboardingWizard | 0 | ❌ Not Started | P3 |
| StatusBadge | 0 | ❌ Not Started | P3 |

### Utilities (Complete)

| File | Tests | Status | Priority |
|------|-------|--------|----------|
| `utils/dates.ts` | 35 | ✅ Complete | - |
| `utils/tenant.ts` | 30 | ✅ Complete | - |
| `utils/formatting.ts` | 32 | ✅ Complete | - |

### Stores (Complete)

| File | Tests | Status | Priority |
|------|-------|--------|----------|
| `stores/auth.ts` | 25 | ✅ Complete | - |

---

## E2E Test Coverage

### Demo Tests (Complete - 32 spec files)

| Test File | Route | Status |
|-----------|-------|--------|
| absences.spec.ts | `/employees/absences` | ✅ Passing |
| accounts.spec.ts | `/accounts` | ✅ Passing |
| balance-confirmations.spec.ts | `/reports/balance-confirmations` | ✅ Passing |
| bank-import.spec.ts | `/banking/import` | ✅ Passing |
| banking.spec.ts | `/banking` | ✅ Passing |
| cash-flow.spec.ts | `/reports/cash-flow` | ✅ Passing |
| cash-payments.spec.ts | `/payments/cash` | ✅ Passing |
| contacts.spec.ts | `/contacts` | ✅ Passing |
| cost-centers.spec.ts | `/settings/cost-centers` | ✅ Passing |
| dashboard.spec.ts | `/dashboard` | ✅ Passing |
| email-settings.spec.ts | `/settings/email` | ✅ Passing |
| employees.spec.ts | `/employees` | ✅ Passing |
| fixed-assets.spec.ts | `/assets` | ✅ Passing |
| inventory.spec.ts | `/inventory` | ✅ Passing |
| invoices.spec.ts | `/invoices` | ✅ Passing |
| journal.spec.ts | `/journal` | ✅ Passing |
| orders.spec.ts | `/orders` | ✅ Passing |
| payment-reminders.spec.ts | `/invoices/reminders` | ✅ Passing |
| payments.spec.ts | `/payments` | ✅ Passing |
| payroll.spec.ts | `/payroll` | ✅ Passing |
| plugins-settings.spec.ts | `/settings/plugins` | ✅ Passing |
| quotes.spec.ts | `/quotes` | ✅ Passing |
| recurring.spec.ts | `/recurring` | ✅ Passing |
| reports.spec.ts | `/reports` | ✅ Passing |
| salary-calculator.spec.ts | `/payroll/calculator` | ✅ Passing |
| settings.spec.ts | `/settings` | ✅ Passing |
| tax-overview.spec.ts | `/tax` | ✅ Passing |
| tsd.spec.ts | `/tsd` | ✅ Passing |
| vat-returns.spec.ts | `/vat-returns` | ✅ Passing |

### Additional E2E Tests

| Test File | Purpose |
|-----------|---------|
| data-verification.spec.ts | Demo data presence verification |
| mobile.spec.ts | Mobile responsiveness tests |
| reset.spec.ts | Demo reset functionality |

---

## Test File Inventory

### Current (20 files, 9,000+ lines)
```
src/tests/
├── components/
│   ├── LanguageSelector.test.ts     (71 lines)
│   ├── TenantSelector.test.ts       (283 lines)
│   ├── ErrorAlert.test.ts           (179 lines)
│   ├── DateRangeFilter.test.ts      (490 lines)
│   ├── LineItemsEditor.test.ts      (420 lines)
│   ├── ContactFormModal.test.ts     (410 lines)
│   ├── PeriodSelector.test.ts       (380 lines)
│   ├── FormModal.test.ts            (270 lines)
│   ├── ExportButton.test.ts         (290 lines) ✅ NEW
│   └── ActivityFeed.test.ts         (340 lines) ✅ NEW
├── i18n/
│   ├── messages.test.ts             (249 lines)
│   └── translation-completeness.test.ts (152 lines)
├── lib/
│   ├── api.test.ts                  (1,957 lines)
│   ├── api-retry.test.ts            (200 lines)
│   └── plugins.test.ts              (446 lines)
├── recurring/
│   └── email-config.test.ts         (312 lines)
├── stores/
│   └── auth.test.ts                 (207 lines)
├── utils/
│   ├── dates.test.ts                (260 lines)
│   ├── formatting.test.ts           (266 lines)
│   └── tenant.test.ts               (269 lines)
└── setup.ts
```

### Remaining (2 files - P3 Low Priority)
```
src/tests/
├── components/
│   ├── OnboardingWizard.test.ts     [TODO - P3]
│   └── StatusBadge.test.ts          [TODO - P3]
```

---

## Running Tests

```bash
# Unit tests
bun test                    # Run all unit tests
bun run test:watch          # Watch mode
bun run test:coverage       # With coverage report

# E2E tests (requires running backend)
bun run test:e2e            # All E2E tests
bun run test:e2e:ui         # With Playwright UI
bun run test:e2e:headed     # With browser visible
```

---

## Progress Log

| Date | Change | Tests Added |
|------|--------|-------------|
| 2026-01-23 | Added ExportButton, ActivityFeed tests | 107 |
| 2026-01-23 | Added FormModal, PeriodSelector tests | 102 |
| 2026-01-23 | Added LineItemsEditor, DateRangeFilter, ContactFormModal tests | 135 |
| 2026-01-23 | Updated E2E inventory - all 32 spec files documented | 0 |
| 2026-01-23 | Migrated test commands from npm to bun | 0 |
| 2026-01-10 | Initial tracking document created | 0 |
