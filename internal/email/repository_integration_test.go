//go:build integration

package email

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/google/uuid"
)

// contains is a helper to check if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// setupEmailTest creates a tenant and ensures email schema
func setupEmailTest(t *testing.T) (*testutil.TestTenant, *PostgresRepository, context.Context) {
	t.Helper()
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	if err := repo.EnsureSchema(ctx, tenant.SchemaName); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	return tenant, repo, ctx
}

func TestPostgresRepository_EnsureSchema(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// EnsureSchema should create email tables
	err := repo.EnsureSchema(ctx, tenant.SchemaName)
	if err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	// Verify tables exist by querying them
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM "+tenant.SchemaName+".email_templates").Scan(&count)
	if err != nil {
		t.Fatalf("email_templates table not created: %v", err)
	}

	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM "+tenant.SchemaName+".email_log").Scan(&count)
	if err != nil {
		t.Fatalf("email_log table not created: %v", err)
	}

	// EnsureSchema should be idempotent
	err = repo.EnsureSchema(ctx, tenant.SchemaName)
	if err != nil {
		t.Fatalf("EnsureSchema second call failed: %v", err)
	}
}

func TestPostgresRepository_GetAndUpdateTenantSettings(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Get initial tenant settings
	settings, err := repo.GetTenantSettings(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("GetTenantSettings failed: %v", err)
	}

	if settings == nil {
		t.Error("expected non-nil settings")
	}

	// Update settings
	newSettings := []byte(`{"smtp":{"host":"smtp.example.com","port":587}}`)
	err = repo.UpdateTenantSettings(ctx, tenant.ID, newSettings)
	if err != nil {
		t.Fatalf("UpdateTenantSettings failed: %v", err)
	}

	// Verify update (check key content, not exact JSON string since PostgreSQL normalizes)
	updatedSettings, err := repo.GetTenantSettings(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("GetTenantSettings after update failed: %v", err)
	}

	// Parse and check that smtp host was saved
	if updatedSettings == nil {
		t.Error("expected non-nil updated settings")
	}
	settingsStr := string(updatedSettings)
	if !contains(settingsStr, "smtp.example.com") {
		t.Errorf("expected settings to contain 'smtp.example.com', got %s", settingsStr)
	}
}

func TestPostgresRepository_GetTenantSettings_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Get settings for non-existent tenant
	_, err := repo.GetTenantSettings(ctx, uuid.New().String())
	if err != ErrSettingsNotFound {
		t.Errorf("expected ErrSettingsNotFound, got %v", err)
	}
}

