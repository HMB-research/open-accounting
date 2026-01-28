# Payment Tracking and Automated Reminders Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Extend existing payment tracking with automated scheduled reminders that run on configurable schedules (before due, on due date, overdue intervals).

**Architecture:** The codebase already has payment tracking (payments package), manual reminders (invoicing/reminder_service.go), and a scheduler (scheduler package). We'll add: (1) reminder rules configuration per tenant, (2) scheduled reminder job in the scheduler, (3) pre-due invoice tracking, (4) new email templates for different reminder stages.

**Tech Stack:** Go, PostgreSQL, robfig/cron, SvelteKit 5, existing email service

---

## Current State Analysis

**Already Implemented:**
- Payment tracking with status (DRAFT, SENT, PARTIALLY_PAID, PAID, OVERDUE, VOIDED)
- Partial payment allocations with remaining balance tracking
- Payment history per invoice via payment_allocations table
- Manual overdue reminder sending (GetOverdueInvoices, SendReminder, SendBulkReminders)
- Email templates including OVERDUE_REMINDER
- Scheduler running daily for recurring invoices
- Frontend page for manual reminder sending

**Gaps to Fill:**
1. **Reminder Rules** - No configurable schedule (e.g., "7 days before due, on due date, 7/14/30 days overdue")
2. **Automated Sending** - Reminders are manual only; scheduler doesn't run them
3. **Pre-Due Reminders** - Current system only tracks overdue invoices, not upcoming ones
4. **Multiple Email Templates** - Only one OVERDUE_REMINDER template; need different templates for different stages

---

## Task 1: Add Reminder Rules Database Schema

**Files:**
- Create: `migrations/021_reminder_rules.up.sql`
- Create: `migrations/021_reminder_rules.down.sql`

**Step 1: Write the up migration**

```sql
-- Migration: Add reminder rules and extend reminder tracking
-- This migration adds configurable reminder schedules per tenant

-- Create function to add reminder rules tables to a tenant schema
CREATE OR REPLACE FUNCTION add_reminder_rules_to_schema(schema_name TEXT)
RETURNS void AS $$
BEGIN
    -- Reminder rules table - defines when to send reminders
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.reminder_rules (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            name VARCHAR(100) NOT NULL,
            trigger_type VARCHAR(20) NOT NULL CHECK (trigger_type IN (
                ''BEFORE_DUE'',
                ''ON_DUE'',
                ''AFTER_DUE''
            )),
            days_offset INTEGER NOT NULL DEFAULT 0,
            email_template_type VARCHAR(50) NOT NULL DEFAULT ''OVERDUE_REMINDER'',
            is_active BOOLEAN DEFAULT true,
            created_at TIMESTAMPTZ DEFAULT NOW(),
            updated_at TIMESTAMPTZ DEFAULT NOW(),
            UNIQUE (tenant_id, trigger_type, days_offset)
        )', schema_name);

    -- Payment reminders table - tracks sent reminders
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.payment_reminders (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            invoice_id UUID NOT NULL,
            invoice_number VARCHAR(50) NOT NULL,
            contact_id UUID NOT NULL,
            contact_name VARCHAR(255) NOT NULL,
            contact_email VARCHAR(255),
            rule_id UUID REFERENCES %I.reminder_rules(id),
            trigger_type VARCHAR(20) NOT NULL,
            days_offset INTEGER NOT NULL DEFAULT 0,
            reminder_number INTEGER NOT NULL DEFAULT 1,
            status VARCHAR(20) DEFAULT ''PENDING'' CHECK (status IN (
                ''PENDING'',
                ''SENT'',
                ''FAILED'',
                ''SKIPPED'',
                ''CANCELED''
            )),
            sent_at TIMESTAMPTZ,
            error_message TEXT,
            created_at TIMESTAMPTZ DEFAULT NOW(),
            updated_at TIMESTAMPTZ DEFAULT NOW()
        )', schema_name, schema_name);

    -- Create indexes
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_reminder_rules_tenant ON %I.reminder_rules(tenant_id)',
        replace(schema_name, 'tenant_', ''), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_reminder_rules_active ON %I.reminder_rules(tenant_id, is_active) WHERE is_active = true',
        replace(schema_name, 'tenant_', ''), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_payment_reminders_tenant ON %I.payment_reminders(tenant_id)',
        replace(schema_name, 'tenant_', ''), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_payment_reminders_invoice ON %I.payment_reminders(invoice_id)',
        replace(schema_name, 'tenant_', ''), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_payment_reminders_status ON %I.payment_reminders(status) WHERE status = ''PENDING''',
        replace(schema_name, 'tenant_', ''), schema_name);

    -- Insert default reminder rules
    EXECUTE format('
        INSERT INTO %I.reminder_rules (tenant_id, name, trigger_type, days_offset, email_template_type)
        VALUES
            ((SELECT id FROM tenants WHERE schema_name = %L LIMIT 1),
             ''7 Days Before Due'', ''BEFORE_DUE'', 7, ''PAYMENT_DUE_SOON''),
            ((SELECT id FROM tenants WHERE schema_name = %L LIMIT 1),
             ''On Due Date'', ''ON_DUE'', 0, ''PAYMENT_DUE_TODAY''),
            ((SELECT id FROM tenants WHERE schema_name = %L LIMIT 1),
             ''7 Days Overdue'', ''AFTER_DUE'', 7, ''OVERDUE_REMINDER''),
            ((SELECT id FROM tenants WHERE schema_name = %L LIMIT 1),
             ''14 Days Overdue'', ''AFTER_DUE'', 14, ''OVERDUE_REMINDER''),
            ((SELECT id FROM tenants WHERE schema_name = %L LIMIT 1),
             ''30 Days Overdue'', ''AFTER_DUE'', 30, ''OVERDUE_REMINDER'')
        ON CONFLICT (tenant_id, trigger_type, days_offset) DO NOTHING
    ', schema_name, schema_name, schema_name, schema_name, schema_name, schema_name);

    -- Add new email template types
    EXECUTE format('
        INSERT INTO %I.email_templates (tenant_id, template_type, name, subject, body_html, body_text)
        VALUES
            ((SELECT id FROM tenants WHERE schema_name = %L LIMIT 1),
             ''PAYMENT_DUE_SOON'',
             ''Payment Due Soon'',
             ''Reminder: Invoice {{invoice_number}} due in {{days_until_due}} days'',
             ''<p>Dear {{contact_name}},</p><p>This is a friendly reminder that invoice {{invoice_number}} for {{total_amount}} is due on {{due_date}} ({{days_until_due}} days from now).</p><p>Please arrange payment before the due date to avoid any late fees.</p><p>Best regards,<br>{{company_name}}</p>'',
             ''Dear {{contact_name}},\n\nThis is a friendly reminder that invoice {{invoice_number}} for {{total_amount}} is due on {{due_date}} ({{days_until_due}} days from now).\n\nPlease arrange payment before the due date to avoid any late fees.\n\nBest regards,\n{{company_name}}''),
            ((SELECT id FROM tenants WHERE schema_name = %L LIMIT 1),
             ''PAYMENT_DUE_TODAY'',
             ''Payment Due Today'',
             ''Invoice {{invoice_number}} is due today'',
             ''<p>Dear {{contact_name}},</p><p>This is a reminder that invoice {{invoice_number}} for {{total_amount}} is due today ({{due_date}}).</p><p>Please arrange payment as soon as possible.</p><p>Best regards,<br>{{company_name}}</p>'',
             ''Dear {{contact_name}},\n\nThis is a reminder that invoice {{invoice_number}} for {{total_amount}} is due today ({{due_date}}).\n\nPlease arrange payment as soon as possible.\n\nBest regards,\n{{company_name}}'')
        ON CONFLICT (tenant_id, template_type) DO NOTHING
    ', schema_name, schema_name, schema_name);
END;
$$ LANGUAGE plpgsql;

-- Apply to all existing tenant schemas
DO $$
DECLARE
    tenant_record RECORD;
BEGIN
    FOR tenant_record IN
        SELECT schema_name FROM tenants WHERE schema_name IS NOT NULL
    LOOP
        PERFORM add_reminder_rules_to_schema(tenant_record.schema_name);
    END LOOP;
END;
$$;
```

**Step 2: Write the down migration**

```sql
-- Rollback: Remove reminder rules tables

CREATE OR REPLACE FUNCTION remove_reminder_rules_from_schema(schema_name TEXT)
RETURNS void AS $$
BEGIN
    EXECUTE format('DROP TABLE IF EXISTS %I.payment_reminders CASCADE', schema_name);
    EXECUTE format('DROP TABLE IF EXISTS %I.reminder_rules CASCADE', schema_name);
    EXECUTE format('DELETE FROM %I.email_templates WHERE template_type IN (''PAYMENT_DUE_SOON'', ''PAYMENT_DUE_TODAY'')', schema_name);
END;
$$ LANGUAGE plpgsql;

DO $$
DECLARE
    tenant_record RECORD;
BEGIN
    FOR tenant_record IN
        SELECT schema_name FROM tenants WHERE schema_name IS NOT NULL
    LOOP
        PERFORM remove_reminder_rules_from_schema(tenant_record.schema_name);
    END LOOP;
END;
$$;

DROP FUNCTION IF EXISTS add_reminder_rules_to_schema(TEXT);
DROP FUNCTION IF EXISTS remove_reminder_rules_from_schema(TEXT);
```

**Step 3: Test migration locally**

Run: `go run cmd/migrate/main.go up`
Expected: Migration applies without errors

