# Open Accounting

[![CI](https://github.com/HMB-research/open-accounting/actions/workflows/ci.yml/badge.svg)](https://github.com/HMB-research/open-accounting/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/HMB-research/open-accounting/branch/main/graph/badge.svg)](https://codecov.io/gh/HMB-research/open-accounting)
[![Go Report Card](https://goreportcard.com/badge/github.com/HMB-research/open-accounting)](https://goreportcard.com/report/github.com/HMB-research/open-accounting)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![GitHub stars](https://img.shields.io/github/stars/HMB-research/open-accounting?style=social)](https://github.com/HMB-research/open-accounting/stargazers)
[![GitHub forks](https://img.shields.io/github/forks/HMB-research/open-accounting?style=social)](https://github.com/HMB-research/open-accounting/network/members)
[![GitHub issues](https://img.shields.io/github/issues/HMB-research/open-accounting)](https://github.com/HMB-research/open-accounting/issues)
[![GitHub pull requests](https://img.shields.io/github/issues-pr/HMB-research/open-accounting)](https://github.com/HMB-research/open-accounting/pulls)

> **Warning**
> This project is currently under active development and is not yet ready for production use. APIs may change without notice, and features may be incomplete or unstable. We welcome contributions and feedback!

Open-source accounting software with double-entry bookkeeping, invoicing, inventory management, and payroll support.

## Features

- **Double-Entry Bookkeeping**: Complete general ledger with immutable journal entries
- **Multi-Tenant**: Schema-per-tenant isolation for secure multi-company support
- **Multi-Currency**: Support for multiple currencies with exchange rate tracking
- **Chart of Accounts**: Hierarchical account structure with 5 account types
- **Financial Reports**: Trial balance, balance sheet, income statement
- **VAT/Tax Support**: Date-aware VAT rates for EU compliance
- **Estonian Tax Compliance**: KMD (VAT declaration) generation with e-MTA XML export
- **REST API**: Full-featured JSON API for integration

## Quick Start

### One-Line Install

```bash
curl -fsSL https://raw.githubusercontent.com/HMB-research/open-accounting/main/install.sh | bash
```

### Manual Setup

1. Clone the repository:
```bash
git clone https://github.com/HMB-research/open-accounting.git
cd open-accounting
```

2. Start with Docker Compose:
```bash
docker-compose up -d
```

3. Run database migrations:
```bash
docker-compose run --rm migrate
```

4. Access the API at http://localhost:8080

## Development

### Prerequisites

- Go 1.22+
- Node.js 22+
- PostgreSQL 16+
- Docker & Docker Compose

### Local Development

```bash
# Start database
docker-compose up -d db

# Run migrations
export DATABASE_URL="postgres://openaccounting:openaccounting@localhost:5432/openaccounting?sslmode=disable"
go run ./cmd/migrate -db "$DATABASE_URL" -path migrations -direction up

# Run API server
go run ./cmd/api

# In another terminal, start frontend
cd frontend
npm install
npm run dev
```

### Available Make Commands

```bash
make help          # Show all available commands
make build         # Build all binaries
make test          # Run tests
make docker-up     # Start Docker containers
make dev           # Start development environment
make migrate-up    # Run database migrations
```

## API Endpoints

### Authentication
- `POST /api/v1/auth/register` - Register new user
- `POST /api/v1/auth/login` - Login and get tokens
- `POST /api/v1/auth/refresh` - Refresh access token

### User
- `GET /api/v1/me` - Get current user
- `GET /api/v1/me/tenants` - List user's organizations

### Tenants
- `POST /api/v1/tenants` - Create organization
- `GET /api/v1/tenants/{id}` - Get organization

### Accounts
- `GET /api/v1/tenants/{id}/accounts` - List accounts
- `POST /api/v1/tenants/{id}/accounts` - Create account
- `GET /api/v1/tenants/{id}/accounts/{accountId}` - Get account

### Journal Entries
- `GET /api/v1/tenants/{id}/journal-entries/{entryId}` - Get entry
- `POST /api/v1/tenants/{id}/journal-entries` - Create entry
- `POST /api/v1/tenants/{id}/journal-entries/{entryId}/post` - Post entry
- `POST /api/v1/tenants/{id}/journal-entries/{entryId}/void` - Void entry

### Contacts
- `GET /api/v1/tenants/{id}/contacts` - List contacts
- `POST /api/v1/tenants/{id}/contacts` - Create contact
- `GET /api/v1/tenants/{id}/contacts/{contactId}` - Get contact
- `PUT /api/v1/tenants/{id}/contacts/{contactId}` - Update contact
- `DELETE /api/v1/tenants/{id}/contacts/{contactId}` - Delete contact

### Invoices
- `GET /api/v1/tenants/{id}/invoices` - List invoices
- `POST /api/v1/tenants/{id}/invoices` - Create invoice
- `GET /api/v1/tenants/{id}/invoices/{invoiceId}` - Get invoice
- `POST /api/v1/tenants/{id}/invoices/{invoiceId}/send` - Send invoice
- `POST /api/v1/tenants/{id}/invoices/{invoiceId}/void` - Void invoice

### Payments
- `GET /api/v1/tenants/{id}/payments` - List payments
- `POST /api/v1/tenants/{id}/payments` - Create payment
- `GET /api/v1/tenants/{id}/payments/{paymentId}` - Get payment
- `POST /api/v1/tenants/{id}/payments/{paymentId}/allocate` - Allocate to invoice
- `GET /api/v1/tenants/{id}/payments/unallocated` - Get unallocated payments

### Reports
- `GET /api/v1/tenants/{id}/reports/trial-balance` - Trial balance
- `GET /api/v1/tenants/{id}/reports/account-balance/{accountId}` - Account balance

### Tax (Estonian KMD)
- `POST /api/v1/tenants/{id}/tax/kmd` - Generate KMD declaration
- `GET /api/v1/tenants/{id}/tax/kmd` - List KMD declarations
- `GET /api/v1/tenants/{id}/tax/kmd/{year}/{month}/xml` - Export KMD to e-MTA XML

### API Documentation
- `GET /swagger/` - Interactive Swagger UI
- `GET /swagger/doc.json` - OpenAPI specification

## Architecture

```
open-accounting/
├── cmd/
│   ├── api/          # HTTP API server
│   └── migrate/      # Database migration tool
├── internal/
│   ├── accounting/   # Core accounting (accounts, journal entries)
│   ├── auth/         # JWT authentication
│   ├── contacts/     # Customer/supplier management
│   ├── invoicing/    # Sales and purchase invoices
│   ├── payments/     # Payment recording and allocation
│   ├── tax/          # Estonian tax compliance (KMD)
│   └── tenant/       # Multi-tenant management
├── migrations/       # SQL migrations
├── frontend/         # SvelteKit frontend
├── docs/             # OpenAPI/Swagger documentation
└── deploy/           # Deployment configurations
```

## Configuration

Environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_URL` | PostgreSQL connection string | Required |
| `PORT` | API server port | 8080 |
| `JWT_SECRET` | JWT signing key | Required for production |
| `ALLOWED_ORIGINS` | CORS allowed origins | localhost:5173,localhost:3000 |

## Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

MIT License - see [LICENSE](LICENSE) file for details.
