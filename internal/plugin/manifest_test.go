package plugin

import (
	"os"
	"testing"
)

func TestParseManifest(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "Valid minimal manifest",
			yaml: `
name: my-plugin
display_name: My Plugin
version: 1.0.0
`,
			wantErr: false,
		},
		{
			name: "Valid full manifest",
			yaml: `
name: expense-tracker
display_name: Expense Tracker
version: 2.1.0
description: Track employee expenses
author: Test Author
license: MIT
homepage: https://github.com/test/expense-tracker
min_app_version: 1.0.0
permissions:
  - invoices:read
  - email:send
backend:
  package: ./backend
  entry: NewService
  hooks:
    - event: invoice.created
      handler: OnInvoiceCreated
  routes:
    - method: GET
      path: /expenses
      handler: ListExpenses
frontend:
  components: ./frontend
  navigation:
    - label: Expenses
      icon: receipt
      path: /expenses
  slots:
    - name: dashboard.widgets
      component: ExpenseWidget.svelte
database:
  migrations: ./migrations
dependencies:
  - other-plugin
settings:
  type: object
  properties:
    enabled:
      type: boolean
      default: true
`,
			wantErr: false,
		},
		{
			name:    "Invalid YAML",
			yaml:    "not: valid: yaml: syntax",
			wantErr: true,
		},
		{
			name:    "Empty YAML",
			yaml:    "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseManifest([]byte(tt.yaml))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseManifest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestManifestValidate(t *testing.T) {
	tests := []struct {
		name     string
		manifest Manifest
		wantErr  bool
		errMsg   string
	}{
		{
			name: "Valid minimal manifest",
			manifest: Manifest{
				Name:        "my-plugin",
				DisplayName: "My Plugin",
				Version:     "1.0.0",
			},
			wantErr: false,
		},
		{
			name: "Missing name",
			manifest: Manifest{
				DisplayName: "My Plugin",
				Version:     "1.0.0",
			},
			wantErr: true,
			errMsg:  "plugin name is required",
		},
		{
			name: "Missing display_name",
			manifest: Manifest{
				Name:    "my-plugin",
				Version: "1.0.0",
			},
			wantErr: true,
			errMsg:  "plugin display_name is required",
		},
		{
			name: "Missing version",
			manifest: Manifest{
				Name:        "my-plugin",
				DisplayName: "My Plugin",
			},
			wantErr: true,
			errMsg:  "plugin version is required",
		},
		{
			name: "Invalid name format - uppercase",
			manifest: Manifest{
				Name:        "MyPlugin",
				DisplayName: "My Plugin",
				Version:     "1.0.0",
			},
			wantErr: true,
			errMsg:  "plugin name must be lowercase",
		},
		{
			name: "Invalid name format - spaces",
			manifest: Manifest{
				Name:        "my plugin",
				DisplayName: "My Plugin",
				Version:     "1.0.0",
			},
			wantErr: true,
			errMsg:  "plugin name must be lowercase",
		},
		{
			name: "Invalid name format - underscore",
			manifest: Manifest{
				Name:        "my_plugin",
				DisplayName: "My Plugin",
				Version:     "1.0.0",
			},
			wantErr: true,
			errMsg:  "plugin name must be lowercase",
		},
		{
			name: "Invalid version format",
			manifest: Manifest{
				Name:        "my-plugin",
				DisplayName: "My Plugin",
				Version:     "v1.0",
			},
			wantErr: true,
			errMsg:  "plugin version must be valid semver",
		},
		{
			name: "Invalid permission",
			manifest: Manifest{
				Name:        "my-plugin",
				DisplayName: "My Plugin",
				Version:     "1.0.0",
				Permissions: []string{"invalid:permission"},
			},
			wantErr: true,
			errMsg:  "invalid permissions",
		},
		{
			name: "Invalid license",
			manifest: Manifest{
				Name:        "my-plugin",
				DisplayName: "My Plugin",
				Version:     "1.0.0",
				License:     "INVALID",
			},
			wantErr: true,
			errMsg:  "license must be an OSI-approved",
		},
		{
			name: "Backend missing package",
			manifest: Manifest{
				Name:        "my-plugin",
				DisplayName: "My Plugin",
				Version:     "1.0.0",
				Backend:     &BackendConfig{Entry: "NewService"},
			},
			wantErr: true,
			errMsg:  "backend.package is required",
		},
		{
			name: "Backend missing entry",
			manifest: Manifest{
				Name:        "my-plugin",
				DisplayName: "My Plugin",
				Version:     "1.0.0",
				Backend:     &BackendConfig{Package: "./backend"},
			},
			wantErr: true,
			errMsg:  "backend.entry is required",
		},
		{
			name: "Routes without permission",
			manifest: Manifest{
				Name:        "my-plugin",
				DisplayName: "My Plugin",
				Version:     "1.0.0",
				Backend: &BackendConfig{
					Package: "./backend",
					Entry:   "NewService",
					Routes:  []RouteConfig{{Method: "GET", Path: "/test", Handler: "Test"}},
				},
			},
			wantErr: true,
			errMsg:  "routes:register permission is required",
		},
		{
			name: "Hooks without permission",
			manifest: Manifest{
				Name:        "my-plugin",
				DisplayName: "My Plugin",
				Version:     "1.0.0",
				Backend: &BackendConfig{
					Package: "./backend",
					Entry:   "NewService",
					Hooks:   []HookConfig{{Event: "test.event", Handler: "OnTest"}},
				},
			},
			wantErr: true,
			errMsg:  "hooks:register permission is required",
		},
		{
			name: "Routes with permission - valid",
			manifest: Manifest{
				Name:        "my-plugin",
				DisplayName: "My Plugin",
				Version:     "1.0.0",
				Permissions: []string{"routes:register"},
				Backend: &BackendConfig{
					Package: "./backend",
					Entry:   "NewService",
					Routes:  []RouteConfig{{Method: "GET", Path: "/test", Handler: "Test"}},
				},
			},
			wantErr: false,
		},
		{
			name: "Hooks with permission - valid",
			manifest: Manifest{
				Name:        "my-plugin",
				DisplayName: "My Plugin",
				Version:     "1.0.0",
				Permissions: []string{"hooks:register"},
				Backend: &BackendConfig{
					Package: "./backend",
					Entry:   "NewService",
					Hooks:   []HookConfig{{Event: "test.event", Handler: "OnTest"}},
				},
			},
			wantErr: false,
		},
		{
			name: "Frontend missing components",
			manifest: Manifest{
				Name:        "my-plugin",
				DisplayName: "My Plugin",
				Version:     "1.0.0",
				Frontend:    &FrontendConfig{},
			},
			wantErr: true,
			errMsg:  "frontend.components is required",
		},
		{
			name: "Database missing migrations path",
			manifest: Manifest{
				Name:        "my-plugin",
				DisplayName: "My Plugin",
				Version:     "1.0.0",
				Permissions: []string{"database:migrate"},
				Database:    &DatabaseConfig{Migrations: ""},
			},
			wantErr: true,
			errMsg:  "database.migrations is required",
		},
		{
			name: "Database without permission",
			manifest: Manifest{
				Name:        "my-plugin",
				DisplayName: "My Plugin",
				Version:     "1.0.0",
				Database:    &DatabaseConfig{Migrations: "./migrations"},
			},
			wantErr: true,
			errMsg:  "database:migrate permission is required",
		},
		{
			name: "Database with permission - valid",
			manifest: Manifest{
				Name:        "my-plugin",
				DisplayName: "My Plugin",
				Version:     "1.0.0",
				Permissions: []string{"database:migrate"},
				Database:    &DatabaseConfig{Migrations: "./migrations"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Manifest.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !containsString(err.Error(), tt.errMsg) {
					t.Errorf("Manifest.Validate() error = %q, should contain %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestIsValidPluginName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Valid names
		{"Simple lowercase", "myplugin", true},
		{"With hyphen", "my-plugin", true},
		{"Multiple hyphens", "my-cool-plugin", true},
		{"Single letter", "a", true},
		{"Numbers", "plugin123", true},
		{"Alphanumeric with hyphens", "my-plugin-v2", true},

		// Invalid names
		{"Empty", "", false},
		{"Uppercase", "MyPlugin", false},
		{"Mixed case", "myPlugin", false},
		{"Starts with number", "123plugin", false},
		{"Starts with hyphen", "-plugin", false},
		{"Ends with hyphen", "plugin-", false},
		{"Underscore", "my_plugin", false},
		{"Space", "my plugin", false},
		{"Special char", "my@plugin", false},
		{"Too long", string(make([]byte, 101)), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidPluginName(tt.input)
			if result != tt.expected {
				t.Errorf("isValidPluginName(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsValidSemver(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		expected bool
	}{
		// Valid versions
		{"Simple version", "1.0.0", true},
		{"With minor", "1.2.3", true},
		{"Large numbers", "100.200.300", true},
		{"With prerelease", "1.0.0-alpha", true},
		{"With prerelease.1", "1.0.0-alpha.1", true},
		{"With build metadata", "1.0.0+build", true},
		{"With both", "1.0.0-beta+build.123", true},
		{"Zero version", "0.0.0", true},

		// Invalid versions
		{"Empty", "", false},
		{"Single number", "1", false},
		{"Two numbers", "1.0", false},
		{"With v prefix", "v1.0.0", false},
		{"With spaces", "1.0.0 beta", false},
		{"Letters in version", "1.0.a", false},
		{"Negative number", "-1.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidSemver(tt.version)
			if result != tt.expected {
				t.Errorf("isValidSemver(%q) = %v, want %v", tt.version, result, tt.expected)
			}
		})
	}
}

func TestIsValidLicense(t *testing.T) {
	tests := []struct {
		name     string
		license  string
		expected bool
	}{
		// Valid licenses
		{"MIT", "MIT", true},
		{"Apache", "Apache-2.0", true},
		{"GPL v3", "GPL-3.0", true},
		{"BSD 3-Clause", "BSD-3-Clause", true},
		{"ISC", "ISC", true},
		{"MPL", "MPL-2.0", true},
		{"AGPL", "AGPL-3.0", true},
		{"Unlicense", "Unlicense", true},

		// Invalid licenses
		{"Empty", "", false},
		{"Unknown", "PROPRIETARY", false},
		{"Lowercase mit", "mit", false},
		{"Typo", "MITT", false},
		{"Commercial", "Commercial", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidLicense(tt.license)
			if result != tt.expected {
				t.Errorf("isValidLicense(%q) = %v, want %v", tt.license, result, tt.expected)
			}
		})
	}
}

func TestManifestRequiredPermissions(t *testing.T) {
	tests := []struct {
		name     string
		manifest Manifest
		expected []string
	}{
		{
			name:     "No requirements",
			manifest: Manifest{Name: "test"},
			expected: nil,
		},
		{
			name: "Routes require routes:register",
			manifest: Manifest{
				Name: "test",
				Backend: &BackendConfig{
					Routes: []RouteConfig{{Method: "GET", Path: "/test"}},
				},
			},
			expected: []string{"routes:register"},
		},
		{
			name: "Hooks require hooks:register",
			manifest: Manifest{
				Name: "test",
				Backend: &BackendConfig{
					Hooks: []HookConfig{{Event: "test.event"}},
				},
			},
			expected: []string{"hooks:register"},
		},
		{
			name: "Database requires database:migrate",
			manifest: Manifest{
				Name:     "test",
				Database: &DatabaseConfig{Migrations: "./migrations"},
			},
			expected: []string{"database:migrate"},
		},
		{
			name: "Multiple requirements",
			manifest: Manifest{
				Name: "test",
				Backend: &BackendConfig{
					Routes: []RouteConfig{{Method: "GET", Path: "/test"}},
					Hooks:  []HookConfig{{Event: "test.event"}},
				},
				Database: &DatabaseConfig{Migrations: "./migrations"},
			},
			expected: []string{"routes:register", "hooks:register", "database:migrate"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.manifest.RequiredPermissions()
			if len(result) != len(tt.expected) {
				t.Errorf("RequiredPermissions() returned %d, want %d", len(result), len(tt.expected))
				return
			}
			for i, perm := range result {
				if perm != tt.expected[i] {
					t.Errorf("RequiredPermissions()[%d] = %q, want %q", i, perm, tt.expected[i])
				}
			}
		})
	}
}

func TestManifestPathHelpers(t *testing.T) {
	manifest := Manifest{
		Name: "test-plugin",
		Backend: &BackendConfig{
			Package: "./backend",
		},
		Frontend: &FrontendConfig{
			Components: "./frontend/components",
		},
		Database: &DatabaseConfig{
			Migrations: "./migrations",
		},
	}

	pluginDir := "/plugins/test-plugin"

	t.Run("GetBackendPath", func(t *testing.T) {
		path := manifest.GetBackendPath(pluginDir)
		expected := "/plugins/test-plugin/backend"
		if path != expected {
			t.Errorf("GetBackendPath() = %q, want %q", path, expected)
		}
	})

	t.Run("GetFrontendPath", func(t *testing.T) {
		path := manifest.GetFrontendPath(pluginDir)
		expected := "/plugins/test-plugin/frontend/components"
		if path != expected {
			t.Errorf("GetFrontendPath() = %q, want %q", path, expected)
		}
	})

	t.Run("GetMigrationPath", func(t *testing.T) {
		path := manifest.GetMigrationPath(pluginDir)
		expected := "/plugins/test-plugin/migrations"
		if path != expected {
			t.Errorf("GetMigrationPath() = %q, want %q", path, expected)
		}
	})

	t.Run("Empty backend path", func(t *testing.T) {
		m := Manifest{Name: "test"}
		if path := m.GetBackendPath(pluginDir); path != "" {
			t.Errorf("GetBackendPath() on empty = %q, want empty", path)
		}
	})

	t.Run("Empty frontend path", func(t *testing.T) {
		m := Manifest{Name: "test"}
		if path := m.GetFrontendPath(pluginDir); path != "" {
			t.Errorf("GetFrontendPath() on empty = %q, want empty", path)
		}
	})

	t.Run("Empty migration path", func(t *testing.T) {
		m := Manifest{Name: "test"}
		if path := m.GetMigrationPath(pluginDir); path != "" {
			t.Errorf("GetMigrationPath() on empty = %q, want empty", path)
		}
	})
}

func TestLoadManifest(t *testing.T) {
	t.Run("Valid manifest file", func(t *testing.T) {
		tmpDir := t.TempDir()
		manifestPath := tmpDir + "/plugin.yaml"
		content := `name: test-plugin
display_name: Test Plugin
version: 1.0.0
description: A test plugin
`
		if err := os.WriteFile(manifestPath, []byte(content), 0600); err != nil {
			t.Fatalf("Failed to write test manifest: %v", err)
		}

		m, err := LoadManifest(manifestPath)
		if err != nil {
			t.Fatalf("LoadManifest() error = %v", err)
		}

		if m.Name != "test-plugin" {
			t.Errorf("Name = %q, want %q", m.Name, "test-plugin")
		}
		if m.DisplayName != "Test Plugin" {
			t.Errorf("DisplayName = %q, want %q", m.DisplayName, "Test Plugin")
		}
		if m.Version != "1.0.0" {
			t.Errorf("Version = %q, want %q", m.Version, "1.0.0")
		}
		if m.Description != "A test plugin" {
			t.Errorf("Description = %q, want %q", m.Description, "A test plugin")
		}
	})

	t.Run("File not found", func(t *testing.T) {
		_, err := LoadManifest("/non/existent/path/plugin.yaml")
		if err == nil {
			t.Error("LoadManifest() expected error for non-existent file")
		}
	})

	t.Run("Invalid YAML content", func(t *testing.T) {
		tmpDir := t.TempDir()
		manifestPath := tmpDir + "/plugin.yaml"
		// Invalid YAML with bad indentation
		content := `name: test
  bad: indentation: here`
		if err := os.WriteFile(manifestPath, []byte(content), 0600); err != nil {
			t.Fatalf("Failed to write test manifest: %v", err)
		}

		_, err := LoadManifest(manifestPath)
		if err == nil {
			t.Error("LoadManifest() expected error for invalid YAML")
		}
	})

	t.Run("Full manifest with all fields", func(t *testing.T) {
		tmpDir := t.TempDir()
		manifestPath := tmpDir + "/plugin.yaml"
		content := `name: expense-tracker
display_name: Expense Tracker
version: 2.1.0
description: Track employee expenses
author: Test Author
license: MIT
homepage: https://github.com/test/expense-tracker
min_app_version: 1.0.0
permissions:
  - invoices:read
  - routes:register
  - hooks:register
backend:
  package: ./backend
  entry: NewService
  hooks:
    - event: invoice.created
      handler: OnInvoiceCreated
  routes:
    - method: GET
      path: /expenses
      handler: ListExpenses
frontend:
  components: ./frontend
  navigation:
    - label: Expenses
      icon: receipt
      path: /expenses
  slots:
    - name: dashboard.widgets
      component: ExpenseWidget.svelte
database:
  migrations: ./migrations
dependencies:
  - other-plugin
settings:
  type: object
  properties:
    enabled:
      type: boolean
      default: true
`
		if err := os.WriteFile(manifestPath, []byte(content), 0600); err != nil {
			t.Fatalf("Failed to write test manifest: %v", err)
		}

		m, err := LoadManifest(manifestPath)
		if err != nil {
			t.Fatalf("LoadManifest() error = %v", err)
		}

		if m.Name != "expense-tracker" {
			t.Errorf("Name = %q, want %q", m.Name, "expense-tracker")
		}
		if m.Author != "Test Author" {
			t.Errorf("Author = %q, want %q", m.Author, "Test Author")
		}
		if m.License != "MIT" {
			t.Errorf("License = %q, want %q", m.License, "MIT")
		}
		if m.Backend == nil {
			t.Fatal("Backend is nil")
		}
		if m.Backend.Package != "./backend" {
			t.Errorf("Backend.Package = %q, want %q", m.Backend.Package, "./backend")
		}
		if len(m.Backend.Hooks) != 1 {
			t.Errorf("Backend.Hooks len = %d, want 1", len(m.Backend.Hooks))
		}
		if len(m.Backend.Routes) != 1 {
			t.Errorf("Backend.Routes len = %d, want 1", len(m.Backend.Routes))
		}
		if m.Frontend == nil {
			t.Fatal("Frontend is nil")
		}
		if m.Frontend.Components != "./frontend" {
			t.Errorf("Frontend.Components = %q, want %q", m.Frontend.Components, "./frontend")
		}
		if len(m.Frontend.Navigation) != 1 {
			t.Errorf("Frontend.Navigation len = %d, want 1", len(m.Frontend.Navigation))
		}
		if len(m.Frontend.Slots) != 1 {
			t.Errorf("Frontend.Slots len = %d, want 1", len(m.Frontend.Slots))
		}
		if m.Database == nil {
			t.Fatal("Database is nil")
		}
		if m.Database.Migrations != "./migrations" {
			t.Errorf("Database.Migrations = %q, want %q", m.Database.Migrations, "./migrations")
		}
		if len(m.Dependencies) != 1 {
			t.Errorf("Dependencies len = %d, want 1", len(m.Dependencies))
		}
		if m.Settings == nil {
			t.Fatal("Settings is nil")
		}
		if m.Settings.Type != "object" {
			t.Errorf("Settings.Type = %q, want %q", m.Settings.Type, "object")
		}
	})

	t.Run("Empty file", func(t *testing.T) {
		tmpDir := t.TempDir()
		manifestPath := tmpDir + "/plugin.yaml"
		if err := os.WriteFile(manifestPath, []byte(""), 0600); err != nil {
			t.Fatalf("Failed to write test manifest: %v", err)
		}

		m, err := LoadManifest(manifestPath)
		if err != nil {
			t.Fatalf("LoadManifest() error = %v", err)
		}

		// Empty YAML results in empty manifest
		if m.Name != "" {
			t.Errorf("Name = %q, want empty", m.Name)
		}
	})
}

func TestManifestToJSON(t *testing.T) {
	manifest := Manifest{
		Name:        "test-plugin",
		DisplayName: "Test Plugin",
		Version:     "1.0.0",
		Description: "A test plugin",
		Author:      "Test Author",
		License:     "MIT",
		Permissions: []string{"contacts:read"},
	}

	jsonData, err := manifest.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error = %v", err)
	}

	if len(jsonData) == 0 {
		t.Error("ToJSON() returned empty data")
	}

	// Parse back to verify
	parsed, err := ParseManifest([]byte(`
name: test-plugin
display_name: Test Plugin
version: 1.0.0
`))
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}

	if parsed.Name != manifest.Name {
		t.Errorf("Parsed name = %q, want %q", parsed.Name, manifest.Name)
	}
}

func TestContainsPermission(t *testing.T) {
	tests := []struct {
		name        string
		permissions []string
		permission  string
		expected    bool
	}{
		{
			name:        "permission_exists",
			permissions: []string{"hooks:register", "routes:register"},
			permission:  "hooks:register",
			expected:    true,
		},
		{
			name:        "permission_not_exists",
			permissions: []string{"hooks:register", "routes:register"},
			permission:  "database:migrate",
			expected:    false,
		},
		{
			name:        "empty_permissions",
			permissions: []string{},
			permission:  "hooks:register",
			expected:    false,
		},
		{
			name:        "nil_permissions",
			permissions: nil,
			permission:  "hooks:register",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsPermission(tt.permissions, tt.permission)
			if result != tt.expected {
				t.Errorf("containsPermission() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestManifestValidateSingleLetter(t *testing.T) {
	manifest := Manifest{
		Name:        "a",
		DisplayName: "Single Letter Plugin",
		Version:     "1.0.0",
	}

	err := manifest.Validate()
	if err != nil {
		t.Errorf("Validate() error = %v, single letter name should be valid", err)
	}
}

func TestManifestValidateWithSettings(t *testing.T) {
	minVal := 0.0
	maxVal := 100.0
	minLen := 1
	maxLen := 255

	manifest := Manifest{
		Name:        "settings-plugin",
		DisplayName: "Settings Plugin",
		Version:     "1.0.0",
		Settings: &SettingsSchema{
			Type: "object",
			Properties: map[string]SettingProperty{
				"enabled": {
					Type:        "boolean",
					Default:     true,
					Description: "Enable the plugin",
				},
				"count": {
					Type:    "number",
					Minimum: &minVal,
					Maximum: &maxVal,
				},
				"name": {
					Type:      "string",
					MinLength: &minLen,
					MaxLength: &maxLen,
					Enum:      []string{"option1", "option2"},
				},
			},
			Required: []string{"enabled"},
		},
	}

	err := manifest.Validate()
	if err != nil {
		t.Errorf("Validate() error = %v, manifest with settings should be valid", err)
	}
}

func TestManifestValidateLicenseVariants(t *testing.T) {
	validLicenses := []string{
		"MIT", "Apache-2.0", "GPL-2.0", "GPL-3.0", "LGPL-2.1", "LGPL-3.0",
		"BSD-2-Clause", "BSD-3-Clause", "MPL-2.0", "ISC", "AGPL-3.0",
		"Unlicense", "WTFPL", "CC0-1.0", "0BSD",
	}

	for _, license := range validLicenses {
		t.Run(license, func(t *testing.T) {
			manifest := Manifest{
				Name:        "test-plugin",
				DisplayName: "Test Plugin",
				Version:     "1.0.0",
				License:     license,
			}

			err := manifest.Validate()
			if err != nil {
				t.Errorf("Validate() with license %q error = %v", license, err)
			}
		})
	}
}

func TestManifestNavigationPosition(t *testing.T) {
	manifest := Manifest{
		Name:        "nav-plugin",
		DisplayName: "Navigation Plugin",
		Version:     "1.0.0",
		Frontend: &FrontendConfig{
			Components: "./frontend",
			Navigation: []NavigationItem{
				{
					Label:    "Dashboard",
					Icon:     "home",
					Path:     "/dashboard",
					Position: "top",
				},
				{
					Label:    "Settings",
					Icon:     "cog",
					Path:     "/settings",
					Position: "bottom",
				},
			},
			Slots: []SlotConfig{
				{
					Name:      "sidebar.main",
					Component: "SidebarWidget.svelte",
				},
			},
		},
	}

	err := manifest.Validate()
	if err != nil {
		t.Errorf("Validate() error = %v", err)
	}

	// Check paths
	pluginDir := "/plugins/nav-plugin"
	frontendPath := manifest.GetFrontendPath(pluginDir)
	if frontendPath != "/plugins/nav-plugin/frontend" {
		t.Errorf("GetFrontendPath() = %q, want %q", frontendPath, "/plugins/nav-plugin/frontend")
	}
}
