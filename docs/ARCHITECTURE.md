# Architecture Overview

This document describes the high-level architecture of Open Accounting.

## System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Clients                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │   Web App    │  │  Mobile App  │  │  API Client  │          │
│  │  (SvelteKit) │  │   (Future)   │  │  (REST/JSON) │          │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘          │
└─────────┼─────────────────┼─────────────────┼───────────────────┘
          │                 │                 │
          └────────────────┬┴─────────────────┘
                           │
                    ┌──────▼──────┐
                    │   Nginx /   │
                    │ Load Balancer│
                    └──────┬──────┘
                           │
┌──────────────────────────┼──────────────────────────────────────┐
│                    API Server                                    │
│  ┌───────────────────────▼───────────────────────────────────┐  │
│  │                    Chi Router                              │  │
│  │  ┌─────────────────────────────────────────────────────┐  │  │
│  │  │              Middleware Chain                        │  │  │
│  │  │  CORS → Logger → Auth → Tenant Context → Handler    │  │  │
│  │  └─────────────────────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                    Service Layer                             ││
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐           ││
│  │  │Accounting│ │Invoicing│ │Payments │ │ Banking │           ││
│  │  └────┬────┘ └────┬────┘ └────┬────┘ └────┬────┘           ││
│  │       └───────────┴──────────┬┴───────────┘                 ││
│  │                              │                               ││
│  │  ┌─────────┐ ┌─────────┐ ┌──▼──────┐ ┌─────────┐           ││
│  │  │ Tenant  │ │  Auth   │ │Contacts │ │   Tax   │           ││
│  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘           ││
│  └─────────────────────────────────────────────────────────────┘│
└──────────────────────────────┬──────────────────────────────────┘
                               │
┌──────────────────────────────▼──────────────────────────────────┐
│                      PostgreSQL Database                         │
│  ┌────────────────┐  ┌────────────────┐  ┌────────────────┐    │
│  │  public schema │  │ tenant_acme    │  │ tenant_beta    │    │
│  │  ─────────────  │  │  ───────────   │  │  ───────────   │    │
│  │  • users        │  │  • accounts    │  │  • accounts    │    │
│  │  • tenants      │  │  • entries     │  │  • entries     │    │
│  │  • tenant_users │  │  • invoices    │  │  • invoices    │    │
│  │  • invitations  │  │  • payments    │  │  • payments    │    │
│  └────────────────┘  └────────────────┘  └────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
```

## Multi-Tenant Architecture

Open Accounting uses a **schema-per-tenant** isolation model:

### Public Schema
Contains shared tables:
- `users` - All system users
- `tenants` - Organization registry
- `tenant_users` - User-tenant memberships with roles
- `user_invitations` - Pending invitations

### Tenant Schemas
Each tenant gets a dedicated PostgreSQL schema (e.g., `tenant_acme`) containing:
- `accounts` - Chart of accounts
- `journal_entries` / `journal_entry_lines` - Double-entry transactions
- `contacts` - Customers and suppliers
- `invoices` / `invoice_lines` - Sales and purchase invoices
- `payments` / `payment_allocations` - Payment tracking
- `bank_accounts` / `bank_transactions` - Banking data

### Benefits
1. **Complete Data Isolation** - No risk of data leaks between tenants
2. **Easy Backup/Restore** - Per-tenant database operations
3. **Performance** - Smaller table sizes, better query performance
4. **Compliance** - Simplified data residency requirements

## Authentication Flow

```
┌──────────┐     ┌──────────┐     ┌──────────┐
│  Client  │────▶│   API    │────▶│ Database │
└──────────┘     └──────────┘     └──────────┘
     │                │
     │  1. Login      │
     │  (email/pass)  │
     │───────────────▶│
     │                │  2. Validate credentials
     │                │  3. Generate JWT tokens
     │◀───────────────│
     │  access_token  │
     │  refresh_token │
     │                │
     │  4. API call   │
     │  + Bearer token│
     │───────────────▶│
     │                │  5. Validate token
     │                │  6. Extract claims
     │                │  7. Check tenant access
     │◀───────────────│
     │   Response     │
