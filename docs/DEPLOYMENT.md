# Deployment Guide

This guide covers deploying Open Accounting to production environments.

## Prerequisites

- Docker and Docker Compose (recommended)
- OR: Go 1.26+, Node.js 22+, PostgreSQL 16+
- A domain name with SSL certificate
- Minimum 2GB RAM, 10GB storage

## Environment Variables

### Backend API

| Variable | Required | Description | Example |
|----------|----------|-------------|---------|
| `DATABASE_URL` | Yes | PostgreSQL connection string | `postgres://user:pass@host:5432/db?sslmode=require` |
| `APP_ENV` | Yes | Set to `production` to reject insecure defaults | `production` |
| `JWT_SECRET` | Yes | Secret key for JWT signing (min 32 chars) | `your-super-secret-key-min-32-chars` |
| `PORT` | No | API server port | `8080` |
| `ALLOWED_ORIGINS` | Yes | CORS allowed origins (comma-separated) | `https://app.example.com,https://admin.example.com` |
| `CORS_DEBUG` | No | Enable verbose CORS logging | `true` |
| `LOG_LEVEL` | No | Log verbosity (trace, debug, info, warn, error) | `debug` |
| `DEMO_RESET_SECRET` | No | Secret key for demo reset endpoint | `your-reset-secret` |
| `PASSWORD_RESET_BASE_URL` | No | Frontend reset URL used in password reset emails | `https://app.example.com/reset-password` |
| `PASSWORD_RESET_SMTP_HOST` | No | SMTP host for password reset email delivery | `smtp.example.com` |
| `PASSWORD_RESET_SMTP_PORT` | No | SMTP port for password reset email delivery | `587` |
| `PASSWORD_RESET_SMTP_USERNAME` | No | SMTP username for password reset email delivery | `mailer@example.com` |
| `PASSWORD_RESET_SMTP_PASSWORD` | No | SMTP password for password reset email delivery | `secret` |
| `PASSWORD_RESET_SMTP_FROM_EMAIL` | No | From address for password reset email delivery | `no-reply@example.com` |
| `PASSWORD_RESET_SMTP_FROM_NAME` | No | From name for password reset email delivery | `Open Accounting` |
| `PASSWORD_RESET_SMTP_USE_TLS` | No | Require TLS for password reset email delivery | `true` |
| `PASSWORD_RESET_EXPOSE_TOKEN` | No | Return reset tokens in API responses for local/dev only | `false` |
| `SCHEDULER_ENABLED` | No | Enable recurring invoice, recurring journal entry, payment reminder, and document retention reminder scheduler jobs | `true` |
| `RECURRING_INVOICE_SCHEDULE` | No | Cron schedule for recurring invoice generation | `0 6 * * *` |
| `RECURRING_JOURNAL_ENTRY_SCHEDULE` | No | Cron schedule for recurring journal entry generation | `15 6 * * *` |
| `DOCUMENT_RETENTION_REMINDER_SCHEDULE` | No | Cron schedule for document retention reminder delivery | `30 9 * * *` |
| `DOCUMENT_RETENTION_REMINDER_HORIZON_DAYS` | No | Retention reminder lookahead horizon in days | `30` |
| `DOCUMENT_RETENTION_REMINDER_INCLUDE_MISSING` | No | Include documents missing retention metadata in reminder digests | `true` |
| `DOCUMENT_RETENTION_REMINDER_MAX_ATTEMPTS` | No | Retry failed document retention reminder delivery attempts before reporting failure | `3` |
| `DOCUMENT_RETENTION_REMINDER_ESCALATE_AFTER_ATTEMPTS` | No | Mark failed document retention reminder delivery as escalated after this many attempts | `3` |

When `APP_ENV=production`, startup fails if `JWT_SECRET` is missing or shorter than 32 characters, or if `ALLOWED_ORIGINS` is empty. Production mode also uses only the configured origins instead of appending localhost development origins.

### Frontend

| Variable | Required | Description | Example |
|----------|----------|-------------|---------|
| `PUBLIC_API_URL` | Yes | Backend API URL (must include `https://`) | `https://api.example.com` |

