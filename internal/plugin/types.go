package plugin

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// PluginState represents the lifecycle state of a plugin
type PluginState string

const (
	StateInstalled PluginState = "installed"
	StateEnabled   PluginState = "enabled"
	StateDisabled  PluginState = "disabled"
	StateFailed    PluginState = "failed"
)

// RepositoryType represents the source control type
type RepositoryType string

const (
	RepoGitHub RepositoryType = "github"
	RepoGitLab RepositoryType = "gitlab"
)

// Registry represents a plugin marketplace source
type Registry struct {
	ID           uuid.UUID  `json:"id"`
	Name         string     `json:"name"`
	URL          string     `json:"url"`
	Description  string     `json:"description,omitempty"`
	IsOfficial   bool       `json:"is_official"`
	IsActive     bool       `json:"is_active"`
	LastSyncedAt *time.Time `json:"last_synced_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// Plugin represents an installed plugin
type Plugin struct {
	ID                 uuid.UUID       `json:"id"`
	Name               string          `json:"name"`
	DisplayName        string          `json:"display_name"`
	Description        string          `json:"description,omitempty"`
	Version            string          `json:"version"`
	RepositoryURL      string          `json:"repository_url"`
	RepositoryType     RepositoryType  `json:"repository_type"`
	Author             string          `json:"author,omitempty"`
	License            string          `json:"license,omitempty"`
	HomepageURL        string          `json:"homepage_url,omitempty"`
	State              PluginState     `json:"state"`
	GrantedPermissions []string        `json:"granted_permissions"`
	Manifest           json.RawMessage `json:"manifest"`
	InstalledAt        time.Time       `json:"installed_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// TenantPlugin represents a plugin enabled for a specific tenant
type TenantPlugin struct {
	ID        uuid.UUID       `json:"id"`
	TenantID  uuid.UUID       `json:"tenant_id"`
	PluginID  uuid.UUID       `json:"plugin_id"`
	IsEnabled bool            `json:"is_enabled"`
	Settings  json.RawMessage `json:"settings,omitempty"`
	EnabledAt *time.Time      `json:"enabled_at,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`

	// Joined fields
	Plugin *Plugin `json:"plugin,omitempty"`
}

// PluginMigration tracks applied migrations for a plugin
type PluginMigration struct {
	ID        uuid.UUID `json:"id"`
	PluginID  uuid.UUID `json:"plugin_id"`
	Version   string    `json:"version"`
	Filename  string    `json:"filename"`
	AppliedAt time.Time `json:"applied_at"`
	Checksum  string    `json:"checksum,omitempty"`
}

// PluginInfo represents plugin information from a registry
type PluginInfo struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name"`
	Description string   `json:"description,omitempty"`
	Repository  string   `json:"repository"`
	Version     string   `json:"version"`
	Author      string   `json:"author,omitempty"`
	License     string   `json:"license,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Downloads   int      `json:"downloads,omitempty"`
	Stars       int      `json:"stars,omitempty"`
}

// RegistryIndex represents the structure of a registry's plugins.yaml
type RegistryIndex struct {
	Version int          `yaml:"version" json:"version"`
	Plugins []PluginInfo `yaml:"plugins" json:"plugins"`
}

// CreateRegistryRequest represents a request to add a new registry
type CreateRegistryRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=255"`
	URL         string `json:"url" validate:"required,url"`
	Description string `json:"description,omitempty"`
}

// InstallPluginRequest represents a request to install a plugin
type InstallPluginRequest struct {
	RepositoryURL string `json:"repository_url" validate:"required,url"`
}

// EnablePluginRequest represents a request to enable a plugin with permissions
type EnablePluginRequest struct {
	GrantedPermissions []string `json:"granted_permissions"`
}

// TenantPluginSettingsRequest represents a request to update tenant plugin settings
type TenantPluginSettingsRequest struct {
	Settings json.RawMessage `json:"settings"`
}

// PluginSearchResult represents search results from registries
type PluginSearchResult struct {
	Plugin   PluginInfo `json:"plugin"`
	Registry string     `json:"registry"`
}
