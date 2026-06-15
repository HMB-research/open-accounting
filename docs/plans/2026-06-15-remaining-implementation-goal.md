# Remaining Implementation Goal

Date: 2026-06-15
Branch: `feat/payroll-history-import`
PR: #62

This plan turns the broad goal of fully tested accounting software with 100% CLI
coverage and current documentation into parallel work slices that subagents can
implement without contending on the same files.

## Goal Prompt

Use this as the next `/goal` prompt when the current goal is ready to be
refocused:

```text
Work through the remaining verified gaps toward fully working, documented
accounting software with 100% CLI tool coverage. Keep PR #62 green and avoid
overclaiming external or production-hardening capabilities.

Coordinate the work as parallel subagent slices with disjoint write scopes.
Each slice must implement a focused missing workflow, add or update tests,
run the local gates listed for that slice, and commit after a successful
stage. Keep docs/status updates as a final integrator slice after feature
work lands.

Current local invariants:
- `cmd/oa` CLI statement coverage must remain 100.0%.
- API-backed CLI command coverage must include command-path, documentation,
  usage, functional `app.run`, and mocked HTTP method/path contract checks.
- `go test -timeout=3m ./docs -count=1` must pass after doc/status changes.
- `golangci-lint run` must pass after Go changes.
- Run focused backend/frontend gates before broad CI.
- Do not mark blocked external integrations as locally complete.
```

## Missing Parts

1. Historical migration and cutover remains partial.
   Missing depth includes provider-specific mapping, cross-file validation
   outside payroll/TSD history, and more dashboard-side mutating cutover
   controls.

2. Document retention and evidence policy remains partial.
   Missing depth includes broader workflow-level evidence enforcement and
   executable follow-up actions beyond the already covered retention,
   evidence upload, replacement, and approval paths.

3. Accountant review workspace remains partial.
   Missing depth includes more direct execution for remaining
   payroll/document/evidence-policy edges beyond the current assignment
   actions.

4. Close/year-end remains partial.
   Missing depth includes accountant-assigned close correction polish beyond
   the direct close and carry-forward assignment completion paths.

5. Auth, plugin, backup, and deployment hardening remain partial.
   These are mostly product-hardening or deployment-operation tracks rather
   than blockers for the payroll-history PR.

6. Direct bank feeds, direct SEPA initiation, e-invoice operator exchange, OCR,
   and automatic e-MTA filing remain blocked.
   These need external partnerships, certification, credentials, or
   infrastructure and should not be assigned as ordinary local PR tasks.

## Current `/goal` Batch

Use these concrete slices before waiting on another full CI cycle:

1. Evidence-policy structured conflicts.
   Main-thread slice: return `evidence_policy_results` plus flattened
   `remediation_actions` from purchase-invoice send/email and fixed-asset
   activation/disposal `409` responses, with handler tests, API docs, status
   docs, and generated Swagger.

2. Auth hardening proof slice.
   Subagent slice: clarify which auth guarantees are locally proven by
   production startup validation, tenant-member/session/API-token controls, and
   API/CLI route coverage, while keeping live rollout proof out of scope.

3. Backup/restore operations proof slice.
   Main-thread slice: add `oa ops backup offsite-sync --preflight` and
   `oa ops backup restore-drill --preflight` parity with the existing scripts,
   plus CLI tests and operator docs. This remains offline proof; live provider
   credentials and host timers stay deployment responsibilities.

4. Plugin runtime hardening slice.
   Subagent slice: identify one locally testable plugin runtime isolation,
   supervision, or operator-control hardening gap.

5. Migration/cutover validation slice.
   Subagent slice: identify one provider-specific mapping or cross-file
   validation gap that is safe to prove with local cutover validator/importer
   tests.

## Subagent Slices

### Slice 1: Migration/Cutover Backend

Owner scope:
- `internal/cutover`
- relevant importer packages only when a mapping requires import execution
- `cmd/api/handlers_migration*`
- `cmd/oa/migration_*`
- migration-related CLI tests

Implement:
- one provider-specific mapping gap, or
- one cross-file validation gap outside payroll/TSD history, or
- one guarded cutover execution/workbench backend control.

