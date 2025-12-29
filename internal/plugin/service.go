package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// Service handles plugin lifecycle management
type Service struct {
	pool  *pgxpool.Pool
	hooks *HookRegistry
	mu    sync.RWMutex

	// Loaded plugins (in-memory cache)
	plugins map[string]*LoadedPlugin

	// Plugin directory for installed plugins
	pluginDir string
}

// LoadedPlugin represents a plugin loaded into memory
type LoadedPlugin struct {
	Plugin   *Plugin
	Manifest *Manifest
}

// NewService creates a new plugin service
func NewService(pool *pgxpool.Pool, pluginDir string) *Service {
	return &Service{
		pool:      pool,
		hooks:     NewHookRegistry(),
		plugins:   make(map[string]*LoadedPlugin),
		pluginDir: pluginDir,
	}
}

// GetHookRegistry returns the hook registry for registering hooks
func (s *Service) GetHookRegistry() *HookRegistry {
	return s.hooks
}

// Registry Management

// ListRegistries returns all plugin registries
func (s *Service) ListRegistries(ctx context.Context) ([]Registry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, url, description, is_official, is_active,
		       last_synced_at, created_at, updated_at
		FROM plugin_registries
		ORDER BY is_official DESC, name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list registries: %w", err)
	}
	defer rows.Close()

	var registries []Registry
	for rows.Next() {
		var r Registry
		err := rows.Scan(
			&r.ID, &r.Name, &r.URL, &r.Description, &r.IsOfficial,
			&r.IsActive, &r.LastSyncedAt, &r.CreatedAt, &r.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan registry: %w", err)
		}
		registries = append(registries, r)
	}

	return registries, nil
}

// GetRegistry returns a registry by ID
func (s *Service) GetRegistry(ctx context.Context, id uuid.UUID) (*Registry, error) {
	var r Registry
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, url, description, is_official, is_active,
		       last_synced_at, created_at, updated_at
		FROM plugin_registries
		WHERE id = $1
	`, id).Scan(
		&r.ID, &r.Name, &r.URL, &r.Description, &r.IsOfficial,
		&r.IsActive, &r.LastSyncedAt, &r.CreatedAt, &r.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("registry not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get registry: %w", err)
	}
	return &r, nil
}

// AddRegistry adds a new plugin registry
func (s *Service) AddRegistry(ctx context.Context, req CreateRegistryRequest) (*Registry, error) {
	// Validate URL format
	if !isValidRegistryURL(req.URL) {
		return nil, fmt.Errorf("invalid registry URL: must be a GitHub or GitLab repository")
	}

	var r Registry
	err := s.pool.QueryRow(ctx, `
		INSERT INTO plugin_registries (name, url, description)
		VALUES ($1, $2, $3)
		RETURNING id, name, url, description, is_official, is_active,
		          last_synced_at, created_at, updated_at
	`, req.Name, req.URL, req.Description).Scan(
		&r.ID, &r.Name, &r.URL, &r.Description, &r.IsOfficial,
		&r.IsActive, &r.LastSyncedAt, &r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to add registry: %w", err)
	}

	log.Info().Str("registry", r.Name).Str("url", r.URL).Msg("Added plugin registry")
	return &r, nil
}

// RemoveRegistry removes a plugin registry
func (s *Service) RemoveRegistry(ctx context.Context, id uuid.UUID) error {
	result, err := s.pool.Exec(ctx, `
		DELETE FROM plugin_registries
		WHERE id = $1 AND is_official = false
	`, id)
	if err != nil {
		return fmt.Errorf("failed to remove registry: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("registry not found or is official (cannot be removed)")
	}

	log.Info().Str("id", id.String()).Msg("Removed plugin registry")
	return nil
}

// UpdateRegistryLastSynced updates the last synced timestamp
func (s *Service) UpdateRegistryLastSynced(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE plugin_registries
		SET last_synced_at = now(), updated_at = now()
		WHERE id = $1
	`, id)
	return err
}

// Plugin Management

