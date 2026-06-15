package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// Service handles plugin lifecycle management
type Service struct {
	repo  Repository
	hooks *HookRegistry
	mu    sync.RWMutex

	// Loaded plugins (in-memory cache)
	plugins         map[string]*LoadedPlugin
	runtimeFailures map[uuid.UUID]PluginRuntimeStatus

	// Plugin directory for installed plugins
	pluginDir string
}

// LoadedPlugin represents a plugin loaded into memory
type LoadedPlugin struct {
	Plugin   *Plugin
	Manifest *Manifest
	Runtime  pluginBackendRuntime
}

// NewService creates a new plugin service with an ORM-backed repository.
func NewService(pool *pgxpool.Pool, pluginDir string) *Service {
	if pool == nil {
		return &Service{
			hooks:           NewHookRegistry(),
			plugins:         make(map[string]*LoadedPlugin),
			runtimeFailures: make(map[uuid.UUID]PluginRuntimeStatus),
			pluginDir:       pluginDir,
		}
	}
	gormDB, err := database.NewGormDBFromPool(context.Background(), pool)
	if err != nil {
		panic(fmt.Errorf("create plugin GORM repository: %w", err))
	}
	return &Service{
		repo:            NewGORMRepository(gormDB),
		hooks:           NewHookRegistry(),
		plugins:         make(map[string]*LoadedPlugin),
		runtimeFailures: make(map[uuid.UUID]PluginRuntimeStatus),
		pluginDir:       pluginDir,
	}
}

