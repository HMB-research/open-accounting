package scheduler

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/recurring"
)

// RecurringService defines the interface for recurring invoice generation
type RecurringService interface {
	GenerateDueInvoices(ctx context.Context, tenantID, schemaName, userID string) ([]recurring.GenerationResult, error)
}

// AutomatedReminderService defines the interface for automated reminders
type AutomatedReminderService interface {
	ProcessRemindersForTenant(ctx context.Context, tenantID, schemaName, companyName string) ([]invoicing.AutomatedReminderResult, error)
}

// DocumentRetentionReminderService defines scheduled document retention reminder delivery.
type DocumentRetentionReminderService interface {
	ProcessRetentionRemindersForTenant(ctx context.Context, tenantID, schemaName, companyName, recipientEmail string, asOf time.Time, horizonDays int, includeMissing bool) (documents.RetentionReminderDeliveryResult, error)
}

// RecurringJournalEntryService defines the interface for recurring journal entry generation.
type RecurringJournalEntryService interface {
	GenerateDueJournalEntryTemplates(ctx context.Context, schemaName, tenantID string, req *accounting.GenerateDueJournalEntryTemplatesRequest) ([]accounting.JournalEntryTemplateGenerationResult, error)
}

// Config holds scheduler configuration
type Config struct {
	// Schedule in cron format (e.g., "0 6 * * *" for 6:00 AM daily)
	RecurringInvoiceSchedule string
	// Schedule for recurring journal entry generation
	RecurringJournalEntrySchedule string
	// Schedule for payment reminders
	ReminderSchedule string
	// Schedule for document retention reminder delivery
	DocumentRetentionReminderSchedule string
	// Horizon in days for document retention reminders
	DocumentRetentionReminderHorizonDays int
	// Whether to include missing retention metadata in scheduled reminders
	DocumentRetentionReminderIncludeMissing bool
	// Whether the scheduler is enabled
	Enabled bool
}

// DefaultConfig returns default scheduler configuration
func DefaultConfig() Config {
	return Config{
		RecurringInvoiceSchedule:                "0 6 * * *",  // 6:00 AM daily
		RecurringJournalEntrySchedule:           "15 6 * * *", // 6:15 AM daily
		ReminderSchedule:                        "0 9 * * *",  // 9:00 AM daily
		DocumentRetentionReminderSchedule:       "30 9 * * *", // 9:30 AM daily
		DocumentRetentionReminderHorizonDays:    documents.DefaultRetentionReminderHorizonDays,
		DocumentRetentionReminderIncludeMissing: true,
		Enabled:                                 true,
	}
}

// Scheduler manages background jobs
type Scheduler struct {
	cron              *cron.Cron
	repo              Repository
	recurring         RecurringService
	journalEntries    RecurringJournalEntryService
	reminder          AutomatedReminderService
	documentRetention DocumentRetentionReminderService
	config            Config
	running           bool
	mu                sync.Mutex
}

var newGormDBFromPool = database.NewGormDBFromPool

// NewScheduler creates a new scheduler instance
func NewScheduler(db *pgxpool.Pool, recurringService *recurring.Service, reminderService *invoicing.AutomatedReminderService, config Config) *Scheduler {
	if db == nil {
		return &Scheduler{
			cron:      cron.New(cron.WithSeconds()),
			recurring: recurringService,
			reminder:  reminderService,
			config:    config,
		}
	}
	gormDB, err := newGormDBFromPool(context.Background(), db)
	if err != nil {
		panic(fmt.Errorf("create scheduler GORM repository: %w", err))
	}
	return &Scheduler{
		cron:      cron.New(cron.WithSeconds()),
		repo:      NewGORMRepository(gormDB),
		recurring: recurringService,
		reminder:  reminderService,
		config:    config,
	}
}

