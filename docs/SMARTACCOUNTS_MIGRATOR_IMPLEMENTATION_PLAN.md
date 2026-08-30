# SmartAccounts migrator: implementation plan

## Decision summary

Create a new private repository, **`HMB-research/smartaccounts-migrator`**,
containing a Go 1.26 CLI named **`sa-migrate`**. It is the only component that
reads SmartAccounts credentials and it is responsible for extraction,
normalisation, private evidence, package creation, and local checkpoints.

Open Accounting receives no SmartAccounts secrets. It owns tenant
authorisation, package staging, mapping review, validation, immutable target
application, reconciliation, audit history, and the in-product import
workbench.

```text
SmartAccounts API + supported exports
                │
                ▼
       sa-migrate (private host)
  capture → normalise → package → stage/watch
                │
     encrypted, content-addressed artifacts
                ▼
       Open Accounting Import Session v1
 validate → map → plan → approve → apply → reconcile
```

The existing `/migration/*` CSV/XML flow remains a **one-time compatibility
bridge**, not the live-sync contract. It lacks source identities, source
cursors, tombstones, revision handling, and durable per-resource import state.

Execution is tracked in [the file-based progress board](./SMARTACCOUNTS_MIGRATOR_PROGRESS.md)
and [the implementation work plan](./SMARTACCOUNTS_MIGRATOR_EXECUTION_PLAN.md).

## Scope and non-negotiable safety rules

- Direction is initially one-way: **SmartAccounts → Open Accounting**.
- The CLI uses SmartAccounts GET endpoints only. It never replays undocumented
  SmartAccounts UI form posts and never writes to SmartAccounts.
- A run has one accountant-approved accounting authority: `gl_history_authoritative`,
  `subledger_authoritative`, or `opening_balances_and_open_items`.
- A changed or deleted posted source transaction is represented by a reviewed
  target correction, void, reversal, adjustment, or evidence-only decision;
  no posted history is overwritten or silently deleted.
- Every source field and source resource must be mapped, retained as evidence,
  or explicitly excluded. There is no implicit data drop.
- SmartAccounts API keys, secrets, bearer tokens, raw source captures, exports,
  invoice files, and proof reports are private. They must not appear in Git,
  command arguments, shell history, logs, packages, or Open Accounting's
  configuration database.

See [the source coverage checklist](./SMARTACCOUNTS_FULL_SYNC_PLAN.md) for the
API versus export-bridge classification.

## Repository plan

```text
smartaccounts-migrator/
├── cmd/sa-migrate/
├── internal/
│   ├── axi/                 # stdout/errors/help contract
│   ├── config/              # profiles and policy validation
│   ├── secrets/             # file:, env:, OS-keyring resolvers
│   ├── smartaccounts/
│   │   ├── signer/          # raw-query HMAC-SHA-256 signer
│   │   ├── client/          # read-only transport and response parsing
│   │   ├── catalog/         # tested v1.7 resource definitions
│   │   └── scheduler/       # pagination, quota, retry and backoff
│   ├── capture/             # full and delta collection
│   ├── bridge/              # validated supported-export ingestion
│   ├── canonical/           # raw source to canonical NDJSON records
│   ├── mapping/             # field mapping and reviewed overrides
│   ├── workspace/           # SQLite state, manifests, hashes and artifacts
│   ├── target/              # OA capability, Import Session v1 and legacy adapter
│   ├── progress/            # local snapshot and OA event watching
│   └── reconcile/           # proof plan and comparison validation
├── schemas/
│   ├── migration-package/v1/
│   ├── mapping/v1/
│   └── export-bridge/v1/
├── testdata/                # synthetic/irreversibly redacted fixtures only
├── docs/
│   ├── USER_GUIDE.md
│   ├── OPERATIONS_RUNBOOK.md
│   ├── RESOURCE_COVERAGE.md
│   ├── MAPPING_REFERENCE.md
│   └── SECURITY.md
└── .github/workflows/
```

Use semantic versions for the CLI; use independently versioned package,
mapping, and export-bridge schemas. Pin the SmartAccounts API contract version
and PDF hash; `source probe` records any observed schema drift before a run.
The target is chosen by a capability handshake, never by assuming a particular
Open Accounting commit is deployed.

