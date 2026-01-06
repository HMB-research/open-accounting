package invoicing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/email"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wneessen/go-mail"
)

func TestNewMockReminderRepository(t *testing.T) {
	repo := NewMockReminderRepository()
	require.NotNil(t, repo)
	assert.Empty(t, repo.OverdueInvoices)
	assert.Empty(t, repo.Reminders)
}

func TestMockReminderRepository_GetOverdueInvoices(t *testing.T) {
	ctx := context.Background()
	repo := NewMockReminderRepository()

	// Test empty result
	invoices, err := repo.GetOverdueInvoices(ctx, "test_schema", "tenant-1", time.Now())
	require.NoError(t, err)
	assert.Empty(t, invoices)

	// Add mock invoices
	repo.AddMockOverdueInvoice(
		"inv-1",
		"INV-2024-001",
		"contact-1",
		"Test Customer",
		"test@example.com",
		"EUR",
		decimal.NewFromInt(1000),
		decimal.NewFromInt(0),
		30,
	)

	invoices, err = repo.GetOverdueInvoices(ctx, "test_schema", "tenant-1", time.Now())
	require.NoError(t, err)
	assert.Len(t, invoices, 1)
	assert.Equal(t, "inv-1", invoices[0].ID)
	assert.Equal(t, "INV-2024-001", invoices[0].InvoiceNumber)
	assert.Equal(t, "Test Customer", invoices[0].ContactName)
}

func TestMockReminderRepository_GetOverdueInvoicesWithError(t *testing.T) {
	ctx := context.Background()
	repo := NewMockReminderRepository()
	repo.GetOverdueErr = assert.AnError

	invoices, err := repo.GetOverdueInvoices(ctx, "test_schema", "tenant-1", time.Now())
	require.Error(t, err)
	assert.Nil(t, invoices)
}

func TestMockReminderRepository_CreateReminder(t *testing.T) {
	ctx := context.Background()
	repo := NewMockReminderRepository()

	reminder := &PaymentReminder{
		ID:            "reminder-1",
		TenantID:      "tenant-1",
		InvoiceID:     "inv-1",
		InvoiceNumber: "INV-2024-001",
		ContactID:     "contact-1",
		ContactName:   "Test Customer",
		ContactEmail:  "test@example.com",
		ReminderNumber: 1,
		Status:        ReminderStatusPending,
	}

	err := repo.CreateReminder(ctx, "test_schema", reminder)
	require.NoError(t, err)

	// Verify it was stored
	reminders := repo.Reminders["inv-1"]
	assert.Len(t, reminders, 1)
	assert.Equal(t, "reminder-1", reminders[0].ID)
}

func TestMockReminderRepository_GetReminderCount(t *testing.T) {
	ctx := context.Background()
	repo := NewMockReminderRepository()

	// Test with no reminders
	count, lastSent, err := repo.GetReminderCount(ctx, "test_schema", "tenant-1", "inv-1")
	require.NoError(t, err)
	assert.Equal(t, 0, count)
	assert.Nil(t, lastSent)

	// Add some reminders
	now := time.Now()
	earlier := now.Add(-24 * time.Hour)

	repo.Reminders["inv-1"] = []PaymentReminder{
		{ID: "r1", InvoiceID: "inv-1", Status: ReminderStatusSent, SentAt: &earlier},
		{ID: "r2", InvoiceID: "inv-1", Status: ReminderStatusSent, SentAt: &now},
		{ID: "r3", InvoiceID: "inv-1", Status: ReminderStatusPending},
		{ID: "r4", InvoiceID: "inv-1", Status: ReminderStatusFailed},
	}

	count, lastSent, err = repo.GetReminderCount(ctx, "test_schema", "tenant-1", "inv-1")
	require.NoError(t, err)
	assert.Equal(t, 2, count) // Only SENT status
	require.NotNil(t, lastSent)
	assert.Equal(t, now.Unix(), lastSent.Unix())
}

