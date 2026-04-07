CREATE TABLE IF NOT EXISTS tenant_period_closes (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    action VARCHAR(20) NOT NULL,
    close_kind VARCHAR(20) NOT NULL,
    period_end_date DATE NOT NULL,
    lock_date_before DATE,
    lock_date_after DATE,
    note TEXT,
    performed_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_tenant_period_closes_action CHECK (action IN ('close', 'reopen')),
    CONSTRAINT chk_tenant_period_closes_close_kind CHECK (close_kind IN ('month_end', 'year_end'))
);

CREATE INDEX IF NOT EXISTS idx_tenant_period_closes_tenant_created_at
    ON tenant_period_closes(tenant_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_tenant_period_closes_tenant_period
    ON tenant_period_closes(tenant_id, period_end_date, created_at DESC);
