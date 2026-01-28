package plugin

import (
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRepositoryType(t *testing.T) {
	tests := []struct {
		name        string
		repoURL     string
		expected    RepositoryType
		expectError bool
	}{
		{
			name:     "GitHub HTTPS URL",
			repoURL:  "https://github.com/owner/repo",
			expected: RepoGitHub,
		},
		{
			name:     "GitHub HTTPS URL with .git",
			repoURL:  "https://github.com/owner/repo.git",
			expected: RepoGitHub,
		},
		{
			name:     "GitHub HTTPS URL with trailing slash",
			repoURL:  "https://github.com/owner/repo/",
			expected: RepoGitHub,
		},
		{
			name:     "GitLab HTTPS URL",
			repoURL:  "https://gitlab.com/owner/repo",
			expected: RepoGitLab,
		},
		{
			name:     "GitLab HTTPS URL with .git",
			repoURL:  "https://gitlab.com/owner/repo.git",
			expected: RepoGitLab,
		},
		{
			name:        "Invalid URL - BitBucket",
			repoURL:     "https://bitbucket.org/owner/repo",
			expectError: true,
		},
		{
			name:        "Invalid URL - SSH GitHub",
			repoURL:     "git@github.com:owner/repo.git",
			expectError: true,
		},
		{
			name:        "Invalid URL - empty",
			repoURL:     "",
			expectError: true,
		},
		{
			name:        "Invalid URL - just domain",
			repoURL:     "https://github.com/",
			expectError: true,
		},
		{
			name:        "Invalid URL - no repo",
			repoURL:     "https://github.com/owner",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseRepositoryType(tt.repoURL)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestIsValidRegistryURL(t *testing.T) {
	tests := []struct {
		name     string
		repoURL  string
		expected bool
	}{
		{
			name:     "Valid GitHub URL",
			repoURL:  "https://github.com/owner/repo",
			expected: true,
		},
		{
			name:     "Valid GitLab URL",
			repoURL:  "https://gitlab.com/owner/repo",
			expected: true,
		},
		{
			name:     "Invalid URL - BitBucket",
			repoURL:  "https://bitbucket.org/owner/repo",
			expected: false,
		},
		{
			name:     "Invalid URL - SSH",
			repoURL:  "git@github.com:owner/repo.git",
			expected: false,
		},
		{
			name:     "Invalid URL - empty",
			repoURL:  "",
			expected: false,
		},
		{
			name:     "Invalid URL - HTTP instead of HTTPS",
			repoURL:  "http://github.com/owner/repo",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidRegistryURL(tt.repoURL)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractRepoInfo(t *testing.T) {
	tests := []struct {
		name          string
		repoURL       string
		expectedOwner string
		expectedRepo  string
		expectError   bool
	}{
		{
			name:          "GitHub HTTPS URL",
			repoURL:       "https://github.com/HMB-research/open-accounting",
			expectedOwner: "HMB-research",
			expectedRepo:  "open-accounting",
		},
		{
			name:          "GitHub HTTPS URL with .git",
			repoURL:       "https://github.com/HMB-research/open-accounting.git",
			expectedOwner: "HMB-research",
			expectedRepo:  "open-accounting",
		},
		{
			name:          "GitHub HTTPS URL with trailing slash",
			repoURL:       "https://github.com/HMB-research/open-accounting/",
			expectedOwner: "HMB-research",
			expectedRepo:  "open-accounting",
		},
		{
			name:          "GitLab HTTPS URL",
			repoURL:       "https://gitlab.com/mygroup/myproject",
			expectedOwner: "mygroup",
			expectedRepo:  "myproject",
		},
		{
			name:          "GitLab HTTPS URL with .git",
			repoURL:       "https://gitlab.com/mygroup/myproject.git",
			expectedOwner: "mygroup",
			expectedRepo:  "myproject",
		},
		{
			name:        "Invalid URL - BitBucket",
			repoURL:     "https://bitbucket.org/owner/repo",
			expectError: true,
		},
		{
			name:        "Invalid URL - SSH",
			repoURL:     "git@github.com:owner/repo.git",
			expectError: true,
		},
		{
			name:        "Invalid URL - empty",
			repoURL:     "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, err := extractRepoInfo(tt.repoURL)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedOwner, owner)
				assert.Equal(t, tt.expectedRepo, repo)
			}
		})
	}
}

func TestGitHubHTTPSRegex(t *testing.T) {
	tests := []struct {
		url     string
		matches bool
	}{
		{"https://github.com/owner/repo", true},
		{"https://github.com/owner/repo.git", true},
		{"https://github.com/owner/repo/", true},
		{"https://github.com/owner-name/repo-name", true},
		{"https://github.com/owner123/repo456", true},
		{"https://github.com/a/b", true},
		{"http://github.com/owner/repo", false},
		{"https://github.com/owner", false},
		{"https://github.com/", false},
		{"https://gitlab.com/owner/repo", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			result := githubHTTPSRegex.MatchString(tt.url)
			assert.Equal(t, tt.matches, result)
		})
	}
}

func TestGitLabHTTPSRegex(t *testing.T) {
	tests := []struct {
		url     string
		matches bool
	}{
		{"https://gitlab.com/owner/repo", true},
		{"https://gitlab.com/owner/repo.git", true},
		{"https://gitlab.com/owner/repo/", true},
		{"https://gitlab.com/group-name/project-name", true},
		{"http://gitlab.com/owner/repo", false},
		{"https://gitlab.com/owner", false},
		{"https://gitlab.com/", false},
		{"https://github.com/owner/repo", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			result := gitlabHTTPSRegex.MatchString(tt.url)
			assert.Equal(t, tt.matches, result)
		})
	}
}

func TestMatchesSearch(t *testing.T) {
	tests := []struct {
		name     string
		plugin   PluginInfo
		query    string
		expected bool
	}{
		{
			name: "Match by name",
			plugin: PluginInfo{
				Name:        "invoice-helper",
				DisplayName: "Invoice Helper",
				Description: "Helps with invoices",
				Tags:        []string{"invoicing", "automation"},
			},
			query:    "invoice",
			expected: true,
		},
		{
			name: "Match by display name",
			plugin: PluginInfo{
				Name:        "my-plugin",
				DisplayName: "Awesome Invoice Tool",
				Description: "Does cool stuff",
				Tags:        []string{},
			},
			query:    "invoice",
			expected: true,
		},
		{
			name: "Match by description",
			plugin: PluginInfo{
				Name:        "helper",
				DisplayName: "Helper",
				Description: "A tool for managing invoices",
				Tags:        []string{},
			},
			query:    "invoice",
			expected: true,
		},
		{
			name: "Match by tag",
			plugin: PluginInfo{
				Name:        "tool",
				DisplayName: "Generic Tool",
				Description: "Does generic things",
				Tags:        []string{"accounting", "invoicing"},
			},
			query:    "invoicing",
			expected: true,
		},
		{
			name: "Case insensitive match",
			plugin: PluginInfo{
				Name:        "UPPERCASE-PLUGIN",
				DisplayName: "Uppercase Plugin",
				Description: "Test plugin",
				Tags:        []string{},
			},
			query:    "uppercase",
			expected: true,
		},
		{
			name: "No match",
			plugin: PluginInfo{
				Name:        "accounting-tool",
				DisplayName: "Accounting Tool",
				Description: "For accounting",
				Tags:        []string{"accounting"},
			},
			query:    "invoicing",
			expected: false,
		},
		{
			name: "Empty query matches all",
			plugin: PluginInfo{
				Name:        "any-plugin",
				DisplayName: "Any Plugin",
				Description: "Description",
				Tags:        []string{},
			},
			query:    "",
			expected: true,
		},
		{
			name: "Partial match in name",
			plugin: PluginInfo{
				Name:        "invoice-generator",
				DisplayName: "Generator",
				Description: "Generates things",
				Tags:        []string{},
			},
			query:    "gener",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesSearch(tt.plugin, tt.query)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasLicenseFile(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Test with no license file
	assert.False(t, hasLicenseFile(tmpDir), "Should return false for directory without license")

	// Test with LICENSE file
	licenseTests := []string{
		"LICENSE",
		"LICENSE.txt",
		"LICENSE.md",
		"LICENSE.MIT",
		"LICENSE.Apache",
		"COPYING",
		"COPYING.txt",
	}

	for _, licenseName := range licenseTests {
		t.Run(licenseName, func(t *testing.T) {
			// Create a new temp dir for each test
			testDir := t.TempDir()

			// Create the license file
			licensePath := testDir + "/" + licenseName
			err := os.WriteFile(licensePath, []byte("MIT License"), 0600)
			require.NoError(t, err)

			assert.True(t, hasLicenseFile(testDir), "Should return true for directory with %s", licenseName)
		})
	}
}

func TestHasLicenseFile_NonExistentDirectory(t *testing.T) {
	// Test with non-existent directory
	result := hasLicenseFile("/non/existent/path/that/does/not/exist")
	assert.False(t, result, "Should return false for non-existent directory")
}

// TestService_RemovePluginFiles tests the removePluginFiles function
func TestService_RemovePluginFiles(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()
	pluginDir := tmpDir + "/plugins"
	err := os.MkdirAll(pluginDir, 0750)
	require.NoError(t, err)

	tests := []struct {
		name        string
		setup       func() string // returns plugin name
		pluginName  string
		expectError bool
	}{
		{
			name: "success_remove_plugin",
			setup: func() string {
				// Create plugin directory with manifest
				pluginPath := pluginDir + "/test-owner-test-plugin"
				err := os.MkdirAll(pluginPath, 0750)
				require.NoError(t, err)
				manifestContent := `name: test-plugin
display_name: Test Plugin
version: 1.0.0`
				err = os.WriteFile(pluginPath+"/plugin.yaml", []byte(manifestContent), 0600)
				require.NoError(t, err)
				return "test-plugin"
			},
			pluginName:  "test-plugin",
			expectError: false,
		},
		{
			name: "plugin_not_found",
			setup: func() string {
				return "nonexistent-plugin"
			},
			pluginName:  "nonexistent-plugin",
			expectError: true,
		},
		{
			name: "skip_non_directory_entries",
			setup: func() string {
				// Create a file instead of directory
				err := os.WriteFile(pluginDir+"/not-a-dir.txt", []byte("content"), 0600)
				require.NoError(t, err)
				return "not-a-dir"
			},
			pluginName:  "not-a-dir",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Fresh plugin directory for each test
			testPluginDir := t.TempDir() + "/plugins"
			err := os.MkdirAll(testPluginDir, 0755)
			require.NoError(t, err)

			repo := NewMockRepository()
			service := NewServiceWithRepository(repo, nil, testPluginDir)

			// Setup test case
			if tt.setup != nil {
				// Create plugin directory with manifest for success case
				if tt.name == "success_remove_plugin" {
					pluginPath := testPluginDir + "/test-owner-test-plugin"
					err := os.MkdirAll(pluginPath, 0755)
					require.NoError(t, err)
					manifestContent := `name: test-plugin
display_name: Test Plugin
version: 1.0.0`
					err = os.WriteFile(pluginPath+"/plugin.yaml", []byte(manifestContent), 0600)
					require.NoError(t, err)
				} else if tt.name == "skip_non_directory_entries" {
					err := os.WriteFile(testPluginDir+"/not-a-dir.txt", []byte("content"), 0600)
					require.NoError(t, err)
				}
			}

			err = service.removePluginFiles(tt.pluginName)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// Verify directory was removed
				_, statErr := os.Stat(testPluginDir + "/test-owner-test-plugin")
				assert.True(t, os.IsNotExist(statErr), "Plugin directory should be removed")
			}
		})
	}
}

// TestService_RemovePluginFiles_InvalidManifest tests removePluginFiles with invalid manifest
func TestService_RemovePluginFiles_InvalidManifest(t *testing.T) {
	tmpDir := t.TempDir()
	pluginDir := tmpDir + "/plugins"
	err := os.MkdirAll(pluginDir, 0755)
	require.NoError(t, err)

	// Create plugin directory with invalid manifest
	pluginPath := pluginDir + "/invalid-plugin"
	err = os.MkdirAll(pluginPath, 0755)
	require.NoError(t, err)
	err = os.WriteFile(pluginPath+"/plugin.yaml", []byte("invalid: yaml: content"), 0600)
	require.NoError(t, err)

	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil, pluginDir)

	// Should return error since no valid plugin found
	err = service.removePluginFiles("some-plugin")
	assert.Error(t, err)
}

// TestService_RemovePluginFiles_EmptyPluginDir tests removePluginFiles with non-existent plugin dir
func TestService_RemovePluginFiles_EmptyPluginDir(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil, "/nonexistent/path")

	err := service.removePluginFiles("any-plugin")
	assert.Error(t, err)
}

// TestService_GetPluginPath tests the getPluginPath function
func TestService_GetPluginPath(t *testing.T) {
	tmpDir := t.TempDir()
	pluginDir := tmpDir + "/plugins"
	err := os.MkdirAll(pluginDir, 0755)
	require.NoError(t, err)

	// Create plugin directory with valid manifest
	pluginPath := pluginDir + "/owner-my-plugin"
	err = os.MkdirAll(pluginPath, 0755)
	require.NoError(t, err)
	manifestContent := `name: my-plugin
display_name: My Plugin
version: 1.0.0`
	err = os.WriteFile(pluginPath+"/plugin.yaml", []byte(manifestContent), 0600)
	require.NoError(t, err)

	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil, pluginDir)

	tests := []struct {
		name       string
		pluginName string
		expectPath string
	}{
		{
			name:       "found",
			pluginName: "my-plugin",
			expectPath: pluginPath,
		},
		{
			name:       "not_found",
			pluginName: "nonexistent-plugin",
			expectPath: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.getPluginPath(tt.pluginName)
			assert.Equal(t, tt.expectPath, result)
		})
	}
}

