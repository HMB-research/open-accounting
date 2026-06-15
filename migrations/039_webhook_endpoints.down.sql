DROP INDEX IF EXISTS idx_webhook_deliveries_tenant_event;
DROP INDEX IF EXISTS idx_webhook_deliveries_endpoint_created;
DROP TABLE IF EXISTS webhook_deliveries;

DROP INDEX IF EXISTS idx_webhook_endpoints_events;
DROP INDEX IF EXISTS idx_webhook_endpoints_tenant_active;
DROP INDEX IF EXISTS idx_webhook_endpoints_tenant_name;
DROP TABLE IF EXISTS webhook_endpoints;
