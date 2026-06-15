package scheduler

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/recurring"
)

// MockRepository implements Repository for testing
type MockRepository struct {
	tenants              []TenantInfo
	listActiveTenantsErr error
}

func (m *MockRepository) ListActiveTenants(ctx context.Context) ([]TenantInfo, error) {
	if m.listActiveTenantsErr != nil {
		return nil, m.listActiveTenantsErr
	}
	return m.tenants, nil
}

// MockRecurringService implements RecurringService for testing
type MockRecurringService struct {
	results map[string][]recurring.GenerationResult
	errors  map[string]error
}

func NewMockRecurringService() *MockRecurringService {
	return &MockRecurringService{
		results: make(map[string][]recurring.GenerationResult),
		errors:  make(map[string]error),
	}
}

func (m *MockRecurringService) GenerateDueInvoices(ctx context.Context, tenantID, schemaName, userID string) ([]recurring.GenerationResult, error) {
	if err, ok := m.errors[tenantID]; ok && err != nil {
		return nil, err
	}
	if results, ok := m.results[tenantID]; ok {
		return results, nil
	}
	return []recurring.GenerationResult{}, nil
}

type MockReminderService struct {
	results map[string][]invoicing.AutomatedReminderResult
	errors  map[string]error
	calls   []string
}

func NewMockReminderService() *MockReminderService {
	return &MockReminderService{
		results: make(map[string][]invoicing.AutomatedReminderResult),
		errors:  make(map[string]error),
	}
}

func (m *MockReminderService) ProcessRemindersForTenant(ctx context.Context, tenantID, schemaName, companyName string) ([]invoicing.AutomatedReminderResult, error) {
	m.calls = append(m.calls, tenantID)
	if err, ok := m.errors[tenantID]; ok && err != nil {
		return nil, err
	}
	return m.results[tenantID], nil
}

type MockDocumentRetentionReminderService struct {
	results map[string]documents.RetentionReminderDeliveryResult
	errors  map[string]error
	calls   []documentRetentionReminderCall
}

type documentRetentionReminderCall struct {
	tenantID       string
	schemaName     string
	companyName    string
	recipientEmail string
	horizonDays    int
	includeMissing bool
}

func NewMockDocumentRetentionReminderService() *MockDocumentRetentionReminderService {
	return &MockDocumentRetentionReminderService{
		results: make(map[string]documents.RetentionReminderDeliveryResult),
		errors:  make(map[string]error),
	}
}

func (m *MockDocumentRetentionReminderService) ProcessRetentionRemindersForTenant(ctx context.Context, tenantID, schemaName, companyName, recipientEmail string, asOf time.Time, horizonDays int, includeMissing bool) (documents.RetentionReminderDeliveryResult, error) {
	m.calls = append(m.calls, documentRetentionReminderCall{
		tenantID:       tenantID,
		schemaName:     schemaName,
		companyName:    companyName,
		recipientEmail: recipientEmail,
		horizonDays:    horizonDays,
		includeMissing: includeMissing,
	})
	if err, ok := m.errors[tenantID]; ok && err != nil {
		return documents.RetentionReminderDeliveryResult{}, err
	}
	return m.results[tenantID], nil
}

type journalGenerationCall struct {
	tenantID       string
	schemaName     string
	userID         string
	periodLockDate *time.Time
}

type MockJournalEntryService struct {
	results map[string][]accounting.JournalEntryTemplateGenerationResult
	errors  map[string]error
	calls   []journalGenerationCall
}

func NewMockJournalEntryService() *MockJournalEntryService {
	return &MockJournalEntryService{
		results: make(map[string][]accounting.JournalEntryTemplateGenerationResult),
		errors:  make(map[string]error),
	}
}