```

### JWT Claims
```json
{
  "user_id": "uuid",
  "email": "user@example.com",
  "tenant_id": "uuid",    // Current tenant context
  "role": "accountant",   // Role in current tenant
  "exp": 1234567890
}
```

## Role-Based Access Control

| Role | Description | Permissions |
|------|-------------|-------------|
| **Owner** | Organization creator | Full access, cannot be removed |
| **Admin** | Administrator | Manage users, settings, full accounting |
| **Accountant** | Accounting staff | Full accounting, no user management |
| **Viewer** | Read-only access | View reports only |

### Permission Matrix

| Permission | Owner | Admin | Accountant | Viewer |
|------------|-------|-------|------------|--------|
| Manage Users | ✅ | ✅ | ❌ | ❌ |
| Manage Settings | ✅ | ✅ | ❌ | ❌ |
| Manage Accounts | ✅ | ✅ | ✅ | ❌ |
| Create Entries | ✅ | ✅ | ✅ | ❌ |
| View Reports | ✅ | ✅ | ✅ | ✅ |
| Manage Invoices | ✅ | ✅ | ✅ | ❌ |
| Manage Banking | ✅ | ✅ | ✅ | ❌ |
| Export Data | ✅ | ✅ | ✅ | ❌ |

## Service Dependencies

```
                    ┌──────────────┐
                    │   Handlers   │
                    └──────┬───────┘
                           │
        ┌──────────────────┼──────────────────┐
        │                  │                  │
        ▼                  ▼                  ▼
┌───────────────┐  ┌───────────────┐  ┌───────────────┐
│   Invoicing   │  │   Payments    │  │   Banking     │
└───────┬───────┘  └───────┬───────┘  └───────┬───────┘
        │                  │                  │
        └────────┬─────────┴──────────────────┘
                 │
                 ▼
        ┌───────────────┐
        │  Accounting   │
        │  (Core)       │
        └───────┬───────┘
                │
                ▼
        ┌───────────────┐
        │  PostgreSQL   │
        └───────────────┘
```

## Database Migrations

Migrations are managed using golang-migrate:

```bash
# Run all pending migrations
go run ./cmd/migrate -db "$DATABASE_URL" -path migrations -direction up

# Rollback last migration
go run ./cmd/migrate -db "$DATABASE_URL" -path migrations -direction down -steps 1
```

### Migration Naming Convention
```
migrations/
├── 001_initial_schema.up.sql
├── 001_initial_schema.down.sql
├── 002_invoicing.up.sql
├── 002_invoicing.down.sql
└── ...
```

## Error Handling

The API uses standard HTTP status codes:

| Code | Meaning |
|------|---------|
| 200 | Success |
| 201 | Created |
| 400 | Bad Request (validation error) |
| 401 | Unauthorized (invalid/missing token) |
| 403 | Forbidden (insufficient permissions) |
| 404 | Not Found |
| 500 | Internal Server Error |

Error response format:
```json
{
  "error": "Human-readable error message"
}
```

## Frontend Architecture

The frontend is built with **SvelteKit** and **Svelte 5** using the new runes reactivity system.

### Technology Stack
- **Framework**: SvelteKit 2.x with Svelte 5
- **Styling**: Tailwind CSS
- **Build Tool**: Vite
- **Type Safety**: TypeScript

### Internationalization (i18n)

The frontend supports multiple languages using **Paraglide-JS**, a compile-time i18n solution.

#### Supported Languages
| Code | Language | Status |
|------|----------|--------|
| `en` | English | Default fallback |
| `et` | Estonian | Primary (Estonian market) |

#### Project Structure
```
frontend/
├── messages/
│   ├── en.json          # English translations (~650 keys)
│   └── et.json          # Estonian translations
├── project.inlang/
│   └── settings.json    # Inlang configuration
├── src/
│   └── lib/
│       └── paraglide/   # Generated translation functions
│           └── messages.js
```

#### Translation Key Naming Convention
```
{page}_{element}

Examples:
- nav_dashboard         → Navigation: Dashboard
- common_save           → Common: Save button
- invoices_newInvoice   → Invoices page: New Invoice
- payroll_monthJan      → Payroll: January
- tsd_statusDraft       → TSD: Draft status
```

#### Usage in Components
```svelte
<script lang="ts">
  import * as m from '$lib/paraglide/messages.js';
</script>

<!-- Simple translation -->
<h1>{m.dashboard_title()}</h1>

<!-- Parameterized translation -->
<p>{m.payroll_emptyState({ year: '2025' })}</p>

<!-- Dynamic translations (use functions, not objects) -->
function getStatusLabel(status: string): string {
  switch (status) {
    case 'DRAFT': return m.payroll_statusDraft();
    case 'APPROVED': return m.payroll_statusApproved();
    default: return status;
  }
}
```

#### Build Commands
```bash
# Compile translations
bun run paraglide

# Full build (includes paraglide)
bun run build

