#!/usr/bin/env bash
# Materialize systemd units and timers for Open Accounting backup operations.

set -euo pipefail

OUTPUT_DIR="${BACKUP_SYSTEMD_OUTPUT_DIR:-./deploy/systemd}"
SCRIPTS_DIR="${BACKUP_SYSTEMD_SCRIPTS_DIR:-/opt/open-accounting/scripts}"
BACKUP_DIR="${BACKUP_DIR:-/backups}"
STATUS_DIR="${BACKUP_STATUS_DIR:-/var/lib/node_exporter/textfile_collector}"
ENV_FILE="${BACKUP_SYSTEMD_ENV_FILE:-/etc/open-accounting/backup.env}"
SYSTEMD_UNIT_DIR="${BACKUP_SYSTEMD_UNIT_DIR:-/etc/systemd/system}"
OFFSITE_PROVIDER="${BACKUP_OFFSITE_PROVIDER:-s3}"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-30}"
MAX_AGE_HOURS="${BACKUP_MAX_AGE_HOURS:-26}"
BACKUP_CALENDAR="${BACKUP_SYSTEMD_BACKUP_CALENDAR:-*-*-* 02:00:00}"
OFFSITE_CALENDAR="${BACKUP_SYSTEMD_OFFSITE_CALENDAR:-*-*-* 02:20:00}"
HEALTH_CALENDAR="${BACKUP_SYSTEMD_HEALTH_CALENDAR:-*-*-* 02:30:00}"
RESTORE_CALENDAR="${BACKUP_SYSTEMD_RESTORE_CALENDAR:-Sun *-*-* 03:00:00}"
DRY_RUN=false

usage() {
    cat <<'EOF'
Usage: scripts/db-backup-systemd-schedule.sh [options]

Options:
  --output-dir DIR          Directory for generated unit files. Defaults to BACKUP_SYSTEMD_OUTPUT_DIR or ./deploy/systemd.
  --scripts-dir DIR         Directory containing backup scripts. Defaults to BACKUP_SYSTEMD_SCRIPTS_DIR or /opt/open-accounting/scripts.
  --backup-dir DIR          Backup directory passed to backup scripts. Defaults to BACKUP_DIR or /backups.
  --status-dir DIR          Prometheus textfile directory. Defaults to BACKUP_STATUS_DIR or /var/lib/node_exporter/textfile_collector.
  --env-file FILE           systemd EnvironmentFile containing credentials and restore settings.
                             Defaults to BACKUP_SYSTEMD_ENV_FILE or /etc/open-accounting/backup.env.
  --systemd-unit-dir DIR    Host systemd unit directory for the generated install helper.
                             Defaults to BACKUP_SYSTEMD_UNIT_DIR or /etc/systemd/system.
  --offsite-provider NAME   Offsite credential template to write: s3 or rclone. Defaults to BACKUP_OFFSITE_PROVIDER or s3.
  --retention-days DAYS     Backup retention passed to db-backup.sh. Defaults to BACKUP_RETENTION_DAYS or 30.
  --max-age-hours HOURS     Backup freshness threshold passed to db-backup-health.sh. Defaults to BACKUP_MAX_AGE_HOURS or 26.
  --backup-calendar VALUE   systemd OnCalendar value for backups. Defaults to daily 02:00 UTC/local system time.
  --offsite-calendar VALUE  systemd OnCalendar value for offsite sync. Defaults to daily 02:20.
  --health-calendar VALUE   systemd OnCalendar value for backup health. Defaults to daily 02:30.
  --restore-calendar VALUE  systemd OnCalendar value for restore drills. Defaults to weekly Sunday 03:00.
  --dry-run                 Validate inputs and print files that would be written.
  -h, --help                Show this help.

The generated units reference ENV_FILE for DATABASE_URL, RESTORE_DATABASE_URL,
RESTORE_DRILL_BACKUP_FILE, and the selected offsite destination credentials.
The generated install helper copies units to SYSTEMD_UNIT_DIR, preserves an
existing ENV_FILE, reloads systemd, and enables the four timers.
EOF
}

fail() {
    echo "ERROR: $*" >&2
    exit 1
}

log() {
    echo "$*"
}

is_non_negative_integer() {
    [[ "$1" =~ ^[0-9]+$ ]]
}

write_file() {
    local path="$1"

    if [ "$DRY_RUN" = true ]; then
        log "Would write: $path"
        cat >/dev/null
        return
    fi

    mkdir -p "$(dirname "$path")"
    cat > "$path"
    log "Wrote: $path"
}

