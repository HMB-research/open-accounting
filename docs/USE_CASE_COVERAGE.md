# Use Case Coverage Matrix

Last reviewed: 2026-06-14

This matrix tracks the active goal of testing every use case and keeping
proper documentation available. It complements
[DEVELOPMENT_STATUS.md](./DEVELOPMENT_STATUS.md), which remains the
authoritative current-state status page.

Status values:

| Status     | Meaning                                                                 |
| ---------- | ----------------------------------------------------------------------- |
| `Verified` | Workflow exists and is covered by the current local or CI test gates.   |
| `Partial`  | Workflow exists, but meaningful product, workflow, or test gaps remain. |
| `Blocked`  | Workflow depends on external certification, partners, or credentials.   |

## Current Evidence Baseline

- PR #62 on `feat/payroll-history-import` was green at commit `a7a0068` in CI run `27475857115`.
- `make test-cli-coverage` verifies `cmd/oa` at 100.0% statement coverage.
- `make test-backend-coverage` enforces the same CLI coverage from the backend coverage gate.
- `go test -timeout=3m ./docs -count=1` keeps the documentation status and route coverage checks active.
- The backend inventory subledger reconciliation stage was locally revalidated with focused API, service, CLI, output, docs, backend coverage, and integration coverage tests.
- The follow-up frontend inventory subledger reconciliation drill-down stage was locally revalidated with prepared Svelte checks, focused API unit coverage, prepared frontend build, and a targeted seeded demo E2E inventory spec.
- The follow-up close remediation actions stage was locally revalidated with focused accounting/API/CLI tests, Swagger regeneration, docs status tests, frontend API type checks, and the CLI coverage gate.
- The follow-up migration remediation actions stage was locally revalidated with focused cutover/API/CLI tests, Swagger regeneration, docs status tests, lint, and the CLI coverage gate.
- The follow-up migration remediation assignment stage was locally revalidated with focused cutover/API/CLI/frontend API tests, Swagger regeneration, docs status tests, lint, and the CLI coverage gate.
- The follow-up migration execution plan stage was locally revalidated with focused cutover/API/CLI/frontend API tests, Swagger regeneration, docs route coverage, and the CLI coverage gate.
- The follow-up server-side migration execution stage was locally revalidated with focused API handler, cutover/banking, CLI route-coverage, docs, Swagger, lint, and CLI coverage gates.
- The follow-up resume-aware migration execution stage was locally revalidated with focused cutover/API/CLI tests, docs updates, Swagger regeneration, lint, and the CLI coverage gate.
- The follow-up saved migration execution run stage was locally revalidated with focused cutover/model/API/CLI/frontend API tests, Swagger regeneration, docs, lint, CLI/backend/integration coverage gates, prepared frontend checks/tests/build, and migration `056` application through the integration gate.
- The follow-up migration workbench stage was locally revalidated with a focused `MigrationWorkbench.test.ts` component suite and prepared Svelte checks.
- The follow-up cross-workspace remediation assignment metadata stage was locally revalidated with focused workspace/accounting/banking/expenses/documents/payroll/tax/CLI tests and frontend API type coverage.
- The follow-up accountant workspace assignment queue stage was locally revalidated with prepared Svelte checks plus targeted review-panel, portfolio-panel, workspace-helper, and frontend API tests.
- The follow-up expense assignment queue stage was locally revalidated with focused review-panel, portfolio-panel, frontend API, prepared Svelte, docs, lint, CLI coverage, and backend coverage gates.
- The follow-up expense assignment completion stage was locally revalidated with focused review-panel and frontend API tests plus prepared Svelte checks.
- The follow-up document assignment completion stage was locally revalidated with focused review-panel tests and prepared Svelte checks.
- The follow-up payroll assignment approval stage was locally revalidated with focused review-panel tests and prepared Svelte checks.
- The follow-up payroll TSD assignment generation stage was locally revalidated with focused review-panel tests and prepared Svelte checks.
- The follow-up payroll payment-date assignment stage was locally revalidated with focused payroll service/API/CLI, review-panel, and frontend API tests plus prepared Svelte checks.
- The follow-up close assignment completion stage was locally revalidated with focused review-panel, workspace-helper, and frontend API tests plus prepared Svelte checks.
- The follow-up KMD assignment execution stage was locally revalidated with focused review-panel, workspace-helper, and frontend API tests plus prepared Svelte checks.
- The follow-up KMD tax remediation actions stage was locally revalidated with focused tax/API/CLI tests, Swagger regeneration, docs status tests, lint, and the CLI coverage gate.
- The follow-up payroll run remediation actions stage was locally revalidated with focused payroll/API/CLI tests, Swagger regeneration, frontend API type checks, docs status tests, lint, and the CLI coverage gate.
- The follow-up document retention and evidence remediation actions stage was locally revalidated with focused documents/API/CLI tests, Swagger regeneration, frontend API type checks, docs status tests, lint, and the CLI coverage gate.
- The follow-up banking remediation actions stage was locally revalidated with focused banking/API/CLI tests, Swagger regeneration, frontend API type checks, docs status tests, lint, and the CLI coverage gate.
- The follow-up expense claim remediation actions stage was locally revalidated with focused expenses/API/CLI tests, Swagger regeneration, docs status tests, lint, and the CLI coverage gate.
- Backend integration tests, smoke E2E, and four seeded demo E2E shards are blocking CI gates.
- The optional hosted-demo E2E job remains informational because it requires external demo URLs and secrets.