// TestService_GetPluginPath_NonExistentDir tests getPluginPath with non-existent directory
func TestService_GetPluginPath_NonExistentDir(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil, "/nonexistent/path")

	result := service.getPluginPath("any-plugin")
	assert.Equal(t, "", result)
}

// TestService_GetPluginPath_InvalidManifest tests getPluginPath skipping invalid manifests
func TestService_GetPluginPath_InvalidManifest(t *testing.T) {
	tmpDir := t.TempDir()
	pluginDir := tmpDir + "/plugins"
	err := os.MkdirAll(pluginDir, 0755)
	require.NoError(t, err)

	// Create plugin directory with invalid manifest
	invalidPath := pluginDir + "/invalid-plugin"
	err = os.MkdirAll(invalidPath, 0755)
	require.NoError(t, err)
	err = os.WriteFile(invalidPath+"/plugin.yaml", []byte("not valid yaml: ["), 0600)
	require.NoError(t, err)

	// Create another with valid manifest
	validPath := pluginDir + "/valid-plugin"
	err = os.MkdirAll(validPath, 0755)
	require.NoError(t, err)
	err = os.WriteFile(validPath+"/plugin.yaml", []byte("name: valid-plugin\ndisplay_name: Valid\nversion: 1.0.0"), 0600)
	require.NoError(t, err)

	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil, pluginDir)

	// Should find the valid plugin
	result := service.getPluginPath("valid-plugin")
	assert.Equal(t, validPath, result)

	// Should not find invalid plugin
	result = service.getPluginPath("invalid-plugin")
	assert.Equal(t, "", result)
}

