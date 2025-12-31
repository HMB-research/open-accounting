package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// Service handles plugin lifecycle management
type Service struct {
	pool  *pgxpool.Pool
	repo  Repository
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
		repo:      NewPostgresRepository(pool),
		hooks:     NewHookRegistry(),
		plugins:   make(map[string]*LoadedPlugin),
		pluginDir: pluginDir,
	}
}

// NewServiceWithRepository creates a new plugin service with a custom repository (for testing)
func NewServiceWithRepository(repo Repository, hooks *HookRegistry, pluginDir string) *Service {
	if hooks == nil {
		hooks = NewHookRegistry()
	}
	return &Service{
		repo:      repo,
		hooks:     hooks,
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
	return s.repo.ListRegistries(ctx)
}

// GetRegistry returns a registry by ID
func (s *Service) GetRegistry(ctx context.Context, id uuid.UUID) (*Registry, error) {
	return s.repo.GetRegistry(ctx, id)
}

// AddRegistry adds a new plugin registry
func (s *Service) AddRegistry(ctx context.Context, req CreateRegistryRequest) (*Registry, error) {
	// Validate URL format
	if !isValidRegistryURL(req.URL) {
		return nil, fmt.Errorf("invalid registry URL: must be a GitHub or GitLab repository")
	}

	r, err := s.repo.CreateRegistry(ctx, req.Name, req.URL, req.Description)
	if err != nil {
		return nil, fmt.Errorf("failed to add registry: %w", err)
	}

	log.Info().Str("registry", r.Name).Str("url", r.URL).Msg("Added plugin registry")
	return r, nil
}

// RemoveRegistry removes a plugin registry
func (s *Service) RemoveRegistry(ctx context.Context, id uuid.UUID) error {
	affected, err := s.repo.DeleteRegistry(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to remove registry: %w", err)
	}

	if affected == 0 {
		return fmt.Errorf("registry not found or is official (cannot be removed)")
	}

	log.Info().Str("id", id.String()).Msg("Removed plugin registry")
	return nil
}

// UpdateRegistryLastSynced updates the last synced timestamp
func (s *Service) UpdateRegistryLastSynced(ctx context.Context, id uuid.UUID) error {
	return s.repo.UpdateRegistryLastSynced(ctx, id)
}

// Plugin Management

// ListPlugins returns all installed plugins
func (s *Service) ListPlugins(ctx context.Context) ([]Plugin, error) {
	return s.repo.ListPlugins(ctx)
}

// GetPlugin returns a plugin by ID
func (s *Service) GetPlugin(ctx context.Context, id uuid.UUID) (*Plugin, error) {
	return s.repo.GetPlugin(ctx, id)
}

// GetPluginByName returns a plugin by name
func (s *Service) GetPluginByName(ctx context.Context, name string) (*Plugin, error) {
	return s.repo.GetPluginByName(ctx, name)
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

	// Insert plugin record via repository
	p, err := s.repo.InsertPluginReturning(ctx, manifest, repoURL, repoType, manifestJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to insert plugin: %w", err)
	}

	log.Info().
		Str("plugin", p.Name).
		Str("version", p.Version).
		Msg("Installed plugin")

	return p, nil
}

// UninstallPlugin removes a plugin
func (s *Service) UninstallPlugin(ctx context.Context, id uuid.UUID) error {
	// Get plugin first
	plugin, err := s.GetPlugin(ctx, id)
	if err != nil {
		return err
	}

	// Check if any tenants have it enabled
	count, err := s.repo.CountEnabledTenantsForPlugin(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to check tenant usage: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("cannot uninstall: plugin is enabled for %d tenant(s)", count)
	}

	// Unload from memory
	s.unloadPlugin(plugin.Name)

	// Delete from database (cascades to tenant_plugins and plugin_migrations)
	_, err = s.repo.DeletePlugin(ctx, id)
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

	// Update plugin state via repository
	if err := s.repo.UpdatePluginState(ctx, id, StateEnabled, permissions); err != nil {
		return fmt.Errorf("failed to update plugin state: %w", err)
	}

	// Load plugin into memory
	plugin.State = StateEnabled
	plugin.GrantedPermissions = permissions
	if err := s.loadPlugin(plugin, &manifest); err != nil {
		log.Error().Err(err).Str("plugin", plugin.Name).Msg("Failed to load plugin")
		// Revert state
		_ = s.repo.UpdatePluginState(ctx, id, StateFailed, nil)
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

	// Update state via repository
	if err := s.repo.UpdatePluginState(ctx, id, StateDisabled, nil); err != nil {
		return fmt.Errorf("failed to update plugin state: %w", err)
	}

	// Disable for all tenants via repository
	if err := s.repo.DisableAllTenantsForPlugin(ctx, id); err != nil {
		return fmt.Errorf("failed to disable for tenants: %w", err)
	}

	log.Info().Str("plugin", plugin.Name).Msg("Disabled plugin")
	return nil
}

// Tenant Plugin Management

// GetTenantPlugins returns all plugins available to a tenant
func (s *Service) GetTenantPlugins(ctx context.Context, tenantID uuid.UUID) ([]TenantPlugin, error) {
	return s.repo.GetTenantPluginsWithAll(ctx, tenantID)
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
	if err := s.repo.EnableTenantPlugin(ctx, tenantID, pluginID, settings); err != nil {
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
	affected, err := s.repo.DisableTenantPlugin(ctx, tenantID, pluginID)
	if err != nil {
		return fmt.Errorf("failed to disable plugin for tenant: %w", err)
	}

	if affected == 0 {
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
	return s.repo.GetTenantPluginSettings(ctx, tenantID, pluginID)
}

// UpdateTenantPluginSettings updates the settings for a plugin for a tenant
func (s *Service) UpdateTenantPluginSettings(ctx context.Context, tenantID, pluginID uuid.UUID, settings json.RawMessage) error {
	return s.repo.UpdateTenantPluginSettings(ctx, tenantID, pluginID, settings)
}

// IsPluginEnabledForTenant checks if a plugin is enabled for a tenant
func (s *Service) IsPluginEnabledForTenant(ctx context.Context, tenantID, pluginID uuid.UUID) (bool, error) {
	return s.repo.IsPluginEnabledForTenant(ctx, tenantID, pluginID)
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
	plugins, err := s.repo.ListEnabledPlugins(ctx)
	if err != nil {
		return fmt.Errorf("failed to list enabled plugins: %w", err)
	}

	for i := range plugins {
		p := &plugins[i]

		var manifest Manifest
		if err := json.Unmarshal(p.Manifest, &manifest); err != nil {
			log.Error().Err(err).Str("plugin", p.Name).Msg("Failed to parse manifest")
			continue
		}

		if err := s.loadPlugin(p, &manifest); err != nil {
			log.Error().Err(err).Str("plugin", p.Name).Msg("Failed to load plugin")
		} else {
			log.Info().Str("plugin", p.Name).Msg("Loaded plugin")
		}
	}

	return nil
}
