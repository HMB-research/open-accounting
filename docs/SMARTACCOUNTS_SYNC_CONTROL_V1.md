# SmartAccounts sync control and archive intake

> Historical v1 boundary note: this document describes the original
> staging-only control. The current operator flow and live activation gates are
> maintained in [SMARTACCOUNTS_OPERATOR_GUIDE.md](./SMARTACCOUNTS_OPERATOR_GUIDE.md).

This is a staged, no-financial-write integration boundary for the private NUC SmartAccounts bridge. It prepares one explicitly selected SmartAccounts source-to-Open-Accounting tenant binding, captures safely through the bridge, and receives a resumable source archive without showing raw records in a browser.

It is not live sync or financial apply. No endpoint here posts journals, creates invoices, vendor invoices, or payments, changes the chart of accounts, or resolves SmartAccounts credentials in Open Accounting.

## Operator flow

1. Sign in, create/select the target Open Accounting tenant, and open **Migration Workbench**.
2. For the opt-in documented API fallback, enter only the pre-provisioned opaque external credential reference (`secret-ref://file/<connection-id>`) and select **Connect external reference**. Never paste an API key or secret into Open Accounting.
3. The private bridge resolves that tenant-bound reference from its external provider, derives an opaque source identity, and validates an accounts snapshot. OA stores only the returned `secret-ref://sa-bridge/<connection-id>` and safe source identity.
4. The control remains tenant/source scoped. There is no all-tenant or all-source action. Hold My Beer OÜ is presentation-only after bridge validation; it is never a hard-coded source ID.
5. The bridge must publish a verified capture policy before OA enables one-click full history. OA must use declared `full_history` bounds or an explicit administrator-provided lower bound for required/windowed resources; it must never silently guess a date. Brave-visible 2017-01-01 evidence for Hold My Beer OÜ is not a universal coded date.
6. Capture is not archive delivery or accounting apply. A separate private bridge-to-OA handoff stages full source evidence and remains review-required.

## Credentials and accounting policy

- OA does **not** require `SMARTACCOUNTS_SOURCE_COMPANY_ID` or a SmartAccounts internal company ID. The bridge derives the source identity from the credential resolved through its external provider after validation.
- `SMARTACCOUNTS_BRIDGE_URL` and `SMARTACCOUNTS_BRIDGE_TOKEN_FILE` configure control access. The file is preferred over `SMARTACCOUNTS_BRIDGE_TOKEN`, which is development/test fallback only. Mount it read-only as a Docker secret; do not publish the bridge port.
- Open Accounting does not accept API keys or secrets. It passes the supplied opaque external credential reference once to the private bridge, never persists or returns it, and discards bridge error bodies. The bridge resolves raw credential material only through its external provider.
- SmartAccounts GL is authoritative. A later reviewed executor may post each verified balanced source journal exactly once by external-ID/revision. Sales invoices, purchase/vendor invoices, and payments are non-posting linked records/evidence and cannot create duplicate GL postings.
- The UI exposes no source records, artifact paths, bridge cursors, raw capture queries, or secret references.

## Capture control

The authenticated tenant routes require the existing manage-settings/create-entries permissions:

```text
POST /api/v1/tenants/{tenantID}/smartaccounts-sync/control
POST /api/v1/tenants/{tenantID}/smartaccounts-sync/dry-run?source_company_id={bridge-derived-source-id}
GET  /api/v1/tenants/{tenantID}/smartaccounts-sync/status?source_company_id={bridge-derived-source-id}
POST /api/v1/tenants/{tenantID}/smartaccounts-sync/apply
```

The bridge capture call has an explicit range today. If no safe bound is available, it may capture optional-filter resources and return a concise required/windowed-resource review prompt; OA must not fabricate dates. Once the rolling governor exists, OA may show only safe `rate_limited` and next-eligible metadata. `POST .../apply` requires `{"confirm":true}` but is intentionally blocked and has no financial-write capability.

## Private resumable archive delivery v1

Migration `069_external_import_archive_delivery` introduces a separate server-to-server receiver. It is **not** the 2 MiB/10,000-record browser import-session route.

