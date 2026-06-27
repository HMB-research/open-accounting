---
name: open-accounting-smartaccounts-migration
description: Use when migrating data from SmartAccounts.eu into open-accounting, preparing SmartAccounts CSV/XML export snapshots, validating SmartAccounts provider preset bundles, mapping missing feature coverage, or building migration runbooks/tools for SmartAccounts cutovers.
---

# Open Accounting SmartAccounts Migration

Use this with `open-accounting-development`, `open-accounting-import-mappers`, and `open-accounting-accounting-integrity`.

## Boundaries

- Treat SmartAccounts web-login credentials as UI credentials only. Do not assume they can export accounting data through the API.
- For live API extraction, require SmartAccounts API key/secret from Settings / Connected services. Keep API credentials in environment variables or macOS Keychain, never in repo files.
- Treat `HMB-research/open-accounting` as public. Branches in a public GitHub repository are public; real SmartAccounts exports must live outside this worktree or in a separate private repository/worktree dedicated to migration data.
- Use repo-relative private paths only as an ignored local scratch fallback. Prefer `/Users/clawdy/private/open-accounting-smartaccounts/...` or a private repo path for real data.
- Prefer the offline snapshot path first: SmartAccounts CSV/XML exports -> snapshot preparer -> canonical bundle -> `migration validate` -> `migration plan` -> non-confirming `migration execute` -> confirmed execution.
- Keep SmartAccounts-specific parsing in cutover mapper/tooling. Do not duplicate domain import logic already owned by accounting, contacts, invoicing, payments, banking, inventory, assets, payroll, and recurring services.
- Do not mutate imported posted history by replaying edited source rows. Corrections should use reversal, void, reopen, or adjustment flows.

## Snapshot Workflow

Prepare a bundle from a directory of SmartAccounts exports:

```bash
go run ./cmd/smartaccounts-snapshot \
  --source-dir /Users/clawdy/private/open-accounting-smartaccounts/export \
  --out-dir /Users/clawdy/private/open-accounting-smartaccounts/prepared \
  --company-id 14369460 \
  --company-name "Hold My Beer OU" \
  --cutover-date 2026-01-01 \
  --json
```

The tool writes:

- `manifest.json` with source hashes, snapshot hash, unsupported files, transformations, and a validation command.
- `bundle/*.csv` or `bundle/*.xml` canonical files ready for the existing migration flow.

Run the emitted validation command, then plan:

```bash
go run ./cmd/oa migration validate --provider-preset smartaccounts ... --json
go run ./cmd/oa migration plan \
  --provider-preset smartaccounts \
  --opening-balance-entry-date 2026-01-01 \
  --bank-transaction-account-id <oa-bank-account-id> \
  ... \
  --json
```

Execute only after accountant review:

```bash
go run ./cmd/oa migration execute --provider-preset smartaccounts ... --json
go run ./cmd/oa migration execute --provider-preset smartaccounts ... --confirm --json
```

## API Extraction Notes

If the user asks for live SmartAccounts API export:

- Confirm or retrieve these names without printing values: `SMART_ACCOUNTS_API_KEY`, `SMART_ACCOUNTS_API_SECRET`, `SMART_ACCOUNTS_BASE_URL`, `SMARTACCOUNTS_API_LANGUAGE`.
- Keep `SMARTACCOUNTS_ALLOW_MUTATIONS` unset or false for migration export work.
- Respect SmartAccounts API signing, timestamp, pagination, and rate limits before adding code.
- Store raw API snapshots separately from canonical bundles so each source payload can be hashed and re-run.

## Gap Map

Track these gaps explicitly in docs/runbooks before claiming migration completeness:

- Direct SmartAccounts API snapshot adapter is separate from the current offline CSV/XML preparer.
- Attachments, invoice PDFs, and report comparison exports need separate handling.
- Bank transactions may require either bank-native statements or a SmartAccounts-specific transaction export shape.
- Opening balances need a cutover date; bank transactions need an Open Accounting bank account id.
- Payroll, TSD/KMD, inventory, fixed assets, recurring invoices, and cost centers must be validated with sample source exports before executing real data.
- Reconciliation reports are not imported as entities; use them as post-import proof.

## Validation

For tooling changes, run:

```bash
go test -count=1 ./internal/cutover
go test -count=1 ./cmd/smartaccounts-snapshot
go run ./cmd/smartaccounts-snapshot --source-dir <sample-dir> --out-dir <tmp-out> --json
go test -timeout=3m ./docs -count=1
```

For real tenant cutovers, also verify:

- SmartAccounts snapshot hash is stable for the same source files.
- `migration validate` has no errors.
- `migration plan` has no blocked steps and all `NEEDS_CONTEXT` fields are supplied.
- Non-confirming `migration execute` is saved and reviewed before `--confirm`.