func TestMockReminderRepository_UpdateReminderStatus(t *testing.T) {
	ctx := context.Background()
	repo := NewMockReminderRepository()

	now := time.Now()
	repo.Reminders["inv-1"] = []PaymentReminder{
		{ID: "r1", InvoiceID: "inv-1", Status: ReminderStatusPending},
	}

	err := repo.UpdateReminderStatus(ctx, "test_schema", "r1", ReminderStatusSent, &now, "")
	require.NoError(t, err)

	// Verify update
	assert.Equal(t, ReminderStatusSent, repo.Reminders["inv-1"][0].Status)
	assert.NotNil(t, repo.Reminders["inv-1"][0].SentAt)

	// Test with error message
	err = repo.UpdateReminderStatus(ctx, "test_schema", "r1", ReminderStatusFailed, nil, "Email failed")
	require.NoError(t, err)
	assert.Equal(t, ReminderStatusFailed, repo.Reminders["inv-1"][0].Status)
	assert.Equal(t, "Email failed", repo.Reminders["inv-1"][0].ErrorMessage)

	// Test updating non-existent reminder (should not error)
	err = repo.UpdateReminderStatus(ctx, "test_schema", "nonexistent", ReminderStatusSent, &now, "")
	require.NoError(t, err)
}

func TestMockReminderRepository_GetRemindersByInvoice(t *testing.T) {
	ctx := context.Background()
	repo := NewMockReminderRepository()

	// Test empty
	reminders, err := repo.GetRemindersByInvoice(ctx, "test_schema", "tenant-1", "inv-1")
	require.NoError(t, err)
	assert.Empty(t, reminders)

	// Add reminders
	repo.Reminders["inv-1"] = []PaymentReminder{
		{ID: "r1", InvoiceID: "inv-1", ReminderNumber: 1},
		{ID: "r2", InvoiceID: "inv-1", ReminderNumber: 2},
	}
	repo.Reminders["inv-2"] = []PaymentReminder{
		{ID: "r3", InvoiceID: "inv-2", ReminderNumber: 1},
	}

	reminders, err = repo.GetRemindersByInvoice(ctx, "test_schema", "tenant-1", "inv-1")
	require.NoError(t, err)
	assert.Len(t, reminders, 2)

	// Check inv-2
	reminders, err = repo.GetRemindersByInvoice(ctx, "test_schema", "tenant-1", "inv-2")
	require.NoError(t, err)
	assert.Len(t, reminders, 1)
}

func TestMockReminderRepository_AddMockOverdueInvoice(t *testing.T) {
	repo := NewMockReminderRepository()

	repo.AddMockOverdueInvoice(
		"inv-1",
		"INV-2024-001",
		"contact-1",
		"Test Customer",
		"test@example.com",
		"EUR",
		decimal.NewFromInt(1000),
		decimal.NewFromInt(500),
		30,
	)

	require.Len(t, repo.OverdueInvoices, 1)
	inv := repo.OverdueInvoices[0]

	assert.Equal(t, "inv-1", inv.ID)
	assert.Equal(t, "INV-2024-001", inv.InvoiceNumber)
	assert.Equal(t, "contact-1", inv.ContactID)
	assert.Equal(t, "Test Customer", inv.ContactName)
	assert.Equal(t, "test@example.com", inv.ContactEmail)
	assert.Equal(t, "EUR", inv.Currency)
	assert.True(t, inv.Total.Equal(decimal.NewFromInt(1000)))
	assert.True(t, inv.AmountPaid.Equal(decimal.NewFromInt(500)))
	assert.True(t, inv.OutstandingAmount.Equal(decimal.NewFromInt(500)))
	assert.Equal(t, 30, inv.DaysOverdue)
}

func TestReminderStatusConstants(t *testing.T) {
	assert.Equal(t, ReminderStatus("PENDING"), ReminderStatusPending)
	assert.Equal(t, ReminderStatus("SENT"), ReminderStatusSent)
	assert.Equal(t, ReminderStatus("FAILED"), ReminderStatusFailed)
	assert.Equal(t, ReminderStatus("CANCELED"), ReminderStatusCanceled)
}

// Test ReminderService with mock repository
func TestNewReminderServiceWithRepository(t *testing.T) {
	repo := NewMockReminderRepository()
	svc := NewReminderServiceWithRepository(repo, nil)
	require.NotNil(t, svc)
}

