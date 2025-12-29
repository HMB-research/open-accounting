package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"
)

// Manifest represents the plugin.yaml configuration file
type Manifest struct {
	// Plugin Metadata
	Name          string `yaml:"name" json:"name"`
	DisplayName   string `yaml:"display_name" json:"display_name"`
	Version       string `yaml:"version" json:"version"`
	Description   string `yaml:"description,omitempty" json:"description,omitempty"`
	Author        string `yaml:"author,omitempty" json:"author,omitempty"`
	License       string `yaml:"license,omitempty" json:"license,omitempty"`
	Homepage      string `yaml:"homepage,omitempty" json:"homepage,omitempty"`
	MinAppVersion string `yaml:"min_app_version,omitempty" json:"min_app_version,omitempty"`

	// Permissions
	Permissions []string `yaml:"permissions,omitempty" json:"permissions,omitempty"`

	// Backend Configuration
	Backend *BackendConfig `yaml:"backend,omitempty" json:"backend,omitempty"`

	// Frontend Configuration
	Frontend *FrontendConfig `yaml:"frontend,omitempty" json:"frontend,omitempty"`

	// Database Configuration
	Database *DatabaseConfig `yaml:"database,omitempty" json:"database,omitempty"`

	// Dependencies
	Dependencies []string `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`

	// Settings Schema (JSON Schema)
	Settings *SettingsSchema `yaml:"settings,omitempty" json:"settings,omitempty"`
}

// BackendConfig represents the backend section of the manifest
type BackendConfig struct {
	Package string        `yaml:"package" json:"package"`
	Entry   string        `yaml:"entry" json:"entry"`
	Hooks   []HookConfig  `yaml:"hooks,omitempty" json:"hooks,omitempty"`
	Routes  []RouteConfig `yaml:"routes,omitempty" json:"routes,omitempty"`
}

// HookConfig represents an event hook subscription
type HookConfig struct {
	Event   string `yaml:"event" json:"event"`
	Handler string `yaml:"handler" json:"handler"`
}

// RouteConfig represents an API route to register
type RouteConfig struct {
	Method  string `yaml:"method" json:"method"`
	Path    string `yaml:"path" json:"path"`
	Handler string `yaml:"handler" json:"handler"`
}

// FrontendConfig represents the frontend section of the manifest
type FrontendConfig struct {
	Components string           `yaml:"components" json:"components"`
	Navigation []NavigationItem `yaml:"navigation,omitempty" json:"navigation,omitempty"`
	Slots      []SlotConfig     `yaml:"slots,omitempty" json:"slots,omitempty"`
}

// NavigationItem represents a navigation menu item
type NavigationItem struct {
	Label    string `yaml:"label" json:"label"`
	Icon     string `yaml:"icon" json:"icon"`
	Path     string `yaml:"path" json:"path"`
	Position string `yaml:"position,omitempty" json:"position,omitempty"`
}

// SlotConfig represents a UI slot injection
type SlotConfig struct {
	Name      string `yaml:"name" json:"name"`
	Component string `yaml:"component" json:"component"`
}

// DatabaseConfig represents the database section of the manifest
type DatabaseConfig struct {
	Migrations string `yaml:"migrations" json:"migrations"`
}

// SettingsSchema represents a JSON Schema for plugin settings
type SettingsSchema struct {
	Type       string                     `yaml:"type" json:"type"`
	Properties map[string]SettingProperty `yaml:"properties,omitempty" json:"properties,omitempty"`
	Required   []string                   `yaml:"required,omitempty" json:"required,omitempty"`
}

// SettingProperty represents a single setting property
type SettingProperty struct {
	Type        string      `yaml:"type" json:"type"`
	Default     interface{} `yaml:"default,omitempty" json:"default,omitempty"`
	Description string      `yaml:"description,omitempty" json:"description,omitempty"`
	Minimum     *float64    `yaml:"minimum,omitempty" json:"minimum,omitempty"`
	Maximum     *float64    `yaml:"maximum,omitempty" json:"maximum,omitempty"`
	MinLength   *int        `yaml:"minLength,omitempty" json:"minLength,omitempty"`
	MaxLength   *int        `yaml:"maxLength,omitempty" json:"maxLength,omitempty"`
	Enum        []string    `yaml:"enum,omitempty" json:"enum,omitempty"`
}

// LoadManifest loads a plugin manifest from a file path
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	return ParseManifest(data)
}

// ParseManifest parses manifest data from bytes
func ParseManifest(data []byte) (*Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	return &m, nil
}

