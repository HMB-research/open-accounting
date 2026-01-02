#!/bin/bash
# Demo Reset Script
# Resets the demo database to initial state
# Run via cron: 0 * * * * /app/scripts/demo-reset.sh

set -e

echo "$(date): Starting demo reset..."

# Check if DEMO_MODE is enabled
if [ "$DEMO_MODE" != "true" ]; then
    echo "DEMO_MODE is not enabled. Skipping reset."
    exit 0
fi

# Check for DATABASE_URL
if [ -z "$DATABASE_URL" ]; then
    echo "ERROR: DATABASE_URL is not set"
    exit 1
fi

# Drop and recreate the demo tenant schema
echo "Dropping demo tenant schema..."
psql "$DATABASE_URL" -c "DROP SCHEMA IF EXISTS tenant_acme CASCADE;" 2>/dev/null || true

# Delete demo data from public tables
echo "Cleaning demo data from public tables..."
psql "$DATABASE_URL" <<EOF
DELETE FROM tenant_users WHERE tenant_id = 'b0000000-0000-0000-0000-000000000001';
DELETE FROM tenants WHERE id = 'b0000000-0000-0000-0000-000000000001';
DELETE FROM users WHERE id = 'a0000000-0000-0000-0000-000000000001';
EOF

# Run seed script
echo "Re-seeding demo data..."
psql "$DATABASE_URL" -f /app/scripts/demo-seed.sql

echo "$(date): Demo reset completed successfully!"