func TestReminderService_GetOverdueInvoicesSummary(t *testing.T) {
	ctx := context.Background()
	repo := NewMockReminderRepository()
	svc := NewReminderServiceWithRepository(repo, nil)

	// Add mock overdue invoices
	repo.AddMockOverdueInvoice("inv-1", "INV-001", "c1", "Customer 1", "c1@test.com", "EUR",
		decimal.NewFromInt(1000), decimal.Zero, 30)
	repo.AddMockOverdueInvoice("inv-2", "INV-002", "c1", "Customer 1", "c1@test.com", "EUR",
		decimal.NewFromInt(500), decimal.Zero, 15)
	repo.AddMockOverdueInvoice("inv-3", "INV-003", "c2", "Customer 2", "c2@test.com", "EUR",
		decimal.NewFromInt(2000), decimal.NewFromInt(500), 45)

	summary, err := svc.GetOverdueInvoicesSummary(ctx, "test_schema", "tenant-1")
	require.NoError(t, err)

	assert.Equal(t, 3, summary.InvoiceCount)
	assert.Equal(t, 2, summary.ContactCount) // 2 unique contacts
	assert.True(t, summary.TotalOverdue.Equal(decimal.NewFromInt(3000)))
	assert.Equal(t, 30, summary.AverageDaysOverdue) // (30+15+45)/3 = 30
	assert.Len(t, summary.Invoices, 3)
}

func TestReminderService_GetOverdueInvoicesSummary_Empty(t *testing.T) {
	ctx := context.Background()
	repo := NewMockReminderRepository()
	svc := NewReminderServiceWithRepository(repo, nil)

	summary, err := svc.GetOverdueInvoicesSummary(ctx, "test_schema", "tenant-1")
	require.NoError(t, err)

	assert.Equal(t, 0, summary.InvoiceCount)
	assert.Equal(t, 0, summary.ContactCount)
	assert.True(t, summary.TotalOverdue.IsZero())
	assert.Equal(t, 0, summary.AverageDaysOverdue)
}

func TestReminderService_GetOverdueInvoicesSummary_Error(t *testing.T) {
	ctx := context.Background()
	repo := NewMockReminderRepository()
	repo.GetOverdueErr = assert.AnError
	svc := NewReminderServiceWithRepository(repo, nil)

	_, err := svc.GetOverdueInvoicesSummary(ctx, "test_schema", "tenant-1")
	require.Error(t, err)
}

func TestReminderService_GetReminderHistory(t *testing.T) {
	ctx := context.Background()
	repo := NewMockReminderRepository()
	svc := NewReminderServiceWithRepository(repo, nil)

	// Add reminders
	now := time.Now()
	repo.Reminders["inv-1"] = []PaymentReminder{
		{ID: "r1", InvoiceID: "inv-1", ReminderNumber: 1, Status: ReminderStatusSent, SentAt: &now},
		{ID: "r2", InvoiceID: "inv-1", ReminderNumber: 2, Status: ReminderStatusPending},
	}

	reminders, err := svc.GetReminderHistory(ctx, "test_schema", "tenant-1", "inv-1")
	require.NoError(t, err)
	assert.Len(t, reminders, 2)
}

// TestReminderService_SendReminder tests SendReminder functionality
func TestReminderService_SendReminder_InvoiceNotFound(t *testing.T) {
	ctx := context.Background()
	repo := NewMockReminderRepository()
	svc := NewReminderServiceWithRepository(repo, nil)

	// No invoices in the mock repo - invoice should not be found
	req := &SendReminderRequest{InvoiceID: "non-existent"}
	result, err := svc.SendReminder(ctx, "tenant-1", "test_schema", req, "Test Company")

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Equal(t, "Invoice not found or not overdue", result.Message)
}

func TestReminderService_SendReminder_NoContactEmail(t *testing.T) {
	ctx := context.Background()
	repo := NewMockReminderRepository()
	svc := NewReminderServiceWithRepository(repo, nil)

	// Add an invoice with no email
	repo.AddMockOverdueInvoice("inv-1", "INV-001", "c1", "Customer 1", "", "EUR",
		decimal.NewFromInt(1000), decimal.Zero, 30)

	req := &SendReminderRequest{InvoiceID: "inv-1"}
	result, err := svc.SendReminder(ctx, "tenant-1", "test_schema", req, "Test Company")

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "does not have an email address")
}