## CLI contract

The CLI follows AXI conventions:

- Default stdout is compact TOON; `--output json` is supported for automation.
- Data and structured errors are written to stdout; progress and diagnostics
  are written to stderr. Exit codes are 0 success/no-op, 1 operational error,
  and 2 usage error.
- Unknown flags fail before any network call. The tool never prompts; every
  action is flag-complete. `-v`, `-V`, and `--version` return a bare version.
- The no-argument view reports compact workspace/run state and next commands.
  Every subcommand has focused `--help` and 2–3 safe examples.
- Optional agent hooks are explicit opt-in only. An installable `SKILL.md` is
  generated from the non-live CLI guidance and CI checks it for drift.

```text
sa-migrate init --workspace <private-dir> --profile hold-my-beer
sa-migrate secrets configure --profile hold-my-beer --smartaccounts-ref file:/...
sa-migrate doctor --profile hold-my-beer
sa-migrate source probe --profile hold-my-beer --resources all --max-requests 50

sa-migrate capture full --profile hold-my-beer --cutoff <rfc3339-tallinn-time>
sa-migrate capture delta --profile hold-my-beer
sa-migrate bridge ingest --package <dir> --kind fixed-assets --input <private-export>
sa-migrate package validate --package <dir>
sa-migrate map check --package <dir>

sa-migrate import prepare --package <dir>
sa-migrate import upload --session <id>
sa-migrate import validate --session <id>
sa-migrate import plan --session <id>
sa-migrate import watch --session <id>
sa-migrate import apply --session <id> --approval-file <private-file> --confirm-session <id>
sa-migrate reconcile plan --session <id>
sa-migrate reconcile verify --result <private-proof-result>
```

`import apply` is the first mutating operation. It must require the exact
session ID, plan digest, second-actor approval and a separate confirmation
value. No `--confirm`-only shortcut is allowed.

## Configuration, secrets and workspace

Profiles contain identifiers, policy, and secret **references**, never values.

```yaml
profile: hold-my-beer
source:
  provider: smartaccounts
  company_id: "14369460"
  api_contract: "1.7"
  api_key_ref: file:/home/clawdy/private/smartaccounts/hold-my-beer/secrets/api-key
  api_secret_ref: file:/home/clawdy/private/smartaccounts/hold-my-beer/secrets/api-secret
target:
  base_url: http://server-nuc:8082
  tenant_id: <open-accounting-tenant-id>
  token_ref: file:/home/clawdy/private/smartaccounts/hold-my-beer/secrets/oa-token
policy:
  mode: one_time_cutover # or continuing_mirror
  accounting_authority: gl_history_authoritative
```

- The workspace is `0700`; secrets, SQLite state, and logs are `0600`.
- On `server-nuc`, run as `clawdy` and keep workspace paths under
  `/home/clawdy/private/`, never a Git checkout.
- Packages are content-addressed and encrypted with a versioned recipient set
  before they leave the host. Private raw payloads and export files remain
  private evidence; only policy-permitted artifacts are uploaded to OA.
- The SmartAccounts signer uses the exact encoded raw query and body for
  HMAC-SHA-256, Europe/Tallinn time, and a preflight clock-skew check.

## Source capture and canonical package v1

The scheduler enforces capacity below SmartAccounts' documented 60
requests/minute and 1,000 requests/day, reserves recovery capacity, and uses
bounded exponential backoff for transient failures. It never blindly retries
400, 401, mapping, signature, or billing failures.

| Source resource class | Collection strategy |
|---|---|
| Change-history endpoints | Overlapping modified-since window, all pages, first-page deletion list, source ID plus payload-hash deduplication |
| List-only settings | Full snapshot plus periodic hash/diff |
| Files | Parent-by-parent file list and exact-byte content hash |
| Account balance | Point-in-time reconciliation, no more than once/hour |
| Article quantities | Batches capped by the documented source limit; capture the default-warehouse limitation |
| Warehouse movements | Rolling date-window snapshot and fingerprint; no unsupported cursor claim |
| UI-only capability | Immutable supported export bridge or signed exclusion; no UI POST replay |

