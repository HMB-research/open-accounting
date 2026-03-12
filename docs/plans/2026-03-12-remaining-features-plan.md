# Remaining Features Plan

**Date:** 2026-03-12

This plan lists the main feature gaps that still matter for Open Accounting to become a trustworthy, useful accountant/SMB product after the current reliability and import foundation work.

## Current Position

Already in place:
- green local backend and frontend baselines
- blocking backend integration tests and smoke E2E
- tenant-scoped API tokens and CLI automation
- CSV imports for chart of accounts, contacts, invoices, and opening balances
- tenant period lock on core write paths

Still missing or incomplete:
- employee and broader incumbent-system migration imports
- close, reopen, and fiscal-year workflows
- attachments and document retention
- stronger accountant review and exception workflows
- server-side authoritative reporting/export pipeline
- security and operational hardening

## Priority Order

### Phase 1: Migration Completion

Goal:
- make cutover realistic for a small accounting firm or SMB without manual SQL

Work:
- employee import with contract, tax, and payroll settings
- invoice/payment linkage import for active receivables and payables
- Merit and SmartAccounts CSV mapping presets
- import validation reports for cross-file consistency

Exit criteria:
- a pilot tenant can migrate master data plus active operational data from CSV alone

### Phase 2: Close Controls

Goal:
- make accounting periods defensible instead of merely editable with a lock date

Work:
- explicit month-end close action
- reopen flow with privileged role checks
- fiscal year configuration and year-end carry-forward workflow
- close/reopen audit events and operator notes

Exit criteria:
- finance users can close and reopen periods without dropping to SQL or editing tenant settings manually

### Phase 3: Document Workflows

Goal:
- attach source evidence to accounting records

Work:
- attachment upload/download/delete for invoices, banking matches, expenses, and assets
- storage abstraction for local disk and object storage
- document metadata, previews, and basic retention surfaces

Exit criteria:
- core accounting records can retain supporting files across restart and deploy

### Phase 4: Accountant Workspace

Goal:
- make the product usable as a daily review surface, not just a transaction entry tool

Work:
- unmatched banking queue
- overdue invoice review
- missing tax data review
- draft declaration review
- tenant-level task summary and exception counts

Exit criteria:
- an accountant can see what needs attention across tenants without manual report hunting

### Phase 5: Authoritative Reporting

Goal:
- make exports reproducible and accountant-grade

Work:
- server-side CSV/XLSX/PDF generation for primary reports
- general ledger
- customer/vendor statements
- improved AR/AP aging validation
- budget-vs-actual and customer profitability later

Exit criteria:
- exported reports are API-backed, testable, and consistent across browsers

### Phase 6: Security and Operations

Goal:
- make production pilots supportable

Work:
- remove insecure defaults
- refresh/session revocation hardening
- password reset flow
- backup/restore drill documentation and automation
- structured errors, metrics, and service runbooks

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
