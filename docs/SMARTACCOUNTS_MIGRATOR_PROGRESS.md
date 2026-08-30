# SmartAccounts migrator progress board

**Project:** SmartAccounts.eu -> Open Accounting migration and one-way sync

**Updated:** 2026-08-29

**Owner:** Hold My Beer OÜ product owner

**Target repositories:** planned private `HMB-research/smartaccounts-migrator`
and Open Accounting

**Current phase:** deployed no-key onboarding pilot. The private bridge,
selected/all tenant onboarding, Brave relay, archive delivery, confirmed GL
and reference planners, exact-match policy approval, streaming reconciliation,
and independent accountant attestation are deployed on server-nuc through
migration `088`. The final onboarding handoffs, fail-closed archive coverage,
selected/all full-claim eligibility gate, and dormant commercial relay control
are deployed with source fingerprint
`3788115c45d406e41130c1f90b8b11510f85afa0657859ab17ed79872c6aab04`.
No Hold My Beer OÜ source row or financial write has been used yet. The
SmartAccounts application recovered after maintenance, but maintenance
invalidated both browser sessions; live onboarding remains paused until the
owner signs back into SmartAccounts and Open Accounting in Brave.

## Current delivery snapshot — 2026-08-28

| Item | Status | Evidence / next gate |
|---|---|---|
| Private bridge and Brave relay | `DONE` | Private repo `HMB-research/smartaccounts-migrator` is private; server-nuc runs bridge commit `257f47f853e8a1c3b9a161b42cbb5ffa440fb465`, healthy on the private Docker network with no published host port. Relay `0.2.1` fixes classic-script global collisions, answers the exact v2 zero-data readiness handshake, binds GL posting dates to the immutable capture window, requires terminal master-detail completeness sentinels, and has a dormant status-first commercial relay. Extension tests (89), full Go race suite, vet and build pass. |
| OA onboarding, archive staging and planners | `DONE` | Server-side selected/all onboarding creates or reuses isolated tenants, binds opaque sources, stages only digest-checked archives, and exposes confirmed-only GL/reference previews. Owner/accountant continuation links carry only tenant/batch/source IDs and revalidate package/preview bindings. API, frontend and migrations through `088` are deployed and `/ready` reports API/database/bridge ready. |
| Minimal-click continuation and retry safety | `DONE` | The deployed UI safely polls post-pairing progress, restores batch state after reload, and offers one `Continue safe workflow` action instead of separate routine refresh/resume controls. Server status guidance auto-advances completed discovery and final schema approval only; source transfer, GL apply and accountant attestation remain explicit. Concurrent exact pairing retries serialize on the immutable catalog receipt and cannot start duplicate onboarding runs. |
| Archive coverage classification | `DONE` | The staged-package coverage endpoint/UI returns counts only and classifies records by exact reviewed entity/resource/schema tuples. Unknown future contracts are `UNCONSUMED`; browser commercial identity gaps remain `REVIEW_REQUIRED`; no source IDs, rows, amounts or digests are rendered. |
| Full-claim eligibility gate | `DONE` | A deterministic matrix covers all 31 browser surfaces, 40 documented API retrieval contracts, master-detail and commercial paths. The owner-only selected/all status is count/fixed-code only and remains `NOT_ELIGIBLE` for every unresolved filter, page, schema, review, tombstone, API or reconciliation gap; a partial GL run cannot be labelled full. |
| Dormant commercial relay control | `DONE` | Migration `088` and the private relay implement exact ordered `client_invoices` then `bank_payments` status/resume control. Until a signed-in review proves the visible selectors/pagers, issue persists safe bindings only and upload/finalize are rejected before bytes or bridge/source access. No apply or full-claim path exists. |
| GL authority and duplicate-posting protection | `DONE` | Only reviewed v2 `general_ledger` / `general_ledger_csv_v1` can create authoritative-once journals. Invoices, payments, commercial details and summary journals are non-posting evidence. Exact replay re-verifies posted target journals; revisions, tombstones and mismatches fail closed. |
| Exact reconciliation and independent approval | `DONE` | Migration `087` stores digest/ID-only apply receipts, server-derived zero-variance policy approval, streamed archive-to-posted-ledger proof, selected/all roll-up, stale-evidence invalidation and a separate accountant attestation. API tokens cannot perform human approval actions. |
| Production rollback gate | `DONE` | Custom-format backup `pre-088-20260828T205100Z.dump` was checksum-verified and enumerated with `pg_restore`; the immediately prior API/frontend/bridge images are tagged `rollback-pre-088-20260828T235300` / `rollback-pre-257f47f-20260828T235300`, and the prior bridge environment is backed up as `bridge-env-pre-257f47f-20260828T235300`. |
| Live HMB source capture | `BLOCKED` | The deployed database still has zero GL apply receipts, tolerance policies, evaluations, approvals and commercial authorizations. SmartAccounts is reachable again, but maintenance invalidated its browser session and the OA session. Resume only after the owner signs into both Brave tabs and relay readiness returns exact v2 `signed_in`. |
| Commercial and remaining-domain coverage | `BLOCKED` | Commercial client-invoice/bank-payment capture remains fail-closed until signed-in Brave evidence proves the exact visible list selector/pager. The remaining 23 statutory/payroll/warehouse/files/report surfaces remain protected evidence/review gaps; do not label a partial GL package a full sync. |
| Financial posting | `BLOCKED` | Requires a finalized staged package, explicit mapping decisions, accountant-approved server-derived exact-match policy, a different financial operator's final GL confirmation, streamed proof, and independent final accountant attestation. No such action has been performed. |