> **Note:** If `PUBLIC_API_URL` is set without a protocol (e.g., `api.example.com`), the frontend will automatically prepend `https://`.

## Docker Deployment

### 1. Production Docker Compose

Create a `docker-compose.prod.yml`:

```yaml
version: '3.8'

services:
  api:
    image: ghcr.io/hmb-research/open-accounting:latest
    environment:
      - DATABASE_URL=${DATABASE_URL}
      - APP_ENV=production
      - JWT_SECRET=${JWT_SECRET}
      - PORT=8080
      - ALLOWED_ORIGINS=${ALLOWED_ORIGINS}
    ports:
      - "8080:8080"
    depends_on:
      - db
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3

  db:
    image: postgres:16-alpine
    environment:
      - POSTGRES_USER=${DB_USER}
      - POSTGRES_PASSWORD=${DB_PASSWORD}
      - POSTGRES_DB=${DB_NAME}
    volumes:
      - postgres_data:/var/lib/postgresql/data
    restart: unless-stopped

  migrate:
    image: ghcr.io/hmb-research/open-accounting:latest
    command: ["./migrate", "-db", "${DATABASE_URL}", "-path", "migrations", "-direction", "up"]
    depends_on:
      - db

volumes:
  postgres_data:
```

### 2. Create Environment File

```bash
# .env.prod
DATABASE_URL=postgres://openaccounting:SECURE_PASSWORD@db:5432/openaccounting?sslmode=disable
APP_ENV=production
JWT_SECRET=your-production-jwt-secret-minimum-32-characters
ALLOWED_ORIGINS=https://your-domain.com

DB_USER=openaccounting
DB_PASSWORD=SECURE_PASSWORD
DB_NAME=openaccounting
BACKUP_RETENTION_DAYS=30
```

### 3. Deploy

```bash
# Pull latest images
docker-compose -f docker-compose.prod.yml pull

# Run migrations
docker-compose -f docker-compose.prod.yml run --rm migrate

# Start services
docker-compose -f docker-compose.prod.yml up -d

# Check logs
docker-compose -f docker-compose.prod.yml logs -f api
```

## Nginx Reverse Proxy

Example Nginx configuration with SSL:

```nginx
server {
    listen 80;
    server_name api.example.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name api.example.com;

    ssl_certificate /etc/letsencrypt/live/api.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/api.example.com/privkey.pem;

    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;

    location / {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;
    }
}
```

## Database Considerations

### PostgreSQL Configuration

For production, tune these settings in `postgresql.conf`:

```ini
# Memory
shared_buffers = 256MB          # 25% of RAM
effective_cache_size = 768MB    # 75% of RAM
work_mem = 16MB

# Connections
max_connections = 100

# WAL
wal_level = replica
max_wal_senders = 3

# Logging
log_statement = 'mod'
log_min_duration_statement = 1000
```

### Backup and Restore Drills

Use PostgreSQL custom-format backups so restore drills can use `pg_restore` with explicit ownership and privilege handling:

```bash
DATABASE_URL="postgres://user:pass@host:5432/openaccounting?sslmode=require" \
  scripts/db-backup.sh --backup-dir /backups --retention-days 30
```

The same script can be run through the Go CLI when the repository scripts are available on the host:

```bash
DATABASE_URL="postgres://user:pass@host:5432/openaccounting?sslmode=require" \
  oa ops backup create --backup-dir /backups --retention-days 30
```

Set `OA_SCRIPT_DIR=/opt/open-accounting/scripts` when the `oa` binary is installed separately from the repository checkout.

The same CLI wrapper exposes offline preflight checks for offsite sync and restore drills without contacting object storage providers or running PostgreSQL restore commands:

```bash
AWS_PROFILE=open-accounting-backups \
  oa ops backup offsite-sync \
    --backup-dir /backups \
    --s3-uri s3://company-backups/open-accounting/prod \
    --preflight

oa ops backup restore-drill \
  --backup /backups/openaccounting_20260528T120000Z.dump \
  --restore-url "postgres://user:pass@localhost:5432/openaccounting_restore_drill?sslmode=disable" \
  --source-url "postgres://user:pass@host:5432/openaccounting?sslmode=require" \
  --preflight
```