**Step 4: Commit**

```bash
git add migrations/021_reminder_rules.up.sql migrations/021_reminder_rules.down.sql
git commit -m "feat(db): add reminder rules schema for automated payment reminders"
```

---

## Task 2: Add Reminder Rule Types

**Files:**
- Create: `internal/invoicing/reminder_rule_types.go`

**Step 1: Write the type definitions**

```go
package invoicing

import "time"

// TriggerType represents when a reminder should be triggered
type TriggerType string

const (
    TriggerBeforeDue TriggerType = "BEFORE_DUE"
    TriggerOnDue     TriggerType = "ON_DUE"
    TriggerAfterDue  TriggerType = "AFTER_DUE"
)

// ReminderRule defines when automated reminders should be sent
type ReminderRule struct {
    ID                string      `json:"id"`
    TenantID          string      `json:"tenant_id"`
    Name              string      `json:"name"`
    TriggerType       TriggerType `json:"trigger_type"`
    DaysOffset        int         `json:"days_offset"`
    EmailTemplateType string      `json:"email_template_type"`
    IsActive          bool        `json:"is_active"`
    CreatedAt         time.Time   `json:"created_at"`
    UpdatedAt         time.Time   `json:"updated_at"`
}

// CreateReminderRuleRequest is the request to create a reminder rule
type CreateReminderRuleRequest struct {
    Name              string      `json:"name"`
    TriggerType       TriggerType `json:"trigger_type"`
    DaysOffset        int         `json:"days_offset"`
    EmailTemplateType string      `json:"email_template_type,omitempty"`
    IsActive          bool        `json:"is_active"`
}

// Validate validates the create rule request
func (r *CreateReminderRuleRequest) Validate() error {
    if r.Name == "" {
        return ErrRuleNameRequired
    }
    if r.TriggerType == "" {
        return ErrTriggerTypeRequired
    }
    if r.TriggerType != TriggerBeforeDue && r.TriggerType != TriggerOnDue && r.TriggerType != TriggerAfterDue {
        return ErrInvalidTriggerType
    }
    if r.DaysOffset < 0 {
        return ErrInvalidDaysOffset
    }
    return nil
}

// UpdateReminderRuleRequest is the request to update a reminder rule
type UpdateReminderRuleRequest struct {
    Name              *string `json:"name,omitempty"`
    EmailTemplateType *string `json:"email_template_type,omitempty"`
    IsActive          *bool   `json:"is_active,omitempty"`
}

// InvoiceForReminder represents an invoice that may need a reminder
type InvoiceForReminder struct {
    ID                string  `json:"id"`
    InvoiceNumber     string  `json:"invoice_number"`
    ContactID         string  `json:"contact_id"`
    ContactName       string  `json:"contact_name"`
    ContactEmail      string  `json:"contact_email,omitempty"`
    IssueDate         string  `json:"issue_date"`
    DueDate           string  `json:"due_date"`
    Total             string  `json:"total"`
    AmountPaid        string  `json:"amount_paid"`
    OutstandingAmount string  `json:"outstanding_amount"`
    Currency          string  `json:"currency"`
    DaysUntilDue      int     `json:"days_until_due"`  // Negative if overdue
    DaysOverdue       int     `json:"days_overdue"`    // 0 if not overdue
}

// AutomatedReminderResult represents the result of an automated reminder run
type AutomatedReminderResult struct {
    TenantID      string    `json:"tenant_id"`
    RuleID        string    `json:"rule_id"`
    RuleName      string    `json:"rule_name"`
    InvoicesFound int       `json:"invoices_found"`
    RemindersSent int       `json:"reminders_sent"`
    Skipped       int       `json:"skipped"`
    Failed        int       `json:"failed"`
    Errors        []string  `json:"errors,omitempty"`
    RunAt         time.Time `json:"run_at"`
}

// Errors
var (
    ErrRuleNameRequired    = &ValidationError{Field: "name", Message: "rule name is required"}
    ErrTriggerTypeRequired = &ValidationError{Field: "trigger_type", Message: "trigger type is required"}
    ErrInvalidTriggerType  = &ValidationError{Field: "trigger_type", Message: "invalid trigger type"}
    ErrInvalidDaysOffset   = &ValidationError{Field: "days_offset", Message: "days offset cannot be negative"}
    ErrRuleNotFound        = &NotFoundError{Entity: "reminder rule"}
)

// ValidationError represents a validation error
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return e.Message
}

// NotFoundError represents a not found error
type NotFoundError struct {
    Entity string
}

func (e *NotFoundError) Error() string {
    return e.Entity + " not found"
}
```

**Step 2: Run tests to verify compilation**

Run: `go build ./internal/invoicing/...`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add internal/invoicing/reminder_rule_types.go
git commit -m "feat(invoicing): add reminder rule types for automated reminders"
```

---

## Task 3: Add Reminder Rule Repository

**Files:**
- Create: `internal/invoicing/reminder_rule_repository.go`

**Step 1: Write the repository**

```go
package invoicing

import (
    "context"
    "fmt"
    "time"

    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"
)

// ReminderRuleRepository defines the interface for reminder rule data access
type ReminderRuleRepository interface {
    // ListRules returns all reminder rules for a tenant
    ListRules(ctx context.Context, schemaName, tenantID string) ([]ReminderRule, error)

    // ListActiveRules returns only active rules
    ListActiveRules(ctx context.Context, schemaName, tenantID string) ([]ReminderRule, error)

    // GetRule returns a single rule by ID
    GetRule(ctx context.Context, schemaName, tenantID, ruleID string) (*ReminderRule, error)

    // CreateRule creates a new reminder rule
    CreateRule(ctx context.Context, schemaName string, rule *ReminderRule) error

    // UpdateRule updates an existing rule
    UpdateRule(ctx context.Context, schemaName string, rule *ReminderRule) error

    // DeleteRule deletes a rule
    DeleteRule(ctx context.Context, schemaName, tenantID, ruleID string) error

    // GetInvoicesForRule returns invoices that match a rule's criteria
    GetInvoicesForRule(ctx context.Context, schemaName, tenantID string, rule *ReminderRule, asOfDate time.Time) ([]InvoiceForReminder, error)

    // HasReminderBeenSent checks if a reminder was already sent for this invoice+rule combo
    HasReminderBeenSent(ctx context.Context, schemaName, tenantID, invoiceID, ruleID string) (bool, error)

    // RecordReminderSent records that a reminder was sent
    RecordReminderSent(ctx context.Context, schemaName string, reminder *PaymentReminder) error
}

// ReminderRulePostgresRepository implements ReminderRuleRepository
type ReminderRulePostgresRepository struct {
    db *pgxpool.Pool
}

// NewReminderRulePostgresRepository creates a new repository
func NewReminderRulePostgresRepository(db *pgxpool.Pool) *ReminderRulePostgresRepository {
    return &ReminderRulePostgresRepository{db: db}
}

func (r *ReminderRulePostgresRepository) ListRules(ctx context.Context, schemaName, tenantID string) ([]ReminderRule, error) {
    query := fmt.Sprintf(`
        SELECT id, tenant_id, name, trigger_type, days_offset, email_template_type, is_active, created_at, updated_at
        FROM %s.reminder_rules
        WHERE tenant_id = $1
        ORDER BY trigger_type, days_offset
    `, schemaName)

    rows, err := r.db.Query(ctx, query, tenantID)
    if err != nil {
        return nil, fmt.Errorf("query rules: %w", err)
    }
    defer rows.Close()

    return scanRules(rows)
}

func (r *ReminderRulePostgresRepository) ListActiveRules(ctx context.Context, schemaName, tenantID string) ([]ReminderRule, error) {
    query := fmt.Sprintf(`
        SELECT id, tenant_id, name, trigger_type, days_offset, email_template_type, is_active, created_at, updated_at
        FROM %s.reminder_rules
        WHERE tenant_id = $1 AND is_active = true
        ORDER BY trigger_type, days_offset
    `, schemaName)

    rows, err := r.db.Query(ctx, query, tenantID)
    if err != nil {
        return nil, fmt.Errorf("query active rules: %w", err)
    }
    defer rows.Close()

    return scanRules(rows)
}

func (r *ReminderRulePostgresRepository) GetRule(ctx context.Context, schemaName, tenantID, ruleID string) (*ReminderRule, error) {
    query := fmt.Sprintf(`
        SELECT id, tenant_id, name, trigger_type, days_offset, email_template_type, is_active, created_at, updated_at
        FROM %s.reminder_rules
        WHERE tenant_id = $1 AND id = $2
    `, schemaName)

    var rule ReminderRule
    err := r.db.QueryRow(ctx, query, tenantID, ruleID).Scan(
        &rule.ID, &rule.TenantID, &rule.Name, &rule.TriggerType, &rule.DaysOffset,
        &rule.EmailTemplateType, &rule.IsActive, &rule.CreatedAt, &rule.UpdatedAt,
    )
    if err == pgx.ErrNoRows {
        return nil, ErrRuleNotFound
    }
    if err != nil {
        return nil, fmt.Errorf("query rule: %w", err)
    }

    return &rule, nil
}

func (r *ReminderRulePostgresRepository) CreateRule(ctx context.Context, schemaName string, rule *ReminderRule) error {
    query := fmt.Sprintf(`
        INSERT INTO %s.reminder_rules (id, tenant_id, name, trigger_type, days_offset, email_template_type, is_active, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
    `, schemaName)

    _, err := r.db.Exec(ctx, query,
        rule.ID, rule.TenantID, rule.Name, rule.TriggerType, rule.DaysOffset,
        rule.EmailTemplateType, rule.IsActive, rule.CreatedAt, rule.UpdatedAt,
    )
    if err != nil {
        return fmt.Errorf("insert rule: %w", err)
    }

    return nil
}

