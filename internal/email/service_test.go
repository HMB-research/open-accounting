package email

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wneessen/go-mail"
)

// MockRepository implements Repository for testing
type MockRepository struct {
	// Function fields for mocking behavior
	EnsureSchemaFn         func(ctx context.Context, schemaName string) error
	GetTenantSettingsFn    func(ctx context.Context, tenantID string) ([]byte, error)
	UpdateTenantSettingsFn func(ctx context.Context, tenantID string, settingsJSON []byte) error
	GetTemplateFn          func(ctx context.Context, schemaName, tenantID string, templateType TemplateType) (*EmailTemplate, error)
	ListTemplatesFn        func(ctx context.Context, schemaName, tenantID string) ([]EmailTemplate, error)
	UpsertTemplateFn       func(ctx context.Context, schemaName string, template *EmailTemplate) error
	CreateEmailLogFn       func(ctx context.Context, schemaName string, log *EmailLog) error
	UpdateEmailLogStatusFn func(ctx context.Context, schemaName, logID string, status EmailStatus, sentAt *time.Time, errorMessage string) error
	GetEmailLogFn          func(ctx context.Context, schemaName, tenantID string, limit int) ([]EmailLog, error)

	// Track calls for assertions
	EnsureSchemaCalled         bool
	GetTenantSettingsCalled    bool
	UpdateTenantSettingsCalled bool
	GetTemplateCalled          bool
	ListTemplatesCalled        bool
	UpsertTemplateCalled       bool
	CreateEmailLogCalled       bool
	UpdateEmailLogStatusCalled bool
	GetEmailLogCalled          bool
}

func (m *MockRepository) EnsureSchema(ctx context.Context, schemaName string) error {
	m.EnsureSchemaCalled = true
	if m.EnsureSchemaFn != nil {
		return m.EnsureSchemaFn(ctx, schemaName)
	}
	return nil
}

func (m *MockRepository) GetTenantSettings(ctx context.Context, tenantID string) ([]byte, error) {
	m.GetTenantSettingsCalled = true
	if m.GetTenantSettingsFn != nil {
		return m.GetTenantSettingsFn(ctx, tenantID)
	}
	return nil, nil
}

func (m *MockRepository) UpdateTenantSettings(ctx context.Context, tenantID string, settingsJSON []byte) error {
	m.UpdateTenantSettingsCalled = true
	if m.UpdateTenantSettingsFn != nil {
		return m.UpdateTenantSettingsFn(ctx, tenantID, settingsJSON)
	}
	return nil
}

func (m *MockRepository) GetTemplate(ctx context.Context, schemaName, tenantID string, templateType TemplateType) (*EmailTemplate, error) {
	m.GetTemplateCalled = true
	if m.GetTemplateFn != nil {
		return m.GetTemplateFn(ctx, schemaName, tenantID, templateType)
	}
	return nil, ErrTemplateNotFound
}

func (m *MockRepository) ListTemplates(ctx context.Context, schemaName, tenantID string) ([]EmailTemplate, error) {
	m.ListTemplatesCalled = true
	if m.ListTemplatesFn != nil {
		return m.ListTemplatesFn(ctx, schemaName, tenantID)
	}
	return []EmailTemplate{}, nil
}

func (m *MockRepository) UpsertTemplate(ctx context.Context, schemaName string, template *EmailTemplate) error {
	m.UpsertTemplateCalled = true
	if m.UpsertTemplateFn != nil {
		return m.UpsertTemplateFn(ctx, schemaName, template)
	}
	return nil
}

func (m *MockRepository) CreateEmailLog(ctx context.Context, schemaName string, log *EmailLog) error {
	m.CreateEmailLogCalled = true
	if m.CreateEmailLogFn != nil {
		return m.CreateEmailLogFn(ctx, schemaName, log)
	}
	return nil
}

func (m *MockRepository) UpdateEmailLogStatus(ctx context.Context, schemaName, logID string, status EmailStatus, sentAt *time.Time, errorMessage string) error {
	m.UpdateEmailLogStatusCalled = true
	if m.UpdateEmailLogStatusFn != nil {
		return m.UpdateEmailLogStatusFn(ctx, schemaName, logID, status, sentAt, errorMessage)
	}
	return nil
}

func (m *MockRepository) GetEmailLog(ctx context.Context, schemaName, tenantID string, limit int) ([]EmailLog, error) {
	m.GetEmailLogCalled = true
	if m.GetEmailLogFn != nil {
		return m.GetEmailLogFn(ctx, schemaName, tenantID, limit)
	}
	return []EmailLog{}, nil
}

// MockMailSender implements MailSender for testing
type MockMailSender struct {
	SendMailFn     func(config *SMTPConfig, m *mail.Msg) error
	SendMailCalled bool
	LastConfig     *SMTPConfig
}

