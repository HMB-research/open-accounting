#!/usr/bin/env bash
# Sync Open Accounting backup dumps and checksums to offsite object storage.

set -euo pipefail

BACKUP_DIR="${BACKUP_DIR:-./backups}"
S3_URI="${BACKUP_OFFSITE_S3_URI:-}"
RCLONE_REMOTE="${BACKUP_OFFSITE_RCLONE_REMOTE:-}"
DRY_RUN=false
PREFLIGHT=false
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
  --preflight               Validate destination and auth environment without scanning backups or calling providers.
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

is_placeholder_value() {
    local value
    local lowered

    value="$(printf '%s' "$1" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
    lowered="$(printf '%s' "$value" | tr '[:upper:]' '[:lower:]')"

    case "$lowered" in
        replace-me|replace_me|change-me|changeme|todo|tbd|placeholder|example|your-*|"<"*">"|*example.com*|*user:pass*|*company-backups*)
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

require_config_value() {
    local name="$1"
    local value="$2"

    [ -n "$value" ] || fail "$name is required for backup offsite preflight"
    if is_placeholder_value "$value"; then
        fail "$name still contains a placeholder value; replace it before enabling backup operations"
    fi
}

validate_readable_file_if_set() {
    local name="$1"
    local path="$2"

    [ -z "$path" ] && return
    require_config_value "$name" "$path"
    [ -r "$path" ] || fail "$name points to a file that is not readable: $path"
}

validate_s3_auth_config() {
    local has_auth=false

    [ -z "${AWS_REGION:-}" ] || require_config_value AWS_REGION "${AWS_REGION:-}"
    [ -z "${AWS_DEFAULT_REGION:-}" ] || require_config_value AWS_DEFAULT_REGION "${AWS_DEFAULT_REGION:-}"

    if [ -n "${AWS_ACCESS_KEY_ID:-}" ] || [ -n "${AWS_SECRET_ACCESS_KEY:-}" ]; then
        require_config_value AWS_ACCESS_KEY_ID "${AWS_ACCESS_KEY_ID:-}"
        require_config_value AWS_SECRET_ACCESS_KEY "${AWS_SECRET_ACCESS_KEY:-}"
        [ -z "${AWS_SESSION_TOKEN:-}" ] || require_config_value AWS_SESSION_TOKEN "${AWS_SESSION_TOKEN:-}"
        has_auth=true
    fi

    if [ -n "${AWS_PROFILE:-}" ]; then
        require_config_value AWS_PROFILE "${AWS_PROFILE:-}"
        has_auth=true
    fi

    if [ -n "${AWS_SHARED_CREDENTIALS_FILE:-}" ]; then
        validate_readable_file_if_set AWS_SHARED_CREDENTIALS_FILE "${AWS_SHARED_CREDENTIALS_FILE:-}"
        has_auth=true
    fi

    if [ -n "${AWS_WEB_IDENTITY_TOKEN_FILE:-}" ] || [ -n "${AWS_ROLE_ARN:-}" ]; then
        validate_readable_file_if_set AWS_WEB_IDENTITY_TOKEN_FILE "${AWS_WEB_IDENTITY_TOKEN_FILE:-}"
        require_config_value AWS_ROLE_ARN "${AWS_ROLE_ARN:-}"
        has_auth=true
    fi

    if [ -n "${AWS_CONTAINER_CREDENTIALS_RELATIVE_URI:-}" ]; then
        require_config_value AWS_CONTAINER_CREDENTIALS_RELATIVE_URI "${AWS_CONTAINER_CREDENTIALS_RELATIVE_URI:-}"
        has_auth=true
    fi

    if [ -n "${AWS_CONTAINER_CREDENTIALS_FULL_URI:-}" ]; then
        require_config_value AWS_CONTAINER_CREDENTIALS_FULL_URI "${AWS_CONTAINER_CREDENTIALS_FULL_URI:-}"
        has_auth=true
    fi

    [ "$has_auth" = true ] || fail "S3 preflight requires AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY, AWS_PROFILE, AWS_SHARED_CREDENTIALS_FILE, web identity, or container credential environment"
}

validate_rclone_config() {
    local default_config

    require_config_value BACKUP_OFFSITE_RCLONE_REMOTE "$RCLONE_REMOTE"
    if [ -n "${RCLONE_CONFIG:-}" ]; then
        validate_readable_file_if_set RCLONE_CONFIG "${RCLONE_CONFIG:-}"
        return
    fi

    default_config="${HOME:-}/.config/rclone/rclone.conf"
    [ -n "${HOME:-}" ] && [ -r "$default_config" ] || fail "RCLONE_CONFIG is required for rclone preflight, or the default rclone config must exist at \$HOME/.config/rclone/rclone.conf"
}

validate_destination_config() {
    if [ -n "$S3_URI" ]; then
        require_config_value BACKUP_OFFSITE_S3_URI "$S3_URI"
        validate_s3_auth_config
        return
    fi

    validate_rclone_config
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
        --preflight)
            PREFLIGHT=true
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

destination_count=0
[ -n "$S3_URI" ] && destination_count=$((destination_count + 1))
[ -n "$RCLONE_REMOTE" ] && destination_count=$((destination_count + 1))
[ "$destination_count" -eq 1 ] || fail "configure exactly one destination with --s3-uri or --rclone-remote"

if [ -n "$S3_URI" ] && [[ "$S3_URI" != s3://* ]]; then
    fail "--s3-uri must start with s3://"
fi

validate_destination_config

if [ "$PREFLIGHT" = true ]; then
    log "Offsite backup preflight passed"
    exit 0
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
