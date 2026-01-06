# SmartAccounts Feature Analysis & Comparison

## Overview

This document analyzes the SmartAccounts.eu accounting software features and compares them with our open-accounting implementation to identify gaps and opportunities.

---

## 1. SmartAccounts Feature Inventory

### 1.1 Dashboard (Avaleht)

| Feature | Description |
|---------|-------------|
| Company Overview | Cash balance, receivables, liabilities summary |
| Overdue Tracking | Separate display of overdue receivables/payables |
| Orders/Offers Summary | Current orders and offers totals |
| Currency Widget | Live ECB exchange rates (SEK, NOK, DKK, USD, GBP, RUB) |
| Cash Flow Chart | Visual comparison of cash/receivables vs liabilities |
| Recent Transactions | Latest accounting entries with quick links |
| 30-Day Cash Forecast | Predictive cash flow chart |
| 6-Month Revenue/Expense Chart | Monthly income, expenses, and profit trend |
| Sales Hits Chart | Top-selling articles pie chart |
| Unpaid Invoices Widget | Breakdown by vendor with pie chart |

### 1.2 Sales/Purchase Module (Ost/müük)

#### Sales Invoices (Müügiarved)
- Invoice list with filtering (client, invoice #, object, date range, payment status)
- Export to PDF, XLS, CSV
- Sortable columns (NR, Client, Invoice, Date, Entry, Due Date, Amount, Currency, Object, Created, Creator)
- Page totals summary (row count, sum, outstanding, overdue)
- Quick add new invoice

#### Purchase Invoices (Ostuarved)
- Similar functionality to sales invoices
- Vendor-centric view

#### Quotes/Offers (Pakkumised)
- Client quote management
- Convert quotes to orders/invoices

#### Orders (Tellimused)
- Order management
- Order tracking

### 1.3 Payments Module (Maksed)

#### Bank Payments (Pangamaksed)
- Bank transaction list
- Multi-bank account support
- Transaction reconciliation
- Auto-import from banks (LHV, SEB, Coop)
- Payment matching

#### Cash Payments (Kassamaksed)
- Cash register transactions
- Cash account management

### 1.4 General Ledger (Pearaamat)

| Feature | Description |
|---------|-------------|
| Journal Entries | Manual entry creation and editing |
| Chart of Accounts | Hierarchical account structure |
| Account Categories | Assets, Liabilities, Equity, Revenue, Expenses |
| Account Balances | Real-time balance display |

### 1.5 Reports (Aruanded)

#### Financial Statements
- **Balance Sheet (Bilanss)** - Standard balance sheet report
- **Income Statement (Kasumiaruanne)** - P&L report

#### Sales Reports (Müügiaruanded)
- Invoice list by period
- Client outstanding invoices
- Balance confirmation (Saldoteatis)
- Cash report
- Article sales report
- Sales by date
- Sales by client
- Sales margins
- Sales margins by invoice
- Client periodic statement
- Client prepayments
- Article margins
- VAT report (MOSS)
- Article sales by month
- Receipts by invoice object
- Sales total summary

#### Purchase Reports (Ostuaruanded)
- Invoice list by period
- Vendor outstanding invoices
- Vendor balance confirmation
- Article purchase report
- Purchases by vendor
- Vendor periodic statement
- Vendor prepayments
- Purchase total summary

#### Other Reports
- Quotes report
- Orders report
- Articles in orders
- Created/modified entries
- Object report
- Entries with counterparty data
- Fixed asset report
- Warehouse movements report
- Annual report helper

### 1.6 Tax Module (Maksud)

| Feature | Description |
|---------|-------------|
| VAT Returns (Käibedeklaratsioonid) | Estonian VAT declaration preparation |
| TSD Returns | Income and social tax returns |
| VAT Settings | VAT code configuration |
| VAT Rate Management | Multiple VAT rates |
| VAT Rate Replacement | Bulk VAT rate updates |

### 1.7 Inventory/Warehouse (Ladu)

| Feature | Description |
|---------|-------------|
| Warehouse Movements | Stock in/out tracking |
| Inventory Import | Bulk inventory import |
| Inventory Report | Current stock levels |
| Movement Report | Stock movement history |
| Multiple Warehouses | Multi-location support |

### 1.8 Fixed Assets (Põhivarad)

| Feature | Description |
|---------|-------------|
| Asset Register | Fixed asset list |
| Depreciation | Automatic depreciation calculation |
| Asset Report | Depreciation schedule |
| Asset Settings | Depreciation methods configuration |

### 1.9 Payroll (Palk)

| Feature | Description |
|---------|-------------|
| Employees (Töötajad) | Employee master data |
| Absences (Puhkused/puudumised) | Vacation and absence tracking |
| Salaries (Töötasud) | Salary calculation and processing |
| Salary Calculator | Gross/net calculator |
| Payroll Reports | Various payroll reports |
| Payroll Settings | Tax rates, pension rates configuration |

### 1.10 Settings & Configuration

#### Company Settings
- Document templates (invoice design)
- Email settings
- Chart of accounts
- Default accounts
- Report settings
- Countries
- Fiscal years (Majandusaastad)
- Cost centers/Objects
- Payment methods
- VAT declaration settings
- Parameters
- Maintenance
- Connected services
- Devices
- Users and groups
- Billing settings

#### User Settings
- Personal information
- My companies (multi-company)
- Billing/subscription

### 1.11 Integrations (Liidetud teenused)

| Integration | Type |
|-------------|------|
| E-invoicing | Send/receive e-invoices |
| Pactrics | ? |
| Shopify | E-commerce sync |
| LHV Bank | Direct bank connection |
| SEB Bank | Direct bank connection |
| Coop Pank | Direct bank connection |
| EveryPay | Payment processing |
| Scoro | Business management |
| ENVOICE | E-invoicing |
| Plus | ? |
| Soapbox | ? |
| Yetim | ? |
| Kensq | ? |
| API | REST API access |

### 1.12 UI/UX Features

- Multi-language (Estonian, English)
- Notifications system
- Quick company switch
- Contextual help on all pages
- Responsive sidebar navigation
- Data export (PDF, XLS, CSV)
- Advanced filtering
- Sortable tables
- Page summary totals

---

## 2. Open-Accounting Current Features

Based on codebase analysis (routes found in `frontend/src/routes/`):

### 2.1 Implemented Features

| Module | Route | Status | Notes |
|--------|-------|--------|-------|
| Dashboard | `/dashboard` | ✅ Yes | KPIs, financial overview |
| Sales Invoices | `/invoices` | ✅ Yes | CRUD, list, filtering |
| Purchase Invoices | `/invoices` | ✅ Yes | Combined with sales (type filter) |
| Bank Payments | `/banking` | ✅ Yes | Import, reconciliation |
| Bank Import | `/banking/import` | ✅ Yes | Statement import |
| Payments | `/payments` | ✅ Yes | Payment tracking |
| Cash Payments | `/payments/cash` | ✅ Yes | Cash income/expense tracking |
| Journal Entries | `/journal` | ✅ Yes | Manual entries |
| Chart of Accounts | `/accounts` | ✅ Yes | With balances |
| Reports | `/reports` | ✅ Yes | Balance Sheet, Income Statement |
| VAT Returns | `/tax` | ✅ Yes | Estonian KMD |
| TSD Returns | `/tsd` | ✅ Yes | Income/social tax |
| Employees | `/employees` | ✅ Yes | Employee master data |
| Payroll | `/payroll` | ✅ Yes | Salary calculation |
| Recurring Invoices | `/recurring` | ✅ Yes | Automatic generation |
| Contacts | `/contacts` | ✅ Yes | Clients and vendors |
| Settings | `/settings/*` | ✅ Yes | Company, email, plugins |
| Login/Auth | `/login` | ✅ Yes | Authentication |
| Admin | `/admin/plugins` | ✅ Yes | Plugin management |
| Fixed Assets | `/assets` | ✅ Yes | Asset register, depreciation, categories |
| Inventory | `/inventory` | ✅ Yes | Products, warehouses, stock, movements |
| Quotes/Offers | `/quotes` | ✅ Yes | Quote management, convert to order/invoice |
| Orders | `/orders` | ✅ Yes | Order tracking, status workflow |
| Multi-tenant | - | ✅ Yes | Demo mode with isolated tenants |
| Multi-language | - | ✅ Yes | Estonian, English (i18n) |

### 2.2 Missing Features (Gap Analysis)

#### Critical Gaps
1. **Fixed Assets Module** - No depreciation tracking
2. **Inventory/Warehouse** - No stock management
3. **Cash Payments** - No cash register support
4. **Quotes/Offers** - No quote management
5. **Orders** - No order management
6. **Multi-company** - Single company per user

#### Report Gaps
1. Sales margins report
2. Article sales by month
3. Vendor periodic statement
4. Object/cost center report
5. 30-day cash forecast
6. Sales hits analysis

#### Dashboard Gaps
1. 30-day cash forecast chart
2. Sales hits pie chart
3. Unpaid invoices by vendor breakdown
4. Currency rates widget

#### Integration Gaps
1. Direct bank connections (LHV, SEB, Coop)
2. E-invoicing send/receive
3. Payment gateway integration
4. E-commerce sync (Shopify)
5. REST API for third-party access

#### UX Gaps
1. Document templates (customizable invoice design)
2. Email templates and sending
3. Balance confirmation PDF generation
4. Notification system
5. Quick add buttons
6. Page totals summary on lists

---

## 3. Prioritized Roadmap Recommendations

### Phase 1: Core Accounting Completeness
1. Cash Payments module
2. Fixed Assets with depreciation
3. Inventory/Warehouse basics
4. Quote → Order → Invoice workflow

### Phase 2: Reporting Excellence
1. Enhanced dashboard with forecasting
2. Sales margin reports
3. Vendor/client periodic statements
4. Cost center/object reporting

### Phase 3: Integration & Automation
1. Estonian bank direct connections
2. E-invoicing (Finbite, ENVOICE)
3. REST API for integrations
4. Email sending with templates

### Phase 4: Advanced Features
1. Multi-company support
2. Document template designer
3. Advanced inventory (multiple warehouses, FIFO)
4. Payment gateway integration

---

## 4. Technical Notes

### SmartAccounts Architecture Observations
- Server-rendered pages with progressive enhancement
- AJAX-based data loading
- Hierarchical navigation structure
- Session-based authentication
- PDF generation for reports and documents
- Bank statement import (various formats)

### Recommendations for Open-Accounting
- Keep SvelteKit SSR approach for SEO and performance
- Add PDF export using existing infrastructure
- Consider WebSocket for real-time notifications
- Plan API-first approach for integrations
