# Open Accounting Development Status

> Last updated: 2026-05-29
> This is the current-state status document. Historical plan docs may be more optimistic than what is verified here.

## Status Definitions

| Status | Meaning |
|--------|---------|
| `Working` | Feature exists, is exercised by the current codebase, and is part of the verified local baseline. |
| `Beta` | Feature exists but still has meaningful product, workflow, or reliability gaps. |
| `Demo-only` | Good enough for seeded/demo flows, not yet a trustworthy production capability. |
| `Missing` | Not implemented to a useful degree yet, but not fundamentally blocked by an external dependency. |
| `Blocked` | Depends on external partnerships, certification, or major missing infrastructure. |

## Verified Engineering Baseline

Full local baseline last completed on 2026-05-28:

- `go test ./...` passes
- `golangci-lint run` passes
- `go test -count=1 -race -tags=integration $(go list ./... | grep -v /testutil)` passes against a fresh PostgreSQL database
- `cd frontend && bun run lint` passes
- `cd frontend && bun run check` passes with 0 errors and 0 warnings
- `cd frontend && bun run test` passes with 21 files and 493 tests
- `cd frontend && bun run build` passes
- `cd frontend && bun run test:e2e:smoke` passes against a fresh locally seeded demo environment
- `cd frontend && bunx playwright test --config=playwright.demo.config.ts --project=demo-chromium --workers=1` passes against a fresh locally seeded demo environment with 251 passed and 12 intentionally skipped reset tests under `CI=true`
- Frontend lint is now blocking in CI
- Backend integration tests are now blocking in CI
- Core accountant smoke E2E is now blocking in CI

Still not done:

- Broad demo E2E remains informational rather than a strict release gate
- Documentation outside this file may still contain historical planning language

## Capability Matrix