// TestService_GetPluginPath_SkipsFiles tests getPluginPath skipping non-directory entries
func TestService_GetPluginPath_SkipsFiles(t *testing.T) {
	tmpDir := t.TempDir()
	pluginDir := tmpDir + "/plugins"
	err := os.MkdirAll(pluginDir, 0755)
	require.NoError(t, err)

	// Create a file (not a directory)
	err = os.WriteFile(pluginDir+"/not-a-dir.txt", []byte("content"), 0600)
	require.NoError(t, err)

	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil, pluginDir)

	result := service.getPluginPath("not-a-dir")
	assert.Equal(t, "", result)
}

// TestParseRegistryIndex tests the parseRegistryIndex function
func TestParseRegistryIndex(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		expectError bool
	}{
		{
			name:        "empty_data",
			data:        []byte{},
			expectError: false, // parseManifestYAML currently returns nil
		},
		{
			name:        "valid_yaml",
			data:        []byte("version: 1\nplugins: []"),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var index RegistryIndex
			err := parseRegistryIndex(tt.data, &index)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestParseManifestYAML tests the parseManifestYAML helper
func TestParseManifestYAML(t *testing.T) {
	var result interface{}
	err := parseManifestYAML([]byte("test: value"), &result)
	// Currently parseManifestYAML returns nil (stub implementation)
	assert.NoError(t, err)
}

// TestService_FetchRegistryIndex_InvalidURL tests FetchRegistryIndex with invalid URL
func TestService_FetchRegistryIndex_InvalidURL(t *testing.T) {
	ctx := t.Context()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil, "/tmp/plugins")

	tests := []struct {
		name string
		url  string
	}{
		{
			name: "invalid_url_format",
			url:  "not-a-url",
		},
		{
			name: "unsupported_host",
			url:  "https://bitbucket.org/owner/repo",
		},
		{
			name: "invalid_github_path",
			url:  "https://github.com/",
		},
		{
			name: "invalid_gitlab_path",
			url:  "https://gitlab.com/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.FetchRegistryIndex(ctx, tt.url)
			assert.Error(t, err)
		})
	}
}

