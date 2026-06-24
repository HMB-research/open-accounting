package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GORMRepository implements Repository using GORM
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository creates a new GORM plugin repository
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

func (r *GORMRepository) dbWithContext(ctx context.Context) (*gorm.DB, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("plugin repository database is not configured")
	}
	return r.db.WithContext(ctx), nil
}

// ListRegistries returns all plugin registries
func (r *GORMRepository) ListRegistries(ctx context.Context) ([]Registry, error) {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return nil, err
	}

	var regModels []models.PluginRegistry
	if err := db.
		Order("is_official DESC, name ASC").
		Find(&regModels).Error; err != nil {
		return nil, fmt.Errorf("list registries: %w", err)
	}

	registries := make([]Registry, len(regModels))
	for i, rm := range regModels {
		registries[i] = modelToRegistry(&rm)
	}
	return registries, nil
}

// GetRegistry returns a registry by ID
func (r *GORMRepository) GetRegistry(ctx context.Context, id uuid.UUID) (*Registry, error) {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return nil, err
	}

	var regModel models.PluginRegistry
	if err := db.Where("id = ?", id).First(&regModel).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("registry not found")
		}
		return nil, fmt.Errorf("get registry: %w", err)
	}

	reg := modelToRegistry(&regModel)
	return &reg, nil
}

// CreateRegistry creates a new registry
func (r *GORMRepository) CreateRegistry(ctx context.Context, name, url, description string) (*Registry, error) {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return nil, err
	}

	regModel := &models.PluginRegistry{
		Name:        name,
		URL:         url,
		Description: description,
		IsOfficial:  false,
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := db.Create(regModel).Error; err != nil {
		return nil, fmt.Errorf("create registry: %w", err)
	}

	reg := modelToRegistry(regModel)
	return &reg, nil
}

// DeleteRegistry deletes a non-official registry
func (r *GORMRepository) DeleteRegistry(ctx context.Context, id uuid.UUID) (int64, error) {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return 0, err
	}

	result := db.
		Where("id = ? AND is_official = ?", id, false).
		Delete(&models.PluginRegistry{})
	if result.Error != nil {
		return 0, fmt.Errorf("delete registry: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// UpdateRegistryLastSynced updates the last synced timestamp
func (r *GORMRepository) UpdateRegistryLastSynced(ctx context.Context, id uuid.UUID) error {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return err
	}

	now := time.Now()
	if err := db.Model(&models.PluginRegistry{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"last_synced_at": now,
			"updated_at":     now,
		}).Error; err != nil {
		return fmt.Errorf("update registry last synced: %w", err)
	}
	return nil
}

// ListPlugins returns all plugins
func (r *GORMRepository) ListPlugins(ctx context.Context) ([]Plugin, error) {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return nil, err
	}

	var pluginModels []models.Plugin
	if err := db.
		Order("display_name ASC").
		Find(&pluginModels).Error; err != nil {
		return nil, fmt.Errorf("list plugins: %w", err)
	}

	plugins := make([]Plugin, len(pluginModels))
	for i, pm := range pluginModels {
		plugins[i] = modelToPlugin(&pm)
	}
	return plugins, nil
}

// GetPlugin returns a plugin by ID
func (r *GORMRepository) GetPlugin(ctx context.Context, id uuid.UUID) (*Plugin, error) {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return nil, err
	}

	var pluginModel models.Plugin
	if err := db.Where("id = ?", id).First(&pluginModel).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("plugin not found")
		}
		return nil, fmt.Errorf("get plugin: %w", err)
	}

	p := modelToPlugin(&pluginModel)
	return &p, nil
}

// GetPluginByName returns a plugin by name
func (r *GORMRepository) GetPluginByName(ctx context.Context, name string) (*Plugin, error) {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return nil, err
	}

	var pluginModel models.Plugin
	if err := db.Where("name = ?", name).First(&pluginModel).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("plugin not found")
		}
		return nil, fmt.Errorf("get plugin by name: %w", err)
	}

	p := modelToPlugin(&pluginModel)
	return &p, nil
}

// CreatePlugin creates a new plugin
func (r *GORMRepository) CreatePlugin(ctx context.Context, p *Plugin) error {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return err
	}

	pluginModel := pluginToModel(p)
	if err := db.Create(pluginModel).Error; err != nil {
		return fmt.Errorf("create plugin: %w", err)
	}
	return nil
}

// UpdatePlugin updates a plugin
func (r *GORMRepository) UpdatePlugin(ctx context.Context, p *Plugin) error {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return err
	}

	if err := db.Model(&models.Plugin{}).
		Where("id = ?", p.ID).
		Updates(map[string]interface{}{
			"state":               p.State,
			"granted_permissions": pq.StringArray(p.GrantedPermissions),
			"updated_at":          time.Now(),
		}).Error; err != nil {
		return fmt.Errorf("update plugin: %w", err)
	}
	return nil
}

