# Open Accounting

🇪🇪 **Made in Estonia** | Open-source accounting software for modern businesses

[![CI](https://github.com/HMB-research/open-accounting/actions/workflows/ci.yml/badge.svg)](https://github.com/HMB-research/open-accounting/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/HMB-research/open-accounting/branch/main/graph/badge.svg)](https://codecov.io/gh/HMB-research/open-accounting)
[![CLI coverage](https://img.shields.io/badge/CLI%20coverage-100%25-brightgreen)](docs/USE_CASE_COVERAGE.md#current-evidence-baseline)
[![Docs gate](https://img.shields.io/badge/docs%20gate-passing-brightgreen)](docs/DEVELOPMENT_STATUS.md#verified-engineering-baseline)
[![Demo E2E](https://img.shields.io/badge/demo%20E2E-blocking%20CI-brightgreen)](docs/demo-e2e-testing.md)
[![Go Report Card](https://goreportcard.com/badge/github.com/HMB-research/open-accounting)](https://goreportcard.com/report/github.com/HMB-research/open-accounting)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go)](https://go.dev/)

[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16+-4169E1?logo=postgresql&logoColor=white)](https://postgresql.org/)
[![SvelteKit](https://img.shields.io/badge/SvelteKit-2-FF3E00?logo=svelte&logoColor=white)](https://kit.svelte.dev/)
[![Vite](https://img.shields.io/badge/Vite-7-646CFF?logo=vite&logoColor=white)](https://vitejs.dev/)
[![TypeScript](https://img.shields.io/badge/TypeScript-5-3178C6?logo=typescript&logoColor=white)](https://typescriptlang.org/)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?logo=docker&logoColor=white)](https://docker.com/)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

> **⚠️ Development Status**
> This project is under active development and not yet production-ready. APIs may change, and features may be incomplete. Contributions and feedback welcome!
>
> Current product caps and gaps are summarized in [Current Product Limits](docs/CURRENT_PRODUCT_LIMITS.md). Detailed feature status and validation evidence live in [Development Status](docs/DEVELOPMENT_STATUS.md), and use-case-level coverage lives in [Use Case Coverage](docs/USE_CASE_COVERAGE.md).
> The `cmd/oa` CLI package is held at 100.0% statement coverage by `make test-backend-coverage` in the backend CI gate and `make test-cli-coverage` for focused CLI changes. Local seeded smoke plus full demo E2E shards are CI gates. The optional remote hosted-demo E2E job remains informational.
> Production hardening, deeper historical cutover tooling, broader accountant-workspace execution, remaining payroll/document/evidence-policy edges, and broader workflow-level policy enforcement are still in progress.

CLI access is available via `go run ./cmd/oa`. It bootstraps a tenant-scoped API token once and then uses that token for subsequent reads and mutations.

---

## 🎮 Demo

The previous hosted Railway demo is currently offline.

For a resettable local demo instead:

```bash
docker-compose up -d db
export DATABASE_URL="postgres://openaccounting:openaccounting@localhost:5432/openaccounting?sslmode=disable"
go run ./cmd/migrate -db "$DATABASE_URL" -path migrations -direction up
DEMO_MODE=true DEMO_RESET_SECRET=test-demo-secret go run ./cmd/api
curl -X POST http://localhost:8080/api/demo/reset -H 'X-Demo-Secret: test-demo-secret'
```

| Credential   | Value               |
| ------------ | ------------------- |
| **Email**    | `demo1@example.com` |
| **Password** | `demo12345`         |

---

## What is Open Accounting?

Open Accounting is a **self-hosted, multi-tenant accounting platform** focused today on **Estonian SMB and accountant workflows**. The current wedge is accounting, invoicing, recurring billing, payroll, bank import/reconciliation, and KMD/TSD export for self-hosted teams that want source access and tenant isolation.

It is not yet a full SmartAccounts/Merit replacement or a production-hardened embedded accounting platform. Built with modern technologies and focused on **Estonian/EU compliance**, it provides:

- **True Double-Entry Bookkeeping** — Immutable journal entries with full audit trail
- **Multi-Company Support** — One installation serves multiple businesses with complete data isolation
- **Role-Based Access** — Owner, Admin, Accountant, and Viewer roles with granular permissions
- **Accountant Review Queue** — Dashboard review surface for overdue invoices, unmatched bank transactions, close status, recent journal activity, and assignment-ready remediation actions, with a cross-tenant portfolio rollup for accountant users
- **Estonian Tax Compliance** — KMD (VAT) declarations with e-MTA XML export
- **Modern Stack** — Go backend, SvelteKit frontend, PostgreSQL database

---

## ✨ Features

> Status note: features listed below exist in the repository. That does not mean each one is production-hardened, accountant-grade, or at full parity with proprietary incumbents.

### Core Accounting

| Feature                    | Description                                                                        |
| -------------------------- | ---------------------------------------------------------------------------------- |
| **Chart of Accounts**      | Hierarchical 5-type account structure (Asset, Liability, Equity, Revenue, Expense) |
| **Journal Entries**        | Draft → Posted → Void workflow with reversal entries                               |
| **Multi-Currency**         | Support for multiple currencies with exchange rate tracking                        |
| **Trial Balance**          | Real-time balance reports as of any date                                           |
| **Balance Sheet**          | Assets, liabilities, and equity statement                                          |
| **Income Statement**       | Revenue and expense summary (P&L)                                                  |
| **Consolidated Reporting** | Multi-company trial balance, balance sheet, and income statement consolidation     |
| **Report Exports**         | Export to Excel, CSV, or PDF formats                                               |
| **VAT Tracking**           | Date-aware VAT rates for proper EU compliance                                      |

### Business Operations

| Feature                | Description                                                                                  |
| ---------------------- | -------------------------------------------------------------------------------------------- |
| **Invoicing**          | Sales and purchase invoices with line items, VAT, CSV import, and manual Estonian e-invoice XML import |
| **Quotes**             | Sales quotes with draft/sent/accepted workflow, conversion to orders, and grouped CSV import |
| **Orders**             | Order management with quote linking, status tracking, draft invoice conversion, and grouped CSV import |
| **Contacts**           | Customer and supplier management                                                             |
| **Payments**           | Payment recording with invoice allocation                                                    |
| **Expenses**           | Receipt-backed expense claims with approval, remediation actions, and ledger posting         |
| **PDF Generation**     | Professional invoice PDFs with customizable branding                                         |
| **Recurring Invoices** | Automated invoice generation on schedule with grouped CSV import                             |

### Fixed Assets

| Feature                  | Description                                                                                                                 |
| ------------------------ | --------------------------------------------------------------------------------------------------------------------------- |
| **Asset Tracking**       | Register, import, and track fixed assets with serial numbers and locations                                                  |
| **Asset Categories**     | IT Equipment, Office Furniture, Vehicles, Software with depreciation settings                                               |
| **Depreciation**         | Straight-line and declining balance methods with configurable useful life                                                   |
| **Asset Lifecycle**      | Draft → Active → Disposed/Sold/Scrapped status workflow                                                                     |
| **Depreciation Entries** | Automatic depreciation calculations with audit trail and required linked ledger posting                                     |
| **Disposal Accounting**  | Approved disposals can post balanced ledger entries for cost removal, accumulated depreciation, proceeds, gains, and losses |

### Banking & Reconciliation

| Feature                | Description                                      |
| ---------------------- | ------------------------------------------------ |
| **Bank Accounts**      | Track multiple bank accounts per company         |
| **Transaction Import** | Generic and LHV CSV/camt.053 bank statement import with account and currency checks |
| **Auto-Matching**      | Intelligent matching of transactions to payments |
| **Reconciliation**     | Full bank reconciliation workflow                |

### Multi-Tenant & Security

| Feature                    | Description                                                                                                                         |
| -------------------------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| **Tenant Isolation**       | Schema-per-tenant for complete data separation                                                                                      |
| **User Management**        | Invite users, assign roles, manage permissions                                                                                      |
| **JWT and API token auth** | Purpose-scoped JWT access/refresh tokens with revocable refresh sessions plus suspend-aware tenant-scoped API tokens for automation |
| **RBAC**                   | Role-based access control with permission checks                                                                                    |
| **API Rate Limiting**      | Token bucket rate limiting with configurable thresholds                                                                             |

### Payroll (Estonian)

| Feature                        | Description                                                                                                                                                                                 |
| ------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Employee Management**        | Full employee lifecycle with personal codes                                                                                                                                                 |
| **Estonian Tax Calculations**  | Income tax, social tax, unemployment insurance                                                                                                                                              |
| **Funded Pension (II Pillar)** | Configurable pension contribution rates                                                                                                                                                     |
| **Payroll Runs**               | Monthly payroll with draft → approved → paid workflow                                                                                                                                       |
| **Payslips**                   | Detailed breakdown of earnings and deductions                                                                                                                                               |
| **TSD Declaration**            | Annex 1 generation with XML/CSV export for e-MTA, approved tax/support evidence before marking submitted or accepted, plus historical TSD CSV import for migration cutovers                    |
| **Historical Payroll Import**  | CSV import of finalized prior payroll runs and payslips through API, web UI, and CLI                                                                                                        |
| **Historical TSD Import**      | CSV import of prior TSD declarations and Annex 1 rows through API and CLI                                                                                                                   |
| **Leave Balance Import**       | CSV import of employee leave balances for migration cutovers through API, web UI, and CLI                                                                                                   |
| **Leave Evidence**             | Approved supporting documents can be required before approving documented leave/absence records                                                                                             |
| **Migration Preflight**        | Non-mutating CSV/XML bundle validation for required columns, duplicate identifiers and history keys, account, contact, employee, payroll-history, commercial-document, inventory, fixed-asset, cost-center/allocation, expense, payment, bank-account, bank-transaction, opening-balance, and historical-journal row values, grouped document and preserved-ID consistency, provider-style aliases for Merit, SmartAccounts, and Directo CSVs including payment currency aliases, cross-file references before cutover imports, and guarded CLI plus server-side API execution for fully ready plans with resume snapshots for interrupted runs, including e-invoice XML, expenses, expense/product/fixed-asset/bank-account GL/recurring-invoice account types, commercial history, inventory, banking, tax, cost allocations, payment bank-account default-currency checks, bank-transaction source-account omitted-currency and description-source checks, payment allocation dates against imported invoice issue dates, journal-line references, totals, percentages, and amount/percentage agreement, and fixed assets |

### Estonian Compliance

| Feature               | Description                                                                 |
| --------------------- | --------------------------------------------------------------------------- |
| **KMD Declaration**   | VAT declaration generation with export for manual filing                    |
| **EU VAT OSS Report** | Quarterly destination-country and VAT-rate report for non-Estonian EU sales |
| **TSD Declaration**   | Payroll tax declaration with XML/CSV export                                 |
| **e-MTA Export**      | XML export for manual upload to the Estonian Tax Board                      |
| **Estonian Defaults** | Pre-configured for Estonian accounting standards                            |

### Plugin Marketplace

| Feature               | Description                                                                                                   |
| --------------------- | ------------------------------------------------------------------------------------------------------------- |
| **Plugin Registries** | Add custom plugin marketplaces (GitHub/GitLab)                                                                |
| **Permission System** | Fine-grained permissions with risk levels                                                                     |
| **Event Hooks**       | 27+ events for plugin integration                                                                             |
| **Outbound Webhooks** | Tenant webhook endpoints with event subscriptions, HMAC signatures, test delivery, and delivery audit history |
| **UI Slots**          | Extend dashboard, invoices, and more with safe manifest-declared cards, links, and actions                    |
| **Two-Level Control** | Instance-wide install, per-tenant enable                                                                      |

> 📖 See [Plugin Documentation](docs/PLUGINS.md) for development guide

---

## 🛠 Technology Stack

| Layer         | Technology                                                              |
| ------------- | ----------------------------------------------------------------------- |
| **Backend**   | Go 1.26+, Chi router, GORM-backed repositories, pgx/v5 pools            |
| **Frontend**  | SvelteKit 2, Svelte 5, Vite 7, TypeScript                               |
| **i18n**      | Paraglide-JS (compile-time translations)                                |
| **Database**  | PostgreSQL 16+                                                          |
| **Auth**      | JWT access/refresh tokens plus tenant-scoped API tokens                 |
| **API Docs**  | Swagger/OpenAPI                                                         |
| **Testing**   | Go unit tests, backend integration tests, Vitest, Playwright demo suite |
| **CI/CD**     | GitHub Actions, Codecov                                                 |
| **Container** | Docker, Docker Compose                                                  |

---

## 🚀 Quick Start

### Docker (Recommended)

```bash
# Clone and start
git clone https://github.com/HMB-research/open-accounting.git
cd open-accounting
docker-compose up -d

# Run migrations
docker-compose run --rm migrate

# Access the app
# API: http://localhost:8080
# Frontend: http://localhost:5173
# Swagger: http://localhost:8080/swagger/
```

### Local Development

```bash
# Prerequisites: Go 1.26+, Node.js 22+, PostgreSQL 16+

# Start database
docker-compose up -d db

# Set environment
export DATABASE_URL="postgres://openaccounting:openaccounting@localhost:5432/openaccounting?sslmode=disable"

# Run migrations
go run ./cmd/migrate -db "$DATABASE_URL" -path migrations -direction up

# Start API (terminal 1)
go run ./cmd/api

# Start frontend (terminal 2)
cd frontend && bun install && bun run dev
```

### CLI bootstrap

```bash
go run ./cmd/oa auth init \
  --base-url http://localhost:8080 \
  --email you@example.com \
  --password 'your-password'

go run ./cmd/oa accounts list
go run ./cmd/oa contacts import --file ./contacts.csv
go run ./cmd/oa employees import --file ./employees.csv
go run ./cmd/oa payments import --file ./payments.csv
go run ./cmd/oa recurring-invoices import --file ./recurring-invoices.csv
go run ./cmd/oa payroll import-history --file ./payroll-history.csv
go run ./cmd/oa payroll import-leave-balances --file ./leave-balances.csv
go run ./cmd/oa assets import --file ./assets.csv
go run ./cmd/oa cost-centers import --file ./cost-centers.csv
go run ./cmd/oa inventory categories import --file ./categories.csv
go run ./cmd/oa inventory warehouses import --file ./warehouses.csv
go run ./cmd/oa inventory products import --file ./products.csv
go run ./cmd/oa inventory stock import --file ./stock.csv
go run ./cmd/oa expenses create --merchant "Office Store" --expense-date 2026-05-30 --expense-account-id <expense-account-id> --payment-account-id <cash-account-id> --amount 120.50
go run ./cmd/oa documents upload --entity-type expense --entity-id <expense-id> --file ./receipt.pdf --document-type receipt
go run ./cmd/oa documents upload --entity-type bank_transaction --entity-id <transaction-id> --file ./evidence.pdf --document-type reconciliation_evidence
go run ./cmd/oa documents evidence-policy --entity-type bank_transaction --entity-id <transaction-id> --document-type reconciliation_evidence --require-approved
go run ./cmd/oa journal import-opening-balances --file ./opening-balances.csv --entry-date 2026-01-01
```

More examples are in [docs/CLI.md](docs/CLI.md).

---

## 📁 Project Structure

```
open-accounting/
├── cmd/
│   ├── api/              # HTTP API server (main application)
│   ├── migrate/          # Database migration CLI tool
│   └── oa/               # Operator CLI using tenant-scoped API tokens
│
├── internal/
│   ├── accounting/       # Core: accounts, journal entries, reports
│   ├── analytics/        # Dashboard metrics and reporting
│   ├── auth/             # JWT authentication, RBAC, rate limiting
│   ├── banking/          # Bank accounts, transactions, reconciliation
│   ├── contacts/         # Customer and supplier management
│   ├── email/            # Email notifications and templates
│   ├── invoicing/        # Sales and purchase invoices
│   ├── payments/         # Payment recording and allocation
│   ├── payroll/          # Estonian payroll with TSD declarations
│   ├── pdf/              # PDF generation for invoices
│   ├── plugin/           # Plugin marketplace system
│   ├── recurring/        # Recurring invoice automation
│   ├── tax/              # Estonian KMD/VAT compliance
│   └── tenant/           # Multi-tenant management, users, invitations
│
├── migrations/           # SQL database migrations
├── frontend/             # SvelteKit web application
├── docs/                 # Documentation (API, Architecture, Deployment)
└── deploy/               # Deployment configurations
```

---

## 📚 Documentation

| Document                                                              | Description                                                                 |
| --------------------------------------------------------------------- | --------------------------------------------------------------------------- |
| [API Reference](docs/API.md)                                          | Complete REST API documentation with examples                               |
| [Architecture](docs/ARCHITECTURE.md)                                  | System design, multi-tenancy, authentication flow                           |
| [CLI Guide](docs/CLI.md)                                              | API-token bootstrap, token management, and import examples for the `oa` CLI |
| [Current Product Limits](docs/CURRENT_PRODUCT_LIMITS.md)               | Concise current caps and gaps before full product parity                    |
| [Use Case Coverage Matrix](docs/USE_CASE_COVERAGE.md)                 | Current use-case status, evidence gates, and remaining gaps                 |
| [Deployment](docs/DEPLOYMENT.md)                                      | Production deployment guide                                                 |
| [EMTA Integration](docs/EMTA_INTEGRATION.md)                          | Estonian Tax Board integration guide                                        |
| [Plugins](docs/PLUGINS.md)                                            | Plugin development and marketplace guide                                    |
| [E2E Testing](docs/demo-e2e-testing.md)                               | End-to-end testing architecture                                             |
| [Swagger UI](/swagger/)                                               | Interactive API explorer (when server is running)                           |

---

## ⚙️ Configuration

| Variable                           | Description                                                              | Default                                      |
| ---------------------------------- | ------------------------------------------------------------------------ | -------------------------------------------- |
| `DATABASE_URL`                     | PostgreSQL connection string                                             | _Required_                                   |
| `PORT`                             | API server port                                                          | `8080`                                       |
| `APP_ENV`                          | Set to `production` to enable production config validation               | unset                                        |
| `JWT_SECRET`                       | JWT signing key, min 32 chars when `APP_ENV=production`                  | development-only fallback outside production |
| `ALLOWED_ORIGINS`                  | CORS allowed origins; required in production                             | local dev origins outside production         |
| `PASSWORD_RESET_BASE_URL`          | Frontend reset URL used in password reset emails                         | unset                                        |
| `PASSWORD_RESET_SMTP_*`            | Global SMTP settings for password reset email delivery                   | unset                                        |
| `PASSWORD_RESET_EXPOSE_TOKEN`      | Return reset tokens in API responses for local/dev only                  | `false`                                      |
| `SCHEDULER_ENABLED`                | Enable recurring invoice, recurring journal, invoice reminder, and document retention reminder scheduler jobs | `true`                                       |
| `RECURRING_INVOICE_SCHEDULE`       | Cron schedule for recurring invoice generation                           | `0 6 * * *`                                  |
| `RECURRING_JOURNAL_ENTRY_SCHEDULE` | Cron schedule for recurring journal entry generation                     | `15 6 * * *`                                 |
| `DOCUMENT_RETENTION_REMINDER_SCHEDULE` | Cron schedule for document retention reminder delivery               | `30 9 * * *`                                 |
| `DOCUMENT_RETENTION_REMINDER_HORIZON_DAYS` | Retention reminder lookahead horizon in days                    | `30`                                         |
| `DOCUMENT_RETENTION_REMINDER_INCLUDE_MISSING` | Include documents missing retention metadata in reminder digests | `true`                                       |
| `DOCUMENT_RETENTION_REMINDER_MAX_ATTEMPTS` | Retry failed retention reminder delivery attempts before reporting failure | `3`                                          |
| `DOCUMENT_RETENTION_REMINDER_ESCALATE_AFTER_ATTEMPTS` | Mark failed retention reminder delivery as escalated after this many attempts | `3`                                          |

---

## 🗺 Roadmap

### Working in repo

- Feature presence only; not a claim of production parity or operational maturity.

- [x] Double-entry bookkeeping with journal entries
- [x] Multi-tenant architecture with schema isolation
- [x] User authentication and RBAC
- [x] Invoicing with PDF generation
- [x] Payment recording and allocation
- [x] Bank transaction import, reconciliation, and accountant remediation actions
- [x] Estonian KMD/VAT compliance
- [x] User invitation system
- [x] Dashboard analytics with charts
- [x] Email notifications
- [x] Webhook notifications
- [x] Recurring invoice automation
- [x] Balance sheet and income statement reports
- [x] Payroll module with Estonian TSD declarations
- [x] API rate limiting
- [x] Plugin marketplace system
- [x] Internationalization (English/Estonian) with Paraglide-JS
- [x] Mobile-responsive frontend with touch-friendly UI
- [x] Report exports (Excel, CSV, PDF)
- [x] Quotes with quote-to-order conversion
- [x] Order management with delivered order-to-invoice conversion
- [x] Fixed assets with depreciation tracking
- [x] Receipt-backed expense tracking with remediation actions, approval, and posting
- [x] Tenant-scoped API token auth and Go CLI
- [x] Access/refresh JWT purpose separation
- [x] Revocable, single-use refresh token sessions
- [x] CLI/API refresh-session listing and revocation
- [x] One-time password reset flow with expiring tokens, request throttling, and refresh-session revocation
- [x] Auth security audit events for login success/failure, logout, password, session, API-token, and tenant-access actions with credential-aware failed-login throttling
- [x] Tenant audit events for organization settings changes
- [x] Tenant admin controls for member refresh-session inspection and revocation
- [x] Tenant admin controls for member API-token inspection and revocation
- [x] Tenant admin security-event visibility for member auth activity
- [x] Tenant admin suspension/restoration of member tenant access with session revocation and audit events
- [x] Tenant-scoped API tokens blocked from creating new tenant organizations outside their tenant boundary
- [x] CSV import for chart of accounts with optional preserved UUIDs, contacts, employees, invoices, quotes, orders, and recurring invoice templates with contact identity lookup including VAT numbers, payments and expenses with contact identity lookup, bank accounts, bank transactions, cost centers, cost allocations, product categories/warehouses/products/stock, fixed assets with supplier identity lookup, payroll/TSD tax history, opening balances, and historical journals with optional preserved line IDs
- [x] Migration bundle validation for cutover imports with API and CLI coverage across e-invoice XML, expenses, commercial history, inventory, banking, payroll/TSD tax history, cost allocations, and fixed assets, including duplicate identifiers and history keys, account, contact, employee, payroll-history, commercial-document, inventory, fixed-asset, cost-center/allocation, expense, payment, bank-account, and bank-transaction row values, expense employee-ID UUID and currency code checks, payment currency code checks, expense, product, fixed-asset, bank-account GL, and recurring-invoice line account-type consistency, commercial-document and payment/expense contact identity references, order quote-contact consistency, payment bank-account default-currency consistency, bank-transaction source-account omitted-currency and description-source consistency, invoice paid-amount consistency, combined invoice paid/allocation totals, payment allocation totals, currencies, payment direction, payment invoice-contact consistency and payment date ordering against imported invoices and e-invoices, fixed-asset source-invoice purchase-type, supplier identity field, purchase-date, and amount-total consistency, stock-adjustment product stockability, recurring line account references, cost-allocation journal-line references plus amount and percentage total consistency and amount/percentage agreement, and grouped document/preserved-ID consistency
- [x] Migration execution plans with ordered API/CLI import steps, dependency hints, and missing-context markers for bank-transaction and opening-balance cutover imports
- [x] Guarded CLI and server-side migration execution that validates, plans, requires explicit confirmation, includes the provider execution CSV canonicalization stage, and runs ready cutover import steps through the existing tenant-scoped APIs and import services
- [x] Dashboard migration workbench for cutover bundle assembly, provider preset catalog discovery, validation, execution planning, saved dry runs, confirmed execution, saved-run monitoring with live event streams, progress/active-step/duration telemetry, saved-run event stream API/CLI access, resume-by-ID selection, and accountant-workspace assignment handoff into deep-linked saved runs
- [x] Tenant period lock on core write paths
- [x] Close/reopen workflow with audit trail in API and company settings
- [x] Fiscal-year close readiness and retained-earnings carry-forward workflow
- [x] Document attachments with review, retention dates, retention-year calculation, audited lifecycle states for replacement/archive/disposal decisions, retention review, reminder actions, evidence-policy approval assignments, scheduled retention reminder digests, configurable reminder retry/escalation controls, and approved-evidence workflow blockers with structured remediation for invoices, journal entries, payments, bank transactions, fixed assets, expenses, quotes, orders, year-end close packs, leave records, TSD declarations, and KMD declarations
- [x] TSD/KMD submission and acceptance evidence blockers requiring approved tax/support documents before declarations can be marked submitted or accepted
- [x] Document evidence policy evaluation through API and CLI
- [x] Optional approved evidence blockers for quote send and order confirmation
- [x] Purchase-invoice send/email, quote/order send/email/confirmation, and fixed-asset activation/disposal evidence enforcement with structured evidence-policy remediation in 409 responses
- [x] Reconciliation completion blocks bank transactions marked as requiring evidence until approved evidence is attached
- [x] Backup creation, offsite sync, health-check, restore-drill, CLI offsite/restore preflight parity, offline host preflight, and systemd schedule template scripts for self-hosted operations

### Still missing for reliable production use

The concise source of truth for current product caps is
[docs/CURRENT_PRODUCT_LIMITS.md](docs/CURRENT_PRODUCT_LIMITS.md). In summary:

- [ ] Remaining mutating migration cutover orchestration beyond the guarded CLI/server-side execution, resume-snapshot, saved-run list/get/event-stream paths, provider preset catalog discovery for Merit, SmartAccounts, and Directo, dashboard workbench live streaming, saved-run progress/duration telemetry, accountant-workspace launch handoff, saved-bundle workspace execution, and KMD VAT-history execution dependency plus same-bundle VAT support reconciliation, especially further provider-specific cutover depth, cross-file validation outside payroll/TSD history and KMD VAT support, and deeper dashboard-side cutover controls
- [ ] Remaining payroll/document/evidence-policy edges and deeper accountant-workspace execution beyond the current assignment queue, direct bank follow-up, reminder actions, pending-document assignment approval, document-retention date setting, evidence/missing-document upload and replacement including evidence-policy violation upload, unapproved-evidence approval, payroll-run calculation/recalculation, payment-date setting and approval, payroll TSD assignment generation, payroll paid-run TSD follow-up generation, declared payroll archive XML export, TSD declaration XML export and acceptance marking, KMD assignment regeneration/export/acceptance marking, KMD INF/EU VAT OSS report generation, expense submit/approve/post assignment completion, fiscal-year close/carry-forward assignment completion, and migration saved-run handoff, especially remaining document evidence-policy follow-up
- [ ] Automated document policy enforcement in remaining workflow blockers
- [ ] Remaining auth hardening beyond the current API/CLI/settings controls for tenant member status, sessions, API tokens, tenant-creation boundaries, audit visibility, failed-login audit, and credential-aware failed-login throttling
- [ ] Broader plugin production hardening beyond the current loopback HTTP runtime, supervised package runtime startup/proxy/shutdown/status/manual restart/automatic crash restart, allowlisted package runtime process environment, and safe operator-bundled frontend component registry, especially OS-level sandboxing and broader resource isolation
- [ ] Direct e-invoice operator send/receive, direct bank feeds, SEPA initiation, and automatic e-MTA submission

---

## 🤝 Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

```bash
# Development workflow
git checkout -b feature/your-feature
make test                    # Run tests
make test-backend-coverage   # Run backend tests and enforce CLI coverage
make lint                    # Check code style
git commit -m "feat: your feature"
git push origin feature/your-feature
# Open a Pull Request
```

### Contributors

<a href="https://github.com/HMB-research/open-accounting/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=HMB-research/open-accounting" />
</a>

---

## 🏆 Supporters

A huge thank you to our supporters who help make this project possible!

### Sponsors

<!-- sponsors -->

_Become the first sponsor! [Support us on GitHub Sponsors](https://github.com/sponsors/HMB-research) or [Ko-fi](https://ko-fi.com/tsopic)_

<!-- sponsors -->

---

## 📄 License

MIT License — see [LICENSE](LICENSE) for details.

---

## 💖 Support

If you find this project useful, consider supporting its development:

[![GitHub Sponsors](https://img.shields.io/badge/Sponsor-GitHub-ea4aaa?logo=github)](https://github.com/sponsors/HMB-research)
[![Ko-fi](https://img.shields.io/badge/Support-Ko--fi-ff5f5f?logo=ko-fi)](https://ko-fi.com/tsopic)
