# Current Product Limits

Last reviewed: 2026-08-05

This is the short current-state gap document for Open Accounting. It separates
what the current code and verification gates support from what still prevents
the repository from being described as fully featured, production-ready
accounting software.

## Status Key

- ✅ **Implemented and verified** — present in the current codebase and covered
  by the documented test or CI evidence.
- ⚠️ **Partial** — usable capabilities exist, but meaningful workflow or
  product-depth gaps remain.
- ☐ **Needs work** — still required for full product parity or operational
  readiness.
- 🚫 **Blocked** — requires external certification, partners, credentials, or
  infrastructure before it can be completed locally.

## Current Verified Cap

Open Accounting can honestly claim a broad self-hosted Estonian SMB/accountant
workflow today:

- ✅ Core double-entry accounting, invoicing, purchase invoices, quotes,
  orders, recurring invoices, fixed assets, expenses, inventory, reports, and
  payment workflows.
- ✅ Manual bank import and reconciliation, SEPA payment-file export, payroll,
  leave records, KMD/TSD export-oriented compliance, and manual Estonian
  e-invoice XML import.
- ✅ Evidence blockers, document lifecycle/retention workflows, accountant
  review queues, migration validation/execution planning, and saved migration
  runs with progress, events, and resume support.
- ✅ A Go operator CLI, tenant-scoped API tokens, and a Svelte dashboard with
  tenant settings, review, tax, banking, payroll, and migration workbench
  flows.

Current gate evidence is maintained in
[Development Status](./DEVELOPMENT_STATUS.md). That evidence is not a
production-readiness claim; it proves the checked use cases and regression gates
for the current branch.

## Gaps From A Full-Featured Product

| Area | ✅ Implemented / verified today | ☐ Needs work for full product parity |
| --- | --- | --- |
| Core accounting and SMB workflows | ✅ Core ledger, journal templates, recurring journals, reports, invoices, purchases, contacts, quotes, orders, recurring invoices, fixed assets, expenses, inventory, reminders, interest, and auditable payment correction exist with backend, CLI, UI, and workflow evidence where applicable. Payment create/import/allocation/reversal updates are atomic and invoice payment updates are row-locked. | ☐ Accountant-grade report auditability, edge-case validation, and deeper workflow polish remain. |
| Tenant administration and settings | ✅ Multi-tenant auth, RBAC, API tokens, sessions, invitations, tenant administration, organization settings, and the Company Settings API/UI route are implemented. The tenant detail GET/PUT route regression is covered so the old 404 failure cannot silently return. | ☐ Broader authentication hardening and administration polish remain before enterprise production readiness. |
| Banking and payments | ✅ Manual CSV and camt.053 imports, matching, persisted auto-match rules, reconciliation, evidence-required blockers, remediation queues, and SEPA pain.001 payment-file export exist. | ☐ Direct bank feeds, direct SEPA initiation, and partner-managed payment submission remain external tracks. |
| Payroll, tax, and compliance exports | ✅ Payroll runs, leave records, payslips, payroll/TSD history import, TSD XML/CSV export, KMD generation/export/history import, KMD INF, EU VAT OSS, local submitted/accepted status tracking, and approved evidence gates exist. | ☐ Automatic e-MTA submission is blocked by external certification/integration work. Leave/document/payroll archive remediation and local filing workflow depth can still improve. |
| Historical migration and cutover | ✅ CSV/XML imports, generic/Merit/SmartAccounts/Directo provider aliases, cross-file validation, migration remediation, dependency-aware execution plans, guarded API/CLI execution, saved runs, progress/events, resume-by-ID, and dashboard workbench flows exist. | ☐ Deeper provider-specific mapping, broader cross-file validation outside the current coverage, and additional dashboard-side mutating cutover controls are still needed. |
| Accountant workspace execution | ✅ Review queues, cross-tenant portfolio rollups, and direct dashboard actions cover overdue invoices, banking follow-up, evidence/document remediation, payroll/TSD, KMD/tax reports, expenses, fiscal-year close, carry-forward, and confirmation-ready migration runs. | ☐ It is not yet a complete accountant cockpit; remaining payroll/document/evidence-policy edges and some close/migration follow-ups need direct execution and stronger end-to-end proof. |
| Documents and evidence policy | ✅ Document review, retention, replacement, archive/disposal, legal hold, purge guards, evidence-policy checks, remediation assignments, and evidence blockers cover many high-risk workflows. | ☐ Policy enforcement is not universal. Broader workflow-level controls, richer follow-up, and remaining edge-case remediation still need implementation and tests. |
| E-invoicing and OCR | ✅ Manual Estonian e-invoice XML import and related validation/evidence workflows exist. | 🚫 Direct e-invoice operator send/receive and OCR capture require external integrations or additional production infrastructure. |
| Operations, backup, and restore | ✅ Backup, offsite-sync, restore-drill, health metrics, CLI preflight, systemd templates, provider examples, and host preflight/install helpers exist. | ☐ Live provider credentials, storage/database connectivity, timer enablement, real-infrastructure backup drills, monitoring/SLOs, and operational runbooks must still be verified per deployment. |
| Plugins and integrations | ✅ Registries, manifests, permissions, webhooks, signed delivery, loopback HTTP runtime, supervised package runtime, runtime status/restart, frontend slots, and secret-safe allowlisted process environments exist. | ☐ OS-level sandboxing, resource isolation, and broader production plugin containment remain incomplete. |
| Production readiness | ✅ CI, backend/frontend coverage, docs gates, CLI coverage, integration shards, smoke E2E, seeded demo E2E, and Docker image validation are active. | ☐ A production rollout still needs security review, deployment hardening, real-infrastructure drills, monitoring/SLOs, support runbooks, and accounting-firm pilot proof. |

## Documentation Source Of Truth

- Use [DEVELOPMENT_STATUS.md](./DEVELOPMENT_STATUS.md) for detailed current
  feature status and verification evidence.
- Use [USE_CASE_COVERAGE.md](./USE_CASE_COVERAGE.md) for use-case-level test
  evidence.
- Use this file for the concise product cap/gap summary and open work.
- Keep new implementation plans outside the active documentation tree unless
  they are promoted into current status, coverage, architecture, or API docs.

## Update Rule

When a stage changes what the product can honestly claim, update this file in
the same commit as `DEVELOPMENT_STATUS.md` and `USE_CASE_COVERAGE.md`. Keep the
✅ column limited to claims backed by authoritative code, test, and
documentation evidence. Do not move an item out of the gaps table until there is authoritative code, test, and documentation evidence for the full workflow being claimed. The ☐ column is the concise view of that remaining work.