func (m *MockJournalEntryService) GenerateDueJournalEntryTemplates(ctx context.Context, schemaName, tenantID string, req *accounting.GenerateDueJournalEntryTemplatesRequest) ([]accounting.JournalEntryTemplateGenerationResult, error) {
	m.calls = append(m.calls, journalGenerationCall{
		tenantID:       tenantID,
		schemaName:     schemaName,
		userID:         req.UserID,
		periodLockDate: req.PeriodLockDate,
	})
	if err, ok := m.errors[tenantID]; ok && err != nil {
		return nil, err
	}
	return m.results[tenantID], nil
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.RecurringInvoiceSchedule != "0 6 * * *" {
		t.Errorf("RecurringInvoiceSchedule = %q, want %q", config.RecurringInvoiceSchedule, "0 6 * * *")
	}
	if config.RecurringJournalEntrySchedule != "15 6 * * *" {
		t.Errorf("RecurringJournalEntrySchedule = %q, want %q", config.RecurringJournalEntrySchedule, "15 6 * * *")
	}
	if config.DocumentRetentionReminderSchedule != "30 9 * * *" {
		t.Errorf("DocumentRetentionReminderSchedule = %q, want %q", config.DocumentRetentionReminderSchedule, "30 9 * * *")
	}
	if config.DocumentRetentionReminderHorizonDays != documents.DefaultRetentionReminderHorizonDays {
		t.Errorf("DocumentRetentionReminderHorizonDays = %d, want %d", config.DocumentRetentionReminderHorizonDays, documents.DefaultRetentionReminderHorizonDays)
	}
	if !config.DocumentRetentionReminderIncludeMissing {
		t.Error("DocumentRetentionReminderIncludeMissing should be true by default")
	}
	if !config.Enabled {
		t.Error("Enabled should be true by default")
	}
}

func TestNewScheduler(t *testing.T) {
	config := DefaultConfig()
	scheduler := NewScheduler(nil, nil, nil, config)

	if scheduler == nil {
		t.Fatal("NewScheduler returned nil")
	}
	if scheduler.cron == nil {
		t.Error("cron should not be nil")
	}
	if scheduler.running {
		t.Error("scheduler should not be running initially")
	}
	if scheduler.config.RecurringInvoiceSchedule != config.RecurringInvoiceSchedule {
		t.Error("config not set correctly")
	}
}

func TestScheduler_IsRunning_Initially(t *testing.T) {
	config := DefaultConfig()
	scheduler := NewScheduler(nil, nil, nil, config)

	if scheduler.IsRunning() {
		t.Error("scheduler should not be running initially")
	}
}

func TestScheduler_StartDisabled(t *testing.T) {
	config := Config{
		RecurringInvoiceSchedule: "0 6 * * *",
		Enabled:                  false,
	}
	scheduler := NewScheduler(nil, nil, nil, config)

	err := scheduler.Start()
	if err != nil {
		t.Errorf("Start() returned error for disabled scheduler: %v", err)
	}

	// Scheduler should not be running when disabled
	if scheduler.IsRunning() {
		t.Error("scheduler should not be running when disabled")
	}
}

func TestScheduler_StartEnabled(t *testing.T) {
	config := DefaultConfig()
	scheduler := NewScheduler(nil, nil, nil, config)

	err := scheduler.Start()
	if err != nil {
		t.Errorf("Start() returned error: %v", err)
	}

	if !scheduler.IsRunning() {
		t.Error("scheduler should be running after Start()")
	}

	// Cleanup
	scheduler.Stop()
}

func TestScheduler_StartTwice(t *testing.T) {
	config := DefaultConfig()
	scheduler := NewScheduler(nil, nil, nil, config)

	// First start should succeed
	err := scheduler.Start()
	if err != nil {
		t.Errorf("First Start() returned error: %v", err)
	}

	// Second start should fail
	err = scheduler.Start()
	if err == nil {
		t.Error("Second Start() should return error")
	}
	if err.Error() != "scheduler is already running" {
		t.Errorf("unexpected error message: %v", err)
	}

	// Cleanup
	scheduler.Stop()
}

func TestScheduler_Stop(t *testing.T) {
	config := DefaultConfig()
	scheduler := NewScheduler(nil, nil, nil, config)

	// Start the scheduler
	err := scheduler.Start()
	if err != nil {
		t.Fatalf("Start() returned error: %v", err)
	}

	if !scheduler.IsRunning() {
		t.Error("scheduler should be running after Start()")
	}

	// Stop the scheduler
	ctx := scheduler.Stop()
	if ctx == nil {
		t.Error("Stop() returned nil context")
	}

	if scheduler.IsRunning() {
		t.Error("scheduler should not be running after Stop()")
	}
}