## Matrix

| Use case family | Status | Covered workflows | Evidence | Remaining gap |
| --- | --- | --- | --- | --- |
| Multi-tenant auth, RBAC, and API-token automation | `Verified` | Registration/login, token bootstrap, refresh-session revocation, tenant user/invitation administration, suspension/restoration, audit visibility, and tenant-scoped API-token use. | Backend tests, CLI coverage gates, API docs, CLI docs, and PR #62 CI. | Broader auth administration polish remains tracked as product hardening. |
| Core ledger and accounting reports | `Verified` | Accounts, grouped account hierarchy, journal entries, templates, recurring journal generation, trial balance, balance sheet, income statement, consolidated reports, annual reports, and CSV/XLSX/PDF exports. | Backend tests, integration gates, API route documentation checks, CLI guide, and seeded demo E2E coverage. | Accountant-grade report auditability and edge-case validation can still deepen. |
| Invoicing, purchases, contacts, payments, reminders, and interest | `Verified` | Sales invoices, purchase invoices, contacts, payment import, payment reversal through offsets, reminders, reminder rules, late-payment interest, e-invoice XML import, and receipt/evidence blockers where implemented. | Backend tests, API docs, CLI docs, smoke E2E, seeded demo E2E, and migration validator tests. | Direct e-invoice operator exchange remains blocked by external dependencies. |
| Banking and reconciliation | `Verified` | Bank accounts, CSV and camt.053 imports, statement account/currency validation, transaction matching, auto-match rules, review states, reconciliation, SEPA payment-file export, evidence-required reconciliation blocking, and bank transaction remediation actions for evidence-required, ready-to-match, unmatched, reconciliation-pending, reconciled archive, and unsupported status follow-up with workspace assignment metadata. | Focused banking remediation service/API/CLI tests, integration gates, migration validator tests, API docs, CLI docs, and demo E2E. | Direct bank feeds and direct SEPA initiation are blocked external tracks. |
| Payroll, leave, and TSD | `Verified` | Employees, salary components, payroll runs, payment-date updates for missing-date remediation, payroll run remediation actions for draft calculation, missing payment dates, zero-payslip review, approval, TSD generation, paid-run declaration follow-up, and declared payroll archive evidence with workspace assignment metadata, payslips, payroll history import, leave balances, leave records, TSD declarations, TSD exports, TSD history import, and TSD declaration remediation actions for empty rows/totals, draft export/submission, submitted declarations awaiting acceptance, missing submission timestamps, rejected declaration review, and accepted declaration archiving with workspace assignment metadata. | `go test -tags=integration ./internal/payroll -count=1`, focused payroll/TSD remediation service/API/CLI tests, backend tests, CLI coverage gates, docs tests, and PR #62 CI. | Automatic e-MTA submission remains blocked by external certification/integration work, and leave/document/payroll archive remediation can still deepen. |
| KMD, VAT, INF, and EU OSS | `Verified` | KMD generation/export, KMD INF A/B, quarterly EU VAT OSS reporting, KMD history import, migration preflight validation for KMD history rows, KMD remediation actions for empty VAT periods, payable/refund/zero declarations, submitted declarations awaiting acceptance, missing submission timestamps, and accepted declaration archiving with workspace assignment metadata, plus dashboard regeneration for empty KMD periods and XML export for actionable KMD review/archive assignments. | Backend tests, focused KMD remediation tax/API/CLI tests, migration validator tests, focused review-panel KMD assignment execution tests, generated OpenAPI docs, API docs, CLI docs, and CI. | Direct e-MTA submission remains blocked, and non-KMD tax remediation can still deepen. |
| Quotes, orders, recurring invoices, expenses, and fixed assets | `Verified` | Quote/order import, PDF download, email delivery, quote-to-invoice, order-to-invoice, recurring invoice template import, expense import, receipt-backed approval/posting, expense remediation actions for receipt upload/review, approval/rejection, rejected-claim resubmission, ledger posting, archive follow-up with workspace assignment metadata, and dashboard completion for draft submission, submitted approval, and approved ledger-posting expense assignments, fixed-asset import, depreciation posting, and disposal posting. | Focused expense remediation service/API/CLI tests, focused frontend API/review-panel tests, focused backend tests, seeded demo E2E, generated OpenAPI docs, API docs, CLI docs, and PR #62 CI. | Broader accountant-assigned execution polish is still limited in some workflow surfaces. |
| Inventory and warehouses | `Verified` | Product/category/warehouse CRUD, imports, stock adjustments, stock import with lot metadata, serialized stock import guards, warehouse stock levels, cost-preserving lot/serial/expiry transfers with source-lot quantity validation, lot-aware reservation allocation and release, lot-aware issue allocation with lot, weighted-average, or standard-cost issue costing plus accounting-ready or transactionally posted COGS journal lines, tenant-level issue costing and valuation policy controls, pick lists, lot reports, standard-cost/weighted-average/FIFO valuation, inventory subledger reconciliation against posted GL balances, frontend reconciliation drill-down with account/product exceptions, fiscal-year close inventory costing review with blocking exception checks, and close remediation actions for inventory costing blockers. | Backend tests, integration gates, API docs, CLI docs, migration tests, migration validator tests, focused frontend API unit tests, prepared frontend checks, targeted seeded demo E2E inventory coverage, and focused close remediation tests. | Broader accountant-assigned remediation outside close and inventory can still deepen. |
| Historical migration and cutover | `Partial` | Chart of accounts, contacts, employees, invoices, quotes, orders, recurring templates, payments, expenses, e-invoice XML, banking, cost centers, cost allocations, product categories, warehouses, products, stock, fixed assets, payroll history, leave balances, TSD/KMD history, opening balances, historical journals, grouped migration remediation actions for ready bundles, unsupported file kinds, missing columns, missing references, duplicate identifiers, grouped consistency failures, malformed IDs, invalid row values, warning review, workspace queue assignment, stable assignment keys, priorities, and due windows, plus dependency-aware execution plans for ready bundles with API/CLI import steps, missing-context markers for bank-transaction and opening-balance imports, guarded CLI plus server-side API execution for fully ready plans, resume snapshots that skip previously succeeded steps when retrying interrupted runs, saved server-side execution run snapshots with list/get APIs, CLI access, and resume-by-ID support, and a dashboard migration workbench for bundle assembly, validation, execution planning, saved dry runs, confirmed execution, saved-run monitoring, and resume-by-ID selection. | Migration bundle validator tests, focused migration remediation, execution-plan, guarded CLI execution, server-side execution, resume-aware execution, saved execution-run cutover/model/API/CLI/frontend API tests, focused migration workbench component tests, prepared Svelte checks, payment bank-account and provider journal-line/cost-allocation cross-reference tests, Merit/SmartAccounts payment, bank-data, expense, cost-allocation, inventory, fixed-asset, and KMD-history alias tests, import tests, CLI coverage gates, API docs, CLI docs, generated OpenAPI docs, and PR #62 CI. | Richer dashboard progress telemetry, accountant-workspace launch handoff, broader vendor mapping presets, and deeper cross-file validation remain open. |
| Document attachments, retention, and evidence policy | `Partial` | Upload/list/download/delete/review/approve/reject, retention metadata, review queues, retention review, retention reminder actions, scheduled retention reminder digest delivery with configurable retry/escalation controls, evidence policy checks, document remediation actions for missing retention, due-soon/expired retention, pending/rejected reviews, missing evidence, unapproved evidence, and evidence-policy violations with workspace assignment metadata, and workflow blockers for reconciliation, assets, purchase invoices, journal entries, payments, expenses, leave records, and close packs. | Backend tests, scheduler tests, focused document remediation service/API/CLI tests, generated OpenAPI docs, API docs, CLI docs, and docs status checks. | Broader workflow-level policy enforcement remains incomplete. |
| Close, reopen, year-end, and carry-forward controls | `Partial` | Period close/reopen, audit history, fiscal-year reviewer sign-off, close packs, approved close-pack evidence, fiscal-year inventory costing review, machine-readable remediation actions for period-close, close-pack evidence, retained earnings, inventory costing, already-posted carry-forward, and carry-forward posting with workspace assignment metadata, ZIP export, carry-forward posting, carry-forward reversal, dashboard assignment queue visibility for close actions, and direct dashboard completion for fiscal-year close and carry-forward posting assignments. | Backend tests, focused accounting/API/CLI close remediation tests, generated OpenAPI docs, CLI docs, frontend API type checks, targeted accountant workspace assignment queue tests, focused close assignment completion tests, prepared Svelte checks, and status docs. | Broader accountant-assigned close correction polish remains deeper than direct close/carry-forward assignment completion. |
| Accountant review workspace | `Partial` | Tenant review queue, cross-tenant portfolio rollup, overdue invoice actions, bank follow-up states with remediation actions, journal evidence follow-up, document-review links, pending-document assignment approval, calculated payroll-run payment-date setting and approval, approved payroll-run TSD generation, KMD regeneration/XML export assignment actions, draft expense submission, submitted expense approval, approved-expense ledger posting, fiscal-year close, and carry-forward posting directly from the dashboard, a dashboard assignment queue that aggregates close, banking, document-retention, expense-claim, payroll-run, TSD, and KMD remediation actions with tenant-scoped deep links plus CLI commands, and cross-tenant portfolio assignment counts/links. Close, migration preflight/execution planning, KMD, TSD declaration, payroll run, expense claim, banking, and document remediation data remain available through API/CLI with workspace queues, stable assignment keys, priorities, and due windows. | Frontend/backend tests, smoke E2E, seeded demo E2E, focused close/migration/KMD/TSD/payroll/banking/expense/document remediation tests, targeted accountant review assignment queue tests, focused payroll payment-date assignment, close, and KMD assignment execution tests, prepared Svelte checks, and status docs. | Deeper executable workspace assignment for mutating migration execution and remaining non-KMD tax/payroll/document edges remains missing. |
| Settings, admin, backup, and restore operations | `Partial` | SMTP settings, templates, production startup checks, backup creation, offsite sync wrapper, backup health metrics, restore-drill wrapper, restore-drill metrics, structured restore-drill failure codes, systemd schedule template generation, and scheduled-drill runbook guidance. | Backend tests, CLI coverage gates, deployment docs, and status docs. | Provider credential provisioning and host-specific timer enablement still need deployment hardening. |
| Plugins, webhooks, and external integrations | `Partial` | Plugin registry/install/permission/enablement/settings flows, webhook subscriptions, signed delivery, test delivery, and delivery history. | Backend tests, API docs, CLI docs, and status docs. | Plugin backend route/hook runtime execution and frontend slot runtime remain incomplete. |
| Direct bank feeds, direct SEPA initiation, e-invoice operator exchange, OCR, and automatic authority filing | `Blocked` | Manual export/import alternatives exist for several flows. | Documented as blocked in status and roadmap docs. | Requires external partnerships, licensing, certification, or additional infrastructure. |

