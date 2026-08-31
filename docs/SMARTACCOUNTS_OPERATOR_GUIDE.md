# Hold My Beer OÜ — SmartAccounts sync operator guide

This is a one-way SmartAccounts → Open Accounting flow. SmartAccounts general
ledger is authoritative. Source invoices, vendor invoices and payments are
archived as evidence only; they never create a second Open Accounting ledger
posting.

## Before starting

1. Sign in to Open Accounting and create or select the **Hold My Beer OÜ**
   tenant. Do not use another company’s tenant.
2. Preferred: keep the existing signed-in **Brave** SmartAccounts tab open and
   use the no-key browser pairing after it is deployed. The locally installed
   relay shares only an opaque selected-company identifier at pairing time;
   it does not copy an API key, cookie, browser token, or ledger data. A
   dedicated read-only API credential remains an explicit fallback only.
3. Decide the historical range that covers payroll absences and warehouse
   movements. SmartAccounts requires an explicit range for those services;
   the application deliberately will not invent a start date.
4. Ensure the NUC has current database migrations through `087` and a healthy
   private bridge. The UI exposes neither the bridge nor source data publicly.

## Minimum-click capture

1. Open **Migration Workbench → SmartAccounts** for the HMB tenant.
2. Choose **Connect with Brave (no API key)**. The installed local relay
   claims a one-time ten-minute pairing from the already signed-in Brave tab.
   This creates only a tenant-scoped opaque source binding and remains
   `AWAITING_BRAVE_BROWSER_CAPTURE`; it cannot read or transfer source data.
   Do not install the relay or start capture until the owner has separately
   approved those actions.
3. API fallback: only if Brave pairing is unavailable, provision a
   least-privilege read-only credential in the bridge's external secret mount,
   enter its opaque `secret-ref://file/<connection-id>` reference, and
   choose **Connect external reference & start full-history capture**.
   - The bridge resolves that reference from the tenant-bound external provider,
     performs one signed account preflight, derives an opaque source identity,
     stores no raw credential in its private state, then starts the read-only
     capture.
   - Open Accounting retains only the opaque binding and safe progress
     metadata. Refreshing the page preserves that opaque binding, not either
     credential reference.
   - The intended replacement is **Connect with Brave session**. The owner
     signs in once in Brave and approves a short-lived, revocable handoff. It
     must never copy an API key, a cookie, or a browser token into Open
     Accounting or the NUC bridge.
4. Watch safe progress. If the API rate limit is reached, use the offered
   **Resume** action after the displayed eligibility time. A retry is bound to
   the same tenant, source, range and selected resources.
5. When prompted, enter the agreed historical `From` and `To` dates and choose
   **Capture missing date-window services**. The `To` date must be at least the
   full capture’s displayed source-as-of date; an earlier completed window is
   not accepted as full coverage. This captures only worker absences and/or
   warehouse movements that need the range; it does not repeat the full-history
   capture. Both capture packages stay visible and durable.

## Selected/all company onboarding

Choose **Read visible SmartAccounts company catalog** and confirm the
metadata-only picker read. The local relay owns the visible picker snapshot;
it sends no rows, cookies, credentials, or financial data. OA issues a
two-minute extension-only capability and persists only its hash. Once the
relay has handed off the exact canonical catalog, the accepted owner receipt
remains selectable for ten minutes.

The panel then requires an explicit choice: **All relay-observed companies**
or **Choose a strict subset**. Nothing is preselected. `All` must include every
company in that durable relay-observed receipt; `Choose` must include at least
one but not every company. This is relay-observed completeness, not a
SmartAccounts API-certified company catalog.

In the separately confirmed create/reuse-and-pair action, Open Accounting's
server—not the browser—does the following independently for each selected
source:

1. Reserves the opaque selector in durable onboarding state. A selector can
   point to only one Open Accounting tenant, and a selected target cannot be
   shared by another source company.
2. Reuses an existing verified browser binding, or one exact-name tenant owned
   by the current user. If neither exists, it creates one source-suffixed
   tenant. It never selects another owner’s tenant and never uses an implicit
   "all tenants" target.
3. Issues an expected-source relay pairing. The relay must claim that exact
   selector before the browser session can configure the source binding; a
   token for one selected company cannot be substituted for another.

