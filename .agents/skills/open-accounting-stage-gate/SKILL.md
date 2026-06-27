---
name: open-accounting-stage-gate
description: Use when advancing open-accounting in staged PR work, especially CLI/API/docs parity, UI workflow coverage, local validation gates, commits, pushes, and PR/CI follow-through.
---

# Open Accounting Stage Gate

Use this after `open-accounting-development` when the task is broader than a single bug fix and the user expects coherent local-green stages committed and pushed.

If GitHub reports merge conflicts or the branch cannot be merged cleanly, load `open-accounting-pr-conflict-recovery` before editing conflicted files.

## Start From Current State

1. Check `git status --short --branch`; leave unrelated untracked paths alone.
2. Check the current PR with `gh pr view --json number,url,headRefName,mergeable,isDraft,statusCheckRollup`.
3. If a PR already exists or the user asks to create one "for now", keep pushing staged commits to the existing PR instead of creating duplicate PRs.
4. If GitHub reports conflicts or `mergeable` is not clean, load `open-accounting-pr-conflict-recovery` and resolve that before starting more feature work.
5. If the branch may already include the requested work, inspect files and PR state before creating new churn.
6. Select the next weak surface from current evidence, not memory alone:
   - `frontend/e2e/demo/*.spec.ts` and `docs/demo-e2e-testing.md` for demo workflow evidence.
   - `docs/USE_CASE_COVERAGE.md` and `docs/DEVELOPMENT_STATUS.md` for current UI/product gaps.
   - `make test-cli-coverage` for missing CLI/API/docs parity.
   - `docs/api_route_coverage_test.go` failures for API Markdown or Swagger gaps.
   - `internal/*/mappers/**` and import docs for provider-specific import parity gaps.
   - `frontend/e2e/demo/*.spec.ts` and Svelte route files for user workflow gaps; load `open-accounting-frontend-workflow` before editing them.

## Implementation Rules

- Keep each stage coherent: one user workflow or one command/API/docs surface at a time.
- Do not add API-only behavior when the CLI is expected to cover the route; add CLI command, command tests, route mapping, and `docs/CLI.md`.
- For API changes, update `docs/API.md`, generated Swagger artifacts, and route coverage mappings in the same stage.
- Prefer reusable services, mappers, and ORM-backed repositories; remove stale direct paths when touching the area.
- Do not preserve legacy code as a fallback unless the product requirement is explicit and the removal plan is written down. If a stage replaces a parser, mapper, query path, or duplicated entry-point behavior, delete the replaced path in the same stage.
- Preserve entry-point parity: frontend/API/CLI should call the same service/repository behavior through their normal layers.
- For Svelte route/component changes, load `open-accounting-frontend-workflow`, run the Svelte MCP autofixer when available, and prove the UI behavior with focused assertions before broad gates.
- For external import formats, load `open-accounting-import-mappers`; prove provider mapper tests, registry routing, API/CLI docs, and any visible UI import workflow in the same stage.
- For accounting-sensitive correction workflows, load `open-accounting-accounting-integrity`; prove the original record is preserved, the offsetting/void/reopen record is linked, locks are enforced, and derived balances are updated through reusable domain services.
- If the user asks to improve the repo development flow, load `open-accounting-skill-maintenance` and update the relevant `.agents/skills` files before the stage commit.

## Local Gates

Run focused gates first, then broad gates when the stage is stable:

```bash
make test-backend-coverage
make test-cli-coverage  # focused CLI-only gate when cmd/oa changed during the inner loop
go test -timeout=3m ./docs -count=1
cd frontend && bun run lint
cd frontend && bun run paraglide
cd frontend && bun run check:prepared
cd frontend && bun run test:prepared
cd frontend && bun run build:prepared
```

Use Go's default package parallelism for backend unit tests. Do not add `-p 1`
to the unit gate unless a current failure proves shared process state; the
integration Make targets already isolate DB-backed tests and should stay the
place for PostgreSQL/DDL serialization.

For Make target dry-runs, pass Makefile variables on the command line, for
example `make GO=echo test-integration-coverage`. Environment assignments such
as `GO=echo make ...` do not override variables assigned inside the Makefile.

Use `make test-backend-coverage` for backend stage closeout. It runs the full
race-enabled Go test suite once and verifies the 100% `cmd/oa` coverage
invariant from the same coverage profile, avoiding a duplicate CLI package test
pass in CI and local closeout.

For repository changes, add and run a focused integration test:

```bash
go test -timeout=5m -tags=integration ./internal/<package> -run '<FocusedTest>' -count=1
```

For user-facing demo workflows, use `open-accounting-demo-e2e` and run the focused Playwright spec before committing.

## Commit And PR Discipline

- Commit only after the relevant local gates pass.
- Use `git add -u` when unrelated untracked paths are present.
- Push each successful stage so PR process is preserved.
- After push, report the commit hash and PR check state separately from local validation.
- Do not claim deploy/live readiness from local or PR-green evidence.
- Preserve exact gate evidence in the final status: commands run, focused E2E spec names, PR number, remote workflow/check state, and any unrelated dirty paths left untouched.