The script writes `openaccounting_<utc>.dump` plus a `.sha256` checksum file. The production Docker Compose file also runs the same pattern daily and keeps generated backups for `BACKUP_RETENTION_DAYS` days, defaulting to 30.
Run backup and restore commands with PostgreSQL client tools from the same major version as the server, or use the production Compose backup service image.

Run a restore drill into a disposable database, never the source database:

```bash
createdb openaccounting_restore_drill

RESTORE_DATABASE_URL="postgres://user:pass@localhost:5432/openaccounting_restore_drill?sslmode=disable" \
  DATABASE_URL="postgres://user:pass@host:5432/openaccounting?sslmode=require" \
  scripts/db-restore-drill.sh \
    --backup /backups/openaccounting_20260528T120000Z.dump \
    --status-file /var/lib/node_exporter/textfile_collector/openaccounting_restore_drill.prom

dropdb openaccounting_restore_drill
```

`db-restore-drill.sh` refuses to run when the restore URL matches `DATABASE_URL`, checks the checksum when present, requires an empty target database unless `--allow-non-empty` is passed, restores with `pg_restore`, and verifies core Open Accounting tables plus applied migrations. Pass `--status-file` to publish Prometheus textfile metrics for restore-drill health, last success time, restored migration/user/tenant counts, and the structured failure code when a scheduled drill fails. Use `--preflight` to check the configured backup file, placeholder-free source/restore URLs, and source-vs-drill separation without running `psql` or `pg_restore`.

Monitor backup freshness and checksum status with the health script. It exits non-zero on missing, stale, undersized, or checksum-failing backups and can emit Prometheus textfile metrics:

```bash
scripts/db-backup-health.sh \
  --backup-dir /backups \
  --max-age-hours 26 \
  --status-file /var/lib/node_exporter/textfile_collector/openaccounting_backup.prom

oa ops backup health \
  --backup-dir /backups \
  --max-age-hours 26 \
  --status-file /var/lib/node_exporter/textfile_collector/openaccounting_backup.prom
```

Alert when `open_accounting_backup_health` is `0` or when `open_accounting_backup_latest_age_seconds` exceeds the expected schedule plus a grace period.

Sync completed dumps and checksum files to offsite storage after the local backup finishes. Use S3-compatible storage when the deployment already has AWS credentials configured:

```bash
BACKUP_OFFSITE_S3_URI="s3://company-backups/open-accounting/prod" \
  scripts/db-backup-offsite-sync.sh --backup-dir /backups
```

Use rclone for providers such as Backblaze B2, Wasabi, SFTP, or another S3-compatible target managed outside the app stack:

```bash
BACKUP_OFFSITE_RCLONE_REMOTE="b2:company-backups/open-accounting/prod" \
  scripts/db-backup-offsite-sync.sh --backup-dir /backups
```

`db-backup-offsite-sync.sh` copies `openaccounting_*.dump` files and matching `.sha256` files, refuses ambiguous destination configuration, and never deletes remote objects. Keep provider credentials outside the repository in the host secret manager, platform variables, or rclone config. Run `scripts/db-backup-offsite-sync.sh --preflight` on the backup host to validate destination and auth environment without scanning backups or calling S3/rclone.

Recommended production schedule:

```text
02:00  scripts/db-backup.sh --backup-dir /backups --retention-days 30
02:20  scripts/db-backup-offsite-sync.sh --backup-dir /backups
02:30  scripts/db-backup-health.sh --backup-dir /backups --max-age-hours 26 --status-file /var/lib/node_exporter/textfile_collector/openaccounting_backup.prom
Weekly scripts/db-restore-drill.sh --backup <newest-synced-dump> --status-file /var/lib/node_exporter/textfile_collector/openaccounting_restore_drill.prom
```

Run `db-restore-drill.sh` from a scheduled job at least weekly against the latest offsite-restored backup copy, not only the local backup directory.

For systemd hosts, generate service and timer templates for the same chain:

```bash
scripts/db-backup-systemd-schedule.sh \
  --output-dir ./deploy/systemd \
  --scripts-dir /opt/open-accounting/scripts \
  --backup-dir /backups \
  --status-dir /var/lib/node_exporter/textfile_collector \
  --env-file /etc/open-accounting/backup.env \
  --offsite-provider s3 \
  --systemd-unit-dir /etc/systemd/system \
  --dry-run
```