The response lists every selected company so an unavailable, ambiguous, or
already-foreign source is marked `REVIEW_REQUIRED` or `FAILED` without
blocking other companies. Retrying the same immutable batch reuses its
reservation and tenant rather than creating a duplicate. If the relay or page
restarts, choose **Resume same pairing batch** and confirm again; this returns
fresh response-only pairing material for the same membership and cannot alter
the catalog or create a new batch. A new accepted catalog receipt may start a
new batch even if its company digest is unchanged.

Only an issued pairing capability is returned once to relay memory; it is not
stored by the page or Open Accounting. Tenant creation and pairing are
non-financial. They do **not** start capture. Every paired target must first
 complete its 31-surface discovery receipt and the required `general_ledger`
(`general_ledger_csv_v1`) schema review. The visible `journal_entries` grid is
summary-only evidence, not an approved CSV posting schema. The partial capture transfer then requires its own later
owner consent. The existing package preview and final **Confirm and apply**
checkbox remain the only route to financial journal writes.

## Brave CSV capture authorization

This is a separate, owner-approved browser-export path. It is deliberately
partial and review-only in this release.

1. On opening the SmartAccounts panel (and when selecting **Check Browser Relay
   again**), Open Accounting sends a fresh 32-byte nonce ping to the installed
   relay. It accepts only a same-window, same-origin reply with the exact v1
   relay, manifest, and workflow-plan versions. The exchange contains only the
   nonce and a `signed_in`, `signed_out`, or `unknown` session state—never a
   company, credential, cookie, source row, or session value. Brave pairing,
   discovery, selected-company onboarding, and CSV capture remain disabled
   until the panel says **Relay ready — SmartAccounts is signed in**. If it is
   missing or stale, reload or update the relay; if it is signed out or unknown,
   sign in (or reload the signed-in SmartAccounts tab) and check again. The
   nonce and readiness state exist only in the mounted page memory.
2. After the Brave session is paired to the selected tenant, enter only the
   historical **History starts** date and confirm the transfer in the same
   action. Open Accounting derives the UTC cutoff and current end date; v1
   allows only the reviewed General Ledger CSV resource (`general_ledger` /
   `general_ledger_csv_v1`). The `journal_entries` grid is summary evidence
   only. Do not guess a
   resource ID or use another tenant’s browser source.
3. The tenant owner starts (or resumes) the server-derived workflow, which
   issues one ten-minute scoped capability. Open Accounting
   stores only its SHA-256 digest plus the tenant, opaque `sa-browser-v1-*`
   source, UUID run, manifest version, and immutable scope. The raw capability is
   returned once to the page and immediately transferred to the installed
   relay; it is not stored in Open Accounting, browser storage, logs, or the
   rendered page. It is reusable only for safe status, retrying an approved
   resource upload, and finalizing that exact run until expiry; it is not a
   single-request token.
   If Brave or its extension restarts, choose **Resume same Brave capture**,
   tick renewed transfer consent, and issue a replacement capability. This
   keeps the same run, tenant, source, manifest, dates, cutoff, and resources;
   it does not start a new bridge capture and invalidates the prior capability.
4. The extension content script transfers the bounded CSV only in memory to
   its service worker. The service worker calls the Open Accounting relay with
   the owner-approved token; Open Accounting accepts only the exact extension
   origin, not `smartaccounts.eu`, page cookies, or a source API key.
5. Same-day retries reuse the immutable workflow and bridge run. Starting the
   same history date on a later UTC day creates a new workflow generation; it
   never extends an old run’s scope. Safe status is limited to `open`, `finalized_partial` with
   `partial_coverage_recorded`, or `finalized_full_blocked` with
   `full_coverage_blocked`. A partial receipt cannot post journals or trigger
   automatic apply. It may later be compiled into the existing staged-package
   review flow once its coverage package is complete and checksum-verified.
   Use **Refresh safe capture status** after finalization or a restart. This
   tenant-owner view reads the existing immutable run without issuing or
   exposing a relay capability. It labels a partial browser scope as partial;
   when the checksum-finalized status becomes `staged_review_required`, Open
   Accounting prepares the non-mutating package preview once automatically.
   Preview never applies data; the final checkbox and financial-apply action
   remain explicit.

The relay URLs are tenant and run scoped:

`/api/v1/smartaccounts-browser-captures/tenants/{tenantID}/runs/{runID}`

They accept only `Authorization: Bearer <capture-token>` from the Brave
extension origin. CSV upload is `PUT .../resources/{resourceID}` with a raw
`text/csv` body of at most 32 MiB and a lower-case
`X-SA-Browser-Resource-SHA256` digest. Finalization is `POST .../finalize`
with `{}` JSON. These are relay endpoints, never browser-cookie or financial
write endpoints.

