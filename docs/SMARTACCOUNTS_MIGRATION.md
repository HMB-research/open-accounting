# SmartAccounts Migration Runbook

This runbook covers the safe Open Accounting path for SmartAccounts CSV/XML cutovers. It is intentionally public and uses placeholders only. Store real exports, prepared bundles, reports, and operator notes outside this public repository or in a separate private repository.

## Privacy Boundary

- Do not save SmartAccounts exports under this repository.
- Do not push tenant data to a public GitHub repository or a branch in a public repository.
- Keep SmartAccounts web-login, API key, and API secret values in macOS Keychain or environment variables.
- Treat SmartAccounts web-login credentials as UI credentials only. API extraction requires SmartAccounts API key/secret credentials from connected services.
- Run `smartaccounts-snapshot` with private `--source-dir` and `--out-dir` paths. The tool refuses to write snapshots inside the public Open Accounting worktree.

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

## Snapshot

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

The manifest records supported prepared files, unsupported files, source/output SHA-256 hashes, a stable snapshot hash, and the exact validation command.

## Validate And Plan

Run the validation command emitted in `manifest.json`, or pass the prepared bundle files directly:

```bash
go run ./cmd/oa migration validate \
  --provider-preset smartaccounts \
  --accounts /path/to/private/smartaccounts/prepared/bundle/accounts.csv \
  --contacts /path/to/private/smartaccounts/prepared/bundle/contacts.csv \
  --json | tee /path/to/private/smartaccounts/reports/validation-report.json
```

Build the execution plan:

```bash
go run ./cmd/oa migration plan \
  --provider-preset smartaccounts \
  --accounts /path/to/private/smartaccounts/prepared/bundle/accounts.csv \
  --contacts /path/to/private/smartaccounts/prepared/bundle/contacts.csv \
  --opening-balance-entry-date YYYY-MM-DD \
  --bank-transaction-account-id <open-accounting-bank-account-id> \
  --json | tee /path/to/private/smartaccounts/reports/plan-report.json
```

Only include `--opening-balance-entry-date` when opening balances are present. Only include `--bank-transaction-account-id` when bank transactions are present.

## Dry Run And Confirmed Execution

Run non-confirming execution first and save the report:

```bash
go run ./cmd/oa migration execute \
  --provider-preset smartaccounts \
  <same file flags used for plan> \
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
  <same file flags used for plan> \
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

## Retry And Rollback

- Re-run snapshot preparation only from unchanged source exports when retrying a failed preparation step.
- If source exports change, treat the new snapshot hash as a new migration attempt.
- Use migration execution resume support for interrupted runs instead of replaying already successful steps.
- Preserve posted accounting history. Corrections after confirmed import should use reversal, void, reopen, or adjustment workflows rather than destructive mutation.

## Coverage Gaps

The offline snapshot path is ready for supported CSV/XML files. Remaining gaps are tracked in [Feature Mapping: Merit And SmartAccounts](./FEATURE_MAPPING_MERIT_SMARTACCOUNTS.md):

- signed live SmartAccounts API export;
- attachments and invoice PDFs;
- bank-statement variants outside supported import formats;
- sample-proven payroll, TSD/KMD, inventory, fixed-asset, recurring-invoice, cost-center, and cost-allocation exports;
- reconciliation report handling.
