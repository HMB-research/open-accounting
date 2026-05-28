#!/usr/bin/env bash
# Check the latest Open Accounting backup and optionally emit Prometheus textfile metrics.

set -euo pipefail

BACKUP_DIR="${BACKUP_DIR:-./backups}"
BACKUP_FILE="${BACKUP_FILE:-}"
MAX_AGE_HOURS="${BACKUP_MAX_AGE_HOURS:-26}"
MIN_SIZE_BYTES="${BACKUP_MIN_SIZE_BYTES:-1}"
STATUS_FILE="${BACKUP_STATUS_FILE:-}"
REQUIRE_CHECKSUM=true
DRY_RUN=false

LATEST_BACKUP=""
AGE_SECONDS=""
SIZE_BYTES=""

usage() {
    cat <<'EOF'
Usage: scripts/db-backup-health.sh [options]

Options:
  --backup-dir DIR             Directory to scan for openaccounting_*.dump backups. Defaults to BACKUP_DIR or ./backups.
  --backup FILE                Exact backup file to check. Defaults to BACKUP_FILE or latest openaccounting_*.dump.
  --max-age-hours HOURS        Maximum acceptable backup age. Defaults to BACKUP_MAX_AGE_HOURS or 26.
  --min-size-bytes BYTES       Minimum acceptable backup size. Defaults to BACKUP_MIN_SIZE_BYTES or 1.
  --status-file FILE           Write Prometheus textfile metrics. Defaults to BACKUP_STATUS_FILE.
  --allow-missing-checksum     Do not fail when FILE.sha256 is absent.
  --dry-run                    Print the planned health check without inspecting files.
  -h, --help                   Show this help.

The check verifies backup freshness, non-empty size, and the .sha256 checksum when present.
EOF
}

log() {
    echo "$*"
}

write_status() {
    local healthy="$1"

    if [ -z "$STATUS_FILE" ]; then
        return
    fi

    mkdir -p "$(dirname "$STATUS_FILE")"
    {
        echo "# HELP open_accounting_backup_health Latest backup health, 1 for healthy and 0 for unhealthy."
        echo "# TYPE open_accounting_backup_health gauge"
        echo "open_accounting_backup_health $healthy"
        if [ -n "$AGE_SECONDS" ]; then
            echo "# HELP open_accounting_backup_latest_age_seconds Age of the latest checked backup."
            echo "# TYPE open_accounting_backup_latest_age_seconds gauge"
            echo "open_accounting_backup_latest_age_seconds $AGE_SECONDS"
        fi
        if [ -n "$SIZE_BYTES" ]; then
            echo "# HELP open_accounting_backup_latest_size_bytes Size of the latest checked backup."
            echo "# TYPE open_accounting_backup_latest_size_bytes gauge"
            echo "open_accounting_backup_latest_size_bytes $SIZE_BYTES"
        fi
    } > "${STATUS_FILE}.tmp"
    mv "${STATUS_FILE}.tmp" "$STATUS_FILE"
}

fail() {
    write_status 0
    echo "ERROR: $*" >&2
    exit 1
}

is_non_negative_integer() {
    [[ "$1" =~ ^[0-9]+$ ]]
}

mtime_epoch() {
    if stat -c %Y "$1" >/dev/null 2>&1; then
        stat -c %Y "$1"
    else
        stat -f %m "$1"
    fi
}

file_size() {
    if stat -c %s "$1" >/dev/null 2>&1; then
        stat -c %s "$1"
    else
        stat -f %z "$1"
    fi
}

latest_backup_in_dir() {
    find "$BACKUP_DIR" -type f -name 'openaccounting_*.dump' -print | sort | tail -n 1
}

verify_checksum() {
    local file="$1"
    local checksum_file="${file}.sha256"
    local dir
    local base

    if [ ! -f "$checksum_file" ]; then
        if [ "$REQUIRE_CHECKSUM" = true ]; then
            fail "checksum file is missing: $checksum_file"
        fi
        return
    fi

    dir="$(cd "$(dirname "$file")" && pwd)"
    base="$(basename "$file")"

    if command -v sha256sum >/dev/null 2>&1; then
        (cd "$dir" && sha256sum -c "${base}.sha256")
    elif command -v shasum >/dev/null 2>&1; then
        (cd "$dir" && shasum -a 256 -c "${base}.sha256")
    else
        fail "checksum file exists but neither sha256sum nor shasum was found"
    fi
}

while [ "$#" -gt 0 ]; do
    case "$1" in
        --backup-dir)
            [ "$#" -ge 2 ] || fail "--backup-dir requires a value"
            BACKUP_DIR="$2"
            shift 2
            ;;
        --backup)
            [ "$#" -ge 2 ] || fail "--backup requires a value"
            BACKUP_FILE="$2"
            shift 2
            ;;
        --max-age-hours)
            [ "$#" -ge 2 ] || fail "--max-age-hours requires a value"
            MAX_AGE_HOURS="$2"
            shift 2
            ;;
        --min-size-bytes)
            [ "$#" -ge 2 ] || fail "--min-size-bytes requires a value"
            MIN_SIZE_BYTES="$2"
            shift 2
            ;;
        --status-file)
            [ "$#" -ge 2 ] || fail "--status-file requires a value"
            STATUS_FILE="$2"
            shift 2
            ;;
        --allow-missing-checksum)
            REQUIRE_CHECKSUM=false
            shift
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

is_non_negative_integer "$MAX_AGE_HOURS" || fail "--max-age-hours must be a non-negative integer"
is_non_negative_integer "$MIN_SIZE_BYTES" || fail "--min-size-bytes must be a non-negative integer"

if [ "$DRY_RUN" = true ]; then
    if [ -n "$BACKUP_FILE" ]; then
        log "Would check backup: $BACKUP_FILE"
    else
        log "Would check latest openaccounting_*.dump in $BACKUP_DIR"
    fi
    [ -z "$STATUS_FILE" ] || log "Would write status metrics: $STATUS_FILE"
    exit 0
fi

if [ -z "$BACKUP_FILE" ]; then
    [ -d "$BACKUP_DIR" ] || fail "backup directory does not exist: $BACKUP_DIR"
    BACKUP_FILE="$(latest_backup_in_dir)"
fi

[ -n "$BACKUP_FILE" ] || fail "no openaccounting_*.dump backup files found in $BACKUP_DIR"
[ -f "$BACKUP_FILE" ] || fail "backup file does not exist: $BACKUP_FILE"

LATEST_BACKUP="$BACKUP_FILE"
SIZE_BYTES="$(file_size "$LATEST_BACKUP")"
if [ "$SIZE_BYTES" -lt "$MIN_SIZE_BYTES" ]; then
    fail "backup is too small: $SIZE_BYTES bytes is below $MIN_SIZE_BYTES"
fi

now_epoch="$(date -u +%s)"
backup_epoch="$(mtime_epoch "$LATEST_BACKUP")"
AGE_SECONDS="$((now_epoch - backup_epoch))"
max_age_seconds="$((MAX_AGE_HOURS * 3600))"
if [ "$AGE_SECONDS" -gt "$max_age_seconds" ]; then
    fail "latest backup is too old: ${AGE_SECONDS}s exceeds ${max_age_seconds}s"
fi

verify_checksum "$LATEST_BACKUP"
write_status 1

log "Backup health passed"
log "Backup: $LATEST_BACKUP"
log "Age seconds: $AGE_SECONDS"
log "Size bytes: $SIZE_BYTES"
