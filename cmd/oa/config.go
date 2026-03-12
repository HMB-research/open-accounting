package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type cliConfig struct {
	BaseURL    string `json:"base_url"`
	TenantID   string `json:"tenant_id"`
	TenantName string `json:"tenant_name,omitempty"`
	TenantSlug string `json:"tenant_slug,omitempty"`
	APIToken   string `json:"api_token"`
}

func defaultBaseURL() string {
	if value := strings.TrimSpace(os.Getenv("OA_BASE_URL")); value != "" {
		return normalizeBaseURL(value)
	}
	return "http://localhost:8080"
}

func configPath() (string, error) {
	baseDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(baseDir, "open-accounting", "config.json"), nil
}

func loadStoredConfig() (*cliConfig, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg cliConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	cfg.BaseURL = normalizeBaseURL(cfg.BaseURL)
	return &cfg, nil
}

func loadRuntimeConfig() (*cliConfig, error) {
	cfg, err := loadStoredConfig()
	if err != nil {
		if os.IsNotExist(err) {
			cfg = &cliConfig{}
		} else {
			return nil, err
		}
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL()
	}
	if value := strings.TrimSpace(os.Getenv("OA_BASE_URL")); value != "" {
		cfg.BaseURL = normalizeBaseURL(value)
	}
	if value := strings.TrimSpace(os.Getenv("OA_API_TOKEN")); value != "" {
		cfg.APIToken = value
	}
	if value := strings.TrimSpace(os.Getenv("OA_TENANT_ID")); value != "" {
		cfg.TenantID = value
	}

	return cfg, nil
}

func saveConfig(cfg *cliConfig) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func deleteConfig() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove config: %w", err)
	}
	return nil
}

func normalizeBaseURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimRight(trimmed, "/")
	if trimmed == "" {
		return defaultBaseURL()
	}
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return trimmed
	}
	return "https://" + trimmed
}
