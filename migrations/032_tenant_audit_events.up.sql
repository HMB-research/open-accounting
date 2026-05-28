CREATE TABLE IF NOT EXISTS tenant_audit_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    actor_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(64) NOT NULL,
    target_type VARCHAR(64) NOT NULL,
    target_id TEXT NOT NULL,
    target_email TEXT,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tenant_audit_events_tenant_created
    ON tenant_audit_events(tenant_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_tenant_audit_events_action
    ON tenant_audit_events(action);