```text
<package-id>/
├── manifest.json
├── hashes.sha256
├── raw/api/<resource>/page-0001.json.gz
├── raw/files/<parent-type>/<parent-id>/<file-id>.bin
├── canonical/<entity>.ndjson
├── bridges/<kind>/
├── mapping/decisions.yaml
├── validation/report.json
├── reconciliation/plan.json
└── audit/events.jsonl
```

Each canonical record includes source provider/company/entity/external ID,
revision, `upsert` or `delete` operation, observation time, relationship
external IDs, payload SHA-256, and structured payload. The manifest includes
all artifact hashes and counts, source window/cursors, deletion counts,
mapping-policy digest, import-policy digest, package signature, and CLI/API
versions. It contains no credentials or secret-bearing URLs.

The local private SQLite state records atomic page checkpoints, cursors,
overlap windows, tombstones, retries, artifact hashes, mapping decisions,
target session IDs, and per-resource counts. It advances a cursor only after
the raw payload, canonical records, hashes, dependencies, and target
acknowledgement are durable.

## Open Accounting: Import Session v1

Add a new tenant-scoped import contract rather than overloading the legacy
`/migration/*` endpoints.

| Endpoint | Responsibility |
|---|---|
| `GET /import-capabilities` | Advertise Import Session schemas, limits, feature support, required scopes and target version |
| `POST /imports` | Create a source-bound session from non-secret manifest metadata |
| `PUT /imports/{id}/artifacts/{sha256}` | Resumable/chunked private artifact upload with expected hash and size |
| `POST /imports/{id}/verify` | Verify package schema, company, signature, hashes and cursor continuity |
| `POST /imports/{id}/validate` | Stage canonical records and report blockers/warnings/counts |
| `GET/PUT /imports/{id}/mappings` | Read/write a versioned mapping set; immutable after planning |
| `POST /imports/{id}/plan` | Build deterministic dependency plan with a digest |
| `GET /imports/{id}/review` | Show planned actions, conflicts, evidence and reconciliation gates |
| `POST /imports/{id}/approve` | Record accountant/owner review of the exact plan digest |
| `POST /imports/{id}/commit` | Apply with `If-Match` plan digest, session-specific idempotency key and separation of duties |
| `POST /imports/{id}/resume` | Resume from durable checkpoint without repeating committed work |
| `GET /imports/{id}` and `/events` | Query status and durable SSE progress stream |
| `POST /imports/{id}/reconcile` and `/close` | Persist reconciliation, waivers, sign-off, retention and final hashes |

Add scopes `imports:read`, `imports:stage`, `imports:commit`,
`imports:approve`, and `imports:evidence`. The CLI service token can stage but
cannot approve. Full-history/production commits require a different approved
actor from the run creator.

### Required persistence

Add tenant-schema tables for runs, artifacts, staged records, external entity
links, mappings, conflicts, approvals, reconciliation checks, events and
attachment links. `external_entity_links` must be unique on tenant, provider,
source company, entity type and external ID. Do not misuse the current UUID-only
journal `source_id` field for SmartAccounts IDs.

Store private artifact references and hashes in OA; do not store API keys or
unbounded raw source data in migration-run JSON. Extend document support for
contacts, products, employees, warehouses, or add a generic immutable source
evidence record for their files. Migration artifact transfer must be resumable;
the interactive document upload size limit remains separate.

### Validation and apply rules

1. Verify manifest/schema/signature/hash, source-company identity, artifact
   sizes, policy and cursor continuity.
2. Validate canonical structure, external identities, dates, currencies,
   amounts, source revisions, duplicate records and attachment checksums.
3. Resolve every relation from staged data, an approved external link, or an
   explicit mapping. Never silently match on an ambiguous name.
4. Apply accounting checks: account/VAT/period validity, journal balance,
   invoice and payment totals, stock/fixed-asset continuity, GL authority,
   evidence and period locks.
5. Resolve or explicitly waive every warning. Any mapping, package, policy,
   cursor, or relevant target-state change invalidates a completed plan.
6. On commit, call existing OA domain services through transaction-aware
   adapters. A row-level service error fails its aggregate; it cannot become a
   false successful step.
7. Record external links only after the target transaction commits; same source
   revision/hash is a no-op. Changes to posted records create reviewed
   corrections; deletion events create tombstones.

