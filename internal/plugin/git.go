package plugin

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Repository URL patterns
var (
	githubHTTPSRegex = regexp.MustCompile(`^https://github\.com/([^/]+)/([^/]+?)(?:\.git)?/?$`)
	gitlabHTTPSRegex = regexp.MustCompile(`^https://gitlab\.com/([^/]+)/([^/]+?)(?:\.git)?/?$`)
)

// parseRepositoryType determines the repository type from the URL
func parseRepositoryType(repoURL string) (RepositoryType, error) {
	if githubHTTPSRegex.MatchString(repoURL) {
		return RepoGitHub, nil
	}
	if gitlabHTTPSRegex.MatchString(repoURL) {
		return RepoGitLab, nil
	}
	return "", fmt.Errorf("invalid repository URL: only GitHub and GitLab HTTPS URLs are supported")
}

// isValidRegistryURL validates if a URL is a valid registry URL (GitHub/GitLab repo)
func isValidRegistryURL(repoURL string) bool {
	_, err := parseRepositoryType(repoURL)
	return err == nil
}

// extractRepoInfo extracts owner and repo name from a repository URL
func extractRepoInfo(repoURL string) (owner, repo string, err error) {
	if matches := githubHTTPSRegex.FindStringSubmatch(repoURL); matches != nil {
		return matches[1], strings.TrimSuffix(matches[2], ".git"), nil
	}
	if matches := gitlabHTTPSRegex.FindStringSubmatch(repoURL); matches != nil {
		return matches[1], strings.TrimSuffix(matches[2], ".git"), nil
	}
	return "", "", fmt.Errorf("could not extract repository info from URL")
}

// cloneRepository clones a plugin repository to the plugins directory
func (s *Service) cloneRepository(ctx context.Context, repoURL string) (string, error) {
	// Extract repo info for directory naming
	owner, repo, err := extractRepoInfo(repoURL)
	if err != nil {
		return "", err
	}

	// Create plugins directory if it doesn't exist
	if err := os.MkdirAll(s.pluginDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create plugins directory: %w", err)
	}

	// Target directory
	targetDir := filepath.Join(s.pluginDir, fmt.Sprintf("%s-%s", owner, repo))

	// Remove existing directory if it exists
	if _, err := os.Stat(targetDir); err == nil {
		if err := os.RemoveAll(targetDir); err != nil {
			return "", fmt.Errorf("failed to remove existing plugin directory: %w", err)
		}
	}

	// Clone the repository
	log.Info().Str("url", repoURL).Str("target", targetDir).Msg("Cloning plugin repository")

	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", repoURL, targetDir)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git clone failed: %s: %w", string(output), err)
	}

	// Verify plugin.yaml exists
	manifestPath := filepath.Join(targetDir, "plugin.yaml")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		// Clean up
		os.RemoveAll(targetDir)
		return "", fmt.Errorf("repository does not contain a plugin.yaml file")
	}

	// Verify LICENSE file exists
	if !hasLicenseFile(targetDir) {
		// Clean up
		os.RemoveAll(targetDir)
		return "", fmt.Errorf("repository does not contain a LICENSE file (open source license required)")
	}

	return targetDir, nil
}

// updateRepository pulls the latest changes for a plugin
// nolint:unused // Reserved for UpdatePlugin implementation
func (s *Service) updateRepository(ctx context.Context, pluginName string) error {
	pluginPath := s.getPluginPath(pluginName)
	if pluginPath == "" {
		return fmt.Errorf("plugin not found in filesystem")
	}

	cmd := exec.CommandContext(ctx, "git", "-C", pluginPath, "pull", "--ff-only")
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git pull failed: %s: %w", string(output), err)
	}

	return nil
}

// removePluginFiles removes the plugin files from the filesystem
func (s *Service) removePluginFiles(pluginName string) error {
	// Find the plugin directory
	entries, err := os.ReadDir(s.pluginDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check if this directory contains the plugin
		manifestPath := filepath.Join(s.pluginDir, entry.Name(), "plugin.yaml")
		manifest, err := LoadManifest(manifestPath)
		if err != nil {
			continue
		}

		if manifest.Name == pluginName {
			return os.RemoveAll(filepath.Join(s.pluginDir, entry.Name()))
		}
	}

	return fmt.Errorf("plugin directory not found")
}

// getPluginPath returns the filesystem path for a plugin
// nolint:unused // Reserved for UpdatePlugin implementation
func (s *Service) getPluginPath(pluginName string) string {
	entries, err := os.ReadDir(s.pluginDir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		manifestPath := filepath.Join(s.pluginDir, entry.Name(), "plugin.yaml")
		manifest, err := LoadManifest(manifestPath)
		if err != nil {
			continue
		}

		if manifest.Name == pluginName {
			return filepath.Join(s.pluginDir, entry.Name())
		}
	}

	return ""
}

