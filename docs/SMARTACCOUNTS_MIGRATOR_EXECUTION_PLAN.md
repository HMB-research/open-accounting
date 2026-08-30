# SmartAccounts migrator execution plan

This plan turns the design into sequential, testable implementation work. It
does not authorise production import, API-key generation, or SmartAccounts UI
automation. Those actions remain gated by the progress board and accountant
approval.

## Work package 0: close design and access gates

### Deliver

1. A signed cutover charter specifying company, operating mode, source-of-truth
   policy, history start, cutoff, period locks, foreign-exchange/rounding basis,
   VAT/tax basis, RPO, retention, approvers and reconciliation tolerance.
2. The field-level coverage ledger and UI-only export-bridge register.
3. The SmartAccounts source authentication runbook below.
4. Threat model and evidence handling design: data classifications, trusted
   hosts, secret/key custodian, package encryption, malware policy, audit and
   recovery procedure.
5. Measured volume, attachment, storage, bandwidth and daily-request budget.

### Acceptance

- No `blocked`, unknown, or unowned resource/field may appear in the intended
  initial migration scope.
- The accountant has approved one GL/subledger/opening authority policy.
- A read-only source probe passes with an API user group that has no unnecessary
  mutation permissions.
- Snapshot design proves complete `T0` to `T1` coverage under concurrent source
  change.

## SmartAccounts authentication and authorisation flow

### Principles

SmartAccounts uses a company API key plus secret, URL timestamp and HMAC-SHA-256
signature. Treat the API secret as source-system root material for this
integration: it is local to the CLI host, never uploaded to Open Accounting,
and never printed.

### Provisioning flow

```text
Company administrator
  → confirms paid/API eligibility
  → creates dedicated API user group
  → assigns required read permissions from coverage ledger
  → enables Connected Services API and generates company API keys
  → transfers key/secret through approved private secret placement

Integration operator
  → writes 0600 secret files under clawdy private workspace
  → configures only file:/keyring secret references in sa-migrate profile
  → runs read-only doctor and source probe
  → records key fingerprint, API contract version and permission test evidence

sa-migrate
  → creates ddMMyyyyHHmmss Europe/Tallinn timestamp
  → canonicalises the exact encoded query/body bytes
  → computes HMAC-SHA-256 hex signature
  → sends only HTTPS request to approved SmartAccounts hostname
  → redacts API key/query/signature in all logs and diagnostics

Open Accounting
  → receives package/artifact hashes and a scoped target token only
  → never receives SmartAccounts credentials
```

### Brave-debugger fallback for non-public retrievals

The public API remains the default contract. If an in-scope field is unavailable
there and a supported export is unavailable or inadequate, a **browser-backed
retrieval bridge** may be investigated. It is a last resort, not a general UI
scraper.

1. Use Brave's debugger with an authenticated **test-company** session to
   observe the request while performing the normal read-only UI action.
2. Create a versioned request contract containing only redacted method/path,
   request and response schema, source page, pagination/filter semantics,
   expected response type, rate behaviour, idempotency, and the SmartAccounts
   release/browser version observed.
3. Classify it as `read_only_retrieval`, `ambiguous`, or `state_changing`.
   Only the first class can proceed. A POST is not automatically read-only:
   its effect must be proven in the test company.
4. Reproduce it against the test company through an isolated browser bridge;
   compare response schema and source report totals. Add a recorded, redacted
   contract fixture and a schema-drift alarm.
5. Obtain owner/accountant approval and add the bridge to the field-coverage
   ledger. A source/UI version change, contract diff, or failed no-side-effect
   check blocks production capture.

The bridge executes inside the user-authorised Brave session. It does **not**
export cookies, session tokens, passwords, MFA codes, API secrets, or browser
profiles to `sa-migrate`, Open Accounting, logs, artifacts, or test fixtures.
If a usable integration requires copying session credentials or replaying a
state-changing UI call, it is rejected in favour of a supported export or
vendor-confirmed API.

### Permission design

Start with a source read-only group. Enable only resource permissions proven
necessary by accepted scope, including file-read access only when the source
attachment type is in scope. Remove general save, delete and file-change
permissions. The precise SmartAccounts permission labels and semantics must be
captured in the source probe evidence; no assumed privilege mapping is allowed.

The Open Accounting token is separate and scoped to `imports:stage` plus read
status only. It cannot approve or commit a production run. A different
accountant/owner approves the exact plan digest.

