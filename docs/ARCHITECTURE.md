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

## Performance Considerations

1. **Connection Pooling** - pgxpool for efficient database connections
2. **Prepared Statements** - Reduces SQL parsing overhead
3. **Schema Isolation** - Smaller tables, faster queries per tenant
4. **Index Strategy** - Composite indexes on (tenant_id, foreign_key)
5. **Pagination** - All list endpoints support limit/offset
