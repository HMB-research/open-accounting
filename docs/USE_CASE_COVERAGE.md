# Use Case Coverage Matrix

Last reviewed: 2026-06-13

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

- PR #62 on `feat/payroll-history-import` was green at commit `50b6b13` in CI run `27460180928`.
- `make test-cli-coverage` verifies `cmd/oa` at 100.0% statement coverage.
- `make test-backend-coverage` enforces the same CLI coverage from the backend coverage gate.
- `go test -timeout=3m ./docs -count=1` keeps the documentation status and route coverage checks active.
- Backend integration tests, smoke E2E, and four seeded demo E2E shards are blocking CI gates.
- The optional hosted-demo E2E job remains informational because it requires external demo URLs and secrets.

## Matrix

| Use case family | Status | Covered workflows | Evidence | Remaining gap |
| --- | --- | --- | --- | --- |
| Multi-tenant auth, RBAC, and API-token automation | `Verified` | Registration/login, token bootstrap, refresh-session revocation, tenant user/invitation administration, suspension/restoration, audit visibility, and tenant-scoped API-token use. | Backend tests, CLI coverage gates, API docs, CLI docs, and PR #62 CI. | Broader auth administration polish remains tracked as product hardening. |
| Core ledger and accounting reports | `Verified` | Accounts, grouped account hierarchy, journal entries, templates, recurring journal generation, trial balance, balance sheet, income statement, consolidated reports, annual reports, and CSV/XLSX/PDF exports. | Backend tests, integration gates, API route documentation checks, CLI guide, and seeded demo E2E coverage. | Accountant-grade report auditability and edge-case validation can still deepen. |
| Invoicing, purchases, contacts, payments, reminders, and interest | `Verified` | Sales invoices, purchase invoices, contacts, payment import, payment reversal through offsets, reminders, reminder rules, late-payment interest, e-invoice XML import, and receipt/evidence blockers where implemented. | Backend tests, API docs, CLI docs, smoke E2E, seeded demo E2E, and migration validator tests. | Direct e-invoice operator exchange remains blocked by external dependencies. |
| Banking and reconciliation | `Verified` | Bank accounts, CSV and camt.053 imports, statement account/currency validation, transaction matching, auto-match rules, review states, reconciliation, SEPA payment-file export, and evidence-required reconciliation blocking. | Integration gates, migration validator tests, API docs, CLI docs, and demo E2E. | Direct bank feeds and direct SEPA initiation are blocked external tracks. |
| Payroll, leave, and TSD | `Verified` | Employees, salary components, payroll runs, payslips, payroll history import, leave balances, leave records, TSD declarations, TSD exports, and TSD history import. | `go test -tags=integration ./internal/payroll -count=1`, backend tests, CLI coverage gates, docs tests, and PR #62 CI. | Automatic e-MTA submission remains blocked by external certification/integration work. |
| KMD, VAT, INF, and EU OSS | `Verified` | KMD generation/export, KMD INF A/B, quarterly EU VAT OSS reporting, KMD history import, and migration preflight validation for KMD history rows. | Backend tests, migration validator tests, API docs, CLI docs, and CI. | Direct e-MTA submission remains blocked. |
| Quotes, orders, recurring invoices, expenses, and fixed assets | `Verified` | Quote/order import, PDF download, email delivery, quote-to-invoice, order-to-invoice, recurring invoice template import, expense import and receipt-backed approval/posting, fixed-asset import, depreciation posting, and disposal posting. | Focused backend tests, seeded demo E2E, API docs, CLI docs, and PR #62 CI. | Accountant-grade polish is still limited in some workflow surfaces. |
| Inventory and warehouses | `Partial` | Product/category/warehouse CRUD, imports, stock adjustments, stock import with lot metadata, serialized stock import guards, warehouse stock levels, transfers, reservations, pick lists, lot reports, and standard-cost/weighted-average/FIFO valuation. | Backend tests, integration gates, API docs, CLI docs, and migration validator tests. | Full lot-ledger allocation and lot-specific costing layers remain thin. |
| Historical migration and cutover | `Partial` | Chart of accounts, contacts, employees, invoices, quotes, orders, recurring templates, payments, expenses, e-invoice XML, banking, cost centers, cost allocations, product categories, warehouses, products, stock, fixed assets, payroll history, leave balances, TSD/KMD history, opening balances, and historical journals. | Migration bundle validator tests, Merit/SmartAccounts inventory and fixed-asset alias tests, import tests, CLI coverage gates, API docs, CLI docs, and PR #62 CI. | Full incumbent-system cutover paths, broader vendor mapping presets, and deeper cross-file validation remain open. |
| Document attachments, retention, and evidence policy | `Partial` | Upload/list/download/delete/review/approve/reject, retention metadata, review queues, retention review, retention reminder actions, scheduled retention reminder digest delivery with configurable retry/escalation controls, evidence policy checks, and workflow blockers for reconciliation, assets, purchase invoices, journal entries, payments, expenses, leave records, and close packs. | Backend tests, scheduler tests, API docs, CLI docs, and docs status checks. | Broader workflow-level policy enforcement remains incomplete. |
| Close, reopen, year-end, and carry-forward controls | `Partial` | Period close/reopen, audit history, fiscal-year reviewer sign-off, close packs, approved close-pack evidence, ZIP export, carry-forward posting, and carry-forward reversal. | Backend tests, API docs, CLI docs, and status docs. | More close exception reporting and operator guidance are still needed. |
| Accountant review workspace | `Partial` | Tenant review queue, cross-tenant portfolio rollup, overdue invoice actions, bank follow-up states, journal evidence follow-up, and document-review links. | Frontend/backend tests, smoke E2E, seeded demo E2E, and status docs. | Deeper exception actions across tax, payroll, close, documents, and migration follow-up remain missing. |
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

1. Complete the remaining historical cutover paths and vendor-specific mapping presets.
2. Expand document policy enforcement beyond the current workflow blockers and scheduled retention reminder delivery controls.
3. Add deeper accountant workspace exception actions for tax, payroll, close, document, and migration follow-up.
4. Finish auth administration and operational hardening for accounting-firm pilots.
5. Harden provider-specific backup credential provisioning and host-specific timer enablement.
6. Keep replacing uncovered migration validator branches with focused tests until the use-case coverage evidence is no longer mostly indirect.
7. Keep externally blocked direct bank, e-invoice, OCR, SEPA, and authority filing tracks documented separately from locally implementable work.