// ListPlugins returns all installed plugins
func (s *Service) ListPlugins(ctx context.Context) ([]Plugin, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, display_name, description, version, repository_url,
		       repository_type, author, license, homepage_url, state,
		       granted_permissions, manifest, installed_at, updated_at
		FROM plugins
		ORDER BY display_name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list plugins: %w", err)
	}
	defer rows.Close()

	var plugins []Plugin
	for rows.Next() {
		var p Plugin
		err := rows.Scan(
			&p.ID, &p.Name, &p.DisplayName, &p.Description, &p.Version,
			&p.RepositoryURL, &p.RepositoryType, &p.Author, &p.License,
			&p.HomepageURL, &p.State, &p.GrantedPermissions, &p.Manifest,
			&p.InstalledAt, &p.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan plugin: %w", err)
		}
		plugins = append(plugins, p)
	}

	return plugins, nil
}

// GetPlugin returns a plugin by ID
func (s *Service) GetPlugin(ctx context.Context, id uuid.UUID) (*Plugin, error) {
	var p Plugin
	err := s.pool.QueryRow(ctx, `
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
		return nil, fmt.Errorf("failed to get plugin: %w", err)
	}
	return &p, nil
}

// GetPluginByName returns a plugin by name
func (s *Service) GetPluginByName(ctx context.Context, name string) (*Plugin, error) {
	var p Plugin
	err := s.pool.QueryRow(ctx, `
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
		return nil, fmt.Errorf("failed to get plugin: %w", err)
	}
	return &p, nil
}

// InstallPlugin installs a plugin from a repository URL
func (s *Service) InstallPlugin(ctx context.Context, repoURL string) (*Plugin, error) {
	// Validate repository URL
	repoType, err := parseRepositoryType(repoURL)
	if err != nil {
		return nil, err
	}

	// Clone the repository
	pluginPath, err := s.cloneRepository(ctx, repoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	// Load and validate manifest
	manifest, err := LoadManifest(pluginPath + "/plugin.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	if err := manifest.Validate(); err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}

	// Check if already installed
	existing, _ := s.GetPluginByName(ctx, manifest.Name)
	if existing != nil {
		return nil, fmt.Errorf("plugin '%s' is already installed", manifest.Name)
	}

	// Convert manifest to JSON
	manifestJSON, err := manifest.ToJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize manifest: %w", err)
	}

	// Insert plugin record
	var p Plugin
	err = s.pool.QueryRow(ctx, `
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
		return nil, fmt.Errorf("failed to insert plugin: %w", err)
	}

	log.Info().
		Str("plugin", p.Name).
		Str("version", p.Version).
		Msg("Installed plugin")

	return &p, nil
}

// UninstallPlugin removes a plugin
func (s *Service) UninstallPlugin(ctx context.Context, id uuid.UUID) error {
	// Get plugin first
	plugin, err := s.GetPlugin(ctx, id)
	if err != nil {
		return err
	}

	// Check if any tenants have it enabled
	var count int
	err = s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM tenant_plugins
		WHERE plugin_id = $1 AND is_enabled = true
	`, id).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check tenant usage: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("cannot uninstall: plugin is enabled for %d tenant(s)", count)
	}

	// Unload from memory
	s.unloadPlugin(plugin.Name)

	// Delete from database (cascades to tenant_plugins and plugin_migrations)
	_, err = s.pool.Exec(ctx, `DELETE FROM plugins WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete plugin: %w", err)
	}

	// Remove plugin files
	if err := s.removePluginFiles(plugin.Name); err != nil {
		log.Warn().Err(err).Str("plugin", plugin.Name).Msg("Failed to remove plugin files")
	}

	log.Info().Str("plugin", plugin.Name).Msg("Uninstalled plugin")
	return nil
}

// EnablePlugin enables a plugin with granted permissions
func (s *Service) EnablePlugin(ctx context.Context, id uuid.UUID, permissions []string) error {
	// Get plugin
	plugin, err := s.GetPlugin(ctx, id)
	if err != nil {
		return err
	}

	if plugin.State == StateEnabled {
		return fmt.Errorf("plugin is already enabled")
	}

	// Validate permissions
	if invalid := ValidatePermissions(permissions); len(invalid) > 0 {
		return fmt.Errorf("invalid permissions: %v", invalid)
	}

	// Parse manifest
	var manifest Manifest
	if err := json.Unmarshal(plugin.Manifest, &manifest); err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}

	// Check if all required permissions are granted
	required := manifest.RequiredPermissions()
	for _, req := range required {
		if !containsPermission(permissions, req) {
			return fmt.Errorf("missing required permission: %s", req)
		}
	}

	// Update plugin state
	_, err = s.pool.Exec(ctx, `
		UPDATE plugins
		SET state = $1, granted_permissions = $2, updated_at = now()
		WHERE id = $3
	`, StateEnabled, permissions, id)
	if err != nil {
		return fmt.Errorf("failed to update plugin state: %w", err)
	}

	// Load plugin into memory
	plugin.State = StateEnabled
	plugin.GrantedPermissions = permissions
	if err := s.loadPlugin(plugin, &manifest); err != nil {
		log.Error().Err(err).Str("plugin", plugin.Name).Msg("Failed to load plugin")
		// Revert state
		_, _ = s.pool.Exec(ctx, `
			UPDATE plugins SET state = $1, updated_at = now() WHERE id = $2
		`, StateFailed, id)
		return fmt.Errorf("failed to load plugin: %w", err)
	}

	log.Info().
		Str("plugin", plugin.Name).
		Strs("permissions", permissions).
		Msg("Enabled plugin")

	return nil
}

// DisablePlugin disables a plugin
func (s *Service) DisablePlugin(ctx context.Context, id uuid.UUID) error {
	plugin, err := s.GetPlugin(ctx, id)
	if err != nil {
		return err
	}

	if plugin.State == StateDisabled {
		return fmt.Errorf("plugin is already disabled")
	}

	// Unload from memory
	s.unloadPlugin(plugin.Name)

	// Update state
	_, err = s.pool.Exec(ctx, `
		UPDATE plugins
		SET state = $1, updated_at = now()
		WHERE id = $2
	`, StateDisabled, id)
	if err != nil {
		return fmt.Errorf("failed to update plugin state: %w", err)
	}

	// Disable for all tenants
	_, err = s.pool.Exec(ctx, `
		UPDATE tenant_plugins
		SET is_enabled = false, updated_at = now()
		WHERE plugin_id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("failed to disable for tenants: %w", err)
	}

	log.Info().Str("plugin", plugin.Name).Msg("Disabled plugin")
	return nil
}