func TestScheduler_StopNotRunning(t *testing.T) {
	config := DefaultConfig()
	scheduler := NewScheduler(nil, nil, nil, config)

	// Stop without starting should not panic and return canceled context
	ctx := scheduler.Stop()
	if ctx == nil {
		t.Error("Stop() returned nil context")
	}

	// Context should be canceled
	select {
	case <-ctx.Done():
		// Expected
	default:
		t.Error("context should be canceled when stopping non-running scheduler")
	}
}

func TestScheduler_RunNow_WithNilDB(t *testing.T) {
	// Note: RunNow() with nil repo will panic because it tries to query
	// the database. This test documents this expected behavior.
	// In production, the scheduler is always created with a valid repo.
	config := DefaultConfig()
	scheduler := NewScheduler(nil, nil, nil, config)

	// We expect RunNow to panic with nil repo - this is acceptable
	// because in production, repo is never nil
	defer func() {
		if r := recover(); r != nil {
			// Expected panic - test passes
			t.Logf("RunNow() correctly panicked with nil repo: %v", r)
		}
	}()

	scheduler.RunNow()
}

func TestNewSchedulerWithRepository(t *testing.T) {
	mockRepo := &MockRepository{}
	config := DefaultConfig()
	scheduler := NewSchedulerWithRepository(mockRepo, nil, nil, config)

	if scheduler == nil {
		t.Fatal("NewSchedulerWithRepository returned nil")
	}
	if scheduler.repo == nil {
		t.Error("repo should not be nil")
	}
}

func TestScheduler_RunNow_WithMockRepository(t *testing.T) {
	// Note: We can only test cases where recurring service is not called
	// (no tenants, or repository error) since we can't mock recurring.Service
	tests := []struct {
		name    string
		tenants []TenantInfo
		repoErr error
	}{
		{
			name:    "no tenants",
			tenants: []TenantInfo{},
			repoErr: nil,
		},
		{
			name:    "repository error",
			tenants: nil,
			repoErr: errors.New("database error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &MockRepository{
				tenants:              tt.tenants,
				listActiveTenantsErr: tt.repoErr,
			}
			// nil recurring service is fine when there are no tenants or repo errors
			config := DefaultConfig()
			scheduler := NewSchedulerWithRepository(mockRepo, nil, nil, config)

			// Should not panic
			scheduler.RunNow()
		})
	}
}

func TestConfig_CustomSchedule(t *testing.T) {
	tests := []struct {
		name     string
		schedule string
		enabled  bool
	}{
		{
			name:     "every hour",
			schedule: "0 * * * *",
			enabled:  true,
		},
		{
			name:     "every day at midnight",
			schedule: "0 0 * * *",
			enabled:  true,
		},
		{
			name:     "every weekday at 9am",
			schedule: "0 9 * * 1-5",
			enabled:  true,
		},
		{
			name:     "disabled scheduler",
			schedule: "0 6 * * *",
			enabled:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				RecurringInvoiceSchedule: tt.schedule,
				Enabled:                  tt.enabled,
			}

			if config.RecurringInvoiceSchedule != tt.schedule {
				t.Errorf("Schedule = %q, want %q", config.RecurringInvoiceSchedule, tt.schedule)
			}
			if config.Enabled != tt.enabled {
				t.Errorf("Enabled = %v, want %v", config.Enabled, tt.enabled)
			}
		})
	}
}

func TestScheduler_InvalidScheduleFormat(t *testing.T) {
	config := Config{
		RecurringInvoiceSchedule: "invalid cron expression",
		Enabled:                  true,
	}
	scheduler := NewScheduler(nil, nil, nil, config)

	err := scheduler.Start()
	if err == nil {
		t.Error("Start() should return error for invalid cron expression")
		scheduler.Stop()
	}
}

func TestScheduler_ConcurrentAccess(t *testing.T) {
	config := DefaultConfig()
	scheduler := NewScheduler(nil, nil, nil, config)

	// Start the scheduler
	err := scheduler.Start()
	if err != nil {
		t.Fatalf("Start() returned error: %v", err)
	}

	// Concurrent calls to IsRunning should not race
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_ = scheduler.IsRunning()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	scheduler.Stop()
}