func TestReminderService_SendReminder_GetOverdueInvoicesError(t *testing.T) {
	ctx := context.Background()
	repo := NewMockReminderRepository()
	repo.GetOverdueErr = assert.AnError
	svc := NewReminderServiceWithRepository(repo, nil)

	req := &SendReminderRequest{InvoiceID: "inv-1"}
	_, err := svc.SendReminder(ctx, "tenant-1", "test_schema", req, "Test Company")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "get overdue invoices")
}

// TestReminderService_SendBulkReminders tests bulk reminder sending
func TestReminderService_SendBulkReminders_AllNotFound(t *testing.T) {
	ctx := context.Background()
	repo := NewMockReminderRepository()
	svc := NewReminderServiceWithRepository(repo, nil)

	req := &SendBulkRemindersRequest{
		InvoiceIDs: []string{"inv-1", "inv-2", "inv-3"},
		Message:    "Please pay",
	}

	result, err := svc.SendBulkReminders(ctx, "tenant-1", "test_schema", req, "Test Company")
	require.NoError(t, err)

	assert.Equal(t, 3, result.TotalRequested)
	assert.Equal(t, 0, result.Successful)
	assert.Equal(t, 3, result.Failed)
	assert.Len(t, result.Results, 3)
}

func TestReminderService_SendBulkReminders_MixedResults(t *testing.T) {
	ctx := context.Background()
	repo := NewMockReminderRepository()
	svc := NewReminderServiceWithRepository(repo, nil)

	// Add some overdue invoices - one with email, one without
	repo.AddMockOverdueInvoice("inv-1", "INV-001", "c1", "Customer 1", "", "EUR",
		decimal.NewFromInt(1000), decimal.Zero, 30) // No email - will fail
	repo.AddMockOverdueInvoice("inv-2", "INV-002", "c2", "Customer 2", "", "EUR",
		decimal.NewFromInt(500), decimal.Zero, 15) // No email - will fail

	req := &SendBulkRemindersRequest{
		InvoiceIDs: []string{"inv-1", "inv-2", "inv-3"},
		Message:    "Please pay",
	}

	result, err := svc.SendBulkReminders(ctx, "tenant-1", "test_schema", req, "Test Company")
	require.NoError(t, err)

	assert.Equal(t, 3, result.TotalRequested)
	// inv-1 and inv-2 fail because no email, inv-3 fails because not found
	assert.Equal(t, 0, result.Successful)
	assert.Equal(t, 3, result.Failed)
}

func TestReminderService_SendBulkReminders_Empty(t *testing.T) {
	ctx := context.Background()
	repo := NewMockReminderRepository()
	svc := NewReminderServiceWithRepository(repo, nil)

	req := &SendBulkRemindersRequest{
		InvoiceIDs: []string{},
	}

	result, err := svc.SendBulkReminders(ctx, "tenant-1", "test_schema", req, "Test Company")
	require.NoError(t, err)

	assert.Equal(t, 0, result.TotalRequested)
	assert.Equal(t, 0, result.Successful)
	assert.Equal(t, 0, result.Failed)
}

// TestReminderService_GetOverdueInvoicesSummary_GetReminderCountError tests error path
func TestReminderService_GetOverdueInvoicesSummary_GetReminderCountError(t *testing.T) {
	ctx := context.Background()

	// Create a mock that errors on GetReminderCount
	repo := NewMockReminderRepository()
	repo.GetReminderCountErr = assert.AnError
	repo.AddMockOverdueInvoice("inv-1", "INV-001", "c1", "Customer 1", "c1@test.com", "EUR",
		decimal.NewFromInt(1000), decimal.Zero, 30)

	svc := NewReminderServiceWithRepository(repo, nil)

	_, err := svc.GetOverdueInvoicesSummary(ctx, "test_schema", "tenant-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get reminder count")
}

// TestReminderService_SendReminder_GetReminderCountError tests the GetReminderCount error path in SendReminder
func TestReminderService_SendReminder_GetReminderCountError(t *testing.T) {
	ctx := context.Background()
	repo := NewMockReminderRepository()
	repo.AddMockOverdueInvoice("inv-1", "INV-001", "c1", "Customer 1", "c1@test.com", "EUR",
		decimal.NewFromInt(1000), decimal.Zero, 30)
	repo.GetReminderCountErr = assert.AnError
	svc := NewReminderServiceWithRepository(repo, nil)

	req := &SendReminderRequest{InvoiceID: "inv-1"}
	_, err := svc.SendReminder(ctx, "tenant-1", "test_schema", req, "Test Company")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "get reminder count")
}