// TestService_SyncRegistry tests the SyncRegistry function
func TestService_SyncRegistry_RegistryNotFound(t *testing.T) {
	ctx := t.Context()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil, "/tmp/plugins")

	err := service.SyncRegistry(ctx, uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "registry not found")
}

// TestService_SearchPlugins tests the SearchPlugins function
func TestService_SearchPlugins_RepoError(t *testing.T) {
	ctx := t.Context()
	repo := NewMockRepository()
	repo.listRegistriesErr = fmt.Errorf("db error")
	service := NewServiceWithRepository(repo, nil, "/tmp/plugins")

	_, err := service.SearchPlugins(ctx, "test")
	assert.Error(t, err)
}

// TestService_SearchPlugins_NoActiveRegistries tests SearchPlugins with no active registries
func TestService_SearchPlugins_NoActiveRegistries(t *testing.T) {
	ctx := t.Context()
	repo := NewMockRepository()
	// Add inactive registry
	regID := uuid.New()
	repo.registries[regID] = &Registry{
		ID:       regID,
		Name:     "Inactive Registry",
		URL:      "https://github.com/test/registry",
		IsActive: false,
	}
	service := NewServiceWithRepository(repo, nil, "/tmp/plugins")

	results, err := service.SearchPlugins(ctx, "test")
	assert.NoError(t, err)
	assert.Empty(t, results)
}