func (m *MockMailSender) SendMail(config *SMTPConfig, msg *mail.Msg) error {
	m.SendMailCalled = true
	m.LastConfig = config
	if m.SendMailFn != nil {
		return m.SendMailFn(config, msg)
	}
	return nil
}

// Helper to create valid SMTP settings JSON
func validSMTPSettingsJSON() []byte {
	settings := map[string]interface{}{
		"smtp_host":       "smtp.example.com",
		"smtp_port":       float64(587),
		"smtp_username":   "user@example.com",
		"smtp_password":   "password",
		"smtp_from_email": "noreply@example.com",
		"smtp_from_name":  "Test Company",
		"smtp_use_tls":    true,
	}
	data, _ := json.Marshal(settings)
	return data
}

// Helper to create empty settings JSON
func emptySMTPSettingsJSON() []byte {
	settings := map[string]interface{}{}
	data, _ := json.Marshal(settings)
	return data
}

func TestService_EnsureSchema(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := &MockRepository{}
		svc := NewServiceWithRepository(repo, &MockMailSender{})

		err := svc.EnsureSchema(context.Background(), "tenant_test")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !repo.EnsureSchemaCalled {
			t.Error("expected EnsureSchema to be called")
		}
	})

	t.Run("repository error", func(t *testing.T) {
		repo := &MockRepository{
			EnsureSchemaFn: func(ctx context.Context, schemaName string) error {
				return errors.New("db error")
			},
		}
		svc := NewServiceWithRepository(repo, &MockMailSender{})

		err := svc.EnsureSchema(context.Background(), "tenant_test")

		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestService_GetSMTPConfig(t *testing.T) {
	t.Run("success with valid config", func(t *testing.T) {
		repo := &MockRepository{
			GetTenantSettingsFn: func(ctx context.Context, tenantID string) ([]byte, error) {
				return validSMTPSettingsJSON(), nil
			},
		}
		svc := NewServiceWithRepository(repo, &MockMailSender{})

		config, err := svc.GetSMTPConfig(context.Background(), "tenant-1")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if config.Host != "smtp.example.com" {
			t.Errorf("Host = %q, want %q", config.Host, "smtp.example.com")
		}
		if config.Port != 587 {
			t.Errorf("Port = %d, want %d", config.Port, 587)
		}
		if config.FromEmail != "noreply@example.com" {
			t.Errorf("FromEmail = %q, want %q", config.FromEmail, "noreply@example.com")
		}
	})

	t.Run("settings not found", func(t *testing.T) {
		repo := &MockRepository{
			GetTenantSettingsFn: func(ctx context.Context, tenantID string) ([]byte, error) {
				return nil, ErrSettingsNotFound
			},
		}
		svc := NewServiceWithRepository(repo, &MockMailSender{})

		_, err := svc.GetSMTPConfig(context.Background(), "tenant-1")

		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("empty settings returns defaults", func(t *testing.T) {
		repo := &MockRepository{
			GetTenantSettingsFn: func(ctx context.Context, tenantID string) ([]byte, error) {
				return emptySMTPSettingsJSON(), nil
			},
		}
		svc := NewServiceWithRepository(repo, &MockMailSender{})

		config, err := svc.GetSMTPConfig(context.Background(), "tenant-1")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		// Default port should be 587 and UseTLS should be true
		if config.Port != 587 {
			t.Errorf("Port = %d, want %d", config.Port, 587)
		}
		if !config.UseTLS {
			t.Error("UseTLS should default to true")
		}
	})
}

func TestService_UpdateSMTPConfig(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var capturedSettings []byte
		repo := &MockRepository{
			GetTenantSettingsFn: func(ctx context.Context, tenantID string) ([]byte, error) {
				return emptySMTPSettingsJSON(), nil
			},
			UpdateTenantSettingsFn: func(ctx context.Context, tenantID string, settingsJSON []byte) error {
				capturedSettings = settingsJSON
				return nil
			},
		}
		svc := NewServiceWithRepository(repo, &MockMailSender{})

		req := &UpdateSMTPConfigRequest{
			Host:      "new.smtp.com",
			Port:      465,
			Username:  "user",
			Password:  "pass",
			FromEmail: "new@example.com",
			FromName:  "New Sender",
			UseTLS:    true,
		}

		err := svc.UpdateSMTPConfig(context.Background(), "tenant-1", req)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !repo.UpdateTenantSettingsCalled {
			t.Error("expected UpdateTenantSettings to be called")
		}

		// Verify the settings were properly merged
		var savedSettings map[string]interface{}
		if err := json.Unmarshal(capturedSettings, &savedSettings); err != nil {
			t.Fatalf("failed to unmarshal saved settings: %v", err)
		}
		if savedSettings["smtp_host"] != "new.smtp.com" {
			t.Errorf("smtp_host = %v, want %v", savedSettings["smtp_host"], "new.smtp.com")
		}
	})

	t.Run("get settings error", func(t *testing.T) {
		repo := &MockRepository{
			GetTenantSettingsFn: func(ctx context.Context, tenantID string) ([]byte, error) {
				return nil, errors.New("db error")
			},
		}
		svc := NewServiceWithRepository(repo, &MockMailSender{})

		err := svc.UpdateSMTPConfig(context.Background(), "tenant-1", &UpdateSMTPConfigRequest{})

		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("update settings error", func(t *testing.T) {
		repo := &MockRepository{
			GetTenantSettingsFn: func(ctx context.Context, tenantID string) ([]byte, error) {
				return emptySMTPSettingsJSON(), nil
			},
			UpdateTenantSettingsFn: func(ctx context.Context, tenantID string, settingsJSON []byte) error {
				return errors.New("db error")
			},
		}
		svc := NewServiceWithRepository(repo, &MockMailSender{})

		err := svc.UpdateSMTPConfig(context.Background(), "tenant-1", &UpdateSMTPConfigRequest{})

		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestService_TestSMTP(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mailer := &MockMailSender{}
		repo := &MockRepository{
			GetTenantSettingsFn: func(ctx context.Context, tenantID string) ([]byte, error) {
				return validSMTPSettingsJSON(), nil
			},
		}
		svc := NewServiceWithRepository(repo, mailer)

		result, err := svc.TestSMTP(context.Background(), "tenant-1", "test@example.com")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !result.Success {
			t.Errorf("Success = false, want true, message: %s", result.Message)
		}
		if !mailer.SendMailCalled {
			t.Error("expected SendMail to be called")
		}
	})

	t.Run("smtp not configured", func(t *testing.T) {
		repo := &MockRepository{
			GetTenantSettingsFn: func(ctx context.Context, tenantID string) ([]byte, error) {
				return emptySMTPSettingsJSON(), nil
			},
		}
		svc := NewServiceWithRepository(repo, &MockMailSender{})

		result, err := svc.TestSMTP(context.Background(), "tenant-1", "test@example.com")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result.Success {
			t.Error("Success = true, want false when SMTP not configured")
		}
		if result.Message != "SMTP is not configured" {
			t.Errorf("Message = %q, want %q", result.Message, "SMTP is not configured")
		}
	})

	t.Run("send mail error", func(t *testing.T) {
		mailer := &MockMailSender{
			SendMailFn: func(config *SMTPConfig, m *mail.Msg) error {
				return errors.New("connection refused")
			},
		}
		repo := &MockRepository{
			GetTenantSettingsFn: func(ctx context.Context, tenantID string) ([]byte, error) {
				return validSMTPSettingsJSON(), nil
			},
		}
		svc := NewServiceWithRepository(repo, mailer)

		result, err := svc.TestSMTP(context.Background(), "tenant-1", "test@example.com")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result.Success {
			t.Error("Success = true, want false when send fails")
		}
	})
}

func TestService_GetTemplate(t *testing.T) {
	t.Run("template found", func(t *testing.T) {
		expectedTemplate := &EmailTemplate{
			ID:           "tmpl-1",
			TenantID:     "tenant-1",
			TemplateType: TemplateInvoiceSend,
			Subject:      "Custom Subject",
			BodyHTML:     "<p>Custom HTML</p>",
			IsActive:     true,
		}
		repo := &MockRepository{
			GetTemplateFn: func(ctx context.Context, schemaName, tenantID string, templateType TemplateType) (*EmailTemplate, error) {
				return expectedTemplate, nil
			},
		}
		svc := NewServiceWithRepository(repo, &MockMailSender{})

		tmpl, err := svc.GetTemplate(context.Background(), "tenant_test", "tenant-1", TemplateInvoiceSend)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if tmpl.Subject != "Custom Subject" {
			t.Errorf("Subject = %q, want %q", tmpl.Subject, "Custom Subject")
		}
	})

	t.Run("template not found returns default", func(t *testing.T) {
		repo := &MockRepository{
			GetTemplateFn: func(ctx context.Context, schemaName, tenantID string, templateType TemplateType) (*EmailTemplate, error) {
				return nil, ErrTemplateNotFound
			},
		}
		svc := NewServiceWithRepository(repo, &MockMailSender{})

		tmpl, err := svc.GetTemplate(context.Background(), "tenant_test", "tenant-1", TemplateInvoiceSend)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if tmpl.TenantID != "tenant-1" {
			t.Errorf("TenantID = %q, want %q", tmpl.TenantID, "tenant-1")
		}
		if tmpl.TemplateType != TemplateInvoiceSend {
			t.Errorf("TemplateType = %q, want %q", tmpl.TemplateType, TemplateInvoiceSend)
		}
	})

	t.Run("unknown template type not found returns error", func(t *testing.T) {
		repo := &MockRepository{
			GetTemplateFn: func(ctx context.Context, schemaName, tenantID string, templateType TemplateType) (*EmailTemplate, error) {
				return nil, ErrTemplateNotFound
			},
		}
		svc := NewServiceWithRepository(repo, &MockMailSender{})

		_, err := svc.GetTemplate(context.Background(), "tenant_test", "tenant-1", "UNKNOWN_TYPE")

		if err == nil {
			t.Error("expected error for unknown template type, got nil")
		}
	})

	t.Run("repository error", func(t *testing.T) {
		repo := &MockRepository{
			GetTemplateFn: func(ctx context.Context, schemaName, tenantID string, templateType TemplateType) (*EmailTemplate, error) {
				return nil, errors.New("db error")
			},
		}
		svc := NewServiceWithRepository(repo, &MockMailSender{})

		_, err := svc.GetTemplate(context.Background(), "tenant_test", "tenant-1", TemplateInvoiceSend)

		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestService_ListTemplates(t *testing.T) {
	t.Run("success with existing templates", func(t *testing.T) {
		existingTemplates := []EmailTemplate{
			{
				ID:           "tmpl-1",
				TenantID:     "tenant-1",
				TemplateType: TemplateInvoiceSend,
				Subject:      "Custom Invoice",
				BodyHTML:     "<p>Custom</p>",
				IsActive:     true,
			},
		}
		repo := &MockRepository{
			ListTemplatesFn: func(ctx context.Context, schemaName, tenantID string) ([]EmailTemplate, error) {
				return existingTemplates, nil
			},
		}
		svc := NewServiceWithRepository(repo, &MockMailSender{})

		templates, err := svc.ListTemplates(context.Background(), "tenant_test", "tenant-1")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		// Should have the existing template plus default templates for missing types
		if len(templates) != 3 { // 1 custom + 2 defaults (payment, overdue)
			t.Errorf("len(templates) = %d, want 3", len(templates))
		}
		if !repo.EnsureSchemaCalled {
			t.Error("expected EnsureSchema to be called")
		}
	})

	t.Run("no existing templates returns all defaults", func(t *testing.T) {
		repo := &MockRepository{
			ListTemplatesFn: func(ctx context.Context, schemaName, tenantID string) ([]EmailTemplate, error) {
				return []EmailTemplate{}, nil
			},
		}
		svc := NewServiceWithRepository(repo, &MockMailSender{})

		templates, err := svc.ListTemplates(context.Background(), "tenant_test", "tenant-1")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(templates) != 3 { // All defaults
			t.Errorf("len(templates) = %d, want 3", len(templates))
		}
	})

	t.Run("ensure schema error", func(t *testing.T) {
		repo := &MockRepository{
			EnsureSchemaFn: func(ctx context.Context, schemaName string) error {
				return errors.New("schema error")
			},
		}
		svc := NewServiceWithRepository(repo, &MockMailSender{})

		_, err := svc.ListTemplates(context.Background(), "tenant_test", "tenant-1")

		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("list templates error", func(t *testing.T) {
		repo := &MockRepository{
			ListTemplatesFn: func(ctx context.Context, schemaName, tenantID string) ([]EmailTemplate, error) {
				return nil, errors.New("db error")
			},
		}
		svc := NewServiceWithRepository(repo, &MockMailSender{})

		_, err := svc.ListTemplates(context.Background(), "tenant_test", "tenant-1")

		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestService_UpdateTemplate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := &MockRepository{
			UpsertTemplateFn: func(ctx context.Context, schemaName string, template *EmailTemplate) error {
				// Simulate the RETURNING behavior
				template.CreatedAt = time.Now()
				template.UpdatedAt = time.Now()
				return nil
			},
		}
		svc := NewServiceWithRepository(repo, &MockMailSender{})

		req := &UpdateTemplateRequest{
			Subject:  "New Subject",
			BodyHTML: "<p>New HTML</p>",
			BodyText: "New Text",
			IsActive: true,
		}

		tmpl, err := svc.UpdateTemplate(context.Background(), "tenant_test", "tenant-1", TemplateInvoiceSend, req)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if tmpl.Subject != "New Subject" {
			t.Errorf("Subject = %q, want %q", tmpl.Subject, "New Subject")
		}
		if !repo.EnsureSchemaCalled {
			t.Error("expected EnsureSchema to be called")
		}
		if !repo.UpsertTemplateCalled {
			t.Error("expected UpsertTemplate to be called")
		}
	})

	t.Run("ensure schema error", func(t *testing.T) {
		repo := &MockRepository{
			EnsureSchemaFn: func(ctx context.Context, schemaName string) error {
				return errors.New("schema error")
			},
		}
		svc := NewServiceWithRepository(repo, &MockMailSender{})

		_, err := svc.UpdateTemplate(context.Background(), "tenant_test", "tenant-1", TemplateInvoiceSend, &UpdateTemplateRequest{})

		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("upsert template error", func(t *testing.T) {
		repo := &MockRepository{
			UpsertTemplateFn: func(ctx context.Context, schemaName string, template *EmailTemplate) error {
				return errors.New("db error")
			},
		}
		svc := NewServiceWithRepository(repo, &MockMailSender{})

		_, err := svc.UpdateTemplate(context.Background(), "tenant_test", "tenant-1", TemplateInvoiceSend, &UpdateTemplateRequest{})

		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestService_SendEmail(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mailer := &MockMailSender{}
		var logCreated bool
		var statusUpdated bool
		var updatedStatus EmailStatus
		repo := &MockRepository{
			GetTenantSettingsFn: func(ctx context.Context, tenantID string) ([]byte, error) {
				return validSMTPSettingsJSON(), nil
			},
			CreateEmailLogFn: func(ctx context.Context, schemaName string, log *EmailLog) error {
				logCreated = true
				return nil
			},
			UpdateEmailLogStatusFn: func(ctx context.Context, schemaName, logID string, status EmailStatus, sentAt *time.Time, errorMessage string) error {
				statusUpdated = true
				updatedStatus = status
				return nil
			},
		}
		svc := NewServiceWithRepository(repo, mailer)

		result, err := svc.SendEmail(
			context.Background(),
			"tenant_test",
			"tenant-1",
			"INVOICE_SEND",
			"customer@example.com",
			"John Doe",
			"Your Invoice",
			"<p>Invoice body</p>",
			"Invoice body",
			nil,
			"invoice-123",
		)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !result.Success {
			t.Error("Success = false, want true")
		}
		if result.LogID == "" {
			t.Error("LogID should not be empty")
		}
		if !logCreated {
			t.Error("expected email log to be created")
		}
		if !statusUpdated || updatedStatus != StatusSent {
			t.Errorf("status should be updated to SENT, got %v", updatedStatus)
		}
		if !mailer.SendMailCalled {
			t.Error("expected SendMail to be called")
		}
	})

	t.Run("smtp not configured", func(t *testing.T) {
		repo := &MockRepository{
			GetTenantSettingsFn: func(ctx context.Context, tenantID string) ([]byte, error) {
				return emptySMTPSettingsJSON(), nil
			},
		}
		svc := NewServiceWithRepository(repo, &MockMailSender{})

		_, err := svc.SendEmail(
			context.Background(),
			"tenant_test",
			"tenant-1",
			"INVOICE_SEND",
			"customer@example.com",
			"",
			"Subject",
			"<p>Body</p>",
			"",
			nil,
			"",
		)

		if err == nil {
			t.Error("expected error when SMTP not configured, got nil")
		}
	})

	t.Run("send mail error updates log as failed", func(t *testing.T) {
		var updatedStatus EmailStatus
		mailer := &MockMailSender{
			SendMailFn: func(config *SMTPConfig, m *mail.Msg) error {
				return errors.New("connection refused")
			},
		}
		repo := &MockRepository{
			GetTenantSettingsFn: func(ctx context.Context, tenantID string) ([]byte, error) {
				return validSMTPSettingsJSON(), nil
			},
			CreateEmailLogFn: func(ctx context.Context, schemaName string, log *EmailLog) error {
				return nil
			},
			UpdateEmailLogStatusFn: func(ctx context.Context, schemaName, logID string, status EmailStatus, sentAt *time.Time, errorMessage string) error {
				updatedStatus = status
				return nil
			},
		}
		svc := NewServiceWithRepository(repo, mailer)

		_, err := svc.SendEmail(
			context.Background(),
			"tenant_test",
			"tenant-1",
			"INVOICE_SEND",
			"customer@example.com",
			"",
			"Subject",
			"<p>Body</p>",
			"",
			nil,
			"",
		)

		if err == nil {
			t.Error("expected error when send fails, got nil")
		}
		if updatedStatus != StatusFailed {
			t.Errorf("status should be updated to FAILED, got %v", updatedStatus)
		}
	})

	t.Run("send mail error with log update failure", func(t *testing.T) {
		mailer := &MockMailSender{
			SendMailFn: func(config *SMTPConfig, m *mail.Msg) error {
				return errors.New("connection refused")
			},
		}
		repo := &MockRepository{
			GetTenantSettingsFn: func(ctx context.Context, tenantID string) ([]byte, error) {
				return validSMTPSettingsJSON(), nil
			},
			CreateEmailLogFn: func(ctx context.Context, schemaName string, log *EmailLog) error {
				return nil
			},
			UpdateEmailLogStatusFn: func(ctx context.Context, schemaName, logID string, status EmailStatus, sentAt *time.Time, errorMessage string) error {
				return errors.New("log update failed")
			},
		}
		svc := NewServiceWithRepository(repo, mailer)

		_, err := svc.SendEmail(
			context.Background(),
			"tenant_test",
			"tenant-1",
			"INVOICE_SEND",
			"customer@example.com",
			"",
			"Subject",
			"<p>Body</p>",
			"",
			nil,
			"",
		)

		// Should still return send error even if log update fails
		if err == nil {
			t.Error("expected error when send fails, got nil")
		}
		if !strings.Contains(err.Error(), "failed to send email") {
			t.Errorf("error should contain 'failed to send email', got %q", err.Error())
		}
	})

	t.Run("with attachments", func(t *testing.T) {
		mailer := &MockMailSender{}
		repo := &MockRepository{
			GetTenantSettingsFn: func(ctx context.Context, tenantID string) ([]byte, error) {
				return validSMTPSettingsJSON(), nil
			},
			CreateEmailLogFn: func(ctx context.Context, schemaName string, log *EmailLog) error {
				return nil
			},
			UpdateEmailLogStatusFn: func(ctx context.Context, schemaName, logID string, status EmailStatus, sentAt *time.Time, errorMessage string) error {
				return nil
			},
		}
		svc := NewServiceWithRepository(repo, mailer)

		attachments := []Attachment{
			{
				Filename:    "invoice.pdf",
				Content:     []byte("PDF content"),
				ContentType: "application/pdf",
			},
		}

		result, err := svc.SendEmail(
			context.Background(),
			"tenant_test",
			"tenant-1",
			"INVOICE_SEND",
			"customer@example.com",
			"John Doe",
			"Your Invoice",
			"<p>Invoice body</p>",
			"",
			attachments,
			"invoice-123",
		)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !result.Success {
			t.Error("Success = false, want true")
		}
	})

	t.Run("create email log error", func(t *testing.T) {
		repo := &MockRepository{
			GetTenantSettingsFn: func(ctx context.Context, tenantID string) ([]byte, error) {
				return validSMTPSettingsJSON(), nil
			},
			CreateEmailLogFn: func(ctx context.Context, schemaName string, log *EmailLog) error {
				return errors.New("db error")
			},
		}
		svc := NewServiceWithRepository(repo, &MockMailSender{})

		_, err := svc.SendEmail(
			context.Background(),
			"tenant_test",
			"tenant-1",
			"INVOICE_SEND",
			"customer@example.com",
			"",
			"Subject",
			"<p>Body</p>",
			"",
			nil,
			"",
		)

		if err == nil {
			t.Error("expected error when create log fails, got nil")
		}
	})

	t.Run("ensure schema error", func(t *testing.T) {
		repo := &MockRepository{
			GetTenantSettingsFn: func(ctx context.Context, tenantID string) ([]byte, error) {
				return validSMTPSettingsJSON(), nil
			},
			EnsureSchemaFn: func(ctx context.Context, schemaName string) error {
				return errors.New("schema error")
			},
		}
		svc := NewServiceWithRepository(repo, &MockMailSender{})

		_, err := svc.SendEmail(
			context.Background(),
			"tenant_test",
			"tenant-1",
			"INVOICE_SEND",
			"customer@example.com",
			"",
			"Subject",
			"<p>Body</p>",
			"",
			nil,
			"",
		)

		if err == nil {
			t.Error("expected error when ensure schema fails, got nil")
		}
	})

	t.Run("success update log error after send", func(t *testing.T) {
		mailer := &MockMailSender{}
		repo := &MockRepository{
			GetTenantSettingsFn: func(ctx context.Context, tenantID string) ([]byte, error) {
				return validSMTPSettingsJSON(), nil
			},
			CreateEmailLogFn: func(ctx context.Context, schemaName string, log *EmailLog) error {
				return nil
			},
			UpdateEmailLogStatusFn: func(ctx context.Context, schemaName, logID string, status EmailStatus, sentAt *time.Time, errorMessage string) error {
				// Return error when updating to SENT status
				if status == StatusSent {
					return errors.New("update log error")
				}
				return nil
			},
		}
		svc := NewServiceWithRepository(repo, mailer)

		// Should still succeed even if log update fails
		result, err := svc.SendEmail(
			context.Background(),
			"tenant_test",
			"tenant-1",
			"INVOICE_SEND",
			"customer@example.com",
			"",
			"Subject",
			"<p>Body</p>",
			"",
			nil,
			"",
		)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !result.Success {
			t.Error("Success = false, want true")
		}
	})
}

func TestService_GetEmailLog(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		expectedLogs := []EmailLog{
			{
				ID:             "log-1",
				TenantID:       "tenant-1",
				EmailType:      "INVOICE_SEND",
				RecipientEmail: "customer@example.com",
				Subject:        "Invoice",
				Status:         StatusSent,
			},
		}
		repo := &MockRepository{
			GetEmailLogFn: func(ctx context.Context, schemaName, tenantID string, limit int) ([]EmailLog, error) {
				return expectedLogs, nil
			},
		}
		svc := NewServiceWithRepository(repo, &MockMailSender{})

		logs, err := svc.GetEmailLog(context.Background(), "tenant_test", "tenant-1", 50)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(logs) != 1 {
			t.Errorf("len(logs) = %d, want 1", len(logs))
		}
		if logs[0].ID != "log-1" {
			t.Errorf("ID = %q, want %q", logs[0].ID, "log-1")
		}
	})

	t.Run("repository error", func(t *testing.T) {
		repo := &MockRepository{
			GetEmailLogFn: func(ctx context.Context, schemaName, tenantID string, limit int) ([]EmailLog, error) {
				return nil, errors.New("db error")
			},
		}
		svc := NewServiceWithRepository(repo, &MockMailSender{})

		_, err := svc.GetEmailLog(context.Background(), "tenant_test", "tenant-1", 50)

		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestParseSMTPConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		config, err := ParseSMTPConfig(validSMTPSettingsJSON())

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if config.Host != "smtp.example.com" {
			t.Errorf("Host = %q, want %q", config.Host, "smtp.example.com")
		}
		if config.Port != 587 {
			t.Errorf("Port = %d, want %d", config.Port, 587)
		}
	})

	t.Run("empty config uses defaults", func(t *testing.T) {
		config, err := ParseSMTPConfig(emptySMTPSettingsJSON())

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if config.Port != 587 {
			t.Errorf("Port = %d, want default 587", config.Port)
		}
		if !config.UseTLS {
			t.Error("UseTLS should default to true")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		_, err := ParseSMTPConfig([]byte("not json"))

		if err == nil {
			t.Error("expected error for invalid JSON, got nil")
		}
	})
}

func TestMergeSMTPConfig(t *testing.T) {
	t.Run("merge into empty settings", func(t *testing.T) {
		req := &UpdateSMTPConfigRequest{
			Host:      "smtp.new.com",
			Port:      587,
			FromEmail: "from@new.com",
		}

		result, err := MergeSMTPConfig(emptySMTPSettingsJSON(), req)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		var settings map[string]interface{}
		if err := json.Unmarshal(result, &settings); err != nil {
			t.Fatalf("failed to unmarshal result: %v", err)
		}
		if settings["smtp_host"] != "smtp.new.com" {
			t.Errorf("smtp_host = %v, want %v", settings["smtp_host"], "smtp.new.com")
		}
	})

	t.Run("preserves password if not provided", func(t *testing.T) {
		existingSettings := map[string]interface{}{
			"smtp_host":     "old.smtp.com",
			"smtp_password": "secret",
		}
		existingJSON, _ := json.Marshal(existingSettings)

		req := &UpdateSMTPConfigRequest{
			Host:     "new.smtp.com",
			Password: "", // Not provided
		}

		result, err := MergeSMTPConfig(existingJSON, req)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		var settings map[string]interface{}
		if err := json.Unmarshal(result, &settings); err != nil {
			t.Fatalf("failed to unmarshal result: %v", err)
		}
		if settings["smtp_password"] != "secret" {
			t.Errorf("smtp_password should be preserved, got %v", settings["smtp_password"])
		}
	})

	t.Run("updates password if provided", func(t *testing.T) {
		existingSettings := map[string]interface{}{
			"smtp_host":     "old.smtp.com",
			"smtp_password": "old_secret",
		}
		existingJSON, _ := json.Marshal(existingSettings)

		req := &UpdateSMTPConfigRequest{
			Host:     "new.smtp.com",
			Password: "new_secret",
		}

		result, err := MergeSMTPConfig(existingJSON, req)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		var settings map[string]interface{}
		if err := json.Unmarshal(result, &settings); err != nil {
			t.Fatalf("failed to unmarshal result: %v", err)
		}
		if settings["smtp_password"] != "new_secret" {
			t.Errorf("smtp_password = %v, want %v", settings["smtp_password"], "new_secret")
		}
	})

	t.Run("handles invalid existing JSON", func(t *testing.T) {
		req := &UpdateSMTPConfigRequest{
			Host: "smtp.new.com",
		}

		result, err := MergeSMTPConfig([]byte("not json"), req)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		var settings map[string]interface{}
		if err := json.Unmarshal(result, &settings); err != nil {
			t.Fatalf("failed to unmarshal result: %v", err)
		}
		if settings["smtp_host"] != "smtp.new.com" {
			t.Errorf("smtp_host = %v, want %v", settings["smtp_host"], "smtp.new.com")
		}
	})
}

func TestDefaultMailSender(t *testing.T) {
	t.Run("SendMail returns error for invalid host", func(t *testing.T) {
		sender := &DefaultMailSender{}
		config := &SMTPConfig{
			Host:      "invalid.host.that.does.not.exist.example.com",
			Port:      587,
			FromEmail: "test@example.com",
			UseTLS:    false,
		}

		m := mail.NewMsg()
		_ = m.From("test@example.com")
		_ = m.To("recipient@example.com")
		m.Subject("Test")
		m.SetBodyString(mail.TypeTextPlain, "Test body")

		err := sender.SendMail(config, m)

		// Should return an error because the host doesn't exist
		if err == nil {
			t.Error("expected error for invalid host, got nil")
		}
	})
}

func TestNewServiceWithRepository(t *testing.T) {
	repo := &MockRepository{}
	mailer := &MockMailSender{}

	svc := NewServiceWithRepository(repo, mailer)

	if svc == nil {
		t.Error("NewServiceWithRepository returned nil")
	}
	if svc.repo != repo {
		t.Error("repo not set correctly")
	}
	if svc.mailer != mailer {
		t.Error("mailer not set correctly")
	}
}

func TestService_RenderTemplate_EdgeCases(t *testing.T) {
	svc := NewServiceWithRepository(&MockRepository{}, &MockMailSender{})

	t.Run("success with all fields", func(t *testing.T) {
		tmpl := &EmailTemplate{
			Subject:  "Invoice {{.InvoiceNumber}}",
			BodyHTML: "<p>Dear {{.ContactName}}</p>",
			BodyText: "Dear {{.ContactName}}",
		}
		data := &TemplateData{
			InvoiceNumber: "INV-001",
			ContactName:   "John Doe",
		}

		subject, bodyHTML, bodyText, err := svc.RenderTemplate(tmpl, data)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if subject != "Invoice INV-001" {
			t.Errorf("subject = %q, want %q", subject, "Invoice INV-001")
		}
		if bodyHTML != "<p>Dear John Doe</p>" {
			t.Errorf("bodyHTML = %q, want %q", bodyHTML, "<p>Dear John Doe</p>")
		}
		if bodyText != "Dear John Doe" {
			t.Errorf("bodyText = %q, want %q", bodyText, "Dear John Doe")
		}
	})

	t.Run("success without body text", func(t *testing.T) {
		tmpl := &EmailTemplate{
			Subject:  "Invoice {{.InvoiceNumber}}",
			BodyHTML: "<p>Dear {{.ContactName}}</p>",
			BodyText: "", // Empty body text
		}
		data := &TemplateData{
			InvoiceNumber: "INV-001",
			ContactName:   "John Doe",
		}

		subject, bodyHTML, bodyText, err := svc.RenderTemplate(tmpl, data)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if subject != "Invoice INV-001" {
			t.Errorf("subject = %q, want %q", subject, "Invoice INV-001")
		}
		if bodyHTML != "<p>Dear John Doe</p>" {
			t.Errorf("bodyHTML = %q, want %q", bodyHTML, "<p>Dear John Doe</p>")
		}
		if bodyText != "" {
			t.Errorf("bodyText = %q, want empty string", bodyText)
		}
	})

	t.Run("invalid subject template", func(t *testing.T) {
		tmpl := &EmailTemplate{
			Subject:  "Invoice {{.InvoiceNumber", // Invalid template syntax
			BodyHTML: "<p>Body</p>",
		}
		data := &TemplateData{}

		_, _, _, err := svc.RenderTemplate(tmpl, data)

		if err == nil {
			t.Error("expected error for invalid subject template")
		}
	})

	t.Run("invalid HTML template", func(t *testing.T) {
		tmpl := &EmailTemplate{
			Subject:  "Subject",
			BodyHTML: "<p>{{.Field</p>", // Invalid template syntax
		}
		data := &TemplateData{}

		_, _, _, err := svc.RenderTemplate(tmpl, data)

		if err == nil {
			t.Error("expected error for invalid HTML template")
		}
	})

	t.Run("invalid text template", func(t *testing.T) {
		tmpl := &EmailTemplate{
			Subject:  "Subject",
			BodyHTML: "<p>Body</p>",
			BodyText: "{{.Field", // Invalid template syntax
		}
		data := &TemplateData{}

		_, _, _, err := svc.RenderTemplate(tmpl, data)

		if err == nil {
			t.Error("expected error for invalid text template")
		}
	})
}

func TestService_SendEmail_GetSettingsError(t *testing.T) {
	repo := &MockRepository{
		GetTenantSettingsFn: func(ctx context.Context, tenantID string) ([]byte, error) {
			return nil, errors.New("db error")
		},
	}
	svc := NewServiceWithRepository(repo, &MockMailSender{})

	_, err := svc.SendEmail(
		context.Background(),
		"tenant_test",
		"tenant-1",
		"INVOICE_SEND",
		"customer@example.com",
		"",
		"Subject",
		"<p>Body</p>",
		"",
		nil,
		"",
	)

	if err == nil {
		t.Error("expected error when get settings fails")
	}
}

func TestService_TestSMTP_GetConfigError(t *testing.T) {
	repo := &MockRepository{
		GetTenantSettingsFn: func(ctx context.Context, tenantID string) ([]byte, error) {
			return nil, errors.New("db error")
		},
	}
	svc := NewServiceWithRepository(repo, &MockMailSender{})

	result, err := svc.TestSMTP(context.Background(), "tenant-1", "test@example.com")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("Success = true, want false when get config fails")
	}
	if result.Message == "" {
		t.Error("Message should contain error info")
	}
}
