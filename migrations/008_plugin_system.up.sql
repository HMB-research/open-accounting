-- Plugin System Migration
-- Adds support for plugin marketplace, registries, and tenant-level plugin management

-- Plugin registries (marketplace sources)
CREATE TABLE plugin_registries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    url VARCHAR(500) NOT NULL UNIQUE,
    description TEXT,
    is_official BOOLEAN DEFAULT false,
    is_active BOOLEAN DEFAULT true,
    last_synced_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

-- Installed plugins (instance-wide)
CREATE TABLE plugins (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL UNIQUE,
    display_name VARCHAR(255) NOT NULL,
    description TEXT,
    version VARCHAR(50) NOT NULL,
    repository_url VARCHAR(500) NOT NULL,
    repository_type VARCHAR(20) DEFAULT 'github',
    author VARCHAR(255),
    license VARCHAR(50),
    homepage_url VARCHAR(500),
    state VARCHAR(20) NOT NULL DEFAULT 'installed',
    granted_permissions TEXT[] DEFAULT '{}',
    manifest JSONB NOT NULL,
    installed_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    CONSTRAINT valid_plugin_state CHECK (state IN ('installed', 'enabled', 'disabled', 'failed'))
);

-- Per-tenant plugin enablement
CREATE TABLE tenant_plugins (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    plugin_id UUID NOT NULL REFERENCES plugins(id) ON DELETE CASCADE,
    is_enabled BOOLEAN DEFAULT false,
    settings JSONB DEFAULT '{}',
    enabled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE (tenant_id, plugin_id)
);

-- Plugin migration tracking
CREATE TABLE plugin_migrations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plugin_id UUID NOT NULL REFERENCES plugins(id) ON DELETE CASCADE,
    version VARCHAR(50) NOT NULL,
    filename VARCHAR(255) NOT NULL,
    applied_at TIMESTAMPTZ DEFAULT now(),
    checksum VARCHAR(64),
    UNIQUE (plugin_id, filename)
);

-- Indexes for performance
CREATE INDEX idx_plugins_state ON plugins(state);
CREATE INDEX idx_plugins_name ON plugins(name);
CREATE INDEX idx_tenant_plugins_tenant_id ON tenant_plugins(tenant_id);
CREATE INDEX idx_tenant_plugins_plugin_id ON tenant_plugins(plugin_id);
CREATE INDEX idx_tenant_plugins_enabled ON tenant_plugins(tenant_id, is_enabled);
CREATE INDEX idx_plugin_migrations_plugin_id ON plugin_migrations(plugin_id);
CREATE INDEX idx_plugin_registries_active ON plugin_registries(is_active);

-- Insert official registry
INSERT INTO plugin_registries (name, url, description, is_official)
VALUES (
    'Open Accounting Official',
    'https://github.com/HMB-research/open-accounting-plugins',
    'Official plugin repository maintained by HMB Research',
    true
);
