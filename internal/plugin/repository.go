package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

// PostgresRepository implements Repository for PostgreSQL
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// ListRegistries returns all plugin registries
func (r *PostgresRepository) ListRegistries(ctx context.Context) ([]Registry, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, url, description, is_official, is_active,
		       last_synced_at, created_at, updated_at
		FROM plugin_registries
		ORDER BY is_official DESC, name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list registries: %w", err)
	}
	defer rows.Close()

	var registries []Registry
	for rows.Next() {
		var reg Registry
		if err := rows.Scan(
			&reg.ID, &reg.Name, &reg.URL, &reg.Description, &reg.IsOfficial,
			&reg.IsActive, &reg.LastSyncedAt, &reg.CreatedAt, &reg.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan registry: %w", err)
		}
		registries = append(registries, reg)
	}
	return registries, nil
}

// GetRegistry returns a registry by ID
func (r *PostgresRepository) GetRegistry(ctx context.Context, id uuid.UUID) (*Registry, error) {
	var reg Registry
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, url, description, is_official, is_active,
		       last_synced_at, created_at, updated_at
		FROM plugin_registries
		WHERE id = $1
	`, id).Scan(
		&reg.ID, &reg.Name, &reg.URL, &reg.Description, &reg.IsOfficial,
		&reg.IsActive, &reg.LastSyncedAt, &reg.CreatedAt, &reg.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("registry not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get registry: %w", err)
	}
	return &reg, nil
}

// CreateRegistry creates a new registry
func (r *PostgresRepository) CreateRegistry(ctx context.Context, name, url, description string) (*Registry, error) {
	var reg Registry
	err := r.pool.QueryRow(ctx, `
		INSERT INTO plugin_registries (name, url, description)
		VALUES ($1, $2, $3)
		RETURNING id, name, url, description, is_official, is_active,
		          last_synced_at, created_at, updated_at
	`, name, url, description).Scan(
		&reg.ID, &reg.Name, &reg.URL, &reg.Description, &reg.IsOfficial,
		&reg.IsActive, &reg.LastSyncedAt, &reg.CreatedAt, &reg.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create registry: %w", err)
	}
	return &reg, nil
}

// DeleteRegistry deletes a non-official registry
func (r *PostgresRepository) DeleteRegistry(ctx context.Context, id uuid.UUID) (int64, error) {
	result, err := r.pool.Exec(ctx, `
		DELETE FROM plugin_registries
		WHERE id = $1 AND is_official = false
	`, id)
	if err != nil {
		return 0, fmt.Errorf("delete registry: %w", err)
	}
	return result.RowsAffected(), nil
}

// UpdateRegistryLastSynced updates the last synced timestamp
func (r *PostgresRepository) UpdateRegistryLastSynced(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE plugin_registries
		SET last_synced_at = now(), updated_at = now()
		WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("update registry last synced: %w", err)
	}
	return nil
}

// ListPlugins returns all plugins
func (r *PostgresRepository) ListPlugins(ctx context.Context) ([]Plugin, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, display_name, description, version, repository_url,
		       repository_type, author, license, homepage_url, state,
		       granted_permissions, manifest, installed_at, updated_at
		FROM plugins
		ORDER BY display_name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list plugins: %w", err)
	}
	defer rows.Close()

	var plugins []Plugin
	for rows.Next() {
		var p Plugin
		if err := rows.Scan(
			&p.ID, &p.Name, &p.DisplayName, &p.Description, &p.Version,
			&p.RepositoryURL, &p.RepositoryType, &p.Author, &p.License,
			&p.HomepageURL, &p.State, &p.GrantedPermissions, &p.Manifest,
			&p.InstalledAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan plugin: %w", err)
		}
		plugins = append(plugins, p)
	}
	return plugins, nil
}

// GetPlugin returns a plugin by ID
func (r *PostgresRepository) GetPlugin(ctx context.Context, id uuid.UUID) (*Plugin, error) {
	var p Plugin
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, display_name, description, version, repository_url,
		       repository_type, author, license, homepage_url, state,
		       granted_permissions, manifest, installed_at, updated_at
		FROM plugins
		WHERE id = $1
	`, id).Scan(
		&p.ID, &p.Name, &p.DisplayName, &p.Description, &p.Version,
		&p.RepositoryURL, &p.RepositoryType, &p.Author, &p.License,
		&p.HomepageURL, &p.State, &p.GrantedPermissions, &p.Manifest,
		&p.InstalledAt, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("plugin not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get plugin: %w", err)
	}
	return &p, nil
}

