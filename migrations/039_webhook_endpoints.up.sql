CREATE TABLE IF NOT EXISTS webhook_endpoints (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    url TEXT NOT NULL,
    events TEXT[] NOT NULL DEFAULT '{}',
    secret TEXT NOT NULL DEFAULT '',
    is_active BOOLEAN NOT NULL DEFAULT true,
    last_delivery_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT webhook_endpoints_events_not_empty CHECK (COALESCE(array_length(events, 1), 0) > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_webhook_endpoints_tenant_name
    ON webhook_endpoints(tenant_id, lower(name));

CREATE INDEX IF NOT EXISTS idx_webhook_endpoints_tenant_active
    ON webhook_endpoints(tenant_id, is_active);

CREATE INDEX IF NOT EXISTS idx_webhook_endpoints_events
    ON webhook_endpoints USING GIN (events);

CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    endpoint_id UUID NOT NULL REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
    event_id TEXT NOT NULL,
    event_type VARCHAR(128) NOT NULL,
    status VARCHAR(16) NOT NULL,
    status_code INTEGER,
    attempt_number INTEGER NOT NULL DEFAULT 1,
    request_body JSONB NOT NULL DEFAULT '{}',
    response_body TEXT NOT NULL DEFAULT '',
    error TEXT NOT NULL DEFAULT '',
    delivered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT webhook_deliveries_status_check CHECK (status IN ('SUCCEEDED', 'FAILED')),
    CONSTRAINT webhook_deliveries_attempt_check CHECK (attempt_number > 0)
);

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_endpoint_created
    ON webhook_deliveries(endpoint_id, delivered_at DESC);

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_tenant_event
    ON webhook_deliveries(tenant_id, event_type, delivered_at DESC);