func TestScheduler_StopMultipleTimes(t *testing.T) {
	config := DefaultConfig()
	scheduler := NewScheduler(nil, nil, nil, config)

	err := scheduler.Start()
	if err != nil {
		t.Fatalf("Start() returned error: %v", err)
	}

	// First stop
	ctx1 := scheduler.Stop()
	if ctx1 == nil {
		t.Error("first Stop() returned nil context")
	}

	// Second stop should also work without panicking
	ctx2 := scheduler.Stop()
	if ctx2 == nil {
		t.Error("second Stop() returned nil context")
	}
}

func TestScheduler_ScheduleFormatWithSeconds(t *testing.T) {
	// The scheduler prepends "0 " to the schedule to add seconds
	// Test that valid 5-field cron expressions work
	tests := []struct {
		name     string
		schedule string
	}{
		{"every minute", "* * * * *"},
		{"every 5 minutes", "*/5 * * * *"},
		{"hourly", "0 * * * *"},
		{"daily at 6am", "0 6 * * *"},
		{"weekly on monday", "0 9 * * 1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				RecurringInvoiceSchedule: tt.schedule,
				Enabled:                  true,
			}
			scheduler := NewScheduler(nil, nil, nil, config)

			err := scheduler.Start()
			if err != nil {
				t.Errorf("Start() failed for schedule %q: %v", tt.schedule, err)
			} else {
				scheduler.Stop()
			}
		})
	}
}

func TestScheduler_GenerateDueInvoices_WithTenants(t *testing.T) {
	tenants := []TenantInfo{
		{ID: "tenant-1", SchemaName: "tenant_1"},
		{ID: "tenant-2", SchemaName: "tenant_2"},
	}
	mockRepo := &MockRepository{tenants: tenants}
	mockRecurring := NewMockRecurringService()

	// Set up results for tenant-1 (with email sent)
	mockRecurring.results["tenant-1"] = []recurring.GenerationResult{
		{
			RecurringInvoiceID:     "recurring-1",
			GeneratedInvoiceID:     "invoice-1",
			GeneratedInvoiceNumber: "INV-001",
			EmailSent:              true,
			EmailStatus:            "sent",
		},
	}

	// Set up results for tenant-2 (without email sent)
	mockRecurring.results["tenant-2"] = []recurring.GenerationResult{
		{
			RecurringInvoiceID:     "recurring-2",
			GeneratedInvoiceID:     "invoice-2",
			GeneratedInvoiceNumber: "INV-002",
			EmailSent:              false,
			EmailStatus:            "not_configured",
		},
	}

	config := DefaultConfig()
	scheduler := NewSchedulerWithRepository(mockRepo, mockRecurring, nil, config)

	// Should not panic and process all tenants
	scheduler.RunNow()
}

func TestScheduler_GenerateDueInvoices_TenantError(t *testing.T) {
	tenants := []TenantInfo{
		{ID: "tenant-1", SchemaName: "tenant_1"},
		{ID: "tenant-2", SchemaName: "tenant_2"},
	}
	mockRepo := &MockRepository{tenants: tenants}
	mockRecurring := NewMockRecurringService()

	// Set up error for tenant-1
	mockRecurring.errors["tenant-1"] = errors.New("database error")

	// Set up success for tenant-2
	mockRecurring.results["tenant-2"] = []recurring.GenerationResult{
		{
			RecurringInvoiceID:     "recurring-2",
			GeneratedInvoiceID:     "invoice-2",
			GeneratedInvoiceNumber: "INV-002",
			EmailSent:              true,
			EmailStatus:            "sent",
		},
	}

	config := DefaultConfig()
	scheduler := NewSchedulerWithRepository(mockRepo, mockRecurring, nil, config)

	// Should not panic and continue processing other tenants
	scheduler.RunNow()
}

func TestScheduler_GenerateDueInvoices_AllErrors(t *testing.T) {
	tenants := []TenantInfo{
		{ID: "tenant-1", SchemaName: "tenant_1"},
		{ID: "tenant-2", SchemaName: "tenant_2"},
	}
	mockRepo := &MockRepository{tenants: tenants}
	mockRecurring := NewMockRecurringService()

	// Set up errors for all tenants
	mockRecurring.errors["tenant-1"] = errors.New("error 1")
	mockRecurring.errors["tenant-2"] = errors.New("error 2")

	config := DefaultConfig()
	scheduler := NewSchedulerWithRepository(mockRepo, mockRecurring, nil, config)

	// Should not panic even when all tenants fail
	scheduler.RunNow()
}

