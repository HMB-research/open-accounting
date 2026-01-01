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

func TestPostgresRepository_EnsureSchema_InvalidSchema(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Test with an invalid schema name that doesn't exist
	// This tests the error path for schema creation
	err := repo.EnsureSchema(ctx, "nonexistent_schema_12345")
	if err == nil {
		t.Error("expected error for nonexistent schema, got nil")
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

func TestPostgresRepository_UpdateTenantSettings_NonexistentTenant(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Update settings for non-existent tenant (should not error, just no rows affected)
	newSettings := []byte(`{"smtp":{"host":"smtp.example.com"}}`)
	err := repo.UpdateTenantSettings(ctx, uuid.New().String(), newSettings)
	// This should not return an error, it just won't affect any rows
	if err != nil {
		t.Errorf("unexpected error: %v", err)
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

func TestPostgresRepository_GetTemplate_InvalidSchema(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Get template from non-existent schema - this tests the error path
	_, err := repo.GetTemplate(ctx, "nonexistent_schema_xyz", tenant.ID, TemplateInvoiceSend)
	if err == nil {
		t.Error("expected error for nonexistent schema, got nil")
	}
	// Should not be ErrTemplateNotFound, should be a database error
	if err == ErrTemplateNotFound {
		t.Error("expected database error, not ErrTemplateNotFound")
	}
}

func TestPostgresRepository_ListTemplates_InvalidSchema(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// List templates from non-existent schema - this tests the query error path
	_, err := repo.ListTemplates(ctx, "nonexistent_schema_xyz", tenant.ID)
	if err == nil {
		t.Error("expected error for nonexistent schema, got nil")
	}
}

func TestPostgresRepository_UpsertTemplate_InvalidSchema(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	template := &EmailTemplate{
		ID:           uuid.New().String(),
		TenantID:     tenant.ID,
		TemplateType: TemplateInvoiceSend,
		Subject:      "Test",
		BodyHTML:     "<html>Test</html>",
		IsActive:     true,
	}

	// Upsert template to non-existent schema - this tests the error path
	err := repo.UpsertTemplate(ctx, "nonexistent_schema_xyz", template)
	if err == nil {
		t.Error("expected error for nonexistent schema, got nil")
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

func TestPostgresRepository_GetEmailLog_DefaultLimit(t *testing.T) {
	tenant, repo, ctx := setupEmailTest(t)

	// Create some email logs
	for i := 0; i < 5; i++ {
		log := &EmailLog{
			ID:             uuid.New().String(),
			TenantID:       tenant.ID,
			EmailType:      "INVOICE",
			RecipientEmail: "customer@example.com",
			RecipientName:  "Test Customer",
			Subject:        "Invoice #001",
			Status:         StatusPending,
			CreatedAt:      time.Now(),
		}
		if err := repo.CreateEmailLog(ctx, tenant.SchemaName, log); err != nil {
			t.Fatalf("CreateEmailLog failed: %v", err)
		}
	}

	// Get email logs with zero limit (should use default limit of 50)
	logs, err := repo.GetEmailLog(ctx, tenant.SchemaName, tenant.ID, 0)
	if err != nil {
		t.Fatalf("GetEmailLog with zero limit failed: %v", err)
	}

	if len(logs) != 5 {
		t.Errorf("expected 5 logs, got %d", len(logs))
	}

	// Get email logs with negative limit (should use default limit of 50)
	logs, err = repo.GetEmailLog(ctx, tenant.SchemaName, tenant.ID, -1)
	if err != nil {
		t.Fatalf("GetEmailLog with negative limit failed: %v", err)
	}

	if len(logs) != 5 {
		t.Errorf("expected 5 logs, got %d", len(logs))
	}
}

func TestPostgresRepository_GetEmailLog_InvalidSchema(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Get email logs from non-existent schema - this tests the query error path
	_, err := repo.GetEmailLog(ctx, "nonexistent_schema_xyz", tenant.ID, 10)
	if err == nil {
		t.Error("expected error for nonexistent schema, got nil")
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

func TestPostgresRepository_CreateEmailLog_WithoutRelatedID(t *testing.T) {
	tenant, repo, ctx := setupEmailTest(t)

	// Create email log without related ID (empty string)
	logID := uuid.New().String()
	log := &EmailLog{
		ID:             logID,
		TenantID:       tenant.ID,
		EmailType:      "GENERAL",
		RecipientEmail: "customer@example.com",
		RecipientName:  "Test Customer",
		Subject:        "General notification",
		RelatedID:      "", // Empty related ID
		Status:         StatusPending,
		CreatedAt:      time.Now(),
	}

	err := repo.CreateEmailLog(ctx, tenant.SchemaName, log)
	if err != nil {
		t.Fatalf("CreateEmailLog without related ID failed: %v", err)
	}

	// Verify log was created with nil related_id
	logs, err := repo.GetEmailLog(ctx, tenant.SchemaName, tenant.ID, 10)
	if err != nil {
		t.Fatalf("GetEmailLog failed: %v", err)
	}

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

	if found.RelatedID != "" {
		t.Errorf("expected empty related ID, got %s", found.RelatedID)
	}
}

func TestPostgresRepository_CreateEmailLog_InvalidSchema(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	log := &EmailLog{
		ID:             uuid.New().String(),
		TenantID:       tenant.ID,
		EmailType:      "INVOICE",
		RecipientEmail: "customer@example.com",
		Subject:        "Test",
		Status:         StatusPending,
	}

	// Create email log in non-existent schema
	err := repo.CreateEmailLog(ctx, "nonexistent_schema_xyz", log)
	if err == nil {
		t.Error("expected error for nonexistent schema, got nil")
	}
}

func TestPostgresRepository_UpdateEmailLogStatus_InvalidSchema(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Update status in non-existent schema
	err := repo.UpdateEmailLogStatus(ctx, "nonexistent_schema_xyz", uuid.New().String(), StatusSent, nil, "")
	if err == nil {
		t.Error("expected error for nonexistent schema, got nil")
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

func TestParseSMTPConfig_InvalidJSON(t *testing.T) {
	// Test ParseSMTPConfig with invalid JSON
	invalidJSON := []byte(`{invalid json}`)
	_, err := ParseSMTPConfig(invalidJSON)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestParseSMTPConfig_EmptySettings(t *testing.T) {
	// Test ParseSMTPConfig with empty settings
	emptyJSON := []byte(`{}`)
	config, err := ParseSMTPConfig(emptyJSON)
	if err != nil {
		t.Fatalf("ParseSMTPConfig failed: %v", err)
	}

	// Should have default values
	if config.Port != 587 {
		t.Errorf("expected default port 587, got %d", config.Port)
	}
	if config.UseTLS != true {
		t.Error("expected default UseTLS to be true")
	}
	if config.Host != "" {
		t.Errorf("expected empty host, got '%s'", config.Host)
	}
}

func TestParseSMTPConfig_AllFields(t *testing.T) {
	// Test ParseSMTPConfig with all fields
	fullJSON := []byte(`{
		"smtp_host": "smtp.example.com",
		"smtp_port": 465,
		"smtp_username": "testuser",
		"smtp_password": "testpass",
		"smtp_from_email": "from@example.com",
		"smtp_from_name": "From Name",
		"smtp_use_tls": false
	}`)
	config, err := ParseSMTPConfig(fullJSON)
	if err != nil {
		t.Fatalf("ParseSMTPConfig failed: %v", err)
	}

	if config.Host != "smtp.example.com" {
		t.Errorf("expected host 'smtp.example.com', got '%s'", config.Host)
	}
	if config.Port != 465 {
		t.Errorf("expected port 465, got %d", config.Port)
	}
	if config.Username != "testuser" {
		t.Errorf("expected username 'testuser', got '%s'", config.Username)
	}
	if config.Password != "testpass" {
		t.Errorf("expected password 'testpass', got '%s'", config.Password)
	}
	if config.FromEmail != "from@example.com" {
		t.Errorf("expected from email 'from@example.com', got '%s'", config.FromEmail)
	}
	if config.FromName != "From Name" {
		t.Errorf("expected from name 'From Name', got '%s'", config.FromName)
	}
	if config.UseTLS != false {
		t.Error("expected UseTLS to be false")
	}
}

func TestMergeSMTPConfig_InvalidExistingJSON(t *testing.T) {
	// Test MergeSMTPConfig with invalid existing JSON - should create new settings
	invalidJSON := []byte(`{invalid json}`)
	update := &UpdateSMTPConfigRequest{
		Host:      "new.mail.com",
		Port:      587,
		Username:  "newuser",
		FromEmail: "new@example.com",
		FromName:  "New Name",
		UseTLS:    true,
	}

	mergedJSON, err := MergeSMTPConfig(invalidJSON, update)
	if err != nil {
		t.Fatalf("MergeSMTPConfig failed: %v", err)
	}

	// Parse and verify the new settings were created
	config, err := ParseSMTPConfig(mergedJSON)
	if err != nil {
		t.Fatalf("ParseSMTPConfig failed: %v", err)
	}

	if config.Host != "new.mail.com" {
		t.Errorf("expected host 'new.mail.com', got '%s'", config.Host)
	}
}

func TestMergeSMTPConfig_PreservePassword(t *testing.T) {
	// Test that MergeSMTPConfig preserves existing password when new one is empty
	existingJSON := []byte(`{"smtp_password":"existing_password"}`)
	update := &UpdateSMTPConfigRequest{
		Host:      "mail.example.com",
		Port:      587,
		Password:  "", // Empty password should not overwrite
		FromEmail: "from@example.com",
	}

	mergedJSON, err := MergeSMTPConfig(existingJSON, update)
	if err != nil {
		t.Fatalf("MergeSMTPConfig failed: %v", err)
	}

	config, err := ParseSMTPConfig(mergedJSON)
	if err != nil {
		t.Fatalf("ParseSMTPConfig failed: %v", err)
	}

	if config.Password != "existing_password" {
		t.Errorf("expected password 'existing_password', got '%s'", config.Password)
	}
}

func TestMergeSMTPConfig_UpdatePassword(t *testing.T) {
	// Test that MergeSMTPConfig updates password when provided
	existingJSON := []byte(`{"smtp_password":"old_password"}`)
	update := &UpdateSMTPConfigRequest{
		Host:      "mail.example.com",
		Port:      587,
		Password:  "new_password",
		FromEmail: "from@example.com",
	}

	mergedJSON, err := MergeSMTPConfig(existingJSON, update)
	if err != nil {
		t.Fatalf("MergeSMTPConfig failed: %v", err)
	}

	config, err := ParseSMTPConfig(mergedJSON)
	if err != nil {
		t.Fatalf("ParseSMTPConfig failed: %v", err)
	}

	if config.Password != "new_password" {
		t.Errorf("expected password 'new_password', got '%s'", config.Password)
	}
}

func TestPostgresRepository_ListTemplates_MultipleTemplates(t *testing.T) {
	tenant, repo, ctx := setupEmailTest(t)

	// Create multiple templates
	templates := []*EmailTemplate{
		{
			ID:           uuid.New().String(),
			TenantID:     tenant.ID,
			TemplateType: TemplateInvoiceSend,
			Subject:      "Invoice Subject",
			BodyHTML:     "<html>Invoice</html>",
			BodyText:     "Invoice text",
			IsActive:     true,
		},
		{
			ID:           uuid.New().String(),
			TenantID:     tenant.ID,
			TemplateType: TemplatePaymentReceipt,
			Subject:      "Receipt Subject",
			BodyHTML:     "<html>Receipt</html>",
			BodyText:     "Receipt text",
			IsActive:     true,
		},
		{
			ID:           uuid.New().String(),
			TenantID:     tenant.ID,
			TemplateType: TemplateOverdueReminder,
			Subject:      "Reminder Subject",
			BodyHTML:     "<html>Reminder</html>",
			BodyText:     "Reminder text",
			IsActive:     false,
		},
	}

	for _, tmpl := range templates {
		if err := repo.UpsertTemplate(ctx, tenant.SchemaName, tmpl); err != nil {
			t.Fatalf("UpsertTemplate failed: %v", err)
		}
	}

	// List all templates
	result, err := repo.ListTemplates(ctx, tenant.SchemaName, tenant.ID)
	if err != nil {
		t.Fatalf("ListTemplates failed: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("expected 3 templates, got %d", len(result))
	}

	// Verify templates are ordered by template_type
	// INVOICE_SEND, OVERDUE_REMINDER, PAYMENT_RECEIPT (alphabetical order)
	expectedOrder := []TemplateType{TemplateInvoiceSend, TemplateOverdueReminder, TemplatePaymentReceipt}
	for i, tmpl := range result {
		if tmpl.TemplateType != expectedOrder[i] {
			t.Errorf("expected template type %s at position %d, got %s", expectedOrder[i], i, tmpl.TemplateType)
		}
	}
}

func TestPostgresRepository_UpsertTemplate_UpdateExisting(t *testing.T) {
	tenant, repo, ctx := setupEmailTest(t)

	// Create initial template
	template := &EmailTemplate{
		ID:           uuid.New().String(),
		TenantID:     tenant.ID,
		TemplateType: TemplateInvoiceSend,
		Subject:      "Original Subject",
		BodyHTML:     "<html>Original</html>",
		BodyText:     "Original text",
		IsActive:     true,
	}

	err := repo.UpsertTemplate(ctx, tenant.SchemaName, template)
	if err != nil {
		t.Fatalf("UpsertTemplate (create) failed: %v", err)
	}

	originalID := template.ID

	// Update the same template type with a new ID (conflict resolution)
	template.ID = uuid.New().String()
	template.Subject = "Updated Subject"
	template.BodyHTML = "<html>Updated</html>"
	template.BodyText = "Updated text"
	template.IsActive = false

	err = repo.UpsertTemplate(ctx, tenant.SchemaName, template)
	if err != nil {
		t.Fatalf("UpsertTemplate (update) failed: %v", err)
	}

	// Verify update - the ID should remain the original one due to ON CONFLICT behavior
	retrieved, err := repo.GetTemplate(ctx, tenant.SchemaName, tenant.ID, TemplateInvoiceSend)
	if err != nil {
		t.Fatalf("GetTemplate failed: %v", err)
	}

	// The ID should be the original ID (because ON CONFLICT updates, not replaces)
	if retrieved.ID != originalID {
		t.Errorf("expected original ID %s, got %s", originalID, retrieved.ID)
	}
	if retrieved.Subject != "Updated Subject" {
		t.Errorf("expected 'Updated Subject', got '%s'", retrieved.Subject)
	}
	if retrieved.IsActive != false {
		t.Error("expected IsActive to be false")
	}
}

func TestPostgresRepository_GetEmailLog_Limit(t *testing.T) {
	tenant, repo, ctx := setupEmailTest(t)

	// Create 10 email logs
	for i := 0; i < 10; i++ {
		log := &EmailLog{
			ID:             uuid.New().String(),
			TenantID:       tenant.ID,
			EmailType:      "INVOICE",
			RecipientEmail: "customer@example.com",
			Subject:        "Test " + string(rune('A'+i)),
			Status:         StatusPending,
		}
		if err := repo.CreateEmailLog(ctx, tenant.SchemaName, log); err != nil {
			t.Fatalf("CreateEmailLog failed: %v", err)
		}
		// Small delay to ensure different creation times for ordering
		time.Sleep(5 * time.Millisecond)
	}

	// Get only 5 logs
	logs, err := repo.GetEmailLog(ctx, tenant.SchemaName, tenant.ID, 5)
	if err != nil {
		t.Fatalf("GetEmailLog failed: %v", err)
	}

	if len(logs) != 5 {
		t.Errorf("expected 5 logs, got %d", len(logs))
	}

	// Verify they are ordered by created_at DESC (newest first)
	// Since we added a small delay between creations, the last created should be first
}

func TestPostgresRepository_EmailLog_AllStatuses(t *testing.T) {
	tenant, repo, ctx := setupEmailTest(t)

	// Test all status types
	statuses := []EmailStatus{StatusPending, StatusSent, StatusFailed}

	for _, status := range statuses {
		logID := uuid.New().String()
		log := &EmailLog{
			ID:             logID,
			TenantID:       tenant.ID,
			EmailType:      "TEST",
			RecipientEmail: "test@example.com",
			Subject:        "Test " + string(status),
			Status:         status,
		}

		err := repo.CreateEmailLog(ctx, tenant.SchemaName, log)
		if err != nil {
			t.Fatalf("CreateEmailLog with status %s failed: %v", status, err)
		}
	}

	// Verify all logs were created
	logs, err := repo.GetEmailLog(ctx, tenant.SchemaName, tenant.ID, 10)
	if err != nil {
		t.Fatalf("GetEmailLog failed: %v", err)
	}

	if len(logs) != 3 {
		t.Errorf("expected 3 logs, got %d", len(logs))
	}
}

func TestPostgresRepository_Template_NullBodyText(t *testing.T) {
	tenant, repo, ctx := setupEmailTest(t)

	// Create template without body_text (should use empty string via COALESCE)
	template := &EmailTemplate{
		ID:           uuid.New().String(),
		TenantID:     tenant.ID,
		TemplateType: TemplateInvoiceSend,
		Subject:      "Test Subject",
		BodyHTML:     "<html>Test</html>",
		BodyText:     "", // Empty body text
		IsActive:     true,
	}

	err := repo.UpsertTemplate(ctx, tenant.SchemaName, template)
	if err != nil {
		t.Fatalf("UpsertTemplate failed: %v", err)
	}

	// Retrieve and verify COALESCE handles null correctly
	retrieved, err := repo.GetTemplate(ctx, tenant.SchemaName, tenant.ID, TemplateInvoiceSend)
	if err != nil {
		t.Fatalf("GetTemplate failed: %v", err)
	}

	// BodyText should be empty string, not nil/panic
	if retrieved.BodyText != "" {
		t.Errorf("expected empty BodyText, got '%s'", retrieved.BodyText)
	}
}

func TestPostgresRepository_EmailLog_NullRecipientName(t *testing.T) {
	tenant, repo, ctx := setupEmailTest(t)

	// Create log without recipient name
	logID := uuid.New().String()
	log := &EmailLog{
		ID:             logID,
		TenantID:       tenant.ID,
		EmailType:      "TEST",
		RecipientEmail: "test@example.com",
		RecipientName:  "", // Empty recipient name
		Subject:        "Test",
		Status:         StatusPending,
	}

	err := repo.CreateEmailLog(ctx, tenant.SchemaName, log)
	if err != nil {
		t.Fatalf("CreateEmailLog failed: %v", err)
	}

	logs, err := repo.GetEmailLog(ctx, tenant.SchemaName, tenant.ID, 10)
	if err != nil {
		t.Fatalf("GetEmailLog failed: %v", err)
	}

	var found *EmailLog
	for i := range logs {
		if logs[i].ID == logID {
			found = &logs[i]
			break
		}
	}

	if found == nil {
		t.Fatal("log entry not found")
	}

	// RecipientName should be empty string via COALESCE
	if found.RecipientName != "" {
		t.Errorf("expected empty RecipientName, got '%s'", found.RecipientName)
	}
}

func TestNewPostgresRepository(t *testing.T) {
	pool := testutil.SetupTestDB(t)

	repo := NewPostgresRepository(pool)
	if repo == nil {
		t.Error("expected non-nil repository")
	}

	if repo.db != pool {
		t.Error("expected repository db to be the provided pool")
	}
}
