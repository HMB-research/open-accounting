package plugin

import (
	"testing"

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
