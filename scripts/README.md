# Open Accounting Scripts

## Database Migrations

### Running Migrations

The `migrate` binary handles database schema migrations.

```bash
# Build the migration tool
go build -o migrate ./cmd/migrate

# Run migrations (applies all pending migrations)
./migrate -db "$DATABASE_URL"

# Or with explicit path
./migrate -db "$DATABASE_URL" -path ./migrations -direction up
```

### Migration Options

| Flag | Description | Default |
|------|-------------|---------|
| `-db` | PostgreSQL connection URL | Required |
| `-path` | Migrations directory | `./migrations` |
| `-direction` | `up` or `down` | `up` |

### Railway Deployment

Add this as a **release command** in Railway:
```bash
./migrate -db $DATABASE_URL
```

Or run manually via Railway CLI:
```bash
railway run ./migrate -db $DATABASE_URL
```

---

## Demo Mode

Demo mode provides sample data for testing and demonstrations. It includes:

- **Demo User**: `demo@openaccounting.io` / `demo123`
- **Demo Organization**: Acme Corporation
- Sample chart of accounts (Estonian standard)
- Sample customers & suppliers
- Sample invoices (various statuses)
- Sample payments

### Enabling Demo Mode

Set these environment variables:

```bash
DEMO_MODE=true
```

### Seeding Demo Data

```bash
# After running migrations
psql $DATABASE_URL -f scripts/demo-seed.sql
```

### Automatic Hourly Reset

For public demos, enable automatic hourly resets using the API endpoint:

#### Option 1: API Endpoint (Recommended for Railway)

1. Set environment variables:
   ```bash
   DEMO_MODE=true
   DEMO_RESET_SECRET=your-secret-key-here
   ```

2. Use an external cron service (e.g., cron-job.org) to call:
   ```bash
   curl -X POST https://your-api.up.railway.app/api/demo/reset \
     -H "X-Demo-Secret: your-secret-key-here"
   ```

   Or with query parameter:
   ```bash
   curl -X POST "https://your-api.up.railway.app/api/demo/reset?secret=your-secret-key-here"
   ```

#### Option 2: Shell Script (for Docker/self-hosted)

1. Set environment variable:
   ```bash
   DEMO_MODE=true
   ```

2. Set up cron job (in Dockerfile or Railway cron):
   ```bash
   0 * * * * /app/scripts/demo-reset.sh >> /var/log/demo-reset.log 2>&1
   ```

### Railway Cron Setup

Create a separate Railway service for the cron job:

1. Create new service → Docker
2. Use this Dockerfile:

```dockerfile
FROM postgres:16-alpine

# Install cron
RUN apk add --no-cache dcron

# Copy scripts
COPY scripts/demo-seed.sql /app/scripts/
COPY scripts/demo-reset.sh /app/scripts/
RUN chmod +x /app/scripts/demo-reset.sh

# Setup cron
RUN echo "0 * * * * /app/scripts/demo-reset.sh >> /var/log/cron.log 2>&1" > /etc/crontabs/root

# Run cron in foreground
CMD ["crond", "-f", "-l", "2"]
```

3. Set environment variables:
   ```
   DATABASE_URL=${{Postgres.DATABASE_URL}}
   DEMO_MODE=true
   ```

### Demo Credentials

| Field | Value |
|-------|-------|
| Email | `demo@example.com` |
| Password | `demo123` |
| Organization | Acme Corporation |

---

## Production Checklist

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `DATABASE_URL` | Yes | PostgreSQL connection string |
| `APP_ENV` | Yes | Set to `production` for production deployments |
| `JWT_SECRET` | Yes | Secret for JWT signing (min 32 chars) |
| `PORT` | No | Server port (default: 8080) |
| `ALLOWED_ORIGINS` | Yes | CORS origins (comma-separated) |
| `DEMO_MODE` | No | Enable demo features (default: false) |

### Startup Sequence

1. **Database**: Ensure PostgreSQL is running and healthy
2. **Migrations**: Run `./migrate -db $DATABASE_URL`
3. **Seed (optional)**: Run `psql $DATABASE_URL -f scripts/demo-seed.sql`
4. **API Server**: Start `./api`

### Health Check

```bash
curl http://localhost:8080/health
# Returns: OK
```

### Backup and Restore Drill

Create custom-format backups with a checksum:

```bash
DATABASE_URL="postgres://user:pass@host:5432/openaccounting?sslmode=require" \
  scripts/db-backup.sh --backup-dir ./backups --retention-days 30
```

The Go CLI exposes the same local operator flow when the scripts are present:

```bash
DATABASE_URL="postgres://user:pass@host:5432/openaccounting?sslmode=require" \
  go run ./cmd/oa ops backup create --backup-dir ./backups --retention-days 30
```

Set `OA_SCRIPT_DIR` when running a built `oa` binary outside the repository checkout.

Use PostgreSQL client tools from the same major version as the server. For self-hosted Docker deployments, the production backup service runs the script from a matching PostgreSQL image.

Verify a backup by restoring it into a separate disposable database:

```bash
createdb openaccounting_restore_drill

RESTORE_DATABASE_URL="postgres://user:pass@localhost:5432/openaccounting_restore_drill?sslmode=disable" \
  DATABASE_URL="postgres://user:pass@host:5432/openaccounting?sslmode=require" \
  scripts/db-restore-drill.sh --backup ./backups/openaccounting_20260528T120000Z.dump

dropdb openaccounting_restore_drill
```

The restore drill refuses to use the same URL as `DATABASE_URL`, checks the `.sha256` file when present, and verifies that core Open Accounting tables and migrations were restored.

Check backup freshness and checksum from cron, systemd timers, or monitoring agents:

```bash
scripts/db-backup-health.sh \
  --backup-dir ./backups \
  --max-age-hours 26 \
  --status-file /var/lib/node_exporter/textfile_collector/openaccounting_backup.prom
```

The health check finds the newest `openaccounting_*.dump`, requires a non-empty file, verifies `FILE.sha256`, fails when the backup is older than the configured threshold, and can write Prometheus textfile metrics.

Sync local backups offsite after the backup script writes both the dump and checksum:

```bash
BACKUP_OFFSITE_S3_URI="s3://company-backups/open-accounting/prod" \
  scripts/db-backup-offsite-sync.sh --backup-dir ./backups

go run ./cmd/oa ops backup offsite-sync --backup-dir ./backups --s3-uri s3://company-backups/open-accounting/prod --dry-run
```

For non-AWS providers, configure an rclone remote on the host and use:

```bash
BACKUP_OFFSITE_RCLONE_REMOTE="b2:company-backups/open-accounting/prod" \
  scripts/db-backup-offsite-sync.sh --backup-dir ./backups
```

The offsite sync helper copies selected `openaccounting_*.dump` files plus matching `.sha256` files and never deletes remote objects. Use `--backup FILE` to sync one specific dump, and `--dry-run` to verify the planned uploads from cron, systemd timers, or deployment automation.

---

## Troubleshooting

### "relation does not exist" errors

Migrations haven't run. Execute:
```bash
./migrate -db $DATABASE_URL
```

### Demo user can't login

Re-run the seed script:
```bash
psql $DATABASE_URL -f scripts/demo-seed.sql
```

### Schema already exists

The seed is idempotent - it uses `ON CONFLICT DO NOTHING`. Safe to re-run.
