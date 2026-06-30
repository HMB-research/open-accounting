# SmartAccounts Migration Runbook

This runbook covers the safe Open Accounting path for SmartAccounts CSV/XML cutovers. It is intentionally public and uses placeholders only. Store real exports, prepared bundles, reports, and operator notes outside this public repository or in a separate private repository.

## Privacy Boundary

- Do not save SmartAccounts exports under this repository.
- Do not push tenant data to a public GitHub repository or a branch in a public repository.
- Keep SmartAccounts web-login, API key, and API secret values in macOS Keychain or environment variables.
- Treat SmartAccounts web-login credentials as UI credentials only. API extraction requires SmartAccounts API key/secret credentials from connected services.
- Run `oa migration smartaccounts-sync` with private `--source-dir` and `--out-dir` paths. The snapshot layer refuses to write prepared bundles inside the public Open Accounting worktree.
- Run migration sync, validation, planning, and execution against a local or otherwise private Open Accounting API target. Bundle payloads are sent to the configured API, and saved dry runs persist replay payloads server-side for review/resume.

## Local Setup

1. Prepare a private workspace:

```bash
mkdir -p /path/to/private/smartaccounts/{export,prepared,reports}
chmod 700 /path/to/private/smartaccounts /path/to/private/smartaccounts/{export,prepared,reports}
```

2. Put raw SmartAccounts CSV/XML exports under `/path/to/private/smartaccounts/export`.

3. Confirm the public code checkout is up to date and clean:

```bash
cd /path/to/open-accounting
git status --short --branch
```

## One-Command Next Sync

For the next sync, use the one-command operator path first:

```bash
go run ./cmd/oa migration smartaccounts-sync \
  --source-dir /path/to/private/smartaccounts/export \
  --out-dir /path/to/private/smartaccounts/prepared \
  --company-id 12345678 \
  --company-name "Example Export OU" \
  --cutover-date YYYY-MM-DD \
  --bank-transaction-account-id <open-accounting-bank-account-id> \
  --opening-balance-entry-date YYYY-MM-DD
```

By default this command:

- prepares a hashed SmartAccounts snapshot and `manifest.json`;
- validates the prepared bundle with the `smartaccounts` provider preset;
- builds the migration execution plan;
- saves a non-mutating dry run through the configured Open Accounting API;
- writes a full private operator report to `smartaccounts-sync-report.json` under `--out-dir`;
- includes `readiness` checks for snapshot inventory, validation, execution plan context, execution status, journal posting decision, and private report reconciliation;
- includes a `parity_checklist` for trial balance, AR/AP, revenue/expense, bank, VAT/tax, payroll/TSD, and inventory/fixed assets with required SmartAccounts evidence, matching Open Accounting evidence, discrepancy risk, blocker status, and next action;
- prints only aggregate counts, paths, hashes, and next action guidance.

Only add `--confirm` after accountant signoff. Confirmed execution uses the same prepared bundle and context, but mutates the configured Open Accounting tenant:

```bash
go run ./cmd/oa migration smartaccounts-sync \
  --source-dir /path/to/private/smartaccounts/export \
  --out-dir /path/to/private/smartaccounts/prepared \
  --company-id 12345678 \
  --company-name "Example Export OU" \
  --cutover-date YYYY-MM-DD \
  --bank-transaction-account-id <open-accounting-bank-account-id> \
  --opening-balance-entry-date YYYY-MM-DD \
  --confirm
```

If `--opening-balance-entry-date` is omitted, it defaults to `--cutover-date`. Only include `--bank-transaction-account-id` when bank transactions are present. Historical journal imports remain draft by default. Add `--post-journal-entries` only after accountant review confirms the journal export is complete and should immediately affect GL-based reports such as trial balance, balance sheet, income statement, and indirect cash flow. Use `--json` when an automation needs a machine-readable operator summary, but keep that output private because it can include tenant identifiers, private paths, hashes, and run artifact locations. The full private report remains in `--out-dir`.

## Current Progress And Integrity Status

This public runbook does not record tenant amounts or private SmartAccounts file names. As of the current migration tooling stage:

- The offline SmartAccounts snapshot path can prepare, validate, plan, save, resume, and execute supported CSV/XML bundles without storing private data in this public repository.
- Confirmed migration execution is still review-gated. The dry run persists its bundle and execution context server-side so accountant workspace actions or `migration execute --resume-run-id` can confirm the reviewed run later.
- Historical journal imports are intentionally draft-only unless `--post-journal-entries` is supplied. Draft imported journals will not appear in posted-ledger reports, so a Balance Sheet or income statement that returns zeros can mean imported GL history is still draft, not that the report endpoint failed.
- Each `smartaccounts-sync-report.json` now records a machine-readable `readiness` section for the run mechanics and a `parity_checklist` section for report-by-report reconciliation status. Treat blocked, pending, or failed checklist items as unresolved migration work.
- Full SmartAccounts parity remains unproven until private reconciliation compares SmartAccounts trial balance, aged AR/AP, income statement, bank, VAT/KMD, payroll/TSD, inventory, and fixed-asset proof reports against Open Accounting outputs at the same dates and accounting basis.
- Do not mark a tenant migration complete while reconciliation differences remain or while any needed source export is only summary-level and lacks line-level accounting data.

## Private Proof Plan

Generate the Open Accounting proof command bundle from the private sync report:

```bash
go run ./cmd/oa migration smartaccounts-proof-plan \
  --report /path/to/private/smartaccounts/prepared/smartaccounts-sync-report.json \
  --out-dir /path/to/private/smartaccounts/proof \
  --as-of YYYY-MM-DD \
  --start YYYY-MM-DD \
  --end YYYY-MM-DD \
  --bank-account-id <open-accounting-bank-account-id> \
  --inventory-method weighted-average
```

This writes two private files:

- `smartaccounts-proof-plan.json`: manifest of checklist areas, required Open Accounting evidence, generated command lines, output paths, missing context, and next action.
- `open-accounting-proof-commands.sh`: private shell script that runs the Open Accounting report commands and writes JSON/CSV/XML artifacts under `--out-dir`; run it with `sh /path/to/private/smartaccounts/proof/open-accounting-proof-commands.sh`.

The proof-plan command refuses output inside this public Open Accounting worktree. The generated artifacts are Open Accounting evidence only; they still need to be compared with private SmartAccounts proof reports before any checklist item can be marked passed. If the plan reports missing context, supply that context and regenerate the plan before running the script.

After private SmartAccounts proof reports have been compared with the generated Open Accounting artifacts, validate the private proof result:

```bash
go run ./cmd/oa migration smartaccounts-proof-result \
  --plan /path/to/private/smartaccounts/proof/smartaccounts-proof-plan.json \
  --result /path/to/private/smartaccounts/proof/smartaccounts-proof-result.json \
  --require-ready \
  --json
```

The proof result file must stay in the private migration workspace. It records one `passed` item for every area in `smartaccounts-proof-plan.json`, with reviewer evidence, review timestamp, SmartAccounts artifact path and SHA-256, Open Accounting artifact path and SHA-256, accounting basis, and proof period. The validator is read-only: it checks that files are outside the public Open Accounting worktree, recomputes local artifact hashes, reports blockers, and only returns ready when every planned parity area is proven. Do not copy validator JSON, hashes, private paths, tenant identifiers, amounts, or source-row details into this public repository.

## Manual Snapshot

Use the lower-level commands when debugging one stage of the sync or when preparing a custom bundle.

Prepare a canonical bundle from private SmartAccounts exports:

```bash
go run ./cmd/smartaccounts-snapshot \
  --source-dir /path/to/private/smartaccounts/export \
  --out-dir /path/to/private/smartaccounts/prepared \
  --company-id 12345678 \
  --company-name "Example Export OU" \
  --cutover-date YYYY-MM-DD \
  --json | tee /path/to/private/smartaccounts/reports/snapshot-report.json
```

The manifest records supported prepared files, unsupported files, source/output SHA-256 hashes, source row ranges inside merged CSV outputs, a stable snapshot hash, and the exact validation command.

## Validate And Plan

Run the validation command emitted in `manifest.json`, or pass the manifest explicitly:

```bash
go run ./cmd/oa migration validate \
  --provider-preset smartaccounts \
  --manifest /path/to/private/smartaccounts/prepared/manifest.json \
  --json | tee /path/to/private/smartaccounts/reports/validation-report.json
```

Build the execution plan:

```bash
go run ./cmd/oa migration plan \
  --provider-preset smartaccounts \
  --manifest /path/to/private/smartaccounts/prepared/manifest.json \
  --opening-balance-entry-date YYYY-MM-DD \
  --bank-transaction-account-id <open-accounting-bank-account-id> \
  --json | tee /path/to/private/smartaccounts/reports/plan-report.json
```

Only include `--opening-balance-entry-date` when opening balances are present. Only include `--bank-transaction-account-id` when bank transactions are present.
If private accountant review has approved posting historical journals, include
`--post-journal-entries` in the plan to preview `journal import --post`; otherwise
the planned journal import remains draft-only.