## Progress and governance

The CLI and OA show separate but linked state:

```text
CLI: DISCOVERED → CAPTURING → NORMALISED → VALIDATED → AWAITING_APPROVAL
     → IMPORTING → RECONCILING → COMPLETED

OA:  CREATED → UPLOADING → VERIFIED → STAGED → VALIDATED → MAPPING_REQUIRED
     → PLANNED → REVIEW_REQUIRED → APPROVED → COMMITTING → RECONCILING
     → PASSED | FAILED | BLOCKED | CANCELLED
```

Every event includes run and phase IDs, entity/resource type, completed/total
records and bytes, source cursor/window, deletion/warning/retry counts, quota
usage, checkpoint, redacted error, and duration. The database event log is
authoritative; CLI watch/SSE is observational.

For each resource the UI shows `not_started`, `extracting`, `staged`, `mapped`,
`validated`, `imported`, `reconciled`, `excluded_with_evidence`, `blocked`, or
`failed`, plus source/staged/mapped/imported/skipped/deleted counts and the
reconciliation disposition.

## Delivery milestones and acceptance gates

| Milestone | Deliverable and exit criteria |
|---|---|
| M0: Governance | Named owner/operator/accountant; source-of-truth policy, history/cutover dates, RPO, evidence retention and every UI-only export/exclusion decision approved |
| M1: CLI foundation | Private repo, reproducible release, secure workspace/secrets, AXI contract, capability matrix and synthetic fixtures |
| M2: Discovery | Every documented source endpoint contract-tested; all UI/export gaps classified, request budget measured |
| M3: Capture/package | Encrypted versioned package, durable checkpoints, maps, hashes and bridge artifacts; no OA writes |
| M4: OA Import Session | Capabilities handshake, staging/validation/planning API and UI, tenant isolation, progress and approvals |
| M5: Core API import | Configuration, masters, commercial documents, payments, selected GL and attachments have mappings, idempotency and reconciliation |
| M6: Export bridges | Bank/matching, taxes, payroll runs, assets, periodic documents, inventory valuation and reports are evidence-backed or explicitly excluded |
| M7: Pilot | Disposable tenant snapshot/delta/retry/correction rehearsal, full applicable reconciliation, restore drill |
| M8: Cutover | Approved Hold My Beer OÜ production snapshot, monitoring, pause/runbook and accountant sign-off |
| M9: Ongoing mode | One-way scheduled deltas meet RPO, rate budget and recurring reconciliation requirements |

No milestone closes on code alone: it requires the deliverable, automated
evidence, resolution/waiver of blocked resources, and required human sign-off.

## Test plan

| Layer | Required coverage |
|---|---|
| Unit/property/fuzz | HMAC raw-query/body output, Tallinn/DST clock handling, pagination, delete list, quota, retry, hashes/encryption, crash recovery, decimal/VAT/currency, normalisers and mapping rules |
| Source contracts | Versioned redacted fixtures for every API/export schema; optional request-budgeted test-company GET suite; schema drift blocks writes |
| OA domain/API | Package/artifact integrity, scopes, dual approval, mapping completeness, external-link idempotency, conflicts, tombstones, immutable correction and progress events |
| Database integration | Real fresh tenant schema, resume at all transaction boundaries, duplicate package no-op, target-state drift plan invalidation, chunk reassembly, tenant/artifact isolation |
| End-to-end | Mock SmartAccounts plus disposable OA tenant: discovery → capture → package → upload → dry-run → approval → apply → reconcile → delta; forced process/network failure at each phase |
| Reconciliation | Same-cutoff trial balance, account balances, AR/AP, bank/cash, VAT/TSD, payroll, stock/valuation, fixed assets, report totals and attachment count/hash parity |
| Security/resilience | Secret/log scan, private mode checks, authz/tenant tests, filename/size attacks, rate exhaustion, 500/503, clock skew, storage corruption, SBOM/dependency/static analysis |

Commit only synthetic or irreversibly anonymised fixtures. Keep raw captures,
source exports, PDFs and proof reports in encrypted private storage. Every
production defect receives a minimised regression fixture.

## Operator guide

### 1. Provision and govern

1. Create the Open Accounting tenant and assign an administrator, integration
   operator and accountant.
