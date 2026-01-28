# Frontend Test Coverage Improvement Plan

> **Date:** 2026-01-10
> **Status:** Draft
> **Goal:** Achieve comprehensive test coverage for all frontend functionality with a systematic tracking system

---

## CI Status Alert

**Latest CI Run:** [#20880383327](https://github.com/HMB-research/open-accounting/actions/runs/20880383327)
**Date:** 2026-01-10
**Result:** ❌ 50 failed / 198 passed / 12 skipped / 1 flaky

### Failure Analysis

| Failure Type | Count | Root Cause |
|--------------|-------|------------|
| Table data not loading | ~30 | Demo data not seeding properly |
| Page load timeout | ~15 | API responses failing/slow |
| Element not visible | ~5 | Authentication issues |

### Affected Test Suites
- `demo-all-views.spec.ts` - Multiple view tests timing out
- `demo/recurring.spec.ts` - All seed data verification tests failing
- `demo/tsd.spec.ts` - All declaration tests failing
- `demo/payment-reminders.spec.ts` - Display tests failing
- `demo/plugins-settings.spec.ts` - Structure test failing
- `demo-env.spec.ts` - Onboarding/viewport tests failing

### Immediate Action Required
1. **Investigate demo reset endpoint** - Tests expect seeded data but tables are empty
2. **Check API health in CI** - Many pages timing out waiting for data
3. **Review authentication flow** - Some tests fail before reaching content

---

## Executive Summary

This plan establishes a structured approach to:
1. Achieve 80%+ unit test coverage for frontend components, utilities, and stores
2. Ensure E2E coverage for all user-facing functionality
3. Implement a progress tracking system to document development status

---

## Current State Analysis

### Test Coverage Summary

| Category | Items | Tested | Coverage |
|----------|-------|--------|----------|
| **Routes/Pages** | 35 | 15 (E2E only) | 43% |
| **Components** | 9 | 1 | 11% |
| **Utility Functions** | 18+ | 0 | 0% |
| **Stores** | 1 | 0 | 0% |
| **API Client** | 100+ methods | ~95% | Excellent |

### What IS Well Tested
- **API Client** (`lib/api.ts`) - 1,957 lines of comprehensive tests
- **Plugin System** (`lib/plugins/manager.ts`) - 446 lines of tests
- **i18n/Localization** - Translation completeness, locale switching
- **E2E Flows** - Major navigation paths covered

### Critical Gaps
1. **8 of 9 components** lack unit tests
2. **All utility functions** untested (dates, tenant, auth)
3. **Auth store** - Critical functionality, zero tests
4. **Form validation** - No testing coverage
5. **Error boundaries** - Limited testing

---

## Phase 1: Core Infrastructure Tests (Priority: Critical)

### 1.1 Auth Store Tests
**File:** `frontend/src/tests/stores/auth.test.ts`
**Target Coverage:** 100%

| Function | Test Cases |
|----------|------------|
| `setTokens()` | Valid tokens, remember-me option, storage selection |
| `updateAccessToken()` | Token refresh, concurrent access |
| `clearTokens()` | Complete cleanup, storage clearing |
| `getAccessToken()` | Retrieval from both storage types |
| `getRefreshToken()` | Retrieval, expiry handling |
| `isAuthenticated` | Derived state accuracy |

### 1.2 Date Utilities Tests
**File:** `frontend/src/tests/utils/dates.test.ts`
**Target Coverage:** 100%

| Function | Test Cases |
|----------|------------|
| `getTodayISO()` | Returns correct format |
| `toISODate()` | Various date inputs, edge cases |
| `getStartOfMonth()` / `getEndOfMonth()` | Month boundaries, leap years |
| `getStartOfQuarter()` / `getEndOfQuarter()` | Quarter calculations |
| `getStartOfYear()` / `getEndOfYear()` | Year boundaries |
| `getDaysAgo()` | Positive/negative values, edge cases |
| `calculateDateRange()` | All preset periods (7 types) |
| `formatDateET()` / `formatDate()` | Locale formatting, edge cases |

### 1.3 Tenant Utilities Tests
**File:** `frontend/src/tests/utils/tenant.test.ts`
**Target Coverage:** 100%

| Function | Test Cases |
|----------|------------|
| `getTenantId()` | Valid URL, missing tenant, invalid format |
| `requireTenantId()` | Success case, error throwing |
| `parseApiError()` | Various error types, fallback messages |
| `createActionHandler()` | Wrapper behavior, error propagation |

---

## Phase 2: Component Tests (Priority: High)

### 2.1 Critical Components

| Component | Priority | Test File | Key Test Cases |
|-----------|----------|-----------|----------------|
| **TenantSelector** | P0 | `TenantSelector.test.ts` | Tenant switching, loading states, error handling |
| **ErrorAlert** | P0 | `ErrorAlert.test.ts` | Display, dismissal, accessibility |
| **DateRangeFilter** | P1 | `DateRangeFilter.test.ts` | Range selection, presets, validation |
| **PeriodSelector** | P1 | `PeriodSelector.test.ts` | Period switching, callbacks |
| **ContactFormModal** | P1 | `ContactFormModal.test.ts` | Form validation, submission, cancel |
| **ExportButton** | P2 | `ExportButton.test.ts` | Export triggering, format options |
| **ActivityFeed** | P2 | `ActivityFeed.test.ts` | Activity rendering, pagination |
| **OnboardingWizard** | P2 | `OnboardingWizard.test.ts` | Step navigation, validation, completion |

### 2.2 Component Test Template

```typescript
// Example: TenantSelector.test.ts
import { render, fireEvent, waitFor } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import TenantSelector from '$lib/components/TenantSelector.svelte';

describe('TenantSelector', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('displays current tenant name', async () => {
    // Test implementation
  });

  it('shows tenant list on click', async () => {
    // Test implementation
  });

  it('switches tenant on selection', async () => {
    // Test implementation
  });

  it('handles loading state', async () => {
    // Test implementation
  });

  it('displays error state appropriately', async () => {
    // Test implementation
  });
});
```

---

## Phase 3: E2E Test Completion (Priority: High)

### 3.1 Pages Missing E2E Coverage

| Route | Current Status | Test File Needed |
|-------|---------------|------------------|
| `/employees/absences` | Not tested | `absences.spec.ts` |
| `/payroll/calculator` | Minimal | Enhance `salary-calculator.spec.ts` |
| `/assets` | Basic only | Enhance `fixed-assets.spec.ts` |
| `/quotes` | Basic only | Enhance `quotes.spec.ts` |
| `/orders` | Basic only | Enhance `orders.spec.ts` |
| `/reports/balance-confirmations` | Not tested | `balance-confirmations.spec.ts` |
| `/reports/cash-flow` | Not tested | `cash-flow.spec.ts` |
| `/settings/cost-centers` | Not tested | `cost-centers.spec.ts` |

### 3.2 E2E Test Patterns

All E2E tests should follow the established demo configuration pattern:

```typescript
// Example structure for new E2E tests
import { test, expect } from '@playwright/test';
import { navigateTo, login } from './utils';

test.describe('Feature Name', () => {
  test.beforeEach(async ({ page }) => {
    await login(page, 'demo1@example.com', 'demo12345');
  });

  test('displays page elements correctly', async ({ page }) => {
    await navigateTo(page, '/route');
    await expect(page.getByRole('heading')).toBeVisible();
  });

  test('handles user interaction', async ({ page }) => {
    // Interaction test
  });

  test('validates form input', async ({ page }) => {
    // Validation test
  });
});
```

---

## Phase 4: Progress Tracking System

### 4.1 Test Coverage Tracking File

Create and maintain: `frontend/TEST_COVERAGE.md`

```markdown
# Frontend Test Coverage Status

> Last Updated: YYYY-MM-DD
> Overall Coverage: XX%

## Coverage by Category

### Unit Tests

| File | Coverage | Status | Last Updated |
|------|----------|--------|--------------|
| `lib/api.ts` | 95% | ✅ Complete | 2026-01-10 |
| `lib/plugins/manager.ts` | 90% | ✅ Complete | 2026-01-10 |
| `stores/auth.ts` | 0% | ❌ Not Started | - |
| `utils/dates.ts` | 0% | ❌ Not Started | - |
| `utils/tenant.ts` | 0% | ❌ Not Started | - |

### Component Tests

| Component | Coverage | Status | Last Updated |
|-----------|----------|--------|--------------|
| LanguageSelector | 80% | ✅ Complete | 2026-01-10 |
| TenantSelector | 0% | ❌ Not Started | - |
| ErrorAlert | 0% | ❌ Not Started | - |
| DateRangeFilter | 0% | ❌ Not Started | - |
| ... | ... | ... | ... |

### E2E Tests

| Route | Status | Test File | Notes |
|-------|--------|-----------|-------|
| `/dashboard` | ✅ Covered | `dashboard.spec.ts` | Full coverage |
| `/invoices` | ✅ Covered | `invoices.spec.ts` | Full coverage |
| `/employees/absences` | ❌ Missing | - | Needs implementation |
| ... | ... | ... | ... |
```

### 4.2 Development Status Tracking File

Create and maintain: `docs/DEVELOPMENT_STATUS.md`

```markdown
# Open Accounting Development Status

> Last Updated: YYYY-MM-DD

## Feature Completion Status

### Core Accounting
| Feature | Status | Tests | Notes |
|---------|--------|-------|-------|
| Chart of Accounts | ✅ Complete | ✅ E2E | - |
| Journal Entries | ✅ Complete | ✅ E2E | - |
| Trial Balance | ✅ Complete | ✅ E2E | - |

### Business Operations
| Feature | Status | Tests | Notes |
|---------|--------|-------|-------|
| Invoicing | ✅ Complete | ✅ E2E | - |
| Quotes | ⚠️ Partial | ⚠️ Basic | Needs workflow tests |
| Orders | ⚠️ Partial | ⚠️ Basic | Needs CRUD tests |

### Areas Needing Finalization

1. **Quotes Module**
   - [ ] Quote-to-order conversion UI
   - [ ] Email quote functionality
   - [ ] Quote PDF generation

2. **Orders Module**
   - [ ] Order status workflow
   - [ ] Order-to-invoice conversion

3. **Fixed Assets**
   - [ ] Depreciation scheduling
   - [ ] Disposal workflow

4. **Inventory**
   - [ ] Stock level tracking
   - [ ] Warehouse management
```

---

## Implementation Timeline

### Week 1: Foundation
- [ ] Create `TEST_COVERAGE.md` and `DEVELOPMENT_STATUS.md`
- [ ] Implement auth store tests
- [ ] Implement date utilities tests
- [ ] Implement tenant utilities tests

### Week 2: Components
- [ ] TenantSelector tests
- [ ] ErrorAlert tests
- [ ] DateRangeFilter tests
- [ ] PeriodSelector tests

### Week 3: Components Continued
- [ ] ContactFormModal tests
- [ ] ExportButton tests
- [ ] OnboardingWizard tests
- [ ] ActivityFeed tests

### Week 4: E2E Completion
- [ ] Missing page E2E tests
- [ ] Enhanced coverage for existing tests
- [ ] Form validation E2E tests

### Week 5: Documentation & Review
- [ ] Update all tracking documents
- [ ] Coverage report generation
- [ ] Gap analysis and remediation

---

## Success Metrics

| Metric | Current | Target | Method |
|--------|---------|--------|--------|
| Unit Test Coverage | ~40% | 80%+ | `vitest --coverage` |
| Component Test Coverage | 11% | 90%+ | Per-component tracking |
| E2E Page Coverage | 43% | 95%+ | Route verification |
| Critical Path Coverage | Unknown | 100% | Manual verification |

---

## Tooling Requirements

### Already Configured
- Vitest for unit testing
- Playwright for E2E testing
- Testing Library for component tests

### To Add
- [ ] Coverage thresholds in `vitest.config.ts`
- [ ] Coverage reporting in CI
- [ ] Automatic coverage badge updates

### Vitest Coverage Configuration

```typescript
// vitest.config.ts additions
export default defineConfig({
  test: {
    coverage: {
      provider: 'v8',
      reporter: ['text', 'json', 'html'],
      thresholds: {
        global: {
          branches: 70,
          functions: 80,
          lines: 80,
          statements: 80
        }
      },
      include: ['src/lib/**/*.ts', 'src/lib/**/*.svelte'],
      exclude: ['src/lib/paraglide/**']
    }
  }
});
```

---

## Appendix: Test File Inventory

### Current Test Files (6 total, 3,187 lines)
1. `src/tests/lib/api.test.ts` - 1,957 lines
2. `src/tests/lib/plugins.test.ts` - 446 lines
3. `src/tests/components/LanguageSelector.test.ts` - 71 lines
4. `src/tests/i18n/messages.test.ts` - 249 lines
5. `src/tests/i18n/translation-completeness.test.ts` - 152 lines
6. `src/tests/recurring/email-config.test.ts` - 312 lines

### Planned New Test Files
1. `src/tests/stores/auth.test.ts`
2. `src/tests/utils/dates.test.ts`
3. `src/tests/utils/tenant.test.ts`
4. `src/tests/components/TenantSelector.test.ts`
5. `src/tests/components/ErrorAlert.test.ts`
6. `src/tests/components/DateRangeFilter.test.ts`
7. `src/tests/components/PeriodSelector.test.ts`
8. `src/tests/components/ContactFormModal.test.ts`
9. `src/tests/components/ExportButton.test.ts`
10. `src/tests/components/OnboardingWizard.test.ts`
11. `src/tests/components/ActivityFeed.test.ts`

### E2E Enhancement Files
1. `e2e/demo/absences.spec.ts` - New
2. `e2e/demo/balance-confirmations.spec.ts` - New
3. `e2e/demo/cash-flow.spec.ts` - New
4. `e2e/demo/cost-centers.spec.ts` - New
