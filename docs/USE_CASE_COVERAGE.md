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
- The follow-up migration progress telemetry stage was locally revalidated with focused cutover/model/API/CLI/frontend API and workbench tests plus prepared Svelte checks.
- The follow-up migration duration telemetry stage was locally revalidated with focused cutover/model/API/CLI/frontend API and workbench tests plus prepared Svelte checks.
- The follow-up migration accountant-workspace launch handoff stage was locally revalidated with focused cutover, generated OpenAPI, docs, frontend API, review-panel, workbench, and prepared Svelte checks.
- The follow-up migration saved-run event stream stage was locally revalidated with focused API/CLI route coverage, Swagger regeneration, docs status tests, lint, and the CLI coverage gate.
- The follow-up migration dashboard live stream stage was locally revalidated with focused frontend API/workbench tests and prepared Svelte checks.
- The follow-up migration provider preset catalog stage was locally revalidated with focused cutover/API/CLI/frontend API/workbench tests and prepared Svelte checks.
- The follow-up migration FK UUID preflight stage was locally revalidated with focused cutover validator tests, docs status tests, lint, and the CLI coverage gate.
- The follow-up product supplier-code migration stage was locally revalidated with focused inventory importer and cutover provider-alias tests, docs status tests, lint, and the CLI coverage gate.
- The follow-up fixed-asset supplier-code migration stage was locally revalidated with focused fixed-asset importer and cutover provider-alias tests, docs status tests, lint, and the CLI coverage gate.
- The follow-up supplier identity migration stage was locally revalidated with focused contact reference, product importer, fixed-asset importer, and cutover provider-alias tests, docs status tests, lint, and the CLI coverage gate.
- The follow-up payment and expense contact identity migration stage was locally revalidated with focused contact reference, payment importer, expense importer, and cutover provider-alias tests, docs status tests, lint, and the CLI coverage gate.
- The follow-up payment allocation consistency stage was locally revalidated with focused cutover validator tests, docs status tests, lint, and the CLI coverage gate.
- The follow-up e-invoice payment allocation consistency stage was locally revalidated with focused cutover validator tests, docs status tests, lint, and the CLI coverage gate.
- The follow-up payment allocation currency consistency stage was locally revalidated with focused cutover validator tests, docs status tests, lint, and the CLI coverage gate.
- The follow-up invoice paid-amount consistency stage was locally revalidated with focused cutover validator tests, docs status tests, lint, and the CLI coverage gate.
- The follow-up combined invoice paid/allocation consistency stage was locally revalidated with focused cutover validator tests, docs status tests, lint, and the CLI coverage gate.
- The follow-up payment allocation direction consistency stage was locally revalidated with focused cutover validator, CLI request, frontend workbench, docs status, lint, and CLI coverage gates.
- The follow-up payment invoice-contact consistency stage was locally revalidated with focused cutover validator tests, docs status tests, lint, and the CLI coverage gate.
- The follow-up e-invoice payment invoice-contact consistency stage was locally revalidated with focused cutover validator tests, docs status tests, lint, and the CLI coverage gate.
- The follow-up fixed-asset source-invoice consistency stage was locally revalidated with focused cutover validator tests, docs status tests, lint, and the CLI coverage gate.
- The follow-up fixed-asset source-invoice amount consistency stage was locally revalidated with focused cutover validator tests, docs status tests, lint, and the CLI coverage gate.
- The follow-up stock-adjustment product stockability stage was locally revalidated with focused cutover validator tests, docs status tests, lint, and the CLI coverage gate.
- The follow-up cost-allocation journal-line total consistency stage was locally revalidated with focused cutover validator tests, docs status tests, lint, and the CLI coverage gate.
- The follow-up cost-allocation journal-line percentage consistency stage was locally revalidated with focused cutover validator tests, docs status tests, lint, and the CLI coverage gate.
- The follow-up cost-allocation amount/percentage consistency stage was locally revalidated with focused cutover validator tests, docs status tests, lint, and the CLI coverage gate.
- The follow-up expense account-type consistency stage was locally revalidated with focused cutover validator tests, docs status tests, lint, and the CLI coverage gate.
- The follow-up product account-type consistency stage was locally revalidated with focused cutover validator tests, docs status tests, lint, and the CLI coverage gate.
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
| Quotes, orders, recurring invoices, expenses, and fixed assets | `Verified` | Quote/order import, PDF download, email delivery, quote-to-invoice, order-to-invoice, recurring invoice template import, expense import, receipt-backed approval/posting, expense remediation actions for receipt upload/review, approval/rejection, rejected-claim resubmission, ledger posting, archive follow-up with workspace assignment metadata, and dashboard completion for draft submission, submitted approval, and approved ledger-posting expense assignments, fixed-asset import with supplier identity lookup, depreciation posting, and disposal posting. | Focused expense remediation service/API/CLI tests, focused frontend API/review-panel tests, focused backend tests, seeded demo E2E, generated OpenAPI docs, API docs, CLI docs, and PR #62 CI. | Broader accountant-assigned execution polish is still limited in some workflow surfaces. |
| Inventory and warehouses | `Verified` | Product/category/warehouse CRUD, imports, stock adjustments, stock import with lot metadata, serialized stock import guards, warehouse stock levels, cost-preserving lot/serial/expiry transfers with source-lot quantity validation, lot-aware reservation allocation and release, lot-aware issue allocation with lot, weighted-average, or standard-cost issue costing plus accounting-ready or transactionally posted COGS journal lines, tenant-level issue costing and valuation policy controls, pick lists, lot reports, standard-cost/weighted-average/FIFO valuation, inventory subledger reconciliation against posted GL balances, frontend reconciliation drill-down with account/product exceptions, fiscal-year close inventory costing review with blocking exception checks, and close remediation actions for inventory costing blockers. | Backend tests, integration gates, API docs, CLI docs, migration tests, migration validator tests, focused frontend API unit tests, prepared frontend checks, targeted seeded demo E2E inventory coverage, and focused close remediation tests. | Broader accountant-assigned remediation outside close and inventory can still deepen. |
| Historical migration and cutover | `Partial` | Chart of accounts, contacts, employees, invoices, quotes, orders, recurring templates, payments, expenses, e-invoice XML, banking, cost centers, cost allocations, product categories, warehouses, products, stock, fixed assets, payroll history, leave balances, TSD/KMD history, opening balances, historical journals, grouped migration remediation actions for ready bundles, unsupported file kinds, missing columns, missing references, duplicate identifiers, grouped consistency failures, malformed IDs, invalid row values, warning review, workspace queue assignment, stable assignment keys, priorities, and due windows, plus dependency-aware execution plans for ready bundles with API/CLI import steps, missing-context markers for bank-transaction and opening-balance imports, guarded CLI plus server-side API execution for fully ready plans, resume snapshots that skip previously succeeded steps when retrying interrupted runs, saved server-side execution run snapshots with list/get APIs, CLI access, status counters, progress percentages, active-step telemetry, per-step timestamps, and duration totals, saved-run event stream API/CLI access, provider preset catalog discovery for generic/Merit/SmartAccounts/Directo mapping metadata, dashboard live stream consumption, resume-by-ID support, accountant-workspace saved-run assignment handoff with deep links into failed/running/blocked/confirmation runs, supplier identity cross-file references by code, registry code, VAT number, email, or name, payment and expense contact identity cross-file references by code, registry code, VAT number, email, or name, invoice `amount_paid` consistency against imported invoice CSV totals and statuses, combined imported invoice paid amount/payment allocation totals, payment allocation totals against imported invoice CSV and e-invoice XML totals, payment allocation currency consistency against imported invoice CSV and e-invoice XML currencies, payment allocation direction consistency against imported invoice CSV and effective e-invoice XML invoice types, ambiguous invoice-number reference checks, fixed-asset source-invoice purchase-type, supplier, and amount-total consistency, stock-adjustment product stockability against same-bundle product type and tracking flags, and expense/product account-type consistency against same-bundle chart-of-account rows, and a dashboard migration workbench for bundle assembly, provider preset selection, validation, execution planning, saved dry runs, confirmed execution, saved-run monitoring with live event updates, progress/active-step/duration display, and resume-by-ID selection. | Migration bundle validator tests, focused migration remediation, execution-plan, guarded CLI execution, server-side execution, resume-aware execution, saved execution-run cutover/model/API/CLI/frontend API tests, focused migration workbench component tests, focused migration progress and duration telemetry tests, focused migration accountant-workspace handoff tests, focused migration dashboard live stream tests, focused migration provider preset catalog tests, focused migration FK UUID preflight tests, focused product supplier-code migration tests, focused fixed-asset supplier-code migration tests, focused supplier identity migration tests, focused payment and expense contact identity migration tests, focused payment allocation consistency migration tests, focused e-invoice payment allocation consistency migration tests, focused payment allocation currency consistency migration tests, focused invoice paid-amount consistency migration tests, focused combined invoice paid/allocation consistency migration tests, focused payment allocation direction consistency migration tests, focused fixed-asset source-invoice consistency migration tests, focused fixed-asset source-invoice amount consistency migration tests, focused stock-adjustment product stockability migration tests, focused product account-type consistency migration tests, prepared Svelte checks, payment bank-account and provider journal-line/cost-allocation cross-reference tests, Merit/SmartAccounts payment, bank-data, expense, cost-allocation, inventory, fixed-asset, and KMD-history alias tests, Directo commercial/bank/journal/payroll/inventory/tax alias tests, import tests, CLI coverage gates, API docs, CLI docs, generated OpenAPI docs, and PR #62 CI. | Further provider-specific mapping depth, additional cross-file validation, and dashboard-side mutating cutover controls remain open. |
| Document attachments, retention, and evidence policy | `Partial` | Upload/list/download/delete/review/approve/reject, retention metadata, review queues, retention review, retention reminder actions, scheduled retention reminder digest delivery with configurable retry/escalation controls, evidence policy checks, document remediation actions for missing retention, due-soon/expired retention, pending/rejected reviews, missing evidence, unapproved evidence, and evidence-policy violations with workspace assignment metadata, and workflow blockers for reconciliation, assets, purchase invoices, journal entries, payments, expenses, leave records, and close packs. | Backend tests, scheduler tests, focused document remediation service/API/CLI tests, generated OpenAPI docs, API docs, CLI docs, and docs status checks. | Broader workflow-level policy enforcement remains incomplete. |
| Close, reopen, year-end, and carry-forward controls | `Partial` | Period close/reopen, audit history, fiscal-year reviewer sign-off, close packs, approved close-pack evidence, fiscal-year inventory costing review, machine-readable remediation actions for period-close, close-pack evidence, retained earnings, inventory costing, already-posted carry-forward, and carry-forward posting with workspace assignment metadata, ZIP export, carry-forward posting, carry-forward reversal, dashboard assignment queue visibility for close actions, and direct dashboard completion for fiscal-year close and carry-forward posting assignments. | Backend tests, focused accounting/API/CLI close remediation tests, generated OpenAPI docs, CLI docs, frontend API type checks, targeted accountant workspace assignment queue tests, focused close assignment completion tests, prepared Svelte checks, and status docs. | Broader accountant-assigned close correction polish remains deeper than direct close/carry-forward assignment completion. |
| Accountant review workspace | `Partial` | Tenant review queue, cross-tenant portfolio rollup, overdue invoice actions, bank follow-up states with remediation actions, journal evidence follow-up, document-review links, pending-document assignment approval, calculated payroll-run payment-date setting and approval, approved payroll-run TSD generation, KMD regeneration/XML export assignment actions, draft expense submission, submitted expense approval, approved-expense ledger posting, fiscal-year close, and carry-forward posting directly from the dashboard, a dashboard assignment queue that aggregates close, banking, document-retention, expense-claim, payroll-run, TSD, KMD, and migration cutover remediation actions with tenant-scoped deep links plus CLI commands, and cross-tenant portfolio assignment counts/links. Close, migration preflight/execution planning, KMD, TSD declaration, payroll run, expense claim, banking, and document remediation data remain available through API/CLI with workspace queues, stable assignment keys, priorities, and due windows. | Frontend/backend tests, smoke E2E, seeded demo E2E, focused close/migration/KMD/TSD/payroll/banking/expense/document remediation tests, targeted accountant review assignment queue tests, focused payroll payment-date assignment, close, KMD assignment execution, and migration saved-run handoff tests, prepared Svelte checks, and status docs. | Deeper executable workspace assignment for mutating migration execution beyond saved-run launch handoff and remaining non-KMD tax/payroll/document edges remains missing. |
| Settings, admin, backup, and restore operations | `Partial` | SMTP settings, templates, production startup checks, backup creation, offsite sync wrapper, backup health metrics, restore-drill wrapper, restore-drill metrics, structured restore-drill failure codes, systemd schedule template generation, and scheduled-drill runbook guidance. | Backend tests, CLI coverage gates, deployment docs, and status docs. | Provider credential provisioning and host-specific timer enablement still need deployment hardening. |
| Plugins, webhooks, and external integrations | `Partial` | Plugin registry/install/permission/enablement/settings flows, webhook subscriptions, signed delivery, test delivery, and delivery history. | Backend tests, API docs, CLI docs, and status docs. | Plugin backend route/hook runtime execution and frontend slot runtime remain incomplete. |
| Direct bank feeds, direct SEPA initiation, e-invoice operator exchange, OCR, and automatic authority filing | `Blocked` | Manual export/import alternatives exist for several flows. | Documented as blocked in status and roadmap docs. | Requires external partnerships, licensing, certification, or additional infrastructure. |