func (r *ReminderRulePostgresRepository) UpdateRule(ctx context.Context, schemaName string, rule *ReminderRule) error {
    query := fmt.Sprintf(`
        UPDATE %s.reminder_rules
        SET name = $1, email_template_type = $2, is_active = $3, updated_at = $4
        WHERE id = $5 AND tenant_id = $6
    `, schemaName)

    result, err := r.db.Exec(ctx, query,
        rule.Name, rule.EmailTemplateType, rule.IsActive, time.Now(), rule.ID, rule.TenantID,
    )
    if err != nil {
        return fmt.Errorf("update rule: %w", err)
    }

    if result.RowsAffected() == 0 {
        return ErrRuleNotFound
    }

    return nil
}

func (r *ReminderRulePostgresRepository) DeleteRule(ctx context.Context, schemaName, tenantID, ruleID string) error {
    query := fmt.Sprintf(`DELETE FROM %s.reminder_rules WHERE tenant_id = $1 AND id = $2`, schemaName)

    result, err := r.db.Exec(ctx, query, tenantID, ruleID)
    if err != nil {
        return fmt.Errorf("delete rule: %w", err)
    }

    if result.RowsAffected() == 0 {
        return ErrRuleNotFound
    }

    return nil
}

func (r *ReminderRulePostgresRepository) GetInvoicesForRule(ctx context.Context, schemaName, tenantID string, rule *ReminderRule, asOfDate time.Time) ([]InvoiceForReminder, error) {
    var query string
    var targetDate time.Time

    switch rule.TriggerType {
    case TriggerBeforeDue:
        // Invoices due in exactly N days
        targetDate = asOfDate.AddDate(0, 0, rule.DaysOffset)
        query = fmt.Sprintf(`
            SELECT i.id, i.invoice_number, i.contact_id, c.name, c.email,
                   i.issue_date::text, i.due_date::text, i.total::text, i.amount_paid::text,
                   (i.total - i.amount_paid)::text, i.currency,
                   (i.due_date::date - $2::date) as days_until_due
            FROM %s.invoices i
            JOIN %s.contacts c ON i.contact_id = c.id
            WHERE i.tenant_id = $1
              AND i.invoice_type = 'SALES'
              AND i.status IN ('SENT', 'PARTIALLY_PAID')
              AND i.due_date::date = $2::date
              AND i.total > i.amount_paid
        `, schemaName, schemaName)
    case TriggerOnDue:
        // Invoices due today
        targetDate = asOfDate
        query = fmt.Sprintf(`
            SELECT i.id, i.invoice_number, i.contact_id, c.name, c.email,
                   i.issue_date::text, i.due_date::text, i.total::text, i.amount_paid::text,
                   (i.total - i.amount_paid)::text, i.currency,
                   0 as days_until_due
            FROM %s.invoices i
            JOIN %s.contacts c ON i.contact_id = c.id
            WHERE i.tenant_id = $1
              AND i.invoice_type = 'SALES'
              AND i.status IN ('SENT', 'PARTIALLY_PAID')
              AND i.due_date::date = $2::date
              AND i.total > i.amount_paid
        `, schemaName, schemaName)
    case TriggerAfterDue:
        // Invoices overdue by exactly N days
        targetDate = asOfDate.AddDate(0, 0, -rule.DaysOffset)
        query = fmt.Sprintf(`
            SELECT i.id, i.invoice_number, i.contact_id, c.name, c.email,
                   i.issue_date::text, i.due_date::text, i.total::text, i.amount_paid::text,
                   (i.total - i.amount_paid)::text, i.currency,
                   ($2::date - i.due_date::date) as days_overdue
            FROM %s.invoices i
            JOIN %s.contacts c ON i.contact_id = c.id
            WHERE i.tenant_id = $1
              AND i.invoice_type = 'SALES'
              AND i.status IN ('SENT', 'PARTIALLY_PAID', 'OVERDUE')
              AND i.due_date::date = $2::date
              AND i.total > i.amount_paid
        `, schemaName, schemaName)
    }

    rows, err := r.db.Query(ctx, query, tenantID, targetDate)
    if err != nil {
        return nil, fmt.Errorf("query invoices for rule: %w", err)
    }
    defer rows.Close()

    var invoices []InvoiceForReminder
    for rows.Next() {
        var inv InvoiceForReminder
        var daysValue int
        err := rows.Scan(
            &inv.ID, &inv.InvoiceNumber, &inv.ContactID, &inv.ContactName, &inv.ContactEmail,
            &inv.IssueDate, &inv.DueDate, &inv.Total, &inv.AmountPaid,
            &inv.OutstandingAmount, &inv.Currency, &daysValue,
        )
        if err != nil {
            return nil, fmt.Errorf("scan invoice: %w", err)
        }

        if rule.TriggerType == TriggerAfterDue {
            inv.DaysOverdue = daysValue
            inv.DaysUntilDue = -daysValue
        } else {
            inv.DaysUntilDue = daysValue
            inv.DaysOverdue = 0
        }

        invoices = append(invoices, inv)
    }

    return invoices, nil
}

func (r *ReminderRulePostgresRepository) HasReminderBeenSent(ctx context.Context, schemaName, tenantID, invoiceID, ruleID string) (bool, error) {
    query := fmt.Sprintf(`
        SELECT COUNT(*) FROM %s.payment_reminders
        WHERE tenant_id = $1 AND invoice_id = $2 AND rule_id = $3 AND status = 'SENT'
    `, schemaName)

    var count int
    err := r.db.QueryRow(ctx, query, tenantID, invoiceID, ruleID).Scan(&count)
    if err != nil {
        return false, fmt.Errorf("check reminder sent: %w", err)
    }

    return count > 0, nil
}

func (r *ReminderRulePostgresRepository) RecordReminderSent(ctx context.Context, schemaName string, reminder *PaymentReminder) error {
    query := fmt.Sprintf(`
        INSERT INTO %s.payment_reminders
        (id, tenant_id, invoice_id, invoice_number, contact_id, contact_name, contact_email,
         rule_id, trigger_type, days_offset, reminder_number, status, sent_at, error_message, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
    `, schemaName)

    _, err := r.db.Exec(ctx, query,
        reminder.ID, reminder.TenantID, reminder.InvoiceID, reminder.InvoiceNumber,
        reminder.ContactID, reminder.ContactName, reminder.ContactEmail,
        reminder.RuleID, reminder.TriggerType, reminder.DaysOffset,
        reminder.ReminderNumber, reminder.Status, reminder.SentAt,
        reminder.ErrorMessage, reminder.CreatedAt, reminder.UpdatedAt,
    )
    if err != nil {
        return fmt.Errorf("insert reminder: %w", err)
    }

    return nil
}

func scanRules(rows pgx.Rows) ([]ReminderRule, error) {
    var rules []ReminderRule
    for rows.Next() {
        var rule ReminderRule
        err := rows.Scan(
            &rule.ID, &rule.TenantID, &rule.Name, &rule.TriggerType, &rule.DaysOffset,
            &rule.EmailTemplateType, &rule.IsActive, &rule.CreatedAt, &rule.UpdatedAt,
        )
        if err != nil {
            return nil, fmt.Errorf("scan rule: %w", err)
        }
        rules = append(rules, rule)
    }
    return rules, nil
}
```

**Step 2: Run build to verify**

Run: `go build ./internal/invoicing/...`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add internal/invoicing/reminder_rule_repository.go
git commit -m "feat(invoicing): add reminder rule repository for database access"
```

---

## Task 4: Update PaymentReminder Type with Rule Fields

**Files:**
- Modify: `internal/invoicing/reminder_types.go`

**Step 1: Add RuleID and related fields to PaymentReminder struct**

Add these fields to the existing PaymentReminder struct:

```go
// PaymentReminder represents a payment reminder for an invoice
type PaymentReminder struct {
    ID             string         `json:"id"`
    TenantID       string         `json:"tenant_id"`
    InvoiceID      string         `json:"invoice_id"`
    InvoiceNumber  string         `json:"invoice_number"`
    ContactID      string         `json:"contact_id"`
    ContactName    string         `json:"contact_name"`
    ContactEmail   string         `json:"contact_email"`
    RuleID         *string        `json:"rule_id,omitempty"`      // NEW: Link to reminder rule
    TriggerType    string         `json:"trigger_type,omitempty"` // NEW: BEFORE_DUE, ON_DUE, AFTER_DUE
    DaysOffset     int            `json:"days_offset,omitempty"`  // NEW: Days from due date
    ReminderNumber int            `json:"reminder_number"`
    Status         ReminderStatus `json:"status"`
    SentAt         *time.Time     `json:"sent_at,omitempty"`
    ErrorMessage   string         `json:"error_message,omitempty"`
    CreatedAt      time.Time      `json:"created_at"`
    UpdatedAt      time.Time      `json:"updated_at"`
}
```

**Step 2: Run tests**

Run: `go test ./internal/invoicing/... -v`
Expected: All tests pass

**Step 3: Commit**

```bash
git add internal/invoicing/reminder_types.go
git commit -m "feat(invoicing): extend PaymentReminder with rule tracking fields"
```

---

## Task 5: Add Automated Reminder Service

**Files:**
- Create: `internal/invoicing/automated_reminder_service.go`

**Step 1: Write the automated reminder service**

