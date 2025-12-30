# Feature Mapping: Merit & SmartAccounts vs Open Accounting

This document maps features from [Merit Aktiva](https://www.merit.ee/en/) and [SmartAccounts](https://www.smartaccounts.eu/en/) to Open Accounting, identifying implementation status, gaps, and blockers.

## Executive Summary

| Category | Merit Features | SmartAccounts Features | Open Accounting Status |
|----------|---------------|----------------------|----------------------|
| Core Accounting | 12 | 10 | 8 implemented |
| Invoicing | 8 | 9 | 6 implemented |
| Banking | 6 | 5 | 4 implemented |
| Payroll | 7 | 6 | 5 implemented |
| Reporting | 8 | 7 | 5 implemented |
| Tax & Compliance | 5 | 6 | 3 implemented |
| Integrations | 4 | 5 | 2 implemented |

**Overall Coverage: ~65% of combined feature set**

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
| Offers/Quotes | ✅ | ✅ | ❌ | **Gap** |

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
| Cash Flow Statement | ✅ | ✅ | ❌ | **Gap** |
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
| Excel export | ✅ | ✅ | ❌ | **Gap** |
| Data migration tools | ✅ | ✅ | ❌ | **Gap** |

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

## Implementation Priority Matrix

### Phase 1: High Priority (Q1 2025)
1. Cash Flow Statement report
2. Bulk payroll processing
3. Excel export functionality
4. Quote/Offer module

### Phase 2: Medium Priority (Q2 2025)
1. Consolidated reporting (multi-company)
2. Budget vs Actual reporting
3. SEPA payment file generation
4. Recurring journal entries

### Phase 3: Lower Priority (Q3 2025)
1. E-commerce integrations (WooCommerce, Shopify)
2. Customer profitability analysis
3. Advanced aging reports
4. INF form generation

### Deferred (Requires External Partnerships)
1. E-invoice integration
2. Bank feeds
3. e-MTA direct submission
4. e-äriregister submission
5. OCR scanning

---

## Testing Requirements for 95% Coverage

### Backend (Go) - Current: ~15% average
Target areas needing tests:
- `internal/contacts` - 0% → 95%
- `internal/database` - 0% → 90%
- `internal/analytics` - 0.9% → 95%
- `internal/payments` - 4.1% → 95%
- `internal/tenant` - 4.2% → 95%
- `internal/email` - 7.2% → 95%
- `internal/pdf` - 8.1% → 95%
- `internal/payroll` - 9.4% → 95%
- `internal/accounting` - 10.7% → 95%

### Frontend (Svelte) - Current: ~34%
Target areas needing tests:
- Component unit tests for all routes
- API client coverage
- State management coverage
- Form validation coverage
- Error handling coverage

### E2E Tests - Current: 7 test files
Additional E2E tests needed:
- Accounts management flow
- Journal entry workflow
- Payroll calculation flow
- Tax reporting flow
- Settings configuration flow
- Plugin management flow
- Banking import flow

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
