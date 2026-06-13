#!/usr/bin/env bash
# Restore an Open Accounting backup into a separate drill database and verify it.

set -euo pipefail

BACKUP_FILE="${BACKUP_FILE:-}"
RESTORE_DATABASE_URL_VALUE="${RESTORE_DATABASE_URL:-}"
SOURCE_DATABASE_URL_VALUE="${DATABASE_URL:-}"
ALLOW_NON_EMPTY=false
SKIP_CHECKSUM=false
DRY_RUN=false
STATUS_FILE="${RESTORE_DRILL_STATUS_FILE:-}"

FAILURE_CODE=""
MIGRATION_COUNT=""
TENANT_COUNT=""
USER_COUNT=""

usage() {
    cat <<'EOF'
Usage: scripts/db-restore-drill.sh [options]

Options:
  --backup FILE            Backup file to restore. Defaults to BACKUP_FILE.
  --restore-url URL        Target drill database URL. Defaults to RESTORE_DATABASE_URL.
  --source-url URL         Source/production database URL used for safety comparison. Defaults to DATABASE_URL.
  --allow-non-empty        Allow restoring into a database that already has application tables.
  --skip-checksum          Skip .sha256 verification when the checksum file exists.
  --status-file FILE       Write Prometheus textfile metrics. Defaults to RESTORE_DRILL_STATUS_FILE.
  --dry-run                Validate arguments and print the planned restore without running pg_restore.
  -h, --help               Show this help.

The restore target must be a separate, disposable PostgreSQL database.
Failures are written as ERROR[code] so scheduled jobs can route alerts by cause.
EOF
}

fail() {
    local code="$1"

    shift
    FAILURE_CODE="$code"
    set +e
    write_status 0
    echo "ERROR[$code]: $*" >&2
    exit 1
}

log() {
    echo "$*"
}

require_command() {
    command -v "$1" >/dev/null 2>&1 || fail "missing_dependency" "$1 is required but was not found in PATH"
}

trim_space() {
    tr -d '[:space:]'
}

query_scalar() {
    local query="$1"
    local output

    if ! output="$(psql "$RESTORE_DATABASE_URL_VALUE" -Atqc "$query")"; then
        fail "restore_query_failed" "database query failed during restore drill verification"
    fi
    printf '%s' "$output" | trim_space
}

write_status() {
    local healthy="$1"

    if [ -z "$STATUS_FILE" ]; then
        return
    fi

    mkdir -p "$(dirname "$STATUS_FILE")"
    {
        echo "# HELP open_accounting_restore_drill_health Latest restore drill health, 1 for healthy and 0 for unhealthy."
        echo "# TYPE open_accounting_restore_drill_health gauge"
        echo "open_accounting_restore_drill_health $healthy"
        if [ "$healthy" = "1" ]; then
            echo "# HELP open_accounting_restore_drill_last_success_timestamp_seconds Unix timestamp of the last successful restore drill."
            echo "# TYPE open_accounting_restore_drill_last_success_timestamp_seconds gauge"
            echo "open_accounting_restore_drill_last_success_timestamp_seconds $(date -u +%s)"
        fi
        if [ -n "$FAILURE_CODE" ]; then
            echo "# HELP open_accounting_restore_drill_failure_info Last restore drill failure code."
            echo "# TYPE open_accounting_restore_drill_failure_info gauge"
            echo "open_accounting_restore_drill_failure_info{code=\"$FAILURE_CODE\"} 1"
        fi
        if [ -n "$MIGRATION_COUNT" ]; then
            echo "# HELP open_accounting_restore_drill_schema_migrations Restored schema migration rows."
            echo "# TYPE open_accounting_restore_drill_schema_migrations gauge"
            echo "open_accounting_restore_drill_schema_migrations $MIGRATION_COUNT"
        fi
        if [ -n "$TENANT_COUNT" ]; then
            echo "# HELP open_accounting_restore_drill_tenants Restored tenant rows."
            echo "# TYPE open_accounting_restore_drill_tenants gauge"
            echo "open_accounting_restore_drill_tenants $TENANT_COUNT"
        fi
        if [ -n "$USER_COUNT" ]; then
            echo "# HELP open_accounting_restore_drill_users Restored user rows."
            echo "# TYPE open_accounting_restore_drill_users gauge"
            echo "open_accounting_restore_drill_users $USER_COUNT"
        fi
    } > "${STATUS_FILE}.tmp"
    mv "${STATUS_FILE}.tmp" "$STATUS_FILE"
}

