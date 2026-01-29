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