| Area | Status | Notes |
|------|--------|-------|
| Core ledger (accounts, journal entries, trial balance, balance sheet, income statement) | `Working` | Core accounting paths exist and are covered by passing backend tests. |
| Invoicing, contacts, payments, reminders, interest, recurring invoices | `Working` | Core SMB workflows are present and part of the verified baseline, including recurring invoice template CSV import, overdue-invoice reminders, automated reminder rules, and late-payment interest calculations. |
| Banking CSV import and reconciliation | `Working` | Manual import and reconciliation work, and reconciliation completion now blocks transactions explicitly marked `EVIDENCE_REQUIRED` until approved reconciliation evidence is attached. Direct bank feeds do not. |
| Payroll, leave management, TSD generation/export | `Working` | Payroll, leave balances/records, and TSD XML/CSV export exist; automatic submission does not. |
| KMD generation/export and historical import | `Working` | KMD generation/export and row-level historical KMD CSV import exist; direct e-MTA submission does not. |
| Quotes, orders, recurring invoices, fixed assets | `Working` | Features exist and have tests, including quote, order, and recurring invoice CSV imports for grouped legacy lines and fixed-asset CSV import for cutover numbers, status, and depreciation/book values. Accountant-grade polish is still limited. |
| Multi-tenant auth, RBAC, tenant isolation | `Working` | Core tenant model is in place; API startup now rejects missing or short production JWT secrets and missing production CORS origins, JWT access/refresh tokens are purpose-separated, refresh tokens are persisted as revocable single-use sessions with API/CLI listing and revocation, tenant user administration blocks owner-role assignment plus self role changes/removal, and tenant user/invitation administration plus audit events now have API/CLI coverage and settings UI visibility. Broader auth administration polish is still needed. |
| CLI and API token automation | `Working` | `cmd/oa` supports operational health/demo checks, auth registration/login/refresh/token bootstrap, tenant lifecycle/user/invitation administration, plugin and plugin-registry administration, token management, accounts, contacts, employees, payroll run lifecycle, payroll history, leave balances and leave records, TSD/KMD declarations, KMD history import, and exports, invoices, quotes/import, orders/import, recurring invoices/import, payments/import, payment reminders and reminder rules, email settings/templates/logs/send actions, interest settings/calculations/history, period close/year-end workflows, banking accounts, bank transaction import/matching/review/reconciliation, quotes, orders, recurring invoices, fixed assets and fixed-asset import, inventory categories/import, warehouses/import, products/import/stock import, cost centers/import, analytics, core reports, document upload/download/review-summary/review-queue/evidence-policy/retention/review workflows, opening-balance imports, and grouped historical journal imports using tenant-scoped API tokens. |
| Chart of accounts, contacts, employee, invoice, quote, order, recurring-invoice-template, payment, cost-center/product-category/warehouse/product/stock, fixed-asset, payroll-history, leave-balance, KMD-history, opening-balance, and historical-journal imports | `Working` | CSV imports exist for core setup and migration data, including employee master data plus recurring base salary setup, finalized payroll runs/payslips, leave balances, row-level historical KMD declarations, quote and order history with grouped lines and preserved numbers/statuses, recurring invoice templates with grouped lines and schedule/email settings, payment history with optional invoice allocation, cost center master data, product category master data, warehouse master data, product master data, signed stock adjustments, fixed assets with legacy numbers/status/cutover depreciation/book values, opening balances, and grouped balanced historical journal entries. Payroll-history, leave-balance, KMD-history, quote, order, recurring-invoice-template, payment, cost-center/product-category/warehouse/product/stock, fixed-asset, opening-balance, and historical-journal imports are exposed through API and CLI flows. |
| Report exports | `Working` | Trial balance, account balance, balance sheet, income statement, cash flow, aging, balance confirmation, and cost-center budget reports have backend CSV, XLSX, and PDF export through API and CLI, with shared invalid-format handling. |
| Cash flow reporting | `Working` | Present in code and UI with API/service validation for malformed and inverted periods plus direct-method and indirect-method classifications for cash/bank, receivables, inventory, payables, payroll, taxes, interest, depreciation/amortization, loans, share capital, dividends, and fixed assets. Tenant-level account-code mapping rules can be saved through API/CLI and request-level overrides can still force custom chart accounts for one-off reports. |
| Settings and admin workflows | `Beta` | Basic settings exist, including SMTP email configuration and templates, but production admin depth is still thin. |
| Period lock on core write paths | `Working` | Tenant `period_lock_date` blocks core back-dated writes across the main mutation paths and is exposed in the CLI close workflow. |
| Close/reopen workflow with audit trail | `Beta` | Explicit close and reopen actions exist in API, CLI, and company settings, with history, operator notes, auditable fiscal-year reviewer sign-off plus approved close-pack evidence on close, close-pack audit evidence metadata plus downloadable ZIP archive export, a safety block against reopening a year-end after carry-forward has been posted, and an explicit carry-forward reversal step for controlled corrections. |
| Fiscal year close checklist and carry-forward workflow | `Beta` | API, CLI, and company settings expose year-end readiness, year-end close packs with trial balance/balance sheet/income statement, retained-earnings mapping, reviewer sign-off before fiscal-year close, approved close-pack evidence collection/review in the UI, approved close-pack audit evidence metadata plus ZIP archive export with a company-settings download action, approved close-pack evidence enforcement before fiscal-year close and carry-forward, explicit carry-forward posting after year-end lock, and dated carry-forward reversal. |
| Invoice, journal-entry, payment, bank-transaction, asset, and year-end close document attachments | `Beta` | Files can be uploaded, listed, downloaded, deleted, reviewed, approved, and rejected for core accounting records, bank reconciliation evidence, fixed assets, and year-end close packs. Document type, retention date, explicit review/approval/rejection decisions, review notes, tenant-wide review queues with a dedicated UI, tenant-wide retention review with a dedicated UI, explicit evidence-policy evaluation, reconciliation completion enforcement, fixed-asset activation evidence enforcement, and close-pack enforcement now exist. Broader workflow-level policy enforcement is still incomplete. |
| Accountant review workspace | `Beta` | The dashboard now includes both a tenant review queue and a cross-tenant portfolio rollup for overdue invoices, banking exceptions, close pressure, and document-evidence follow-up on unmatched bank transactions. Accountants can now send overdue-invoice reminders and set bank-transaction follow-up states and review notes directly from the dashboard, but broader exception actions across other workflows are still missing. |
| Plugin marketplace | `Beta` | Registry, install, permission, tenant enablement, and settings flows exist. Backend hook/route runtime execution is not implemented and is now rejected explicitly instead of silently no-oping; frontend slots are metadata-only and render host fallback content until a frontend runtime exists. |
| Inventory and warehouse flows | `Beta` | Product/category/warehouse CRUD, CSV imports, signed stock adjustments, stock import, per-warehouse stock levels, warehouse transfers, reserve/release stock allocations, and standard-cost plus weighted-average inventory valuation exist through API and CLI. Fulfillment allocation and FIFO/layered costing remain thin. |
| Core accountant smoke E2E gate | `Working` | CI now blocks on auth setup plus invoices, reports, banking, and payroll route coverage. |
| Backup and restore drills | `Beta` | Operator scripts now create PostgreSQL custom-format backups with checksums, sync backup/checksum files to S3-compatible or rclone-managed offsite storage, restore backups into a separate drill database with core table and migration verification, and health-check backup freshness/checksums with optional Prometheus textfile output. Provider credentials and scheduler installation remain deployment responsibilities. |
| Demo seeded flows and broad view coverage | `Demo-only` | Useful for demos and regression checks, not the same as release-quality smoke coverage. |
| Historical payroll, KMD history, quote/order, recurring templates, payment, cost-center/product-category/warehouse/product/stock, fixed assets, historical journals, and broader incumbent-system migration imports | `Beta` | Employee master-data import, finalized historical payroll run/payslip CSV import, leave-balance CSV import, row-level KMD history import, quote/order history CSV import with grouped lines and preserved numbers/statuses, recurring invoice template CSV import with grouped lines and schedule/email settings, payment history CSV import with optional invoice allocation, cost center master CSV import, product category master CSV import, warehouse master CSV import, product master CSV import, signed stock adjustment CSV import, fixed-asset CSV import, and grouped historical journal-entry CSV import now exist. Full incumbent-system cutover paths are still missing. |
| Broader document retention and reconciliation evidence workflow | `Beta` | Reconciliation evidence can now be attached to bank transactions and assets, and close-pack evidence can be attached to deterministic year-end close entities, with document type, review status, approval/rejection decisions, review notes, retention metadata, tenant-wide retention review, API/CLI evidence-policy evaluation, a reconciliation-completion blocker for transactions marked `EVIDENCE_REQUIRED`, a fixed-asset activation blocker for missing approved asset evidence, and fiscal-year close/carry-forward blockers for missing approved close packs. Broader retention automation remains thin. |
| Direct bank feeds, SEPA initiation, e-invoice, OCR, automatic e-MTA submission | `Blocked` | Requires external partnerships, licensing, certification, or additional infrastructure. |

