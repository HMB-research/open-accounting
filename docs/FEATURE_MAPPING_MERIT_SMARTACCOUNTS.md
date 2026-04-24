# Feature Mapping: Merit & SmartAccounts vs Open Accounting

This document maps features from [Merit Aktiva](https://www.merit.ee/en/) and [SmartAccounts](https://www.smartaccounts.eu/en/) to Open Accounting, identifying implementation status, gaps, and blockers.

This is a competitive-gap document, not the authoritative current-state status page. For the verified repository baseline as of 2026-04-24, use [DEVELOPMENT_STATUS.md](./DEVELOPMENT_STATUS.md). Statuses here evaluate parity depth, not just whether some feature exists in code.

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
| Account grouping | ✅ | ✅ | ⚠️ | Partial |
| System accounts (locked) | ✅ | ✅ | ✅ | Implemented |
| Account deactivation | ✅ | ✅ | ✅ | Implemented |

### 1.2 Journal Entries

| Feature | Merit | SmartAccounts | Open Accounting | Status |
|---------|-------|---------------|-----------------|--------|
| Manual journal entries | ✅ | ✅ | ✅ | Implemented |
| Auto-generated entries | ✅ | ✅ | ✅ | Implemented |
| Entry numbering | ✅ | ✅ | ✅ | Implemented |
| Entry reversal/void | ✅ | ✅ | ✅ | Implemented |
| Recurring entries | ✅ | ✅ | ❌ | **Gap** |
| Entry templates | ✅ | ✅ | ❌ | **Gap** |
| Multi-currency entries | ✅ | ✅ | ⚠️ | Partial |

### 1.3 Multi-Tenancy

| Feature | Merit | SmartAccounts | Open Accounting | Status |
|---------|-------|---------------|-----------------|--------|
| Multiple companies | ✅ | ✅ | ✅ | Implemented |
| Company switching | ✅ | ✅ | ✅ | Implemented |
| Shared chart of accounts | ❌ | ❌ | ❌ | N/A |
| Consolidated reporting | ✅ | ✅ | ❌ | **Gap** |

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
| Invoice templates | ✅ | ✅ | ⚠️ | Partial |
| Credit notes | ✅ | ✅ | ✅ | Implemented |
| Invoice reminders | ✅ | ✅ | ✅ | Implemented |
| Recurring invoices | ✅ | ✅ | ✅ | Implemented |
| E-invoice (Estonian e-arve) | ✅ | ✅ | ❌ | **Blocker** |
| Offers/Quotes | ✅ | ✅ | ⚠️ | Partial |

### 2.2 Purchase Invoices

| Feature | Merit | SmartAccounts | Open Accounting | Status |
|---------|-------|---------------|-----------------|--------|
| Purchase invoice entry | ✅ | ✅ | ⚠️ | Partial |
| Expense categorization | ✅ | ✅ | ✅ | Implemented |
| Supplier management | ✅ | ✅ | ✅ | Implemented |
| OCR scanning | ✅ | ✅ | ❌ | **Blocker** |
| E-invoice import | ✅ | ✅ | ❌ | **Blocker** |

---

## 3. Banking Features

### 3.1 Bank Reconciliation

| Feature | Merit | SmartAccounts | Open Accounting | Status |
|---------|-------|---------------|-----------------|--------|
| Manual transaction import | ✅ | ✅ | ✅ | Implemented |
| CSV import | ✅ | ✅ | ✅ | Implemented |
| Transaction matching | ✅ | ✅ | ✅ | Implemented |
| Auto-matching rules | ✅ | ✅ | ⚠️ | Partial |
| Bank feed (Swedbank Gateway) | ❌ | ✅ | ❌ | **Blocker** |
| Multi-bank support | ✅ | ✅ | ✅ | Implemented |

### 3.2 Payment Management

| Feature | Merit | SmartAccounts | Open Accounting | Status |
|---------|-------|---------------|-----------------|--------|
| Payment recording | ✅ | ✅ | ✅ | Implemented |
| Partial payments | ✅ | ✅ | ✅ | Implemented |
| Payment reminders | ✅ | ✅ | ✅ | Implemented |
| Direct bank payments | ✅ | ✅ | ❌ | **Blocker** |
| SEPA payments | ✅ | ✅ | ❌ | **Gap** |

---

## 4. Payroll Features

### 4.1 Employee Management

| Feature | Merit | SmartAccounts | Open Accounting | Status |
|---------|-------|---------------|-----------------|--------|
| Employee records | ✅ | ✅ | ✅ | Implemented |
| Contract management | ✅ | ✅ | ✅ | Implemented |
| Tax exemptions | ✅ | ✅ | ✅ | Implemented |
| Pension fund enrollment | ✅ | ✅ | ✅ | Implemented |
| Multiple employments | ✅ | ✅ | ⚠️ | Partial |

### 4.2 Salary Calculation

| Feature | Merit | SmartAccounts | Open Accounting | Status |
|---------|-------|---------------|-----------------|--------|
| Gross/net calculation | ✅ | ✅ | ✅ | Implemented |
| Estonian social tax | ✅ | ✅ | ✅ | Implemented |
| Income tax calculation | ✅ | ✅ | ✅ | Implemented |
| Unemployment insurance | ✅ | ✅ | ✅ | Implemented |
| Pension contributions | ✅ | ✅ | ✅ | Implemented |
| Payslip generation | ✅ | ✅ | ⚠️ | Partial |
| Historical payroll and leave-balance import | ✅ | ✅ | ⚠️ | API/UI/CLI import exists; broader cutover still partial |
| Bulk payroll processing | ✅ | ✅ | ❌ | **Gap** |

### 4.3 Tax Declarations

| Feature | Merit | SmartAccounts | Open Accounting | Status |
|---------|-------|---------------|-----------------|--------|
| TSD form generation | ✅ | ✅ | ✅ | Implemented |
| e-MTA submission | ✅ | ✅ | ❌ | **Blocker** |
| INF form | ✅ | ✅ | ❌ | **Gap** |

---

## 5. Reporting Features

### 5.1 Financial Reports

| Feature | Merit | SmartAccounts | Open Accounting | Status |
|---------|-------|---------------|-----------------|--------|
| Trial Balance | ✅ | ✅ | ✅ | Implemented |
| Balance Sheet | ✅ | ✅ | ✅ | Implemented |
| Income Statement | ✅ | ✅ | ✅ | Implemented |
| Cash Flow Statement | ✅ | ✅ | ⚠️ | Partial |
| Aging reports | ✅ | ✅ | ⚠️ | Partial |
| Custom date ranges | ✅ | ✅ | ✅ | Implemented |

### 5.2 Management Reports

| Feature | Merit | SmartAccounts | Open Accounting | Status |
|---------|-------|---------------|-----------------|--------|
| Dashboard analytics | ✅ | ✅ | ✅ | Implemented |
| Revenue by period | ✅ | ✅ | ✅ | Implemented |
| Expense breakdown | ✅ | ✅ | ✅ | Implemented |
| Customer profitability | ✅ | ✅ | ❌ | **Gap** |
| Budget vs actual | ✅ | ✅ | ❌ | **Gap** |

---

## 6. Tax & Compliance

### 6.1 VAT

| Feature | Merit | SmartAccounts | Open Accounting | Status |
|---------|-------|---------------|-----------------|--------|
| VAT calculation | ✅ | ✅ | ✅ | Implemented |
| Multi-rate VAT | ✅ | ✅ | ✅ | Implemented |
| VAT declaration (KMD) | ✅ | ✅ | ⚠️ | Partial |
| e-MTA VAT submission | ✅ | ✅ | ❌ | **Blocker** |
| EU VAT (MOSS) | ✅ | ✅ | ❌ | **Gap** |
| Reverse charge VAT | ✅ | ✅ | ⚠️ | Partial |

### 6.2 Annual Reporting

| Feature | Merit | SmartAccounts | Open Accounting | Status |
|---------|-------|---------------|-----------------|--------|
| Annual report generation | ✅ | ✅ | ❌ | **Gap** |
| e-äriregister submission | ✅ | ✅ | ❌ | **Blocker** |

---

## 7. Integration Features

### 7.1 API & Integrations

| Feature | Merit | SmartAccounts | Open Accounting | Status |
|---------|-------|---------------|-----------------|--------|
| REST API | ✅ | ✅ | ✅ | Implemented |
| Webhook notifications | ⚠️ | ❌ | ⚠️ | Partial |
| WooCommerce | ❌ | ✅ | ❌ | **Gap** |
| Shopify | ❌ | ❌ | ❌ | **Gap** |
| Scoro | ❌ | ✅ | ❌ | **Gap** |
| Plugin system | ❌ | ❌ | ✅ | Implemented |

### 7.2 Data Import/Export

| Feature | Merit | SmartAccounts | Open Accounting | Status |
|---------|-------|---------------|-----------------|--------|
| CSV export | ✅ | ✅ | ✅ | Implemented |
| Excel export | ✅ | ✅ | ⚠️ | Partial |
| Data migration tools | ✅ | ✅ | ⚠️ | Partial: CSV imports cover setup data, invoices, opening balances, employees, finalized payroll history, and leave balances; full incumbent-system migration is still incomplete |

---

## 8. Mobile Features

### 8.1 Mobile Access

| Feature | Merit | SmartAccounts | Open Accounting | Status |
|---------|-------|---------------|-----------------|--------|
| Responsive web design | ⚠️ | ✅ | ✅ | Implemented |
| Native mobile app | ❌ | ✅ (Android) | ❌ | **Gap** |
| Receipt capture | ❌ | ✅ | ❌ | **Gap** |
| Expense tracking | ❌ | ✅ | ❌ | **Gap** |

---

## Blockers Summary

These features cannot be implemented without external dependencies or significant infrastructure:

### 1. **E-Invoice (e-arve) Integration**
- **Requirement**: Connect to Omniva e-invoice center
- **Blocker**: Requires partnership agreement with Omniva and certificate authentication
- **Workaround**: Manual PDF invoice sending (implemented)

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
