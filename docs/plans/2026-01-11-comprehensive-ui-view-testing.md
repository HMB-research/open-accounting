# Comprehensive UI View Testing Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Systematically test all 33 UI views in the open-accounting application, document which views work correctly vs. have issues, and create a structured issues report.

**Architecture:** Execute manual E2E tests against the live Railway demo environment for each view. For each view, test: (1) page loads, (2) data displays correctly, (3) all buttons/links work, (4) CRUD operations function, (5) error states handled. Document findings in a structured markdown report.

**Tech Stack:** Playwright E2E tests, Railway demo environment (https://open-accounting.up.railway.app), GitHub Issues for tracking

---

## Phase 1: Setup Testing Infrastructure

### Task 1: Create Issues Documentation Template

**Files:**
- Create: `docs/UI_ISSUES_REPORT.md`

**Step 1: Create the issues tracking document**

Create `docs/UI_ISSUES_REPORT.md`:

```markdown
# UI Views Issues Report

> Last Updated: 2026-01-11
> Tested Against: Railway Demo Environment

## Summary

| Category | Working | Issues | Not Tested |
|----------|---------|--------|------------|
| Landing/Auth | 0/2 | 0 | 2 |
| Core Accounting | 0/6 | 0 | 6 |
| Business Operations | 0/8 | 0 | 8 |
| Payroll | 0/4 | 0 | 4 |
| Banking | 0/2 | 0 | 2 |
| Reports | 0/3 | 0 | 3 |
| Settings | 0/5 | 0 | 5 |
| Admin | 0/1 | 0 | 1 |
| **Total** | **0/33** | **0** | **33** |

---

## Testing Criteria

Each view is tested for:
1. **Page Load** - Does the page load without errors?
2. **Data Display** - Does data render correctly in tables/lists?
3. **Navigation** - Do all links/buttons navigate correctly?
4. **CRUD Operations** - Can you Create, Read, Update, Delete?
5. **Error Handling** - Are errors displayed appropriately?
6. **Responsive** - Does it work on mobile viewport?

### Status Legend
- ✅ **Working** - All criteria pass
- ⚠️ **Partial** - Some issues exist (see notes)
- ❌ **Broken** - Critical issues prevent usage
- 🔲 **Not Tested** - Awaiting testing

---

## Detailed View Reports

### Landing & Authentication

#### / (Landing Page)
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | N/A | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /login
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | N/A | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

### Core Accounting

#### /dashboard
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | N/A | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /accounts
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /journal
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /invoices
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /invoices/reminders
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /contacts
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

### Business Operations

#### /quotes
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Known Issues:**
- Quote-to-Order conversion needs verification
- Email quote functionality needs implementation
- Quote PDF generation needs verification

**Overall:** 🔲 Not Tested

---

#### /orders
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Known Issues:**
- Order-to-Invoice conversion needs verification
- Order status workflow needs testing

**Overall:** 🔲 Not Tested

---

#### /payments
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /payments/cash
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /recurring
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /assets
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /inventory
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Known Issues:**
- Stock level tracking not implemented
- Warehouse management not implemented

**Overall:** 🔲 Not Tested

---

### Payroll

#### /employees
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /employees/absences
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Known Issues:**
- Leave balance tracking needs verification

**Overall:** 🔲 Not Tested

---

#### /payroll
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /payroll/calculator
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | N/A | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /tsd
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

### Banking

#### /banking
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /banking/import
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

### Tax & Compliance

#### /tax
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | N/A | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /vat-returns
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

### Reports

#### /reports
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | N/A | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /reports/balance-confirmations
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /reports/cash-flow
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | N/A | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

### Settings

#### /settings
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | N/A | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /settings/company
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /settings/email
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /settings/plugins
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

#### /settings/cost-centers
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Known Issues:**
- Cost center assignment to transactions needs UI

**Overall:** 🔲 Not Tested

---

### Admin

#### /admin/plugins
| Criteria | Status | Notes |
|----------|--------|-------|
| Page Load | 🔲 | |
| Data Display | 🔲 | |
| Navigation | 🔲 | |
| CRUD | 🔲 | |
| Errors | 🔲 | |
| Responsive | 🔲 | |

**Overall:** 🔲 Not Tested

---

## Issues Summary

### Critical Issues (Blocking)
_None identified yet_

### Major Issues (Functional Problems)
_None identified yet_

### Minor Issues (Polish/UX)
_None identified yet_

---

## Change Log

| Date | Tester | Changes |
|------|--------|---------|
| 2026-01-11 | - | Initial template created |
```

**Step 2: Commit the template**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs: add UI issues report template for systematic testing"
```

---

## Phase 2: Authentication & Landing Tests

### Task 2: Test Landing Page (/)

**Files:**
- Modify: `docs/UI_ISSUES_REPORT.md`

**Step 1: Run automated E2E test**

Run: `cd frontend && npx playwright test --config=playwright.demo.config.ts e2e/demo-all-views.spec.ts -g "Landing" --headed`
Expected: Observe page behavior and note any issues

**Step 2: Manual verification checklist**

Open in browser: `https://open-accounting.up.railway.app`

Check:
- [ ] Page loads without console errors
- [ ] Hero section visible
- [ ] Navigation links work
- [ ] "Try Demo" or login link visible
- [ ] Responsive on mobile (375px width)

**Step 3: Update report with findings**

Update `docs/UI_ISSUES_REPORT.md` landing page section with actual results.

**Step 4: Commit**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs(testing): complete landing page testing"
```

---

### Task 3: Test Login Page (/login)

**Files:**
- Modify: `docs/UI_ISSUES_REPORT.md`

**Step 1: Run automated E2E test**

Run: `cd frontend && npx playwright test --config=playwright.demo.config.ts e2e/demo-all-views.spec.ts -g "Login" --headed`

**Step 2: Manual verification checklist**

Open: `https://open-accounting.up.railway.app/login`

Check:
- [ ] Email field accepts input
- [ ] Password field accepts input
- [ ] Password visibility toggle works
- [ ] Sign In button submits form
- [ ] Error shown for invalid credentials
- [ ] Successful login redirects to dashboard
- [ ] "Remember me" checkbox works
- [ ] Responsive on mobile

**Step 3: Update report and commit**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs(testing): complete login page testing"
```

---

## Phase 3: Core Accounting Views

### Task 4: Test Dashboard (/dashboard)

**Files:**
- Modify: `docs/UI_ISSUES_REPORT.md`

**Step 1: Run automated test**

Run: `cd frontend && npx playwright test --config=playwright.demo.config.ts e2e/demo/dashboard.spec.ts --headed`

**Step 2: Manual verification**

Login and navigate to `/dashboard?tenant=b0000000-0000-0000-0001-000000000001`

Check:
- [ ] Page loads with tenant selector
- [ ] Summary cards display data
- [ ] Charts render (if any)
- [ ] Recent activity shows
- [ ] Quick action buttons work
- [ ] All navigation links work

**Step 3: Update report and commit**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs(testing): complete dashboard testing"
```

---

### Task 5: Test Accounts/Chart of Accounts (/accounts)

**Files:**
- Modify: `docs/UI_ISSUES_REPORT.md`

**Step 1: Run automated test**

Run: `cd frontend && npx playwright test --config=playwright.demo.config.ts e2e/demo/accounts.spec.ts --headed`

**Step 2: Manual verification**

Check:
- [ ] Account list loads
- [ ] Account hierarchy displays correctly
- [ ] Can create new account
- [ ] Can edit existing account
- [ ] Can delete account (if allowed)
- [ ] Account types filter works
- [ ] Search/filter functionality

**Step 3: Update report and commit**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs(testing): complete accounts page testing"
```

---

### Task 6: Test Journal Entries (/journal)

**Files:**
- Modify: `docs/UI_ISSUES_REPORT.md`

**Step 1: Run automated test**

Run: `cd frontend && npx playwright test --config=playwright.demo.config.ts e2e/demo/journal.spec.ts --headed`

**Step 2: Manual verification**

Check:
- [ ] Journal entries list loads
- [ ] Can create new journal entry
- [ ] Debit/credit totals calculate correctly
- [ ] Can post draft entries
- [ ] Can void posted entries
- [ ] Date range filter works
- [ ] Account filter works

**Step 3: Update report and commit**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs(testing): complete journal entries testing"
```

---

### Task 7: Test Invoices (/invoices)

**Files:**
- Modify: `docs/UI_ISSUES_REPORT.md`

**Step 1: Run automated test**

Run: `cd frontend && npx playwright test --config=playwright.demo.config.ts e2e/demo/invoices.spec.ts --headed`

**Step 2: Manual verification**

Check:
- [ ] Invoice list loads with pagination
- [ ] Status filter works (Draft, Sent, Paid, etc.)
- [ ] Can create new invoice
- [ ] Line items add/remove correctly
- [ ] Totals calculate correctly
- [ ] VAT calculates correctly
- [ ] Can send invoice
- [ ] Can download PDF
- [ ] Can mark as paid

**Step 3: Update report and commit**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs(testing): complete invoices testing"
```

---

### Task 8: Test Payment Reminders (/invoices/reminders)

**Files:**
- Modify: `docs/UI_ISSUES_REPORT.md`

**Step 1: Run automated test**

Run: `cd frontend && npx playwright test --config=playwright.demo.config.ts e2e/demo/payment-reminders.spec.ts --headed`

**Step 2: Manual verification**

Check:
- [ ] Reminder list shows overdue invoices
- [ ] Can send reminder email
- [ ] Reminder templates display
- [ ] Due date calculations correct

**Step 3: Update report and commit**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs(testing): complete payment reminders testing"
```

---

### Task 9: Test Contacts (/contacts)

**Files:**
- Modify: `docs/UI_ISSUES_REPORT.md`

**Step 1: Run automated test**

Run: `cd frontend && npx playwright test --config=playwright.demo.config.ts e2e/demo/contacts.spec.ts --headed`

**Step 2: Manual verification**

Check:
- [ ] Contact list loads
- [ ] Can create new customer
- [ ] Can create new supplier
- [ ] Can edit contact details
- [ ] Can delete contact
- [ ] Customer/Supplier filter works
- [ ] Contact search works

**Step 3: Update report and commit**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs(testing): complete contacts testing"
```

---

## Phase 4: Business Operations Views

### Task 10: Test Quotes (/quotes)

**Files:**
- Modify: `docs/UI_ISSUES_REPORT.md`

**Step 1: Run automated test**

Run: `cd frontend && npx playwright test --config=playwright.demo.config.ts e2e/demo/quotes.spec.ts --headed`

**Step 2: Manual verification - CRITICAL**

Check:
- [ ] Quote list loads
- [ ] Can create new quote
- [ ] Line items work correctly
- [ ] **Can convert quote to order** (⚠️ Known issue)
- [ ] **Can email quote** (⚠️ Needs implementation)
- [ ] **Can download PDF** (⚠️ Needs verification)
- [ ] Status workflow (Draft → Sent → Accepted/Rejected)
- [ ] Expiry date handling

**Step 3: Document any issues found**

For each issue found, document:
- Steps to reproduce
- Expected behavior
- Actual behavior
- Screenshots (if applicable)

**Step 4: Update report and commit**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs(testing): complete quotes testing - document issues"
```

---

### Task 11: Test Orders (/orders)

**Files:**
- Modify: `docs/UI_ISSUES_REPORT.md`

**Step 1: Run automated test**

Run: `cd frontend && npx playwright test --config=playwright.demo.config.ts e2e/demo/orders.spec.ts --headed`

**Step 2: Manual verification - CRITICAL**

Check:
- [ ] Order list loads
- [ ] Can create new order
- [ ] Line items work correctly
- [ ] **Can convert order to invoice** (⚠️ Needs verification)
- [ ] **Order status workflow** (⚠️ Needs testing)
- [ ] Order fulfillment tracking

**Step 3: Update report and commit**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs(testing): complete orders testing - document issues"
```

---

### Task 12: Test Payments (/payments)

**Files:**
- Modify: `docs/UI_ISSUES_REPORT.md`

**Step 1: Run automated test**

Run: `cd frontend && npx playwright test --config=playwright.demo.config.ts e2e/demo/payments.spec.ts --headed`

**Step 2: Manual verification**

Check:
- [ ] Payment list loads
- [ ] Can record new payment
- [ ] Can allocate to invoice
- [ ] Payment methods display
- [ ] Bank account selection works

**Step 3: Update report and commit**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs(testing): complete payments testing"
```

---

### Task 13: Test Cash Payments (/payments/cash)

**Files:**
- Modify: `docs/UI_ISSUES_REPORT.md`

**Step 1: Run automated test**

Run: `cd frontend && npx playwright test --config=playwright.demo.config.ts e2e/demo/cash-payments.spec.ts --headed`

**Step 2: Manual verification**

Check:
- [ ] Cash payment list loads
- [ ] Can record cash receipt
- [ ] Can record cash disbursement
- [ ] Cash balance displays correctly

**Step 3: Update report and commit**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs(testing): complete cash payments testing"
```

---

### Task 14: Test Recurring Invoices (/recurring)

**Files:**
- Modify: `docs/UI_ISSUES_REPORT.md`

**Step 1: Run automated test**

Run: `cd frontend && npx playwright test --config=playwright.demo.config.ts e2e/demo/recurring.spec.ts --headed`

**Step 2: Manual verification**

Check:
- [ ] Recurring invoice list loads
- [ ] Can create new recurring template
- [ ] Frequency options work (Monthly, Quarterly, etc.)
- [ ] Can pause/resume recurring
- [ ] Next generation date displays

**Step 3: Update report and commit**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs(testing): complete recurring invoices testing"
```

---

### Task 15: Test Fixed Assets (/assets)

**Files:**
- Modify: `docs/UI_ISSUES_REPORT.md`

**Step 1: Run automated test**

Run: `cd frontend && npx playwright test --config=playwright.demo.config.ts e2e/demo/fixed-assets.spec.ts --headed`

**Step 2: Manual verification**

Check:
- [ ] Asset list loads
- [ ] Can create new asset
- [ ] Asset categories display
- [ ] Depreciation calculation shows
- [ ] Can change asset status (Draft → Active → Disposed)
- [ ] Asset details page works

**Step 3: Update report and commit**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs(testing): complete fixed assets testing"
```

---

### Task 16: Test Inventory (/inventory)

**Files:**
- Modify: `docs/UI_ISSUES_REPORT.md`

**Step 1: Run automated test**

Run: `cd frontend && npx playwright test --config=playwright.demo.config.ts e2e/demo/inventory.spec.ts --headed`

**Step 2: Manual verification - KNOWN ISSUES**

Check:
- [ ] Inventory page loads
- [ ] Product list displays
- [ ] Can create new product
- [ ] **Stock levels tracking** (⚠️ Not implemented)
- [ ] **Warehouse management** (⚠️ Not implemented)

**Step 3: Update report and commit**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs(testing): complete inventory testing - document limitations"
```

---

## Phase 5: Payroll Views

### Task 17: Test Employees (/employees)

**Files:**
- Modify: `docs/UI_ISSUES_REPORT.md`

**Step 1: Run automated test**

Run: `cd frontend && npx playwright test --config=playwright.demo.config.ts e2e/demo/employees.spec.ts --headed`

**Step 2: Manual verification**

Check:
- [ ] Employee list loads
- [ ] Can add new employee
- [ ] Employee details form works
- [ ] Tax information fields work
- [ ] Estonian tax settings (social tax, pension)

**Step 3: Update report and commit**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs(testing): complete employees testing"
```

---

### Task 18: Test Employee Absences (/employees/absences)

**Files:**
- Modify: `docs/UI_ISSUES_REPORT.md`

**Step 1: Run automated test**

Run: `cd frontend && npx playwright test --config=playwright.demo.config.ts e2e/demo/absences.spec.ts --headed`

**Step 2: Manual verification - NEEDS VERIFICATION**

Check:
- [ ] Absence list loads
- [ ] Can request new absence
- [ ] Absence types available
- [ ] **Leave balance displays** (⚠️ Needs verification)
- [ ] Approval workflow works
- [ ] Calendar view (if any)

**Step 3: Update report and commit**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs(testing): complete absences testing - verify leave balance"
```

---

### Task 19: Test Payroll Runs (/payroll)

**Files:**
- Modify: `docs/UI_ISSUES_REPORT.md`

**Step 1: Run automated test**

Run: `cd frontend && npx playwright test --config=playwright.demo.config.ts e2e/demo/payroll.spec.ts --headed`

**Step 2: Manual verification**

Check:
- [ ] Payroll run list loads
- [ ] Can create new payroll run
- [ ] Employee selection works
- [ ] Salary calculations correct
- [ ] Tax calculations correct (Estonian)
- [ ] Can approve payroll
- [ ] Can mark as paid

**Step 3: Update report and commit**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs(testing): complete payroll testing"
```

---

### Task 20: Test Salary Calculator (/payroll/calculator)

**Files:**
- Modify: `docs/UI_ISSUES_REPORT.md`

**Step 1: Run automated test**

Run: `cd frontend && npx playwright test --config=playwright.demo.config.ts e2e/demo/salary-calculator.spec.ts --headed`

**Step 2: Manual verification**

Check:
- [ ] Calculator page loads
- [ ] Gross salary input works
- [ ] Net salary calculates
- [ ] Tax breakdown displays
- [ ] Estonian tax rates applied correctly

**Step 3: Update report and commit**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs(testing): complete salary calculator testing"
```

---

### Task 21: Test TSD Declarations (/tsd)

**Files:**
- Modify: `docs/UI_ISSUES_REPORT.md`

**Step 1: Run automated test**

Run: `cd frontend && npx playwright test --config=playwright.demo.config.ts e2e/demo/tsd.spec.ts --headed`

**Step 2: Manual verification**

Check:
- [ ] TSD list loads
- [ ] Can create new declaration
- [ ] Declaration periods correct
- [ ] Can submit declaration
- [ ] XML export works
- [ ] CSV export works

**Step 3: Update report and commit**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs(testing): complete TSD testing"
```

---

## Phase 6: Banking Views

### Task 22: Test Banking (/banking)

**Files:**
- Modify: `docs/UI_ISSUES_REPORT.md`

**Step 1: Run automated test**

Run: `cd frontend && npx playwright test --config=playwright.demo.config.ts e2e/demo/banking.spec.ts --headed`

**Step 2: Manual verification**

Check:
- [ ] Bank account list loads
- [ ] Can add new bank account
- [ ] Account balances display
- [ ] Transaction list loads
- [ ] Reconciliation status shows

**Step 3: Update report and commit**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs(testing): complete banking testing"
```

---

### Task 23: Test Bank Import (/banking/import)

**Files:**
- Modify: `docs/UI_ISSUES_REPORT.md`

**Step 1: Run automated test**

Run: `cd frontend && npx playwright test --config=playwright.demo.config.ts e2e/demo/bank-import.spec.ts --headed`

**Step 2: Manual verification**

Check:
- [ ] Import page loads
- [ ] File upload works
- [ ] CSV format supported
- [ ] Column mapping works
- [ ] Preview before import
- [ ] Import completes successfully

**Step 3: Update report and commit**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs(testing): complete bank import testing"
```

---

## Phase 7: Tax & Reports Views

### Task 24: Test Tax Overview (/tax)

**Files:**
- Modify: `docs/UI_ISSUES_REPORT.md`

**Step 1: Run automated test**

Run: `cd frontend && npx playwright test --config=playwright.demo.config.ts e2e/demo/tax-overview.spec.ts --headed`

**Step 2: Manual verification**

Check:
- [ ] Tax page loads
- [ ] VAT summary displays
- [ ] Tax periods show
- [ ] Links to reports work

**Step 3: Update report and commit**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs(testing): complete tax overview testing"
```

---

### Task 25: Test VAT Returns (/vat-returns)

**Files:**
- Modify: `docs/UI_ISSUES_REPORT.md`

**Step 1: Run automated test**

Run: `cd frontend && npx playwright test --config=playwright.demo.config.ts e2e/demo/vat-returns.spec.ts --headed`

**Step 2: Manual verification**

Check:
- [ ] VAT return list loads
- [ ] Can create new return
- [ ] Period selection works
- [ ] VAT calculations correct
- [ ] Can submit return
- [ ] Export functionality works

**Step 3: Update report and commit**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs(testing): complete VAT returns testing"
```

---

### Task 26: Test Reports Hub (/reports)

**Files:**
- Modify: `docs/UI_ISSUES_REPORT.md`

**Step 1: Run automated test**

Run: `cd frontend && npx playwright test --config=playwright.demo.config.ts e2e/demo/reports.spec.ts --headed`

**Step 2: Manual verification**

Check:
- [ ] Reports page loads
- [ ] Report type links work
- [ ] Trial Balance accessible
- [ ] Balance Sheet accessible
- [ ] Income Statement accessible
- [ ] Date range selection works
- [ ] Export options work (PDF, Excel, CSV)

**Step 3: Update report and commit**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs(testing): complete reports hub testing"
```

---

### Task 27: Test Balance Confirmations (/reports/balance-confirmations)

**Files:**
- Modify: `docs/UI_ISSUES_REPORT.md`

**Step 1: Run automated test**

Run: `cd frontend && npx playwright test --config=playwright.demo.config.ts e2e/demo/balance-confirmations.spec.ts --headed`

**Step 2: Manual verification**

Check:
- [ ] Page loads
- [ ] Customer/supplier selection works
- [ ] Balance calculation correct
- [ ] Can generate confirmation letter
- [ ] Can send confirmation email

**Step 3: Update report and commit**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs(testing): complete balance confirmations testing"
```

---

### Task 28: Test Cash Flow Report (/reports/cash-flow)

**Files:**
- Modify: `docs/UI_ISSUES_REPORT.md`

**Step 1: Run automated test**

Run: `cd frontend && npx playwright test --config=playwright.demo.config.ts e2e/demo/cash-flow.spec.ts --headed`

**Step 2: Manual verification**

Check:
- [ ] Cash flow report loads
- [ ] Period selection works
- [ ] Operating activities section
- [ ] Investing activities section
- [ ] Financing activities section
- [ ] Net change calculates correctly

**Step 3: Update report and commit**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs(testing): complete cash flow testing"
```

---

## Phase 8: Settings Views

### Task 29: Test Settings Hub (/settings)

**Files:**
- Modify: `docs/UI_ISSUES_REPORT.md`

**Step 1: Run automated test**

Run: `cd frontend && npx playwright test --config=playwright.demo.config.ts e2e/demo/settings.spec.ts --headed`

**Step 2: Manual verification**

Check:
- [ ] Settings page loads
- [ ] All setting category links work
- [ ] Company settings link
- [ ] Email settings link
- [ ] Plugin settings link

**Step 3: Update report and commit**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs(testing): complete settings hub testing"
```

---

### Task 30: Test Company Settings (/settings/company)

**Files:**
- Modify: `docs/UI_ISSUES_REPORT.md`

**Step 1: Manual verification**

Check:
- [ ] Company info displays
- [ ] Can edit company name
- [ ] Can edit address
- [ ] Can edit VAT number
- [ ] Can upload logo
- [ ] Save button works

**Step 2: Update report and commit**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs(testing): complete company settings testing"
```

---

### Task 31: Test Email Settings (/settings/email)

**Files:**
- Modify: `docs/UI_ISSUES_REPORT.md`

**Step 1: Run automated test**

Run: `cd frontend && npx playwright test --config=playwright.demo.config.ts e2e/demo/email-settings.spec.ts --headed`

**Step 2: Manual verification**

Check:
- [ ] SMTP settings form loads
- [ ] Can configure SMTP
- [ ] Email templates editable
- [ ] Test email functionality

**Step 3: Update report and commit**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs(testing): complete email settings testing"
```

---

### Task 32: Test Plugin Settings (/settings/plugins)

**Files:**
- Modify: `docs/UI_ISSUES_REPORT.md`

**Step 1: Run automated test**

Run: `cd frontend && npx playwright test --config=playwright.demo.config.ts e2e/demo/plugins-settings.spec.ts --headed`

**Step 2: Manual verification**

Check:
- [ ] Plugin list loads
- [ ] Can enable/disable plugins
- [ ] Plugin configuration works
- [ ] Changes save correctly

**Step 3: Update report and commit**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs(testing): complete plugin settings testing"
```

---

### Task 33: Test Cost Centers (/settings/cost-centers)

**Files:**
- Modify: `docs/UI_ISSUES_REPORT.md`

**Step 1: Run automated test**

Run: `cd frontend && npx playwright test --config=playwright.demo.config.ts e2e/demo/cost-centers.spec.ts --headed`

**Step 2: Manual verification - KNOWN ISSUES**

Check:
- [ ] Cost center list loads
- [ ] Can create new cost center
- [ ] Can edit cost center
- [ ] Hierarchy displays
- [ ] **Assignment to transactions** (⚠️ Needs UI work)

**Step 3: Update report and commit**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs(testing): complete cost centers testing - document limitations"
```

---

### Task 34: Test Admin Plugins (/admin/plugins)

**Files:**
- Modify: `docs/UI_ISSUES_REPORT.md`

**Step 1: Manual verification**

Check:
- [ ] Admin plugins page loads
- [ ] Plugin management works
- [ ] System-level plugin config

**Step 2: Update report and commit**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs(testing): complete admin plugins testing"
```

---

## Phase 9: Final Summary

### Task 35: Generate Final Issues Summary

**Files:**
- Modify: `docs/UI_ISSUES_REPORT.md`

**Step 1: Update summary table**

Count all views tested and categorize:
- Working (✅)
- Partial Issues (⚠️)
- Broken (❌)

**Step 2: Prioritize issues**

Sort issues by severity:
- Critical (blocking functionality)
- Major (functional problems)
- Minor (polish/UX)

**Step 3: Create GitHub issues for critical/major items**

For each critical or major issue:

Run: `gh issue create --title "[VIEW] Issue description" --body "..." --label "bug"`

**Step 4: Final commit**

```bash
git add docs/UI_ISSUES_REPORT.md
git commit -m "docs(testing): complete UI view testing - final summary"
```

---

## Summary

| Phase | Tasks | Views Covered |
|-------|-------|---------------|
| Phase 1 | 1 | Setup |
| Phase 2 | 2-3 | 2 (Landing, Login) |
| Phase 3 | 4-9 | 6 (Dashboard, Accounts, Journal, Invoices, Reminders, Contacts) |
| Phase 4 | 10-16 | 7 (Quotes, Orders, Payments, Cash, Recurring, Assets, Inventory) |
| Phase 5 | 17-21 | 5 (Employees, Absences, Payroll, Calculator, TSD) |
| Phase 6 | 22-23 | 2 (Banking, Import) |
| Phase 7 | 24-28 | 5 (Tax, VAT, Reports, Balance, Cash Flow) |
| Phase 8 | 29-34 | 6 (Settings hub, Company, Email, Plugins, Cost Centers, Admin) |
| Phase 9 | 35 | Final Summary |
| **Total** | **35** | **33 Views** |
