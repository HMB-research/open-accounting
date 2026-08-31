# SmartAccounts.eu full-sync checklist and plan

## Objective and scope

Build an auditable, one-way **SmartAccounts.eu to Open Accounting** connector
for Hold My Beer OÜ. "Full sync" means that every business capability is
classified as one of:

1. API-synchronised;
2. retrieved through a supported export bridge; or
3. explicitly excluded with retained source evidence and an accountant-approved
   reconciliation procedure.

It does **not** mean replaying undocumented SmartAccounts UI *mutations*.
Read-only, authenticated UI/export endpoints that are required to cover a
published-API gap may be discovered and exercised through the administrator's
Brave session. Each such endpoint must be recorded with its capture version,
read-only proof, schema/hash test, session-bound fallback behaviour, and
resource reconciliation rule. It is not permitted to extract browser cookies
or use any private endpoint for source writes.

The initial production direction is read-only SmartAccounts -> Open Accounting.
No SmartAccounts write endpoint is used.

## Current OA target disposition (audited canonical contracts)

The table below is the current OA-side boundary, not a claim that any company
has completed a full sync. A checksum-finalized staged package can be inspected
through the count-only `archive-coverage` endpoint; an unknown canonical
contract is deliberately `UNCONSUMED`, never silently promoted to an archive
or writer path.

| Current canonical domain | OA target disposition | Guardrail / remaining work |
| --- | --- | --- |
| Reviewed v2 `general_ledger_journal` (`general_ledger_csv_v1`) and the existing normalized API `general.entries.get` path | Apply-gated only | SmartAccounts GL is authoritative; a digest-bound preview, an independently approved exact-match policy, and explicit confirmation create each journal once. `journal_entries` is retained as a summary artifact/evidence and has no generic authoritative or archive-record adapter. |
| API accounts, clients, vendors and articles | Confirmed non-financial reference apply | Accounts/contacts/items have deterministic identity/revision guards. Browser clients/vendors require proven ISO-2; browser articles remain review-only until an explicit VAT mapping exists. A master-detail relay state of `finalized_archived_evidence` means only that the bridge sealed evidence: OA may expose a reference preview only after its exact tenant/source/package/digest is separately delivered and verified as `STAGED_REVIEW_REQUIRED`. |
| Reviewed API commercial invoices, offers/orders and payments; browser commercial evidence | API: archive-only evidence. Browser: review-required evidence only. | They never create an OA invoice, payment, or GL posting, so they cannot duplicate authoritative source GL. OA now has an owner/batch-bound, extension-only capability relay for the fixed `client_invoices` then `bank_payments` order. It is status-first (the extension creates sequence 1 only after a capability `GET` receives `404`) and deliberately held at `list_selector_required`: upload/finalize are rejected before source bytes are read until a separate reviewed visible-selector/pager contract exists. A future sealed receipt remains immutable identity-review evidence and cannot claim completed archive coverage, staging promotion, preview, apply, or full reconciliation. |
| Bank/cash/payment-method configuration and payment references | Archive-only evidence | No bank account, cash account, allocation, or alternate payment-posting writer is enabled. |
| Reviewed API VAT percentages and report-row definitions; browser statutory/report snapshots | API configuration: archive-only evidence. Browser statutory/report records: unconsumed until exact delivery contract review. | Configuration and report evidence are not VAT/TSD declarations, filing state, or period-report imports. |
| Reviewed API workers, absences, payout types and vacation balances; browser salary/wage reports | API foundation: archive-only sensitive evidence. Browser salary/wage records: unconsumed until exact delivery contract review. | No payroll calculation, worker contract, absence, or salary-run writer is enabled. |
| Reviewed API warehouse movements; browser warehouses, inventory, reports, fixed assets and depreciation | API movements: archive-only evidence. Browser operational records: unconsumed until exact delivery contract review. | No inventory valuation/movement, asset-register, or depreciation writer is enabled. |
| Declared package artifacts; files/comments | Manifest artifacts: protected archive storage. Canonical file/comment records: unconsumed until reviewed. | Artifacts are tenant-isolated protected storage even without a canonical-record writer. A file/comment/attachment record needs its own exact reviewed contract; no generic evidence entity is silently archive-complete. |
| Tombstones, review-required records, source-binding/count failures, malformed or unknown contracts | Review required / unconsumed | Never auto-delete, overwrite, post, or claim coverage. A new canonical contract must first add a reviewed target consumer. |

This makes the current boundary explicit: archive retention is coverage of
evidence, while only the two separately confirmed target paths can mutate OA;
neither path can post invoices or payments. `ARCHIVE_ONLY` is a closed
target-side matcher over the exact reviewed `(entity_type, resource,
source_schema)` tuple. A familiar evidence entity with a new schema/resource
is `UNCONSUMED` until a reviewed consumer is added. Declared manifest
artifacts remain protected archive storage independently; they are not a
generic canonical-record allowlist.