2. In SmartAccounts, enable a dedicated read-only API user/group for Hold My
   Beer OÜ. Use a test company first where possible.
3. Agree whether this is an initial cutover, an ongoing mirror, or both;
   choose authority policy, history/cutover dates, RPO, retention and correction
   policy.
4. Provision a `clawdy` private workspace and place secret files there. Never
   paste a credential into chat, Git, a command line or a package.

### 2. Initialise and discover

```bash
sudo -u clawdy install -d -m 700 /home/clawdy/private/smartaccounts/hold-my-beer
sa-migrate init \
  --workspace /home/clawdy/private/smartaccounts/hold-my-beer \
  --profile hold-my-beer
sa-migrate secrets configure --profile hold-my-beer \
  --smartaccounts-ref file:/home/clawdy/private/smartaccounts/hold-my-beer/secrets/api-key
sa-migrate doctor --profile hold-my-beer
sa-migrate source probe --profile hold-my-beer --resources all --max-requests 50
```

Resolve every `unknown`, `unsupported`, and `requires_export` discovery result
before capture. Add a supported private export for each applicable bank,
declaration, payroll-run, fixed-asset, inventory-valuation, periodic-document,
or report area. Do not automate undocumented UI POST endpoints.

### 3. Capture, bridge, map and package

```bash
sa-migrate capture full --profile hold-my-beer --cutoff <rfc3339-tallinn-time>
sa-migrate bridge ingest --package <package-dir> --kind fixed-assets --input <private-export>
sa-migrate package validate --package <package-dir>
sa-migrate map check --package <package-dir>
```

Review mapping conflicts and choose a disposition for every field. Preserve
external IDs, tax treatment, currency, dates, accounting policy and source
file/evidence status. A changed mapping creates a new plan rather than editing
an approved plan in place.

### 4. Stage, review and approve in Open Accounting

```bash
sa-migrate import prepare --package <package-dir>
sa-migrate import upload --session <session-id>
sa-migrate import validate --session <session-id>
sa-migrate import plan --session <session-id>
sa-migrate import watch --session <session-id>
```

In **Hold My Beer OÜ → Integrations → SmartAccounts**, review target actions,
warnings, evidence coverage, quota estimate and reconciliation gates. An
accountant/owner who did not create the session approves the exact plan digest.

### 5. Pilot, apply and reconcile

1. Apply the package to a disposable tenant first; rehearse interruption and
   resume.
2. Generate source and target evidence at the identical cutoff/basis.

```bash
sa-migrate reconcile plan --session <session-id>
sa-migrate reconcile verify --result <private-proof-result>
```

3. Fix problems using a successor package; do not edit or retry blindly.
4. For production, verify the OA backup/restore drill, freeze the source
   window, create a fresh package, validate, plan, approve and then apply:

```bash
sa-migrate import apply --session <session-id> \
  --approval-file <private-approval> --confirm-session <session-id>
```

5. Mark cutover complete only after applicable reconciliation checks pass or
   have documented accountant-approved waivers.

### 6. Ongoing operation and incident response

```bash
sa-migrate capture delta --profile hold-my-beer
sa-migrate import watch --session <session-id>
sa-migrate reconcile verify --result <private-proof-result>
```

Daily, review cursor lag, quota headroom, failures, tombstones and unmapped
values. Monthly, re-run report reconciliation and list-resource diff. Rotate
secrets and test backup/restore and pause/resume at least quarterly.

To handle an incident: pause future jobs, preserve package/checkpoint/log and
evidence hashes, classify staged versus posted impact, create a corrected
successor or accountant-approved reversal/adjustment, reconcile, record the
incident, and only then resume. Never use a restore to erase an unknown mixed
accounting state.

## Remaining product decisions

1. Confirm the separate repository name, ownership and private release channel.
2. Choose package transfer: OA artifact upload versus a mutually accessible
   private object store.
3. Select the accounting-authority policy and decide whether first production
   work is one-time history, ongoing mirror, or both.
4. Approve the first-release treatment for UI-only resources: export import,
   evidence-only, or deferred.
5. Name the tenant owner, accountant approver, historical start/cutover date,
   RPO, retention period and reconciliation tolerances.
