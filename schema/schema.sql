-- Open Accounting Schema
-- Multi-tenant accounting system with schema-per-tenant isolation

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- =============================================================================
-- PUBLIC SCHEMA: Shared tables across all tenants
-- =============================================================================

-- Tenants (Organizations)
CREATE TABLE tenants (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) NOT NULL UNIQUE,
    schema_name VARCHAR(100) NOT NULL UNIQUE,
    settings JSONB NOT NULL DEFAULT '{}',
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Users
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Tenant-User membership
CREATE TABLE tenant_users (
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL DEFAULT 'user',
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, user_id)
);

-- VAT Rates (Shared reference data)
CREATE TABLE vat_rates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    country_code CHAR(2) NOT NULL,
    rate_type VARCHAR(50) NOT NULL,
    rate NUMERIC(5,2) NOT NULL,
    name VARCHAR(100) NOT NULL,
    valid_from DATE NOT NULL,
    valid_to DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (country_code, rate_type, valid_from)
);

CREATE INDEX idx_vat_rates_lookup ON vat_rates(country_code, valid_from, valid_to);

-- =============================================================================
-- TENANT SCHEMA TEMPLATE
-- These tables are created in each tenant's schema
-- =============================================================================

-- Note: The following tables are created dynamically per tenant using
-- the create_tenant_schema() function. They serve as the template/reference.

-- Chart of Accounts
-- CREATE TABLE accounts (...)

-- Fiscal Years
-- CREATE TABLE fiscal_years (...)

-- Journal Entries (Double-entry bookkeeping)
-- CREATE TABLE journal_entries (...)
-- CREATE TABLE journal_entry_lines (...)

-- Contacts (Customers/Suppliers)
-- CREATE TABLE contacts (...)

-- Invoices
-- CREATE TABLE invoices (...)
-- CREATE TABLE invoice_lines (...)

-- Payments
-- CREATE TABLE payments (...)
-- CREATE TABLE payment_allocations (...)

-- Products & Inventory
-- CREATE TABLE products (...)
-- CREATE TABLE inventory_lots (...)
-- CREATE TABLE inventory_movements (...)

-- Payroll
-- CREATE TABLE employees (...)
-- CREATE TABLE employee_contracts (...)
-- CREATE TABLE payroll_runs (...)
-- CREATE TABLE payroll_entries (...)

-- Banking
-- CREATE TABLE bank_accounts (...)
-- CREATE TABLE bank_transactions (...)

-- Audit
-- CREATE TABLE audit_log (...)

-- E-Invoice
-- CREATE TABLE einvoice_log (...)
