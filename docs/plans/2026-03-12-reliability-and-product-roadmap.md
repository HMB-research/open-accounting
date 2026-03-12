# Reliability and Product Roadmap

**Date:** 2026-03-12

**Goal:** Turn Open Accounting from a broad demo into a trustworthy accounting product with a clear wedge, a green main branch, and a realistic path to adoption.

**Status update later on 2026-03-12:**

- backend unit tests are green locally
- backend tagged integration tests are green locally
- frontend unit tests are green locally
- backend integration tests are now blocking in CI
- documentation truthfulness cleanup is in place
- blocking smoke E2E now covers auth setup, invoices, reports, banking, and payroll in CI

## Product Decision

**Primary wedge for the next 90 days:**

Estonian SMB and accountant workflow with:
- multi-tenant company management
- invoicing and payments
- manual bank import and reconciliation
- payroll
- KMD and TSD generation/export
- core financial reporting

**De-prioritize for the first 90 days:**
- plugin marketplace expansion
- new showcase modules unless they close a major adoption gap
- direct bank feeds
- SEPA payment initiation
- automatic e-MTA submission
- OCR
- broad "embedded accounting platform" positioning

## Current State

The repository has substantial breadth, but trust and product fit are the main blockers.

### Reliability blockers

- Documentation makes stronger completion claims than the codebase supports.

### Product blockers

- No migration/import story strong enough for real adoption.
- Missing fiscal-year close and carry-forward workflows.
- Missing document and attachment workflows.
- Security and operations are below accounting-product expectations.
- Several parity features are still blocked by external partnerships or compliance requirements.

## 90-Day Outcomes

By day 90, Open Accounting should be able to credibly claim:

- main branch stays green
- setup is reproducible
- core accountant workflow is usable without demo-only shortcuts
- feature claims match actual implementation status
- production deployment has minimum operational safety

It should not claim full parity with SmartAccounts or Merit by day 90.

## 30 / 60 / 90 Day Plan

### Days 0-30: Trust Baseline

**Theme:** Stop lying to yourselves. Make the repo green, measurable, and internally consistent.

**Workstreams**

1. Reliability
- fix backend red tests
- fix frontend red tests
- remove flaky or misleading test assumptions
- make local verification commands part of the contributor workflow

2. CI hardening
- make backend integration tests blocking
- make frontend unit tests blocking
- keep E2E split into smoke and demo suites
- make smoke E2E blocking for core paths

3. Documentation correction
- replace inflated completion claims with a capability matrix
- split status into `working`, `beta`, `demo-only`, `blocked`
- update README positioning to match the chosen wedge

4. Product scope control
- freeze net-new modules unless they support the wedge
- define "must-have for accountant workflow" vs "later parity"

**Exit criteria**

- `go test ./...` passes locally and in CI
- `bun run check` and `bun run test` pass locally and in CI
- one blocking smoke E2E suite covers login, tenant selection, invoices, reports, banking, payroll
- README and development-status docs no longer overstate coverage or completion

### Days 31-60: Useful Core Product

**Theme:** Remove the biggest adoption blockers for a real accountant or SMB.

**Workstreams**

1. Data import and migration
- opening balances import
- chart of accounts import
- contacts import
- invoices import
- employees import
- migration templates for Merit and SmartAccounts CSV exports

2. Accounting controls
- fiscal year settings
- period locking
- month-end close
- year-end close checklist
- reopen-with-audit-trail flow

3. Document workflows
- file attachments for purchase invoices, receipts, assets, and reconciliation evidence
- document list and preview per record
- storage abstraction for local/object storage

4. Accountant workflow
- exception queues for unmatched bank items
- overdue invoices review
- missing tax data review
- draft declaration review
- tenant-level task summary

5. Reporting quality
- server-side report generation for CSV/XLSX/PDF
- general ledger report
- improved AR and AP aging
- customer and vendor statements

**Exit criteria**

- new tenant can be onboarded with imports instead of demo seed assumptions
- period lock prevents back-dated changes without explicit privileged action
- documents can be attached to core records and survive restart/deploy
- accountant can complete a month-end flow without dropping to SQL or manual file hacks

### Days 61-90: Production Safety and Market Credibility

**Theme:** Make the product supportable in production for early adopters.

**Workstreams**

1. Security
- remove insecure auth defaults
- add refresh token rotation and revocation
- add password reset flow
- add audit log for authentication, settings, role changes, close/reopen actions
- define MFA design, even if implementation lands after day 90

2. Operations
- automated backups
- documented restore drill
- structured error reporting
- request and database metrics
- service health and runbook docs
- migration safety checks

3. Product polish
- reconcile mobile and accessibility gaps on core routes
- unify error handling and empty states
- improve onboarding copy and settings UX

4. Early-adopter readiness
- publish an honest feature matrix
- publish a self-hosting guide with backup and restore steps
- create sample accountant onboarding docs

**Exit criteria**

- backup and restore have been tested successfully
- privileged actions are auditable
- no insecure production defaults remain
- first design-partner users could run a pilot on the chosen wedge

## GitHub Issue Backlog

The list below is ordered by practical priority, not by implementation convenience.

### P0: Must Start Immediately

1. `fix(test): restore green backend suite`
- Scope: repair broken accounting tests and database panic cases
- Files: `internal/accounting/*`, `internal/database/*`
- Done when: `go test ./...` passes reliably

