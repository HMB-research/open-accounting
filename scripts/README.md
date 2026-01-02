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

For public demos, enable automatic hourly resets:

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
| `JWT_SECRET` | Yes | Secret for JWT signing (min 32 chars) |
| `PORT` | No | Server port (default: 8080) |
| `ALLOWED_ORIGINS` | No | CORS origins (comma-separated) |
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
