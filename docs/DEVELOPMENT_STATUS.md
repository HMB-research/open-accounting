# Open Accounting Development Status

> Last updated: 2026-03-12
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

Local verification completed on 2026-03-12:

- `go test ./...` passes
- `go test -count=1 -race -tags=integration $(go list ./... | grep -v /testutil)` passes against a fresh PostgreSQL database
- `cd frontend && bun run test` passes with 15 files and 437 tests
- `cd frontend && bun run check` passes with 0 errors and 0 warnings
- `cd frontend && bun run test:e2e:smoke` passes against a fresh locally seeded demo environment
- Backend integration tests are now blocking in CI
- Core accountant smoke E2E is now blocking in CI

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
| CLI and API token automation | `Working` | `cmd/oa` supports token bootstrap, token management, accounts, contacts, invoices, and opening-balance imports using tenant-scoped API tokens. |
| Chart of accounts, contacts, invoice, and opening-balance imports | `Working` | CSV imports exist in API, web UI, and CLI for core setup and migration data. |
| Report exports | `Beta` | CSV/XLSX export exists, but the current path is mostly client-side and not yet authoritative. |
| Cash flow reporting | `Beta` | Present in code and UI, but needs more accountant-grade validation before stronger claims. |
| Settings and admin workflows | `Beta` | Basic settings exist, but production admin depth is still thin. |
| Period lock on core write paths | `Working` | Tenant `period_lock_date` blocks core back-dated writes across the main mutation paths. |
| Close/reopen workflow with audit trail | `Beta` | Explicit close and reopen actions exist in the API and company settings, with history and operator notes. Fiscal-year checklist/carry-forward work is still missing. |
| Plugin marketplace | `Beta` | Significant functionality exists, but it is not part of the primary product wedge for reliability. |
| Inventory and warehouse flows | `Beta` | Inventory structures exist, but the module is not yet complete enough to market as finished. |
| Core accountant smoke E2E gate | `Working` | CI now blocks on auth setup plus invoices, reports, banking, and payroll route coverage. |
| Demo seeded flows and broad view coverage | `Demo-only` | Useful for demos and regression checks, not the same as release-quality smoke coverage. |
| Employee and incumbent-system migration imports | `Missing` | Adoption gap remains for payroll history and broader historical cutover. |
| Fiscal year close checklist and carry-forward workflow | `Missing` | Hard requirement for trustworthy year-end operations beyond the current close/reopen controls. |
| Attachments and document workflows | `Missing` | Purchase invoice, receipt, and reconciliation evidence handling is still absent. |
| Direct bank feeds, SEPA initiation, e-invoice, OCR, automatic e-MTA submission | `Blocked` | Requires external partnerships, licensing, certification, or additional infrastructure. |

## What The Project Can Honestly Claim Today

- Open Accounting is a broad, real codebase with working accounting, invoicing, payroll, banking, and multi-tenant foundations.
- The local backend, frontend, and tagged backend integration test baselines are green as of 2026-03-12.
- The project now includes a working Go CLI and tenant-scoped API tokens for scriptable reads and writes.
- The project is still not production-ready for accounting firms that need broader migration imports, close controls, document retention, and hardened operations.
- The strongest near-term wedge is Estonian SMB/accountant workflow with manual bank import, invoicing, payroll, KMD/TSD export, and core reporting.

## Immediate Priorities

1. Implement employee and incumbent-system migration imports.
2. Finish fiscal-year close, carry-forward, and year-end checklist workflows on top of the new close/reopen foundation.
3. Add attachments and document storage for accounting records.
4. Remove insecure production defaults and add stronger session management.
5. Separate smoke vs broader demo E2E coverage more cleanly over time.

## Related Docs

- [Reliability and Product Roadmap](./plans/2026-03-12-reliability-and-product-roadmap.md)
- [Feature Mapping: Merit & SmartAccounts](./FEATURE_MAPPING_MERIT_SMARTACCOUNTS.md)
- [Deployment Guide](./DEPLOYMENT.md)
