package scheduler

import (
	"context"
	"errors"
	"testing"

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

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.RecurringInvoiceSchedule != "0 6 * * *" {
		t.Errorf("RecurringInvoiceSchedule = %q, want %q", config.RecurringInvoiceSchedule, "0 6 * * *")
	}
	if !config.Enabled {
		t.Error("Enabled should be true by default")
	}
}

func TestNewScheduler(t *testing.T) {
	config := DefaultConfig()
	scheduler := NewScheduler(nil, nil, config)

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
	scheduler := NewScheduler(nil, nil, config)

	if scheduler.IsRunning() {
		t.Error("scheduler should not be running initially")
	}
}

func TestScheduler_StartDisabled(t *testing.T) {
	config := Config{
		RecurringInvoiceSchedule: "0 6 * * *",
		Enabled:                  false,
	}
	scheduler := NewScheduler(nil, nil, config)

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
	scheduler := NewScheduler(nil, nil, config)

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
	scheduler := NewScheduler(nil, nil, config)

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
	scheduler := NewScheduler(nil, nil, config)

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
	scheduler := NewScheduler(nil, nil, config)

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
	scheduler := NewScheduler(nil, nil, config)

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
	scheduler := NewSchedulerWithRepository(mockRepo, nil, config)

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
			scheduler := NewSchedulerWithRepository(mockRepo, nil, config)

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
	scheduler := NewScheduler(nil, nil, config)

	err := scheduler.Start()
	if err == nil {
		t.Error("Start() should return error for invalid cron expression")
		scheduler.Stop()
	}
}

func TestScheduler_ConcurrentAccess(t *testing.T) {
	config := DefaultConfig()
	scheduler := NewScheduler(nil, nil, config)

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
	scheduler := NewScheduler(nil, nil, config)

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
			scheduler := NewScheduler(nil, nil, config)

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
	scheduler := NewSchedulerWithRepository(mockRepo, mockRecurring, config)

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
	scheduler := NewSchedulerWithRepository(mockRepo, mockRecurring, config)

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
	scheduler := NewSchedulerWithRepository(mockRepo, mockRecurring, config)

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
	scheduler := NewSchedulerWithRepository(mockRepo, mockRecurring, config)

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
	scheduler := NewSchedulerWithRepository(mockRepo, mockRecurring, config)

	// Should process all invoices
	scheduler.RunNow()
}