// TestService_CloneRepository_InvalidURL tests cloneRepository with invalid URL
func TestService_CloneRepository_InvalidURL(t *testing.T) {
	ctx := t.Context()
	tmpDir := t.TempDir()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil, tmpDir)

	_, err := service.cloneRepository(ctx, "invalid-url")
	assert.Error(t, err)
}

// TestService_CloneRepository_CreateDirError tests cloneRepository when mkdir fails
func TestService_CloneRepository_CreateDirError(t *testing.T) {
	ctx := t.Context()
	repo := NewMockRepository()
	// Use a path that cannot be created (file instead of directory parent)
	tmpFile := t.TempDir() + "/file.txt"
	err := os.WriteFile(tmpFile, []byte("content"), 0600)
	require.NoError(t, err)

	service := NewServiceWithRepository(repo, nil, tmpFile+"/plugins")

	_, err = service.cloneRepository(ctx, "https://github.com/owner/repo")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create plugins directory")
}

// TestService_UpdateRepository_PluginNotFound tests updateRepository when plugin not found
func TestService_UpdateRepository_PluginNotFound(t *testing.T) {
	ctx := t.Context()
	tmpDir := t.TempDir()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil, tmpDir)

	err := service.updateRepository(ctx, "nonexistent-plugin")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "plugin not found in filesystem")
}