### Bootstrap and test commands

These are proposed commands, not commands available today:

```bash
sa-migrate init --workspace /home/clawdy/private/smartaccounts/hold-my-beer \
  --profile hold-my-beer
sa-migrate secrets configure --profile hold-my-beer \
  --smartaccounts-key-ref file:/home/clawdy/private/smartaccounts/hold-my-beer/secrets/api-key \
  --smartaccounts-secret-ref file:/home/clawdy/private/smartaccounts/hold-my-beer/secrets/api-secret
sa-migrate doctor --profile hold-my-beer
sa-migrate source probe --profile hold-my-beer --resources all --max-requests 50
```

`doctor` verifies file modes/ownership, secret references without echoing them,
TLS and hostname policy, local clock health, source timestamp/signature
verification, capability/permission responses, and the planned request budget.

### Rotation, revocation and failure handling

1. Pause capture before credential rotation and preserve cursor/checkpoint/run
   state.
2. An administrator generates/replaces the source credential according to the
   capabilities verified at that time; do not assume dual active keys exist.
3. Operator replaces the private secret reference, runs `doctor`, then resumes
   only with the same source window/cursor validation.
4. Revoke compromised or unused credentials immediately and mark related runs
   for security review. Rotate the OA token independently.
5. `401`, signature failure, unexpected redirect, hostname mismatch, clock
   failure, source billing error, or permission denial changes a run to
   `BLOCKED`; capture stops without cursor advance. The operator fixes access
   and runs `doctor` before an explicit resume.

## Work package 1: create the CLI foundation

### Build

- Create private `HMB-research/smartaccounts-migrator` repository and release
  policy.
- Implement `sa-migrate` home/help/version and AXI stdout/error contract.
- Implement profile validation, private workspace initialisation, `file:`,
  `env:` and OS-keyring secret-reference resolvers, redacted structured logging
  and local SQLite WAL state.
- Add reproducible Go build, dependency/SBOM/license/vulnerability checks,
  signed artifact release and synthetic fixture policy.

### Tests

- Unknown flag/required flag/no-prompt/output-channel/version fast-path tests.
- Secret-redaction tests for argv, configs, logs, errors, state and manifests.
- Workspace ownership/mode, path traversal, unsafe filesystem, disk-full and
  corrupted-state recovery tests.

### Exit criteria

`sa-migrate doctor` succeeds with no source data, and the private workspace,
redaction and release checks pass in CI.

## Work package 2: build read-only SmartAccounts capture

### Build

- Implement exact-query/body HMAC signer and TLS/redirect/host guard.
- Implement resource catalogue, request budgeter, retry classifier, page
  iterator, source deletion collector and attachment downloader.
- Implement full snapshot and delta collectors with the following exact
  consistency protocol:
  1. record `T0` before the first mutable-resource request;
  2. capture static resources and all mutable-resource pages to a private,
     hashed raw store;
  3. collect each page's deletion list and terminal page marker;
  4. replay mutable resources from an overlapping `T0` window;
  5. record final `T1`; replay again if any cursor/window has not reached `T1`;
  6. only then mark package capture complete.
- Add separate full-list diff for list-only resources and a rolling-window,
  fingerprinted strategy for warehouse movements.

### Tests

- Golden HMAC/encoding fixtures, Europe/Tallinn DST/clock-skew tests, rate
  limits, 500/503 backoff, 401/403/billing halt, pagination, first-page deletes,
  same-second updates, out-of-order pages and interrupted/resumed pages.
- Contract fixtures for every accepted API endpoint and explicit schema-drift
  failure before any target write.

### Exit criteria

Two equivalent captures are byte/hash equivalent where the source is unchanged,
and a simulated concurrent source update/delete is represented exactly once in
the replay package.

## Work package 3: canonicalisation, coverage and export bridges

### Build

- Define `migration-package/v1`, canonical NDJSON, manifest, hash and signature
  schemas.
- Implement normalisers and a generated field-level coverage ledger.
- Implement mapping rules, versioned overrides, unresolved-field report and
  deterministic package validator.
- Implement an export-bridge interface. Each bridge declares source/UI version,
  collector, file schema, date/basis, row/file counts, hashes, evidence
  classification, import adapter or evidence-only disposition, and
  reconciliation metric.