// NewSchedulerWithRepository creates a scheduler with custom repositories (for testing)
func NewSchedulerWithRepository(repo Repository, recurringService RecurringService, reminderService AutomatedReminderService, config Config) *Scheduler {
	return &Scheduler{
		cron:      cron.New(cron.WithSeconds()),
		repo:      repo,
		recurring: recurringService,
		reminder:  reminderService,
		config:    config,
	}
}

// SetRecurringJournalEntryService enables scheduled recurring journal entry generation.
func (s *Scheduler) SetRecurringJournalEntryService(service RecurringJournalEntryService) {
	s.journalEntries = service
}

// SetDocumentRetentionReminderService enables scheduled document retention reminder delivery.
func (s *Scheduler) SetDocumentRetentionReminderService(service DocumentRetentionReminderService) {
	s.documentRetention = service
}

// Start starts the scheduler
func (s *Scheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("scheduler is already running")
	}

	if !s.config.Enabled {
		log.Info().Msg("Scheduler is disabled")
		return nil
	}

	// Add recurring invoice generation job
	// Convert standard cron (5 fields) to 6-field format by prepending "0" for seconds
	schedule := "0 " + s.config.RecurringInvoiceSchedule
	_, err := s.cron.AddFunc(schedule, s.generateDueInvoices)
	if err != nil {
		return fmt.Errorf("failed to add recurring invoice job: %w", err)
	}

	// Add recurring journal entry generation job
	if s.journalEntries != nil && s.config.RecurringJournalEntrySchedule != "" {
		journalSchedule := "0 " + s.config.RecurringJournalEntrySchedule
		_, err := s.cron.AddFunc(journalSchedule, s.generateDueJournalEntries)
		if err != nil {
			return fmt.Errorf("failed to add recurring journal entry job: %w", err)
		}
		log.Info().Str("schedule", s.config.RecurringJournalEntrySchedule).Msg("Recurring journal entry job scheduled")
	}

	// Add payment reminder job
	if s.reminder != nil && s.config.ReminderSchedule != "" {
		reminderSchedule := "0 " + s.config.ReminderSchedule
		_, err := s.cron.AddFunc(reminderSchedule, s.processPaymentReminders)
		if err != nil {
			return fmt.Errorf("failed to add reminder job: %w", err)
		}
		log.Info().Str("schedule", s.config.ReminderSchedule).Msg("Payment reminder job scheduled")
	}

	if s.documentRetention != nil && s.config.DocumentRetentionReminderSchedule != "" {
		retentionSchedule := "0 " + s.config.DocumentRetentionReminderSchedule
		_, err := s.cron.AddFunc(retentionSchedule, s.processDocumentRetentionReminders)
		if err != nil {
			return fmt.Errorf("failed to add document retention reminder job: %w", err)
		}
		log.Info().
			Str("schedule", s.config.DocumentRetentionReminderSchedule).
			Int("horizon_days", s.config.DocumentRetentionReminderHorizonDays).
			Bool("include_missing", s.config.DocumentRetentionReminderIncludeMissing).
			Msg("Document retention reminder job scheduled")
	}

	s.cron.Start()
	s.running = true

	log.Info().
		Str("recurring_schedule", s.config.RecurringInvoiceSchedule).
		Str("recurring_journal_schedule", s.config.RecurringJournalEntrySchedule).
		Str("reminder_schedule", s.config.ReminderSchedule).
		Str("document_retention_reminder_schedule", s.config.DocumentRetentionReminderSchedule).
		Msg("Scheduler started")

	return nil
}

// Stop stops the scheduler gracefully
func (s *Scheduler) Stop() context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx
	}

	ctx := s.cron.Stop()
	s.running = false
	log.Info().Msg("Scheduler stopped")
	return ctx
}