// DeletePlugin deletes a plugin
func (r *GORMRepository) DeletePlugin(ctx context.Context, id uuid.UUID) (int64, error) {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return 0, err
	}

	result := db.Where("id = ?", id).Delete(&models.Plugin{})
	if result.Error != nil {
		return 0, fmt.Errorf("delete plugin: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// ListTenantPlugins returns all plugins enabled for a tenant
func (r *GORMRepository) ListTenantPlugins(ctx context.Context, tenantID uuid.UUID) ([]TenantPlugin, error) {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return nil, err
	}

	var tpModels []models.TenantPlugin
	if err := db.
		Preload("Plugin").
		Joins("JOIN plugins ON plugins.id = tenant_plugins.plugin_id").
		Where("tenant_plugins.tenant_id = ?", tenantID).
		Order("plugins.display_name ASC").
		Find(&tpModels).Error; err != nil {
		return nil, fmt.Errorf("list tenant plugins: %w", err)
	}

	result := make([]TenantPlugin, len(tpModels))
	for i, tp := range tpModels {
		result[i] = modelToTenantPlugin(&tp)
	}
	return result, nil
}

// GetTenantPlugin returns a tenant plugin
func (r *GORMRepository) GetTenantPlugin(ctx context.Context, tenantID, pluginID uuid.UUID) (*TenantPlugin, error) {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return nil, err
	}

	var tpModel models.TenantPlugin
	if err := db.
		Preload("Plugin").
		Where("tenant_id = ? AND plugin_id = ?", tenantID, pluginID).
		First(&tpModel).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("tenant plugin not found")
		}
		return nil, fmt.Errorf("get tenant plugin: %w", err)
	}

	tp := modelToTenantPlugin(&tpModel)
	return &tp, nil
}

// CreateTenantPlugin enables a plugin for a tenant
func (r *GORMRepository) CreateTenantPlugin(ctx context.Context, tenantID, pluginID uuid.UUID, settings json.RawMessage) error {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return err
	}

	now := time.Now()
	tpModel := &models.TenantPlugin{
		TenantID:  tenantID,
		PluginID:  pluginID,
		IsEnabled: true,
		Settings:  settings,
		EnabledAt: &now,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := db.Create(tpModel).Error; err != nil {
		return fmt.Errorf("create tenant plugin: %w", err)
	}
	return nil
}

// EnableTenantPlugin enables a plugin for a tenant (upsert)
func (r *GORMRepository) EnableTenantPlugin(ctx context.Context, tenantID, pluginID uuid.UUID, settings json.RawMessage) error {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return err
	}

	now := time.Now()
	tpModel := &models.TenantPlugin{
		TenantID:  tenantID,
		PluginID:  pluginID,
		IsEnabled: true,
		Settings:  settings,
		EnabledAt: &now,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := db.
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "tenant_id"}, {Name: "plugin_id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"is_enabled": true,
				"settings":   settings,
				"enabled_at": &now,
				"updated_at": now,
			}),
		}).
		Create(tpModel).Error; err != nil {
		return fmt.Errorf("enable tenant plugin: %w", err)
	}
	return nil
}

// DisableTenantPlugin disables a plugin for a tenant
func (r *GORMRepository) DisableTenantPlugin(ctx context.Context, tenantID, pluginID uuid.UUID) (int64, error) {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return 0, err
	}

	result := db.Model(&models.TenantPlugin{}).
		Where("tenant_id = ? AND plugin_id = ?", tenantID, pluginID).
		Updates(map[string]interface{}{
			"is_enabled": false,
			"updated_at": time.Now(),
		})
	if result.Error != nil {
		return 0, fmt.Errorf("disable tenant plugin: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// GetTenantPluginSettings returns the settings for a tenant plugin
func (r *GORMRepository) GetTenantPluginSettings(ctx context.Context, tenantID, pluginID uuid.UUID) (json.RawMessage, error) {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return nil, err
	}

	var result struct {
		Settings json.RawMessage `gorm:"column:settings"`
	}
	err = db.Model(&models.TenantPlugin{}).
		Select("settings").
		Where("tenant_id = ? AND plugin_id = ?", tenantID, pluginID).
		Take(&result).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return json.RawMessage("{}"), nil
		}
		return nil, fmt.Errorf("get tenant plugin settings: %w", err)
	}
	if result.Settings == nil {
		return json.RawMessage("{}"), nil
	}
	return result.Settings, nil
}

