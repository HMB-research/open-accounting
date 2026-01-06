# Accountant View Audit Report

**Date:** 2026-01-05
**Auditor:** Claude Code
**User:** demo2@example.com (Demo Company 2)

## Executive Summary

Audited all 12 views from an accountant's perspective. Found **14 issues** across 8 views, with **2 critical bugs**, **5 high-priority issues**, and **7 medium/low issues**.

---

## Findings by View

### 1. Dashboard (Töölaud)
**Status:** Partial issues

| Issue | Severity | Description |
|-------|----------|-------------|
| Revenue/Expenses show 0 € | High | Summary cards show no data despite invoices existing |
| Cash flow chart empty | High | No data displayed in Rahavoog section |
| Revenue vs Expenses chart empty | High | 6-month chart shows no bars |
| Activity feed empty | Medium | Shows "Viimased tegevused puuduvad" despite transactions |

**Working:** Receivables (33,931 €), Invoice status counts, Quick action links

---

### 2. Accounts (Kontoplaan)
**Status:** Functional with gaps

| Issue | Severity | Description |
|-------|----------|-------------|
| No account balances | High | Accountants need to see current balances per account |
| Mixed languages | Low | Account names in English, UI in Estonian |

**Working:** 33 accounts grouped by type, system account badges, create account button

---

### 3. Journal (Pearaamat)
**Status:** No data

| Issue | Severity | Description |
|-------|----------|-------------|
| No journal entries | Medium | Demo data should include sample entries |

**Working:** Correct title "Pearaamat", create entry button, empty state message

---

### 4. Contacts (Kontaktid)
**Status:** Fully functional

**Working:** 7 contacts, type filtering, search, all fields displayed correctly

---

### 5. Invoices (Arved)
**Status:** Functional with bug

| Issue | Severity | Description |
|-------|----------|-------------|
| Client column shows "-" | Medium | Customer/contact name not displayed |

**Working:** 9 invoices, status badges, PDF download, send button, filters

---

### 6. Payments (Maksed)
**Status:** Functional

| Issue | Severity | Description |
|-------|----------|-------------|
| All payment methods "Muu" | Low | More specific methods would be helpful |

**Working:** 4 payments, contact names, invoice references, unallocated amounts

---

### 7. Employees (Töötajad)
**Status:** Data issue

| Issue | Severity | Description |
|-------|----------|-------------|
| Pension rates incorrect | Medium | Shows 400%, 200% instead of 0-6% Estonian rates |

**Working:** 4 employees, tax settings, positions, active filter, search

---

### 8. Payroll (Palgaarvestused)
**Status:** Needs year navigation

| Issue | Severity | Description |
|-------|----------|-------------|
| Default year 2026 empty | Low | Should default to year with data or most recent |

**Working:** Year filter, Estonian tax rates reference card, create button

---

### 9. Recurring Invoices (Püsiarved)
**Status:** Date logic issue

| Issue | Severity | Description |
|-------|----------|-------------|
| Next generation dates in past | Medium | Shows 01/01/2025 instead of future dates |

**Working:** 3 recurring invoices, frequencies, action buttons, generated counts

---

### 10. Banking (Panga võrdlemine)
**Status:** Critical bug

| Issue | Severity | Description |
|-------|----------|-------------|
| Balance shows "EUR NaN" | **Critical** | Calculation error displaying NaN |
| Status text in English | Medium | UNMATCHED/MATCHED/RECONCILED should be Estonian |

**Working:** 8 transactions, filters, match/unmatch actions, auto-match button

---

### 11. Reports (Finantsaruanded)
**Status:** Functional with concerns

| Issue | Severity | Description |
|-------|----------|-------------|
| Share Capital in Debit | Medium | Equity typically has credit balance (verify) |
| Mixed languages | Low | Account names in English |

**Working:** Trial balance generates, balance verification, export button, date picker

---

### 12. TSD (TSD deklaratsioonid)
**Status:** Fully functional

**Working:** 3 declarations (Oct-Dec 2024), tax breakdown, XML/CSV export, status badges, mark as submitted

---

## Priority Action Items

### Critical (Fix Immediately)
1. **Banking: EUR NaN balance** - Fix calculation in banking/+page.svelte

### High Priority
2. **Dashboard: Revenue/Expenses analytics** - Ensure journal entries feed into analytics
3. **Dashboard: Charts empty** - Revenue vs Expenses and Cash flow need data
4. **Accounts: Add account balances** - Display current balance per account

### Medium Priority
5. **Invoices: Client column empty** - Display contact name
6. **Employees: Pension rate display** - Fix percentage display (likely *100 issue)
7. **Recurring: Next generation dates** - Calculate future dates correctly
8. **Banking: Translate status text** - UNMATCHED → Seostamata, etc.
9. **Journal: Add demo entries** - Seed sample journal entries

### Low Priority
10. **Mixed language account names** - Translate or keep consistent
11. **Payroll: Default to year with data**
12. **Payment methods: Add variety to demo data**

---

## Data Gaps Identified

The demo data is missing:
1. **Journal entries** - No manual entries to show ledger functionality
2. **Revenue/expense transactions** - Need dated entries for analytics
3. **Activity feed data** - Recent transactions not showing

---

## Recommendations

1. **Run demo reset** with updated seed data (dynamic dates added to handlers.go)
2. **Fix Banking NaN bug** - likely null/undefined in balance calculation
3. **Add account balances column** - critical for accountant workflow
4. **Translate English strings** - account names, status badges
5. **Test recurring invoice date logic** - ensure future dates calculated

---

## Views Summary

| View | Status | Issues |
|------|--------|--------|
| Dashboard | Partial | 4 |
| Accounts | Functional | 2 |
| Journal | No data | 1 |
| Contacts | **OK** | 0 |
| Invoices | Functional | 1 |
| Payments | Functional | 1 |
| Employees | Data issue | 1 |
| Payroll | Functional | 1 |
| Recurring | Date issue | 1 |
| Banking | **Critical bug** | 2 |
| Reports | Functional | 2 |
| TSD | **OK** | 0 |

**Total Issues: 14** (2 Critical, 5 High, 7 Medium/Low)