// Tenant Plugin Management

// GetTenantPlugins returns all plugins available to a tenant
func (s *Service) GetTenantPlugins(ctx context.Context, tenantID uuid.UUID) ([]TenantPlugin, error) {
	rows, err := s.pool.Query(ctx, `
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
		return nil, fmt.Errorf("failed to list tenant plugins: %w", err)
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
			return nil, fmt.Errorf("failed to scan tenant plugin: %w", err)
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

// EnableForTenant enables a plugin for a specific tenant
func (s *Service) EnableForTenant(ctx context.Context, tenantID, pluginID uuid.UUID, settings json.RawMessage) error {
	// Check plugin exists and is enabled instance-wide
	plugin, err := s.GetPlugin(ctx, pluginID)
	if err != nil {
		return err
	}
	if plugin.State != StateEnabled {
		return fmt.Errorf("plugin is not enabled at instance level")
	}

	// Upsert tenant_plugins record
	_, err = s.pool.Exec(ctx, `
		INSERT INTO tenant_plugins (tenant_id, plugin_id, is_enabled, settings, enabled_at)
		VALUES ($1, $2, true, $3, now())
		ON CONFLICT (tenant_id, plugin_id) DO UPDATE
		SET is_enabled = true, settings = $3, enabled_at = now(), updated_at = now()
	`, tenantID, pluginID, settings)
	if err != nil {
		return fmt.Errorf("failed to enable plugin for tenant: %w", err)
	}

	log.Info().
		Str("plugin", plugin.Name).
		Str("tenant", tenantID.String()).
		Msg("Enabled plugin for tenant")

	return nil
}

// DisableForTenant disables a plugin for a specific tenant
func (s *Service) DisableForTenant(ctx context.Context, tenantID, pluginID uuid.UUID) error {
	result, err := s.pool.Exec(ctx, `
		UPDATE tenant_plugins
		SET is_enabled = false, updated_at = now()
		WHERE tenant_id = $1 AND plugin_id = $2
	`, tenantID, pluginID)
	if err != nil {
		return fmt.Errorf("failed to disable plugin for tenant: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("plugin not found for tenant")
	}

	log.Info().
		Str("plugin_id", pluginID.String()).
		Str("tenant", tenantID.String()).
		Msg("Disabled plugin for tenant")

	return nil
}

// GetTenantPluginSettings returns the settings for a plugin for a tenant
func (s *Service) GetTenantPluginSettings(ctx context.Context, tenantID, pluginID uuid.UUID) (json.RawMessage, error) {
	var settings json.RawMessage
	err := s.pool.QueryRow(ctx, `
		SELECT settings FROM tenant_plugins
		WHERE tenant_id = $1 AND plugin_id = $2
	`, tenantID, pluginID).Scan(&settings)
	if err == pgx.ErrNoRows {
		return json.RawMessage("{}"), nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get settings: %w", err)
	}
	return settings, nil
}

// UpdateTenantPluginSettings updates the settings for a plugin for a tenant
func (s *Service) UpdateTenantPluginSettings(ctx context.Context, tenantID, pluginID uuid.UUID, settings json.RawMessage) error {
	result, err := s.pool.Exec(ctx, `
		UPDATE tenant_plugins
		SET settings = $3, updated_at = now()
		WHERE tenant_id = $1 AND plugin_id = $2
	`, tenantID, pluginID, settings)
	if err != nil {
		return fmt.Errorf("failed to update settings: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("plugin not found for tenant")
	}

	return nil
}

// IsPluginEnabledForTenant checks if a plugin is enabled for a tenant
func (s *Service) IsPluginEnabledForTenant(ctx context.Context, tenantID, pluginID uuid.UUID) (bool, error) {
	var enabled bool
	err := s.pool.QueryRow(ctx, `
		SELECT is_enabled FROM tenant_plugins
		WHERE tenant_id = $1 AND plugin_id = $2
	`, tenantID, pluginID).Scan(&enabled)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return enabled, nil
}

// Internal methods

func (s *Service) loadPlugin(plugin *Plugin, manifest *Manifest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Store in memory
	s.plugins[plugin.Name] = &LoadedPlugin{
		Plugin:   plugin,
		Manifest: manifest,
	}

	// Register hooks if any
	if manifest.Backend != nil {
		for _, hook := range manifest.Backend.Hooks {
			s.hooks.registerPluginHook(plugin.ID, hook.Event, hook.Handler)
		}
	}

	return nil
}

func (s *Service) unloadPlugin(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	loaded, exists := s.plugins[name]
	if !exists {
		return
	}

	// Unregister hooks
	s.hooks.unregisterPluginHooks(loaded.Plugin.ID)

	delete(s.plugins, name)
}

// GetLoadedPlugin returns a loaded plugin by name
func (s *Service) GetLoadedPlugin(name string) (*LoadedPlugin, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, exists := s.plugins[name]
	return p, exists
}

// LoadEnabledPlugins loads all enabled plugins into memory on startup
func (s *Service) LoadEnabledPlugins(ctx context.Context) error {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, display_name, description, version, repository_url,
		       repository_type, author, license, homepage_url, state,
		       granted_permissions, manifest, installed_at, updated_at
		FROM plugins
		WHERE state = 'enabled'
	`)
	if err != nil {
		return fmt.Errorf("failed to list enabled plugins: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var p Plugin
		err := rows.Scan(
			&p.ID, &p.Name, &p.DisplayName, &p.Description, &p.Version,
			&p.RepositoryURL, &p.RepositoryType, &p.Author, &p.License,
			&p.HomepageURL, &p.State, &p.GrantedPermissions, &p.Manifest,
			&p.InstalledAt, &p.UpdatedAt,
		)
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan enabled plugin")
			continue
		}

		var manifest Manifest
		if err := json.Unmarshal(p.Manifest, &manifest); err != nil {
			log.Error().Err(err).Str("plugin", p.Name).Msg("Failed to parse manifest")
			continue
		}

		if err := s.loadPlugin(&p, &manifest); err != nil {
			log.Error().Err(err).Str("plugin", p.Name).Msg("Failed to load plugin")
		} else {
			log.Info().Str("plugin", p.Name).Msg("Loaded plugin")
		}
	}

	return nil
}