## What The Project Can Honestly Claim Today

- Open Accounting is a broad, real codebase with working accounting, invoicing, payroll, banking, and multi-tenant foundations.
- The local backend, frontend, tagged backend integration, smoke E2E, and full local demo E2E baselines were green in the last full local baseline on 2026-05-28.
- The project now includes a working Go CLI and tenant-scoped API tokens for scriptable reads and writes.
- Historical payroll run/payslip import, leave-balance import, KMD history import, quote/order history import, recurring invoice template import, payment history import, cost center/product category/warehouse/product master/stock import, fixed-asset import, and historical journal import are now available through API and CLI, but broader incumbent-system cutover is still incomplete.
- The project is still not production-ready for accounting firms that need full historical cutover tooling, year-end reversal/reopen tooling, document retention controls, and hardened operations.
- The strongest near-term wedge is Estonian SMB/accountant workflow with manual bank import, invoicing, payroll, KMD/TSD export, and core reporting.

## Immediate Priorities

1. Extend historical migration beyond payroll, tax history, quotes/orders/recurring templates, cost-center/product-category/warehouse/product/stock, and fixed assets into broader incumbent-system cutover imports.
2. Extend the new accountant portfolio rollup into deeper exception actions beyond overdue reminders and banking, including dedicated accounting follow-up workflows.
3. Add more automated workflow blocks on top of document evidence-policy evaluation beyond reconciliation, fixed-asset activation, and year-end close packs.
4. Add broader auth administration controls and remaining auth hardening.
5. Harden provider-specific offsite backup credentials and scheduled restore-drill execution in real deployments.

## Related Docs

- [Reliability and Product Roadmap](./plans/2026-03-12-reliability-and-product-roadmap.md)
- [Feature Mapping: Merit & SmartAccounts](./FEATURE_MAPPING_MERIT_SMARTACCOUNTS.md)
- [API Reference](./API.md)
- [CLI Guide](./CLI.md)
- [Deployment Guide](./DEPLOYMENT.md)