// UpdateTenantPluginSettings updates tenant plugin settings
func (r *GORMRepository) UpdateTenantPluginSettings(ctx context.Context, tenantID, pluginID uuid.UUID, settings json.RawMessage) error {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return err
	}

	result := db.Model(&models.TenantPlugin{}).
		Where("tenant_id = ? AND plugin_id = ?", tenantID, pluginID).
		Updates(map[string]interface{}{
			"settings":   settings,
			"updated_at": time.Now(),
		})
	if result.Error != nil {
		return fmt.Errorf("update tenant plugin settings: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("tenant plugin not found")
	}
	return nil
}

// DeleteTenantPlugin removes a plugin from a tenant
func (r *GORMRepository) DeleteTenantPlugin(ctx context.Context, tenantID, pluginID uuid.UUID) error {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return err
	}

	if err := db.
		Where("tenant_id = ? AND plugin_id = ?", tenantID, pluginID).
		Delete(&models.TenantPlugin{}).Error; err != nil {
		return fmt.Errorf("delete tenant plugin: %w", err)
	}
	return nil
}

// IsPluginEnabledForTenant checks if a plugin is enabled for a tenant
func (r *GORMRepository) IsPluginEnabledForTenant(ctx context.Context, tenantID, pluginID uuid.UUID) (bool, error) {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return false, err
	}

	var count int64
	if err := db.Model(&models.TenantPlugin{}).
		Where("tenant_id = ? AND plugin_id = ? AND is_enabled = ?", tenantID, pluginID, true).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check tenant plugin: %w", err)
	}
	return count > 0, nil
}

// ListEnabledPlugins returns all enabled plugins
func (r *GORMRepository) ListEnabledPlugins(ctx context.Context) ([]Plugin, error) {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return nil, err
	}

	var pluginModels []models.Plugin
	if err := db.
		Where("state = ?", "enabled").
		Order("display_name ASC").
		Find(&pluginModels).Error; err != nil {
		return nil, fmt.Errorf("list enabled plugins: %w", err)
	}

	plugins := make([]Plugin, len(pluginModels))
	for i, pm := range pluginModels {
		plugins[i] = modelToPlugin(&pm)
	}
	return plugins, nil
}

// InsertPluginReturning inserts a plugin and returns the created record
func (r *GORMRepository) InsertPluginReturning(ctx context.Context, manifest *Manifest, repoURL string, repoType RepositoryType, manifestJSON []byte) (*Plugin, error) {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	pluginModel := &models.Plugin{
		Name:               manifest.Name,
		DisplayName:        manifest.DisplayName,
		Description:        manifest.Description,
		Version:            manifest.Version,
		RepositoryURL:      repoURL,
		RepositoryType:     models.RepositoryType(repoType),
		Author:             manifest.Author,
		License:            manifest.License,
		HomepageURL:        manifest.Homepage,
		State:              models.PluginStateInstalled,
		GrantedPermissions: []string{},
		Manifest:           manifestJSON,
		InstalledAt:        now,
		UpdatedAt:          now,
	}

	if err := db.Create(pluginModel).Error; err != nil {
		return nil, fmt.Errorf("insert plugin: %w", err)
	}

	p := modelToPlugin(pluginModel)
	return &p, nil
}

// CountEnabledTenantsForPlugin counts tenants that have the plugin enabled
func (r *GORMRepository) CountEnabledTenantsForPlugin(ctx context.Context, pluginID uuid.UUID) (int, error) {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return 0, err
	}

	var count int64
	if err := db.Model(&models.TenantPlugin{}).
		Where("plugin_id = ? AND is_enabled = ?", pluginID, true).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count enabled tenants: %w", err)
	}
	return int(count), nil
}

// UpdatePluginState updates a plugin's state and permissions
func (r *GORMRepository) UpdatePluginState(ctx context.Context, pluginID uuid.UUID, state PluginState, permissions []string) error {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return err
	}

	if err := db.Model(&models.Plugin{}).
		Where("id = ?", pluginID).
		Updates(map[string]interface{}{
			"state":               state,
			"granted_permissions": pq.StringArray(permissions),
			"updated_at":          time.Now(),
		}).Error; err != nil {
		return fmt.Errorf("update plugin state: %w", err)
	}
	return nil
}

// DisableAllTenantsForPlugin disables the plugin for all tenants
func (r *GORMRepository) DisableAllTenantsForPlugin(ctx context.Context, pluginID uuid.UUID) error {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return err
	}

	if err := db.Model(&models.TenantPlugin{}).
		Where("plugin_id = ?", pluginID).
		Updates(map[string]interface{}{
			"is_enabled": false,
			"updated_at": time.Now(),
		}).Error; err != nil {
		return fmt.Errorf("disable plugin for all tenants: %w", err)
	}
	return nil
}