func TestPostgresRepository_TemplateOperations(t *testing.T) {
	tenant, repo, ctx := setupEmailTest(t)

	// Create/Upsert a template
	template := &EmailTemplate{
		ID:           uuid.New().String(),
		TenantID:     tenant.ID,
		TemplateType: TemplateInvoiceSend,
		Subject:      "Invoice {{.InvoiceNumber}}",
		BodyHTML:     "<html><body>Invoice content</body></html>",
		BodyText:     "Invoice content",
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err := repo.UpsertTemplate(ctx, tenant.SchemaName, template)
	if err != nil {
		t.Fatalf("UpsertTemplate failed: %v", err)
	}

	// Get the template
	retrieved, err := repo.GetTemplate(ctx, tenant.SchemaName, tenant.ID, TemplateInvoiceSend)
	if err != nil {
		t.Fatalf("GetTemplate failed: %v", err)
	}

	if retrieved.Subject != template.Subject {
		t.Errorf("expected subject %s, got %s", template.Subject, retrieved.Subject)
	}
	if retrieved.BodyHTML != template.BodyHTML {
		t.Errorf("expected body HTML %s, got %s", template.BodyHTML, retrieved.BodyHTML)
	}

	// List templates
	templates, err := repo.ListTemplates(ctx, tenant.SchemaName, tenant.ID)
	if err != nil {
		t.Fatalf("ListTemplates failed: %v", err)
	}

	if len(templates) < 1 {
		t.Errorf("expected at least 1 template, got %d", len(templates))
	}

	// Update template via upsert
	template.Subject = "Updated Invoice {{.InvoiceNumber}}"
	err = repo.UpsertTemplate(ctx, tenant.SchemaName, template)
	if err != nil {
		t.Fatalf("UpsertTemplate (update) failed: %v", err)
	}

	// Verify update
	updated, err := repo.GetTemplate(ctx, tenant.SchemaName, tenant.ID, TemplateInvoiceSend)
	if err != nil {
		t.Fatalf("GetTemplate after update failed: %v", err)
	}

	if updated.Subject != "Updated Invoice {{.InvoiceNumber}}" {
		t.Errorf("expected updated subject, got %s", updated.Subject)
	}
}

func TestPostgresRepository_GetTemplate_NotFound(t *testing.T) {
	tenant, repo, ctx := setupEmailTest(t)

	// Get non-existent template
	_, err := repo.GetTemplate(ctx, tenant.SchemaName, tenant.ID, TemplateInvoiceSend)
	if err != ErrTemplateNotFound {
		t.Errorf("expected ErrTemplateNotFound, got %v", err)
	}
}

func TestPostgresRepository_EmailLogOperations(t *testing.T) {
	tenant, repo, ctx := setupEmailTest(t)

	// Create email log
	logID := uuid.New().String()
	log := &EmailLog{
		ID:             logID,
		TenantID:       tenant.ID,
		EmailType:      "INVOICE",
		RecipientEmail: "customer@example.com",
		RecipientName:  "Test Customer",
		Subject:        "Invoice #001",
		Status:         StatusPending,
		CreatedAt:      time.Now(),
	}

	err := repo.CreateEmailLog(ctx, tenant.SchemaName, log)
	if err != nil {
		t.Fatalf("CreateEmailLog failed: %v", err)
	}

	// Get email logs
	logs, err := repo.GetEmailLog(ctx, tenant.SchemaName, tenant.ID, 10)
	if err != nil {
		t.Fatalf("GetEmailLog failed: %v", err)
	}

	if len(logs) < 1 {
		t.Fatalf("expected at least 1 log entry, got %d", len(logs))
	}

	// Find our log entry
	var found *EmailLog
	for i := range logs {
		if logs[i].ID == logID {
			found = &logs[i]
			break
		}
	}

	if found == nil {
		t.Fatal("created log entry not found")
	}

	if found.RecipientEmail != log.RecipientEmail {
		t.Errorf("expected email %s, got %s", log.RecipientEmail, found.RecipientEmail)
	}
	if found.Status != StatusPending {
		t.Errorf("expected status PENDING, got %s", found.Status)
	}

	// Update email log status to SENT
	sentAt := time.Now()
	err = repo.UpdateEmailLogStatus(ctx, tenant.SchemaName, logID, StatusSent, &sentAt, "")
	if err != nil {
		t.Fatalf("UpdateEmailLogStatus failed: %v", err)
	}

	// Verify status update
	updatedLogs, err := repo.GetEmailLog(ctx, tenant.SchemaName, tenant.ID, 10)
	if err != nil {
		t.Fatalf("GetEmailLog after update failed: %v", err)
	}

	for _, l := range updatedLogs {
		if l.ID == logID {
			if l.Status != StatusSent {
				t.Errorf("expected status SENT, got %s", l.Status)
			}
			break
		}
	}

	// Update status with error
	err = repo.UpdateEmailLogStatus(ctx, tenant.SchemaName, logID, StatusFailed, nil, "SMTP connection error")
	if err != nil {
		t.Fatalf("UpdateEmailLogStatus with error failed: %v", err)
	}

	// Verify error was saved
	failedLogs, err := repo.GetEmailLog(ctx, tenant.SchemaName, tenant.ID, 10)
	if err != nil {
		t.Fatalf("GetEmailLog for failed status: %v", err)
	}

	for _, l := range failedLogs {
		if l.ID == logID {
			if l.Status != StatusFailed {
				t.Errorf("expected status FAILED, got %s", l.Status)
			}
			if l.ErrorMessage != "SMTP connection error" {
				t.Errorf("expected error message 'SMTP connection error', got '%s'", l.ErrorMessage)
			}
			break
		}
	}
}

func TestPostgresRepository_ListTemplates_Empty(t *testing.T) {
	tenant, repo, ctx := setupEmailTest(t)

	// List templates when none exist
	templates, err := repo.ListTemplates(ctx, tenant.SchemaName, tenant.ID)
	if err != nil {
		t.Fatalf("ListTemplates failed: %v", err)
	}

	// Should return empty slice or nil when no templates exist
	if len(templates) != 0 {
		t.Errorf("expected 0 templates, got %d", len(templates))
	}
}

func TestPostgresRepository_GetEmailLog_Empty(t *testing.T) {
	tenant, repo, ctx := setupEmailTest(t)

	// Get email logs when none exist
	logs, err := repo.GetEmailLog(ctx, tenant.SchemaName, tenant.ID, 10)
	if err != nil {
		t.Fatalf("GetEmailLog failed: %v", err)
	}

	// Should return empty slice or nil when no logs exist
	if len(logs) != 0 {
		t.Errorf("expected 0 logs, got %d", len(logs))
	}
}

func TestPostgresRepository_CreateEmailLog_WithRelatedID(t *testing.T) {
	tenant, repo, ctx := setupEmailTest(t)

	// Create email log with related ID (e.g., an invoice ID)
	logID := uuid.New().String()
	relatedID := uuid.New().String()
	log := &EmailLog{
		ID:             logID,
		TenantID:       tenant.ID,
		EmailType:      "INVOICE",
		RecipientEmail: "customer@example.com",
		RecipientName:  "Test Customer",
		Subject:        "Invoice #001",
		RelatedID:      relatedID,
		Status:         StatusPending,
		CreatedAt:      time.Now(),
	}

	err := repo.CreateEmailLog(ctx, tenant.SchemaName, log)
	if err != nil {
		t.Fatalf("CreateEmailLog with related ID failed: %v", err)
	}

	// Verify log was created
	logs, err := repo.GetEmailLog(ctx, tenant.SchemaName, tenant.ID, 10)
	if err != nil {
		t.Fatalf("GetEmailLog failed: %v", err)
	}

	if len(logs) < 1 {
		t.Fatal("expected at least 1 log entry")
	}

	// Find our log entry and verify related ID
	for _, l := range logs {
		if l.ID == logID {
			if l.RelatedID != relatedID {
				t.Errorf("expected related ID %s, got %s", relatedID, l.RelatedID)
			}
			break
		}
	}
}

func TestPostgresRepository_ParseAndMergeSMTPConfig(t *testing.T) {
	// Test ParseSMTPConfig - uses flat keys like smtp_host, smtp_port, etc.
	validJSON := []byte(`{"smtp_host":"mail.example.com","smtp_port":587,"smtp_username":"user","smtp_password":"pass","smtp_from_email":"noreply@example.com","smtp_from_name":"Test Company"}`)
	config, err := ParseSMTPConfig(validJSON)
	if err != nil {
		t.Fatalf("ParseSMTPConfig failed: %v", err)
	}

	if config.Host != "mail.example.com" {
		t.Errorf("expected host 'mail.example.com', got '%s'", config.Host)
	}
	if config.Port != 587 {
		t.Errorf("expected port 587, got %d", config.Port)
	}

	// Test MergeSMTPConfig
	update := &UpdateSMTPConfigRequest{
		Host: "updated.mail.com",
		Port: 465,
	}
	mergedJSON, err := MergeSMTPConfig(validJSON, update)
	if err != nil {
		t.Fatalf("MergeSMTPConfig failed: %v", err)
	}

	// Parse the merged result to verify
	mergedConfig, err := ParseSMTPConfig(mergedJSON)
	if err != nil {
		t.Fatalf("ParseSMTPConfig on merged result failed: %v", err)
	}

	if mergedConfig.Host != "updated.mail.com" {
		t.Errorf("expected merged host 'updated.mail.com', got '%s'", mergedConfig.Host)
	}
	if mergedConfig.Port != 465 {
		t.Errorf("expected merged port 465, got %d", mergedConfig.Port)
	}
}