The original planning matrix below is retained as historical design context;
the table above is the operative progress record.

## Status vocabulary

| Status | Meaning |
|---|---|
| `DONE` | Evidence is linked and acceptance criteria passed. |
| `ACTIVE` | Work is under way with a named owner and next evidence. |
| `BLOCKED` | Cannot advance until the listed decision, access, or external input exists. |
| `PLANNED` | Defined work not yet started. |
| `DEFERRED` | Intentionally outside the current release; its evidence-only treatment is approved. |

## Delivery dashboard

| ID | Workstream | Status | Evidence / exit criterion | Dependency |
|---|---|---|---|---|
| R0 | SmartAccounts source and UI capability inventory | `DONE` | API v1.7 catalog, authenticated UI route review, API/export gap classification | None |
| R1 | Separate CLI and canonical package design | `DONE` | [Implementation plan](./SMARTACCOUNTS_MIGRATOR_IMPLEMENTATION_PLAN.md) | R0 |
| R2 | OA Import Session v1 design | `DONE` | Tenant-scoped session, artifact, mapping, approval, event and reconciliation contract | R1 |
| P0-1 | Snapshot consistency and cursor protocol | `PLANNED` | Written algorithm, race-condition tests, and source-cutoff proof | R0 |
| P0-2 | Field-level coverage ledger | `PLANNED` | One row for every source field with approved disposition | R0 |
| P0-3 | SmartAccounts authentication and source-access design | `PLANNED` | Least-privilege access, signing, rotation, revocation and incident flow approved | R0 |
| P0-4 | Export-bridge contracts | `PLANNED` | Format, collector, validation, cadence, evidence and reconciliation rule per UI-only domain | P0-2 |
| P0-4a | Brave-debugger fallback contracts | `PLANNED` | Read-only request catalogue, test-company replay evidence and per-request safety approval | P0-4 |
| P0-5 | Cutover, period-close, FX and tax basis decisions | `BLOCKED` | Accountant-approved cutover charter | Business/accountant decision |
| P0-6 | Privacy, evidence and encryption key lifecycle | `PLANNED` | Data classification, access model, retention/disposal and recovery procedure approved | P0-2 |
| M1 | CLI repository foundation | `DONE` | Private `sa-migrate`/`sa-bridge`, SQLite state, private workspace, fixtures and local validation | P0-1, P0-3, P0-6 |
| M2 | Read-only source capture | `DONE` | Signed API client, persisted quota scheduler, checkpointing, child queue and source-contract tests | M1 |
| M3 | Canonical package and mappings | `ACTIVE` | Deterministic protected packages, v2 GL adapter, browser master snapshots and strict review gates are implemented; live vendor schemas/field ledger still require a signed-in source run | M2, P0-2, P0-4 |
| M4 | OA Import Session v1 | `DONE` | Session/bridge archive API, tenant binding, staging, durable run history and review UI | M3 |
| M5 | Approved core apply | `DONE` | Idempotent GL executor, exact target replay checks, server-owned tolerance policy, streaming proof and independent accountant approval are deployed; live approval is still intentionally required | M4, P0-5 |
| M6 | UI-only export bridges | `ACTIVE` | v2 GL and master-detail relay are implemented; commercial selectors and 23 remaining evidence domains stay fail-closed pending signed-in review | M5, P0-4 |
| M7 | Pilot and production cutover | `ACTIVE` | Production plumbing and rollback evidence are deployed; signed-in HMB capture, reconciliation, accountant sign-off and monitored cutover remain | M5, M6 |

## P0 closure checklist

### P0-1: Snapshot consistency

- [ ] Document source time and clock requirements, including Europe/Tallinn,
  clock-skew limits and an NTP/clock-health preflight.