// TestReminderService_SendReminder_CreateReminderError tests the CreateReminder error path in SendReminder
func TestReminderService_SendReminder_CreateReminderError(t *testing.T) {
	ctx := context.Background()
	repo := NewMockReminderRepository()
	repo.AddMockOverdueInvoice("inv-1", "INV-001", "c1", "Customer 1", "c1@test.com", "EUR",
		decimal.NewFromInt(1000), decimal.Zero, 30)
	repo.CreateReminderErr = assert.AnError
	svc := NewReminderServiceWithRepository(repo, nil)

	req := &SendReminderRequest{InvoiceID: "inv-1"}
	_, err := svc.SendReminder(ctx, "tenant-1", "test_schema", req, "Test Company")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "create reminder")
}

// Test SendReminderRequest validation
func TestSendReminderRequest_Defaults(t *testing.T) {
	req := &SendReminderRequest{
		InvoiceID: "inv-1",
		Message:   "Custom message",
	}

	assert.Equal(t, "inv-1", req.InvoiceID)
	assert.Equal(t, "Custom message", req.Message)
}

// Test SendBulkRemindersRequest validation
func TestSendBulkRemindersRequest_Defaults(t *testing.T) {
	req := &SendBulkRemindersRequest{
		InvoiceIDs: []string{"inv-1", "inv-2"},
		Message:    "Bulk message",
	}

	assert.Len(t, req.InvoiceIDs, 2)
	assert.Equal(t, "Bulk message", req.Message)
}