// GetTenantPluginsWithAll returns all plugins available to a tenant (enabled or not)
func (r *GORMRepository) GetTenantPluginsWithAll(ctx context.Context, tenantID uuid.UUID) ([]TenantPlugin, error) {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return nil, err
	}

	var results []struct {
		TPID        *uuid.UUID
		TPTenantID  *uuid.UUID
		TPPluginID  *uuid.UUID
		TPIsEnabled *bool
		TPSettings  json.RawMessage
		TPEnabledAt *time.Time
		TPCreatedAt *time.Time
		TPUpdatedAt *time.Time
		models.Plugin
	}

	if err := db.
		Table("plugins AS p").
		Select(`
			tp.id AS tp_id,
			tp.tenant_id AS tp_tenant_id,
			tp.plugin_id AS tp_plugin_id,
			tp.is_enabled AS tp_is_enabled,
			tp.settings AS tp_settings,
			tp.enabled_at AS tp_enabled_at,
			tp.created_at AS tp_created_at,
			tp.updated_at AS tp_updated_at,
			p.*
		`).
		Joins("LEFT JOIN tenant_plugins AS tp ON tp.plugin_id = p.id AND tp.tenant_id = ?", tenantID).
		Where("p.state = ?", models.PluginStateEnabled).
		Order("p.display_name ASC").
		Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("list tenant plugins: %w", err)
	}

	tenantPlugins := make([]TenantPlugin, 0, len(results))
	for _, res := range results {
		p := modelToPlugin(&res.Plugin)
		tp := TenantPlugin{
			PluginID: res.ID,
			TenantID: tenantID,
			Plugin:   &p,
		}

		if res.TPID != nil {
			tp.ID = *res.TPID
			if res.TPIsEnabled != nil {
				tp.IsEnabled = *res.TPIsEnabled
			}
			tp.Settings = res.TPSettings
			tp.EnabledAt = res.TPEnabledAt
			if res.TPCreatedAt != nil {
				tp.CreatedAt = *res.TPCreatedAt
			}
			if res.TPUpdatedAt != nil {
				tp.UpdatedAt = *res.TPUpdatedAt
			}
		}

		tenantPlugins = append(tenantPlugins, tp)
	}

	return tenantPlugins, nil
}

// Conversion helpers

func modelToRegistry(m *models.PluginRegistry) Registry {
	return Registry{
		ID:           m.ID,
		Name:         m.Name,
		URL:          m.URL,
		Description:  m.Description,
		IsOfficial:   m.IsOfficial,
		IsActive:     m.IsActive,
		LastSyncedAt: m.LastSyncedAt,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}

func modelToPlugin(m *models.Plugin) Plugin {
	return Plugin{
		ID:                 m.ID,
		Name:               m.Name,
		DisplayName:        m.DisplayName,
		Description:        m.Description,
		Version:            m.Version,
		RepositoryURL:      m.RepositoryURL,
		RepositoryType:     RepositoryType(m.RepositoryType),
		Author:             m.Author,
		License:            m.License,
		HomepageURL:        m.HomepageURL,
		State:              PluginState(m.State),
		GrantedPermissions: []string(m.GrantedPermissions),
		Manifest:           m.Manifest,
		InstalledAt:        m.InstalledAt,
		UpdatedAt:          m.UpdatedAt,
	}
}

func pluginToModel(p *Plugin) *models.Plugin {
	return &models.Plugin{
		ID:                 p.ID,
		Name:               p.Name,
		DisplayName:        p.DisplayName,
		Description:        p.Description,
		Version:            p.Version,
		RepositoryURL:      p.RepositoryURL,
		RepositoryType:     models.RepositoryType(p.RepositoryType),
		Author:             p.Author,
		License:            p.License,
		HomepageURL:        p.HomepageURL,
		State:              models.PluginState(p.State),
		GrantedPermissions: pq.StringArray(p.GrantedPermissions),
		Manifest:           p.Manifest,
		InstalledAt:        p.InstalledAt,
		UpdatedAt:          p.UpdatedAt,
	}
}

func modelToTenantPlugin(m *models.TenantPlugin) TenantPlugin {
	tp := TenantPlugin{
		ID:        m.ID,
		TenantID:  m.TenantID,
		PluginID:  m.PluginID,
		IsEnabled: m.IsEnabled,
		Settings:  m.Settings,
		EnabledAt: m.EnabledAt,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}

	if m.Plugin != nil {
		p := modelToPlugin(m.Plugin)
		tp.Plugin = &p
	}

	return tp
}

// Ensure GORMRepository implements Repository interface
var _ Repository = (*GORMRepository)(nil)