### Deterministic full-claim coverage gate

`internal/smartaccountssync/full_claim_coverage.go` keeps a data-minimized,
deterministic inventory of every resource in the 31-surface
`smartaccounts-brave-ui-v2` manifest, the separate master-detail/commercial
routes, and all 40 documented API v1.7 GET methods. Each row records only its
source surface, resource ID, contract version, disposition, and fixed
blocker—never a company, source record, row, amount, credential, or URL.

The immutable `smartaccounts-full-claim-domain-plan-v2` groups those routes by
business domain and selects exactly one primary source route. API routes are
preferred where they cover the same domain; Brave/export routes remain auditable
alternatives and cannot create duplicate blockers for an API-primary domain.
This is not a relaxation: a selected route stays blocking until its live source
access, schema, completeness, reconciliation, tombstone handling, and
independent accountant attestation have all been bound to that exact plan.
Filter/page-only, unconsumed, review-required, missing-endpoint, and explicit
export-contract selections remain hard product blockers. The v2 plan also
contains ten mandatory source-path obligations for material capability gaps in
the documented API: recurring workflows, purchase-order/receipt lifecycle,
e-invoice queues, bank import/statement matching, VAT/TSD filing receipts,
inventory valuation/count sheets, fixed assets/depreciation, payroll runs,
report outputs, and allowlisted company financial-year/default/audit metadata.
Each begins as a blocked `vendor_immutable_export` selection; it can be
replaced only in a newer reviewed plan by a vendor-confirmed signed API route,
an independently reviewed authenticated Brave read/export contract, or a
versioned immutable vendor export. No neighbouring API endpoint, generic
archive, browser grid, or caller-supplied evidence can satisfy it. Migration
`089` supplies an append-only,
digest-only per-domain evidence ledger. Each row is bound to exactly one
`(batch, tenant, source company, package, scope digest, reconciliation evidence digest, plan version, domain)`
and can contain only reviewed route metadata, six required boolean proof gates,
an opaque evidence digest, and a timestamp. It holds no source records, names,
amounts, URLs, request/response payloads, browser state, credentials, cookies,
or free-form notes. A changed package, scope, reconciliation generation,
source, tenant, or plan requires a distinct receipt; the existing receipt
cannot be overwritten.

Migration `090` retains historical v1 receipts for audit but only permits the
current v2 plan to participate in a new full-claim decision. Consequently the
product remains `NOT_ELIGIBLE` until every selected source has current proof
for all API and non-API obligations.

The ledger is server-internal and intentionally has no HTTP write endpoint.
`ELIGIBLE` additionally requires the exact selected route for every domain and
every selected source to have all six gates: live-source validation, schema
validation, completeness validation, reconciliation validation, resolved
tombstones, and accountant attestation. A persisted reconciliation `PASS` by
itself is never enough. With the current reviewed plan its unresolved static
gaps still keep the deployed status `NOT_ELIGIBLE`.

The owner-only `GET /smartaccounts-sync/browser-onboarding/batches/{batchID}/full-claim-eligibility`
combines that fixed selected-domain plan with the original immutable
selected/all batch. It returns only aggregate counts and fixed blocker codes:
every selected source must have a current reconciliation `PASS`, with no
tombstone or selected-domain coverage gap, before it can be `ELIGIBLE`. It is
read-only and never returns a source name, ID, package, digest, proof, amount,
token, or capability. It does not apply, approve, or change reconciliation
state. With the current reviewed plan it remains `NOT_ELIGIBLE`; in particular
a partial GL capture can never be presented as a full sync.

## Research record

### Published SmartAccounts API

The current vendor reference is SmartAccounts API version 1.7 (28 April 2025).
It documents HMAC-SHA-256 signed requests, an Estonian-time timestamp with a
15-minute freshness window, 60 requests/minute, 1,000 requests/day, variable
pagination, and deletion lists on the first page of change-history queries.

Change-history resources should be read with an overlapping modified-since
window and deduplicated by external identity plus a content/version hash. List
resources require periodic full-list hashing/diffing. Warehouse movements
require a rolling date-window snapshot because they have neither a published
change cursor nor deletion feed.

| SmartAccounts area | Published source service | Sync mode |
|---|---|---|
| Files | Files for supported documents, including binary content | Parent-by-parent list/diff and content hash |
| Ledger and account balance evidence | GL entries and date-specific account balances | Incremental GL plus point-in-time reconciliation |
| Accounting configuration | Accounts, bank/cash accounts, countries, groups, objects, payment methods, report rows, warehouses, VAT percentages and document-template identities | Full snapshot plus scheduled list diff |
| Commercial masters | Articles, clients and vendors | Incremental |
| Sales and purchasing | Sales invoices, vendor invoices, sales offers/orders and payments | Incremental, including rows and available source documents |
| Payroll foundation | Payout types, absence types, workers, absences and vacation report | Mixed: list diff for settings/workers; incremental absences |
| Inventory | Article quantities and warehouse movements | Current/default-warehouse quantity snapshot plus rolling movement window |