Historical migration coverage also includes same-field payment invoice-contact
consistency for imported invoice CSV and e-invoice XML allocations, including
customer-mode sales e-invoices where the payment contact must match the buyer
party rather than the seller party. Fixed-asset source-invoice consistency now
requires same-bundle source invoices to be purchase invoices and rejects
same-field supplier/contact mismatches before cutover import execution.
Fixed-asset source-invoice amount consistency also rejects same-bundle asset
purchase costs that exceed the imported source invoice total.
Stock-adjustment product stockability also rejects same-bundle stock rows for
service products or products with `track_inventory=false`.
Cost-allocation journal-line total consistency also rejects same-bundle
allocation totals that exceed the referenced historical journal line debit or
credit amount. The focused cost-allocation journal-line total consistency migration tests
cover both accepted split allocations and over-allocated journal lines.
Cost-allocation journal-line percentage consistency also rejects same-bundle
allocation percentage totals above 100 percent for one historical journal line.
The focused cost-allocation journal-line percentage consistency migration tests
cover both exact 100 percent splits and over-allocated percentage splits.
Cost-allocation amount/percentage consistency also rejects same-bundle rows
where `amount` and `allocation_percentage` disagree with the referenced
historical journal line amount. The focused cost-allocation amount/percentage consistency migration tests cover both matching and mismatched rows.
Expense account-type consistency also rejects same-bundle expense rows where
`expense_account_code` does not reference an `EXPENSE` account or
`payment_account_code` does not reference an `ASSET` or `LIABILITY` account.
The focused expense account-type consistency migration tests cover accepted
expense/payment account combinations and rejected type mismatches.
Product account-type consistency also rejects same-bundle product rows where
`sale_account_code` does not reference a `REVENUE` account,
`purchase_account_code` does not reference an `EXPENSE` account, or
`inventory_account_code` does not reference an `ASSET` account. The focused
product account-type consistency migration tests cover accepted product account
combinations and rejected type mismatches.

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
2. Complete the remaining historical cutover mutating orchestration polish, further provider-specific mapping depth, additional cross-file validation, and deeper dashboard cutover controls.
3. Expand document policy enforcement beyond the current workflow blockers and scheduled retention reminder delivery controls.
4. Add deeper executable accountant workspace exception actions for mutating migration execution beyond saved-run launch handoff and remaining non-KMD tax/payroll/document edges.
5. Finish auth administration and operational hardening for accounting-firm pilots.
6. Harden provider-specific backup credential provisioning and host-specific timer enablement.
7. Keep replacing uncovered migration validator branches with focused tests until the use-case coverage evidence is no longer mostly indirect.
8. Keep externally blocked direct bank, e-invoice, OCR, SEPA, and authority filing tracks documented separately from locally implementable work.