## Stage Gates To Keep Current

Run these before claiming a broad goal stage is complete:

```sh
golangci-lint run
go test -timeout=3m ./docs -count=1
make test-cli-coverage
make test-backend-coverage
DATABASE_URL=postgres://openaccounting:openaccounting@localhost:5432/openaccounting?sslmode=disable make test-integration-coverage
cd frontend && bun run lint
cd frontend && bun run check:prepared
cd frontend && bun run test:prepared
cd frontend && bun run build:prepared
cd frontend && bun run test:e2e:smoke
cd frontend && bun run test:e2e
```

For focused stages, run the smallest relevant subset first, then the shared
gates that protect the changed surface.

## Open Goal Work Items

The following items still prevent the project from honestly claiming fully
working, production-ready accounting software:

1. Extend remediation actions beyond year-end close, migration preflight, KMD tax, TSD declarations, payroll runs, banking transactions, expense claims, and document evidence/retention into remaining non-KMD tax/payroll edges and accountant-workspace follow-up surfaces.
2. Complete the remaining historical cutover dashboard progress telemetry, accountant-workspace launch handoff, mutating orchestration polish, and vendor-specific mapping presets.
3. Expand document policy enforcement beyond the current workflow blockers and scheduled retention reminder delivery controls.
4. Add deeper executable accountant workspace exception actions for mutating migration execution and remaining non-KMD tax/payroll/document edges.
5. Finish auth administration and operational hardening for accounting-firm pilots.
6. Harden provider-specific backup credential provisioning and host-specific timer enablement.
7. Keep replacing uncovered migration validator branches with focused tests until the use-case coverage evidence is no longer mostly indirect.
8. Keep externally blocked direct bank, e-invoice, OCR, SEPA, and authority filing tracks documented separately from locally implementable work.