Remove `--dry-run` after reviewing the paths. The generator writes an `open-accounting-backup.env.example` tailored to `--offsite-provider s3|rclone`, an `open-accounting-backup-preflight.sh` helper, and an executable `open-accounting-backup-install.sh` helper. The install helper preserves any existing environment file, runs preflight, reloads systemd, and enables the four timers only after preflight passes.

Use a non-secret rehearsal before touching live credentials. This validates the host wiring, placeholder detection, local script paths, restore drill backup path, and offsite auth configuration without connecting to PostgreSQL or object storage:

```bash
tmpdir="$(mktemp -d)"
mkdir -p "$tmpdir/backups" "$tmpdir/status"
printf 'scratch backup\n' > "$tmpdir/backups/openaccounting_latest.dump"
printf '[preflight]\ntype = s3\nprovider = Other\n' > "$tmpdir/rclone.conf"
cat > "$tmpdir/backup.env" <<EOF
DATABASE_URL=postgres://backup_preflight:local_only@db.internal:5432/openaccounting?sslmode=require
RESTORE_DATABASE_URL=postgres://restore_preflight:local_only@localhost:5432/openaccounting_restore_drill?sslmode=disable
RESTORE_DRILL_BACKUP_FILE=$tmpdir/backups/openaccounting_latest.dump
BACKUP_OFFSITE_RCLONE_REMOTE=preflight:open-accounting/prod
RCLONE_CONFIG=$tmpdir/rclone.conf
EOF

scripts/db-backup-systemd-schedule.sh \
  --output-dir "$tmpdir/systemd" \
  --scripts-dir "$PWD/scripts" \
  --backup-dir "$tmpdir/backups" \
  --status-dir "$tmpdir/status" \
  --env-file "$tmpdir/backup.env" \
  --offsite-provider rclone
"$tmpdir/systemd/open-accounting-backup-preflight.sh" "$tmpdir/backup.env"
```

For production, install host values at `/etc/open-accounting/backup.env` through the secret manager or a root-readable file outside the repository, then run `./deploy/systemd/open-accounting-backup-preflight.sh /etc/open-accounting/backup.env` before `./deploy/systemd/open-accounting-backup-install.sh /etc/systemd/system`. Preflight rejects missing values and generated placeholders such as `replace-me`, `user:pass@db.example.com`, and `company-backups`; it does not prove that live credentials can connect, so keep the first timer run and restore drill monitored.

### Connection Pooling

For high-traffic deployments, use PgBouncer:

```ini
# pgbouncer.ini
[databases]
openaccounting = host=db port=5432 dbname=openaccounting

[pgbouncer]
listen_addr = 0.0.0.0
listen_port = 6432
auth_type = md5
pool_mode = transaction
max_client_conn = 1000
default_pool_size = 20
```

## Kubernetes Deployment

Basic Kubernetes manifests:

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: open-accounting-api
spec:
  replicas: 3
  selector:
    matchLabels:
      app: open-accounting-api
  template:
    metadata:
      labels:
        app: open-accounting-api
    spec:
      containers:
      - name: api
        image: ghcr.io/hmb-research/open-accounting:latest
        ports:
        - containerPort: 8080
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: open-accounting-secrets
              key: database-url
        - name: APP_ENV
          value: production
        - name: JWT_SECRET
          valueFrom:
            secretKeyRef:
              name: open-accounting-secrets
              key: jwt-secret
        resources:
          requests:
            memory: "256Mi"
            cpu: "200m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: open-accounting-api
spec:
  selector:
    app: open-accounting-api
  ports:
  - port: 80
    targetPort: 8080
  type: ClusterIP