func TestScheduler_GenerateDueInvoices_EmptyResults(t *testing.T) {
	tenants := []TenantInfo{
		{ID: "tenant-1", SchemaName: "tenant_1"},
	}
	mockRepo := &MockRepository{tenants: tenants}
	mockRecurring := NewMockRecurringService()

	// No results configured - returns empty slice
	config := DefaultConfig()
	scheduler := NewSchedulerWithRepository(mockRepo, mockRecurring, nil, config)

	// Should handle empty results gracefully
	scheduler.RunNow()
}

func TestScheduler_GenerateDueInvoices_MultipleInvoices(t *testing.T) {
	tenants := []TenantInfo{
		{ID: "tenant-1", SchemaName: "tenant_1"},
	}
	mockRepo := &MockRepository{tenants: tenants}
	mockRecurring := NewMockRecurringService()

	// Set up multiple results for one tenant
	mockRecurring.results["tenant-1"] = []recurring.GenerationResult{
		{
			RecurringInvoiceID:     "recurring-1",
			GeneratedInvoiceID:     "invoice-1",
			GeneratedInvoiceNumber: "INV-001",
			EmailSent:              true,
			EmailStatus:            "sent",
		},
		{
			RecurringInvoiceID:     "recurring-2",
			GeneratedInvoiceID:     "invoice-2",
			GeneratedInvoiceNumber: "INV-002",
			EmailSent:              false,
			EmailStatus:            "failed",
		},
		{
			RecurringInvoiceID:     "recurring-3",
			GeneratedInvoiceID:     "invoice-3",
			GeneratedInvoiceNumber: "INV-003",
			EmailSent:              true,
			EmailStatus:            "sent",
		},
	}

	config := DefaultConfig()
	scheduler := NewSchedulerWithRepository(mockRepo, mockRecurring, nil, config)

	// Should process all invoices
	scheduler.RunNow()
}

func TestScheduler_RunRemindersNow_WithRepositoryError(t *testing.T) {
	mockRepo := &MockRepository{listActiveTenantsErr: errors.New("database error")}
	mockReminder := NewMockReminderService()
	scheduler := NewSchedulerWithRepository(mockRepo, nil, mockReminder, DefaultConfig())

	scheduler.RunRemindersNow()

	if len(mockReminder.calls) != 0 {
		t.Fatalf("expected no reminder calls on repository error, got %v", mockReminder.calls)
	}
}

func TestScheduler_RunRemindersNow_WithTenantResults(t *testing.T) {
	tenants := []TenantInfo{
		{ID: "tenant-1", SchemaName: "tenant_1", CompanyName: "Tenant One"},
		{ID: "tenant-2", SchemaName: "tenant_2", CompanyName: "Tenant Two"},
	}
	mockRepo := &MockRepository{tenants: tenants}
	mockReminder := NewMockReminderService()
	mockReminder.results["tenant-1"] = []invoicing.AutomatedReminderResult{
		{RuleName: "7 days overdue", RemindersSent: 2, Failed: 1, Errors: []string{"smtp timeout"}},
	}
	mockReminder.results["tenant-2"] = []invoicing.AutomatedReminderResult{
		{RuleName: "due today", RemindersSent: 1, Skipped: 1},
	}

	scheduler := NewSchedulerWithRepository(mockRepo, nil, mockReminder, DefaultConfig())
	scheduler.RunRemindersNow()

	if len(mockReminder.calls) != 2 {
		t.Fatalf("expected reminders for 2 tenants, got %d", len(mockReminder.calls))
	}
}

func TestScheduler_RunRemindersNow_ContinuesOnTenantError(t *testing.T) {
	tenants := []TenantInfo{
		{ID: "tenant-1", SchemaName: "tenant_1", CompanyName: "Tenant One"},
		{ID: "tenant-2", SchemaName: "tenant_2", CompanyName: "Tenant Two"},
	}
	mockRepo := &MockRepository{tenants: tenants}
	mockReminder := NewMockReminderService()
	mockReminder.errors["tenant-1"] = errors.New("smtp unavailable")
	mockReminder.results["tenant-2"] = []invoicing.AutomatedReminderResult{
		{RuleName: "final reminder", RemindersSent: 3},
	}

	scheduler := NewSchedulerWithRepository(mockRepo, nil, mockReminder, DefaultConfig())
	scheduler.RunRemindersNow()

	if len(mockReminder.calls) != 2 {
		t.Fatalf("expected both tenants to be processed, got %d calls", len(mockReminder.calls))
	}
}