// ToJSON converts the manifest to JSON
func (m *Manifest) ToJSON() (json.RawMessage, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Validate checks if the manifest is valid
func (m *Manifest) Validate() error {
	// Validate required fields
	if m.Name == "" {
		return fmt.Errorf("plugin name is required")
	}
	if m.DisplayName == "" {
		return fmt.Errorf("plugin display_name is required")
	}
	if m.Version == "" {
		return fmt.Errorf("plugin version is required")
	}

	// Validate name format (lowercase, hyphens only)
	if !isValidPluginName(m.Name) {
		return fmt.Errorf("plugin name must be lowercase alphanumeric with hyphens only")
	}

	// Validate version format (semver)
	if !isValidSemver(m.Version) {
		return fmt.Errorf("plugin version must be valid semver (e.g., 1.0.0)")
	}

	// Validate permissions
	if invalid := ValidatePermissions(m.Permissions); len(invalid) > 0 {
		return fmt.Errorf("invalid permissions: %v", invalid)
	}

	// Validate license if provided
	if m.License != "" && !isValidLicense(m.License) {
		return fmt.Errorf("license must be an OSI-approved license identifier")
	}

	// Validate backend config
	if m.Backend != nil {
		if m.Backend.Package == "" {
			return fmt.Errorf("backend.package is required when backend is specified")
		}
		if m.Backend.Entry == "" {
			return fmt.Errorf("backend.entry is required when backend is specified")
		}

		// Check for routes:register permission if routes are defined
		if len(m.Backend.Routes) > 0 && !containsPermission(m.Permissions, "routes:register") {
			return fmt.Errorf("routes:register permission is required to define API routes")
		}

		// Check for hooks:register permission if hooks are defined
		if len(m.Backend.Hooks) > 0 && !containsPermission(m.Permissions, "hooks:register") {
			return fmt.Errorf("hooks:register permission is required to define event hooks")
		}
	}

	// Validate frontend config
	if m.Frontend != nil {
		if m.Frontend.Components == "" {
			return fmt.Errorf("frontend.components is required when frontend is specified")
		}
	}

	// Validate database config
	if m.Database != nil {
		if m.Database.Migrations == "" {
			return fmt.Errorf("database.migrations is required when database is specified")
		}
		if !containsPermission(m.Permissions, "database:migrate") {
			return fmt.Errorf("database:migrate permission is required to define migrations")
		}
	}

	return nil
}

// RequiredPermissions returns a list of permissions that would be required
// based on the manifest configuration
func (m *Manifest) RequiredPermissions() []string {
	var required []string

	if m.Backend != nil {
		if len(m.Backend.Routes) > 0 {
			required = append(required, "routes:register")
		}
		if len(m.Backend.Hooks) > 0 {
			required = append(required, "hooks:register")
		}
	}

	if m.Database != nil && m.Database.Migrations != "" {
		required = append(required, "database:migrate")
	}

	return required
}

// GetMigrationPath returns the full path to the migrations directory
func (m *Manifest) GetMigrationPath(pluginDir string) string {
	if m.Database == nil || m.Database.Migrations == "" {
		return ""
	}
	return filepath.Join(pluginDir, m.Database.Migrations)
}

// GetFrontendPath returns the full path to the frontend components directory
func (m *Manifest) GetFrontendPath(pluginDir string) string {
	if m.Frontend == nil || m.Frontend.Components == "" {
		return ""
	}
	return filepath.Join(pluginDir, m.Frontend.Components)
}

// GetBackendPath returns the full path to the backend package directory
func (m *Manifest) GetBackendPath(pluginDir string) string {
	if m.Backend == nil || m.Backend.Package == "" {
		return ""
	}
	return filepath.Join(pluginDir, m.Backend.Package)
}

// Helper functions

var pluginNameRegex = regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$|^[a-z]$`)
var semverRegex = regexp.MustCompile(`^\d+\.\d+\.\d+(-[a-zA-Z0-9.-]+)?(\+[a-zA-Z0-9.-]+)?$`)

func isValidPluginName(name string) bool {
	if len(name) < 1 || len(name) > 100 {
		return false
	}
	return pluginNameRegex.MatchString(name)
}

func isValidSemver(version string) bool {
	return semverRegex.MatchString(version)
}

// OSI-approved licenses commonly used
var validLicenses = map[string]bool{
	"MIT":          true,
	"Apache-2.0":   true,
	"GPL-2.0":      true,
	"GPL-3.0":      true,
	"LGPL-2.1":     true,
	"LGPL-3.0":     true,
	"BSD-2-Clause": true,
	"BSD-3-Clause": true,
	"MPL-2.0":      true,
	"ISC":          true,
	"AGPL-3.0":     true,
	"Unlicense":    true,
	"WTFPL":        true,
	"CC0-1.0":      true,
	"0BSD":         true,
}

func isValidLicense(license string) bool {
	return validLicenses[license]
}

func containsPermission(permissions []string, permission string) bool {
	for _, p := range permissions {
		if p == permission {
			return true
		}
	}
	return false
}
