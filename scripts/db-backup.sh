#!/usr/bin/env bash
# Create a PostgreSQL custom-format backup for Open Accounting.

set -euo pipefail

DATABASE_URL_VALUE="${DATABASE_URL:-}"
BACKUP_DIR="${BACKUP_DIR:-./backups}"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-30}"
OUTPUT_FILE=""
DRY_RUN=false

usage() {
    cat <<'EOF'
Usage: scripts/db-backup.sh [options]

Options:
  --database-url URL       PostgreSQL connection URL. Defaults to DATABASE_URL.
  --backup-dir DIR         Directory for generated backups. Defaults to BACKUP_DIR or ./backups.
  --output FILE            Exact backup file path. Defaults to openaccounting_<utc>.dump in backup dir.
  --retention-days DAYS    Delete generated backups older than DAYS. Defaults to BACKUP_RETENTION_DAYS or 30.
  --no-retention           Disable retention cleanup.
  --dry-run                Print the planned backup path without running pg_dump.
  -h, --help               Show this help.

The script writes PostgreSQL custom-format dumps and a .sha256 checksum file.
EOF
}

fail() {
    echo "ERROR: $*" >&2
    exit 1
}

log() {
    echo "$*"
}

require_command() {
    command -v "$1" >/dev/null 2>&1 || fail "$1 is required but was not found in PATH"
}

is_non_negative_integer() {
    [[ "$1" =~ ^[0-9]+$ ]]
}

write_checksum() {
    local file="$1"
    local dir
    local base

    dir="$(cd "$(dirname "$file")" && pwd)"
    base="$(basename "$file")"

    if command -v sha256sum >/dev/null 2>&1; then
        (cd "$dir" && sha256sum "$base" > "$base.sha256")
    elif command -v shasum >/dev/null 2>&1; then
        (cd "$dir" && shasum -a 256 "$base" > "$base.sha256")
    else
        log "WARN: neither sha256sum nor shasum was found; checksum file was not written"
        return
    fi

    log "Checksum: $file.sha256"
}

cleanup_retained_backups() {
    if [ -z "$RETENTION_DAYS" ]; then
        return
    fi

    find "$BACKUP_DIR" -type f \
        \( -name 'openaccounting_*.dump' -o -name 'openaccounting_*.dump.sha256' \) \
        -mtime +"$RETENTION_DAYS" -delete
}

while [ "$#" -gt 0 ]; do
    case "$1" in
        --database-url)
            [ "$#" -ge 2 ] || fail "--database-url requires a value"
            DATABASE_URL_VALUE="$2"
            shift 2
            ;;
        --backup-dir)
            [ "$#" -ge 2 ] || fail "--backup-dir requires a value"
            BACKUP_DIR="$2"
            shift 2
            ;;
        --output)
            [ "$#" -ge 2 ] || fail "--output requires a value"
            OUTPUT_FILE="$2"
            shift 2
            ;;
        --retention-days)
            [ "$#" -ge 2 ] || fail "--retention-days requires a value"
            RETENTION_DAYS="$2"
            shift 2
            ;;
        --no-retention)
            RETENTION_DAYS=""
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

[ -n "$DATABASE_URL_VALUE" ] || fail "DATABASE_URL is required; pass --database-url or set DATABASE_URL"

if [ -n "$RETENTION_DAYS" ] && ! is_non_negative_integer "$RETENTION_DAYS"; then
    fail "--retention-days must be a non-negative integer"
fi

if [ -z "$OUTPUT_FILE" ]; then
    timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
    OUTPUT_FILE="$BACKUP_DIR/openaccounting_${timestamp}.dump"
fi

if [ "$DRY_RUN" = true ]; then
    log "Would create backup: $OUTPUT_FILE"
    if [ -n "$RETENTION_DAYS" ]; then
        log "Would delete generated backups older than $RETENTION_DAYS days from $BACKUP_DIR"
    fi
    exit 0
fi

require_command pg_dump
mkdir -p "$(dirname "$OUTPUT_FILE")"
mkdir -p "$BACKUP_DIR"

tmp_file="${OUTPUT_FILE}.tmp"
rm -f "$tmp_file"

log "Creating backup: $OUTPUT_FILE"
pg_dump \
    --format=custom \
    --no-owner \
    --no-privileges \
    --dbname="$DATABASE_URL_VALUE" \
    --file="$tmp_file"

mv "$tmp_file" "$OUTPUT_FILE"
write_checksum "$OUTPUT_FILE"
cleanup_retained_backups

log "Backup complete: $OUTPUT_FILE"
