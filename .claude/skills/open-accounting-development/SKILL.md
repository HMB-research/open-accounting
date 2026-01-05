---
name: open-accounting-development
description: Use when developing or debugging open-accounting - covers multi-tenant architecture, testing strategy (especially E2E for demo interface), and layer responsibilities. Activate for feature work, bug fixes, test writing, or demo mode issues.
---

# Open Accounting Development Guide

## Architecture Context

### Multi-Tenant Data Flow

```
Request → Auth Middleware (JWT → user_id, tenant_id, role)
        → Tenant Middleware (sets schema name: "tenant_{uuid}")
        → Handler (validates input, extracts tenant context)
        → Service (business logic, orchestrates repositories)
        → Repository (schema-qualified queries: SELECT FROM {schema}.table)
```

### Layer Responsibilities

| Layer | Owns | Does NOT Own |
|-------|------|--------------|
| Handler | HTTP concerns, input validation, response formatting | Business logic, DB transactions |
| Service | Business rules, orchestration, cross-entity logic | HTTP details, direct SQL |
| Repository | Data access, schema qualification, query building | Business validation, HTTP context |

### Key Files for Debugging

- **Tenant context**: `internal/auth/middleware.go` (JWT extraction)
- **Schema routing**: `internal/tenant/service.go` (schema name generation)
- **Demo detection**: Check for `demo@example.com` user or demo tenant ID
- **Multi-tenant queries**: All repositories use `schemaName` parameter for table qualification

## Testing Strategy

### Decision Tree - Which Test Type?

```
Is the change in...
├── Repository layer? → Integration test (needs real DB)
│   └── File: internal/{domain}/*_test.go with //go:build integration
│
├── Service layer? → Unit test with mocked repository
│   └── File: internal/{domain}/*_test.go
│
├── Handler layer? → Unit test with mocked service
│   └── File: internal/{domain}/handlers_test.go
│
├── Frontend component? → Vitest component test
│   └── File: frontend/src/tests/components/*.test.ts
│
├── User-facing workflow? → E2E test
│   └── Demo flow? → frontend/e2e/demo-*.spec.ts
│   └── Regular flow? → frontend/e2e/*.spec.ts
```

### Coverage Targets

| Layer | Target |
|-------|--------|
| Backend (unit + integration) | 90%+ |
| Frontend | 95%+ |
| Critical paths (auth, payments) | 95%+ |
| Demo interface | **100% E2E** |

### Demo E2E Priority

The demo at `open-accounting.up.railway.app` is the first impression for users. All demo functionality must have E2E coverage:

- Login/logout flow
- Dashboard widgets and navigation
- CRUD operations (invoices, contacts, payments)
- Report generation and exports
- Error states and edge cases

## Demo Mode Reference

### Credentials

- **Email**: `demo@example.com`
- **Password**: `demo123`
- **Live demo**: `open-accounting.up.railway.app`

### Demo Data Seeding Flow

```
Login as demo@example.com
  → Backend checks if demo tenant exists
  → If not: creates tenant + schema + seeds demo data
  → If exists: optionally resets to fresh state (hourly on Railway)
```

### Key Demo Files

| Purpose | Location |
|---------|----------|
| Seed logic | `internal/tenant/demo_seed.go` |
| Demo handlers | `internal/tenant/handlers.go` (demo reset endpoint) |
| E2E tests | `frontend/e2e/demo-*.spec.ts` |
| Test config | `frontend/playwright.config.ts` |
| Test reports | `frontend/playwright-report-demo/` |

### Multi-User Parallel Testing

E2E tests support parallel execution with isolated demo data:

- Each test worker gets unique demo seed
- Tenant IDs passed via URL parameters for data isolation
- Prevents test interference when running in CI

### Debugging Demo Issues

1. **Check tenant schema exists**:
   ```sql
   SELECT schema_name FROM information_schema.schemata
   WHERE schema_name LIKE 'tenant_%';
   ```

2. **Verify seed data**: Check `internal/tenant/demo_seed.go` for expected accounts/contacts/invoices

3. **Check E2E logs**: Review `frontend/playwright-report-demo/` for failure screenshots and traces

4. **Test locally**: `cd frontend && npm run test:e2e:demo`

## Documentation Checklist

After implementing a feature or fix, update relevant docs:

| Change Type | Update |
|-------------|--------|
| API change | `docs/API.md` |
| Architecture change | `docs/ARCHITECTURE.md` |
| Demo behavior change | `README.md` demo section |
| New E2E test patterns | `docs/plans/` design doc |
| Translation keys added | Both `messages/en.json` and `messages/et.json` |

### Plan Documents

For non-trivial work, create a design doc before implementation:

- **Location**: `docs/plans/YYYY-MM-DD-{topic}-design.md`
- **Purpose**: Capture decisions, trade-offs, implementation approach
- **Example**: `docs/plans/2026-01-04-demo-data-reset-testing-design.md`

### Commit Message Format

```
feat: add new capability
fix: resolve bug
docs: documentation only
test: add or update tests
refactor: restructure without behavior change
chore: maintenance tasks
```

## Quick Reference

### Common Commands

```bash
# Backend
go test -race -cover ./...                    # Unit tests
go test -tags=integration -race ./...         # Integration tests
go run ./cmd/api                              # Start API server

# Frontend
cd frontend
npm run dev                                   # Dev server
npm test                                      # Vitest unit tests
npm run test:e2e                              # Playwright E2E
npm run test:e2e:demo                         # Demo-specific E2E
npm run check                                 # TypeScript check
npm run paraglide                             # Compile translations
```

### Project URLs (Local)

- API: `http://localhost:8080`
- Frontend: `http://localhost:5173`
- Swagger: `http://localhost:8080/swagger/`
