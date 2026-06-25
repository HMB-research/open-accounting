# Current Product Limits

Last reviewed: 2026-06-25

This is the short current-state gap document for Open Accounting. It is the
place to check what still prevents the repository from being described as fully
featured, production-ready accounting software.

## Current Verified Cap

Open Accounting can honestly claim a broad self-hosted Estonian SMB/accountant
workflow today: core double-entry accounting, invoicing, purchase invoices,
payments, manual bank import and reconciliation, payroll, leave records, KMD/TSD
export-oriented compliance, document evidence workflows, accountant review
queues, a Go operator CLI, and a Svelte dashboard.

Current gate evidence is maintained in
[Development Status](./DEVELOPMENT_STATUS.md). That evidence is not a
production-readiness claim; it proves the checked use cases and regression gates
for the current branch.

## Gaps From A Full-Featured Product

| Area | Current cap | What is still missing |
| --- | --- | --- |
| Historical migration and cutover | Many CSV/XML imports, provider presets, validation, saved runs, live progress, guarded API/CLI execution, dashboard workbench flows, and KMD VAT-total reconciliation against same-bundle invoice/e-invoice/journal VAT support exist. | Deeper provider-specific mapping, more cross-file validation beyond the currently covered payroll/TSD, KMD VAT support, commercial document, payment, banking, inventory, fixed-asset, and allocation checks, and more dashboard-side mutating cutover controls. |
| Accountant workspace execution | The review workspace can surface and execute many banking, expense, payroll, TSD, KMD, tax-report, document, close, and migration remediation actions. | It is not yet a complete accountant cockpit. Remaining payroll/document/evidence-policy edges and some close/migration follow-ups still need direct execution flows and stronger end-to-end proof. |
| Document and evidence policy | Evidence blockers and structured remediation exist for many high-risk workflows, including invoices, journals, payments, bank reconciliation, fixed assets, expenses, quotes, orders, leave records, TSD/KMD declarations, and close packs. | Policy enforcement is not universal. Broader workflow-level policy controls, richer evidence follow-up, and remaining edge-case remediation still need implementation and tests. |
| Tax and authority filing | KMD/TSD generation, export, history import, submission/acceptance status tracking, and evidence gates are implemented for manual filing workflows. | Automatic e-MTA submission remains blocked by external certification/integration work. Direct authority filing should not be described as locally complete. |
| Banking and payments | Manual bank imports, matching, reconciliation, SEPA pain.001 export, and remediation queues exist. | Direct bank feeds and direct SEPA initiation remain blocked by external bank/partner integration work. |
| E-invoicing and OCR | Manual Estonian e-invoice XML import exists. | Direct e-invoice operator send/receive and OCR capture are not implemented as production integrations. |
| Operations, backup, and restore | Backup/offsite/restore-drill scripts, CLI preflight parity, metrics, and host helper templates exist. | Live provider credential provisioning, storage/database connectivity proof, and host timer enablement must still be performed and verified per deployment. |
| Plugins | Registries, manifests, permissions, webhooks, loopback HTTP runtime, supervised package runtime, runtime status/restart, and allowlisted process environments exist. | OS-level sandboxing, resource isolation, and broader production plugin containment are still incomplete. |
| Production hardening | CI gates, docs gates, CLI coverage gates, demo E2E shards, and integration shards are strong for a development branch. | A production rollout still needs security review, live deployment hardening, backup drills against real infrastructure, operational runbooks, monitoring/SLOs, and accounting-firm pilot proof. |

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
the same commit as `DEVELOPMENT_STATUS.md` and `USE_CASE_COVERAGE.md`. Do not move an item out of the gaps table until there is authoritative code, test, and documentation evidence for the full workflow being claimed.