// generateDueInvoices generates all due recurring invoices for all tenants
func (s *Scheduler) generateDueInvoices() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	log.Info().Msg("Starting scheduled recurring invoice generation")

	// Get all active tenants via repository
	tenants, err := s.repo.ListActiveTenants(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get tenants for scheduled invoice generation")
		return
	}

	totalGenerated := 0
	totalEmails := 0
	totalErrors := 0

	for _, t := range tenants {
		results, err := s.recurring.GenerateDueInvoices(ctx, t.ID, t.SchemaName, "system")
		if err != nil {
			log.Error().
				Err(err).
				Str("tenant_id", t.ID).
				Msg("Failed to generate due invoices for tenant")
			totalErrors++
			continue
		}

		for _, result := range results {
			totalGenerated++
			if result.EmailSent {
				totalEmails++
			}
			log.Info().
				Str("tenant_id", t.ID).
				Str("recurring_id", result.RecurringInvoiceID).
				Str("invoice_id", result.GeneratedInvoiceID).
				Str("invoice_number", result.GeneratedInvoiceNumber).
				Bool("email_sent", result.EmailSent).
				Str("email_status", result.EmailStatus).
				Msg("Generated invoice from recurring template")
		}
	}

	log.Info().
		Int("invoices_generated", totalGenerated).
		Int("emails_sent", totalEmails).
		Int("tenant_errors", totalErrors).
		Msg("Completed scheduled recurring invoice generation")
}

// generateDueJournalEntries generates all due recurring journal entries for all tenants.
func (s *Scheduler) generateDueJournalEntries() {
	if s.journalEntries == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	log.Info().Msg("Starting scheduled recurring journal entry generation")

	tenants, err := s.repo.ListActiveTenants(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get tenants for scheduled journal entry generation")
		return
	}

	totalGenerated := 0
	totalErrors := 0

	for _, t := range tenants {
		lockDate := parseTenantPeriodLockDate(t)
		results, err := s.journalEntries.GenerateDueJournalEntryTemplates(ctx, t.SchemaName, t.ID, &accounting.GenerateDueJournalEntryTemplatesRequest{
			UserID:         "system",
			PeriodLockDate: lockDate,
		})
		if err != nil {
			log.Error().
				Err(err).
				Str("tenant_id", t.ID).
				Msg("Failed to generate due journal entries for tenant")
			totalErrors++
			continue
		}

		for _, result := range results {
			if result.Status == "error" {
				totalErrors++
				log.Warn().
					Str("tenant_id", t.ID).
					Str("template_id", result.TemplateID).
					Str("error", result.Error).
					Msg("Recurring journal template generation failed")
				continue
			}
			totalGenerated++
			log.Info().
				Str("tenant_id", t.ID).
				Str("template_id", result.TemplateID).
				Str("template_name", result.TemplateName).
				Str("entry_id", result.GeneratedEntryID).
				Str("entry_number", result.GeneratedEntryNumber).
				Msg("Generated journal entry from recurring template")
		}
	}

	log.Info().
		Int("journal_entries_generated", totalGenerated).
		Int("tenant_or_template_errors", totalErrors).
		Msg("Completed scheduled recurring journal entry generation")
}

func parseTenantPeriodLockDate(t TenantInfo) *time.Time {
	raw := strings.TrimSpace(t.PeriodLockDate)
	if raw == "" {
		return nil
	}
	lockDate, err := time.Parse("2006-01-02", raw)
	if err != nil {
		log.Warn().
			Err(err).
			Str("tenant_id", t.ID).
			Str("period_lock_date", raw).
			Msg("Ignoring invalid tenant period lock date for scheduled journal generation")
		return nil
	}
	return &lockDate
}