Local gates:
- `go test ./internal/cutover ./cmd/api ./cmd/oa -run 'Test(Migration|CLI|Execute|Provider|Validate)' -count=1`
- `make test-cli-coverage`
- `golangci-lint run`
- `make swagger` if API output changes
- `go test -timeout=3m ./docs -count=1` if generated or status docs change

### Slice 2: Document/Evidence Policy Backend

Owner scope:
- `internal/documents`
- workflow-specific evidence guards in `cmd/api`
- document-related `cmd/oa` commands and tests

Implement:
- one remaining executable evidence-policy follow-up, or
- one workflow-level policy enforcement gap with clear remediation output.

Local gates:
- `go test ./internal/documents ./cmd/api ./cmd/oa -run 'Test(Document|Evidence|Retention|Policy)' -count=1`
- `make test-cli-coverage`
- `golangci-lint run`
- `make swagger` if API output changes
- `go test -timeout=3m ./docs -count=1` if generated or status docs change

### Slice 3: Accountant Workspace Frontend Execution

Owner scope:
- `frontend/src/lib/components/AccountantReviewPanel.svelte`
- `frontend/src/lib/review/workspace.ts`
- `frontend/src/lib/api.ts`
- matching tests under `frontend/src/tests/`

Implement:
- one direct workspace action for a remaining payroll, document,
  evidence-policy, migration, or close assignment row.

Local gates:
- `cd frontend && bun run test:prepared -- src/tests/components/AccountantReviewPanel.test.ts src/tests/lib/review-workspace.test.ts src/tests/lib/api.test.ts`
- `cd frontend && bun run check:prepared`
- `cd frontend && bun run lint`

### Slice 4: Close/Year-End Workspace Polish

Owner scope:
- close/carry-forward service or API files only if needed
- accountant workspace frontend files for direct actions
- related backend/frontend tests

Implement:
- one accountant-assigned close correction workflow beyond already completed
  direct close/carry-forward completion.

Local gates:
- `go test ./internal/accounting ./cmd/api ./cmd/oa -run 'Test(YearEnd|Close|CarryForward|Remediation)' -count=1`
- `cd frontend && bun run test:prepared -- src/tests/components/AccountantReviewPanel.test.ts src/tests/lib/review-workspace.test.ts`
- `make test-cli-coverage` if CLI output changes
- `golangci-lint run` if Go code changes

### Slice 5: Auth Hardening

Owner scope:
- `internal/auth`
- auth and tenant API handlers/tests
- auth-related CLI commands/tests
- frontend settings only if explicitly included

Implement:
- one incremental auth-hardening gap that has deterministic local tests.

Local gates:
- `go test ./internal/auth ./internal/tenant ./cmd/api ./cmd/oa -run 'Test(Auth|Login|Session|Token|Tenant|Security)' -count=1`
- `make test-cli-coverage`
- `golangci-lint run`
- frontend prepared checks if UI changes

### Slice 6: Plugin Runtime Hardening

Owner scope:
- `internal/plugin`
- plugin API handlers
- plugin CLI commands/tests
- plugin frontend manager tests only if needed

Implement:
- one incremental runtime isolation, supervision, or operator-control hardening
  improvement that is locally testable.

Local gates:
- `go test ./internal/plugin ./cmd/api ./cmd/oa -run 'Test(Plugin|Runtime|Package|Supervisor|Restart)' -count=1`
- `make test-cli-coverage`
- `make swagger` if API output changes
- `golangci-lint run`

## Integrator Slice

After one or more implementation slices land:

Owner scope:
- `README.md`
- `docs/DEVELOPMENT_STATUS.md`
- `docs/USE_CASE_COVERAGE.md`
- `docs/API.md`
- `docs/CLI.md`
- generated Swagger/OpenAPI files when API output changes
- `docs/status_docs_test.go` when status snippets need to be locked

Local gates:
- `go test -timeout=3m ./docs -count=1`
- `git diff --check`
- any focused gates from the implemented slices
- `make test-cli-coverage`

## Assignment Rules

- Keep each subagent on one write scope.
- Tell every subagent that other agents may be editing the repo and they must
  not revert unrelated changes.
- Prefer one commit per successful slice.
- Do not wait for full CI after every small patch; batch independent slices,
  then run the shared local gates and push once the batch is coherent.
- Treat deployment-only and externally blocked items as documented limitations,
  not locally complete features.
