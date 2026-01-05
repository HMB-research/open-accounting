# Ralph Wiggum Loop: Missing Modules Development

## Overview

This document defines an iterative development loop for implementing missing modules identified in the SmartAccounts feature comparison.

```
┌─────────────────────────────────────────────────────────────────┐
│                    RALPH WIGGUM LOOP                            │
│                                                                 │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐  │
│  │  PICK    │───►│  PLAN    │───►│  BUILD   │───►│  TEST    │  │
│  │  MODULE  │    │  FEATURE │    │  FEATURE │    │  & DEMO  │  │
│  └──────────┘    └──────────┘    └──────────┘    └──────────┘  │
│       ▲                                               │         │
│       │                                               │         │
│       └───────────────────────────────────────────────┘         │
│                         NEXT MODULE                             │
└─────────────────────────────────────────────────────────────────┘
```

---

## Missing Modules Queue

| Priority | Module | Complexity | Dependencies |
|----------|--------|------------|--------------|
| 1 | Cash Payments | Low | Payments, Accounts |
| 2 | Quotes/Offers | Medium | Contacts, Invoices |
| 3 | Orders | Medium | Quotes, Invoices |
| 4 | Fixed Assets | High | Accounts, Journal |
| 5 | Inventory/Warehouse | High | Invoices, Accounts |

---

## Loop Prompt

Copy and paste this prompt to start each iteration:

```
## Ralph Loop: Module Development

I'm implementing missing modules for open-accounting. Current queue:

1. ⬜ Cash Payments - Cash register transactions
2. ⬜ Quotes/Offers - Client quote management
3. ⬜ Orders - Order tracking and fulfillment
4. ⬜ Fixed Assets - Asset register with depreciation
5. ⬜ Inventory/Warehouse - Stock management

### Current Iteration

**Module:** [PICK NEXT UNCOMPLETED]

### Phase 1: Database Schema
- [ ] Design tables (migrations)
- [ ] Add to demo seed data
- [ ] Verify multi-tenant isolation

### Phase 2: Backend API
- [ ] Create handlers in handlers.go
- [ ] Add routes
- [ ] Implement CRUD operations
- [ ] Add to API types

### Phase 3: Frontend
- [ ] Create route folder
- [ ] Build +page.svelte
- [ ] Add to navigation
- [ ] Implement i18n strings (en.json, et.json)

### Phase 4: Testing & Demo
- [ ] Run E2E tests
- [ ] Verify in demo mode
- [ ] Screenshot for documentation

### Completion Criteria
- [ ] Feature works in demo mode
- [ ] No console errors
- [ ] Data persists correctly
- [ ] Navigation accessible

When complete, update the queue above and proceed to next module.
```

---

## Module Specifications

### 1. Cash Payments (Kassamaksed)

**Purpose:** Track cash register transactions separate from bank payments