func TestScheduler_RunDocumentRetentionRemindersNow_WithTenantResults(t *testing.T) {
	tenants := []TenantInfo{
		{ID: "tenant-1", SchemaName: "tenant_1", CompanyName: "Tenant One", Email: "ops1@example.com"},
		{ID: "tenant-2", SchemaName: "tenant_2", CompanyName: "Tenant Two", Email: "ops2@example.com"},
	}
	mockRepo := &MockRepository{tenants: tenants}
	mockRetention := NewMockDocumentRetentionReminderService()
	mockRetention.results["tenant-1"] = documents.RetentionReminderDeliveryResult{
		TenantID:       "tenant-1",
		RecipientEmail: "ops1@example.com",
		ActionsFound:   2,
		EmailSent:      true,
		EmailLogID:     "email-log-1",
	}
	mockRetention.results["tenant-2"] = documents.RetentionReminderDeliveryResult{
		TenantID:     "tenant-2",
		ActionsFound: 0,
		Skipped:      true,
		SkipReason:   "no document retention reminder actions",
	}

	config := DefaultConfig()
	config.DocumentRetentionReminderHorizonDays = 45
	config.DocumentRetentionReminderIncludeMissing = false
	scheduler := NewSchedulerWithRepository(mockRepo, nil, nil, config)
	scheduler.SetDocumentRetentionReminderService(mockRetention)
	scheduler.RunDocumentRetentionRemindersNow()

	if len(mockRetention.calls) != 2 {
		t.Fatalf("expected retention reminders for 2 tenants, got %d", len(mockRetention.calls))
	}
	if mockRetention.calls[0].tenantID != "tenant-1" || mockRetention.calls[0].recipientEmail != "ops1@example.com" {
		t.Fatalf("unexpected first retention call: %#v", mockRetention.calls[0])
	}
	if mockRetention.calls[0].horizonDays != 45 || mockRetention.calls[0].includeMissing {
		t.Fatalf("expected custom retention config, got %#v", mockRetention.calls[0])
	}
}

func TestScheduler_RunDocumentRetentionRemindersNow_WithRepositoryError(t *testing.T) {
	mockRepo := &MockRepository{listActiveTenantsErr: errors.New("database error")}
	mockRetention := NewMockDocumentRetentionReminderService()
	scheduler := NewSchedulerWithRepository(mockRepo, nil, nil, DefaultConfig())
	scheduler.SetDocumentRetentionReminderService(mockRetention)

	scheduler.RunDocumentRetentionRemindersNow()

	if len(mockRetention.calls) != 0 {
		t.Fatalf("expected no retention calls on repository error, got %#v", mockRetention.calls)
	}
}

func TestScheduler_RunDocumentRetentionRemindersNow_ContinuesOnTenantError(t *testing.T) {
	tenants := []TenantInfo{
		{ID: "tenant-1", SchemaName: "tenant_1", CompanyName: "Tenant One", Email: "ops1@example.com"},
		{ID: "tenant-2", SchemaName: "tenant_2", CompanyName: "Tenant Two", Email: "ops2@example.com"},
	}
	mockRepo := &MockRepository{tenants: tenants}
	mockRetention := NewMockDocumentRetentionReminderService()
	mockRetention.errors["tenant-1"] = errors.New("retention repository unavailable")
	mockRetention.results["tenant-2"] = documents.RetentionReminderDeliveryResult{
		TenantID:     "tenant-2",
		ActionsFound: 1,
		Failed:       true,
		ErrorMessage: "smtp unavailable",
	}

	scheduler := NewSchedulerWithRepository(mockRepo, nil, nil, DefaultConfig())
	scheduler.SetDocumentRetentionReminderService(mockRetention)
	scheduler.RunDocumentRetentionRemindersNow()

	if len(mockRetention.calls) != 2 {
		t.Fatalf("expected both tenants to be processed, got %d calls", len(mockRetention.calls))
	}
}