- Implement a separately flagged `browser_bridge` interface only for approved
  `read_only_retrieval` contracts. It runs through the supervised Brave-debugger
  surface, defaults to disabled, retains no session credential, and emits the
  same immutable evidence manifest as an export bridge.
- Start only with bridges whose contract is accepted. Candidate domains are bank
  statements/matching, tax declarations, payroll runs, fixed assets, periodic
  documents, inventory valuation and reports.

### Tests

- Deterministic manifest/package digests; malformed/oversized/malicious file
  rejection; encryption round-trip; field disposition completeness; mapping
  compatibility; bridge schema/date/basis/hash validation.

### Exit criteria

No package validates with a silent field drop, unknown mapping, missing artifact
hash, unowned UI-only domain, or unapproved browser-backed request.

## Work package 4: implement Open Accounting Import Session v1

### Build

- Add import capability handshake, artifact upload, session lifecycle,
  validation, versioned mapping, plan/review/approval/commit/resume/event/
  reconciliation APIs and operator UI.
- Add tenant tables for import runs/items/artifacts/events, external links,
  tombstones, mappings, conflicts, approvals, reconciliation checks and
  attachment links.
- Add scoped OA token permissions and second-actor production approval.
- Implement plan digest invalidation when package, mapping, policy, source
  cursor or relevant target state changes.

### Tests

- Tenant isolation, scopes, package hash/signature/schema, chunk assembly,
  concurrent session, plan invalidation, event durability and resume boundary
  tests using a real temporary PostgreSQL tenant schema.

### Exit criteria

The target accepts an encrypted/verified package through a resumable session,
shows durable progress, and cannot apply without independent approval of its
exact plan digest.

## Work package 5: implement safe domain adapters

### Build

- Implement dependency-ordered adapters around existing OA account, contact,
  product, warehouse, invoice, payment, journal, bank, asset, payroll, tax,
  quote/order and document services.
- Use provider/company/type/external-ID links for idempotency; do not rely on
  business-name matching or legacy import duplicate handling.
- Enforce selected accounting authority mechanically to prevent source GL and
  target-generated GL double posting.
- Add immutable revision/tombstone workflows and source-file supersession.
- Add document entity support or generic source-evidence entities for all
  accepted attachment parent types, with MIME/path validation and malware
  quarantine before accessibility.

### Tests

- Identical package no-op, source revision, source deletion, posted/locked
  correction, payment allocation, duplicate-GL prevention, attachment revision,
  tenant isolation and every accounting invariant.

### Exit criteria

All accepted core resources can be applied, resumed and re-run without duplicate
financial impact; changed posted data always produces a review item rather than
an overwrite.

## Work package 6: prove reconciliation and run a pilot

### Build

- Implement same-cutoff reconciliation for trial balance, account balances,
  AR/AP, bank/cash, currency/FX, VAT/TSD, payroll, inventory/valuation, fixed
  assets, report totals and attachment counts/hashes.
- Implement SLO dashboard and alerts: cursor lag, source quota headroom,
  duration, retries, failures, unmapped fields, schema drift and reconciliation
  variance.
- Document pause, recovery, incident, key rotation, retention/disposal and
  backup/restore procedures.

### Tests and exit criteria

- End-to-end disposable-tenant run: discovery → capture → package → staging →
  approval → apply → reconciliation → delta → correction/delete → resume.
- Fault-inject every page, artifact, transaction and source outage boundary.
- Rehearse Open Accounting backup restore and prove package evidence/hash
  verification after recovery.
- Accountant signs every applicable reconciliation domain before a production
  run can be scheduled.

## Work package 7: controlled production cutover

1. Verify the M0–M6 evidence and named operator/accountant availability.
2. Back up and verify Open Accounting; record the source freeze/cutoff.
3. Generate a new production package, validate, plan, review and obtain a
   second-actor approval.
4. Apply once, observe durable events, then run final reconciliation at the
   approved cutoff.
5. Close the session only with signed evidence/waivers. Enable scheduled deltas
   only after the initial session is reconciled.

## Definition of done

The first production cutover is complete only when all in-scope fields and
resources have a reviewed disposition; hashes and source windows verify;
approval, idempotency, correction, tenant isolation, attachment security and
backup/restore tests pass; all applicable reconciliation metrics meet approved
tolerance; and the owner/accountant formally sign the closed Import Session.