**Database:**
```sql
CREATE TABLE cash_payments (
    id SERIAL PRIMARY KEY,
    tenant_id INTEGER NOT NULL REFERENCES tenants(id),
    payment_number VARCHAR(50),
    payment_date DATE NOT NULL,
    description TEXT,
    amount DECIMAL(15,2) NOT NULL,
    payment_type VARCHAR(20), -- 'income' or 'expense'
    contact_id INTEGER REFERENCES contacts(id),
    account_id INTEGER REFERENCES accounts(id),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

**Routes:** `/payments/cash`

**SmartAccounts Reference:** Kassamaksed menu under Maksed

---

### 2. Quotes/Offers (Pakkumised)

**Purpose:** Create quotes for clients that can convert to orders/invoices

**Database:**
```sql
CREATE TABLE quotes (
    id SERIAL PRIMARY KEY,
    tenant_id INTEGER NOT NULL REFERENCES tenants(id),
    quote_number VARCHAR(50),
    contact_id INTEGER NOT NULL REFERENCES contacts(id),
    quote_date DATE NOT NULL,
    valid_until DATE,
    status VARCHAR(20) DEFAULT 'draft', -- draft, sent, accepted, rejected, converted
    subtotal DECIMAL(15,2),
    vat_amount DECIMAL(15,2),
    total DECIMAL(15,2),
    notes TEXT,
    converted_to_invoice_id INTEGER REFERENCES invoices(id),
    converted_to_order_id INTEGER REFERENCES orders(id),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE quote_lines (
    id SERIAL PRIMARY KEY,
    quote_id INTEGER NOT NULL REFERENCES quotes(id) ON DELETE CASCADE,
    description TEXT NOT NULL,
    quantity DECIMAL(15,4) DEFAULT 1,
    unit_price DECIMAL(15,2),
    vat_rate DECIMAL(5,2),
    line_total DECIMAL(15,2)
);
```

**Routes:** `/quotes`

**SmartAccounts Reference:** Pakkumised menu under Ost/müük

---

### 3. Orders (Tellimused)

**Purpose:** Track customer orders before invoicing

**Database:**
```sql
CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    tenant_id INTEGER NOT NULL REFERENCES tenants(id),
    order_number VARCHAR(50),
    contact_id INTEGER NOT NULL REFERENCES contacts(id),
    order_date DATE NOT NULL,
    expected_delivery DATE,
    status VARCHAR(20) DEFAULT 'pending', -- pending, confirmed, shipped, delivered, cancelled
    subtotal DECIMAL(15,2),
    vat_amount DECIMAL(15,2),
    total DECIMAL(15,2),
    notes TEXT,
    quote_id INTEGER REFERENCES quotes(id),
    converted_to_invoice_id INTEGER REFERENCES invoices(id),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE order_lines (
    id SERIAL PRIMARY KEY,
    order_id INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    description TEXT NOT NULL,
    quantity DECIMAL(15,4) DEFAULT 1,
    unit_price DECIMAL(15,2),
    vat_rate DECIMAL(5,2),
    line_total DECIMAL(15,2)
);
```

**Routes:** `/orders`

**SmartAccounts Reference:** Tellimused menu under Ost/müük

---

### 4. Fixed Assets (Põhivarad)

**Purpose:** Track fixed assets and calculate depreciation

**Database:**
```sql
CREATE TABLE fixed_assets (
    id SERIAL PRIMARY KEY,
    tenant_id INTEGER NOT NULL REFERENCES tenants(id),
    asset_number VARCHAR(50),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    acquisition_date DATE NOT NULL,
    acquisition_cost DECIMAL(15,2) NOT NULL,
    residual_value DECIMAL(15,2) DEFAULT 0,
    useful_life_months INTEGER NOT NULL,
    depreciation_method VARCHAR(20) DEFAULT 'straight_line', -- straight_line, declining_balance
    asset_account_id INTEGER REFERENCES accounts(id),
    depreciation_account_id INTEGER REFERENCES accounts(id),
    expense_account_id INTEGER REFERENCES accounts(id),
    status VARCHAR(20) DEFAULT 'active', -- active, disposed, fully_depreciated
    disposal_date DATE,
    disposal_amount DECIMAL(15,2),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE depreciation_entries (
    id SERIAL PRIMARY KEY,
    fixed_asset_id INTEGER NOT NULL REFERENCES fixed_assets(id) ON DELETE CASCADE,
    depreciation_date DATE NOT NULL,
    amount DECIMAL(15,2) NOT NULL,
    accumulated_depreciation DECIMAL(15,2) NOT NULL,
    book_value DECIMAL(15,2) NOT NULL,
    journal_entry_id INTEGER REFERENCES journal_entries(id),
    created_at TIMESTAMP DEFAULT NOW()
);
```

**Routes:** `/fixed-assets`

**SmartAccounts Reference:** Põhivarad menu with Amortiseerimised

---

### 5. Inventory/Warehouse (Ladu)

**Purpose:** Track stock levels and warehouse movements

**Database:**
```sql
CREATE TABLE warehouses (
    id SERIAL PRIMARY KEY,
    tenant_id INTEGER NOT NULL REFERENCES tenants(id),
    name VARCHAR(255) NOT NULL,
    code VARCHAR(50),
    address TEXT,
    is_default BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE inventory_items (
    id SERIAL PRIMARY KEY,
    tenant_id INTEGER NOT NULL REFERENCES tenants(id),
    sku VARCHAR(100),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    unit VARCHAR(50) DEFAULT 'pcs',
    cost_price DECIMAL(15,2),
    sale_price DECIMAL(15,2),
    reorder_level DECIMAL(15,4),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE inventory_stock (
    id SERIAL PRIMARY KEY,
    inventory_item_id INTEGER NOT NULL REFERENCES inventory_items(id),
    warehouse_id INTEGER NOT NULL REFERENCES warehouses(id),
    quantity DECIMAL(15,4) DEFAULT 0,
    reserved_quantity DECIMAL(15,4) DEFAULT 0,
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(inventory_item_id, warehouse_id)
);

CREATE TABLE inventory_movements (
    id SERIAL PRIMARY KEY,
    tenant_id INTEGER NOT NULL REFERENCES tenants(id),
    inventory_item_id INTEGER NOT NULL REFERENCES inventory_items(id),
    warehouse_id INTEGER NOT NULL REFERENCES warehouses(id),
    movement_type VARCHAR(20) NOT NULL, -- 'in', 'out', 'transfer', 'adjustment'
    quantity DECIMAL(15,4) NOT NULL,
    unit_cost DECIMAL(15,2),
    reference_type VARCHAR(50), -- 'invoice', 'order', 'manual'
    reference_id INTEGER,
    notes TEXT,
    movement_date TIMESTAMP DEFAULT NOW(),
    created_at TIMESTAMP DEFAULT NOW()
);
```

**Routes:** `/inventory`, `/inventory/warehouses`, `/inventory/movements`

**SmartAccounts Reference:** Ladu menu with Laoliikumised, Laoseisud, Laod

---

## Progress Tracking

Update this section after each iteration:

| Module | Started | Schema | Backend | Frontend | Tests | Complete |
|--------|---------|--------|---------|----------|-------|----------|
| Cash Payments | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Quotes/Offers | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Orders | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Fixed Assets | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Inventory | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

**All 5 modules completed on 2026-01-05**

---

## Ralph Wiggum Loop Command

Run this command to start the autonomous development loop:

```
/ralph-loop "Implement missing accounting modules for open-accounting. Read docs/plans/ralph-loop-missing-modules.md for specifications. Work through modules in order: 1) Cash Payments 2) Quotes/Offers 3) Orders 4) Fixed Assets 5) Inventory/Warehouse. For each module: create database schema with migration, add demo seed data, implement backend API handlers, create frontend Svelte page with navigation and i18n, test in demo mode. Update progress table in the plan doc after each module. After all 5 complete, re-evaluate against SmartAccounts." --max-iterations 50 --completion-promise "All 5 modules implemented"
```

### Shorter Version (Single Module)

```
/ralph-loop "Implement Cash Payments module for open-accounting. See docs/plans/ralph-loop-missing-modules.md for schema. Create: 1) DB migration 2) Backend handlers 3) Frontend /payments/cash route 4) i18n strings 5) Demo seed data. Test in demo mode." --max-iterations 15
```

### Cancel Command

To stop the loop at any time:
```
/cancel-ralph
```

---

## Phase 2: Additional Missing Features

After completing the initial 5 modules, a detailed comparison with SmartAccounts revealed additional gaps. These are prioritized for the next development cycle.

### High Priority (Estonian Compliance & Core Reports)

| # | Feature | Estonian | Description |
|---|---------|----------|-------------|
| 1 | **VAT Returns** | Käibedeklaratsioonid (KMD) | Estonian VAT declaration management with XML export |
| 2 | **Cash Flow Statement** | Rahavoogude aruanne | Standard financial report (operating/investing/financing) |
| 3 | **Leave Management** | Puhkused/puudumised | Employee leave tracking, vacation days, sick leave |

### Medium Priority (Enhanced Functionality)

| # | Feature | Estonian | Description |
|---|---------|----------|-------------|
| 4 | **Payment Reminders** | Meeldetuletused | Automated payment reminder system for overdue invoices |
| 5 | **E-Invoice Import** | E-arve import | Import Estonian e-invoice XML format |
| 6 | **Annual Report** | Majandusaasta aruanne | Generate Estonian annual financial report |
| 7 | **Salary Calculator** | Palgakalkulaator | Calculate gross/net salary with Estonian taxes |
| 8 | **Purchase Orders** | Ostutellimsued | Separate purchase order management view |

### Low Priority (Nice to Have)

| # | Feature | Estonian | Description |
|---|---------|----------|-------------|
| 9 | Pending Purchase Invoices | Ootel ostuarved | Queue for purchase invoices awaiting approval |
| 10 | Payment Orders Export | Maksekorralduste eksport | Export bank payment orders file |
| 11 | Stock Levels Import | Laoseisude import | Bulk import inventory stock levels |
| 12 | Fixed Assets Settings | Põhivara seaded | Configuration page for asset categories |
| 13 | Payroll Settings | Palgaarvestuse seaded | Configuration page for payroll parameters |
| 14 | Stock Levels Report | Laoseisude aruanne | Inventory stock report by warehouse |
| 15 | Warehouse Movements Report | Laoliikumiste aruanne | Report of all inventory movements |
| 16 | Wage Reports | Palgaaruanded | Detailed payroll reports |

### Settings & Configuration (Missing)

| Feature | Estonian | URL | Description |
|---------|----------|-----|-------------|
| Document Templates | Dokumendimallid | /settings/invoicetemplate | Customize invoice/quote templates |
| Report Settings | Aruannete seaded | /reportsettings | Configure report parameters |
| Countries | Riigid | /countries | Country codes and settings |
| Business Years | Majandusaastad | /businessyears | Fiscal year management |
| Cost Centers | Objektid | /objects | Project/department tracking |
| Default Accounts | Vaikimisi kontod | /defaultaccounts | Auto-fill accounts for transactions |
| VAT Declaration Settings | Käibedeklaratsiooni seaded | /returnsettings | KMD configuration |
| VAT Rate Replacement | Käibemaksumäärade asendamine | /settings/vatupdate | Bulk update VAT rates |
| Maintenance | Hooldus | /settings/maintenance | System maintenance tools |
| Connected Services | Liidetud teenused | /settings/connectedservices | Third-party integrations |
| Devices | Seadmed | /settings/devices | POS devices management |

### Missing Reports

SmartAccounts has 34+ reports. High-value missing reports:

| Report | Estonian | Priority |
|--------|----------|----------|
| Cash Report | Kassaaruanne | HIGH |
| Sales by Articles | Artiklite müügiaruanne | MEDIUM |
| Sales by Date | Müügikäive päevade lõikes | MEDIUM |
| Sales Margins | Müügimarginaalid | MEDIUM |
| Client Prepayments | Klientide ettemaksud | LOW |
| VAT Report (MOSS) | Käibemaksuaruanne | HIGH |
| Object Report | Objekti aruanne | LOW |
| Warehouse Movements | Laoliikumised perioodil | MEDIUM |
| Balance Confirmation | Saldoteatis | MEDIUM |
| Entries by Date Created | Sisestatud/muudetud kanded | LOW |

---

## Phase 2 Progress Tracking

| Feature | Started | Schema | Backend | Frontend | Tests | Complete |
|---------|---------|--------|---------|----------|-------|----------|
| VAT Returns (KMD) | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| Cash Flow Statement | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| Leave Management | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| Payment Reminders | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| E-Invoice Import | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| Annual Report | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| Salary Calculator | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| Purchase Orders | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |

---

## Phase 2 Loop Command

```
/ralph-loop "Implement Phase 2 features for open-accounting. Read docs/plans/ralph-loop-missing-modules.md Phase 2 section. Work through: 1) VAT Returns (KMD) 2) Cash Flow Statement 3) Leave Management. For each: create database schema, backend API, frontend page, i18n, demo data, tests." --max-iterations 50
```

---

## Phase 2 Feature Specifications

### 1. VAT Returns (Käibedeklaratsioonid / KMD)

**Purpose:** Estonian VAT declaration management with automatic calculation and XML export

**Database:**
```sql
CREATE TABLE vat_returns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    year INTEGER NOT NULL,
    month INTEGER NOT NULL, -- 1-12
    due_date DATE NOT NULL,
    status VARCHAR(20) DEFAULT 'draft', -- draft, submitted, paid
    -- KMD fields (Estonian VAT declaration)
    kmd_1 DECIMAL(15,2) DEFAULT 0, -- Taxable transactions 22%
    kmd_2 DECIMAL(15,2) DEFAULT 0, -- Taxable transactions 9%
    kmd_3 DECIMAL(15,2) DEFAULT 0, -- Zero-rated exports
    kmd_4 DECIMAL(15,2) DEFAULT 0, -- Input VAT 22%
    kmd_5 DECIMAL(15,2) DEFAULT 0, -- Input VAT 9%
    kmd_total DECIMAL(15,2) DEFAULT 0, -- Total to pay
    paid_amount DECIMAL(15,2) DEFAULT 0,
    currency VARCHAR(3) DEFAULT 'EUR',
    submitted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(tenant_id, year, month)
);
```

**Routes:** `/vat-returns`

**Features:**
- Auto-calculate from invoices and purchases
- XML export for e-MTA submission
- PDF/XLS/CSV export
- Payment tracking

---

### 2. Cash Flow Statement (Rahavoogude aruanne)

**Purpose:** Estonian-standard financial report showing cash flows from operating, investing, and financing activities

**Implementation:** Report generator (no new tables needed)

**Routes:** `/reports/cash-flow`

**Report Types:**
- Lihtne (Simple)
- Võrdlev: eelnevad kuud (Comparative: previous months)
- Võrdlev: eelnevad majandusaastad (Comparative: previous business years)
- Võrdlev: eelnevad kvartalid (Comparative: previous quarters)
- Võrdlev: objektid (Comparative: cost centers)

**Sections (Estonian Standard):**

1. **Rahavood äritegevusest (Operating Activities)**
   - Kaupade või teenuste müügist laekunud raha (Cash from sales)
   - Kaupade, materjalide ja teenuste eest makstud raha (Cash paid for goods/services)
   - Makstud palgad (Wages paid)
   - Makstud tulumaks (Income tax paid)
   - Makstud intressid (Interest paid)

2. **Rahavood investeerimistegevusest (Investing Activities)**
   - Materiaalse ja immateriaalse põhivara ost ja müük (Fixed assets)
   - Kinnisvarainvesteeringute ost ja müük (Investment property)
   - Tütar- ja sidusettevõtete ost ja müük (Subsidiaries)
   - Muude finantsinvesteeringute ost ja müük (Other investments)
   - Teistele osapooltele antud laenud (Loans given)
   - Antud laenude laekumised (Loan repayments received)
   - Saadud intressid ja dividendid (Interest/dividends received)

3. **Rahavood finantseerimistegevusest (Financing Activities)**
   - Laenude saamine (Loans received)
   - Saadud laenude tagasimaksmine (Loan repayments)
   - Kapitalirendi maksed (Finance lease payments)
   - Aktsiate emiteerimine (Share issuance)
   - Omaaktsiate ostmine ja müük (Treasury shares)
   - Dividendide maksmine (Dividends paid)

**Export:** PDF, XLS, CSV

---

### 3. Leave Management (Puhkused/puudumised)

**Purpose:** Employee leave tracking with Estonian-specific leave types

**Database:**
```sql
CREATE TABLE employee_absences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    employee_id UUID NOT NULL REFERENCES employees(id),
    absence_type VARCHAR(50) NOT NULL, -- See types below
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    days_count INTEGER NOT NULL,
    description TEXT,
    status VARCHAR(20) DEFAULT 'approved', -- pending, approved, rejected
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE public_holidays (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    country_code VARCHAR(2) DEFAULT 'EE',
    holiday_date DATE NOT NULL,
    name VARCHAR(255) NOT NULL,
    name_et VARCHAR(255),
    UNIQUE(country_code, holiday_date)
);
```

**Estonian Leave Types:**
- `annual_leave` - Põhipuhkus (töötasu säilitatakse)
- `annual_leave_avg` - Põhipuhkus (6 kuu keskmine tasu)
- `unpaid_leave` - Tasustamata puhkus
- `sick_leave` - Haigusleht
- `care_leave` - Hooldusleht
- `study_leave_paid` - Õppepuhkus tasustatud
- `study_leave_unpaid` - Õppepuhkus tasustamata
- `maternity_leave` - Rasedus- ja sünnituspuhkus
- `paternity_leave` - Isapuhkus
- `child_leave` - Lapsepuhkus

**Routes:** `/employees/absences`

**Features:**
- Calendar view of absences
- Public holiday display
- Working days calculation per month
- Leave balance tracking

---

### 4. Salary Calculator (Palgakalkulaator)

**Purpose:** Calculate gross/net salary with Estonian tax rates

**Implementation:** Calculator tool (no database needed, uses current tax rates)

**Routes:** `/payroll/calculator`

**Features:**
- Year selection (tax rates change annually)
- Calculation modes:
  - Kulu tööandjale → Bruto → Neto (Employer cost to Net)
  - Bruto → Neto (Gross to Net)
  - Neto → Bruto (Net to Gross)
- Employment type settings:
  - Tööleping (Employment contract)
  - Juhatuse liikme tasu (Board member fee)
- Tax options:
  - Arvesta sotsiaalmaksu min. kuumäära alusel (Social tax minimum)
  - Isik on jõudnud vanaduspensioniikka (Retirement age - no unemployment insurance)
  - Maksuvaba tulu (Tax-free income amount)
  - Tööandja töötuskindlustusmakse (Employer unemployment insurance)
  - Töötaja töötuskindlustusmakse (Employee unemployment insurance)
  - Kogumispension (Pension pillar: 2%, 4%, 6%)

**Estonian Tax Rates (2026):**
- Social tax: 33%
- Unemployment insurance (employer): 0.8%
- Unemployment insurance (employee): 1.6%
- Income tax: 20%
- Tax-free income: sliding scale based on income

---

### 5. Invoice Enhancements (From SmartAccounts Analysis)

**Missing Invoice Features:**
- Copy/duplicate invoice
- Late fee percentage (Viivis) per invoice
- Estonian payment reference number (Viitenumber)
- Cost centers (Objektid) per document and line
- PDF template selection
- File attachments
- Internal notes field
- Email sent status tracking
- Line-level discounts (AH %)

---

### 5. Cost Centers / Objects (Objektid)

**Purpose:** Track revenue/expenses by project, department, or cost center

**Database:**
```sql
CREATE TABLE cost_centers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    code VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    is_active BOOLEAN DEFAULT true,
    parent_id UUID REFERENCES cost_centers(id),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(tenant_id, code)
);
```

**Routes:** `/settings/cost-centers`

**Usage:** Add `cost_center_id` to invoices, journal entries, invoice lines

---

## Complete SmartAccounts Feature Matrix

### TIER 1: Critical Estonian Compliance

| Feature | Estonian | SmartAccounts | Open-Accounting | Priority |
|---------|----------|---------------|-----------------|----------|
| VAT Returns | Käibedeklaratsioonid (KMD) | ✅ | ❌ | **CRITICAL** |
| Annual Report Generator | Majandusaasta aruanne | ✅ | ❌ | **CRITICAL** |
| E-Invoice Import | E-arve import | ✅ | ❌ | **HIGH** |
| E-Invoice Export/Sending | E-arve väljastamine | ✅ | ❌ | **HIGH** |

### TIER 2: Important Financial Features

| Feature | Estonian | SmartAccounts | Open-Accounting | Priority |
|---------|----------|---------------|-----------------|----------|
| Cash Flow Statement | Rahavoogude aruanne | ✅ | ❌ | HIGH |
| Leave Management | Puhkused/puudumised | ✅ | ❌ | HIGH |
| Salary Calculator | Palgakalkulaator | ✅ | ❌ | HIGH |
| Payment Reminders | Meeldetuletused | ✅ | ❌ | HIGH |
| Balance Confirmations | Saldoteatis | ✅ | ❌ | HIGH |
| Prepayment Tracking | Ettemaksed | ✅ | ❌ | MEDIUM |

### TIER 3: Enhanced Functionality

| Feature | Estonian | SmartAccounts | Open-Accounting | Priority |
|---------|----------|---------------|-----------------|----------|
| Cost Centers/Objects | Objektid | ✅ | ❌ | MEDIUM |
| Business Years | Majandusaastad | ✅ | ❌ | MEDIUM |
| Default Accounts | Vaikimisi kontod | ✅ | ❌ | MEDIUM |
| Purchase Orders (separate) | Ostutellimused | ✅ | ✅ (combined) | LOW |
| Pending Invoices Queue | Ootel ostuarved | ✅ | ❌ | LOW |
| Payment Reference Numbers | Viitenumber | ✅ | ❌ | MEDIUM |
| Client/Vendor Statements | Väljavõte | ✅ | ❌ | MEDIUM |

### TIER 4: Reports (33 Missing)

**Sales Reports (Müügiaruanded):**
| Report | Estonian | Priority |
|--------|----------|----------|
| Sales Invoice List | Müügiarvete nimekiri | LOW |
| Client Receivables | Klientide tasumata arved | MEDIUM |
| Balance Confirmation PDF | Saldoteatis | HIGH |
| Cash Report | Kassaaruanne | HIGH |
| Sales by Articles | Artiklite müügiaruanne | MEDIUM |
| Sales by Date (Graph) | Müügikäive päevade lõikes | MEDIUM |
| Sales by Client | Müük klientide lõikes | MEDIUM |
| Sales Margins | Müügimarginaalid | MEDIUM |
| Invoice-based Margins | Müügimarginaalid, arvepõhine | LOW |
| Client Periodic Statement | Kliendi väljavõte | MEDIUM |
| Client Prepayments | Klientide ettemaksed | MEDIUM |
| Article Margins (FIFO) | Artiklite müügimarginaalid | LOW |
| VAT Report (MOSS) | Käibemaksuaruanne | HIGH |
| Monthly Sales by Article | Artiklite müügikäive kuude lõikes | LOW |
| Receipts by Object | Laekumised objektiga | LOW |
| Sales Total by Article | Artiklite müügi koondaruanne | LOW |

**Purchase Reports (Ostuaruanded):**
| Report | Estonian | Priority |
|--------|----------|----------|
| Purchase Invoice List | Ostuarvete nimekiri | LOW |
| Vendor Payables | Hankijate tasumata arved | MEDIUM |
| Vendor Balance Confirmation | Hankija saldoteatis | HIGH |
| Purchases by Articles | Artiklite ostuaruanne | MEDIUM |
| Purchases by Vendor | Ost hankijate lõikes | MEDIUM |
| Vendor Periodic Statement | Hankija väljavõte | MEDIUM |
| Vendor Prepayments | Hankijate ettemaksed | MEDIUM |
| Purchase Total by Article | Artiklite ostu koondaruanne | LOW |

**Other Reports:**
| Report | Estonian | Priority |
|--------|----------|----------|
| Quotes Report | Pakkumised | LOW |
| Articles in Orders | Artiklid tellimustes | LOW |
| Orders Report | Tellimused | LOW |
| Entries by Date Created | Sisestatud/muudetud kanded | LOW |
| Cost Center Report | Objekti aruanne | MEDIUM |
| Entries with Counterparty | Kanded osapoole andmetega | LOW |
| Fixed Asset Report | Põhivaraaruanne | MEDIUM |
| Warehouse Movements | Laoliikumised perioodil | MEDIUM |
| Annual Report Helper | Majandusaasta aruande abi | HIGH |

### TIER 5: Invoice Enhancements

| Feature | Estonian | Priority |
|---------|----------|----------|
| Copy/Duplicate Invoice | Kopeeri | LOW |
| Late Fee Percentage | Viivis (%) | LOW |
| File Attachments | Failid | LOW |
| Internal Notes | Siseinfo | LOW |
| Email Sent Status | Saadetud olek | LOW |
| Line-level Discounts | Rida allahindlus (AH %) | LOW |
| PDF Template Selection | PDF mall | MEDIUM |

### TIER 6: Settings & Configuration

| Feature | Estonian | Priority |
|---------|----------|----------|
| Document Templates | Dokumendimallid | MEDIUM |
| Email Settings | E-kirjad | LOW |
| Report Settings | Aruannete seaded | LOW |
| Countries | Riigid | LOW |
| VAT Declaration Settings | Käibedeklaratsiooni seaded | MEDIUM |
| VAT Rate Bulk Update | Käibemaksumäärade asendamine | LOW |
| System Parameters | Parameetrid | LOW |
| Maintenance Tools | Hooldus | LOW |
| Connected Services | Liidetud teenused | LOW |
| Devices (POS) | Seadmed | LOW |
| Payroll Settings | Palgaarvestuse seaded | MEDIUM |
| Fixed Asset Settings | Põhivara seaded | LOW |

### TIER 7: Import/Export

| Feature | Estonian | Priority |
|---------|----------|----------|
| Payment Orders Export | Maksekorralduste eksport | MEDIUM |
| Stock Levels Import | Laoseisude import | LOW |
| Article Import/Export | Artiklite import/eksport | LOW |
| Bank Statement Import | Pangamaksete import | ✅ HAVE |

---

## Summary: Feature Completeness

**After Phase 1 (5 modules):** ~70% feature parity

**Total Missing Features Identified:** 65+
- TIER 1 Critical: 4 features
- TIER 2 Important: 6 features
- TIER 3 Enhanced: 7 features
- TIER 4 Reports: 33 reports
- TIER 5 Invoice: 7 enhancements
- TIER 6 Settings: 12 configs
- TIER 7 Import/Export: 3 features

**Recommended Phase 2 Priority:**
1. VAT Returns (KMD) - Critical compliance
2. Annual Report Generator - Critical compliance
3. Cash Flow Statement - Standard report
4. Leave Management - Payroll completeness
5. Balance Confirmations - Year-end requirement
6. Payment Reminders - Cash flow management
