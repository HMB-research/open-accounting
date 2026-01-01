package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// PluginState represents the lifecycle state of a plugin
type PluginState string

const (
	PluginStateInstalled PluginState = "installed"
	PluginStateEnabled   PluginState = "enabled"
	PluginStateDisabled  PluginState = "disabled"
	PluginStateFailed    PluginState = "failed"
)

// RepositoryType represents the source control type
type RepositoryType string

const (
	RepoGitHub RepositoryType = "github"
	RepoGitLab RepositoryType = "gitlab"
)

// PluginRegistry represents a plugin marketplace source (GORM model)
type PluginRegistry struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name         string     `gorm:"size:255;not null" json:"name"`
	URL          string     `gorm:"type:text;not null" json:"url"`
	Description  string     `gorm:"type:text" json:"description,omitempty"`
	IsOfficial   bool       `gorm:"column:is_official;not null;default:false" json:"is_official"`
	IsActive     bool       `gorm:"column:is_active;not null;default:true" json:"is_active"`
	LastSyncedAt *time.Time `gorm:"column:last_synced_at" json:"last_synced_at,omitempty"`
	CreatedAt    time.Time  `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"not null;default:now()" json:"updated_at"`
}

// TableName returns the table name for GORM
func (PluginRegistry) TableName() string {
	return "plugin_registries"
}

// Plugin represents an installed plugin (GORM model)
type Plugin struct {
	ID                 uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name               string          `gorm:"size:255;not null;uniqueIndex" json:"name"`
	DisplayName        string          `gorm:"column:display_name;size:255;not null" json:"display_name"`
	Description        string          `gorm:"type:text" json:"description,omitempty"`
	Version            string          `gorm:"size:50;not null" json:"version"`
	RepositoryURL      string          `gorm:"column:repository_url;type:text;not null" json:"repository_url"`
	RepositoryType     RepositoryType  `gorm:"column:repository_type;size:50;not null" json:"repository_type"`
	Author             string          `gorm:"size:255" json:"author,omitempty"`
	License            string          `gorm:"size:50" json:"license,omitempty"`
	HomepageURL        string          `gorm:"column:homepage_url;type:text" json:"homepage_url,omitempty"`
	State              PluginState     `gorm:"size:50;not null;default:'installed'" json:"state"`
	GrantedPermissions pq.StringArray  `gorm:"column:granted_permissions;type:text[]" json:"granted_permissions"`
	Manifest           json.RawMessage `gorm:"type:jsonb" json:"manifest"`
	InstalledAt        time.Time       `gorm:"column:installed_at;not null;default:now()" json:"installed_at"`
	UpdatedAt          time.Time       `gorm:"not null;default:now()" json:"updated_at"`

	// Relations
	TenantPlugins []TenantPlugin `gorm:"foreignKey:PluginID" json:"tenant_plugins,omitempty"`
}

// TableName returns the table name for GORM
func (Plugin) TableName() string {
	return "plugins"
}

// TenantPlugin represents a plugin enabled for a specific tenant (GORM model)
type TenantPlugin struct {
	ID        uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID  uuid.UUID       `gorm:"column:tenant_id;type:uuid;not null;index" json:"tenant_id"`
	PluginID  uuid.UUID       `gorm:"column:plugin_id;type:uuid;not null;index" json:"plugin_id"`
	IsEnabled bool            `gorm:"column:is_enabled;not null;default:true" json:"is_enabled"`
	Settings  json.RawMessage `gorm:"type:jsonb" json:"settings,omitempty"`
	EnabledAt *time.Time      `gorm:"column:enabled_at" json:"enabled_at,omitempty"`
	CreatedAt time.Time       `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time       `gorm:"not null;default:now()" json:"updated_at"`

	// Relations
	Plugin *Plugin `gorm:"foreignKey:PluginID" json:"plugin,omitempty"`
}

// TableName returns the table name for GORM
func (TenantPlugin) TableName() string {
	return "tenant_plugins"
}

// PluginMigration tracks applied migrations for a plugin (GORM model)
type PluginMigration struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	PluginID  uuid.UUID `gorm:"column:plugin_id;type:uuid;not null;index" json:"plugin_id"`
	Version   string    `gorm:"size:50;not null" json:"version"`
	Filename  string    `gorm:"size:255;not null" json:"filename"`
	AppliedAt time.Time `gorm:"column:applied_at;not null;default:now()" json:"applied_at"`
	Checksum  string    `gorm:"size:64" json:"checksum,omitempty"`

	// Relations
	Plugin *Plugin `gorm:"foreignKey:PluginID" json:"plugin,omitempty"`
}

// TableName returns the table name for GORM
func (PluginMigration) TableName() string {
	return "plugin_migrations"
}