// GetPluginByName returns a plugin by name
func (r *PostgresRepository) GetPluginByName(ctx context.Context, name string) (*Plugin, error) {
	var p Plugin
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, display_name, description, version, repository_url,
		       repository_type, author, license, homepage_url, state,
		       granted_permissions, manifest, installed_at, updated_at
		FROM plugins
		WHERE name = $1
	`, name).Scan(
		&p.ID, &p.Name, &p.DisplayName, &p.Description, &p.Version,
		&p.RepositoryURL, &p.RepositoryType, &p.Author, &p.License,
		&p.HomepageURL, &p.State, &p.GrantedPermissions, &p.Manifest,
		&p.InstalledAt, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("plugin not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get plugin by name: %w", err)
	}
	return &p, nil
}

// CreatePlugin creates a new plugin
func (r *PostgresRepository) CreatePlugin(ctx context.Context, p *Plugin) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO plugins (id, name, display_name, description, version,
		                     repository_url, repository_type, author, license,
		                     homepage_url, state, granted_permissions, manifest,
		                     installed_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`, p.ID, p.Name, p.DisplayName, p.Description, p.Version,
		p.RepositoryURL, p.RepositoryType, p.Author, p.License,
		p.HomepageURL, p.State, p.GrantedPermissions, p.Manifest,
		p.InstalledAt, p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create plugin: %w", err)
	}
	return nil
}