Direct file flags such as `--accounts` and `--contacts` remain available for generic bundles or manual overrides, but the manifest is preferred for SmartAccounts snapshots because it carries every prepared file, including repeated e-invoice XML payloads.

## Dry Run And Confirmed Execution

Run non-confirming execution first and save the report:

```bash
go run ./cmd/oa migration execute \
  --provider-preset smartaccounts \
  --manifest /path/to/private/smartaccounts/prepared/manifest.json \
  <same context flags used for plan> \
  --json | tee /path/to/private/smartaccounts/reports/execute-dry-run.json
```

Proceed to confirmed execution only after:

- validation summary has `ready=true`;
- plan summary has `blocked_step_count=0`;
- all `NEEDS_CONTEXT` items have been supplied;
- the snapshot hash and source export inventory are reviewed;
- accountant/operator signoff is recorded in private notes;
- the decision to leave historical journals draft or to post them through `--post-journal-entries` is recorded in private notes.

Confirmed execution:

```bash
go run ./cmd/oa migration execute \
  --provider-preset smartaccounts \
  --manifest /path/to/private/smartaccounts/prepared/manifest.json \
  <same context flags used for plan> \
  --confirm \
  --json | tee /path/to/private/smartaccounts/reports/execute-confirmed.json
```

If private accountant review approves immediate GL posting of historical journals,
add `--post-journal-entries` to the confirmed execution command.

## Reconciliation

Use SmartAccounts reports as private proof artifacts rather than importable entities:

- trial balance at cutover date;
- aged receivables and aged payables;
- VAT/KMD and TSD period summaries;
- bank balances and unreconciled items;
- inventory value and fixed-asset register totals;
- invoice/payment exception lists.

Compare those reports against Open Accounting reports after import and keep reconciliation evidence in private storage.
Use the private report `parity_checklist` as the canonical reconciliation worklist. Status values are `pending`, `blocked`, `ready_for_review`, `passed`, and `failed`. The sync command can mark a report area `ready_for_review` after confirmed execution, but it does not mark any area `passed`; only private comparison of SmartAccounts proof reports to Open Accounting outputs can do that.

Do not use the dashboard as the only reconciliation surface. Dashboard totals mix accounting bases:

- receivables and payables come from invoice subledger balances;
- revenue and expenses come from posted general-ledger journal lines;
- dashboard cash flow is payment-subledger based.

For large discrepancies, compare each basis separately:

- SmartAccounts aged receivables/payables against Open Accounting aging and balance-confirmation reports;
- SmartAccounts trial balance against Open Accounting trial balance, balance sheet, and account-balance reports;
- SmartAccounts revenue/expense period reports against Open Accounting income statement for the same dates;
- SmartAccounts bank balances/reconciliations against both Open Accounting bank-transaction balances and GL bank/cash account balances;
- VAT/KMD and payroll/TSD reports against the matching Open Accounting tax reports and GL tax accounts.

Confirm whether the cutover strategy is `opening balances plus open subledger` or `full historical GL plus subledger`. Mixing both without a clear cutover date can double count, while omitting opening balances can leave the GL baseline absent.

Balance Sheet, trial balance, income statement, and indirect cash-flow reports are posted-GL views. If a confirmed migration imported historical journals without `--post-journal-entries`, post or review those draft journals before treating those report outputs as final parity evidence. Direct cash-flow views also require payment and bank/payment-subledger data; GL parity alone does not prove cash movement parity.

## Retry And Rollback

- Re-run snapshot preparation only from unchanged source exports when retrying a failed preparation step.
- If source exports change, treat the new snapshot hash as a new migration attempt.
- Use migration execution resume support for interrupted runs instead of replaying already successful steps.
- Preserve posted accounting history. Corrections after confirmed import should use reversal, void, reopen, or adjustment workflows rather than destructive mutation.

## Coverage Gaps

The offline snapshot path is ready for supported CSV/XML files. Remaining gaps are tracked in [Feature Mapping: Merit And SmartAccounts](./FEATURE_MAPPING_MERIT_SMARTACCOUNTS.md):

- signed live SmartAccounts API export;
- SmartAccounts UI grid invoices and journal entries when they are summary-only
  and do not include line-level invoice fields or account/debit-credit splits;
- attachments and invoice PDFs;
- bank-statement variants outside supported import formats;
- sample-proven payroll, TSD/KMD, inventory, fixed-asset, recurring-invoice, cost-center, and cost-allocation exports;
- reconciliation report handling.
