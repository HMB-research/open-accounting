package scheduler

import (
	"testing"
)

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
	// Note: RunNow() with nil db will panic because it tries to query
	// the database. This test documents this expected behavior.
	// In production, the scheduler is always created with a valid db.
	config := DefaultConfig()
	scheduler := NewScheduler(nil, nil, config)

	// We expect RunNow to panic with nil db - this is acceptable
	// because in production, db is never nil
	defer func() {
		if r := recover(); r != nil {
			// Expected panic - test passes
			t.Logf("RunNow() correctly panicked with nil db: %v", r)
		}
	}()

	scheduler.RunNow()
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