// Test PaymentReminder struct initialization
func TestPaymentReminder_Initialization(t *testing.T) {
	now := time.Now()
	reminder := &PaymentReminder{
		ID:             "rem-1",
		TenantID:       "tenant-1",
		InvoiceID:      "inv-1",
		InvoiceNumber:  "INV-001",
		ContactID:      "contact-1",
		ContactName:    "Test Customer",
		ContactEmail:   "test@example.com",
		ReminderNumber: 1,
		Status:         ReminderStatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	assert.Equal(t, "rem-1", reminder.ID)
	assert.Equal(t, ReminderStatusPending, reminder.Status)
	assert.Equal(t, 1, reminder.ReminderNumber)
}

// Test ReminderResult struct
func TestReminderResult_Success(t *testing.T) {
	result := ReminderResult{
		InvoiceID:     "inv-1",
		InvoiceNumber: "INV-001",
		Success:       true,
		Message:       "Sent successfully",
		ReminderID:    "rem-1",
	}

	assert.True(t, result.Success)
	assert.Equal(t, "rem-1", result.ReminderID)
}

func TestReminderResult_Failure(t *testing.T) {
	result := ReminderResult{
		InvoiceID: "inv-1",
		Success:   false,
		Message:   "Failed to send",
	}

	assert.False(t, result.Success)
	assert.Empty(t, result.ReminderID)
}

// Test BulkReminderResult struct
func TestBulkReminderResult_Aggregation(t *testing.T) {
	result := BulkReminderResult{
		TotalRequested: 5,
		Successful:     3,
		Failed:         2,
		Results: []ReminderResult{
			{InvoiceID: "inv-1", Success: true},
			{InvoiceID: "inv-2", Success: true},
			{InvoiceID: "inv-3", Success: true},
			{InvoiceID: "inv-4", Success: false},
			{InvoiceID: "inv-5", Success: false},
		},
	}

	assert.Equal(t, 5, result.TotalRequested)
	assert.Equal(t, 3, result.Successful)
	assert.Equal(t, 2, result.Failed)
	assert.Len(t, result.Results, 5)
}

// Test OverdueInvoice struct
func TestOverdueInvoice_OutstandingCalculation(t *testing.T) {
	inv := OverdueInvoice{
		ID:                "inv-1",
		InvoiceNumber:     "INV-001",
		ContactID:         "c1",
		ContactName:       "Customer 1",
		ContactEmail:      "c1@test.com",
		IssueDate:         "2024-01-01",
		DueDate:           "2024-01-15",
		Total:             decimal.NewFromInt(1000),
		AmountPaid:        decimal.NewFromInt(300),
		OutstandingAmount: decimal.NewFromInt(700),
		Currency:          "EUR",
		DaysOverdue:       15,
	}

	assert.True(t, inv.OutstandingAmount.Equal(inv.Total.Sub(inv.AmountPaid)))
	assert.Equal(t, 15, inv.DaysOverdue)
}

// Test OverdueInvoicesSummary struct
func TestOverdueInvoicesSummary_Statistics(t *testing.T) {
	summary := OverdueInvoicesSummary{
		TotalOverdue:       decimal.NewFromInt(5000),
		InvoiceCount:       10,
		ContactCount:       5,
		AverageDaysOverdue: 20,
		Invoices:           make([]OverdueInvoice, 10),
		GeneratedAt:        time.Now(),
	}

	assert.Equal(t, 10, summary.InvoiceCount)
	assert.Equal(t, 5, summary.ContactCount)
	assert.Equal(t, 20, summary.AverageDaysOverdue)
	assert.True(t, summary.TotalOverdue.Equal(decimal.NewFromInt(5000)))
}

// TestNewReminderService tests the NewReminderService constructor with nil pool
func TestNewReminderService(t *testing.T) {
	// NewReminderService should create a service with nil pool (won't panic until used)
	svc := NewReminderService(nil, nil)
	require.NotNil(t, svc)
	assert.Nil(t, svc.db)
	assert.Nil(t, svc.emailService)
	assert.NotNil(t, svc.repo)
}

// Mock email repository for testing
type mockEmailRepository struct {
	GetTenantSettingsResult    []byte
	GetTenantSettingsErr       error
	GetTemplateResult          *email.EmailTemplate
	GetTemplateErr             error
	EnsureSchemaErr            error
	CreateEmailLogErr          error
	UpdateEmailLogStatusErr    error
}

func (m *mockEmailRepository) EnsureSchema(ctx context.Context, schemaName string) error {
	return m.EnsureSchemaErr
}
func (m *mockEmailRepository) GetTenantSettings(ctx context.Context, tenantID string) ([]byte, error) {
	if m.GetTenantSettingsErr != nil {
		return nil, m.GetTenantSettingsErr
	}
	if m.GetTenantSettingsResult != nil {
		return m.GetTenantSettingsResult, nil
	}
	// Return default valid SMTP settings
	return []byte(`{"smtp_host":"smtp.test.com","smtp_port":587,"smtp_username":"user","smtp_password":"pass","smtp_from_email":"test@test.com","smtp_use_tls":true}`), nil
}
func (m *mockEmailRepository) UpdateTenantSettings(ctx context.Context, tenantID string, settingsJSON []byte) error {
	return nil
}
func (m *mockEmailRepository) GetTemplate(ctx context.Context, schemaName, tenantID string, templateType email.TemplateType) (*email.EmailTemplate, error) {
	if m.GetTemplateErr != nil {
		return nil, m.GetTemplateErr
	}
	if m.GetTemplateResult != nil {
		return m.GetTemplateResult, nil
	}
	return nil, email.ErrTemplateNotFound
}
func (m *mockEmailRepository) ListTemplates(ctx context.Context, schemaName, tenantID string) ([]email.EmailTemplate, error) {
	return []email.EmailTemplate{}, nil
}
func (m *mockEmailRepository) UpsertTemplate(ctx context.Context, schemaName string, template *email.EmailTemplate) error {
	return nil
}
func (m *mockEmailRepository) CreateEmailLog(ctx context.Context, schemaName string, log *email.EmailLog) error {
	return m.CreateEmailLogErr
}
func (m *mockEmailRepository) UpdateEmailLogStatus(ctx context.Context, schemaName, logID string, status email.EmailStatus, sentAt *time.Time, errorMessage string) error {
	return m.UpdateEmailLogStatusErr
}
func (m *mockEmailRepository) GetEmailLog(ctx context.Context, schemaName, tenantID string, limit int) ([]email.EmailLog, error) {
	return []email.EmailLog{}, nil
}

// Mock mail sender for testing
type mockMailSender struct {
	SendMailErr    error
	SendMailCalled bool
}

func (m *mockMailSender) SendMail(config *email.SMTPConfig, msg *mail.Msg) error {
	m.SendMailCalled = true
	return m.SendMailErr
}

// TestReminderService_SendReminder_GetTemplateError tests the GetTemplate error path
func TestReminderService_SendReminder_GetTemplateError(t *testing.T) {
	ctx := context.Background()
	repo := NewMockReminderRepository()
	repo.AddMockOverdueInvoice("inv-1", "INV-001", "c1", "Customer 1", "c1@test.com", "EUR",
		decimal.NewFromInt(1000), decimal.Zero, 30)

	// Create email service with mock that errors on GetTemplate
	emailRepo := &mockEmailRepository{
		GetTemplateErr: errors.New("template error"),
	}
	emailSvc := email.NewServiceWithRepository(emailRepo, &mockMailSender{})
	svc := NewReminderServiceWithRepository(repo, emailSvc)

	req := &SendReminderRequest{InvoiceID: "inv-1"}
	result, err := svc.SendReminder(ctx, "tenant-1", "test_schema", req, "Test Company")

	require.NoError(t, err) // SendReminder returns ReminderResult, not error, for template failures
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "Failed to get email template")
}