while [ "$#" -gt 0 ]; do
    case "$1" in
        --output-dir)
            [ "$#" -ge 2 ] || fail "--output-dir requires a value"
            OUTPUT_DIR="$2"
            shift 2
            ;;
        --scripts-dir)
            [ "$#" -ge 2 ] || fail "--scripts-dir requires a value"
            SCRIPTS_DIR="$2"
            shift 2
            ;;
        --backup-dir)
            [ "$#" -ge 2 ] || fail "--backup-dir requires a value"
            BACKUP_DIR="$2"
            shift 2
            ;;
        --status-dir)
            [ "$#" -ge 2 ] || fail "--status-dir requires a value"
            STATUS_DIR="$2"
            shift 2
            ;;
        --env-file)
            [ "$#" -ge 2 ] || fail "--env-file requires a value"
            ENV_FILE="$2"
            shift 2
            ;;
        --systemd-unit-dir)
            [ "$#" -ge 2 ] || fail "--systemd-unit-dir requires a value"
            SYSTEMD_UNIT_DIR="$2"
            shift 2
            ;;
        --offsite-provider)
            [ "$#" -ge 2 ] || fail "--offsite-provider requires a value"
            OFFSITE_PROVIDER="$2"
            shift 2
            ;;
        --retention-days)
            [ "$#" -ge 2 ] || fail "--retention-days requires a value"
            RETENTION_DAYS="$2"
            shift 2
            ;;
        --max-age-hours)
            [ "$#" -ge 2 ] || fail "--max-age-hours requires a value"
            MAX_AGE_HOURS="$2"
            shift 2
            ;;
        --backup-calendar)
            [ "$#" -ge 2 ] || fail "--backup-calendar requires a value"
            BACKUP_CALENDAR="$2"
            shift 2
            ;;
        --offsite-calendar)
            [ "$#" -ge 2 ] || fail "--offsite-calendar requires a value"
            OFFSITE_CALENDAR="$2"
            shift 2
            ;;
        --health-calendar)
            [ "$#" -ge 2 ] || fail "--health-calendar requires a value"
            HEALTH_CALENDAR="$2"
            shift 2
            ;;
        --restore-calendar)
            [ "$#" -ge 2 ] || fail "--restore-calendar requires a value"
            RESTORE_CALENDAR="$2"
            shift 2
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            fail "unknown option: $1"
            ;;
    esac
done

is_non_negative_integer "$RETENTION_DAYS" || fail "--retention-days must be a non-negative integer"
is_non_negative_integer "$MAX_AGE_HOURS" || fail "--max-age-hours must be a non-negative integer"
case "$OFFSITE_PROVIDER" in
    s3|rclone)
        ;;
    *)
        fail "--offsite-provider must be either s3 or rclone"
        ;;
esac

backup_metrics_file="$STATUS_DIR/openaccounting_backup.prom"
restore_metrics_file="$STATUS_DIR/openaccounting_restore_drill.prom"
install_helper="$OUTPUT_DIR/open-accounting-backup-install.sh"

if [ "$OFFSITE_PROVIDER" = "s3" ]; then
    write_file "$OUTPUT_DIR/open-accounting-backup.env.example" <<EOF
# Copy to $ENV_FILE and store the real file in the host secret manager or a
# root-readable path outside the repository. Do not commit live credentials.
DATABASE_URL=postgres://user:pass@db.example.com:5432/openaccounting?sslmode=require
RESTORE_DATABASE_URL=postgres://user:pass@localhost:5432/openaccounting_restore_drill?sslmode=disable
RESTORE_DRILL_BACKUP_FILE=/backups/offsite-restored/openaccounting_latest.dump

# S3-compatible offsite destination for db-backup-offsite-sync.sh.
BACKUP_OFFSITE_S3_URI=s3://company-backups/open-accounting/prod
AWS_REGION=eu-north-1
AWS_ACCESS_KEY_ID=replace-me
AWS_SECRET_ACCESS_KEY=replace-me
EOF
else
    write_file "$OUTPUT_DIR/open-accounting-backup.env.example" <<EOF
# Copy to $ENV_FILE and store the real file in the host secret manager or a
# root-readable path outside the repository. Do not commit live credentials.
DATABASE_URL=postgres://user:pass@db.example.com:5432/openaccounting?sslmode=require
RESTORE_DATABASE_URL=postgres://user:pass@localhost:5432/openaccounting_restore_drill?sslmode=disable
RESTORE_DRILL_BACKUP_FILE=/backups/offsite-restored/openaccounting_latest.dump

# rclone-managed offsite destination for db-backup-offsite-sync.sh.
BACKUP_OFFSITE_RCLONE_REMOTE=remote:company-backups/open-accounting/prod
RCLONE_CONFIG=/etc/open-accounting/rclone.conf
EOF
fi

write_file "$OUTPUT_DIR/open-accounting-backup.service" <<EOF
[Unit]
Description=Create Open Accounting PostgreSQL backup
Wants=network-online.target
After=network-online.target

[Service]
Type=oneshot
EnvironmentFile=$ENV_FILE
ExecStart=$SCRIPTS_DIR/db-backup.sh --backup-dir $BACKUP_DIR --retention-days $RETENTION_DAYS
EOF

