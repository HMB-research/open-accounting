package documents

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/email"
)

type mockRetentionEmailService struct {
	getTemplateErr    error
	renderErr         error
	sendErr           error
	templateRequested email.TemplateType
	templateData      *email.TemplateData
	sendCalls         int
	recipientEmail    string
	emailType         string
}

func (m *mockRetentionEmailService) GetTemplate(ctx context.Context, schemaName, tenantID string, templateType email.TemplateType) (*email.EmailTemplate, error) {
	m.templateRequested = templateType
	if m.getTemplateErr != nil {
		return nil, m.getTemplateErr
	}
	return &email.EmailTemplate{
		TemplateType: templateType,
		Subject:      "Retention {{.RetentionActionCount}}",
		BodyHTML:     "{{range .RetentionActions}}<p>{{.Action}} {{.FileName}} {{.Message}}</p>{{end}}",
		BodyText:     "{{range .RetentionActions}}{{.Action}} {{.FileName}}\n{{end}}",
		IsActive:     true,
	}, nil
}

func (m *mockRetentionEmailService) RenderTemplate(tmpl *email.EmailTemplate, data *email.TemplateData) (string, string, string, error) {
	m.templateData = data
	if m.renderErr != nil {
		return "", "", "", m.renderErr
	}
	return fmt.Sprintf("Retention %d", data.RetentionActionCount), "<p>body</p>", "body", nil
}

func (m *mockRetentionEmailService) SendEmail(ctx context.Context, schemaName, tenantID, emailType, recipient, recipientName, subject, bodyHTML, bodyText string, attachments []email.Attachment, relatedID string) (*email.EmailSentResponse, error) {
	m.sendCalls++
	m.recipientEmail = recipient
	m.emailType = emailType
	if m.sendErr != nil {
		return nil, m.sendErr
	}
	return &email.EmailSentResponse{Success: true, LogID: "email-log-1"}, nil
}

func TestRetentionReminderService_ProcessRetentionRemindersForTenant(t *testing.T) {
	asOf := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	retentionUntil := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	repo := newMockRepository()
	repo.docs["doc-due"] = &Document{
		ID:             "doc-due",
		TenantID:       "tenant-1",
		EntityType:     EntityTypeInvoice,
		EntityID:       "invoice-1",
		DocumentType:   DocumentTypeReceipt,
		FileName:       "receipt.pdf",
		ReviewStatus:   ReviewStatusPending,
		RetentionUntil: &retentionUntil,
		CreatedAt:      asOf.Add(-time.Hour),
	}
	docSvc := NewService(repo, &mockStore{})
	emailSvc := &mockRetentionEmailService{}
	svc := NewRetentionReminderService(docSvc, emailSvc)

	result, err := svc.ProcessRetentionRemindersForTenant(context.Background(), "tenant-1", "tenant_demo", "Demo OU", "ops@example.com", asOf, 30, true)
	if err != nil {
		t.Fatalf("ProcessRetentionRemindersForTenant failed: %v", err)
	}

	if !result.EmailSent || result.EmailLogID != "email-log-1" {
		t.Fatalf("expected email to be sent with log id, got %#v", result)
	}
	if result.ActionsFound != 2 {
		t.Fatalf("expected due-soon and pending-review actions, got %#v", result)
	}
	if emailSvc.templateRequested != email.TemplateDocumentRetentionReminder {
		t.Fatalf("expected retention template, got %s", emailSvc.templateRequested)
	}
	if emailSvc.recipientEmail != "ops@example.com" || emailSvc.emailType != string(email.TemplateDocumentRetentionReminder) {
		t.Fatalf("unexpected send target/type: %#v", emailSvc)
	}
	if emailSvc.templateData == nil || emailSvc.templateData.RetentionActionCount != 2 {
		t.Fatalf("expected template data with 2 actions, got %#v", emailSvc.templateData)
	}
	if got := emailSvc.templateData.RetentionActions[0].RetentionUntil; got != "2026-03-20" {
		t.Fatalf("expected retention date in template data, got %q", got)
	}
	if got := emailSvc.templateData.RetentionActions[0].DaysUntilRetention; got != "5" {
		t.Fatalf("expected days until retention in template data, got %q", got)
	}
}

func TestRetentionReminderService_SkipsWithoutRecipient(t *testing.T) {
	svc := NewRetentionReminderService(NewService(newMockRepository(), &mockStore{}), &mockRetentionEmailService{})

	result, err := svc.ProcessRetentionRemindersForTenant(context.Background(), "tenant-1", "tenant_demo", "Demo OU", " ", time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), 30, true)
	if err != nil {
		t.Fatalf("ProcessRetentionRemindersForTenant failed: %v", err)
	}
	if !result.Skipped || result.SkipReason != "tenant settings email is not configured" {
		t.Fatalf("expected skip without recipient, got %#v", result)
	}
}

func TestRetentionReminderService_SkipsWithoutActions(t *testing.T) {
	emailSvc := &mockRetentionEmailService{}
	svc := NewRetentionReminderService(NewService(newMockRepository(), &mockStore{}), emailSvc)

	result, err := svc.ProcessRetentionRemindersForTenant(context.Background(), "tenant-1", "tenant_demo", "Demo OU", "ops@example.com", time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), 30, true)
	if err != nil {
		t.Fatalf("ProcessRetentionRemindersForTenant failed: %v", err)
	}
	if !result.Skipped || result.SkipReason != "no document retention reminder actions" {
		t.Fatalf("expected skip without actions, got %#v", result)
	}
	if emailSvc.sendCalls != 0 {
		t.Fatalf("expected no email sends, got %d", emailSvc.sendCalls)
	}
}

func TestRetentionReminderService_ReportsTemplateRenderAndSendFailures(t *testing.T) {
	asOf := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	retentionUntil := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		emailSvc  *mockRetentionEmailService
		wantError string
	}{
		{
			name:      "template error",
			emailSvc:  &mockRetentionEmailService{getTemplateErr: fmt.Errorf("template missing")},
			wantError: "get template: template missing",
		},
		{
			name:      "render error",
			emailSvc:  &mockRetentionEmailService{renderErr: fmt.Errorf("bad template")},
			wantError: "render template: bad template",
		},
		{
			name:      "send error",
			emailSvc:  &mockRetentionEmailService{sendErr: fmt.Errorf("smtp down")},
			wantError: "send email: smtp down",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepository()
			repo.docs["doc-expired"] = &Document{
				ID:             "doc-expired",
				TenantID:       "tenant-1",
				EntityType:     EntityTypeInvoice,
				EntityID:       "invoice-1",
				DocumentType:   DocumentTypeReceipt,
				FileName:       "receipt.pdf",
				ReviewStatus:   ReviewStatusApproved,
				RetentionUntil: &retentionUntil,
				CreatedAt:      asOf,
			}
			svc := NewRetentionReminderService(NewService(repo, &mockStore{}), tt.emailSvc)

			result, err := svc.ProcessRetentionRemindersForTenant(context.Background(), "tenant-1", "tenant_demo", "Demo OU", "ops@example.com", asOf, 30, true)
			if err != nil {
				t.Fatalf("ProcessRetentionRemindersForTenant returned unexpected hard error: %v", err)
			}
			if !result.Failed || !strings.Contains(result.ErrorMessage, tt.wantError) {
				t.Fatalf("expected failed result containing %q, got %#v", tt.wantError, result)
			}
		})
	}
}