### Authenticated UI review and private read fallback

The authenticated SmartAccounts UI exposes additional capabilities not covered
by an equivalent documented API endpoint. It is predominantly server-rendered
and includes session/CSRF-bound form actions. The importer may use only the
verified **read/export** surfaces discovered through Brave for a documented
gap; the stable signed API stays the first choice. A private read fallback is
implemented behind a versioned adapter and must stop with `REVIEW_REQUIRED` if
its schema, access policy, page/result hash, or source cutoff changes.

| UI area observed | Required coverage decision |
|---|---|
| Periodic invoices and reminders | Supported read-only export, or explicit exclusion |
| Purchase orders | Export bridge; the API only publishes client orders |
| Pending/unhandled documents and e-invoice import inbox | Export original files and processing state; do not substitute a rendered invoice PDF |
| Bank-payment import/export, imported statement rows and matching state | Bank-statement/export bridge and reconciliation; payments alone are insufficient |
| VAT returns and TSD | Export declaration detail, filing state and proof; a VAT-return GL posting is not a declaration |
| Warehouse inventory, movement reports and multi-warehouse valuation | Export bridge plus stock-value reconciliation |
| Fixed assets, depreciation and fixed-asset reports | Export asset-register evidence and depreciation schedule |
| Salary sheets, calculation, reports and settings | Export bridge; API worker/absence data and salary GL entries are not payroll-run detail |
| Annual report and other report outputs | Export/proof evidence; report-row configuration is not a report result |
| Company settings, financial years, defaults, users, billing, devices and audit history | Configure separately or explicitly exclude; never sync credentials or MFA data |

The API service is enabled and Brave is connected through the Chrome debugger
MCP, but SmartAccounts is currently signed out in that session. The published
PDF does not provide URLs for account-balance and warehouse settings services,
so the bridge marks those API methods `brave_discovery_required` rather than
guessing a URL. They may not be skipped in a full-sync run: either the signed
endpoint is verified against the source signature test, or the versioned Brave
read/export fallback must provide the same cutoff/hash evidence. The desired
minimum-click authentication path is a signed-in Brave-session handoff; it may
not transfer browser cookies or reusable session tokens to the NUC.

## Pre-build checklist

- [ ] Hold My Beer OÜ owner approves SmartAccounts as source of truth and
  chooses one-time cutover or continuing mirror.
- [ ] Owner enables a dedicated, least-privilege, read-only SmartAccounts API
  user/group; API key and secret are stored outside Git and rotated successfully.
- [ ] Test company (or vendor-approved test environment) is available for all
  documented GET calls and response-schema captures.
- [ ] Every documented endpoint has a captured schema, count, page/deletion
  behaviour, attachment behaviour and request-budget estimate.
- [ ] Every UI-only area above has a supported export, an approved manual
  evidence process, or an explicit signed exclusion.
- [ ] Accountant selects **one** accounting-authority policy:
  - GL-authoritative: import SmartAccounts ledger entries; keep operational
    documents linked but do not post duplicate Open Accounting GL; or
  - subledger-authoritative: import invoices/payments and only source GL that
    is not represented by Open Accounting postings.
- [ ] Accountant approves source-change and source-delete handling: immutable
  target revisions, voids and reversals rather than in-place updates to posted
  accounting records.
- [ ] Historical start date, cutover date, incremental-sync RPO, evidence
  retention and permitted evidence readers are agreed.

## Open Accounting implementation gaps

The current SmartAccounts code is an offline CSV/XML migration preset only. It
has no signed API client, connection, cursors, source-ID map, sync-run/audit
records, raw-payload manifest or attachment mapping.

- Add tenant-scoped connection and encrypted secret reference, signed GET
  client, rate-budget manager, retry/error policy and circuit-breaker.
- Add connection, cursor, run, run-item, external-identity, payload-hash,
  attachment-link, mapping-decision and tombstone persistence.
- Add adapter rules for the source/target mismatches: cash accounts, generic
  objects, groups, partner contact arrays, payment components/prepayments,
  source rounding/interest/object/warehouse fields and external identifiers.
- Extend document attachment coverage for source clients, vendors and articles,
  or retain these attachments in the encrypted source-evidence store.
- Add an immutable correction workflow for edited/deleted source invoices,
  payments and journals. Open Accounting must never overwrite posted history.
- Represent export-bridge snapshots separately from accounting transactions so
  source reports remain evidence, not blindly imported records.

