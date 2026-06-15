-- Global security audit events for auth-sensitive account actions.

CREATE TABLE IF NOT EXISTS security_audit_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    actor_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    actor_email VARCHAR(255),
    action VARCHAR(64) NOT NULL,
    target_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    target_email VARCHAR(255),
    request_ip TEXT,
    user_agent TEXT,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_security_audit_events_actor_created
    ON security_audit_events(actor_user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_security_audit_events_target_created
    ON security_audit_events(target_user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_security_audit_events_action
    ON security_audit_events(action);
