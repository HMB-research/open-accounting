-- Open Accounting Initial Schema Migration - DOWN
-- This drops all core tables and functions

-- Drop functions
DROP FUNCTION IF EXISTS drop_tenant_schema(TEXT);
DROP FUNCTION IF EXISTS create_default_chart_of_accounts(TEXT, UUID);
DROP FUNCTION IF EXISTS create_tenant_schema(TEXT);

-- Drop VAT rates
DROP TABLE IF EXISTS vat_rates;

-- Drop tenant management tables
DROP TABLE IF EXISTS tenant_users;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS tenants;

-- Drop UUID extension (only if no other objects depend on it)
-- DROP EXTENSION IF EXISTS "uuid-ossp";
