package email

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Repository defines the contract for email data access
type Repository interface {
	// Schema management
	EnsureSchema(ctx context.Context, schemaName string) error

	// SMTP Config operations
	GetTenantSettings(ctx context.Context, tenantID string) ([]byte, error)
	UpdateTenantSettings(ctx context.Context, tenantID string, settingsJSON []byte) error

	// Template operations
	GetTemplate(ctx context.Context, schemaName, tenantID string, templateType TemplateType) (*EmailTemplate, error)
	ListTemplates(ctx context.Context, schemaName, tenantID string) ([]EmailTemplate, error)
	UpsertTemplate(ctx context.Context, schemaName string, template *EmailTemplate) error

	// Email log operations
	CreateEmailLog(ctx context.Context, schemaName string, log *EmailLog) error
	UpdateEmailLogStatus(ctx context.Context, schemaName, logID string, status EmailStatus, sentAt *time.Time, errorMessage string) error
	GetEmailLog(ctx context.Context, schemaName, tenantID string, limit int) ([]EmailLog, error)
}

// Common errors
var (
	ErrTemplateNotFound = fmt.Errorf("template not found")
	ErrSettingsNotFound = fmt.Errorf("settings not found")
)

// ParseSMTPConfig parses SMTP config from settings JSON
func ParseSMTPConfig(settingsJSON []byte) (*SMTPConfig, error) {
	var settings map[string]interface{}
	if err := json.Unmarshal(settingsJSON, &settings); err != nil {
		return nil, fmt.Errorf("failed to parse settings: %w", err)
	}

	config := &SMTPConfig{
		Port:   587,
		UseTLS: true,
	}

	if host, ok := settings["smtp_host"].(string); ok {
		config.Host = host
	}
	if port, ok := settings["smtp_port"].(float64); ok {
		config.Port = int(port)
	}
	if username, ok := settings["smtp_username"].(string); ok {
		config.Username = username
	}
	if password, ok := settings["smtp_password"].(string); ok {
		config.Password = password
	}
	if fromEmail, ok := settings["smtp_from_email"].(string); ok {
		config.FromEmail = fromEmail
	}
	if fromName, ok := settings["smtp_from_name"].(string); ok {
		config.FromName = fromName
	}
	if useTLS, ok := settings["smtp_use_tls"].(bool); ok {
		config.UseTLS = useTLS
	}

	return config, nil
}

// MergeSMTPConfig merges SMTP config into existing settings JSON
func MergeSMTPConfig(existingJSON []byte, req *UpdateSMTPConfigRequest) ([]byte, error) {
	var settings map[string]interface{}
	if err := json.Unmarshal(existingJSON, &settings); err != nil {
		settings = make(map[string]interface{})
	}

	settings["smtp_host"] = req.Host
	settings["smtp_port"] = req.Port
	settings["smtp_username"] = req.Username
	if req.Password != "" {
		settings["smtp_password"] = req.Password
	}
	settings["smtp_from_email"] = req.FromEmail
	settings["smtp_from_name"] = req.FromName
	settings["smtp_use_tls"] = req.UseTLS

	return json.Marshal(settings)
}
