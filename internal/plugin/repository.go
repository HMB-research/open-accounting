package plugin

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

// Repository defines the interface for plugin data access
type Repository interface {
	// Registry operations
	ListRegistries(ctx context.Context) ([]Registry, error)
	GetRegistry(ctx context.Context, id uuid.UUID) (*Registry, error)
	CreateRegistry(ctx context.Context, name, url, description string) (*Registry, error)
	DeleteRegistry(ctx context.Context, id uuid.UUID) (int64, error)
	UpdateRegistryLastSynced(ctx context.Context, id uuid.UUID) error

	// Plugin operations
	ListPlugins(ctx context.Context) ([]Plugin, error)
	GetPlugin(ctx context.Context, id uuid.UUID) (*Plugin, error)
	GetPluginByName(ctx context.Context, name string) (*Plugin, error)
	CreatePlugin(ctx context.Context, p *Plugin) error
	UpdatePlugin(ctx context.Context, p *Plugin) error
	DeletePlugin(ctx context.Context, id uuid.UUID) (int64, error)

	// Tenant plugin operations
	ListTenantPlugins(ctx context.Context, tenantID uuid.UUID) ([]TenantPlugin, error)
	GetTenantPlugin(ctx context.Context, tenantID, pluginID uuid.UUID) (*TenantPlugin, error)
	CreateTenantPlugin(ctx context.Context, tenantID, pluginID uuid.UUID, settings json.RawMessage) error
	EnableTenantPlugin(ctx context.Context, tenantID, pluginID uuid.UUID, settings json.RawMessage) error
	DisableTenantPlugin(ctx context.Context, tenantID, pluginID uuid.UUID) (int64, error)
	GetTenantPluginSettings(ctx context.Context, tenantID, pluginID uuid.UUID) (json.RawMessage, error)
	UpdateTenantPluginSettings(ctx context.Context, tenantID, pluginID uuid.UUID, settings json.RawMessage) error
	DeleteTenantPlugin(ctx context.Context, tenantID, pluginID uuid.UUID) error
	IsPluginEnabledForTenant(ctx context.Context, tenantID, pluginID uuid.UUID) (bool, error)

	// Enabled plugins query
	ListEnabledPlugins(ctx context.Context) ([]Plugin, error)

	// Additional operations for service refactoring
	InsertPluginReturning(ctx context.Context, manifest *Manifest, repoURL string, repoType RepositoryType, manifestJSON []byte) (*Plugin, error)
	CountEnabledTenantsForPlugin(ctx context.Context, pluginID uuid.UUID) (int, error)
	UpdatePluginState(ctx context.Context, pluginID uuid.UUID, state PluginState, permissions []string) error
	DisableAllTenantsForPlugin(ctx context.Context, pluginID uuid.UUID) error
	GetTenantPluginsWithAll(ctx context.Context, tenantID uuid.UUID) ([]TenantPlugin, error)
}
