package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"

	"github.com/HMB-research/open-accounting/internal/recurring"
)

// Config holds scheduler configuration
type Config struct {
	// Schedule in cron format (e.g., "0 6 * * *" for 6:00 AM daily)
	RecurringInvoiceSchedule string
	// Whether the scheduler is enabled
	Enabled bool
}

// DefaultConfig returns default scheduler configuration
func DefaultConfig() Config {
	return Config{
		RecurringInvoiceSchedule: "0 6 * * *", // 6:00 AM daily
		Enabled:                  true,
	}
}

// Scheduler manages background jobs
type Scheduler struct {
	cron      *cron.Cron
	db        *pgxpool.Pool
	recurring *recurring.Service
	config    Config
	running   bool
	mu        sync.Mutex
}

// NewScheduler creates a new scheduler instance
func NewScheduler(db *pgxpool.Pool, recurringService *recurring.Service, config Config) *Scheduler {
	return &Scheduler{
		cron:      cron.New(cron.WithSeconds()),
		db:        db,
		recurring: recurringService,
		config:    config,
	}
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

	s.cron.Start()
	s.running = true

	log.Info().
		Str("schedule", s.config.RecurringInvoiceSchedule).
		Msg("Scheduler started - recurring invoice generation scheduled")

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

	// Get all active tenants
	rows, err := s.db.Query(ctx, `
		SELECT id, schema_name FROM tenants WHERE is_active = true
	`)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get tenants for scheduled invoice generation")
		return
	}
	defer rows.Close()

	type tenantInfo struct {
		ID         string
		SchemaName string
	}

	var tenants []tenantInfo
	for rows.Next() {
		var t tenantInfo
		if err := rows.Scan(&t.ID, &t.SchemaName); err != nil {
			log.Error().Err(err).Msg("Failed to scan tenant")
			continue
		}
		tenants = append(tenants, t)
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

// RunNow manually triggers the recurring invoice generation
func (s *Scheduler) RunNow() {
	s.generateDueInvoices()
}

// IsRunning returns whether the scheduler is currently running
func (s *Scheduler) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}