# Type check
bun run check
```

#### Adding New Translations
1. Add keys to `messages/en.json` and `messages/et.json`
2. Run `bun run paraglide` to generate TypeScript functions
3. Import and use in components: `m.your_new_key()`

#### Language Detection Priority
1. URL parameter (`?lang=et`)
2. localStorage (`preferredLanguage`)
3. Browser language (`navigator.language`)
4. Default: Estonian (`et`)

## Testing

### Backend Tests
Go standard library testing with race detection:

```bash
go test -race -cover ./...
```

### Frontend Tests
Vitest with jsdom environment for unit and integration tests:

```bash
cd frontend

# Run all tests
bun test

# Watch mode
bun run test:watch

# Coverage report
bun run test:coverage
```

#### Test Structure
```
frontend/src/tests/
├── setup.ts                   # Test environment setup
├── mocks/
│   └── app/                   # SvelteKit mocks
│       ├── navigation.ts      # $app/navigation mock
│       └── stores.ts          # $app/stores mock
├── i18n/
│   ├── messages.test.ts       # Translation value tests
│   └── translation-completeness.test.ts  # Key parity tests
└── components/
    └── LanguageSelector.test.ts  # Component tests
```

#### i18n Test Coverage
- **Messages Tests**: Verify translations return expected values for both locales
- **Completeness Tests**: Ensure all keys exist in both English and Estonian
- **Placeholder Tests**: Verify parameterized translations have matching placeholders

## Repository Pattern

The codebase uses the **Repository Pattern** for data access, providing abstraction between business logic and database operations.

### Repository Interface Structure

Each domain package defines a `Repository` interface and provides a PostgreSQL implementation:

```go
// Repository interface (domain contract)
type Repository interface {
    Create(ctx context.Context, schemaName string, entity *Entity) error
    GetByID(ctx context.Context, schemaName, id string) (*Entity, error)
    List(ctx context.Context, schemaName string) ([]Entity, error)
    // ...
}

// PostgresRepository implementation
type PostgresRepository struct {
    db *pgxpool.Pool
}
```

### Multi-Tenant Schema Qualification

All repository queries use schema-qualified table names for tenant isolation:

```go
query := fmt.Sprintf(`
    SELECT id, name FROM %s.accounts WHERE id = $1
`, schemaName)
```

### Benefits

1. **Testability** - Interfaces enable mocking for unit tests
2. **Flexibility** - Implementation can be swapped (e.g., to GORM)
3. **Separation of Concerns** - Business logic doesn't depend on database details
4. **Multi-Tenancy** - Schema name passed explicitly to every operation

## Performance Considerations

1. **Connection Pooling** - pgxpool for efficient database connections
2. **Prepared Statements** - Reduces SQL parsing overhead
3. **Schema Isolation** - Smaller tables, faster queries per tenant
4. **Index Strategy** - Composite indexes on (tenant_id, foreign_key)
5. **Pagination** - All list endpoints support limit/offset
6. **Compile-time i18n** - Paraglide generates optimized code with zero runtime overhead

## Testing Strategy

### Current Verification Gates

Coverage is tracked in CI and Codecov, but the repository does not currently claim fixed 90%+/95% thresholds as a maintained standard.

| Layer | Current Gate |
|-------|--------------|
| Backend | `go test ./...` must pass |
| Backend integration | `go test -tags=integration -race ...` must pass |
| Frontend | `bun run check` and `bun run test` must pass |
| E2E | Demo suite exists; blocking smoke E2E is still in progress |

### Backend Testing

```bash
# Unit tests (no database required)
go test -race -cover ./...

# Integration tests (requires PostgreSQL)
DATABASE_URL="postgres://..." go test -tags=integration -race -cover ./...
```

### Integration Test Structure

Integration tests use the `//go:build integration` build tag and test real database operations:

```go
//go:build integration

package accounting

func TestPostgresRepository_CreateAccount(t *testing.T) {
    pool := testutil.SetupTestDB(t)
    tenant := testutil.CreateTestTenant(t, pool)
    repo := NewPostgresRepository(pool)

    // Test actual database operations
}
```

### Test Utilities

The `internal/testutil` package provides shared test infrastructure:

- `SetupTestDB(t)` - Creates isolated test database connection
- `CreateTestTenant(t, pool)` - Creates tenant with schema for testing
- `CreateTestUser(t, pool, tenantID)` - Creates test user

### Mocking Strategy

For unit tests, domain packages provide mock implementations:

```go
type MockRepository struct {
    entities map[string]*Entity
    err      error
}

func (m *MockRepository) GetByID(ctx context.Context, schemaName, id string) (*Entity, error) {
    if m.err != nil {
        return nil, m.err
    }
    return m.entities[id], nil
}
```