```go
package invoicing

import (
    "context"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/rs/zerolog/log"

    "github.com/HMB-research/open-accounting/internal/email"
)

// AutomatedReminderService handles scheduled reminder processing
type AutomatedReminderService struct {
    db           *pgxpool.Pool
    ruleRepo     ReminderRuleRepository
    emailService *email.Service
}

// NewAutomatedReminderService creates a new automated reminder service
func NewAutomatedReminderService(db *pgxpool.Pool, emailService *email.Service) *AutomatedReminderService {
    return &AutomatedReminderService{
        db:           db,
        ruleRepo:     NewReminderRulePostgresRepository(db),
        emailService: emailService,
    }
}

// NewAutomatedReminderServiceWithRepository creates a service with custom repository (for testing)
func NewAutomatedReminderServiceWithRepository(ruleRepo ReminderRuleRepository, emailService *email.Service) *AutomatedReminderService {
    return &AutomatedReminderService{
        ruleRepo:     ruleRepo,
        emailService: emailService,
    }
}

// ProcessRemindersForTenant processes all reminder rules for a tenant
func (s *AutomatedReminderService) ProcessRemindersForTenant(ctx context.Context, tenantID, schemaName, companyName string) ([]AutomatedReminderResult, error) {
    rules, err := s.ruleRepo.ListActiveRules(ctx, schemaName, tenantID)
    if err != nil {
        return nil, fmt.Errorf("list active rules: %w", err)
    }

    var results []AutomatedReminderResult
    asOfDate := time.Now()

    for _, rule := range rules {
        result := s.processRule(ctx, tenantID, schemaName, companyName, &rule, asOfDate)
        results = append(results, result)
    }

    return results, nil
}

func (s *AutomatedReminderService) processRule(ctx context.Context, tenantID, schemaName, companyName string, rule *ReminderRule, asOfDate time.Time) AutomatedReminderResult {
    result := AutomatedReminderResult{
        TenantID: tenantID,
        RuleID:   rule.ID,
        RuleName: rule.Name,
        RunAt:    time.Now(),
    }

    // Get invoices matching this rule
    invoices, err := s.ruleRepo.GetInvoicesForRule(ctx, schemaName, tenantID, rule, asOfDate)
    if err != nil {
        result.Errors = append(result.Errors, fmt.Sprintf("get invoices: %v", err))
        return result
    }

    result.InvoicesFound = len(invoices)

    for _, inv := range invoices {
        // Check if reminder already sent for this rule
        sent, err := s.ruleRepo.HasReminderBeenSent(ctx, schemaName, tenantID, inv.ID, rule.ID)
        if err != nil {
            result.Errors = append(result.Errors, fmt.Sprintf("check sent for %s: %v", inv.InvoiceNumber, err))
            result.Failed++
            continue
        }

        if sent {
            result.Skipped++
            continue
        }

        // Skip if no email
        if inv.ContactEmail == "" {
            result.Skipped++
            continue
        }

        // Send the reminder
        err = s.sendReminder(ctx, tenantID, schemaName, companyName, rule, &inv)
        if err != nil {
            result.Errors = append(result.Errors, fmt.Sprintf("send %s: %v", inv.InvoiceNumber, err))
            result.Failed++
            continue
        }

        result.RemindersSent++
    }

    return result
}

func (s *AutomatedReminderService) sendReminder(ctx context.Context, tenantID, schemaName, companyName string, rule *ReminderRule, inv *InvoiceForReminder) error {
    // Get email template
    templateType := email.TemplateType(rule.EmailTemplateType)
    template, err := s.emailService.GetTemplate(ctx, schemaName, tenantID, templateType)
    if err != nil {
        // Fall back to default overdue template
        template, err = s.emailService.GetTemplate(ctx, schemaName, tenantID, email.TemplateOverdueReminder)
        if err != nil {
            return fmt.Errorf("get template: %w", err)
        }
    }

    // Prepare template data
    data := &email.TemplateData{
        CompanyName:   companyName,
        ContactName:   inv.ContactName,
        InvoiceNumber: inv.InvoiceNumber,
        TotalAmount:   inv.OutstandingAmount,
        Currency:      inv.Currency,
        DueDate:       inv.DueDate,
        DaysOverdue:   inv.DaysOverdue,
        DaysUntilDue:  inv.DaysUntilDue,
    }

    // Render template
    subject, bodyHTML, bodyText, err := s.emailService.RenderTemplate(template, data)
    if err != nil {
        return fmt.Errorf("render template: %w", err)
    }

    // Send email
    _, err = s.emailService.SendEmail(
        ctx,
        schemaName,
        tenantID,
        rule.EmailTemplateType,
        inv.ContactEmail,
        inv.ContactName,
        subject,
        bodyHTML,
        bodyText,
        nil,
        inv.ID,
    )
    if err != nil {
        s.recordReminder(ctx, schemaName, tenantID, rule, inv, ReminderStatusFailed, err.Error())
        return fmt.Errorf("send email: %w", err)
    }

    // Record successful reminder
    s.recordReminder(ctx, schemaName, tenantID, rule, inv, ReminderStatusSent, "")

    return nil
}

func (s *AutomatedReminderService) recordReminder(ctx context.Context, schemaName, tenantID string, rule *ReminderRule, inv *InvoiceForReminder, status ReminderStatus, errorMsg string) {
    now := time.Now()
    reminder := &PaymentReminder{
        ID:            uuid.New().String(),
        TenantID:      tenantID,
        InvoiceID:     inv.ID,
        InvoiceNumber: inv.InvoiceNumber,
        ContactID:     inv.ContactID,
        ContactName:   inv.ContactName,
        ContactEmail:  inv.ContactEmail,
        RuleID:        &rule.ID,
        TriggerType:   string(rule.TriggerType),
        DaysOffset:    rule.DaysOffset,
        Status:        status,
        ErrorMessage:  errorMsg,
        CreatedAt:     now,
        UpdatedAt:     now,
    }

    if status == ReminderStatusSent {
        reminder.SentAt = &now
    }

    if err := s.ruleRepo.RecordReminderSent(ctx, schemaName, reminder); err != nil {
        log.Error().Err(err).Str("invoice_id", inv.ID).Msg("Failed to record reminder")
    }
}

// ListRules returns all reminder rules for a tenant
func (s *AutomatedReminderService) ListRules(ctx context.Context, tenantID, schemaName string) ([]ReminderRule, error) {
    return s.ruleRepo.ListRules(ctx, schemaName, tenantID)
}

// GetRule returns a single rule
func (s *AutomatedReminderService) GetRule(ctx context.Context, tenantID, schemaName, ruleID string) (*ReminderRule, error) {
    return s.ruleRepo.GetRule(ctx, schemaName, tenantID, ruleID)
}

// CreateRule creates a new reminder rule
func (s *AutomatedReminderService) CreateRule(ctx context.Context, tenantID, schemaName string, req *CreateReminderRuleRequest) (*ReminderRule, error) {
    if err := req.Validate(); err != nil {
        return nil, err
    }

    templateType := req.EmailTemplateType
    if templateType == "" {
        switch req.TriggerType {
        case TriggerBeforeDue:
            templateType = "PAYMENT_DUE_SOON"
        case TriggerOnDue:
            templateType = "PAYMENT_DUE_TODAY"
        default:
            templateType = "OVERDUE_REMINDER"
        }
    }

    rule := &ReminderRule{
        ID:                uuid.New().String(),
        TenantID:          tenantID,
        Name:              req.Name,
        TriggerType:       req.TriggerType,
        DaysOffset:        req.DaysOffset,
        EmailTemplateType: templateType,
        IsActive:          req.IsActive,
        CreatedAt:         time.Now(),
        UpdatedAt:         time.Now(),
    }

    if err := s.ruleRepo.CreateRule(ctx, schemaName, rule); err != nil {
        return nil, fmt.Errorf("create rule: %w", err)
    }

    return rule, nil
}

// UpdateRule updates an existing rule
func (s *AutomatedReminderService) UpdateRule(ctx context.Context, tenantID, schemaName, ruleID string, req *UpdateReminderRuleRequest) (*ReminderRule, error) {
    rule, err := s.ruleRepo.GetRule(ctx, schemaName, tenantID, ruleID)
    if err != nil {
        return nil, err
    }

    if req.Name != nil {
        rule.Name = *req.Name
    }
    if req.EmailTemplateType != nil {
        rule.EmailTemplateType = *req.EmailTemplateType
    }
    if req.IsActive != nil {
        rule.IsActive = *req.IsActive
    }
    rule.UpdatedAt = time.Now()

    if err := s.ruleRepo.UpdateRule(ctx, schemaName, rule); err != nil {
        return nil, err
    }

    return rule, nil
}

// DeleteRule deletes a rule
func (s *AutomatedReminderService) DeleteRule(ctx context.Context, tenantID, schemaName, ruleID string) error {
    return s.ruleRepo.DeleteRule(ctx, schemaName, tenantID, ruleID)
}
```

**Step 2: Update email TemplateData to include DaysUntilDue**

In `internal/email/types.go`, add the DaysUntilDue field to TemplateData:

```go
// TemplateData holds data for rendering email templates
type TemplateData struct {
    // Common fields
    CompanyName string
    ContactName string
    Message     string

    // Invoice fields
    InvoiceNumber string
    TotalAmount   string
    Currency      string
    DueDate       string
    IssueDate     string
    DaysOverdue   int
    DaysUntilDue  int  // NEW: For pre-due reminders

    // Payment fields
    Amount      string
    PaymentDate string
    Reference   string
}
```

