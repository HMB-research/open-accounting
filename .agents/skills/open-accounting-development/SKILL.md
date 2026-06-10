---
name: open-accounting-development
description: Use when developing or debugging open-accounting - covers multi-tenant architecture, testing strategy (especially E2E for demo interface), and layer responsibilities. Activate for feature work, bug fixes, test writing, or demo mode issues.
---

# Open Accounting Development Guide

## Related Workflow Skills

Use this skill for architecture, layer boundaries, and general testing decisions. Load a focused workflow skill when the task matches:

- `open-accounting-stage-gate`: staged PR work, CLI/API/docs parity, local validation gates, commits, pushes, and PR/CI follow-through.
- `open-accounting-demo-e2e`: local demo Playwright runs, demo reset, auth setup, branch API verification, localhost port conflicts, and CORS/debugging.
- `open-accounting-frontend-workflow`: Svelte route/component edits, frontend form payload contracts, Svelte MCP validation, and stable Playwright assertions.
- `open-accounting-import-mappers`: external import format work, provider/bank-specific mapper boundaries, official sample fixtures, registry parity, and legacy parser removal.
- `open-accounting-accounting-integrity`: payment reversals, invoice paid-state changes, journal/close correction flows, period locks, evidence blockers, and audit-preserving accounting history changes.
- `open-accounting-pr-conflict-recovery`: PR merge conflicts, branch rebases/merges, conflicted frontend tests, post-conflict local gates, and follow-up pushes.
- `open-accounting-skill-maintenance`: capture workflow learnings in repo skills after repeated friction, user preference changes, or new staged-validation patterns.

## Architecture Context

### Multi-Tenant Data Flow

```
Request → Auth Middleware (JWT → user_id, tenant_id, role)
        → Tenant Middleware (sets schema name: "tenant_{uuid}")
        → Handler (validates input, extracts tenant context)
        → Service (business logic, orchestrates repositories)
        → Repository/ORM (schema-qualified persistence for {schema}.table)
```

### Persistence and Reuse Bias

This repo treats ORM/repository-backed persistence and reusable application code as the default engineering standard, even when that turns a small change into a focused refactor.

- Prefer repository methods and ORM-backed implementations for all persistence work. Do not add raw SQL in handlers or services.
- If a feature needs persistence behavior that is not exposed yet, refactor the repository interface and its Postgres/GORM implementations rather than bypassing the layer.
- Use direct SQL only in migrations or repository implementations where the ORM cannot express the query clearly. Keep those exceptions schema-qualified, documented by tests, and isolated behind repository methods.
- Prefer shared parser, mapper, validation, and formatting helpers over command-specific, handler-specific, or one-off copies. Create reusable package boundaries when multiple entry points need the same behavior.
- Treat refactoring as part of feature delivery when the current shape would otherwise force duplicated logic, direct SQL, or entry-point-specific behavior. Do not keep legacy paths just to minimize the diff.
- Remove related legacy direct-query, duplicated mapper, or format-specific code while touching the area. Keeping legacy paths around for compatibility is not preferred unless there is an explicit product requirement and a concrete removal plan.
- When the user says not to keep legacy code, treat stale parser branches, raw SQL shortcuts, compatibility-only helpers, and duplicated command/handler behavior as deletion candidates in the same stage. Do not leave them as dormant fallback paths.
- CLI, API, service, and frontend entry points should call the same reusable service/mapper/repository behavior instead of maintaining parallel implementations.
- For bank, payroll, invoice, or other external imports, put parsing and normalization behind reusable mapper packages. Use a bank/provider-specific mapper boundary when formats differ so additional formats can be added without branching through handlers or CLI commands.
- For provider-specific import changes, load `open-accounting-import-mappers` and verify the mapper, registry, API/CLI/docs, and user-facing workflow together.
- When the user asks to improve skills or the work reveals a repeatable process gap, update `.agents/skills` in the same stage using concise, reusable guidance. Prefer updating an existing focused skill; add a new skill only when the trigger and workflow are distinct.