// UpdatePlugin updates a plugin
func (r *PostgresRepository) UpdatePlugin(ctx context.Context, p *Plugin) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE plugins
		SET state = $2, granted_permissions = $3, updated_at = $4
		WHERE id = $1
	`, p.ID, p.State, p.GrantedPermissions, time.Now())
	if err != nil {
		return fmt.Errorf("update plugin: %w", err)
	}
	return nil
}

// DeletePlugin deletes a plugin
func (r *PostgresRepository) DeletePlugin(ctx context.Context, id uuid.UUID) (int64, error) {
	result, err := r.pool.Exec(ctx, `DELETE FROM plugins WHERE id = $1`, id)
	if err != nil {
		return 0, fmt.Errorf("delete plugin: %w", err)
	}
	return result.RowsAffected(), nil
}

// ListTenantPlugins returns all plugins enabled for a tenant
func (r *PostgresRepository) ListTenantPlugins(ctx context.Context, tenantID uuid.UUID) ([]TenantPlugin, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT tp.id, tp.tenant_id, tp.plugin_id, tp.is_enabled, tp.settings,
		       tp.enabled_at, tp.created_at, tp.updated_at,
		       p.id, p.name, p.display_name, p.description, p.version,
		       p.repository_url, p.repository_type, p.author, p.license,
		       p.homepage_url, p.state, p.granted_permissions, p.manifest,
		       p.installed_at, p.updated_at
		FROM tenant_plugins tp
		JOIN plugins p ON p.id = tp.plugin_id
		WHERE tp.tenant_id = $1
		ORDER BY p.display_name ASC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list tenant plugins: %w", err)
	}
	defer rows.Close()

	var plugins []TenantPlugin
	for rows.Next() {
		var tp TenantPlugin
		var p Plugin
		if err := rows.Scan(
			&tp.ID, &tp.TenantID, &tp.PluginID, &tp.IsEnabled, &tp.Settings,
			&tp.EnabledAt, &tp.CreatedAt, &tp.UpdatedAt,
			&p.ID, &p.Name, &p.DisplayName, &p.Description, &p.Version,
			&p.RepositoryURL, &p.RepositoryType, &p.Author, &p.License,
			&p.HomepageURL, &p.State, &p.GrantedPermissions, &p.Manifest,
			&p.InstalledAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan tenant plugin: %w", err)
		}
		tp.Plugin = &p
		plugins = append(plugins, tp)
	}
	return plugins, nil
}

// GetTenantPlugin returns a tenant plugin
func (r *PostgresRepository) GetTenantPlugin(ctx context.Context, tenantID, pluginID uuid.UUID) (*TenantPlugin, error) {
	var tp TenantPlugin
	var p Plugin
	err := r.pool.QueryRow(ctx, `
		SELECT tp.id, tp.tenant_id, tp.plugin_id, tp.is_enabled, tp.settings,
		       tp.enabled_at, tp.created_at, tp.updated_at,
		       p.id, p.name, p.display_name, p.description, p.version,
		       p.repository_url, p.repository_type, p.author, p.license,
		       p.homepage_url, p.state, p.granted_permissions, p.manifest,
		       p.installed_at, p.updated_at
		FROM tenant_plugins tp
		JOIN plugins p ON p.id = tp.plugin_id
		WHERE tp.tenant_id = $1 AND tp.plugin_id = $2
	`, tenantID, pluginID).Scan(
		&tp.ID, &tp.TenantID, &tp.PluginID, &tp.IsEnabled, &tp.Settings,
		&tp.EnabledAt, &tp.CreatedAt, &tp.UpdatedAt,
		&p.ID, &p.Name, &p.DisplayName, &p.Description, &p.Version,
		&p.RepositoryURL, &p.RepositoryType, &p.Author, &p.License,
		&p.HomepageURL, &p.State, &p.GrantedPermissions, &p.Manifest,
		&p.InstalledAt, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("tenant plugin not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get tenant plugin: %w", err)
	}
	tp.Plugin = &p
	return &tp, nil
}

// CreateTenantPlugin enables a plugin for a tenant
func (r *PostgresRepository) CreateTenantPlugin(ctx context.Context, tenantID, pluginID uuid.UUID, settings json.RawMessage) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO tenant_plugins (tenant_id, plugin_id, enabled_at, settings)
		VALUES ($1, $2, now(), $3)
	`, tenantID, pluginID, settings)
	if err != nil {
		return fmt.Errorf("create tenant plugin: %w", err)
	}
	return nil
}

