# Frontend Test Coverage Status

> Last Updated: 2026-01-23
> Unit Tests: 240+ passing
> E2E Tests: 32 spec files (demo configuration)

## Quick Stats

| Metric | Current | Target |
|--------|---------|--------|
| Unit Test Files | 6 | 17 |
| Unit Tests | 240 | 400+ |
| Component Coverage | 11% (1/9) | 90%+ |
| Utility Coverage | 0% (0/3) | 100% |
| Store Coverage | 0% (0/1) | 100% |

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

### Components (Needs Work)

| Component | Tests | Status | Priority |
|-----------|-------|--------|----------|
| LanguageSelector | 8 | ✅ Complete | - |
| TenantSelector | 0 | ❌ Not Started | P0 |
| ErrorAlert | 0 | ❌ Not Started | P0 |
| DateRangeFilter | 0 | ❌ Not Started | P1 |
| PeriodSelector | 0 | ❌ Not Started | P1 |
| ContactFormModal | 0 | ❌ Not Started | P1 |
| ExportButton | 0 | ❌ Not Started | P2 |
| OnboardingWizard | 0 | ❌ Not Started | P2 |
| ActivityFeed | 0 | ❌ Not Started | P2 |

### Utilities (Critical Gap)

| File | Tests | Status | Priority |
|------|-------|--------|----------|
| `utils/dates.ts` | 0 | ❌ Not Started | P0 |
| `utils/tenant.ts` | 0 | ❌ Not Started | P0 |

### Stores (Critical Gap)

| File | Tests | Status | Priority |
|------|-------|--------|----------|
| `stores/auth.ts` | 0 | ❌ Not Started | P0 |

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

### Current (6 files, 3,187 lines)
```
src/tests/
├── components/
│   └── LanguageSelector.test.ts     (71 lines)
├── i18n/
│   ├── messages.test.ts             (249 lines)
│   └── translation-completeness.test.ts (152 lines)
├── lib/
│   ├── api.test.ts                  (1,957 lines)
│   └── plugins.test.ts              (446 lines)
├── recurring/
│   └── email-config.test.ts         (312 lines)
└── setup.ts
```

### Planned (11 new files)
```
src/tests/
├── stores/
│   └── auth.test.ts                 [NEW - P0]
├── utils/
│   ├── dates.test.ts                [NEW - P0]
│   └── tenant.test.ts               [NEW - P0]
├── components/
│   ├── TenantSelector.test.ts       [NEW - P0]
│   ├── ErrorAlert.test.ts           [NEW - P0]
│   ├── DateRangeFilter.test.ts      [NEW - P1]
│   ├── PeriodSelector.test.ts       [NEW - P1]
│   ├── ContactFormModal.test.ts     [NEW - P1]
│   ├── ExportButton.test.ts         [NEW - P2]
│   ├── OnboardingWizard.test.ts     [NEW - P2]
│   └── ActivityFeed.test.ts         [NEW - P2]
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
| 2026-01-23 | Updated E2E inventory - all 32 spec files documented | 0 |
| 2026-01-23 | Migrated test commands from npm to bun | 0 |
| 2026-01-10 | Initial tracking document created | 0 |