// hasLicenseFile checks if the directory contains a LICENSE file
func hasLicenseFile(dir string) bool {
	licenseFiles := []string{
		"LICENSE",
		"LICENSE.txt",
		"LICENSE.md",
		"LICENSE.MIT",
		"LICENSE.Apache",
		"COPYING",
		"COPYING.txt",
	}

	for _, name := range licenseFiles {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}

	return false
}

// FetchRegistryIndex fetches the plugins.yaml index from a registry
func (s *Service) FetchRegistryIndex(ctx context.Context, registryURL string) (*RegistryIndex, error) {
	// Parse the registry URL
	u, err := url.Parse(registryURL)
	if err != nil {
		return nil, fmt.Errorf("invalid registry URL: %w", err)
	}

	// Determine raw content URL based on host
	var rawURL string
	switch u.Host {
	case "github.com":
		// Convert github.com/owner/repo to raw.githubusercontent.com/owner/repo/main/plugins.yaml
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid GitHub URL format")
		}
		rawURL = fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/plugins.yaml", parts[0], parts[1])
	case "gitlab.com":
		// Convert gitlab.com/owner/repo to gitlab.com/owner/repo/-/raw/main/plugins.yaml
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid GitLab URL format")
		}
		rawURL = fmt.Sprintf("https://gitlab.com/%s/%s/-/raw/main/plugins.yaml", parts[0], parts[1])
	default:
		return nil, fmt.Errorf("unsupported registry host: %s", u.Host)
	}

	// Fetch the file using curl (simpler than adding http client)
	cmd := exec.CommandContext(ctx, "curl", "-sSfL", rawURL)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch registry index: %w", err)
	}

	// Parse the index
	var index RegistryIndex
	if err := parseRegistryIndex(output, &index); err != nil {
		return nil, fmt.Errorf("failed to parse registry index: %w", err)
	}

	return &index, nil
}

// parseRegistryIndex parses the YAML content of a registry index
func parseRegistryIndex(data []byte, index *RegistryIndex) error {
	return parseManifestYAML(data, index)
}

// parseManifestYAML is a helper to parse YAML content
func parseManifestYAML(data []byte, v interface{}) error {
	// Use the yaml package from manifest.go
	return nil // This will be implemented using gopkg.in/yaml.v3
}

// SyncRegistry fetches and caches the plugin list from a registry
func (s *Service) SyncRegistry(ctx context.Context, registryID uuid.UUID) error {
	registry, err := s.GetRegistry(ctx, registryID)
	if err != nil {
		return err
	}

	index, err := s.FetchRegistryIndex(ctx, registry.URL)
	if err != nil {
		return fmt.Errorf("failed to sync registry %s: %w", registry.Name, err)
	}

	// Update last synced timestamp
	if err := s.UpdateRegistryLastSynced(ctx, registryID); err != nil {
		log.Warn().Err(err).Str("registry", registry.Name).Msg("Failed to update last synced")
	}

	log.Info().
		Str("registry", registry.Name).
		Int("plugins", len(index.Plugins)).
		Msg("Synced registry")

	return nil
}

// SearchPlugins searches for plugins across all active registries
func (s *Service) SearchPlugins(ctx context.Context, query string) ([]PluginSearchResult, error) {
	registries, err := s.ListRegistries(ctx)
	if err != nil {
		return nil, err
	}

	var results []PluginSearchResult
	query = strings.ToLower(query)

	for _, registry := range registries {
		if !registry.IsActive {
			continue
		}

		index, err := s.FetchRegistryIndex(ctx, registry.URL)
		if err != nil {
			log.Warn().Err(err).Str("registry", registry.Name).Msg("Failed to fetch registry")
			continue
		}

		for _, plugin := range index.Plugins {
			// Search in name, display name, description, and tags
			if matchesSearch(plugin, query) {
				results = append(results, PluginSearchResult{
					Plugin:   plugin,
					Registry: registry.Name,
				})
			}
		}
	}

	return results, nil
}

// matchesSearch checks if a plugin matches the search query
func matchesSearch(plugin PluginInfo, query string) bool {
	if strings.Contains(strings.ToLower(plugin.Name), query) {
		return true
	}
	if strings.Contains(strings.ToLower(plugin.DisplayName), query) {
		return true
	}
	if strings.Contains(strings.ToLower(plugin.Description), query) {
		return true
	}
	for _, tag := range plugin.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			return true
		}
	}
	return false
}
