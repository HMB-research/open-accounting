---
name: open-accounting-demo-e2e
description: Use when running or debugging open-accounting local demo Playwright E2E, including demo reset, auth setup, API/frontend ports, CORS, stale localhost servers, and branch-code verification.
---

# Open Accounting Demo E2E

Use this with `open-accounting-development` for local demo Playwright checks. The resettable local demo is the source of truth unless a hosted demo URL is explicitly requested.

## Preflight

1. Inspect listeners before running Playwright:
   ```bash
   lsof -nP -iTCP:5173 -sTCP:LISTEN || true
   lsof -nP -iTCP:8080 -sTCP:LISTEN || true
   ```
2. Do not trust an existing `localhost:5173` server until verified. It can be another repo. If uncertain, use a clean port such as `5174`.
3. Do not trust the Docker Compose API on `localhost:8080` for branch-code verification. It may be a stale image. Use `curl /health` only as liveness, not proof of current source.

## Branch API Pattern

Run the current working tree API on an alternate port against the local Postgres container:

```bash
DATABASE_URL='postgres://openaccounting:openaccounting@localhost:5432/openaccounting?sslmode=disable' \
PORT=18080 \
JWT_SECRET='development-only-insecure-jwt-secret' \
ALLOWED_ORIGINS='http://localhost:5173,http://localhost:5174' \
DEMO_MODE=true \
DEMO_RESET_SECRET='test-demo-secret' \
go run ./cmd/api
```

Then reset demo data through that branch API:

```bash
curl -fsS -X POST http://localhost:18080/api/demo/reset \
  -H 'X-Demo-Secret: test-demo-secret'
```

If using a frontend port other than `5173`, include it in `ALLOWED_ORIGINS` before starting the API. Confirm CORS if auth reports "Unable to connect to server":

```bash
curl -sS -D - -o /tmp/options.out -X OPTIONS http://localhost:18080/api/v1/auth/login \
  -H 'Origin: http://localhost:5174' \
  -H 'Access-Control-Request-Method: POST' \
  -H 'Access-Control-Request-Headers: content-type'
```

## Focused Playwright Command

Use a clean frontend port and point the browser at the branch API:

```bash
cd frontend
BASE_URL=http://localhost:5174 \
PUBLIC_API_URL=http://localhost:18080 \
DEMO_RESET_SECRET=test-demo-secret \
bunx playwright test --config=playwright.demo.config.ts --project=demo-chromium e2e/demo/<spec>.spec.ts --workers=1
```

The config starts the frontend server only. Start the API separately first.

## Assertion Discipline

- Prefer API waits and visible UI states over fixed sleeps.
- Assert mutation responses and rendered UI, not only that a page did not crash.
- Use unique test data for creates so seeded demo records cannot satisfy assertions.
- Scope generic controls such as selects and buttons to the route panel or filter bar that owns them.
- When filtering records, assert both the expected visible row and at least one expected absent row.

## Failure Triage

- Login page shows another product or brand: wrong frontend server was reused; switch `BASE_URL` to a clean port.
- Auth setup says credentials fail: test direct API login before changing seed data.
- Auth setup says unable to connect: check `PUBLIC_API_URL`, API liveness, and CORS for the frontend origin.
- UI crashes after successful API response: inspect the screenshot and console context; treat this as a real product bug before weakening the test.
- Keep the branch API process running only for the E2E run; stop it before final status.