2. `fix(frontend-test): resolve generated i18n modules in Vitest`
- Scope: fix Paraglide import resolution and stabilize frontend unit test setup
- Files: `frontend/vitest.config.ts`, `frontend/src/tests/*`
- Done when: `bun run test` passes in CI and locally

3. `ci: make integration tests blocking`
- Scope: remove `continue-on-error` for backend integration tests after stabilization
- Files: `.github/workflows/ci.yml`
- Done when: failing integration tests fail the pipeline

4. `ci: add blocking smoke e2e suite for core accountant flow`
- Scope: separate smoke tests from demo-data regression tests
- Files: `frontend/e2e/*`, `.github/workflows/ci.yml`
- Done when: login -> tenant -> invoices -> reports -> banking -> payroll path is blocking

5. `docs: replace inflated completion claims with capability matrix`
- Scope: rewrite README and status docs to match reality
- Files: `README.md`, `docs/DEVELOPMENT_STATUS.md`
- Done when: no doc claims conflict with the current codebase or test status

6. `product: define supported wedge and freeze non-core expansion`
- Scope: create and adopt a product support matrix
- Files: `README.md`, `docs/DEVELOPMENT_STATUS.md`, `docs/plans/*`
- Done when: roadmap and issue triage follow one product direction

### P1: High-Value Product Work

7. `feat(import): opening balances import`
- Scope: import ledger opening balances safely with validation and preview
- Done when: a new tenant can start with imported balances without SQL

8. `feat(import): chart of accounts and contacts import`
- Scope: CSV import plus validation report
- Done when: accounting setup can be completed in under one hour for a small company

9. `feat(import): invoice and employee import`
- Scope: migrate operational history into the product
- Done when: pilot users can move active work, not just master data

10. `feat(accounting): period lock and reopen workflow`
- Scope: lock posting periods, enforce privileges, record audit trail
- Done when: users cannot accidentally mutate closed periods

11. `feat(accounting): fiscal year and month-end close workflow`
- Scope: year config, close checklist, close state, carry-forward rules
- Done when: finance users can operate a controlled close cycle

12. `feat(documents): attachments for invoices, receipts, assets, reconciliation`
- Scope: upload, download, preview, delete with audit metadata
- Done when: core accounting records can hold supporting documents

13. `feat(reports): server-side export pipeline`
- Scope: move critical report exports from browser-only generation to API-backed output
- Done when: CSV/XLSX/PDF exports are reproducible and testable

14. `feat(reports): general ledger and customer/vendor statements`
- Scope: reports accountants expect to use weekly
- Done when: accountant workflow no longer depends on manual data extraction

15. `feat(accountant): tenant exception dashboard`
- Scope: unmatched bank items, overdue invoices, missing tax data, pending declarations
- Done when: accountant can prioritize work across tenants from one view

### P1: Security and Ops

16. `security: remove insecure production defaults`
- Scope: fail startup without valid secrets in production mode
- Files: `cmd/api/main.go`
- Done when: the service cannot start in production with a default JWT secret

17. `security: refresh token rotation and revocation`
- Scope: persistent sessions, revocation list or session store, logout-all support
- Files: `internal/auth/*`, `frontend/src/lib/stores/auth.ts`
- Done when: stolen refresh tokens are controllable

18. `security: password reset flow`
- Scope: forgot password, reset token, expiry, abuse protection
- Done when: admin support is not required for basic account recovery

19. `security: audit log for privileged actions`
- Scope: auth events, settings changes, user role changes, close/reopen, declaration actions
- Done when: admins can answer who changed what and when

20. `ops: automated backups and restore verification`
- Scope: script or service integration plus documented restore test
- Files: `docs/DEPLOYMENT.md`, deploy config
- Done when: backups are not just documented, but exercised

21. `ops: observability baseline`
- Scope: request metrics, error metrics, DB pool metrics, structured logs
- Done when: production incidents are diagnosable without guesswork

### P2: Important But Not First

22. `feat(purchase): stronger purchase invoice workflow`
- Scope: draft, approval, attachment-first flow, supplier statement support

23. `feat(reporting): budget vs actual and cost center reporting`
- Scope: turn cost centers into a useful management tool

24. `ux: unify mobile and accessibility behavior on core routes`
- Scope: fix core navigation, tables, forms, and modals on mobile

25. `feat(accounting): recurring journal entries and entry templates`
- Scope: reduce manual bookkeeping workload

26. `feat(payroll): bulk payroll processing improvements`
- Scope: reduce per-employee manual work in payroll runs

### P3: External Dependency Track

These should be tracked, but not used as the main success criteria for the next 90 days.

27. `integration: Estonian e-invoice send/receive`
28. `integration: bank feed connections`
29. `integration: SEPA payment initiation`
30. `integration: automatic e-MTA submission`
31. `integration: annual filing / e-ariregister integration`
32. `integration: OCR purchase invoice capture`

## Suggested Milestone Structure

### Milestone 1: Green Main
- issues 1-6

### Milestone 2: Import and Close
- issues 7-12

### Milestone 3: Accountant Workflow
- issues 13-15, 22, 23

### Milestone 4: Production Safety
- issues 16-21

### Milestone 5: External Integrations
- issues 27-32

## Definition of Success

Open Accounting is "actually useful" when:

- a small Estonian company can onboard without SQL
- an accountant can run monthly operations across multiple tenants
- the product can survive normal production failure modes
- reported capabilities match actual capabilities
- early adopters trust it enough to pilot real workflows

It becomes "full featured" later. First it needs to become dependable.
