# Demo E2E Testing

This guide describes the local and CI Playwright suites that verify the resettable demo environment.

## What The Suite Covers

The demo E2E tests verify:

1. Demo users can authenticate and reuse saved browser state.
2. Seeded demo tenants contain representative accounting data.
3. Core pages render data or the expected empty state without API errors.
4. Critical workflows are reachable from the UI: accounting, invoicing, banking, payroll, tax, reports, settings, and admin plugin screens.
5. Mobile and tablet viewports remain usable for the covered pages.

## Local Demo Environment

Start a local API with demo reset enabled before running Playwright:

```bash
docker-compose up -d db
export DATABASE_URL="postgres://openaccounting:openaccounting@localhost:5432/openaccounting?sslmode=disable"
go run ./cmd/migrate -db "$DATABASE_URL" -path migrations -direction up
DEMO_MODE=true DEMO_RESET_SECRET=test-demo-secret go run ./cmd/api
```

Seed or reset demo data:

```bash
curl -X POST http://localhost:8080/api/demo/reset \
  -H "Content-Type: application/json" \
  -H "X-Demo-Secret: test-demo-secret"
```

Playwright starts the SvelteKit dev server automatically for localhost targets. Override the frontend or API URL if needed:

```bash
export BASE_URL=http://localhost:5173
export PUBLIC_API_URL=http://localhost:8080
export DEMO_RESET_SECRET=test-demo-secret
```

## Commands

Run commands from `frontend/`.

| Command | Purpose |
|---------|---------|
| `bun run test:e2e:smoke` | Blocking CI smoke suite for the core accountant flow |
| `bun run test:e2e:verify` | Strict seeded-data verification tests |
| `bun run test:e2e` | Full Playwright suite from `playwright.demo.config.ts` |
| `bun run test:e2e:ui` | Playwright UI mode |
| `bun run test:e2e:debug` | Playwright debug mode |
| `bun run test:e2e:loop` | Retry the full demo suite against an existing environment |
| `bun run test:e2e:loop:verify` | Retry only the seeded-data verification tests |

To run only the main demo project without the smoke project:

```bash
bunx playwright test --config=playwright.demo.config.ts --project=demo-chromium
```

To run reset-idempotency tests locally, keep `CI` unset because `e2e/demo/reset.spec.ts` intentionally skips in CI:

```bash
DEMO_RESET_SECRET=test-demo-secret \
PUBLIC_API_URL=http://localhost:8080 \
bunx playwright test --config=playwright.demo.config.ts e2e/demo/reset.spec.ts
```

## Retry Loop

`scripts/test-loop.sh` assumes the frontend/API/database environment is already available. It retries failed demo tests and calls `/api/demo/reset` between attempts.

```bash
# Run all demo tests with up to 10 attempts
../scripts/test-loop.sh 10

# Run a specific test file with up to 20 attempts
../scripts/test-loop.sh 20 e2e/demo/data-verification.spec.ts

# Run tests matching a Playwright file/filter argument
../scripts/test-loop.sh 5 banking
```

The loop uses these environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `BASE_URL` | `http://localhost:5173` | Frontend URL |
| `PUBLIC_API_URL` | `http://localhost:8080` | Backend API URL |
| `DEMO_RESET_SECRET` | `test-demo-secret` | Secret for the demo reset API |

## Demo Users

The auth setup project authenticates all four demo users and stores state under `frontend/.auth/`.

| Worker | Email | Tenant ID |
|--------|-------|-----------|
| 0 | `demo1@example.com` | `b0000000-0000-0000-0001-000000000001` |
| 1 | `demo2@example.com` | `b0000000-0000-0000-0002-000000000001` |
| 2 | `demo3@example.com` | `b0000000-0000-0000-0003-000000000001` |
| 3 | `demo4@example.com` | `b0000000-0000-0000-0004-000000000001` |

## Seeded Data Checks

`e2e/demo/data-verification.spec.ts` checks the seeded dataset through both UI and API paths.

| Area | Expected seeded data |
|------|----------------------|
| Accounts | 33 chart of accounts entries |
| Journal | 16 journal entries |
| Contacts | 7 contacts |
| Invoices | 9 invoices |
| Payments | 4 payments |
| Employees | 5 employees |
| Payroll | 3 payroll runs |
| Recurring invoices | 3 recurring invoices |
| Banking | 2 bank accounts |
| TSD | 3 declarations |

## CI Integration

The GitHub Actions `e2e-smoke` job is blocking. It starts PostgreSQL, builds the API and migration binaries, runs migrations, starts the API in demo mode, seeds demo data through `/api/demo/reset`, and runs:

```bash
cd frontend
CI=true \
BASE_URL=http://localhost:5173 \
PUBLIC_API_URL=http://localhost:8080 \
DEMO_RESET_SECRET=test-demo-secret \
bun run test:e2e:smoke
```

The broader `e2e` job runs the full `demo-chromium` project in shards and is blocking. Each shard starts its own local PostgreSQL-backed demo environment, seeds all four demo users through `/api/demo/reset`, builds the frontend, and runs:

```bash
cd frontend
CI=true \
BASE_URL=http://localhost:5173 \
PUBLIC_API_URL=http://localhost:8080 \
DEMO_RESET_SECRET=test-demo-secret \
bunx playwright test --config=playwright.demo.config.ts --project=demo-chromium --shard=<shard>/2
```

The separate `e2e-demo` job targets an externally hosted demo and remains optional/informational because it only runs when hosted demo URLs and secrets are explicitly configured.

## Troubleshooting

If login fails, verify the API is in demo mode and the reset endpoint succeeded:

```bash
curl http://localhost:8080/health
curl http://localhost:8080/api/demo/status?user=2 -H "X-Demo-Secret: test-demo-secret"
curl -X POST http://localhost:8080/api/demo/reset -H "X-Demo-Secret: test-demo-secret"
```

If Playwright cannot start the frontend because a port is busy, choose another local port and allow that origin in the API CORS configuration:

```bash
ALLOWED_ORIGINS=http://localhost:5185 DEMO_MODE=true DEMO_RESET_SECRET=test-demo-secret go run ./cmd/api
BASE_URL=http://localhost:5185 PUBLIC_API_URL=http://localhost:8080 DEMO_RESET_SECRET=test-demo-secret bun run test:e2e:smoke
```