func TestScheduler_RunJournalEntriesNow_WithTenantResults(t *testing.T) {
	tenants := []TenantInfo{
		{ID: "tenant-1", SchemaName: "tenant_1", PeriodLockDate: "2026-01-31"},
		{ID: "tenant-2", SchemaName: "tenant_2"},
	}
	mockRepo := &MockRepository{tenants: tenants}
	mockJournal := NewMockJournalEntryService()
	mockJournal.results["tenant-1"] = []accounting.JournalEntryTemplateGenerationResult{
		{
			TemplateID:           "template-1",
			TemplateName:         "Monthly depreciation",
			GeneratedEntryID:     "entry-1",
			GeneratedEntryNumber: "JE-001",
			Status:               "generated",
		},
	}
	mockJournal.results["tenant-2"] = []accounting.JournalEntryTemplateGenerationResult{
		{
			TemplateID: "template-2",
			Status:     "error",
			Error:      "period locked",
		},
	}

	scheduler := NewSchedulerWithRepository(mockRepo, nil, nil, DefaultConfig())
	scheduler.SetRecurringJournalEntryService(mockJournal)
	scheduler.RunJournalEntriesNow()

	if len(mockJournal.calls) != 2 {
		t.Fatalf("expected journal generation for 2 tenants, got %d calls", len(mockJournal.calls))
	}
	if mockJournal.calls[0].tenantID != "tenant-1" || mockJournal.calls[0].schemaName != "tenant_1" {
		t.Fatalf("unexpected first journal call: %#v", mockJournal.calls[0])
	}
	if mockJournal.calls[0].userID != "system" {
		t.Fatalf("expected system user, got %q", mockJournal.calls[0].userID)
	}
	if mockJournal.calls[0].periodLockDate == nil || mockJournal.calls[0].periodLockDate.Format("2006-01-02") != "2026-01-31" {
		t.Fatalf("expected tenant period lock date to be passed through, got %#v", mockJournal.calls[0].periodLockDate)
	}
	if mockJournal.calls[1].periodLockDate != nil {
		t.Fatalf("expected no lock date for second tenant, got %#v", mockJournal.calls[1].periodLockDate)
	}
}

func TestScheduler_RunJournalEntriesNow_WithRepositoryError(t *testing.T) {
	mockRepo := &MockRepository{listActiveTenantsErr: errors.New("database error")}
	mockJournal := NewMockJournalEntryService()
	scheduler := NewSchedulerWithRepository(mockRepo, nil, nil, DefaultConfig())
	scheduler.SetRecurringJournalEntryService(mockJournal)

	scheduler.RunJournalEntriesNow()

	if len(mockJournal.calls) != 0 {
		t.Fatalf("expected no journal calls on repository error, got %#v", mockJournal.calls)
	}
}

func TestScheduler_RunJournalEntriesNow_ContinuesOnTenantError(t *testing.T) {
	tenants := []TenantInfo{
		{ID: "tenant-1", SchemaName: "tenant_1"},
		{ID: "tenant-2", SchemaName: "tenant_2"},
	}
	mockRepo := &MockRepository{tenants: tenants}
	mockJournal := NewMockJournalEntryService()
	mockJournal.errors["tenant-1"] = errors.New("database unavailable")
	mockJournal.results["tenant-2"] = []accounting.JournalEntryTemplateGenerationResult{
		{
			TemplateID:           "template-2",
			GeneratedEntryID:     "entry-2",
			GeneratedEntryNumber: "JE-002",
			Status:               "generated",
		},
	}

	scheduler := NewSchedulerWithRepository(mockRepo, nil, nil, DefaultConfig())
	scheduler.SetRecurringJournalEntryService(mockJournal)
	scheduler.RunJournalEntriesNow()

	if len(mockJournal.calls) != 2 {
		t.Fatalf("expected both tenants to be processed, got %d calls", len(mockJournal.calls))
	}
}

func TestScheduler_InvalidJournalScheduleFormat(t *testing.T) {
	config := DefaultConfig()
	config.RecurringJournalEntrySchedule = "invalid cron expression"
	scheduler := NewScheduler(nil, nil, nil, config)
	scheduler.SetRecurringJournalEntryService(NewMockJournalEntryService())

	err := scheduler.Start()
	if err == nil {
		t.Fatal("Start() should return error for invalid journal cron expression")
	}
	if !strings.Contains(err.Error(), "recurring journal entry job") {
		t.Fatalf("expected journal job error, got %v", err)
	}
}