**Step 3: Run tests**

Run: `go build ./internal/invoicing/... ./internal/email/...`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add internal/invoicing/automated_reminder_service.go internal/email/types.go
git commit -m "feat(invoicing): add automated reminder service for scheduled processing"
```

---

## Task 6: Extend Scheduler with Reminder Job

**Files:**
- Modify: `internal/scheduler/scheduler.go`

**Step 1: Add reminder service dependency and job**

```go
package scheduler

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/robfig/cron/v3"
    "github.com/rs/zerolog/log"

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

// Config holds scheduler configuration
type Config struct {
    RecurringInvoiceSchedule string
    ReminderSchedule         string // NEW: Cron schedule for reminders
    Enabled                  bool
}

// DefaultConfig returns default scheduler configuration
func DefaultConfig() Config {
    return Config{
        RecurringInvoiceSchedule: "0 6 * * *",  // 6:00 AM daily
        ReminderSchedule:         "0 9 * * *",  // 9:00 AM daily
        Enabled:                  true,
    }
}

// Scheduler manages background jobs
type Scheduler struct {
    cron      *cron.Cron
    repo      Repository
    recurring RecurringService
    reminder  AutomatedReminderService
    config    Config
    running   bool
    mu        sync.Mutex
}

// NewScheduler creates a new scheduler instance
func NewScheduler(db *pgxpool.Pool, recurringService *recurring.Service, reminderService *invoicing.AutomatedReminderService, config Config) *Scheduler {
    return &Scheduler{
        cron:      cron.New(cron.WithSeconds()),
        repo:      NewPostgresRepository(db),
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
    schedule := "0 " + s.config.RecurringInvoiceSchedule
    _, err := s.cron.AddFunc(schedule, s.generateDueInvoices)
    if err != nil {
        return fmt.Errorf("failed to add recurring invoice job: %w", err)
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

    s.cron.Start()
    s.running = true

    log.Info().
        Str("recurring_schedule", s.config.RecurringInvoiceSchedule).
        Str("reminder_schedule", s.config.ReminderSchedule).
        Msg("Scheduler started")

    return nil
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

// RunRemindersNow manually triggers the payment reminder processing
func (s *Scheduler) RunRemindersNow() {
    s.processPaymentReminders()
}

// ... rest of existing methods (Stop, generateDueInvoices, RunNow, IsRunning) unchanged
```

**Step 2: Update Repository interface to include CompanyName**

In `internal/scheduler/repository.go`, ensure Tenant struct includes CompanyName:

```go
type Tenant struct {
    ID          string
    SchemaName  string
    CompanyName string  // Needed for email templates
}
```

And update the query in ListActiveTenants to fetch it.

**Step 3: Run tests**

Run: `go build ./internal/scheduler/...`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add internal/scheduler/scheduler.go internal/scheduler/repository.go
git commit -m "feat(scheduler): add automated payment reminder job"
```

---

## Task 7: Add Reminder Rule API Handlers

**Files:**
- Modify: `cmd/api/handlers.go` (add service)
- Create: `cmd/api/handlers_reminder_rules.go`

**Step 1: Add AutomatedReminderService to Handlers struct**

In `cmd/api/handlers.go`, add:

```go
type Handlers struct {
    // ... existing services
    automatedReminderService *invoicing.AutomatedReminderService
}
```

And update NewHandlers to accept it.

**Step 2: Create reminder rule handlers**

```go
package main

import (
    "encoding/json"
    "net/http"

    "github.com/go-chi/chi/v5"

    "github.com/HMB-research/open-accounting/internal/invoicing"
)

// ListReminderRules lists all reminder rules for a tenant
// @Summary List reminder rules
// @Tags Reminder Rules
// @Param tenant_id path string true "Tenant ID"
// @Success 200 {array} invoicing.ReminderRule
// @Router /tenants/{tenant_id}/reminder-rules [get]
func (h *Handlers) ListReminderRules(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    tenantID := chi.URLParam(r, "tenant_id")

    schemaName, err := h.getSchemaName(ctx, tenantID)
    if err != nil {
        h.respondError(w, r, http.StatusBadRequest, "Invalid tenant")
        return
    }

    rules, err := h.automatedReminderService.ListRules(ctx, tenantID, schemaName)
    if err != nil {
        h.logger.Error().Err(err).Msg("Failed to list reminder rules")
        h.respondError(w, r, http.StatusInternalServerError, "Failed to list rules")
        return
    }

    h.respondJSON(w, http.StatusOK, rules)
}

// GetReminderRule gets a single reminder rule
// @Summary Get reminder rule
// @Tags Reminder Rules
// @Param tenant_id path string true "Tenant ID"
// @Param rule_id path string true "Rule ID"
// @Success 200 {object} invoicing.ReminderRule
// @Router /tenants/{tenant_id}/reminder-rules/{rule_id} [get]
func (h *Handlers) GetReminderRule(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    tenantID := chi.URLParam(r, "tenant_id")
    ruleID := chi.URLParam(r, "rule_id")

    schemaName, err := h.getSchemaName(ctx, tenantID)
    if err != nil {
        h.respondError(w, r, http.StatusBadRequest, "Invalid tenant")
        return
    }

    rule, err := h.automatedReminderService.GetRule(ctx, tenantID, schemaName, ruleID)
    if err != nil {
        if _, ok := err.(*invoicing.NotFoundError); ok {
            h.respondError(w, r, http.StatusNotFound, "Rule not found")
            return
        }
        h.logger.Error().Err(err).Msg("Failed to get reminder rule")
        h.respondError(w, r, http.StatusInternalServerError, "Failed to get rule")
        return
    }

    h.respondJSON(w, http.StatusOK, rule)
}

// CreateReminderRule creates a new reminder rule
// @Summary Create reminder rule
// @Tags Reminder Rules
// @Param tenant_id path string true "Tenant ID"
// @Param body body invoicing.CreateReminderRuleRequest true "Rule data"
// @Success 201 {object} invoicing.ReminderRule
// @Router /tenants/{tenant_id}/reminder-rules [post]
func (h *Handlers) CreateReminderRule(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    tenantID := chi.URLParam(r, "tenant_id")

    schemaName, err := h.getSchemaName(ctx, tenantID)
    if err != nil {
        h.respondError(w, r, http.StatusBadRequest, "Invalid tenant")
        return
    }

    var req invoicing.CreateReminderRuleRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        h.respondError(w, r, http.StatusBadRequest, "Invalid request body")
        return
    }

    rule, err := h.automatedReminderService.CreateRule(ctx, tenantID, schemaName, &req)
    if err != nil {
        if _, ok := err.(*invoicing.ValidationError); ok {
            h.respondError(w, r, http.StatusBadRequest, err.Error())
            return
        }
        h.logger.Error().Err(err).Msg("Failed to create reminder rule")
        h.respondError(w, r, http.StatusInternalServerError, "Failed to create rule")
        return
    }

    h.respondJSON(w, http.StatusCreated, rule)
}

// UpdateReminderRule updates a reminder rule
// @Summary Update reminder rule
// @Tags Reminder Rules
// @Param tenant_id path string true "Tenant ID"
// @Param rule_id path string true "Rule ID"
// @Param body body invoicing.UpdateReminderRuleRequest true "Rule data"
// @Success 200 {object} invoicing.ReminderRule
// @Router /tenants/{tenant_id}/reminder-rules/{rule_id} [put]
func (h *Handlers) UpdateReminderRule(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    tenantID := chi.URLParam(r, "tenant_id")
    ruleID := chi.URLParam(r, "rule_id")

    schemaName, err := h.getSchemaName(ctx, tenantID)
    if err != nil {
        h.respondError(w, r, http.StatusBadRequest, "Invalid tenant")
        return
    }

    var req invoicing.UpdateReminderRuleRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        h.respondError(w, r, http.StatusBadRequest, "Invalid request body")
        return
    }

    rule, err := h.automatedReminderService.UpdateRule(ctx, tenantID, schemaName, ruleID, &req)
    if err != nil {
        if _, ok := err.(*invoicing.NotFoundError); ok {
            h.respondError(w, r, http.StatusNotFound, "Rule not found")
            return
        }
        h.logger.Error().Err(err).Msg("Failed to update reminder rule")
        h.respondError(w, r, http.StatusInternalServerError, "Failed to update rule")
        return
    }

    h.respondJSON(w, http.StatusOK, rule)
}

// DeleteReminderRule deletes a reminder rule
// @Summary Delete reminder rule
// @Tags Reminder Rules
// @Param tenant_id path string true "Tenant ID"
// @Param rule_id path string true "Rule ID"
// @Success 204 "No Content"
// @Router /tenants/{tenant_id}/reminder-rules/{rule_id} [delete]
func (h *Handlers) DeleteReminderRule(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    tenantID := chi.URLParam(r, "tenant_id")
    ruleID := chi.URLParam(r, "rule_id")

    schemaName, err := h.getSchemaName(ctx, tenantID)
    if err != nil {
        h.respondError(w, r, http.StatusBadRequest, "Invalid tenant")
        return
    }

    err = h.automatedReminderService.DeleteRule(ctx, tenantID, schemaName, ruleID)
    if err != nil {
        if _, ok := err.(*invoicing.NotFoundError); ok {
            h.respondError(w, r, http.StatusNotFound, "Rule not found")
            return
        }
        h.logger.Error().Err(err).Msg("Failed to delete reminder rule")
        h.respondError(w, r, http.StatusInternalServerError, "Failed to delete rule")
        return
    }

    w.WriteHeader(http.StatusNoContent)
}

// TriggerReminders manually triggers reminder processing for a tenant
// @Summary Trigger reminders
// @Tags Reminder Rules
// @Param tenant_id path string true "Tenant ID"
// @Success 200 {array} invoicing.AutomatedReminderResult
// @Router /tenants/{tenant_id}/reminder-rules/trigger [post]
func (h *Handlers) TriggerReminders(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    tenantID := chi.URLParam(r, "tenant_id")

    schemaName, err := h.getSchemaName(ctx, tenantID)
    if err != nil {
        h.respondError(w, r, http.StatusBadRequest, "Invalid tenant")
        return
    }

    tenant, err := h.tenantService.GetTenant(ctx, tenantID)
    if err != nil {
        h.respondError(w, r, http.StatusBadRequest, "Tenant not found")
        return
    }

    results, err := h.automatedReminderService.ProcessRemindersForTenant(ctx, tenantID, schemaName, tenant.CompanyName)
    if err != nil {
        h.logger.Error().Err(err).Msg("Failed to trigger reminders")
        h.respondError(w, r, http.StatusInternalServerError, "Failed to process reminders")
        return
    }

    h.respondJSON(w, http.StatusOK, results)
}
```

**Step 3: Add routes**

In `cmd/api/routes.go`, add:

```go
// Reminder Rules
r.Route("/tenants/{tenant_id}/reminder-rules", func(r chi.Router) {
    r.Use(h.TenantContext)
    r.Get("/", h.ListReminderRules)
    r.Post("/", h.CreateReminderRule)
    r.Post("/trigger", h.TriggerReminders)
    r.Get("/{rule_id}", h.GetReminderRule)
    r.Put("/{rule_id}", h.UpdateReminderRule)
    r.Delete("/{rule_id}", h.DeleteReminderRule)
})
```

**Step 4: Run tests**

Run: `go build ./cmd/api/...`
Expected: Build succeeds

**Step 5: Commit**

```bash
git add cmd/api/handlers.go cmd/api/handlers_reminder_rules.go cmd/api/routes.go
git commit -m "feat(api): add reminder rules CRUD endpoints"
```

---

## Task 8: Add Frontend API Methods

**Files:**
- Modify: `frontend/src/lib/api.ts`

**Step 1: Add TypeScript types and API methods**

```typescript
// Reminder Rule Types
export interface ReminderRule {
  id: string;
  tenant_id: string;
  name: string;
  trigger_type: 'BEFORE_DUE' | 'ON_DUE' | 'AFTER_DUE';
  days_offset: number;
  email_template_type: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface CreateReminderRuleRequest {
  name: string;
  trigger_type: 'BEFORE_DUE' | 'ON_DUE' | 'AFTER_DUE';
  days_offset: number;
  email_template_type?: string;
  is_active: boolean;
}

export interface UpdateReminderRuleRequest {
  name?: string;
  email_template_type?: string;
  is_active?: boolean;
}

export interface AutomatedReminderResult {
  tenant_id: string;
  rule_id: string;
  rule_name: string;
  invoices_found: number;
  reminders_sent: number;
  skipped: number;
  failed: number;
  errors?: string[];
  run_at: string;
}

// API Methods
export const api = {
  // ... existing methods

  // Reminder Rules
  async listReminderRules(tenantId: string): Promise<ReminderRule[]> {
    return this.fetch(`/tenants/${tenantId}/reminder-rules`);
  },

  async getReminderRule(tenantId: string, ruleId: string): Promise<ReminderRule> {
    return this.fetch(`/tenants/${tenantId}/reminder-rules/${ruleId}`);
  },

  async createReminderRule(tenantId: string, data: CreateReminderRuleRequest): Promise<ReminderRule> {
    return this.fetch(`/tenants/${tenantId}/reminder-rules`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  },

  async updateReminderRule(tenantId: string, ruleId: string, data: UpdateReminderRuleRequest): Promise<ReminderRule> {
    return this.fetch(`/tenants/${tenantId}/reminder-rules/${ruleId}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  },

  async deleteReminderRule(tenantId: string, ruleId: string): Promise<void> {
    return this.fetch(`/tenants/${tenantId}/reminder-rules/${ruleId}`, {
      method: 'DELETE',
    });
  },

  async triggerReminders(tenantId: string): Promise<AutomatedReminderResult[]> {
    return this.fetch(`/tenants/${tenantId}/reminder-rules/trigger`, {
      method: 'POST',
    });
  },
};
```

**Step 2: Run type check**

Run: `cd frontend && bun run check`
Expected: No type errors

**Step 3: Commit**

```bash
git add frontend/src/lib/api.ts
git commit -m "feat(frontend): add reminder rules API methods"
```

---

## Task 9: Create Reminder Rules Settings Page

**Files:**
- Create: `frontend/src/routes/settings/reminders/+page.svelte`

**Step 1: Create the settings page**

```svelte
<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { api, type ReminderRule, type CreateReminderRuleRequest, type AutomatedReminderResult } from '$lib/api';
  import * as m from '$lib/paraglide/messages.js';

  let tenantId = $derived($page.url.searchParams.get('tenant') || '');
  let isLoading = $state(false);
  let isSaving = $state(false);
  let isTriggering = $state(false);
  let error = $state('');
  let successMessage = $state('');

  let rules = $state<ReminderRule[]>([]);
  let showCreateModal = $state(false);
  let editingRule = $state<ReminderRule | null>(null);
  let triggerResults = $state<AutomatedReminderResult[] | null>(null);

  // Form state
  let formName = $state('');
  let formTriggerType = $state<'BEFORE_DUE' | 'ON_DUE' | 'AFTER_DUE'>('AFTER_DUE');
  let formDaysOffset = $state(7);
  let formIsActive = $state(true);

  const triggerTypeLabels = {
    BEFORE_DUE: 'Before Due Date',
    ON_DUE: 'On Due Date',
    AFTER_DUE: 'After Due Date (Overdue)',
  };

  onMount(() => {
    if (tenantId) {
      loadRules();
    }
  });

  async function loadRules() {
    isLoading = true;
    error = '';
    try {
      rules = await api.listReminderRules(tenantId);
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to load rules';
    } finally {
      isLoading = false;
    }
  }

  function openCreateModal() {
    formName = '';
    formTriggerType = 'AFTER_DUE';
    formDaysOffset = 7;
    formIsActive = true;
    editingRule = null;
    showCreateModal = true;
  }

  function openEditModal(rule: ReminderRule) {
    formName = rule.name;
    formTriggerType = rule.trigger_type;
    formDaysOffset = rule.days_offset;
    formIsActive = rule.is_active;
    editingRule = rule;
    showCreateModal = true;
  }

  function closeModal() {
    showCreateModal = false;
    editingRule = null;
  }

  async function saveRule() {
    isSaving = true;
    error = '';
    successMessage = '';

    try {
      if (editingRule) {
        await api.updateReminderRule(tenantId, editingRule.id, {
          name: formName,
          is_active: formIsActive,
        });
        successMessage = 'Rule updated successfully';
      } else {
        await api.createReminderRule(tenantId, {
          name: formName,
          trigger_type: formTriggerType,
          days_offset: formDaysOffset,
          is_active: formIsActive,
        });
        successMessage = 'Rule created successfully';
      }
      closeModal();
      await loadRules();
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to save rule';
    } finally {
      isSaving = false;
    }
  }

  async function deleteRule(rule: ReminderRule) {
    if (!confirm(`Delete rule "${rule.name}"?`)) return;

    try {
      await api.deleteReminderRule(tenantId, rule.id);
      successMessage = 'Rule deleted';
      await loadRules();
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to delete rule';
    }
  }

  async function toggleRule(rule: ReminderRule) {
    try {
      await api.updateReminderRule(tenantId, rule.id, {
        is_active: !rule.is_active,
      });
      await loadRules();
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to toggle rule';
    }
  }

  async function triggerReminders() {
    isTriggering = true;
    error = '';
    triggerResults = null;

    try {
      triggerResults = await api.triggerReminders(tenantId);
      successMessage = `Processed ${triggerResults.length} rules`;
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to trigger reminders';
    } finally {
      isTriggering = false;
    }
  }

  function formatTrigger(rule: ReminderRule): string {
    if (rule.trigger_type === 'BEFORE_DUE') {
      return `${rule.days_offset} days before due`;
    } else if (rule.trigger_type === 'ON_DUE') {
      return 'On due date';
    } else {
      return `${rule.days_offset} days overdue`;
    }
  }
</script>

<svelte:head>
  <title>Reminder Settings - Open Accounting</title>
</svelte:head>

<div class="container">
  <div class="header">
    <h1>Payment Reminder Rules</h1>
    <div class="header-actions">
      <button class="btn btn-secondary" onclick={triggerReminders} disabled={isTriggering}>
        {isTriggering ? 'Processing...' : 'Run Now'}
      </button>
      <button class="btn btn-primary" onclick={openCreateModal}>
        Add Rule
      </button>
    </div>
  </div>

  {#if error}
    <div class="alert alert-error">{error}</div>
  {/if}

  {#if successMessage}
    <div class="alert alert-success">{successMessage}</div>
  {/if}

  {#if triggerResults}
    <div class="card results-card">
      <h3>Manual Run Results</h3>
      {#each triggerResults as result}
        <div class="result-item">
          <strong>{result.rule_name}</strong>:
          {result.reminders_sent} sent, {result.skipped} skipped, {result.failed} failed
          {#if result.errors?.length}
            <ul class="error-list">
              {#each result.errors as err}
                <li>{err}</li>
              {/each}
            </ul>
          {/if}
        </div>
      {/each}
      <button class="btn btn-sm btn-secondary" onclick={() => triggerResults = null}>Dismiss</button>
    </div>
  {/if}

  <div class="card">
    <p class="description">
      Configure when automatic payment reminders are sent to your customers.
      Reminders are processed daily at 9:00 AM.
    </p>

    {#if isLoading}
      <p>Loading...</p>
    {:else if rules.length === 0}
      <p class="empty-state">No reminder rules configured. Click "Add Rule" to create one.</p>
    {:else}
      <table class="table">
        <thead>
          <tr>
            <th>Name</th>
            <th>Trigger</th>
            <th>Template</th>
            <th>Status</th>
            <th class="text-right">Actions</th>
          </tr>
        </thead>
        <tbody>
          {#each rules as rule}
            <tr class:inactive={!rule.is_active}>
              <td>{rule.name}</td>
              <td>{formatTrigger(rule)}</td>
              <td><code>{rule.email_template_type}</code></td>
              <td>
                <button
                  class="status-toggle"
                  class:active={rule.is_active}
                  onclick={() => toggleRule(rule)}
                >
                  {rule.is_active ? 'Active' : 'Inactive'}
                </button>
              </td>
              <td class="text-right">
                <button class="btn btn-sm btn-secondary" onclick={() => openEditModal(rule)}>
                  Edit
                </button>
                <button class="btn btn-sm btn-danger" onclick={() => deleteRule(rule)}>
                  Delete
                </button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>
</div>

{#if showCreateModal}
  <div class="modal-overlay" onclick={closeModal}>
    <div class="modal" onclick={(e) => e.stopPropagation()}>
      <div class="modal-header">
        <h2>{editingRule ? 'Edit Rule' : 'Create Rule'}</h2>
        <button class="btn-close" onclick={closeModal}>&times;</button>
      </div>

      <div class="modal-body">
        <div class="form-group">
          <label for="name">Rule Name</label>
          <input type="text" id="name" bind:value={formName} placeholder="e.g., 7 Days Overdue" />
        </div>

        {#if !editingRule}
          <div class="form-group">
            <label for="triggerType">Trigger Type</label>
            <select id="triggerType" bind:value={formTriggerType}>
              <option value="BEFORE_DUE">Before Due Date</option>
              <option value="ON_DUE">On Due Date</option>
              <option value="AFTER_DUE">After Due Date (Overdue)</option>
            </select>
          </div>

          {#if formTriggerType !== 'ON_DUE'}
            <div class="form-group">
              <label for="daysOffset">Days Offset</label>
              <input type="number" id="daysOffset" bind:value={formDaysOffset} min="1" max="365" />
              <small>
                {formTriggerType === 'BEFORE_DUE'
                  ? `Reminder sent ${formDaysOffset} days before due date`
                  : `Reminder sent ${formDaysOffset} days after due date`}
              </small>
            </div>
          {/if}
        {/if}

        <div class="form-group">
          <label class="checkbox-label">
            <input type="checkbox" bind:checked={formIsActive} />
            Active
          </label>
        </div>
      </div>

      <div class="modal-footer">
        <button class="btn btn-secondary" onclick={closeModal} disabled={isSaving}>
          Cancel
        </button>
        <button class="btn btn-primary" onclick={saveRule} disabled={isSaving || !formName}>
          {isSaving ? 'Saving...' : 'Save'}
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  .header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1.5rem;
  }

  .header-actions {
    display: flex;
    gap: 0.5rem;
  }

  .description {
    color: var(--color-text-muted);
    margin-bottom: 1.5rem;
  }

  .table {
    width: 100%;
    border-collapse: collapse;
  }

  .table th,
  .table td {
    padding: 0.75rem;
    border-bottom: 1px solid var(--color-border);
    text-align: left;
  }

  .table th {
    font-weight: 600;
    color: var(--color-text-muted);
    font-size: 0.875rem;
  }

  .text-right {
    text-align: right;
  }

  .inactive {
    opacity: 0.6;
  }

  .status-toggle {
    padding: 0.25rem 0.5rem;
    border: none;
    border-radius: 0.25rem;
    cursor: pointer;
    font-size: 0.75rem;
    background: var(--color-error);
    color: white;
  }

  .status-toggle.active {
    background: var(--color-success);
  }

  .results-card {
    margin-bottom: 1rem;
    background: var(--color-bg);
  }

  .results-card h3 {
    margin-bottom: 0.5rem;
  }

  .result-item {
    padding: 0.5rem 0;
    border-bottom: 1px solid var(--color-border);
  }

  .error-list {
    color: var(--color-error);
    font-size: 0.875rem;
    margin: 0.25rem 0 0 1rem;
  }

  .btn-sm {
    padding: 0.25rem 0.5rem;
    font-size: 0.875rem;
  }

  .btn-danger {
    background: var(--color-error);
    color: white;
  }

  .empty-state {
    text-align: center;
    padding: 2rem;
    color: var(--color-text-muted);
  }

  code {
    background: var(--color-bg);
    padding: 0.125rem 0.375rem;
    border-radius: 0.25rem;
    font-size: 0.875rem;
  }

  /* Modal styles */
  .modal-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
  }

  .modal {
    background: var(--color-surface);
    border-radius: 0.5rem;
    max-width: 500px;
    width: 100%;
    margin: 1rem;
  }

  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 1rem 1.5rem;
    border-bottom: 1px solid var(--color-border);
  }

  .modal-header h2 {
    margin: 0;
    font-size: 1.25rem;
  }

  .btn-close {
    background: none;
    border: none;
    font-size: 1.5rem;
    cursor: pointer;
    color: var(--color-text-muted);
  }

  .modal-body {
    padding: 1.5rem;
  }

  .modal-footer {
    display: flex;
    justify-content: flex-end;
    gap: 0.75rem;
    padding: 1rem 1.5rem;
    border-top: 1px solid var(--color-border);
  }

  .form-group {
    margin-bottom: 1rem;
  }

  .form-group label {
    display: block;
    margin-bottom: 0.25rem;
    font-weight: 500;
  }

  .form-group input[type='text'],
  .form-group input[type='number'],
  .form-group select {
    width: 100%;
    padding: 0.5rem;
    border: 1px solid var(--color-border);
    border-radius: 0.25rem;
  }

  .form-group small {
    display: block;
    margin-top: 0.25rem;
    color: var(--color-text-muted);
    font-size: 0.75rem;
  }

  .checkbox-label {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    cursor: pointer;
  }

  .checkbox-label input {
    width: auto;
  }
</style>
```

**Step 2: Add navigation link**

Add a link to the settings page from the reminders page or settings menu.

**Step 3: Run dev server to test**

Run: `cd frontend && bun run dev`
Expected: Page loads and displays correctly

**Step 4: Commit**

```bash
git add frontend/src/routes/settings/reminders/+page.svelte
git commit -m "feat(frontend): add reminder rules settings page"
```

---

## Task 10: Add Unit Tests for Automated Reminder Service

**Files:**
- Create: `internal/invoicing/automated_reminder_service_test.go`

**Step 1: Write tests**

```go
package invoicing

import (
    "context"
    "testing"
    "time"
)

type mockReminderRuleRepo struct {
    rules            []ReminderRule
    invoices         []InvoiceForReminder
    sentReminders    map[string]bool
    recordedReminder *PaymentReminder
}

func (m *mockReminderRuleRepo) ListRules(ctx context.Context, schemaName, tenantID string) ([]ReminderRule, error) {
    return m.rules, nil
}

func (m *mockReminderRuleRepo) ListActiveRules(ctx context.Context, schemaName, tenantID string) ([]ReminderRule, error) {
    var active []ReminderRule
    for _, r := range m.rules {
        if r.IsActive {
            active = append(active, r)
        }
    }
    return active, nil
}

func (m *mockReminderRuleRepo) GetRule(ctx context.Context, schemaName, tenantID, ruleID string) (*ReminderRule, error) {
    for _, r := range m.rules {
        if r.ID == ruleID {
            return &r, nil
        }
    }
    return nil, ErrRuleNotFound
}

func (m *mockReminderRuleRepo) CreateRule(ctx context.Context, schemaName string, rule *ReminderRule) error {
    m.rules = append(m.rules, *rule)
    return nil
}

func (m *mockReminderRuleRepo) UpdateRule(ctx context.Context, schemaName string, rule *ReminderRule) error {
    for i, r := range m.rules {
        if r.ID == rule.ID {
            m.rules[i] = *rule
            return nil
        }
    }
    return ErrRuleNotFound
}

func (m *mockReminderRuleRepo) DeleteRule(ctx context.Context, schemaName, tenantID, ruleID string) error {
    for i, r := range m.rules {
        if r.ID == ruleID {
            m.rules = append(m.rules[:i], m.rules[i+1:]...)
            return nil
        }
    }
    return ErrRuleNotFound
}

func (m *mockReminderRuleRepo) GetInvoicesForRule(ctx context.Context, schemaName, tenantID string, rule *ReminderRule, asOfDate time.Time) ([]InvoiceForReminder, error) {
    return m.invoices, nil
}

func (m *mockReminderRuleRepo) HasReminderBeenSent(ctx context.Context, schemaName, tenantID, invoiceID, ruleID string) (bool, error) {
    key := invoiceID + ":" + ruleID
    return m.sentReminders[key], nil
}

func (m *mockReminderRuleRepo) RecordReminderSent(ctx context.Context, schemaName string, reminder *PaymentReminder) error {
    m.recordedReminder = reminder
    if m.sentReminders == nil {
        m.sentReminders = make(map[string]bool)
    }
    if reminder.RuleID != nil {
        m.sentReminders[reminder.InvoiceID+":"+*reminder.RuleID] = true
    }
    return nil
}

func TestCreateReminderRule(t *testing.T) {
    repo := &mockReminderRuleRepo{}
    service := NewAutomatedReminderServiceWithRepository(repo, nil)

    req := &CreateReminderRuleRequest{
        Name:        "7 Days Overdue",
        TriggerType: TriggerAfterDue,
        DaysOffset:  7,
        IsActive:    true,
    }

    rule, err := service.CreateRule(context.Background(), "tenant-1", "tenant_abc", req)
    if err != nil {
        t.Fatalf("CreateRule failed: %v", err)
    }

    if rule.Name != "7 Days Overdue" {
        t.Errorf("Expected name '7 Days Overdue', got '%s'", rule.Name)
    }
    if rule.TriggerType != TriggerAfterDue {
        t.Errorf("Expected trigger type AFTER_DUE, got '%s'", rule.TriggerType)
    }
    if rule.EmailTemplateType != "OVERDUE_REMINDER" {
        t.Errorf("Expected template OVERDUE_REMINDER, got '%s'", rule.EmailTemplateType)
    }
}

func TestCreateReminderRuleValidation(t *testing.T) {
    repo := &mockReminderRuleRepo{}
    service := NewAutomatedReminderServiceWithRepository(repo, nil)

    tests := []struct {
        name    string
        req     *CreateReminderRuleRequest
        wantErr bool
    }{
        {
            name: "missing name",
            req: &CreateReminderRuleRequest{
                TriggerType: TriggerAfterDue,
                DaysOffset:  7,
            },
            wantErr: true,
        },
        {
            name: "missing trigger type",
            req: &CreateReminderRuleRequest{
                Name:       "Test",
                DaysOffset: 7,
            },
            wantErr: true,
        },
        {
            name: "negative days offset",
            req: &CreateReminderRuleRequest{
                Name:        "Test",
                TriggerType: TriggerAfterDue,
                DaysOffset:  -1,
            },
            wantErr: true,
        },
        {
            name: "valid request",
            req: &CreateReminderRuleRequest{
                Name:        "Test",
                TriggerType: TriggerAfterDue,
                DaysOffset:  7,
                IsActive:    true,
            },
            wantErr: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := service.CreateRule(context.Background(), "tenant-1", "tenant_abc", tt.req)
            if (err != nil) != tt.wantErr {
                t.Errorf("CreateRule() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}

func TestListRules(t *testing.T) {
    repo := &mockReminderRuleRepo{
        rules: []ReminderRule{
            {ID: "1", Name: "Rule 1", IsActive: true},
            {ID: "2", Name: "Rule 2", IsActive: false},
        },
    }
    service := NewAutomatedReminderServiceWithRepository(repo, nil)

    rules, err := service.ListRules(context.Background(), "tenant-1", "tenant_abc")
    if err != nil {
        t.Fatalf("ListRules failed: %v", err)
    }

    if len(rules) != 2 {
        t.Errorf("Expected 2 rules, got %d", len(rules))
    }
}

func TestDefaultTemplateAssignment(t *testing.T) {
    repo := &mockReminderRuleRepo{}
    service := NewAutomatedReminderServiceWithRepository(repo, nil)

    tests := []struct {
        triggerType      TriggerType
        expectedTemplate string
    }{
        {TriggerBeforeDue, "PAYMENT_DUE_SOON"},
        {TriggerOnDue, "PAYMENT_DUE_TODAY"},
        {TriggerAfterDue, "OVERDUE_REMINDER"},
    }

    for _, tt := range tests {
        t.Run(string(tt.triggerType), func(t *testing.T) {
            req := &CreateReminderRuleRequest{
                Name:        "Test",
                TriggerType: tt.triggerType,
                DaysOffset:  7,
                IsActive:    true,
            }

            rule, err := service.CreateRule(context.Background(), "tenant-1", "tenant_abc", req)
            if err != nil {
                t.Fatalf("CreateRule failed: %v", err)
            }

            if rule.EmailTemplateType != tt.expectedTemplate {
                t.Errorf("Expected template '%s', got '%s'", tt.expectedTemplate, rule.EmailTemplateType)
            }
        })
    }
}
```

**Step 2: Run tests**

Run: `go test ./internal/invoicing/... -v -run TestReminder`
Expected: All tests pass

**Step 3: Commit**

```bash
git add internal/invoicing/automated_reminder_service_test.go
git commit -m "test(invoicing): add unit tests for automated reminder service"
```

---

## Task 11: Update Main Application Initialization

**Files:**
- Modify: `cmd/api/main.go`

**Step 1: Initialize AutomatedReminderService and update Scheduler**

```go
// In main.go, add:

// Initialize automated reminder service
automatedReminderService := invoicing.NewAutomatedReminderService(db, emailService)

// Initialize scheduler with reminder service
schedulerConfig := scheduler.DefaultConfig()
sched := scheduler.NewScheduler(db, recurringService, automatedReminderService, schedulerConfig)

// Update Handlers initialization to include automatedReminderService
handlers := NewHandlers(
    // ... existing services
    automatedReminderService,
)
```

**Step 2: Run build**

Run: `go build ./cmd/api/...`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add cmd/api/main.go
git commit -m "feat(api): wire up automated reminder service in main"
```

---

## Task 12: Add Integration Test

**Files:**
- Create: `internal/invoicing/reminder_integration_test.go`

**Step 1: Write integration test (requires test database)**

```go
//go:build integration

package invoicing

import (
    "context"
    "testing"
    "time"

    "github.com/HMB-research/open-accounting/internal/testutil"
)

func TestReminderRuleIntegration(t *testing.T) {
    db, cleanup := testutil.SetupTestDB(t)
    defer cleanup()

    repo := NewReminderRulePostgresRepository(db)
    tenantID := "test-tenant"
    schemaName := "tenant_test"

    // Create a rule
    rule := &ReminderRule{
        ID:                "rule-1",
        TenantID:          tenantID,
        Name:              "7 Days Overdue",
        TriggerType:       TriggerAfterDue,
        DaysOffset:        7,
        EmailTemplateType: "OVERDUE_REMINDER",
        IsActive:          true,
        CreatedAt:         time.Now(),
        UpdatedAt:         time.Now(),
    }

    err := repo.CreateRule(context.Background(), schemaName, rule)
    if err != nil {
        t.Fatalf("CreateRule failed: %v", err)
    }

    // List rules
    rules, err := repo.ListRules(context.Background(), schemaName, tenantID)
    if err != nil {
        t.Fatalf("ListRules failed: %v", err)
    }

    if len(rules) != 1 {
        t.Errorf("Expected 1 rule, got %d", len(rules))
    }

    // Update rule
    rule.Name = "Updated Name"
    err = repo.UpdateRule(context.Background(), schemaName, rule)
    if err != nil {
        t.Fatalf("UpdateRule failed: %v", err)
    }

    // Verify update
    updated, err := repo.GetRule(context.Background(), schemaName, tenantID, rule.ID)
    if err != nil {
        t.Fatalf("GetRule failed: %v", err)
    }

    if updated.Name != "Updated Name" {
        t.Errorf("Expected name 'Updated Name', got '%s'", updated.Name)
    }

    // Delete rule
    err = repo.DeleteRule(context.Background(), schemaName, tenantID, rule.ID)
    if err != nil {
        t.Fatalf("DeleteRule failed: %v", err)
    }

    // Verify deletion
    rules, _ = repo.ListRules(context.Background(), schemaName, tenantID)
    if len(rules) != 0 {
        t.Errorf("Expected 0 rules after deletion, got %d", len(rules))
    }
}
```

**Step 2: Run integration tests (if test DB available)**

Run: `go test ./internal/invoicing/... -v -tags=integration`
Expected: Tests pass (or skip if no DB)

**Step 3: Commit**

```bash
git add internal/invoicing/reminder_integration_test.go
git commit -m "test(invoicing): add reminder rule integration tests"
```

---

## Task 13: Run Migration and Full Test

**Step 1: Run database migration**

Run: `go run cmd/migrate/main.go up`
Expected: Migration 021 applies successfully

**Step 2: Run all tests**

Run: `go test ./... -v`
Expected: All tests pass

**Step 3: Run linter**

Run: `golangci-lint run`
Expected: No lint errors

**Step 4: Start server and test manually**

Run: `go run cmd/api/main.go`
Test: Create a reminder rule via API, verify it appears in the database

**Step 5: Final commit**

```bash
git add -A
git commit -m "feat: complete payment tracking and automated reminders implementation"
```

---

## Summary

This plan implements:

1. **Database schema** for reminder rules with configurable triggers (BEFORE_DUE, ON_DUE, AFTER_DUE)
2. **New email templates** for pre-due and due-today reminders
3. **Automated reminder service** that processes rules and sends emails
4. **Scheduler integration** for daily automated processing
5. **REST API endpoints** for CRUD operations on reminder rules
6. **Frontend settings page** for managing reminder rules
7. **Comprehensive tests** for all new functionality

The implementation follows existing codebase patterns:
- Multi-tenant schema isolation
- Repository pattern for data access
- Service layer for business logic
- Chi router for HTTP handlers
- Svelte 5 with runes for frontend state