verify_checksum() {
    local file="$1"
    local checksum_file="${file}.sha256"
    local dir
    local base

    if [ "$SKIP_CHECKSUM" = true ] || [ ! -f "$checksum_file" ]; then
        return
    fi

    dir="$(cd "$(dirname "$file")" && pwd)"
    base="$(basename "$file")"

    if command -v sha256sum >/dev/null 2>&1; then
        if ! (cd "$dir" && sha256sum -c "${base}.sha256"); then
            fail "checksum_failed" "checksum verification failed: $checksum_file"
        fi
    elif command -v shasum >/dev/null 2>&1; then
        if ! (cd "$dir" && shasum -a 256 -c "${base}.sha256"); then
            fail "checksum_failed" "checksum verification failed: $checksum_file"
        fi
    else
        fail "missing_dependency" "checksum file exists but neither sha256sum nor shasum was found"
    fi
}

ensure_empty_target() {
    local table_count

    table_count="$(query_scalar "SELECT count(*) FROM information_schema.tables WHERE table_schema NOT IN ('pg_catalog', 'information_schema') AND table_type = 'BASE TABLE';")"
    if [ "$table_count" != "0" ] && [ "$ALLOW_NON_EMPTY" != true ]; then
        fail "restore_target_not_empty" "restore target already contains $table_count tables; use a fresh drill database or pass --allow-non-empty"
    fi
}

verify_restore() {
    local required_tables

    required_tables="$(query_scalar "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name IN ('schema_migrations', 'tenants', 'users', 'tenant_users');")"
    [ "$required_tables" = "4" ] || fail "restore_verification_failed" "restore verification failed: expected core public tables were not restored"

    MIGRATION_COUNT="$(query_scalar "SELECT count(*) FROM public.schema_migrations;")"
    [ "$MIGRATION_COUNT" != "0" ] || fail "restore_verification_failed" "restore verification failed: schema_migrations is empty"

    TENANT_COUNT="$(query_scalar "SELECT count(*) FROM public.tenants;")"
    USER_COUNT="$(query_scalar "SELECT count(*) FROM public.users;")"

    write_status 1
    log "Restore verification passed"
    log "Migrations: $MIGRATION_COUNT"
    log "Tenants: $TENANT_COUNT"
    log "Users: $USER_COUNT"
}

while [ "$#" -gt 0 ]; do
    case "$1" in
        --backup)
            [ "$#" -ge 2 ] || fail "invalid_arguments" "--backup requires a value"
            BACKUP_FILE="$2"
            shift 2
            ;;
        --restore-url)
            [ "$#" -ge 2 ] || fail "invalid_arguments" "--restore-url requires a value"
            RESTORE_DATABASE_URL_VALUE="$2"
            shift 2
            ;;
        --source-url)
            [ "$#" -ge 2 ] || fail "invalid_arguments" "--source-url requires a value"
            SOURCE_DATABASE_URL_VALUE="$2"
            shift 2
            ;;
        --status-file)
            [ "$#" -ge 2 ] || fail "invalid_arguments" "--status-file requires a value"
            STATUS_FILE="$2"
            shift 2
            ;;
        --allow-non-empty)
            ALLOW_NON_EMPTY=true
            shift
            ;;
        --skip-checksum)
            SKIP_CHECKSUM=true
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
            fail "invalid_arguments" "unknown option: $1"
            ;;
    esac
done

[ -n "$BACKUP_FILE" ] || fail "invalid_arguments" "backup file is required; pass --backup or set BACKUP_FILE"
[ -n "$RESTORE_DATABASE_URL_VALUE" ] || fail "invalid_arguments" "restore database URL is required; pass --restore-url or set RESTORE_DATABASE_URL"
[ -f "$BACKUP_FILE" ] || fail "backup_not_found" "backup file does not exist: $BACKUP_FILE"

if [ -n "$SOURCE_DATABASE_URL_VALUE" ] && [ "$SOURCE_DATABASE_URL_VALUE" = "$RESTORE_DATABASE_URL_VALUE" ]; then
    fail "unsafe_restore_target" "restore URL matches the source DATABASE_URL; refusing to restore into the source database"
fi

if [ "$DRY_RUN" = true ]; then
    log "Would restore backup: $BACKUP_FILE"
    log "Would restore into the configured drill database"
    [ -z "$STATUS_FILE" ] || log "Would write status metrics: $STATUS_FILE"
    exit 0
fi

require_command pg_restore
require_command psql

verify_checksum "$BACKUP_FILE"
ensure_empty_target

log "Restoring backup into drill database"
if ! pg_restore \
    --clean \
    --if-exists \
    --no-owner \
    --no-privileges \
    --dbname="$RESTORE_DATABASE_URL_VALUE" \
    "$BACKUP_FILE"; then
    fail "restore_failed" "pg_restore failed"
fi

verify_restore