write_file "$OUTPUT_DIR/open-accounting-backup.timer" <<EOF
[Unit]
Description=Run Open Accounting PostgreSQL backup

[Timer]
OnCalendar=$BACKUP_CALENDAR
Persistent=true
RandomizedDelaySec=5m

[Install]
WantedBy=timers.target
EOF

write_file "$OUTPUT_DIR/open-accounting-backup-offsite.service" <<EOF
[Unit]
Description=Sync Open Accounting backups offsite
Wants=network-online.target
After=network-online.target open-accounting-backup.service

[Service]
Type=oneshot
EnvironmentFile=$ENV_FILE
ExecStart=$SCRIPTS_DIR/db-backup-offsite-sync.sh --backup-dir $BACKUP_DIR
EOF

write_file "$OUTPUT_DIR/open-accounting-backup-offsite.timer" <<EOF
[Unit]
Description=Run Open Accounting offsite backup sync

[Timer]
OnCalendar=$OFFSITE_CALENDAR
Persistent=true
RandomizedDelaySec=5m

[Install]
WantedBy=timers.target
EOF

write_file "$OUTPUT_DIR/open-accounting-backup-health.service" <<EOF
[Unit]
Description=Check Open Accounting backup health

[Service]
Type=oneshot
EnvironmentFile=$ENV_FILE
ExecStart=$SCRIPTS_DIR/db-backup-health.sh --backup-dir $BACKUP_DIR --max-age-hours $MAX_AGE_HOURS --status-file $backup_metrics_file
EOF

write_file "$OUTPUT_DIR/open-accounting-backup-health.timer" <<EOF
[Unit]
Description=Run Open Accounting backup health check

[Timer]
OnCalendar=$HEALTH_CALENDAR
Persistent=true
RandomizedDelaySec=5m

[Install]
WantedBy=timers.target
EOF

write_file "$OUTPUT_DIR/open-accounting-restore-drill.service" <<EOF
[Unit]
Description=Run Open Accounting restore drill
Wants=network-online.target
After=network-online.target

[Service]
Type=oneshot
EnvironmentFile=$ENV_FILE
ExecStart=$SCRIPTS_DIR/db-restore-drill.sh --backup \${RESTORE_DRILL_BACKUP_FILE} --restore-url \${RESTORE_DATABASE_URL} --source-url \${DATABASE_URL} --status-file $restore_metrics_file
EOF

write_file "$OUTPUT_DIR/open-accounting-restore-drill.timer" <<EOF
[Unit]
Description=Run Open Accounting weekly restore drill

[Timer]
OnCalendar=$RESTORE_CALENDAR
Persistent=true
RandomizedDelaySec=30m

[Install]
WantedBy=timers.target
EOF

write_file "$install_helper" <<EOF
#!/usr/bin/env bash
# Install generated Open Accounting backup units on a systemd host.

set -euo pipefail

SOURCE_DIR="\$(cd "\$(dirname "\${BASH_SOURCE[0]}")" && pwd)"
SYSTEMD_UNIT_DIR="\${1:-$SYSTEMD_UNIT_DIR}"
ENV_FILE="$ENV_FILE"

install -d "\$SYSTEMD_UNIT_DIR"
install -d "\$(dirname "\$ENV_FILE")"

for unit in \
  open-accounting-backup.service \
  open-accounting-backup.timer \
  open-accounting-backup-offsite.service \
  open-accounting-backup-offsite.timer \
  open-accounting-backup-health.service \
  open-accounting-backup-health.timer \
  open-accounting-restore-drill.service \
  open-accounting-restore-drill.timer; do
  install -m 0644 "\$SOURCE_DIR/\$unit" "\$SYSTEMD_UNIT_DIR/\$unit"
done

if [ ! -f "\$ENV_FILE" ]; then
  install -m 0600 "\$SOURCE_DIR/open-accounting-backup.env.example" "\$ENV_FILE"
  echo "Installed environment template at \$ENV_FILE; edit it with live credentials before the next timer run."
else
  echo "Preserved existing environment file at \$ENV_FILE"
fi

systemctl daemon-reload
systemctl enable --now \
  open-accounting-backup.timer \
  open-accounting-backup-offsite.timer \
  open-accounting-backup-health.timer \
  open-accounting-restore-drill.timer
systemctl list-timers 'open-accounting-backup*' 'open-accounting-restore-drill*'
EOF

if [ "$DRY_RUN" = false ]; then
    chmod 0755 "$install_helper"
fi

if [ "$DRY_RUN" = true ]; then
    log "Dry run complete"
else
    log "Generated Open Accounting backup systemd schedule in $OUTPUT_DIR"
fi
log "Review $OUTPUT_DIR/open-accounting-backup.env.example, install real secrets at $ENV_FILE, then run $install_helper $SYSTEMD_UNIT_DIR to copy units and enable the four timers."
