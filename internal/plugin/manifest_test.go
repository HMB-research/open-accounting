package plugin

import (
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
