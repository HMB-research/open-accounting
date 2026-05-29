#!/usr/bin/env bash
# Sync Open Accounting backup dumps and checksums to offsite object storage.

set -euo pipefail

BACKUP_DIR="${BACKUP_DIR:-./backups}"
S3_URI="${BACKUP_OFFSITE_S3_URI:-}"
RCLONE_REMOTE="${BACKUP_OFFSITE_RCLONE_REMOTE:-}"
DRY_RUN=false
BACKUP_FILES=()
FILES_TO_SYNC=()

usage() {
    cat <<'EOF'
Usage: scripts/db-backup-offsite-sync.sh [options]

Options:
  --backup-dir DIR          Directory to scan for openaccounting_*.dump backups. Defaults to BACKUP_DIR or ./backups.
  --backup FILE             Exact backup file to sync. Can be repeated. Defaults to all openaccounting_*.dump files in backup dir.
  --s3-uri URI              Destination S3 URI, for example s3://bucket/prefix. Defaults to BACKUP_OFFSITE_S3_URI.
  --rclone-remote REMOTE    Destination rclone remote path, for example remote:bucket/prefix. Defaults to BACKUP_OFFSITE_RCLONE_REMOTE.
  --dry-run                 Validate inputs and print planned uploads without calling aws or rclone.
  -h, --help                Show this help.

Exactly one destination is required. The script never deletes remote objects.
It syncs each selected .dump file and the matching FILE.sha256 checksum when present.
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

contains_file() {
    local candidate="$1"
    local existing

    if [ "${#FILES_TO_SYNC[@]}" -eq 0 ]; then
        return 1
    fi

    for existing in "${FILES_TO_SYNC[@]}"; do
        if [ "$existing" = "$candidate" ]; then
            return 0
        fi
    done
    return 1
}

add_file() {
    local file="$1"

    if ! contains_file "$file"; then
        FILES_TO_SYNC+=("$file")
    fi
}

add_backup_and_checksum() {
    local backup_file="$1"
    local checksum_file="${backup_file}.sha256"

    [ -f "$backup_file" ] || fail "backup file does not exist: $backup_file"
    add_file "$backup_file"
    if [ -f "$checksum_file" ]; then
        add_file "$checksum_file"
    else
        log "WARN: checksum file is missing and will not be synced: $checksum_file"
    fi
}

trim_trailing_slash() {
    local value="$1"
    while [ "${value%/}" != "$value" ]; do
        value="${value%/}"
    done
    echo "$value"
}

destination_for_file() {
    local file="$1"
    local base

    base="$(basename "$file")"
    if [ -n "$S3_URI" ]; then
        echo "$(trim_trailing_slash "$S3_URI")/$base"
        return
    fi
    echo "$(trim_trailing_slash "$RCLONE_REMOTE")/$base"
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
            BACKUP_FILES+=("$2")
            shift 2
            ;;
        --s3-uri)
            [ "$#" -ge 2 ] || fail "--s3-uri requires a value"
            S3_URI="$2"
            shift 2
            ;;
        --rclone-remote)
            [ "$#" -ge 2 ] || fail "--rclone-remote requires a value"
            RCLONE_REMOTE="$2"
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

destination_count=0
[ -n "$S3_URI" ] && destination_count=$((destination_count + 1))
[ -n "$RCLONE_REMOTE" ] && destination_count=$((destination_count + 1))
[ "$destination_count" -eq 1 ] || fail "configure exactly one destination with --s3-uri or --rclone-remote"

if [ -n "$S3_URI" ] && [[ "$S3_URI" != s3://* ]]; then
    fail "--s3-uri must start with s3://"
fi

if [ "${#BACKUP_FILES[@]}" -gt 0 ]; then
    for backup_file in "${BACKUP_FILES[@]}"; do
        add_backup_and_checksum "$backup_file"
    done
else
    [ -d "$BACKUP_DIR" ] || fail "backup directory does not exist: $BACKUP_DIR"
    while IFS= read -r backup_file; do
        add_backup_and_checksum "$backup_file"
    done < <(find "$BACKUP_DIR" -type f -name 'openaccounting_*.dump' -print | sort)
fi

[ "${#FILES_TO_SYNC[@]}" -gt 0 ] || fail "no openaccounting_*.dump backup files found"

if [ "$DRY_RUN" = true ]; then
    log "Would sync ${#FILES_TO_SYNC[@]} files offsite"
    for file in "${FILES_TO_SYNC[@]}"; do
        log "$file -> $(destination_for_file "$file")"
    done
    exit 0
fi

if [ -n "$S3_URI" ]; then
    require_command aws
    for file in "${FILES_TO_SYNC[@]}"; do
        aws s3 cp "$file" "$(destination_for_file "$file")" --only-show-errors
    done
else
    require_command rclone
    for file in "${FILES_TO_SYNC[@]}"; do
        rclone copyto "$file" "$(destination_for_file "$file")"
    done
fi

log "Offsite backup sync complete"