// processPaymentReminders sends automated payment reminders for all tenants
func (s *Scheduler) processPaymentReminders() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	log.Info().Msg("Starting scheduled payment reminder processing")

	tenants, err := s.repo.ListActiveTenants(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get tenants for reminder processing")
		return
	}

	totalReminders := 0
	totalErrors := 0

	for _, t := range tenants {
		results, err := s.reminder.ProcessRemindersForTenant(ctx, t.ID, t.SchemaName, t.CompanyName)
		if err != nil {
			log.Error().Err(err).Str("tenant_id", t.ID).Msg("Failed to process reminders for tenant")
			totalErrors++
			continue
		}

		for _, result := range results {
			totalReminders += result.RemindersSent
			if len(result.Errors) > 0 {
				log.Warn().
					Str("tenant_id", t.ID).
					Str("rule", result.RuleName).
					Int("sent", result.RemindersSent).
					Int("failed", result.Failed).
					Strs("errors", result.Errors).
					Msg("Reminder rule completed with errors")
			} else if result.RemindersSent > 0 {
				log.Info().
					Str("tenant_id", t.ID).
					Str("rule", result.RuleName).
					Int("sent", result.RemindersSent).
					Int("skipped", result.Skipped).
					Msg("Reminder rule completed")
			}
		}
	}

	log.Info().
		Int("reminders_sent", totalReminders).
		Int("tenant_errors", totalErrors).
		Msg("Completed scheduled payment reminder processing")
}

// processDocumentRetentionReminders sends document retention reminder digests for all tenants.
func (s *Scheduler) processDocumentRetentionReminders() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	log.Info().Msg("Starting scheduled document retention reminder processing")

	tenants, err := s.repo.ListActiveTenants(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get tenants for document retention reminder processing")
		return
	}

	asOf := time.Now()
	totalActions := 0
	totalSent := 0
	totalSkipped := 0
	totalFailed := 0
	totalEscalated := 0
	totalErrors := 0

	for _, t := range tenants {
		result, err := s.documentRetention.ProcessRetentionRemindersForTenant(
			ctx,
			t.ID,
			t.SchemaName,
			t.CompanyName,
			t.Email,
			asOf,
			s.config.DocumentRetentionReminderHorizonDays,
			s.config.DocumentRetentionReminderIncludeMissing,
		)
		if err != nil {
			log.Error().Err(err).Str("tenant_id", t.ID).Msg("Failed to process document retention reminders for tenant")
			totalErrors++
			continue
		}

		totalActions += result.ActionsFound
		switch {
		case result.EmailSent:
			totalSent++
			log.Info().
				Str("tenant_id", t.ID).
				Str("recipient_email", result.RecipientEmail).
				Int("actions", result.ActionsFound).
				Int("attempts", result.DeliveryAttempts).
				Str("email_log_id", result.EmailLogID).
				Msg("Document retention reminder sent")
		case result.Failed:
			totalFailed++
			if result.Escalated {
				totalEscalated++
			}
			log.Warn().
				Str("tenant_id", t.ID).
				Str("recipient_email", result.RecipientEmail).
				Int("actions", result.ActionsFound).
				Int("attempts", result.DeliveryAttempts).
				Bool("escalated", result.Escalated).
				Str("escalation_reason", result.EscalationReason).
				Str("error", result.ErrorMessage).
				Msg("Document retention reminder failed")
		case result.Skipped:
			totalSkipped++
			log.Debug().
				Str("tenant_id", t.ID).
				Str("reason", result.SkipReason).
				Int("actions", result.ActionsFound).
				Msg("Document retention reminder skipped")
		}
	}

	log.Info().
		Int("actions_found", totalActions).
		Int("emails_sent", totalSent).
		Int("skipped", totalSkipped).
		Int("failed", totalFailed).
		Int("escalated", totalEscalated).
		Int("tenant_errors", totalErrors).
		Msg("Completed scheduled document retention reminder processing")
}

// RunNow manually triggers the recurring invoice generation
func (s *Scheduler) RunNow() {
	s.generateDueInvoices()
}

// RunRemindersNow manually triggers the payment reminder processing
func (s *Scheduler) RunRemindersNow() {
	s.processPaymentReminders()
}

// RunJournalEntriesNow manually triggers recurring journal entry generation.
func (s *Scheduler) RunJournalEntriesNow() {
	s.generateDueJournalEntries()
}

// RunDocumentRetentionRemindersNow manually triggers document retention reminder delivery.
func (s *Scheduler) RunDocumentRetentionRemindersNow() {
	s.processDocumentRetentionReminders()
}

// IsRunning returns whether the scheduler is currently running
func (s *Scheduler) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}