The owner UI status route is separate from the relay route:

`GET /api/v1/tenants/{tenantID}/smartaccounts-sync/browser-captures/{runID}`

It returns only the bound tenant/source/run scope, structured coverage receipt,
and safe package-staging digest/count status. It never returns the relay token,
its hash, CSV data, cookies, credentials, or bridge error body.

The guided workflow endpoints are tenant-owner-only and return no source data:

`POST /api/v1/tenants/{tenantID}/smartaccounts-sync/browser-capture-workflows`

`GET /api/v1/tenants/{tenantID}/smartaccounts-sync/browser-capture-workflows/{workflowID}`

The POST body is limited to `source_company_id`, `from_inclusive`, and
`transfer_consent_confirmed`. `false` consent can safely return a plan but no
capability; the UI sends `true` only at the owner’s transfer action. The raw
capability is emitted directly to extension memory with the
`open-accounting.smartaccounts-browser-workflow-issued.v1` envelope. Its plan
repeats tenant/source/run/manifest/scope and declares
`eligible_resource_ids:["general_ledger"]`; the extension rejects a mismatch.
The reviewed source schema is `general_ledger_csv_v1`. The visible
`journal_entries` grid is summary evidence only and never becomes a posting
source.

## Brave browser contract discovery receipt

This is a separate metadata-only prerequisite; it is neither a CSV transfer
nor a financial operation.

1. After the browser source is paired and the panel says **Relay ready —
   SmartAccounts is signed in**, choose **Discover browser contracts**. The UI
   asks for an explicit metadata-only consent. It derives the complete fixed
   31-surface `smartaccounts-brave-ui-v2` manifest on the server; an operator
   cannot submit a manual resource list, and the journal surface alone is not
   full discovery.
2. The optional second checkbox permits only a bounded response-header-name
   probe for CSV surfaces. Leave it unchecked for ordinary metadata discovery.
   It never permits a header value, CSV body, source row, cookie, credential,
   URL query, hidden field, capture, package staging, or accounting apply.
3. The relay gets the tenant/source/discovery UUID binding and action-time
   consent in a same-window event. It has at most ten minutes to inspect the
   fixed manifest; the extension independently rejects stale consent (older
   than two minutes at issue) and expired/mismatched events.
4. The only rendered result is a **Redacted discovery receipt**: status,
   SHA-256 integrity digest, and aggregate counts for capture-ready,
   filter-review, page-only, private-endpoint, and binding-blocked surfaces.
   The receipt never reveals the paired source selector, routes, control IDs,
   header names/values, cookies, source rows, credentials, consent record, or
   bridge token.
5. A `completed` receipt covers all 31 authorized surfaces. `expired` or
   `discovery_failed` can safely record only the inspected subset; use a new
   owner consent and discovery action to retry. A receipt is evidence for
   follow-up review only—it does not broaden the journal CSV workflow and
   cannot post accounting data.

## Required coverage and reconciliation gates

The full-history run intentionally reports two vendor-documentation gaps:
`general.account_balances.get` and `settings.warehouses.get`. Their endpoint
URLs are absent from SmartAccounts API v1.7. The bridge will never guess them.
Full coverage remains blocked until either SmartAccounts confirms the signed
read-only API endpoints or a user connects the approved Brave debugger and a
read-only request contract is verified.

Use the finalized package only when all of the following are true:

- required date-window services are archived;
- account-balance and warehouse discovery gaps are resolved with a verified
  read-only contract;
- the archive package is `STAGED_REVIEW_REQUIRED` with complete digests;
- the source chart has either explicit mappings or reviewed safe chart-import
  decisions; and
- the accountant accepts reconciliation issues, cutoff and source as-of date.

These conditions are enforced by the server as well as the UI. Calling the
preview endpoint directly cannot skip a missing date-window, a window that
ends before the full-capture cutoff, or an endpoint discovery requirement.

## GL preview and apply

For a complete full-history package, choose **Prepare chart and GL preview**.
Review every proposed account import/mapping, journal count, balance check,
cutoff, and any issue. The final checkbox is intentionally separate. Selecting
**Confirm and apply GL plan** posts each approved SmartAccounts journal once,
using source external ID plus revision for idempotency. Exact replay is a
no-op; changed revisions and tombstones become correction review items.

