#!/usr/bin/env bash
# Sync Open Accounting backup dumps and checksums to offsite object storage.

set -euo pipefail

BACKUP_DIR="${BACKUP_DIR:-./backups}"
S3_URI="${BACKUP_OFFSITE_S3_URI:-}"
RCLONE_REMOTE="${BACKUP_OFFSITE_RCLONE_REMOTE:-}"
STATUS_FILE="${BACKUP_OFFSITE_STATUS_FILE:-}"
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
  --status-file FILE        Write Prometheus textfile metrics. Defaults to BACKUP_OFFSITE_STATUS_FILE.
  --preflight               Validate destination and auth environment without scanning backups or calling providers.
  --dry-run                 Validate inputs and print planned uploads without calling aws or rclone.
  -h, --help                Show this help.

Exactly one destination is required. The script never deletes remote objects.
It requires, syncs, and verifies each selected .dump file against its matching
FILE.sha256 checksum before reporting success.
EOF
}

write_status() {
    local healthy="$1"

    if [ -z "$STATUS_FILE" ]; then
        return
    fi

    mkdir -p "$(dirname "$STATUS_FILE")"
    {
        echo "# HELP open_accounting_offsite_backup_health Latest offsite backup copy health, 1 for healthy and 0 for unhealthy."
        echo "# TYPE open_accounting_offsite_backup_health gauge"
        echo "open_accounting_offsite_backup_health $healthy"
        if [ "$healthy" = "1" ]; then
            echo "# HELP open_accounting_offsite_backup_last_success_timestamp_seconds Unix timestamp of the last verified offsite copy."
            echo "# TYPE open_accounting_offsite_backup_last_success_timestamp_seconds gauge"
            echo "open_accounting_offsite_backup_last_success_timestamp_seconds $(date -u +%s)"
        fi
    } > "${STATUS_FILE}.tmp"
    mv "${STATUS_FILE}.tmp" "$STATUS_FILE"
}

fail() {
    write_status 0
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
    [ -f "$checksum_file" ] || fail "checksum file is missing: $checksum_file"
    add_file "$checksum_file"
}

verify_local_checksum() {
    local file="$1"
    local checksum_file="${file}.sha256"
    local dir
    local base

    dir="$(cd "$(dirname "$file")" && pwd)"
    base="$(basename "$file")"
    if command -v sha256sum >/dev/null 2>&1; then
        (cd "$dir" && sha256sum -c "${base}.sha256")
    elif command -v shasum >/dev/null 2>&1; then
        (cd "$dir" && shasum -a 256 -c "${base}.sha256")
    else
        fail "checksum verification requires sha256sum or shasum"
    fi
}

verify_s3_backup() {
    local file="$1"
    local destination="$2"
    local verification_file
    local verification_checksum
    local dir
    local base

    verification_file="$(mktemp)"
    verification_checksum="${verification_file}.sha256"
    trap 'rm -f "$verification_file" "$verification_checksum"' RETURN
    aws s3 cp "$destination" "$verification_file" --only-show-errors

    dir="$(dirname "$verification_file")"
    base="$(basename "$verification_file")"
    cp "${file}.sha256" "$verification_checksum"
    sed -i.bak "s|  .*|  ${base}|" "$verification_checksum"
    rm -f "${verification_checksum}.bak"
    if command -v sha256sum >/dev/null 2>&1; then
        (cd "$dir" && sha256sum -c "$(basename "$verification_checksum")")
    elif command -v shasum >/dev/null 2>&1; then
        (cd "$dir" && shasum -a 256 -c "$(basename "$verification_checksum")")
    else
        fail "checksum verification requires sha256sum or shasum"
    fi
}

verify_rclone_backup() {
    local file="$1"
    local destination="$2"

    rclone check --one-way --checksum "$file" "$destination"
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
        --status-file)
            [ "$#" -ge 2 ] || fail "--status-file requires a value"
            STATUS_FILE="$2"
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
    [ -z "$STATUS_FILE" ] || log "Would write status metrics: $STATUS_FILE"
    exit 0
fi

for backup_file in "${FILES_TO_SYNC[@]}"; do
    case "$backup_file" in
        *.dump)
            verify_local_checksum "$backup_file"
            ;;
    esac
done

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

for backup_file in "${FILES_TO_SYNC[@]}"; do
    case "$backup_file" in
        *.dump)
            if [ -n "$S3_URI" ]; then
                verify_s3_backup "$backup_file" "$(destination_for_file "$backup_file")"
            else
                verify_rclone_backup "$backup_file" "$(destination_for_file "$backup_file")"
            fi
            ;;
    esac
done

write_status 1
log "Offsite backup sync complete"