### Layer Responsibilities

| Layer | Owns | Does NOT Own |
|-------|------|--------------|
| Handler | HTTP concerns, input validation, response formatting | Business logic, DB transactions |
| Service | Business rules, orchestration, cross-entity logic, reusable mapper orchestration | HTTP details, direct SQL |
| Repository | Data access, schema qualification, ORM/query building | Business validation, HTTP context |

### Key Files for Debugging

- **Tenant context**: `internal/auth/middleware.go` (JWT extraction)
- **Schema routing**: `internal/tenant/service.go` (schema name generation)
- **Demo detection**: Check for `DEMO_MODE=true`, demo tenant IDs, or `demoN@example.com` users
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
│   └── Demo flow? → frontend/e2e/demo/*.spec.ts
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

The resettable local demo is the authoritative demo verification target unless a hosted demo URL is explicitly configured. All demo functionality must have E2E coverage:

- Login/logout flow
- Dashboard widgets and navigation
- CRUD operations (invoices, contacts, payments)
- Report generation and exports
- Error states and edge cases

For concrete local server, reset, CORS, and Playwright commands, use `open-accounting-demo-e2e`.

### Test Gate Performance

- Run backend stage closeout with `make test-backend-coverage`; it uses Go's default package parallelism, runs the race-enabled suite once, and verifies the 100% `cmd/oa` coverage invariant from the same profile.
- Avoid `-p 1` for unit tests unless a current failure proves shared process state. It materially slows local and CI feedback.
- Measure focused gate timings before optimizing so the stage can report before/after evidence.
- Prefer focused package/test commands during the inner loop; reserve full `make test-integration-coverage` for stage closeout or CI because the full local integration gate can take around 11 minutes.
- Keep DB-backed tests in the tagged integration gate (`make test-integration-coverage`); schema setup and teardown deliberately serialize PostgreSQL DDL there.
- For slow auth/security tests, prefer injectable bcrypt costs, clocks, token generators, and timing controls over production-cost hashing or sleeps.
- For integration speedups, reduce repeated tenant schema lifecycle by grouping related repository cases or reusable fixtures while preserving real ORM/Postgres coverage.
- For integration shard speedups, measure package durations from CI logs before changing shard counts. The current package round-robin can become imbalanced when heavy ORM/Postgres packages cluster on one shard; prefer weight-aware package buckets or reusable tenant fixtures before adding runners.
- Run frontend Paraglide/SvelteKit-writing gates serially (`check`, `test`, `build`) when executing them locally. Parallel runs can race on generated `frontend/src/lib/paraglide` or `.svelte-kit` files and produce false negatives.
- In Playwright demo specs, speed up broad view checks with route-owned readiness selectors and shared helpers instead of fixed sleeps. Do not remove shared navigation stability globally unless all dependent specs are refactored to own their route readiness.
- Keep broad demo Playwright specs inside `frontend/e2e/demo/` so default file-order sharding distributes them with the rest of the demo suite. Root-level broad specs sort after `demo/` and can create a slow tail shard.
- Use uploaded `demo-test-results.json` artifacts from CI before changing shard counts. Download `playwright-results-shard-*` artifacts and run `scripts/parse-playwright-spec-times.mjs /tmp/playwright-results/*/demo-test-results.json` so broad spec optimization is based on measured executed durations. Rebalance or refactor by measured spec/test durations first, then add runners only when wall time remains bounded by setup overhead.

## Demo Mode Reference

### Credentials

- **Emails**: `demo1@example.com`, `demo2@example.com`, `demo3@example.com`, `demo4@example.com`
- **Password**: `demo12345`
- **Local demo API**: run with `DEMO_MODE=true DEMO_RESET_SECRET=test-demo-secret`
- **Hosted demo**: optional; only use when `TEST_DEMO=true`, `BASE_URL`, and `PUBLIC_API_URL` are explicitly provided

### Demo Data Seeding Flow

```
Start API with DEMO_MODE=true
  → POST /api/demo/reset with X-Demo-Secret
  → Backend recreates and seeds tenant_demo1..tenant_demo4
  → Playwright auth setup logs in all four demo users
