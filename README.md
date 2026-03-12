# Open Accounting

🇪🇪 **Made in Estonia** | Open-source accounting software for modern businesses

[![CI](https://github.com/HMB-research/open-accounting/actions/workflows/ci.yml/badge.svg)](https://github.com/HMB-research/open-accounting/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/HMB-research/open-accounting/branch/main/graph/badge.svg)](https://codecov.io/gh/HMB-research/open-accounting)
[![Go Report Card](https://goreportcard.com/badge/github.com/HMB-research/open-accounting)](https://goreportcard.com/report/github.com/HMB-research/open-accounting)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev/)

[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16+-4169E1?logo=postgresql&logoColor=white)](https://postgresql.org/)
[![SvelteKit](https://img.shields.io/badge/SvelteKit-2-FF3E00?logo=svelte&logoColor=white)](https://kit.svelte.dev/)
[![Vite](https://img.shields.io/badge/Vite-7-646CFF?logo=vite&logoColor=white)](https://vitejs.dev/)
[![TypeScript](https://img.shields.io/badge/TypeScript-5-3178C6?logo=typescript&logoColor=white)](https://typescriptlang.org/)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?logo=docker&logoColor=white)](https://docker.com/)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

> **⚠️ Development Status**
> This project is under active development and not yet production-ready. APIs may change, and features may be incomplete. Contributions and feedback welcome!
>
> Verified locally on 2026-03-12:
> `go test ./...`, `go test -count=1 -race -tags=integration $(go list ./... | grep -v /testutil)`, `cd frontend && bun run test`, and `cd frontend && bun run test:e2e:smoke` pass.
> Production hardening, broader migration imports, fiscal-year close/carry-forward work, deeper accountant exception actions, and broader document retention/reconciliation workflows are still in progress.

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

| Credential | Value |
|------------|-------|
| **Email** | `demo1@example.com` |
| **Password** | `demo12345` |

---

## What is Open Accounting?

Open Accounting is a **self-hosted, multi-tenant accounting platform** focused today on **Estonian SMB and accountant workflows**. The current wedge is accounting, invoicing, payroll, bank import/reconciliation, and KMD/TSD export for self-hosted teams that want source access and tenant isolation.

It is not yet a full SmartAccounts/Merit replacement or a production-hardened embedded accounting platform. Built with modern technologies and focused on **Estonian/EU compliance**, it provides:

- **True Double-Entry Bookkeeping** — Immutable journal entries with full audit trail
- **Multi-Company Support** — One installation serves multiple businesses with complete data isolation
- **Role-Based Access** — Owner, Admin, Accountant, and Viewer roles with granular permissions
- **Accountant Review Queue** — Dashboard review surface for overdue invoices, unmatched bank transactions, close status, and recent journal activity, with a cross-tenant portfolio rollup for accountant users
- **Estonian Tax Compliance** — KMD (VAT) declarations with e-MTA XML export
- **Modern Stack** — Go backend, SvelteKit frontend, PostgreSQL database

---

## ✨ Features

> Status note: features listed below exist in the repository. That does not mean each one is production-hardened, accountant-grade, or at full parity with proprietary incumbents.

### Core Accounting
| Feature | Description |
|---------|-------------|
| **Chart of Accounts** | Hierarchical 5-type account structure (Asset, Liability, Equity, Revenue, Expense) |
| **Journal Entries** | Draft → Posted → Void workflow with reversal entries |
| **Multi-Currency** | Support for multiple currencies with exchange rate tracking |
| **Trial Balance** | Real-time balance reports as of any date |
| **Balance Sheet** | Assets, liabilities, and equity statement |
| **Income Statement** | Revenue and expense summary (P&L) |
| **Report Exports** | Export to Excel, CSV, or PDF formats |
| **VAT Tracking** | Date-aware VAT rates for proper EU compliance |

### Business Operations
| Feature | Description |
|---------|-------------|
| **Invoicing** | Sales and purchase invoices with line items and VAT |
| **Quotes** | Sales quotes with draft/sent/accepted workflow, conversion to orders |
| **Orders** | Order management with quote linking and status tracking |
| **Contacts** | Customer and supplier management |
| **Payments** | Payment recording with invoice allocation |
| **PDF Generation** | Professional invoice PDFs with customizable branding |
| **Recurring Invoices** | Automated invoice generation on schedule |

### Fixed Assets
| Feature | Description |
|---------|-------------|
| **Asset Tracking** | Register and track fixed assets with serial numbers and locations |
| **Asset Categories** | IT Equipment, Office Furniture, Vehicles, Software with depreciation settings |
| **Depreciation** | Straight-line and declining balance methods with configurable useful life |
| **Asset Lifecycle** | Draft → Active → Disposed/Sold/Scrapped status workflow |
| **Depreciation Entries** | Automatic depreciation calculations with audit trail |

### Banking & Reconciliation
| Feature | Description |
|---------|-------------|
| **Bank Accounts** | Track multiple bank accounts per company |
| **Transaction Import** | CSV import for bank statements |
| **Auto-Matching** | Intelligent matching of transactions to payments |
| **Reconciliation** | Full bank reconciliation workflow |

### Multi-Tenant & Security
| Feature | Description |
|---------|-------------|
| **Tenant Isolation** | Schema-per-tenant for complete data separation |
| **User Management** | Invite users, assign roles, manage permissions |
| **JWT and API token auth** | JWT access/refresh tokens plus tenant-scoped API tokens for automation |
| **RBAC** | Role-based access control with permission checks |
| **API Rate Limiting** | Token bucket rate limiting with configurable thresholds |

### Payroll (Estonian)
| Feature | Description |
|---------|-------------|
| **Employee Management** | Full employee lifecycle with personal codes |
| **Estonian Tax Calculations** | Income tax, social tax, unemployment insurance |
| **Funded Pension (II Pillar)** | Configurable pension contribution rates |
| **Payroll Runs** | Monthly payroll with draft → approved → paid workflow |
| **Payslips** | Detailed breakdown of earnings and deductions |
| **TSD Declaration** | Annex 1 generation with XML/CSV export for e-MTA |

### Estonian Compliance
| Feature | Description |
|---------|-------------|
| **KMD Declaration** | VAT declaration generation with export for manual filing |
| **TSD Declaration** | Payroll tax declaration with XML/CSV export |
| **e-MTA Export** | XML export for manual upload to the Estonian Tax Board |
| **Estonian Defaults** | Pre-configured for Estonian accounting standards |

### Plugin Marketplace
| Feature | Description |
|---------|-------------|
| **Plugin Registries** | Add custom plugin marketplaces (GitHub/GitLab) |
| **Permission System** | Fine-grained permissions with risk levels |
| **Event Hooks** | 27+ events for plugin integration |
| **UI Slots** | Extend dashboard, invoices, and more |
| **Two-Level Control** | Instance-wide install, per-tenant enable |

> 📖 See [Plugin Documentation](docs/PLUGINS.md) for development guide

---

## 🛠 Technology Stack

| Layer | Technology |
|-------|------------|
| **Backend** | Go 1.24+, Chi router, pgx/v5, sqlc (shared tables) |
| **Frontend** | SvelteKit 2, Svelte 5, Vite 7, TypeScript |
| **i18n** | Paraglide-JS (compile-time translations) |
| **Database** | PostgreSQL 16+ |
| **Auth** | JWT access/refresh tokens plus tenant-scoped API tokens |
| **API Docs** | Swagger/OpenAPI |
| **Testing** | Go unit tests, backend integration tests, Vitest, Playwright demo suite |
| **CI/CD** | GitHub Actions, Codecov |
| **Container** | Docker, Docker Compose |

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
# Prerequisites: Go 1.24+, Node.js 22+, PostgreSQL 16+

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

| Document | Description |
|----------|-------------|
| [API Reference](docs/API.md) | Complete REST API documentation with examples |
| [Architecture](docs/ARCHITECTURE.md) | System design, multi-tenancy, authentication flow |
| [CLI Guide](docs/CLI.md) | API-token bootstrap, token management, and import examples for the `oa` CLI |
| [Deployment](docs/DEPLOYMENT.md) | Production deployment guide |
| [EMTA Integration](docs/EMTA_INTEGRATION.md) | Estonian Tax Board integration guide |
| [Plugins](docs/PLUGINS.md) | Plugin development and marketplace guide |
| [E2E Testing](docs/plans/2026-01-09-e2e-test-consolidation-design.md) | End-to-end testing architecture |
| [Swagger UI](/swagger/) | Interactive API explorer (when server is running) |

---

## ⚙️ Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_URL` | PostgreSQL connection string | *Required* |
| `PORT` | API server port | `8080` |
| `JWT_SECRET` | JWT signing key | *Required in production* |
| `ALLOWED_ORIGINS` | CORS allowed origins | `localhost:5173,localhost:3000` |

---

## 🗺 Roadmap

### Working in repo
- Feature presence only; not a claim of production parity or operational maturity.

- [x] Double-entry bookkeeping with journal entries
- [x] Multi-tenant architecture with schema isolation
- [x] User authentication and RBAC
- [x] Invoicing with PDF generation
- [x] Payment recording and allocation
- [x] Bank transaction import and reconciliation
- [x] Estonian KMD/VAT compliance
- [x] User invitation system
- [x] Dashboard analytics with charts
- [x] Email notifications
- [x] Recurring invoice automation
- [x] Balance sheet and income statement reports
- [x] Payroll module with Estonian TSD declarations
- [x] API rate limiting
- [x] Plugin marketplace system
- [x] Internationalization (English/Estonian) with Paraglide-JS
- [x] Mobile-responsive frontend with touch-friendly UI
- [x] Report exports (Excel, CSV, PDF)
- [x] Quotes with quote-to-order conversion
- [x] Order management
- [x] Fixed assets with depreciation tracking
- [x] Tenant-scoped API token auth and Go CLI
- [x] CSV import for chart of accounts, contacts, invoices, and opening balances
- [x] Tenant period lock on core write paths
- [x] Close/reopen workflow with audit trail in API and company settings
- [x] Document attachments for invoices, journal entries, and payments

### Still missing for reliable production use
- [ ] Invoice, employee, and external migration imports
- [ ] Fiscal year close checklist and carry-forward workflow
- [ ] Broader document retention, reconciliation evidence, and approval workflows
- [ ] Backup/restore verification and stronger auth/session controls
- [ ] E-invoice, direct bank feeds, SEPA initiation, and automatic e-MTA submission

---

## 🤝 Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

```bash
# Development workflow
git checkout -b feature/your-feature
make test                    # Run tests
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
*Become the first sponsor! [Support us on GitHub Sponsors](https://github.com/sponsors/HMB-research) or [Ko-fi](https://ko-fi.com/tsopic)*
<!-- sponsors -->

---

## 📄 License

MIT License — see [LICENSE](LICENSE) for details.

---

## 💖 Support

If you find this project useful, consider supporting its development:

[![GitHub Sponsors](https://img.shields.io/badge/Sponsor-GitHub-ea4aaa?logo=github)](https://github.com/sponsors/HMB-research)
[![Ko-fi](https://img.shields.io/badge/Support-Ko--fi-ff5f5f?logo=ko-fi)](https://ko-fi.com/tsopic)
