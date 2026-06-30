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
- Prefer the one-command offline sync path first: SmartAccounts CSV/XML exports -> `oa migration smartaccounts-sync` -> private operator report and saved dry run -> accountant review -> rerun with `--confirm`. Use the lower-level snapshot preparer -> `migration validate` -> `migration plan` -> non-confirming `migration execute` flow only when debugging a specific stage.
- Keep SmartAccounts-specific parsing in cutover mapper/tooling. Do not duplicate domain import logic already owned by accounting, contacts, invoicing, payments, banking, inventory, assets, payroll, and recurring services.
- Do not mutate imported posted history by replaying edited source rows. Corrections should use reversal, void, reopen, or adjustment flows.

## Snapshot Workflow

Prepare, validate, plan, and save a dry run from a directory of SmartAccounts exports:

```bash
go run ./cmd/oa migration smartaccounts-sync \
  --source-dir /path/to/private/smartaccounts/export \
  --out-dir /path/to/private/smartaccounts/prepared \
  --company-id 12345678 \
  --company-name "Example Export OU" \
  --cutover-date YYYY-MM-DD \
  --bank-transaction-account-id <oa-bank-account-id> \
  --opening-balance-entry-date YYYY-MM-DD
```

The command writes a private `smartaccounts-sync-report.json` under `--out-dir`.
Review its `readiness` checks for run mechanics and its `parity_checklist`
for report-by-report reconciliation status. Do not treat a migration as
complete while any checklist item is `pending`, `blocked`, or `failed`.
Add `--confirm` only after accountant review. Add `--post-journal-entries`
only when reviewed historical journals should be posted immediately and included
in GL-based reports. If `--opening-balance-entry-date` is omitted, it defaults
to `--cutover-date`.

After a private sync report exists, generate the Open Accounting proof bundle:

```bash
go run ./cmd/oa migration smartaccounts-proof-plan \
  --report /path/to/private/smartaccounts/prepared/smartaccounts-sync-report.json \
  --out-dir /path/to/private/smartaccounts/proof \
  --as-of YYYY-MM-DD \
  --start YYYY-MM-DD \
  --end YYYY-MM-DD \
  --bank-account-id <oa-bank-account-id> \
  --inventory-method weighted-average
```

The proof-plan command writes a private `smartaccounts-proof-plan.json` and
`open-accounting-proof-commands.sh`. It turns the sync report checklist into
Open Accounting report commands for trial balance, AR/AP, income statement,
bank/cash flow, KMD, payroll/TSD, inventory, and fixed assets. It does not mark
parity as passed; compare the generated artifacts with private SmartAccounts
proof reports first. If the plan reports missing context, supply it and
regenerate the plan before running the script.

For manual stage debugging, prepare a bundle from a directory of SmartAccounts exports:

```bash
go run ./cmd/smartaccounts-snapshot \
  --source-dir /path/to/private/smartaccounts/export \
  --out-dir /path/to/private/smartaccounts/prepared \
  --company-id 12345678 \
  --company-name "Example Export OU" \
  --cutover-date YYYY-MM-DD \
  --json
```

The tool writes:

- `manifest.json` with source hashes, source row ranges, snapshot hash, unsupported files, transformations, and a validation command.
- `bundle/*.csv` or `bundle/*.xml` canonical files ready for the existing migration flow.

Run the emitted validation command, then plan:

```bash
go run ./cmd/oa migration validate \
  --provider-preset smartaccounts \
  --manifest /path/to/private/smartaccounts/prepared/manifest.json \
  --json
go run ./cmd/oa migration plan \
  --provider-preset smartaccounts \
  --manifest /path/to/private/smartaccounts/prepared/manifest.json \
  --opening-balance-entry-date 2026-01-01 \
  --bank-transaction-account-id <oa-bank-account-id> \
  --json
```

Execute only after accountant review:

```bash
go run ./cmd/oa migration execute --provider-preset smartaccounts --manifest /path/to/private/smartaccounts/prepared/manifest.json --json
go run ./cmd/oa migration execute --provider-preset smartaccounts --manifest /path/to/private/smartaccounts/prepared/manifest.json --confirm --json
```

SmartAccounts UI grid CSV exports can contain preamble rows, Estonian headers,
localized enum/date/decimal values, duplicate client/vendor contacts, and
summary-only invoice or journal rows. The snapshot tooling handles deterministic
grid normalization, but do not import invoice or journal grid summaries as
posted history when line-level fields are missing.

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
- Invoice and journal UI grid exports are summary-only unless proven otherwise; full history needs API/detail exports with line-level invoice fields and account/debit-credit journal splits.
- Payroll, TSD/KMD, inventory, fixed assets, recurring invoices, and cost centers must be validated with sample source exports before executing real data.
- Reconciliation reports are not imported as entities; use them as post-import proof.

## Validation

For tooling changes, run:

```bash
go test -count=1 ./internal/cutover
go test -count=1 ./cmd/smartaccounts-snapshot
go test -count=1 ./cmd/oa -run 'TestCLIMigrationSmartAccounts(Sync|ProofPlan)Command|TestSmartAccountsSyncHelpers'
go run ./cmd/smartaccounts-snapshot --source-dir <sample-dir> --out-dir <tmp-out> --json
go test -timeout=3m ./docs -count=1
```

For real tenant cutovers, also verify:

- SmartAccounts snapshot hash is stable for the same source files.
- `migration validate` has no errors.
- `migration plan` has no blocked steps and all `NEEDS_CONTEXT` fields are supplied.
- Non-confirming `migration execute` is saved and reviewed before `--confirm`.
- The private `smartaccounts-sync-report.json` has no blocked or pending
  `readiness` checks except the final private reconciliation gate before
  confirmation.
- `migration smartaccounts-proof-plan` has generated a private proof manifest
  and executable Open Accounting report script for the selected as-of and period.
- Every `parity_checklist` area is reconciled against private SmartAccounts
  proof reports and marked pass/fail in private evidence before cutover closure.
