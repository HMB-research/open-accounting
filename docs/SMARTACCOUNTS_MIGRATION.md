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
- includes private reconciliation targets for trial balance, AR/AP, revenue/expense, bank, VAT/tax, payroll/TSD, and inventory/fixed assets;
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

If `--opening-balance-entry-date` is omitted, it defaults to `--cutover-date`. Only include `--bank-transaction-account-id` when bank transactions are present. Use `--json` when an automation needs the public-safe aggregate summary; the full private report remains in `--out-dir`.

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
- accountant/operator signoff is recorded in private notes.

Confirmed execution:

```bash
go run ./cmd/oa migration execute \
  --provider-preset smartaccounts \
  --manifest /path/to/private/smartaccounts/prepared/manifest.json \
  <same context flags used for plan> \
  --confirm \
  --json | tee /path/to/private/smartaccounts/reports/execute-confirmed.json
```

## Reconciliation

Use SmartAccounts reports as private proof artifacts rather than importable entities:

- trial balance at cutover date;
- aged receivables and aged payables;
- VAT/KMD and TSD period summaries;
- bank balances and unreconciled items;
- inventory value and fixed-asset register totals;
- invoice/payment exception lists.

Compare those reports against Open Accounting reports after import and keep reconciliation evidence in private storage.

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
