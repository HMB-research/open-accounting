#!/usr/bin/env bash
# Restore an Open Accounting backup into a separate drill database and verify it.

set -euo pipefail

BACKUP_FILE="${BACKUP_FILE:-}"
RESTORE_DATABASE_URL_VALUE="${RESTORE_DATABASE_URL:-}"
SOURCE_DATABASE_URL_VALUE="${DATABASE_URL:-}"
ALLOW_NON_EMPTY=false
SKIP_CHECKSUM=false
DRY_RUN=false

usage() {
    cat <<'EOF'
Usage: scripts/db-restore-drill.sh [options]

Options:
  --backup FILE            Backup file to restore. Defaults to BACKUP_FILE.
  --restore-url URL        Target drill database URL. Defaults to RESTORE_DATABASE_URL.
  --source-url URL         Source/production database URL used for safety comparison. Defaults to DATABASE_URL.
  --allow-non-empty        Allow restoring into a database that already has application tables.
  --skip-checksum          Skip .sha256 verification when the checksum file exists.
  --dry-run                Validate arguments and print the planned restore without running pg_restore.
  -h, --help               Show this help.

The restore target must be a separate, disposable PostgreSQL database.
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

trim_space() {
    tr -d '[:space:]'
}

query_scalar() {
    local query="$1"
    psql "$RESTORE_DATABASE_URL_VALUE" -Atqc "$query" | trim_space
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
        (cd "$dir" && sha256sum -c "${base}.sha256")
    elif command -v shasum >/dev/null 2>&1; then
        (cd "$dir" && shasum -a 256 -c "${base}.sha256")
    else
        fail "checksum file exists but neither sha256sum nor shasum was found"
    fi
}

ensure_empty_target() {
    local table_count

    table_count="$(query_scalar "SELECT count(*) FROM information_schema.tables WHERE table_schema NOT IN ('pg_catalog', 'information_schema') AND table_type = 'BASE TABLE';")"
    if [ "$table_count" != "0" ] && [ "$ALLOW_NON_EMPTY" != true ]; then
        fail "restore target already contains $table_count tables; use a fresh drill database or pass --allow-non-empty"
    fi
}

verify_restore() {
    local required_tables
    local migration_count
    local tenant_count
    local user_count

    required_tables="$(query_scalar "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name IN ('schema_migrations', 'tenants', 'users', 'tenant_users');")"
    [ "$required_tables" = "4" ] || fail "restore verification failed: expected core public tables were not restored"

    migration_count="$(query_scalar "SELECT count(*) FROM public.schema_migrations;")"
    [ "$migration_count" != "0" ] || fail "restore verification failed: schema_migrations is empty"

    tenant_count="$(query_scalar "SELECT count(*) FROM public.tenants;")"
    user_count="$(query_scalar "SELECT count(*) FROM public.users;")"

    log "Restore verification passed"
    log "Migrations: $migration_count"
    log "Tenants: $tenant_count"
    log "Users: $user_count"
}

while [ "$#" -gt 0 ]; do
    case "$1" in
        --backup)
            [ "$#" -ge 2 ] || fail "--backup requires a value"
            BACKUP_FILE="$2"
            shift 2
            ;;
        --restore-url)
            [ "$#" -ge 2 ] || fail "--restore-url requires a value"
            RESTORE_DATABASE_URL_VALUE="$2"
            shift 2
            ;;
        --source-url)
            [ "$#" -ge 2 ] || fail "--source-url requires a value"
            SOURCE_DATABASE_URL_VALUE="$2"
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
            fail "unknown option: $1"
            ;;
    esac
done

[ -n "$BACKUP_FILE" ] || fail "backup file is required; pass --backup or set BACKUP_FILE"
[ -n "$RESTORE_DATABASE_URL_VALUE" ] || fail "restore database URL is required; pass --restore-url or set RESTORE_DATABASE_URL"
[ -f "$BACKUP_FILE" ] || fail "backup file does not exist: $BACKUP_FILE"

if [ -n "$SOURCE_DATABASE_URL_VALUE" ] && [ "$SOURCE_DATABASE_URL_VALUE" = "$RESTORE_DATABASE_URL_VALUE" ]; then
    fail "restore URL matches the source DATABASE_URL; refusing to restore into the source database"
fi

if [ "$DRY_RUN" = true ]; then
    log "Would restore backup: $BACKUP_FILE"
    log "Would restore into the configured drill database"
    exit 0
fi

require_command pg_restore
require_command psql

verify_checksum "$BACKUP_FILE"
ensure_empty_target

log "Restoring backup into drill database"
pg_restore \
    --clean \
    --if-exists \
    --no-owner \
    --no-privileges \
    --dbname="$RESTORE_DATABASE_URL_VALUE" \
    "$BACKUP_FILE"

verify_restore
