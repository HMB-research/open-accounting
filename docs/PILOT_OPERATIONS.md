# Production Pilot Operations

This runbook is the required operator handoff for a controlled production pilot.
It does not authorize a live cutover by itself: a named operator must execute the
checks using deployment credentials held outside the repository.

## Backup and Restore Response

The pilot recovery objective is a backup no older than 26 hours and restoration
by the next business day. Keep local backup storage encrypted at rest and use an
encrypted offsite destination: an S3 bucket with default SSE-KMS encryption or
an rclone remote configured with `crypt`. Never put database URLs, S3 keys, or
rclone credentials in this repository.

1. Install `/etc/open-accounting/backup.env` from the secret manager with
   `DATABASE_URL`, a separate `RESTORE_DATABASE_URL`, the offsite destination,
   and the latest offsite-restored dump path. The restore target must be a
   disposable database, never the production database.
2. Generate and preflight the systemd chain with `oa ops backup schedule-systemd`
   and `open-accounting-backup-preflight.sh`; enable timers only after preflight
   succeeds.
3. Verify a daily local backup, its checksum, and its encrypted offsite copy.
   The offsite sync downloads/checks the copy against the local SHA-256 sidecar
   before emitting a success metric; do not bypass a checksum failure.
   Run `oa ops backup health --backup-dir /backups --max-age-hours 26`.
4. Run the weekly drill against the newest offsite copy with
   `oa ops backup restore-drill --backup <file> --restore-url <drill-url> --source-url <production-url>`.
   Preserve the resulting Prometheus textfile metric and drill log in the
   private operations record.
5. When an alert fires, stop any cutover, inspect the script failure code, fix
   the failed backup/sync/drill, and complete a successful drill before clearing
   the incident. Escalate a backup older than 26 hours immediately.

Load `deploy/monitoring/open-accounting-pilot-alerts.yml` into Prometheus and
route `severity=critical, service=open-accounting` to the pilot operator in
Alertmanager. The node exporter must expose both the textfile collector and
`node_systemd_unit_state`.

## Webhook Egress Defense in Depth

Application-level webhook validation blocks private/reserved addresses and DNS
rebinding. On Docker hosts, apply the matching egress policy only after
allowlisting every required private API dependency—at minimum the PostgreSQL
container address:

```bash
sudo deploy/docker/apply-webhook-egress-policy.sh \
  --api-container <compose-project>-api-1 \
  --allow-private <postgres-container-ip> \
  --dry-run
```

Review the dry run, then rerun without `--dry-run`. The script modifies the
host `DOCKER-USER` chain; remove it during rollback with `--remove`. It blocks
Docker-forwarded traffic only, so it supplements rather than replaces the
application's webhook checks.

## Pilot Readiness Record

Before enabling evidence blocking or importing pilot data, record privately:

- successful backup, offsite sync, and restore-drill timestamps;
- Alertmanager receiver and test alert confirmation;
- firewall dry-run/review and the approved private dependency allowlist;
- the SmartAccounts proof-result hash, reviewer, and all reconciled report areas.

Use [the private pilot readiness record template](./PILOT_READINESS_RECORD_TEMPLATE.md)
to record outcomes consistently. A pilot is not ready while any required row is
`FAIL`, `BLOCKED`, or `NOT_RUN`.
