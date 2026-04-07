package invoicing

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/email"
	"github.com/shopspring/decimal"
	"github.com/wneessen/go-mail"
)

type reminderEmailRepo struct {
	getTemplateErr      error
	createEmailLogCalls int
	updateEmailLogCalls int
}

func (m *reminderEmailRepo) EnsureSchema(ctx context.Context, schemaName string) error {
	return nil
}

func (m *reminderEmailRepo) GetTenantSettings(ctx context.Context, tenantID string) ([]byte, error) {
	settings := map[string]interface{}{
		"smtp_host":       "smtp.example.com",
		"smtp_port":       587,
		"smtp_username":   "user@example.com",
		"smtp_password":   "password",
		"smtp_from_email": "accounts@example.com",
		"smtp_from_name":  "Open Accounting",
		"smtp_use_tls":    true,
	}
	return json.Marshal(settings)
}

func (m *reminderEmailRepo) UpdateTenantSettings(ctx context.Context, tenantID string, settingsJSON []byte) error {
	return nil
}

func (m *reminderEmailRepo) GetTemplate(ctx context.Context, schemaName, tenantID string, templateType email.TemplateType) (*email.EmailTemplate, error) {
	if m.getTemplateErr != nil {
		return nil, m.getTemplateErr
	}
	return nil, email.ErrTemplateNotFound
}

func (m *reminderEmailRepo) ListTemplates(ctx context.Context, schemaName, tenantID string) ([]email.EmailTemplate, error) {
	return nil, nil
}

func (m *reminderEmailRepo) UpsertTemplate(ctx context.Context, schemaName string, template *email.EmailTemplate) error {
	return nil
}

func (m *reminderEmailRepo) CreateEmailLog(ctx context.Context, schemaName string, log *email.EmailLog) error {
	m.createEmailLogCalls++
	return nil
}

func (m *reminderEmailRepo) UpdateEmailLogStatus(ctx context.Context, schemaName, logID string, status email.EmailStatus, sentAt *time.Time, errorMessage string) error {
	m.updateEmailLogCalls++
	return nil
}

func (m *reminderEmailRepo) GetEmailLog(ctx context.Context, schemaName, tenantID string, limit int) ([]email.EmailLog, error) {
	return nil, nil
}

type reminderMailSender struct {
	sendErr error
	calls   int
}

func (m *reminderMailSender) SendMail(config *email.SMTPConfig, msg *mail.Msg) error {
	m.calls++
	return m.sendErr
}

func TestNewReminderService(t *testing.T) {
	svc := NewReminderService(nil, nil)
	if svc == nil || svc.repo == nil {
		t.Fatal("expected reminder service with repository")
	}
}

func TestReminderServiceSendReminderSuccess(t *testing.T) {
	repo := NewMockReminderRepository()
	repo.AddMockOverdueInvoice(
		"inv-1", "INV-001", "contact-1", "Test Contact", "contact@example.com",
		"EUR", decimal.NewFromInt(100), decimal.Zero, 12,
	)
	emailRepo := &reminderEmailRepo{}
	mailer := &reminderMailSender{}
	emailSvc := email.NewServiceWithRepository(emailRepo, mailer)
	svc := NewReminderServiceWithRepository(repo, emailSvc)

	result, err := svc.SendReminder(context.Background(), "tenant-1", "tenant_test", &SendReminderRequest{
		InvoiceID: "inv-1",
		Message:   "Please settle this invoice.",
	}, "Test Company")
	if err != nil {
		t.Fatalf("SendReminder failed: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected successful reminder result, got %+v", result)
	}
	if mailer.calls != 1 {
		t.Fatalf("expected one email send call, got %d", mailer.calls)
	}
	if len(repo.Reminders["inv-1"]) != 1 || repo.Reminders["inv-1"][0].Status != ReminderStatusSent {
		t.Fatalf("expected sent reminder to be recorded, got %+v", repo.Reminders["inv-1"])
	}
}

func TestReminderServiceSendBulkRemindersTracksMixedResults(t *testing.T) {
	repo := NewMockReminderRepository()
	repo.AddMockOverdueInvoice(
		"inv-1", "INV-001", "contact-1", "Test Contact", "contact@example.com",
		"EUR", decimal.NewFromInt(100), decimal.Zero, 12,
	)
	repo.AddMockOverdueInvoice(
		"inv-2", "INV-002", "contact-2", "No Email", "",
		"EUR", decimal.NewFromInt(80), decimal.Zero, 4,
	)
	emailSvc := email.NewServiceWithRepository(&reminderEmailRepo{}, &reminderMailSender{})
	svc := NewReminderServiceWithRepository(repo, emailSvc)

	result, err := svc.SendBulkReminders(context.Background(), "tenant-1", "tenant_test", &SendBulkRemindersRequest{
		InvoiceIDs: []string{"inv-1", "inv-2", "missing"},
		Message:    "Bulk reminder",
	}, "Test Company")
	if err != nil {
		t.Fatalf("SendBulkReminders failed: %v", err)
	}
	if result.TotalRequested != 3 || result.Successful != 1 || result.Failed != 2 {
		t.Fatalf("unexpected bulk result: %+v", result)
	}
}

func TestReminderServiceSendReminderEmailFailureReturnsResult(t *testing.T) {
	repo := NewMockReminderRepository()
	repo.AddMockOverdueInvoice(
		"inv-1", "INV-003", "contact-1", "Test Contact", "contact@example.com",
		"EUR", decimal.NewFromInt(100), decimal.Zero, 12,
	)
	emailSvc := email.NewServiceWithRepository(&reminderEmailRepo{}, &reminderMailSender{sendErr: errors.New("smtp down")})
	svc := NewReminderServiceWithRepository(repo, emailSvc)

	result, err := svc.SendReminder(context.Background(), "tenant-1", "tenant_test", &SendReminderRequest{
		InvoiceID: "inv-1",
	}, "Test Company")
	if err != nil {
		t.Fatalf("SendReminder returned unexpected error: %v", err)
	}
	if result.Success {
		t.Fatalf("expected unsuccessful reminder result, got %+v", result)
	}
	if len(repo.Reminders["inv-1"]) != 1 || repo.Reminders["inv-1"][0].Status != ReminderStatusFailed {
		t.Fatalf("expected failed reminder to be recorded, got %+v", repo.Reminders["inv-1"])
	}
}