```

## Railway Deployment

Railway provides a simple PaaS deployment option. The project includes Railway configuration files.

### Services Setup

Deploy two separate services:

1. **Backend API** (`open-accounting-api`)
   - Root directory: `/` (uses Go backend)
   - Environment variables:
     ```
     DATABASE_URL=<from Railway PostgreSQL>
     APP_ENV=production
     JWT_SECRET=<generate secure 32+ char secret>
     ALLOWED_ORIGINS=https://your-frontend.up.railway.app
     ```

2. **Frontend** (`open-accounting`)
   - Root directory: `/frontend`
   - Environment variables:
     ```
     PUBLIC_API_URL=https://your-api.up.railway.app
     ```

3. **PostgreSQL Database**
   - Add PostgreSQL plugin from Railway dashboard
   - Copy `DATABASE_URL` to API service

### Demo Mode (Optional)

For demo deployments with sample data and hourly reset:

```
DEMO_MODE=true
DEMO_RESET_SECRET=<your-secret-key>
```

Trigger reset via: `POST /api/demo/reset` with `X-Demo-Secret` header.

#### Multi-User Demo Setup

The demo environment supports 4 parallel users for E2E testing:

| User | Email | Password | Tenant |
|------|-------|----------|--------|
| Demo 1 | demo1@example.com | demo12345 | demo1 |
| Demo 2 | demo2@example.com | demo12345 | demo2 |
| Demo 3 | demo3@example.com | demo12345 | demo3 |
| Demo 4 | demo4@example.com | demo12345 | demo4 |

Each demo user has isolated data in separate PostgreSQL schemas (`tenant_demo1`, `tenant_demo2`, `tenant_demo3`, `tenant_demo4`), enabling parallel E2E test execution without data conflicts.

The demo reset endpoint (`POST /api/demo/reset`) resets all 4 demo tenants simultaneously.

## CORS Troubleshooting

If you encounter CORS errors like:

```
Access to fetch at 'https://api.example.com/...' has been blocked by CORS policy:
No 'Access-Control-Allow-Origin' header is present on the requested resource.
```

### Common Causes & Solutions

1. **Missing ALLOWED_ORIGINS**
   - Ensure `ALLOWED_ORIGINS` includes your frontend URL
   - Example: `ALLOWED_ORIGINS=https://app.example.com`

2. **Multiple Origins**
   - Use comma-separated values (no spaces around commas)
   - Example: `ALLOWED_ORIGINS=https://app.example.com,https://staging.example.com`

3. **Protocol Mismatch**
   - Ensure both URLs use `https://`
   - `http://` and `https://` are treated as different origins

4. **Trailing Slashes**
   - Don't include trailing slashes in origins
   - Correct: `https://app.example.com`
   - Wrong: `https://app.example.com/`

### Debugging

Enable verbose CORS logging:
```
CORS_DEBUG=true
```

Check API logs at startup for:
```
CORS configuration allowed_origins=["http://localhost:5173","https://app.example.com"]
```

### Verify Configuration

```bash
# Test preflight request
curl -X OPTIONS https://api.example.com/api/v1/auth/login \
  -H "Origin: https://app.example.com" \
  -H "Access-Control-Request-Method: POST" \
  -H "Access-Control-Request-Headers: Content-Type" \
  -v

# Should return headers including:
# Access-Control-Allow-Origin: https://app.example.com
# Access-Control-Allow-Methods: GET, POST, PUT, PATCH, DELETE, OPTIONS
```

## Security Checklist

- [ ] Use strong, unique `JWT_SECRET` (min 32 characters)
- [ ] Set `APP_ENV=production` so startup rejects insecure API defaults
- [ ] Enable SSL/TLS for all connections
- [ ] Use `sslmode=require` for database connections
- [ ] Configure firewall rules (only expose ports 80/443)
- [ ] Enable database connection encryption
- [ ] Set up automated backups and scheduled restore drills
- [ ] Configure log rotation
- [ ] Use secrets management (Vault, AWS Secrets Manager, etc.)
- [ ] Enable rate limiting at reverse proxy level
- [ ] Regular security updates for OS and dependencies

## Monitoring

### Health Check Endpoint

```
GET /health
```

Returns `200 OK` with body `OK` when healthy.

### Recommended Metrics

- Request latency (p50, p95, p99)
- Error rate (4xx, 5xx responses)
- Database connection pool usage
- Memory and CPU usage

### Logging

The API outputs structured logs. Recommended log aggregation:
- ELK Stack (Elasticsearch, Logstash, Kibana)
- Grafana Loki
- CloudWatch Logs (AWS)
