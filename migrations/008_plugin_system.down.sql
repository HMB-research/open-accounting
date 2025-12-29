-- Rollback Plugin System Migration

-- Drop indexes
DROP INDEX IF EXISTS idx_plugin_registries_active;
DROP INDEX IF EXISTS idx_plugin_migrations_plugin_id;
DROP INDEX IF EXISTS idx_tenant_plugins_enabled;
DROP INDEX IF EXISTS idx_tenant_plugins_plugin_id;
DROP INDEX IF EXISTS idx_tenant_plugins_tenant_id;
DROP INDEX IF EXISTS idx_plugins_name;
DROP INDEX IF EXISTS idx_plugins_state;

-- Drop tables in reverse order (respecting foreign keys)
DROP TABLE IF EXISTS plugin_migrations;
DROP TABLE IF EXISTS tenant_plugins;
DROP TABLE IF EXISTS plugins;
DROP TABLE IF EXISTS plugin_registries;
