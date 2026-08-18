# Private Pilot Readiness Record Template

Copy this template to the private operations record for a named pilot. Do not
commit the completed record, SmartAccounts exports, report totals, tenant IDs,
full URLs, credentials, object-storage paths, or artifact hashes to this public
repository.

Use only these outcomes:

- `PASS` — the check ran successfully and has private evidence.
- `FAIL` — the check ran and failed; cutover is prohibited.
- `BLOCKED` — required context, access, or a supported import step is missing.
- `NOT_RUN` — the check has not yet been attempted; cutover is prohibited.

The pilot is ready only when every required row is `PASS`, an accountant has
approved the SmartAccounts proof, and the named operator has approved this
record. A `PASS` must cite a private evidence reference and completion time;
do not put sensitive values into the public runbook.

```text
Pilot reference: <private internal reference>
Prepared at (UTC): <timestamp>
Prepared by: <operator>
Accountant reviewer: <reviewer>
Decision: NOT_READY | READY_FOR_CUTOVER

RECOVERY AND OBSERVABILITY
Outcome: <PASS|FAIL|BLOCKED|NOT_RUN>
Local encrypted backup: <outcome> — evidence: <private reference>, completed: <UTC>
Offsite copy + checksum verification: <outcome> — evidence: <private reference>, completed: <UTC>
Backup freshness (<=26 hours): <outcome> — evidence: <private reference>, completed: <UTC>
Isolated restore drill (next-business-day objective): <outcome> — evidence: <private reference>, completed: <UTC>
Prometheus textfile metrics visible: <outcome> — evidence: <private reference>, completed: <UTC>
Alertmanager test alert received: <outcome> — evidence: <private reference>, completed: <UTC>
Backup/offsite/health/restore timers healthy: <outcome> — evidence: <private reference>, completed: <UTC>

WEBHOOK EGRESS
Outcome: <PASS|FAIL|BLOCKED|NOT_RUN>
Approved private dependency allowlist reviewed: <outcome> — evidence: <private reference>
Host egress dry run reviewed: <outcome> — evidence: <private reference>
Host egress policy applied: <outcome> — evidence: <private reference>
Public webhook delivery regression checked: <outcome> — evidence: <private reference>

SMARTACCOUNTS STAGING CUTOVER
Outcome: <PASS|FAIL|BLOCKED|NOT_RUN>
Scrubbed representative export: <outcome> — evidence: <private reference>
Snapshot/validation/plan/saved dry run: <outcome> — evidence: <private reference>
All supported plan steps ready or explicitly unsupported: <outcome> — evidence: <private reference>
Trial balance and balance sheet reconciliation: <outcome> — evidence: <private reference>
Profit and loss, receivables, payables, and bank reconciliation: <outcome> — evidence: <private reference>
VAT/KMD, payroll/TSD, inventory, fixed assets, and migration counts: <outcome> — evidence: <private reference>
`smartaccounts-proof-result --require-ready`: <outcome> — evidence: <private reference>
Accountant approval of the proof: <outcome> — evidence: <private reference>

EVIDENCE POLICY
Outcome: <PASS|FAIL|BLOCKED|NOT_RUN>
Pilot tenant `block_high_risk` setting verified: <outcome> — evidence: <private reference>
Blocked high-risk action leaves accounting state unchanged: <outcome> — evidence: <private reference>
Upload, review, approve, and retry workflow verified: <outcome> — evidence: <private reference>
Tenant audit history records enforcement/override: <outcome> — evidence: <private reference>

EXCEPTIONS AND DECISION
Open discrepancy or unsupported source-format gap: <none|private reference>
Required remediation and owner: <private reference>
Operator approval: <name and UTC timestamp>
Accountant approval: <name and UTC timestamp>
```

Before signing `READY_FOR_CUTOVER`, rerun the recovery health check and ensure
the evidence policy is enabled on the actual pilot tenant. Direct bank feeds,
automatic e-MTA filing, and operator-network e-invoicing are explicitly out of
scope for this pilot record.