// TestReminderService_SendReminder_SendEmailError tests the SendEmail error path
func TestReminderService_SendReminder_SendEmailError(t *testing.T) {
	ctx := context.Background()
	repo := NewMockReminderRepository()
	repo.AddMockOverdueInvoice("inv-1", "INV-001", "c1", "Customer 1", "c1@test.com", "EUR",
		decimal.NewFromInt(1000), decimal.Zero, 30)

	// Create email service with mock that succeeds on template but fails on send
	emailRepo := &mockEmailRepository{}
	mailer := &mockMailSender{SendMailErr: errors.New("SMTP connection failed")}
	emailSvc := email.NewServiceWithRepository(emailRepo, mailer)
	svc := NewReminderServiceWithRepository(repo, emailSvc)

	req := &SendReminderRequest{InvoiceID: "inv-1"}
	result, err := svc.SendReminder(ctx, "tenant-1", "test_schema", req, "Test Company")

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "Failed to send email")
}

// TestReminderService_SendReminder_Success tests successful email sending
func TestReminderService_SendReminder_Success(t *testing.T) {
	ctx := context.Background()
	repo := NewMockReminderRepository()
	repo.AddMockOverdueInvoice("inv-1", "INV-001", "c1", "Customer 1", "c1@test.com", "EUR",
		decimal.NewFromInt(1000), decimal.Zero, 30)

	// Create email service with all mocks succeeding
	emailRepo := &mockEmailRepository{}
	mailer := &mockMailSender{}
	emailSvc := email.NewServiceWithRepository(emailRepo, mailer)
	svc := NewReminderServiceWithRepository(repo, emailSvc)

	req := &SendReminderRequest{InvoiceID: "inv-1", Message: "Please pay promptly"}
	result, err := svc.SendReminder(ctx, "tenant-1", "test_schema", req, "Test Company")

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.Message, "sent successfully")
	assert.NotEmpty(t, result.ReminderID)
	assert.True(t, mailer.SendMailCalled)
}

// TestReminderService_SendBulkReminders_WithEmailService tests bulk reminders with email service
func TestReminderService_SendBulkReminders_WithEmailService(t *testing.T) {
	ctx := context.Background()
	repo := NewMockReminderRepository()
	repo.AddMockOverdueInvoice("inv-1", "INV-001", "c1", "Customer 1", "c1@test.com", "EUR",
		decimal.NewFromInt(1000), decimal.Zero, 30)
	repo.AddMockOverdueInvoice("inv-2", "INV-002", "c2", "Customer 2", "c2@test.com", "EUR",
		decimal.NewFromInt(500), decimal.Zero, 15)

	// Create email service
	emailRepo := &mockEmailRepository{}
	mailer := &mockMailSender{}
	emailSvc := email.NewServiceWithRepository(emailRepo, mailer)
	svc := NewReminderServiceWithRepository(repo, emailSvc)

	req := &SendBulkRemindersRequest{
		InvoiceIDs: []string{"inv-1", "inv-2", "inv-3"},
		Message:    "Please pay",
	}

	result, err := svc.SendBulkReminders(ctx, "tenant-1", "test_schema", req, "Test Company")
	require.NoError(t, err)

	assert.Equal(t, 3, result.TotalRequested)
	assert.Equal(t, 2, result.Successful) // inv-1 and inv-2 succeed
	assert.Equal(t, 1, result.Failed)     // inv-3 not found
}