// NewServiceWithRepository creates a new plugin service with a custom repository (for testing)
func NewServiceWithRepository(repo Repository, hooks *HookRegistry, pluginDir string) *Service {
	if hooks == nil {
		hooks = NewHookRegistry()
	}
	return &Service{
		repo:            repo,
		hooks:           hooks,
		plugins:         make(map[string]*LoadedPlugin),
		runtimeFailures: make(map[uuid.UUID]PluginRuntimeStatus),
		pluginDir:       pluginDir,
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
	s.clearRuntimeFailure(plugin.ID)

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
	plugins, err := s.repo.GetTenantPluginsWithAll(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if plugins == nil {
		return []TenantPlugin{}, nil
	}
	return plugins, nil
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

// GetPluginRuntimeStatus returns the operator-visible backend runtime status for an installed plugin.
func (s *Service) GetPluginRuntimeStatus(ctx context.Context, pluginID uuid.UUID) (*PluginRuntimeStatus, error) {
	plugin, err := s.GetPlugin(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	manifest, err := parsePluginManifest(plugin)
	if err != nil {
		return nil, err
	}
	status := s.runtimeStatusForPlugin(plugin, manifest)
	return &status, nil
}

// RestartPluginRuntime manually restarts a supervised package runtime.
func (s *Service) RestartPluginRuntime(ctx context.Context, pluginID uuid.UUID) (*PluginRuntimeStatus, error) {
	plugin, err := s.GetPlugin(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	if plugin.State != StateEnabled {
		return nil, fmt.Errorf("%w: plugin %q is not enabled", ErrPluginNotEnabled, plugin.Name)
	}
	manifest, err := parsePluginManifest(plugin)
	if err != nil {
		return nil, err
	}
	if manifest.Backend == nil || normalizedBackendRuntime(manifest.Backend.Runtime) != BackendRuntimePackage {
		return nil, fmt.Errorf("%w: plugin %q does not use a supervised package runtime", ErrPluginRuntimeUnsupported, plugin.Name)
	}

	current := s.runtimeStatusForPlugin(plugin, manifest)
	stats := packageRuntimeStats{
		RestartCount: current.RestartCount + 1,
		CrashCount:   current.CrashCount,
	}
	if err := s.loadPluginWithRuntimeStats(plugin, manifest, stats); err != nil {
		return nil, err
	}
	return s.GetPluginRuntimeStatus(ctx, pluginID)
}

// Internal methods

func (s *Service) loadPlugin(plugin *Plugin, manifest *Manifest) error {
	return s.loadPluginWithRuntimeStats(plugin, manifest, packageRuntimeStats{})
}

func (s *Service) loadPluginWithRuntimeStats(plugin *Plugin, manifest *Manifest, stats packageRuntimeStats) error {
	runtime, err := s.backendRuntimeForPluginWithStats(plugin, manifest, stats)
	if err != nil {
		s.recordRuntimeFailure(plugin, manifest, err, stats)
		return err
	}

	s.mu.Lock()
	previous := s.plugins[plugin.Name]
	s.plugins[plugin.Name] = &LoadedPlugin{
		Plugin:   plugin,
		Manifest: manifest,
		Runtime:  runtime,
	}
	delete(s.runtimeFailures, plugin.ID)
	s.mu.Unlock()

	if previous != nil {
		s.hooks.unregisterPluginHooks(previous.Plugin.ID)
		if previous.Runtime != nil {
			ctx, cancel := context.WithTimeout(context.Background(), packageRuntimeShutdownTimeout)
			if err := previous.Runtime.close(ctx); err != nil {
				log.Warn().Err(err).Str("plugin", previous.Plugin.Name).Msg("Failed to stop previous plugin runtime")
			}
			cancel()
		}
	}

	// Register hooks if any
	if manifest.Backend != nil {
		for _, hook := range manifest.Backend.Hooks {
			if runtime == nil {
				s.hooks.registerPluginHook(plugin.ID, hook.Event, hook.Handler)
				continue
			}
			hook := hook
			s.hooks.registerPluginHookHandler(plugin.ID, hook.Event, hook.Handler, func(ctx context.Context, event Event) error {
				return runtime.invokeHook(ctx, plugin.ID, plugin.Name, hook, event)
			})
		}
	}

	return nil
}

func (s *Service) backendRuntimeForPlugin(plugin *Plugin, manifest *Manifest) (pluginBackendRuntime, error) {
	return s.backendRuntimeForPluginWithStats(plugin, manifest, packageRuntimeStats{})
}

func (s *Service) backendRuntimeForPluginWithStats(plugin *Plugin, manifest *Manifest, stats packageRuntimeStats) (pluginBackendRuntime, error) {
	if manifest.Backend == nil {
		return nil, nil
	}

	hasHooks := len(manifest.Backend.Hooks) > 0
	hasRoutes := len(manifest.Backend.Routes) > 0
	if !hasHooks && !hasRoutes {
		return nil, nil
	}

	if strings.EqualFold(strings.TrimSpace(manifest.Backend.Runtime), BackendRuntimePackage) {
		return s.startPackageBackendRuntimeWithStats(plugin, manifest, stats)
	}

	runtime, err := newRuntimeHTTPClient(manifest.Backend)
	if err == nil {
		return runtime, nil
	}
	if !errors.Is(err, ErrPluginRuntimeUnavailable) {
		return nil, err
	}
	switch {
	case hasHooks && hasRoutes:
		return nil, fmt.Errorf("backend hooks and routes are declared but plugin backend runtime execution is not implemented")
	case hasHooks:
		return nil, fmt.Errorf("backend hooks are declared but plugin backend runtime execution is not implemented")
	case hasRoutes:
		return nil, fmt.Errorf("backend routes are declared but plugin backend runtime execution is not implemented")
	default:
		return nil, nil
	}
}

func (s *Service) InvokeTenantPluginRoute(
	ctx context.Context,
	tenantID uuid.UUID,
	pluginID uuid.UUID,
	method string,
	path string,
	rawQuery string,
	headers http.Header,
	body io.Reader,
) (*RuntimeRouteResponse, error) {
	enabled, err := s.IsPluginEnabledForTenant(ctx, tenantID, pluginID)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, ErrPluginNotEnabled
	}

	plugin, err := s.GetPlugin(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	if plugin.State != StateEnabled {
		return nil, ErrPluginNotEnabled
	}

	var manifest Manifest
	if err := json.Unmarshal(plugin.Manifest, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}
	route, ok := findRuntimeRoute(&manifest, method, normalizeRuntimePath(path))
	if !ok {
		return nil, ErrPluginRouteNotFound
	}
	runtime, err := s.runtimeForTenantPluginRoute(plugin, &manifest)
	if err != nil {
		return nil, err
	}
	if runtime == nil {
		return nil, ErrPluginRuntimeUnavailable
	}
	if body == nil {
		body = strings.NewReader("")
	}
	return runtime.invokeRoute(ctx, plugin.ID, tenantID, route, method, normalizeRuntimePath(path), rawQuery, headers, body)
}

func (s *Service) runtimeForTenantPluginRoute(plugin *Plugin, manifest *Manifest) (pluginBackendRuntime, error) {
	if manifest.Backend == nil {
		return nil, nil
	}
	if strings.EqualFold(strings.TrimSpace(manifest.Backend.Runtime), BackendRuntimePackage) {
		s.mu.RLock()
		loaded, ok := s.plugins[plugin.Name]
		s.mu.RUnlock()
		if !ok || loaded.Runtime == nil {
			return nil, fmt.Errorf("%w: package runtime for plugin %q is not loaded", ErrPluginRuntimeUnavailable, plugin.Name)
		}
		return loaded.Runtime, nil
	}
	return s.backendRuntimeForPlugin(plugin, manifest)
}

func parsePluginManifest(plugin *Plugin) (*Manifest, error) {
	if plugin == nil {
		return nil, fmt.Errorf("plugin is nil")
	}
	var manifest Manifest
	if err := json.Unmarshal(plugin.Manifest, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}
	return &manifest, nil
}

func (s *Service) runtimeStatusForPlugin(plugin *Plugin, manifest *Manifest) PluginRuntimeStatus {
	base := baseRuntimeStatus(plugin, manifest)
	if plugin == nil || manifest == nil || manifest.Backend == nil || !manifestDeclaresRuntimeWork(manifest) {
		return base
	}

	switch normalizedBackendRuntime(manifest.Backend.Runtime) {
	case BackendRuntimePackage:
		s.mu.RLock()
		loaded := s.plugins[plugin.Name]
		failure, hasFailure := s.runtimeFailures[plugin.ID]
		s.mu.RUnlock()
		if loaded != nil && loaded.Runtime != nil {
			return decorateRuntimeStatus(plugin, manifest, loaded.Runtime.status())
		}
		if hasFailure {
			return decorateRuntimeStatus(plugin, manifest, failure)
		}
		if plugin.State != StateEnabled {
			base.State = RuntimeStateNotLoaded
			base.Health = RuntimeHealthNotApplicable
			base.Message = "plugin is not enabled"
			if plugin.State == StateFailed {
				base.State = RuntimeStateFailed
				base.Health = RuntimeHealthUnhealthy
				base.Message = "plugin failed to load"
			}
			return base
		}
		base.State = RuntimeStateNotLoaded
		base.Health = RuntimeHealthUnknown
		base.Message = "package runtime is not loaded"
		return base
	case BackendRuntimeHTTP:
		s.mu.RLock()
		loaded := s.plugins[plugin.Name]
		s.mu.RUnlock()
		if loaded != nil && loaded.Runtime != nil {
			return decorateRuntimeStatus(plugin, manifest, loaded.Runtime.status())
		}
		base.State = RuntimeStateExternal
		base.Health = RuntimeHealthUnknown
		base.Message = "external HTTP runtime is operator-managed"
		base.BaseURL = strings.TrimSpace(manifest.Backend.BaseURL)
		return base
	default:
		base.State = RuntimeStateNotConfigured
		base.Health = RuntimeHealthUnknown
		base.Message = "backend hooks or routes require backend.runtime"
		return base
	}
}

func (s *Service) recordRuntimeFailure(plugin *Plugin, manifest *Manifest, err error, stats packageRuntimeStats) {
	if plugin == nil || err == nil {
		return
	}
	status := baseRuntimeStatus(plugin, manifest)
	status.State = RuntimeStateFailed
	status.Health = RuntimeHealthUnhealthy
	status.Message = "runtime failed to start"
	status.RestartCount = stats.RestartCount
	status.CrashCount = stats.CrashCount
	status.LastError = err.Error()

	s.mu.Lock()
	if s.runtimeFailures == nil {
		s.runtimeFailures = make(map[uuid.UUID]PluginRuntimeStatus)
	}
	s.runtimeFailures[plugin.ID] = status
	s.mu.Unlock()
}

func baseRuntimeStatus(plugin *Plugin, manifest *Manifest) PluginRuntimeStatus {
	status := PluginRuntimeStatus{
		State:   RuntimeStateNotConfigured,
		Health:  RuntimeHealthNotApplicable,
		Runtime: "none",
		Message: "plugin does not declare backend runtime work",
	}
	if plugin != nil {
		status.PluginID = plugin.ID
		status.PluginName = plugin.Name
		status.DisplayName = plugin.DisplayName
	}
	if manifest == nil || manifest.Backend == nil {
		return status
	}

	status.HookCount = len(manifest.Backend.Hooks)
	status.RouteCount = len(manifest.Backend.Routes)
	runtime := normalizedBackendRuntime(manifest.Backend.Runtime)
	if runtime == "" && manifestDeclaresRuntimeWork(manifest) {
		runtime = "legacy"
	}
	if runtime != "" {
		status.Runtime = runtime
	}
	if !manifestDeclaresRuntimeWork(manifest) {
		return status
	}

	switch runtime {
	case BackendRuntimePackage:
		status.State = RuntimeStateNotLoaded
		status.Health = RuntimeHealthUnknown
		status.Message = "package runtime is not loaded"
	case BackendRuntimeHTTP:
		status.State = RuntimeStateExternal
		status.Health = RuntimeHealthUnknown
		status.Message = "external HTTP runtime is operator-managed"
		status.BaseURL = strings.TrimSpace(manifest.Backend.BaseURL)
	default:
		status.State = RuntimeStateNotConfigured
		status.Health = RuntimeHealthUnknown
		status.Message = "backend hooks or routes require backend.runtime"
	}
	return status
}

func decorateRuntimeStatus(plugin *Plugin, manifest *Manifest, status PluginRuntimeStatus) PluginRuntimeStatus {
	base := baseRuntimeStatus(plugin, manifest)
	if status.PluginID == uuid.Nil {
		status.PluginID = base.PluginID
	}
	if status.PluginName == "" {
		status.PluginName = base.PluginName
	}
	status.DisplayName = base.DisplayName
	if status.Runtime == "" || status.Runtime == "none" {
		status.Runtime = base.Runtime
	}
	status.HookCount = base.HookCount
	status.RouteCount = base.RouteCount
	return status
}

func manifestDeclaresRuntimeWork(manifest *Manifest) bool {
	return manifest != nil &&
		manifest.Backend != nil &&
		(len(manifest.Backend.Hooks) > 0 || len(manifest.Backend.Routes) > 0)
}

func findRuntimeRoute(manifest *Manifest, method string, path string) (RouteConfig, bool) {
	if manifest.Backend == nil {
		return RouteConfig{}, false
	}
	normalizedMethod := strings.ToUpper(strings.TrimSpace(method))
	normalizedPath := normalizeRuntimePath(path)
	for _, route := range manifest.Backend.Routes {
		if strings.ToUpper(strings.TrimSpace(route.Method)) == normalizedMethod &&
			normalizeRuntimePath(route.Path) == normalizedPath {
			return route, true
		}
	}
	return RouteConfig{}, false
}

func normalizeRuntimePath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" || trimmed == "/" {
		return "/"
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	return trimmed
}

func (s *Service) unloadPlugin(name string) {
	s.mu.Lock()
	loaded, exists := s.plugins[name]
	if !exists {
		s.mu.Unlock()
		return
	}
	delete(s.plugins, name)
	s.mu.Unlock()

	// Unregister hooks
	s.hooks.unregisterPluginHooks(loaded.Plugin.ID)
	s.clearRuntimeFailure(loaded.Plugin.ID)

	if loaded.Runtime != nil {
		ctx, cancel := context.WithTimeout(context.Background(), packageRuntimeShutdownTimeout)
		if err := loaded.Runtime.close(ctx); err != nil {
			log.Warn().Err(err).Str("plugin", loaded.Plugin.Name).Msg("Failed to stop plugin runtime")
		}
		cancel()
	}
}

func (s *Service) clearRuntimeFailure(pluginID uuid.UUID) {
	s.mu.Lock()
	if s.runtimeFailures != nil {
		delete(s.runtimeFailures, pluginID)
	}
	s.mu.Unlock()
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