- [ ] Capture immutable start watermark `T0` before a full initial extraction.
- [ ] Store every endpoint's request window and terminal page checkpoint.
- [ ] Re-run all change-history resources from an overlapping `T0` window after
  the baseline completes; deduplicate by provider/company/resource/external ID
  plus revision or payload hash.
- [ ] Record first-page deletion lists before advancing each resource cursor.
- [ ] Define a static-resource full-list hash/diff and warehouse-movement
  rolling-window procedure.
- [ ] Capture final watermark `T1`, then prove no resource is left between
  `T0` and `T1` before target apply is permitted.
- [ ] Test source edits, deletes, page reordering, interrupted pages, identical
  timestamps, and source changes during the extraction.

### P0-2: Field-level coverage ledger

- [ ] Generate `coverage/<resource>/<field>` rows from the tested source
  schemas, not hand-written lists alone.
- [ ] Record source type, sensitivity, cardinality, target field or rule,
  transform/version, losslessness, target posting effect and reconciliation
  metric.
- [ ] Require one disposition: `mapped`, `derived`, `evidence_only`,
  `excluded`, or `blocked`.
- [ ] Require owner and accountant approval for every non-lossless disposition.
- [ ] Make unresolved, changed, or unapproved rows a planning/commit blocker.

### P0-3: SmartAccounts authentication

- [ ] Confirm current subscription/API eligibility and company-administrator
  authority; do not rely on a plan name or historical pricing assumption.
- [ ] Create a dedicated API user group and grant only the read permissions
  required by the accepted coverage ledger. Remove save, delete and file-change
  permissions unless a validated source read requires them.
- [ ] Enable API access and generate the company key pair only after the above
  review. API-key creation is a user/admin action and must not be automated by
  the CLI.
- [ ] Store API key and secret as private secret references; record only a
  non-secret key fingerprint and activation date in the workspace.
- [ ] Verify HMAC-SHA-256 signing, exact URL encoding, Europe/Tallinn timestamp,
  API hostname allowlist, TLS verification, redirect rejection and signature
  failure behaviour using read-only probes.
- [ ] Define rotation/revocation with a planned pause and credential cutover;
  do not assume SmartAccounts supports overlapping active key pairs.
- [ ] On 401/signature/billing/access failure, stop capture, retain checkpoints,
  notify the operator and require an explicit `doctor`/resume check. Never
  retry authentication failures blindly.

### P0-4 through P0-6: source evidence and accounting policy

- [ ] Specify one supported export schema, exporter, collection cadence,
  integrity hash, private storage path and validation rule for each UI-only
  domain.
- [ ] Define which exports are importable and which are evidence-only; do not
  call a UI-only domain "synced" without an accepted bridge.
- [ ] Where an accepted export is unavailable, use the Brave debugger only to
  catalogue and contract-test a narrowly scoped read-only UI retrieval. Record
  method, redacted request/response schema, authentication boundary, source
  page, pagination, rate, idempotency, stability evidence and owner approval.
- [ ] Reject browser-backed save, delete, import, payment, tax filing, user,
  settings or other state-changing calls from automation. Such workflows remain
  manual/export-backed unless SmartAccounts provides a supported integration
  contract.
- [ ] Record period locks, opening-balance boundaries, late-source-document
  process, source correction path, foreign-exchange/rate source, rounding,
  VAT rate effective dates and report basis in the cutover charter.
- [ ] Classify data by confidentiality, including payroll/employee data; define
  encryption recipients, key rotation/recovery, access review, retention,
  disposal/legal hold and restore verification.
- [ ] Add attachment MIME/path validation and malware/quarantine policy before
  accepting source files.

## Decisions awaiting the owner or accountant

| Decision | Needed from | Blocks |
|---|---|---|
| One-time cutover, ongoing mirror, or both | Product owner and accountant | M1 onward |
| GL-authoritative, subledger-authoritative, or opening/open-items policy | Accountant | M3 onward |
| Historical start, cutover date and locked-period policy | Accountant | M3 onward |
| UI-only resource treatment and export owners | Product owner/accountant | M3 and M6 |
| Evidence retention, access roles, key custodians and recovery owners | Product owner/security owner | M1 onward |
| OA tenant owner, integration operator and independent approver | Product owner | M4 onward |
| Production RPO, alert recipients and reconciliation tolerance | Product owner/accountant | M7 onward |

## How to maintain this board

1. Update a row only with a link to the review, test, report, run ID, or signed
   decision that supports it.
2. Move `ACTIVE` to `DONE` only after its explicit exit criterion passes.
3. Add each new source schema field or UI capability to P0-2/P0-4 before it is
   allowed into a production package.
4. Preserve decision history in the relevant package and OA Import Session;
   never overwrite an approved historical decision.
5. Reassess status after every SmartAccounts API/schema change, OA migration,
   cutover rehearsal, security incident or reconciliation variance.