```

### Key Demo Files

| Purpose | Location |
|---------|----------|
| Seed template | `internal/demo/seed_template.sql` |
| Demo handlers | `cmd/api/handlers_demo.go` (demo reset/status endpoints) |
| E2E tests | `frontend/e2e/demo/*.spec.ts`, `frontend/e2e/smoke/*.spec.ts` |
| Test config | `frontend/playwright.demo.config.ts` |
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

4. **Test locally**: use `open-accounting-demo-e2e` for branch-code verification, or `cd frontend && bun run test:e2e:smoke` for the gate smoke suite when the local API is already correct

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

For stage commits, pushes, and PR status checks, use `open-accounting-stage-gate`.

## Error Handling Patterns

### Tenant Validation (Frontend)

All action handlers must validate tenant context and show errors to users:

```typescript
import { requireTenantId, parseApiError } from '$lib/utils/tenant';
import ErrorAlert from '$lib/components/ErrorAlert.svelte';

// In action handler:
async function doAction() {
    const tenantId = requireTenantId($page, (err) => (error = err));
    if (!tenantId) return;  // Shows error to user automatically

    actionLoading = true;
    error = '';
    try {
        await api.someAction(tenantId, ...);
        success = 'Action completed';
    } catch (err) {
        error = parseApiError(err);  // Handles 403, 401, network errors
    } finally {
        actionLoading = false;
    }
}
```

### Common Error Types

| Error | Cause | User Message |
|-------|-------|--------------|
| No tenant | URL missing `?tenant=` param | "Please select an organization first" |
| Access denied | User not member of tenant | "Access denied to this organization" |
| Session expired | JWT expired | "Your session has expired. Please log in again." |
| Network error | API unreachable | "Network error. Please try again." |

### Key Files

| Purpose | Location |
|---------|----------|
| Tenant validation | `frontend/src/lib/utils/tenant.ts` |
| Error alerts | `frontend/src/lib/components/ErrorAlert.svelte` |
| Global error page | `frontend/src/routes/+error.svelte` |
| API error parsing | `frontend/src/lib/utils/tenant.ts` (parseApiError) |

## MCP Server

The project includes an MCP server for AI assistant integration.

### Setup

```bash
cd mcp-server
bun install
bun run dev  # Development
```

### Add to Codex

```bash
Codex mcp add open-accounting -- npx tsx /path/to/mcp-server/src/index.ts
```

### Environment Variables

```bash
OPEN_ACCOUNTING_API_URL=http://localhost:8080
OPEN_ACCOUNTING_API_TOKEN=your-jwt-token
```

### Available Tools

- `list_invoices` - List invoices with filters
- `create_invoice` - Create new invoice
- `get_account_balance` - Get account balance
- `generate_report` - Generate financial reports
- `list_contacts` - List customers/vendors
- `record_payment` - Record a payment
- `get_chart_of_accounts` - Get chart of accounts

## Quick Reference

### Common Commands

```bash
# Backend
go test -race -cover ./...                    # Unit tests
go test -tags=integration -race ./...         # Integration tests
go run ./cmd/api                              # Start API server

# Frontend
cd frontend
bun run dev                                   # Dev server
bun test                                      # Vitest unit tests
bun run test:e2e                              # Playwright E2E
bun run test:e2e:smoke                        # Blocking smoke E2E
bun run test:e2e:verify                       # Seeded demo data verification
bun run check                                 # TypeScript check
bun run paraglide                             # Compile translations
```

### Project URLs (Local)

- API: `http://localhost:8080`
- Frontend: `http://localhost:5173`
- Swagger: `http://localhost:8080/swagger/`
