---
name: open-accounting-frontend-workflow
description: Use when editing open-accounting Svelte routes, components, or frontend tests, especially form payloads, tenant-scoped UI, demo E2E assertions, Svelte MCP validation, and Playwright locator stability.
---

# Open Accounting Frontend Workflow

Use this with `open-accounting-development` for Svelte UI work and with `open-accounting-stage-gate` before committing a frontend stage.

## Svelte Route And Component Rules

- Run the official Svelte MCP autofixer after editing `.svelte` files or Svelte component tests when the tool is available. Fix hard issues before running broader gates.
- Pass the exact current component source to the Svelte MCP and rerun it after fixes. If a large component input is accidentally truncated or malformed during tool entry, discard that result and rerun with valid current file contents before treating the validation as meaningful.
- Remove stale `svelte-ignore` comments when the ignored issue no longer exists. Keep an ignore only when the UI pattern is intentional and the reason is still current.
- Key each blocks by stable domain IDs, not indexes or unkeyed collections, when rendering persisted records.
- Keep frontend forms aligned with API contracts. If an API expects an RFC3339 timestamp, do not submit a raw `YYYY-MM-DD` date input value.
- Prefer route-local helper functions or shared API-client helpers for payload normalization when multiple handlers need the same shape.
- Update both `messages/en.json` and `messages/et.json` when adding user-facing text.
- After significant route or workflow changes, open the affected local page in the in-app Browser when practical. Treat it as a smoke check that the route renders and core controls are visible; focused E2E remains the behavioral proof.
- For modal workflows, prefer accessible dialog markup with explicit close/cancel actions, Escape handling, submit-disabled loading states, and no stale `svelte-ignore` comments.

## E2E Test Discipline

- Use `open-accounting-demo-e2e` for branch-code demo verification. Do not rely on the Docker Compose API on `localhost:8080` as proof of current source.
- Wait for route API responses or stable loaded UI states instead of sleeping.
- For demo specs, prefer `waitForRouteReady` with a selector owned by the current route shell, then assert specific controls or rows. Do not replace a sleep with a broad text match, and do not make a generic loading-class wait block routes whose data loader is not the behavior under test.
- Do not leave unconditional assertions such as `expect(true)`.
- For create/update workflows, assert the API response status and important response fields, then assert the rendered row or detail view.
- For filters, assert both sides: the included record is visible and the excluded record is absent.
- Use unique references, document numbers, or names in tests so seeded data and prior runs cannot satisfy the assertion accidentally.
- Scope generic controls to their owning UI region. For example, prefer a payments filter container over `page.locator('select').first()` because the tenant selector may also be present.

## Focused Gates

After a frontend route or component change, run the smallest useful focused gate first, then the normal stage gates:

```bash
cd frontend && bun run check
cd frontend && bun run lint
cd frontend && bun run test
```

Run Paraglide/SvelteKit-writing gates serially. In this repo `check`, `test`,
and `build` can compile Paraglide or touch `.svelte-kit`; running them in
parallel can fail with filesystem races such as `ENOTEMPTY` while removing
`frontend/src/lib/paraglide`.

For demo workflows, start the branch API with `open-accounting-demo-e2e` and run the affected focused Playwright spec before committing.
