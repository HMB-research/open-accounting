# Feature Mapping: Merit & SmartAccounts vs Open Accounting

This document maps features from [Merit Aktiva](https://www.merit.ee/en/) and [SmartAccounts](https://www.smartaccounts.eu/en/) to Open Accounting, identifying implementation status, gaps, and blockers.

This is a competitive-gap document, not the authoritative current-state status page. For the verified repository baseline, including the last full local baseline and current branch revalidation dates, use [DEVELOPMENT_STATUS.md](./DEVELOPMENT_STATUS.md). Statuses here evaluate parity depth, not just whether some feature exists in code.

## Executive Summary

| Category | Merit Features | SmartAccounts Features | Open Accounting Status |
|----------|---------------|----------------------|----------------------|
| Core Accounting | 12 | 10 | Broad coverage, mixed depth |
| Invoicing | 8 | 9 | Broad coverage, some parity gaps |
| Banking | 6 | 5 | Manual-import workflow present, direct integrations missing |
| Payroll | 7 | 6 | Strong local coverage, some compliance depth still missing |
| Reporting | 8 | 7 | Core reports present, accountant-grade depth incomplete |
| Tax & Compliance | 5 | 6 | Export-centric today, direct submissions missing |
| Integrations | 4 | 5 | Plugin foundation exists, partner integrations missing |

**Overall breadth:** roughly 60-70% of the combined feature surface exists in some form, but production depth and accountant workflow completeness are materially lower than that headline number.

---

## 1. Core Accounting Features

### 1.1 Chart of Accounts

| Feature | Merit | SmartAccounts | Open Accounting | Status |
|---------|-------|---------------|-----------------|--------|
| Multi-level account hierarchy | ✅ | ✅ | ✅ | Implemented |
| Account types (Asset/Liability/Equity/Revenue/Expense) | ✅ | ✅ | ✅ | Implemented |
| Custom account codes | ✅ | ✅ | ✅ | Implemented |
| Account grouping | ✅ | ✅ | ✅ | Implemented with parent-child chart hierarchy, CSV parent-code import, and API/CLI grouped view |
| System accounts (locked) | ✅ | ✅ | ✅ | Implemented |
| Account deactivation | ✅ | ✅ | ✅ | Implemented |

### 1.2 Journal Entries

| Feature | Merit | SmartAccounts | Open Accounting | Status |
|---------|-------|---------------|-----------------|--------|
| Manual journal entries | ✅ | ✅ | ✅ | Implemented |
| Auto-generated entries | ✅ | ✅ | ✅ | Implemented |
| Entry numbering | ✅ | ✅ | ✅ | Implemented |
| Entry reversal/void | ✅ | ✅ | ✅ | Implemented |
| Recurring entries | ✅ | ✅ | ✅ | Implemented through recurring journal templates with API/CLI single and due-batch generation plus background scheduled due generation across active tenants |
| Entry templates | ✅ | ✅ | ✅ | Implemented through reusable balanced journal entry templates with API/CLI apply workflow |
| Multi-currency entries | ✅ | ✅ | ✅ | Implemented for manual and template journal entries with line currency, positive exchange rate validation, base-currency balancing, API/CLI support, and historical journal import |

### 1.3 Multi-Tenancy

| Feature | Merit | SmartAccounts | Open Accounting | Status |
|---------|-------|---------------|-----------------|--------|
| Multiple companies | ✅ | ✅ | ✅ | Implemented |
| Company switching | ✅ | ✅ | ✅ | Implemented |
| Shared chart of accounts | ❌ | ❌ | ❌ | N/A |
| Consolidated reporting | ✅ | ✅ | ✅ | Implemented as multi-tenant consolidated trial balance, balance sheet, and income statement report with API/CLI coverage and tenant-access checks |

---

## 2. Invoicing Features

### 2.1 Sales Invoices

| Feature | Merit | SmartAccounts | Open Accounting | Status |
|---------|-------|---------------|-----------------|--------|
| Invoice creation | ✅ | ✅ | ✅ | Implemented |
| Invoice numbering (auto) | ✅ | ✅ | ✅ | Implemented |
| Multiple VAT rates | ✅ | ✅ | ✅ | Implemented |
| PDF generation | ✅ | ✅ | ✅ | Implemented |
| Email sending | ✅ | ✅ | ✅ | Implemented |
| Invoice templates | ✅ | ✅ | ✅ | Implemented through recurring invoice templates with API/CLI create, import, create-from-invoice, update, pause/resume, manual generation, and due-batch generation |
| Credit notes | ✅ | ✅ | ✅ | Implemented |
| Invoice reminders | ✅ | ✅ | ✅ | Implemented |
| Recurring invoices | ✅ | ✅ | ✅ | Implemented |
| E-invoice (Estonian e-arve) | ✅ | ✅ | ⚠️ | Manual Estonian e-invoice XML import is implemented through API/CLI; direct operator-network sending/receiving remains blocked |
| Offers/Quotes | ✅ | ✅ | ✅ | Implemented with quote lifecycle, import, PDF/email delivery, and quote-to-invoice conversion through API/CLI plus UI delivery flows |

### 2.2 Purchase Invoices

| Feature | Merit | SmartAccounts | Open Accounting | Status |
|---------|-------|---------------|-----------------|--------|
| Purchase invoice entry | ✅ | ✅ | ✅ | Implemented through `PURCHASE` invoices/supplier bills with API/CLI creation, filtering, PDF/download, voiding, attachments, payment allocation, and CSV import |
| Expense categorization | ✅ | ✅ | ✅ | Implemented |
| Supplier management | ✅ | ✅ | ✅ | Implemented |
| OCR scanning | ✅ | ✅ | ❌ | **Blocker** |
| E-invoice import | ✅ | ✅ | ✅ | Manual Estonian e-invoice XML import is implemented through API/CLI; operator-network receiving remains blocked |

---

## 3. Banking Features

### 3.1 Bank Reconciliation

| Feature | Merit | SmartAccounts | Open Accounting | Status |
|---------|-------|---------------|-----------------|--------|
| Manual transaction import | ✅ | ✅ | ✅ | Implemented |
| CSV import | ✅ | ✅ | ✅ | Implemented |
| Transaction matching | ✅ | ✅ | ✅ | Implemented |
| Auto-matching rules | ✅ | ✅ | ✅ | Implemented - persisted bank-account or tenant-wide rules tune match field, priority, confidence, date window, exact amount, active state, API, and CLI coverage |
| Bank feed (Swedbank Gateway) | ❌ | ✅ | ❌ | **Blocker** |
| Multi-bank support | ✅ | ✅ | ✅ | Implemented |

### 3.2 Payment Management

| Feature | Merit | SmartAccounts | Open Accounting | Status |
|---------|-------|---------------|-----------------|--------|
| Payment recording | ✅ | ✅ | ✅ | Implemented |
| Partial payments | ✅ | ✅ | ✅ | Implemented |
| Payment reminders | ✅ | ✅ | ✅ | Implemented |
| Direct bank payments | ✅ | ✅ | ❌ | **Blocker** |
| SEPA payments | ✅ | ✅ | ✅ | Implemented as pain.001 XML export for manual bank upload; direct bank submission remains a separate blocker |

---

## 4. Payroll Features

### 4.1 Employee Management

| Feature | Merit | SmartAccounts | Open Accounting | Status |
|---------|-------|---------------|-----------------|--------|
| Employee records | ✅ | ✅ | ✅ | Implemented |
| Contract management | ✅ | ✅ | ✅ | Implemented |
| Tax exemptions | ✅ | ✅ | ✅ | Implemented |
| Pension fund enrollment | ✅ | ✅ | ✅ | Implemented |
| Multiple employments | ✅ | ✅ | ✅ | Implemented through date-bounded salary components for secondary employments, with API/CLI management and payroll gross salary inclusion |

### 4.2 Salary Calculation

| Feature | Merit | SmartAccounts | Open Accounting | Status |
|---------|-------|---------------|-----------------|--------|
| Gross/net calculation | ✅ | ✅ | ✅ | Implemented |
| Estonian social tax | ✅ | ✅ | ✅ | Implemented |
| Income tax calculation | ✅ | ✅ | ✅ | Implemented |
| Unemployment insurance | ✅ | ✅ | ✅ | Implemented |
| Pension contributions | ✅ | ✅ | ✅ | Implemented |
| Payslip generation | ✅ | ✅ | ✅ | Implemented with calculated payslip records plus generated PDF download through API/CLI |
| Historical payroll, TSD, and leave-balance import | ✅ | ✅ | ⚠️ | Payroll and leave API/UI/CLI import exists; TSD history API/CLI import exists; broader cutover still partial |
| Bulk payroll processing | ✅ | ✅ | ✅ | Implemented via payroll run process API/CLI |

### 4.3 Tax Declarations

| Feature | Merit | SmartAccounts | Open Accounting | Status |
|---------|-------|---------------|-----------------|--------|
| TSD form generation | ✅ | ✅ | ✅ | Implemented |
| e-MTA submission | ✅ | ✅ | ❌ | **Blocker** |
| INF form | ✅ | ✅ | ✅ | Implemented as KMD INF A/B report; e-MTA submission remains blocked |

---

## 5. Reporting Features

### 5.1 Financial Reports

| Feature | Merit | SmartAccounts | Open Accounting | Status |
|---------|-------|---------------|-----------------|--------|
| Trial Balance | ✅ | ✅ | ✅ | Implemented |
| Balance Sheet | ✅ | ✅ | ✅ | Implemented |
| Income Statement | ✅ | ✅ | ✅ | Implemented |
| Cash Flow Statement | ✅ | ✅ | ✅ | Implemented through direct and indirect cash-flow statements with API/CLI JSON/CSV/XLSX/PDF export, annual-report inclusion, and tenant/request-level account mapping |
| Aging reports | ✅ | ✅ | ✅ | Implemented for receivables and payables with API/CLI JSON/CSV/XLSX/PDF export |
| Sales margin reports | ✅ | ✅ | ✅ | Implemented through API/CLI CSV/XLSX/PDF exports |
| Custom date ranges | ✅ | ✅ | ✅ | Implemented |

### 5.2 Management Reports

| Feature | Merit | SmartAccounts | Open Accounting | Status |
|---------|-------|---------------|-----------------|--------|
| Dashboard analytics | ✅ | ✅ | ✅ | Implemented |
| Revenue by period | ✅ | ✅ | ✅ | Implemented |
| Expense breakdown | ✅ | ✅ | ✅ | Implemented |
| Customer profitability | ✅ | ✅ | ✅ | Implemented through first-class customer profitability API/CLI JSON/CSV/XLSX/PDF reporting with product-cost-backed customer revenue, estimated cost, profit, profit percent, and supporting invoice-line detail |
| Budget vs actual | ✅ | ✅ | ✅ | Implemented through cost-center budget-vs-actual API/CLI CSV/XLSX/PDF reports |

---

## 6. Tax & Compliance

### 6.1 VAT

| Feature | Merit | SmartAccounts | Open Accounting | Status |
|---------|-------|---------------|-----------------|--------|
| VAT calculation | ✅ | ✅ | ✅ | Implemented |
| Multi-rate VAT | ✅ | ✅ | ✅ | Implemented |
| VAT declaration (KMD) | ✅ | ✅ | ✅ | Implemented with generation, listing, e-MTA XML export, KMD INF A/B reporting, historical import, API/CLI coverage, and tests; direct e-MTA submission remains a separate blocker |
| e-MTA VAT submission | ✅ | ✅ | ❌ | **Blocker** |
| EU VAT (MOSS) | ✅ | ✅ | ✅ | Implemented as quarterly EU VAT OSS report grouped by destination member state and VAT rate, with API/CLI coverage for manual filing support |
| Reverse charge VAT | ✅ | ✅ | ✅ | Implemented - invoice lines support reverse-charge treatment through API, CLI, CSV import, persistence, and KMD self-assessed output/input VAT aggregation |

### 6.2 Annual Reporting

| Feature | Merit | SmartAccounts | Open Accounting | Status |
|---------|-------|---------------|-----------------|--------|
| Annual report generation | ✅ | ✅ | ✅ | Implemented as annual report pack; e-äriregister submission remains blocked |
| e-äriregister submission | ✅ | ✅ | ❌ | **Blocker** |

---

## 7. Integration Features

### 7.1 API & Integrations

| Feature | Merit | SmartAccounts | Open Accounting | Status |
|---------|-------|---------------|-----------------|--------|
| REST API | ✅ | ✅ | ✅ | Implemented |
| Webhook notifications | ⚠️ | ❌ | ✅ | Implemented as tenant outbound webhook endpoints with event subscriptions, signed POST delivery, test delivery, delivery history, and CLI/API coverage |
| WooCommerce | ❌ | ✅ | ❌ | **Gap** |
| Shopify | ❌ | ❌ | ❌ | **Gap** |
| Scoro | ❌ | ✅ | ❌ | **Gap** |
| Plugin system | ❌ | ❌ | ✅ | Implemented |

### 7.2 Data Import/Export

| Feature | Merit | SmartAccounts | Open Accounting | Status |
|---------|-------|---------------|-----------------|--------|
| CSV export | ✅ | ✅ | ✅ | Implemented |
| Excel export | ✅ | ✅ | ✅ | Implemented for core financial statements, cash flow, aging, balance confirmations, contact statements, sales margin, budget-vs-actual, and cost-center budget reports |
| Data migration tools | ✅ | ✅ | ⚠️ | Partial: CSV and XML imports cover setup data, invoices, Estonian e-invoices, payments, expenses, opening balances, employees, finalized payroll history, leave balances, historical TSD/KMD declarations, quotes, orders, recurring invoice templates, bank accounts, bank transactions including standard ISO 20022 camt.053 statements, cost centers, cost allocations, product categories, warehouses, products, stock adjustments with lot metadata, fixed assets, and historical journal entries; a migration bundle validator checks required columns, XML payloads, e-invoice supplier/customer party contact references by validation mode, opening-balance debit/credit totals, grouped historical-journal date/line/amount/base-currency balances, provider preset header aliases for generic, Merit, SmartAccounts, and Directo CSVs, and same-bundle references before import, but deeper incumbent-system templates and full cutover remain incomplete |

---

## 8. Mobile Features

### 8.1 Mobile Access

| Feature | Merit | SmartAccounts | Open Accounting | Status |
|---------|-------|---------------|-----------------|--------|
| Responsive web design | ⚠️ | ✅ | ✅ | Implemented |
| Native mobile app | ❌ | ✅ (Android) | ❌ | **Gap** |
| Receipt capture | ❌ | ✅ | ✅ | Implemented through receipt documents linked to expenses; native mobile capture and OCR remain separate gaps |
| Expense tracking | ❌ | ✅ | ✅ | Implemented through draft/submitted/approved/rejected/posted expense claims with receipt enforcement, CSV import, and ledger posting |

---

## Blockers Summary

These features cannot be implemented without external dependencies or significant infrastructure:

### 1. **E-Invoice (e-arve) Integration**
- **Requirement**: Direct operator-network send/receive through Omniva or another e-invoice operator
- **Blocker**: Requires partnership agreement with Omniva and certificate authentication
- **Workaround**: Manual Estonian e-invoice XML import and manual PDF invoice sending are implemented

### 2. **Bank Feed Integration (Swedbank Gateway, SEB, LHV)**
- **Requirement**: Direct bank API access
- **Blocker**: Requires banking partnership agreements and PSD2 compliance
- **Workaround**: CSV import from bank statements (implemented)

### 3. **e-MTA Tax Submission**
- **Requirement**: Direct integration with Estonian Tax Authority
- **Blocker**: Requires X-Road certification and digital signing
- **Workaround**: Export XML files for manual upload (partial)

### 4. **e-äriregister Submission**
- **Requirement**: Integration with Estonian Business Registry
- **Blocker**: Requires X-Road and digital signatures
- **Workaround**: Generate PDF reports for manual submission

### 5. **OCR Invoice Scanning**
- **Requirement**: Machine learning/OCR service
- **Blocker**: Requires third-party OCR API (Google Vision, AWS Textract, or custom ML)
- **Workaround**: Manual invoice entry

### 6. **Direct Bank Payments**
- **Requirement**: SEPA payment initiation
- **Blocker**: Requires PSD2 PISP license or banking partnership
- **Workaround**: Manual payment through bank

---

## Priority Themes

This document's older quarter-based priorities have been superseded by the 2026 roadmap. The current priority order is:

1. Reliability and truthful status reporting
2. Imports, close controls, and attachments
3. Server-side reporting depth and accountant workflow improvements
4. Security and operational hardening
5. Partner-dependent integrations such as e-invoice, bank feeds, and automatic tax submission

---

## Verification Note

Testing and coverage status changed materially after this comparison was first drafted. For the current verified baseline, see [DEVELOPMENT_STATUS.md](./DEVELOPMENT_STATUS.md) and the CI workflow rather than relying on historical coverage percentages in this file.

---

## API Compatibility Notes

### Merit API Compatibility
Merit uses a REST API with:
- API ID and API Key authentication
- Unix timestamp validation
- Specific endpoints for invoices, contacts, etc.

**Recommendation**: Create Merit-compatible API adapter plugin

### SmartAccounts API Compatibility
SmartAccounts API features:
- REST endpoints
- Integration with Envoice, WooCommerce, ShopRoller

**Recommendation**: Create SmartAccounts import/export adapter

---

## Sources
- [Merit API Documentation](https://api.merit.ee/)
- [SmartAccounts Features](https://www.smartaccounts.eu/en/features/)
- [Merit Aktiva](https://www.merit.ee/en/)
- [SmartAccounts](https://www.smartaccounts.eu/en/)
