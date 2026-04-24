# Open Accounting Development Status

> Last updated: 2026-04-24
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

Full local baseline last completed on 2026-03-13:

- `go test ./...` passes
- `go test -count=1 -race -tags=integration $(go list ./... | grep -v /testutil)` passes against a fresh PostgreSQL database
- `cd frontend && bun run test` passes with 21 files and 491 tests
- `cd frontend && bun run check` passes with 0 errors and 0 warnings
- `cd frontend && bun run test:e2e:smoke` passes against a fresh locally seeded demo environment
- Backend integration tests are now blocking in CI
- Core accountant smoke E2E is now blocking in CI

Targeted verification completed on 2026-04-24 after moving to Go 1.26.2 and adding historical payroll and leave-balance imports:

- `go test ./...` passes
- `cd frontend && bun run test -- api.test.ts` passes with 179 tests
- `cd frontend && bun run check` passes with 0 errors and 0 warnings
- Swagger docs were regenerated with `swag` built against Go 1.26.2

Still not done:

- Demo E2E remains informational rather than a strict release gate
- Documentation outside this file may still contain historical planning language

## Capability Matrix

| Area | Status | Notes |
|------|--------|-------|
| Core ledger (accounts, journal entries, trial balance, balance sheet, income statement) | `Working` | Core accounting paths exist and are covered by passing backend tests. |
| Invoicing, contacts, payments, recurring invoices | `Working` | Core SMB workflows are present and part of the verified baseline. |
| Banking CSV import and reconciliation | `Working` | Manual import and reconciliation work; direct bank feeds do not. |
| Payroll, TSD generation/export | `Working` | Payroll and TSD XML/CSV export exist; automatic submission does not. |
| KMD generation/export | `Working` | KMD generation/export exists; direct e-MTA submission does not. |
| Quotes, orders, fixed assets | `Working` | Features exist and have tests, but accountant-grade polish is still limited. |
| Multi-tenant auth, RBAC, tenant isolation | `Working` | Core tenant model is in place; auth hardening is still needed for production trust. |
| CLI and API token automation | `Working` | `cmd/oa` supports token bootstrap, token management, accounts, contacts, employees, payroll history, leave balances, invoices, document workflows, and opening-balance imports using tenant-scoped API tokens. |
| Chart of accounts, contacts, employee, invoice, payroll-history, leave-balance, and opening-balance imports | `Working` | CSV imports exist for core setup and migration data, including employee master data plus recurring base salary setup, finalized payroll runs/payslips, and leave balances. Payroll-history and leave-balance imports are exposed through API, web UI, and CLI flows. |
| Report exports | `Beta` | CSV/XLSX export exists, but the current path is mostly client-side and not yet authoritative. |
| Cash flow reporting | `Beta` | Present in code and UI, but needs more accountant-grade validation before stronger claims. |
| Settings and admin workflows | `Beta` | Basic settings exist, but production admin depth is still thin. |
| Period lock on core write paths | `Working` | Tenant `period_lock_date` blocks core back-dated writes across the main mutation paths. |
| Close/reopen workflow with audit trail | `Beta` | Explicit close and reopen actions exist in the API and company settings, with history, operator notes, and a safety block against reopening a year-end after carry-forward has been posted. |
| Fiscal year close checklist and carry-forward workflow | `Beta` | Company settings now expose year-end readiness, retained-earnings mapping, and an explicit carry-forward journal step after year-end lock. Reopen reversal and deeper year-end packs are still missing. |
| Invoice, journal-entry, payment, bank-transaction, and asset document attachments | `Beta` | Files can be uploaded, listed, downloaded, and deleted for core accounting records, bank reconciliation evidence, and fixed assets. Basic document type, retention date, and review metadata now exist, but full approval flow and admin retention controls are still missing. |
| Accountant review workspace | `Beta` | The dashboard now includes both a tenant review queue and a cross-tenant portfolio rollup for overdue invoices, banking exceptions, close pressure, and document-evidence follow-up on unmatched bank transactions. Accountants can now set bank-transaction follow-up states and review notes directly from the dashboard, but broader exception actions across other workflows are still missing. |
| Plugin marketplace | `Beta` | Significant functionality exists, but it is not part of the primary product wedge for reliability. |
| Inventory and warehouse flows | `Beta` | Inventory structures exist, but the module is not yet complete enough to market as finished. |
| Core accountant smoke E2E gate | `Working` | CI now blocks on auth setup plus invoices, reports, banking, and payroll route coverage. |
| Demo seeded flows and broad view coverage | `Demo-only` | Useful for demos and regression checks, not the same as release-quality smoke coverage. |
| Historical payroll and broader incumbent-system migration imports | `Beta` | Employee master-data import, finalized historical payroll run/payslip CSV import, and leave-balance CSV import now exist. Broader tax-history and full incumbent-system cutover paths are still missing. |
| Broader document retention and reconciliation evidence workflow | `Beta` | Reconciliation evidence can now be attached to bank transactions and assets with document type, review status, and retention metadata. Approval workflow, policy automation, and admin retention controls are still missing. |
| Direct bank feeds, SEPA initiation, e-invoice, OCR, automatic e-MTA submission | `Blocked` | Requires external partnerships, licensing, certification, or additional infrastructure. |

## What The Project Can Honestly Claim Today

- Open Accounting is a broad, real codebase with working accounting, invoicing, payroll, banking, and multi-tenant foundations.
- The local backend, frontend, and tagged backend integration test baselines were green in the last full local baseline on 2026-03-13.
- The project now includes a working Go CLI and tenant-scoped API tokens for scriptable reads and writes.
- Historical payroll run/payslip import and leave-balance import are now available through API, web UI, and CLI, but broader incumbent-system cutover is still incomplete.
- The project is still not production-ready for accounting firms that need full historical cutover tooling, year-end reversal/reopen tooling, document retention controls, and hardened operations.
- The strongest near-term wedge is Estonian SMB/accountant workflow with manual bank import, invoicing, payroll, KMD/TSD export, and core reporting.

## Immediate Priorities

1. Extend historical migration beyond payroll runs and leave balances into tax-history and broader incumbent-system cutover imports.
2. Add year-end reversal/reopen handling and fuller year-end packs on top of the new close/reopen and carry-forward foundation.
3. Extend the new accountant portfolio rollup into deeper exception actions beyond banking, including dedicated accounting follow-up workflows.
4. Add approval workflow and admin retention controls on top of the new reconciliation-evidence document layer.
5. Remove insecure production defaults and add stronger session management.

## Related Docs

- [Reliability and Product Roadmap](./plans/2026-03-12-reliability-and-product-roadmap.md)
- [Feature Mapping: Merit & SmartAccounts](./FEATURE_MAPPING_MERIT_SMARTACCOUNTS.md)
- [API Reference](./API.md)
- [CLI Guide](./CLI.md)
- [Deployment Guide](./DEPLOYMENT.md)