// EnableTenantPlugin enables a plugin for a tenant (upsert)
func (r *PostgresRepository) EnableTenantPlugin(ctx context.Context, tenantID, pluginID uuid.UUID, settings json.RawMessage) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO tenant_plugins (tenant_id, plugin_id, is_enabled, settings, enabled_at)
		VALUES ($1, $2, true, $3, now())
		ON CONFLICT (tenant_id, plugin_id) DO UPDATE
		SET is_enabled = true, settings = $3, enabled_at = now(), updated_at = now()
	`, tenantID, pluginID, settings)
	if err != nil {
		return fmt.Errorf("enable tenant plugin: %w", err)
	}
	return nil
}

// DisableTenantPlugin disables a plugin for a tenant
func (r *PostgresRepository) DisableTenantPlugin(ctx context.Context, tenantID, pluginID uuid.UUID) (int64, error) {
	result, err := r.pool.Exec(ctx, `
		UPDATE tenant_plugins
		SET is_enabled = false, updated_at = now()
		WHERE tenant_id = $1 AND plugin_id = $2
	`, tenantID, pluginID)
	if err != nil {
		return 0, fmt.Errorf("disable tenant plugin: %w", err)
	}
	return result.RowsAffected(), nil
}

// GetTenantPluginSettings returns the settings for a tenant plugin
func (r *PostgresRepository) GetTenantPluginSettings(ctx context.Context, tenantID, pluginID uuid.UUID) (json.RawMessage, error) {
	var settings json.RawMessage
	err := r.pool.QueryRow(ctx, `
		SELECT settings FROM tenant_plugins
		WHERE tenant_id = $1 AND plugin_id = $2
	`, tenantID, pluginID).Scan(&settings)
	if err == pgx.ErrNoRows {
		return json.RawMessage("{}"), nil
	}
	if err != nil {
		return nil, fmt.Errorf("get tenant plugin settings: %w", err)
	}
	return settings, nil
}

// UpdateTenantPluginSettings updates tenant plugin settings
func (r *PostgresRepository) UpdateTenantPluginSettings(ctx context.Context, tenantID, pluginID uuid.UUID, settings json.RawMessage) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE tenant_plugins
		SET settings = $3, updated_at = now()
		WHERE tenant_id = $1 AND plugin_id = $2
	`, tenantID, pluginID, settings)
	if err != nil {
		return fmt.Errorf("update tenant plugin settings: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("tenant plugin not found")
	}
	return nil
}

// DeleteTenantPlugin removes a plugin from a tenant
func (r *PostgresRepository) DeleteTenantPlugin(ctx context.Context, tenantID, pluginID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		DELETE FROM tenant_plugins
		WHERE tenant_id = $1 AND plugin_id = $2
	`, tenantID, pluginID)
	if err != nil {
		return fmt.Errorf("delete tenant plugin: %w", err)
	}
	return nil
}

// IsPluginEnabledForTenant checks if a plugin is enabled for a tenant
func (r *PostgresRepository) IsPluginEnabledForTenant(ctx context.Context, tenantID, pluginID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM tenant_plugins
			WHERE tenant_id = $1 AND plugin_id = $2
		)
	`, tenantID, pluginID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check tenant plugin: %w", err)
	}
	return exists, nil
}

// ListEnabledPlugins returns all enabled plugins
func (r *PostgresRepository) ListEnabledPlugins(ctx context.Context) ([]Plugin, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, display_name, description, version, repository_url,
		       repository_type, author, license, homepage_url, state,
		       granted_permissions, manifest, installed_at, updated_at
		FROM plugins
		WHERE state = 'enabled'
		ORDER BY display_name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list enabled plugins: %w", err)
	}
	defer rows.Close()

	var plugins []Plugin
	for rows.Next() {
		var p Plugin
		if err := rows.Scan(
			&p.ID, &p.Name, &p.DisplayName, &p.Description, &p.Version,
			&p.RepositoryURL, &p.RepositoryType, &p.Author, &p.License,
			&p.HomepageURL, &p.State, &p.GrantedPermissions, &p.Manifest,
			&p.InstalledAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan enabled plugin: %w", err)
		}
		plugins = append(plugins, p)
	}
	return plugins, nil
}

// InsertPluginReturning inserts a plugin and returns the created record
func (r *PostgresRepository) InsertPluginReturning(ctx context.Context, manifest *Manifest, repoURL string, repoType RepositoryType, manifestJSON []byte) (*Plugin, error) {
	var p Plugin
	err := r.pool.QueryRow(ctx, `
		INSERT INTO plugins (
			name, display_name, description, version, repository_url,
			repository_type, author, license, homepage_url, state,
			granted_permissions, manifest
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, name, display_name, description, version, repository_url,
		          repository_type, author, license, homepage_url, state,
		          granted_permissions, manifest, installed_at, updated_at
	`,
		manifest.Name, manifest.DisplayName, manifest.Description, manifest.Version,
		repoURL, repoType, manifest.Author, manifest.License, manifest.Homepage,
		StateInstalled, []string{}, manifestJSON,
	).Scan(
		&p.ID, &p.Name, &p.DisplayName, &p.Description, &p.Version,
		&p.RepositoryURL, &p.RepositoryType, &p.Author, &p.License,
		&p.HomepageURL, &p.State, &p.GrantedPermissions, &p.Manifest,
		&p.InstalledAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert plugin: %w", err)
	}
	return &p, nil
}

