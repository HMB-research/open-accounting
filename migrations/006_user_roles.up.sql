-- Migration: Add proper user roles with constraints
-- Roles: owner, admin, accountant, viewer

-- Add role constraint to tenant_users
ALTER TABLE tenant_users
DROP CONSTRAINT IF EXISTS tenant_users_role_check;

ALTER TABLE tenant_users
ADD CONSTRAINT tenant_users_role_check
CHECK (role IN ('owner', 'admin', 'accountant', 'viewer'));

-- Update any existing 'user' roles to 'accountant'
UPDATE tenant_users SET role = 'accountant' WHERE role = 'user';

-- Create role permissions reference table
CREATE TABLE IF NOT EXISTS role_permissions (
    role VARCHAR(50) PRIMARY KEY,
    description TEXT NOT NULL,
    can_manage_users BOOLEAN DEFAULT false,
    can_manage_settings BOOLEAN DEFAULT false,
    can_manage_accounts BOOLEAN DEFAULT false,
    can_create_entries BOOLEAN DEFAULT false,
    can_approve_entries BOOLEAN DEFAULT false,
    can_view_reports BOOLEAN DEFAULT false,
    can_manage_invoices BOOLEAN DEFAULT false,
    can_manage_payments BOOLEAN DEFAULT false,
    can_manage_contacts BOOLEAN DEFAULT false,
    can_manage_banking BOOLEAN DEFAULT false,
    can_export_data BOOLEAN DEFAULT false
);

-- Insert role definitions
INSERT INTO role_permissions (role, description, can_manage_users, can_manage_settings, can_manage_accounts, can_create_entries, can_approve_entries, can_view_reports, can_manage_invoices, can_manage_payments, can_manage_contacts, can_manage_banking, can_export_data)
VALUES
    ('owner', 'Full access - can manage everything including deleting the organization', true, true, true, true, true, true, true, true, true, true, true),
    ('admin', 'Administrative access - can manage users, settings, and all accounting functions', true, true, true, true, true, true, true, true, true, true, true),
    ('accountant', 'Accounting access - can manage all accounting functions but not users or settings', false, false, true, true, true, true, true, true, true, true, true),
    ('viewer', 'Read-only access - can view reports and data but cannot modify anything', false, false, false, false, false, true, false, false, false, false, false)
ON CONFLICT (role) DO UPDATE SET
    description = EXCLUDED.description,
    can_manage_users = EXCLUDED.can_manage_users,
    can_manage_settings = EXCLUDED.can_manage_settings,
    can_manage_accounts = EXCLUDED.can_manage_accounts,
    can_create_entries = EXCLUDED.can_create_entries,
    can_approve_entries = EXCLUDED.can_approve_entries,
    can_view_reports = EXCLUDED.can_view_reports,
    can_manage_invoices = EXCLUDED.can_manage_invoices,
    can_manage_payments = EXCLUDED.can_manage_payments,
    can_manage_contacts = EXCLUDED.can_manage_contacts,
    can_manage_banking = EXCLUDED.can_manage_banking,
    can_export_data = EXCLUDED.can_export_data;

-- Add invited_by column to track who invited the user
ALTER TABLE tenant_users
ADD COLUMN IF NOT EXISTS invited_by UUID REFERENCES users(id),
ADD COLUMN IF NOT EXISTS invited_at TIMESTAMPTZ;

-- Create user invitations table for pending invitations
CREATE TABLE IF NOT EXISTS user_invitations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL CHECK (role IN ('admin', 'accountant', 'viewer')),
    invited_by UUID NOT NULL REFERENCES users(id),
    token VARCHAR(255) NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (tenant_id, email)
);

CREATE INDEX IF NOT EXISTS idx_invitations_token ON user_invitations(token) WHERE accepted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_invitations_email ON user_invitations(email);