## Delivery phases

1. **Contract discovery** — execute all read-only published API calls against
   the test company; complete the authenticated UI/export matrix; capture
   redacted schemas and exact request-cost estimates.
2. **Connector foundation** — add tenant connection, secret reference, HMAC
   request signer, cursor/run/audit/external-ID storage and an operator status
   interface.
3. **Initial snapshot** — capture a source-window timestamp; import in
   dependency order (configuration, partners, products/warehouses, commercial
   documents, payments, selected GL, attachments); then replay an overlapping
   change window. Keep all financial writes review-gated.
4. **Export bridges** — ingest immutable source packages for taxes, bank
   statements/matching, fixed assets, payroll runs, recurring documents,
   inventory valuation and report outputs. Record hashes, period and source
   path for each package.
5. **Reconciliation** — prove trial balance, account balances, AR/AP aging,
   bank/cash, VAT/TSD, inventory, fixed assets, payroll, report totals and
   attachment counts/hashes at the same cutoff.
6. **Pilot deltas** — run SmartAccounts -> Open Accounting read-only action
   plans, then reviewed writes. Monitor cursor lag, retries, request budget,
   tombstones, unmapped fields and reconciliation drift.
7. **Production** — enable one-way incremental synchronisation only after
   accountant sign-off, backup/restore testing, alerting and a pause/runbook.

## GL-authoritative execution contract

The archive delivery state `STAGED_REVIEW_REQUIRED` is deliberately not an
apply authorization. The reviewed executor must use the following state
transitions for one selected `(tenant, provider, source_company_id, package)`:

```text
STAGED_REVIEW_REQUIRED
  -> PREVIEW_READY | REVIEW_REQUIRED
PREVIEW_READY + explicit confirm + identical preview digest
  -> APPLYING -> APPLIED | REVIEW_REQUIRED
```

The preview is built entirely server-side from the completed archive. It must
require the manifest declaration `general_ledger_authority: smartaccounts` and
`smartaccounts_gl_authoritative: true`, a completed record/artifact digest,
and an exact selected tenant/source bridge control. It must reject a stale or
variance-bearing ledger before it can become `PREVIEW_READY`.

The executor must persist four tenant-local, immutable audit structures:

1. a preview/reconciliation receipt with package/manifest/record digest,
   selected cutoff, account totals, journal totals, issue list, and preview
   digest;
2. a source-account mapping/import decision keyed by provider/source/account
   external ID and source revision;
3. a financial-posting identity keyed by
   `(provider, source_company_id, resource, external_id)` with the accepted
   revision, OA journal ID, package ID, status, and timestamps; and
4. correction/tombstone review events. A different source revision or
   tombstone is never an in-place overwrite of a posted journal.

Before a journal can be created, every source account must resolve to a
selected OA account or to an explicit chart-import decision. Account code,
name, type, parent/dependency, active state, and source revision must be
reconciled. A changed source account used by a posted journal becomes review
required rather than an automatic chart mutation.

For each verified source general-ledger journal, the executor validates
nonempty external ID/revision, valid date/currency, at least two lines, one
positive debit/credit per line, mapped accounts, and equal base debit/credit.
It creates a draft journal with a source reference, records its durable source
identity in the same transaction, and posts it only under explicit
confirmation. Retry with the same revision returns the existing result;
different revision or deletion stops for an accountant correction/void
workflow. This applies even if a source package is replayed.

Source sales invoices, purchase/vendor invoices, payments, offers, orders,
payroll, bank data, files, and every other non-ledger resource remain archive
records/evidence in the GL-authoritative mode. They may be linked to an OA
object later, but no executor step may make them generate an additional
financial journal. Reconciliation reports counts/digests for those resources
separately from GL debit/credit and account totals.

## Launch gates

- No duplicate GL posting between source invoices/payments and source entries.
- Every source field is mapped, retained as source evidence, or explicitly
  excluded; no silent data loss.
- All retries are idempotent and no cursor advances before the page, its
  dependencies and required files are durable.
- Deletions create source tombstones and reviewed target corrections; they do
  not erase accounting history.
- The calculated 24-hour request budget meets the agreed RPO below the vendor
  limit, including attachment jobs and recovery reserve.
- Evidence encryption, access control, retention, backup and restore are
  verified.

## References

- SmartAccounts API help and method matrix:
  <https://www.smartaccounts.eu/et/abimaterjalid-ja-klienditugi/liidetud-teenused/api/>
- SmartAccounts API v1.7 (28 April 2025):
  <https://www.smartaccounts.eu/uuskodu2015/wp-content/uploads/2025/04/SmartAccounts_API_latest.pdf>
- Vendored source reference:
  `docs/reference/vendor/smartaccounts/SmartAccounts_API_2026-06-12.pdf`