// CountEnabledTenantsForPlugin counts tenants that have the plugin enabled
func (r *PostgresRepository) CountEnabledTenantsForPlugin(ctx context.Context, pluginID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM tenant_plugins
		WHERE plugin_id = $1 AND is_enabled = true
	`, pluginID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count enabled tenants: %w", err)
	}
	return count, nil
}

// UpdatePluginState updates a plugin's state and permissions
func (r *PostgresRepository) UpdatePluginState(ctx context.Context, pluginID uuid.UUID, state PluginState, permissions []string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE plugins
		SET state = $1, granted_permissions = $2, updated_at = now()
		WHERE id = $3
	`, state, permissions, pluginID)
	if err != nil {
		return fmt.Errorf("update plugin state: %w", err)
	}
	return nil
}

// DisableAllTenantsForPlugin disables the plugin for all tenants
func (r *PostgresRepository) DisableAllTenantsForPlugin(ctx context.Context, pluginID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE tenant_plugins
		SET is_enabled = false, updated_at = now()
		WHERE plugin_id = $1
	`, pluginID)
	if err != nil {
		return fmt.Errorf("disable plugin for all tenants: %w", err)
	}
	return nil
}

// GetTenantPluginsWithAll returns all plugins available to a tenant (enabled or not)
func (r *PostgresRepository) GetTenantPluginsWithAll(ctx context.Context, tenantID uuid.UUID) ([]TenantPlugin, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT tp.id, tp.tenant_id, tp.plugin_id, tp.is_enabled, tp.settings,
		       tp.enabled_at, tp.created_at, tp.updated_at,
		       p.id, p.name, p.display_name, p.description, p.version,
		       p.repository_url, p.repository_type, p.author, p.license,
		       p.homepage_url, p.state, p.granted_permissions, p.manifest,
		       p.installed_at, p.updated_at
		FROM plugins p
		LEFT JOIN tenant_plugins tp ON tp.plugin_id = p.id AND tp.tenant_id = $1
		WHERE p.state = 'enabled'
		ORDER BY p.display_name ASC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list tenant plugins: %w", err)
	}
	defer rows.Close()

	var results []TenantPlugin
	for rows.Next() {
		var tp TenantPlugin
		var p Plugin
		var tpID, tpTenantID, tpPluginID *uuid.UUID
		var tpIsEnabled *bool
		var tpSettings json.RawMessage
		var tpEnabledAt, tpCreatedAt, tpUpdatedAt *time.Time

		err := rows.Scan(
			&tpID, &tpTenantID, &tpPluginID, &tpIsEnabled, &tpSettings,
			&tpEnabledAt, &tpCreatedAt, &tpUpdatedAt,
			&p.ID, &p.Name, &p.DisplayName, &p.Description, &p.Version,
			&p.RepositoryURL, &p.RepositoryType, &p.Author, &p.License,
			&p.HomepageURL, &p.State, &p.GrantedPermissions, &p.Manifest,
			&p.InstalledAt, &p.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan tenant plugin: %w", err)
		}

		tp.Plugin = &p
		tp.PluginID = p.ID
		tp.TenantID = tenantID

		if tpID != nil {
			tp.ID = *tpID
			tp.IsEnabled = *tpIsEnabled
			tp.Settings = tpSettings
			tp.EnabledAt = tpEnabledAt
			tp.CreatedAt = *tpCreatedAt
			tp.UpdatedAt = *tpUpdatedAt
		}

		results = append(results, tp)
	}

	return results, nil
}
