# Remaining Features Plan

**Date:** 2026-03-12
**Reviewed:** 2026-06-12

This plan is a historical planning artifact. For the authoritative verified
baseline, use [DEVELOPMENT_STATUS.md](../DEVELOPMENT_STATUS.md). The sections
below keep the March product framing, but the current-position and priority
language has been refreshed so this file no longer lists completed work as
missing.

This plan lists the main feature gaps that still matter for Open Accounting to become a trustworthy, useful accountant/SMB product after the current reliability and import foundation work.

## Current Position

Already in place:
- green backend, frontend, integration, smoke E2E, and seeded demo E2E baselines in the verified status record
- blocking backend integration tests, smoke E2E, and four-shard seeded demo E2E in CI
- tenant-scoped API tokens and broad `cmd/oa` automation with 100.0% CLI package coverage enforced by the backend gate
- CSV/XML migration imports for chart of accounts, contacts, employees, invoices, Estonian e-invoices, payments, expenses, payroll history, leave balances, TSD/KMD history, quotes, orders, recurring invoice templates, bank accounts, bank transactions, cost centers, cost allocations, product categories, warehouses, products, stock adjustments with lot metadata, fixed assets, opening balances, and historical journal entries
- non-mutating migration bundle validation for required columns, XML payloads, and same-bundle references across the supported cutover files
- audited period close/reopen, fiscal-year close evidence, year-end carry-forward, and carry-forward reversal workflows
- document upload/download/review/retention/evidence-policy flows for core accounting records, reconciliation, assets, expenses, commercial documents, leave records, and year-end close packs
- server-side CSV/XLSX/PDF exports for primary accounting and management reports
- production startup checks for auth/CORS configuration, refresh-token/session revocation, password reset, privileged security events, backup scripts, offsite sync, restore-drill wrappers, and deployment documentation

Still incomplete or intentionally blocked:
- broader incumbent-system cutover remains incomplete despite broad CSV/XML import coverage; vendor-specific mapping presets and deeper cross-file validation remain useful next steps
- accountant review exists, but deeper exception actions beyond overdue invoices, bank follow-up, journal evidence, and document review are still needed
- document policy enforcement exists for several workflows, but broader retention automation and workflow-level policy coverage remain thin
- auth administration and operational hardening are improved but not yet complete for accounting-firm production use
- direct bank feeds, SEPA initiation, direct e-invoice operator exchange, OCR, automatic e-MTA submission, and e-ariregister filing remain external-dependency tracks

## Priority Order

### Phase 1: Migration Completion

Goal:
- make cutover realistic for a small accounting firm or SMB without manual SQL

Work:
- extend vendor-specific mapping presets for Merit, SmartAccounts, and common bank/provider exports
- deepen migration bundle validation for remaining cross-file consistency checks
- cover more incumbent-system history families where pilots still require manual work

Exit criteria:
- a pilot tenant can migrate master data plus active operational data from documented CSV/XML bundles without manual SQL

### Phase 2: Close Controls

Goal:
- make accounting periods defensible instead of merely editable with a lock date

Work:
- expand close exception reporting and operator guidance around late corrections
- keep year-end close evidence, carry-forward, and reversal flows covered by API/CLI/docs/tests

Exit criteria:
- finance users can complete and correct a controlled year-end close without dropping to SQL or editing tenant settings manually

### Phase 3: Document Workflows

Goal:
- attach source evidence to accounting records

Work:
- broaden automated policy enforcement beyond the workflows already covered
- add deeper retention automation, reminders, and operational review surfaces
- keep storage/deployment behavior documented and tested for restore/review scenarios

Exit criteria:
- accounting records retain supporting files across restart/deploy and policy failures are surfaced before risky workflow actions

### Phase 4: Accountant Workspace

Goal:
- make the product usable as a daily review surface, not just a transaction entry tool

Work:
- add deeper exception actions across tax, payroll, documents, close, and migration follow-up
- reduce jumps from portfolio summaries to manual record hunting
- keep cross-tenant visibility aligned with RBAC and audit expectations

Exit criteria:
- an accountant can see and resolve most routine tenant exceptions without manual report hunting

### Phase 5: Authoritative Reporting

Goal:
- make exports reproducible and accountant-grade

Work:
- add remaining accountant-grade depth and validation to reports already exported server-side
- improve report auditability, reconciliation to ledger state, and edge-case coverage

Exit criteria:
- exported reports are API-backed, testable, consistent across browsers, and defensible for accountant review

### Phase 6: Security and Operations

Goal:
- make production pilots supportable

Work:
- complete remaining auth administration controls and production hardening
- harden backup credential scheduling and recurring restore-drill operations in real deployments
- improve metrics, structured errors, and service runbooks

Exit criteria:
- early adopters can run a pilot with basic operational safety and auditable privileged actions

## Blocked Items

These remain outside the immediate implementation plan because they need external dependencies or certification:
- direct bank feeds
- SEPA initiation
- Estonian e-invoice exchange
- automatic e-MTA submission
- OCR invoice capture
- direct annual filing integrations

## This Tranche

Implemented in this tranche:
- invoice CSV import in API, web UI, and CLI
- grouped multi-line invoice imports with contact matching and row-level errors
- remaining-features plan aligned to the verified repo state
- the later staged work listed in [DEVELOPMENT_STATUS.md](../DEVELOPMENT_STATUS.md), including period close controls, document evidence workflows, server-side exports, auth/session hardening, backup/restore wrappers, broad migration imports, 100.0% CLI coverage enforcement, and current PR #62 CI validation
