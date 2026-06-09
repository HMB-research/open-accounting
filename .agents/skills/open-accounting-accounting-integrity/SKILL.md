---
name: open-accounting-accounting-integrity
description: Use when implementing or reviewing accounting-sensitive open-accounting workflows such as payment reversals, invoice paid-state changes, journal void/reopen flows, period locks, evidence blockers, or any change that could rewrite accounting history.
---

# Open Accounting Accounting Integrity

Use this with `open-accounting-development` when a change touches posted accounting history, payment/invoice balances, fiscal locks, journal lifecycle, or audit evidence.

## Core Rules

- Prefer auditable offsetting actions over destructive mutation. Corrections should create reversal, void, adjustment, or reopening records that preserve the original event and operator reason.
- Do not delete or silently rewrite posted ledger, payment, invoice, close, payroll, tax, or reconciliation history unless an existing domain rule explicitly permits it.
- Enforce tenant period locks on the effective accounting date for the new correction event.
- Require a non-empty reason or note for reversal, void, reopen, and other audit-sensitive correction workflows.
- Keep API, CLI, service, repository, and frontend paths on the same reusable workflow. Do not duplicate accounting correction logic per entry point.
- Use repositories and ORM-backed persistence for state changes. Raw SQL belongs in migrations or isolated repository implementation details only.

## Implementation Checklist

- Add service-level validation before repository writes: entity exists, tenant matches, state permits correction, reason is present, and the correction date is allowed.
- Make the repository method atomic when it creates a correction record and marks the original record.
- Preserve links in both directions when useful, for example original `reversed_by_*` fields plus correction `reversal_of_*` fields.
- Update derived business state through existing domain services, for example invoice paid amount/status through invoicing service methods.
- Return conflict errors for already-corrected or disallowed correction attempts, not silent success.
- Update API docs, CLI docs, route coverage mappings, focused tests, and generated Swagger when adding a route.

## Focused Validation

Run the focused package tests for each touched accounting surface before broader gates:

```bash
go test -count=1 ./internal/payments ./internal/invoicing ./cmd/api ./cmd/oa
go test -timeout=5m -tags=integration ./internal/payments -run TestGORMRepository_CreateReversal -count=1
make test-cli-coverage
go test -timeout=3m ./docs -count=1
```

For frontend exposure of a correction workflow, also load `open-accounting-frontend-workflow` and prove the user workflow with the focused component or demo E2E test.
