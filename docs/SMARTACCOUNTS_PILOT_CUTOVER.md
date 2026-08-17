# SmartAccounts Pilot Cutover

Keep source exports, prepared bundles, proof artifacts, report totals, and
reviewer evidence in private storage outside this public worktree. A scrubbed
representative export must preserve account codes, identifiers, relationships,
dates, currencies, and monetary totals while replacing names, emails, bank
identifiers, addresses, and file attachments.

1. Run `oa migration smartaccounts-sync` against the private source directory
   to create the snapshot, canonical bundle, saved dry run, and
   `smartaccounts-sync-report.json`.
2. Resolve every `BLOCKED` and `NEEDS_CONTEXT` item. Do not confirm an import
   from summary-only invoice or journal exports that lack line-level history.
3. Generate the private proof plan with `oa migration smartaccounts-proof-plan`.
   It must cover trial balance, balance sheet, income statement, receivables,
   payables, bank/cash flow, VAT/KMD, payroll/TSD, inventory, fixed assets, and
   migration counts applicable to the export.
4. Run the non-confirming execution against a disposable pilot tenant. Compare
   each generated Open Accounting artifact with its private SmartAccounts proof
   report, record its hash and reviewer, and write the private proof result.
5. Run `oa migration smartaccounts-proof-result --require-ready --json`. Only
   after it reports every planned area as passed may an accountant approve the
   saved run for `--confirm` in the live pilot tenant.

For each incompatibility, add an anonymized fixture and a mapper/validator test
to this repository. Never commit original exports, raw reports, proof hashes,
tenant identifiers, or amounts.