```text
PUT  /api/v1/internal/bridge/tenants/{tenantID}/packages/{packageID}/manifest
PUT  /api/v1/internal/bridge/tenants/{tenantID}/packages/{packageID}/records/{sequence}
PUT  /api/v1/internal/bridge/tenants/{tenantID}/packages/{packageID}/artifacts/{artifactID}/chunks/{sequence}
GET  /api/v1/internal/bridge/tenants/{tenantID}/packages/{packageID}
POST /api/v1/internal/bridge/tenants/{tenantID}/packages/{packageID}/finalize
```

This route is private-network only and uses a dedicated HMAC secret—not a browser bearer token. Each request signs `v1\nMETHOD\nPATH\nTENANT\nTIMESTAMP\nNONCE\nCONTENT_SHA256`. Required headers are `X-OA-Bridge-Tenant`, `X-OA-Bridge-Timestamp` (UTC, five-minute lifetime), `X-OA-Bridge-Nonce`, `X-OA-Bridge-Content-SHA256`, and `X-OA-Bridge-Signature`. OA verifies the digest before parsing, stores the nonce to reject replay, and requires header and path tenant to match.

The manifest binds provider, bridge-derived source ID, source snapshot hash, GL authority, explicit scope/cutoff, resource counts/digests, artifact inventory, record count, manifest/package digests, and `records_sha256` for concatenated decoded ordered NDJSON. OA first requires an existing configured bridge control for that same tenant/source, then binds the provider/source to exactly one tenant before accepting data.

Records are raw `application/x-ndjson` and artifacts use their declared media type. Each raw chunk is capped at 1 MiB and includes `X-OA-Bridge-Chunk-SHA256`; record chunks also include `X-OA-Bridge-Record-Count`, artifact chunks `X-OA-Bridge-Chunk-Count`. HMAC content hash must equal chunk hash. Sequences are strict: exact replay is a no-op; changed content conflicts. OA verifies ordered record count/digest plus each artifact chunk count/bytes/digest before finalization.

Finalization creates `STAGED_REVIEW_REQUIRED` archive metadata only. Tenant-schema archive chunks have no browser-facing route. A later normalizer may derive a GL receipt/plan from verified journals; a reviewed executor remains responsible for account mapping, idempotency, reconciliation, and explicit confirmation.

## Deployment and migrations

Apply migrations `064` through `088` in order. `064` remains unchanged because it is already deployed; later migrations are forward-only upgrades. `068` adds the capture-run pointer, `069` adds public replay-nonce metadata plus tenant-local archive manifest/record/artifact chunk tables, and `079` adds tenant-local confirmed-only reference-master preview and external-identity resume state. Migration `080` adds reviewed browser CSV-schema control records; `081` adds catalog capability hashes, accepted relay picker metadata/digests, immutable owner-selected/all manifests, and safe per-source target/pairing outcomes; `082`–`083` add the serial batch workflow and capture checkpoints; `084` records the v2 General Ledger-only contract; `085` adds master-detail authorization state; `086` persists the optional header-probe choice; `087` adds durable reconciliation/tolerance/actor-separation receipts; and `088` adds only token-hash and count/digest control state for the selector-blocked commercial relay. None of these migrations enables an implicit financial writer.

Mount two separate owner-readable, read-only Docker secrets in OA and bridge:

```text
SMARTACCOUNTS_BRIDGE_URL=http://sa-bridge:8084
SMARTACCOUNTS_BRIDGE_TOKEN_FILE=/run/secrets/smartaccounts_bridge_control_hmac
SMARTACCOUNTS_PACKAGE_DELIVERY_TOKEN_FILE=/run/secrets/smartaccounts_package_delivery_hmac
```

`SMARTACCOUNTS_PACKAGE_DELIVERY_TOKEN` remains a development/test fallback. Neither secret belongs in `docker inspect` environment values. Keep the bridge and internal receiver behind the NUC Docker/private network and firewall even though the HMAC verifier rejects browser requests.