Do not apply merely to test the UI. Applying is the only step that can create
or post Open Accounting journals.

## Exact-match policy and reconciliation approval

Migration `087_smartaccounts_reconciliation_receipts` adds the mandatory
multi-actor gate around GL apply and selected/all completion. It stores only
IDs, fixed status codes, counts and SHA-256 handles—never source rows, proof
payloads, names, amounts, mappings, browser tokens or credentials.

1. An active accountant opens the selected source and asks OA to derive the
   `smartaccounts-exact-match-v1` policy candidate for the exact staged package
   and GL preview. OA supports zero variance only; the browser cannot submit a
   tolerance, rate, amount or free-form digest.
2. The accountant explicitly confirms that server-derived candidate. OA binds
   the immutable policy to the tenant, opaque source, package, scope and exact
   preview. The candidate digest is forgotten by the page after confirmation.
3. A different financial operator reviews the GL preview and confirms apply.
   The page resolves the current policy immediately and submits only its opaque
   `tolerance_policy_id`; OA rechecks the policy binding and actor separation.
   The policy approver cannot apply the GL.
4. OA streams the protected archive and exact posted journals in memory,
   verifies payload revisions, mapping/identity snapshots, source IDs, posted
   state, line semantics, VAT flags and original/base debit-credit equality,
   and persists only the resulting proof handles.
5. An independent accountant reads the tenant/batch/source-bound safe
   evaluation and confirms the exact evidence and tolerance handles. API
   tokens are rejected for policy confirmation and final attestation. Stale or
   changed package, preview, mapping, journal, proof or unresolved revision
   state invalidates an earlier ready/pass result.

`PASS` means every originally selected source has current independently
attested evidence. A partial browser GL capture remains partial even if its
own proof is internally exact; it cannot satisfy a full-history/all-domain
claim. `PARTIAL_FAILURE` is terminal only after every selected source has
reached a terminal state.

## Non-financial reference-master preview and apply

Migration `079_smartaccounts_reference_master_state` adds a separate,
tenant-and-source-bound path for a staged package's reviewed non-financial
masters. It is available for a complete or explicitly partial package; it does
not turn a partial package into a claim of complete history.

Choose **Prepare non-financial master preview** to inspect only the source
schemas that currently have a reviewed native OA projection:

- `account` from `settings.accounts.get`;
- `customer` and `vendor` from a delivered
  `smartaccounts-browser-master-detail-v1` client/vendor snapshot only when
  the canonical record proves its reviewed ISO-2 country code.

Browser `articles` are deliberately **review-only**: the visible SmartAccounts
VAT selector is not a proven numeric rate, so OA has no approved VAT-rate
projection and must never infer or default 22 percent. Article payloads remain
protected evidence until a separate owner-reviewed VAT mapping exists.

The preview exposes only source external IDs, deterministic target IDs, action
kind, reconciliation counts and review issues. Canonical payloads stay in the
tenant archive. It never creates a journal, invoice or payment.

Use **Confirm and apply reference masters** only after reviewing the exact
preview digest. Each source identity is tenant/provider/source/entity/external
ID scoped. Exact replay is a no-op; an interrupted exact revision can resume
only after OA verifies the deterministic target contains the exact projected
fields. A changed revision, tombstone, local code/name collision, unsupported
warehouse article, or incomplete source mapping is review-required and never
overwrites an OA master. Source account defaults, article account links and all
article VAT treatment remain for manual mapping review.

## Brave discovery

Use the connected **Brave** session through the Chrome debugger MCP. The MCP
name does not require a separate Google Chrome installation. Enable the
ChatGPT/Codex browser extension in Brave through **Settings → Computer use**,
then sign in to SmartAccounts normally. The inspection must be read-only and
capture only request method, URL shape and response schema—never browser
cookies, credentials, or source data. If the tab is signed out, the owner must
sign in in Brave before discovery can continue.

## Incident and recovery

- `rate_limited`: wait for the displayed time, then resume the same run.
- `interrupted`: resume the exact run; sealed source pages and archive chunks
  are idempotent.
- source key rotation: reconnect and validate; the bridge blocks capture until
  the new source identity/snapshot is verified.
- mapping, balance, revision, deletion or stale-data issue: do not apply.
  Resolve through accountant review and create a new preview.
- no data is deleted by this connector. Source archive chunks, progress and
  posting identities remain tenant-isolated evidence.
