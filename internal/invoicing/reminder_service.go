package invoicing

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/email"
)

// ReminderRepository defines the interface for payment reminder data access
type ReminderRepository interface {
	// GetOverdueInvoices retrieves all overdue sales invoices
	GetOverdueInvoices(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) ([]OverdueInvoice, error)

	// GetReminderCount gets the number of reminders sent for an invoice
	GetReminderCount(ctx context.Context, schemaName, tenantID, invoiceID string) (int, *time.Time, error)

	// CreateReminder creates a new payment reminder record
	CreateReminder(ctx context.Context, schemaName string, reminder *PaymentReminder) error

	// UpdateReminderStatus updates the status of a reminder
	UpdateReminderStatus(ctx context.Context, schemaName, reminderID string, status ReminderStatus, sentAt *time.Time, errorMsg string) error

	// GetRemindersByInvoice gets all reminders for an invoice
	GetRemindersByInvoice(ctx context.Context, schemaName, tenantID, invoiceID string) ([]PaymentReminder, error)
}

// ReminderService provides payment reminder operations
type ReminderService struct {
	db           *pgxpool.Pool
	repo         ReminderRepository
	emailService *email.Service
}

// NewReminderService creates a new reminder service
func NewReminderService(db *pgxpool.Pool, emailService *email.Service) *ReminderService {
	return &ReminderService{
		db:           db,
		repo:         NewReminderPostgresRepository(db),
		emailService: emailService,
	}
}

// NewReminderServiceWithRepository creates a reminder service with custom repository
func NewReminderServiceWithRepository(repo ReminderRepository, emailService *email.Service) *ReminderService {
	return &ReminderService{
		repo:         repo,
		emailService: emailService,
	}
}

// GetOverdueInvoicesSummary retrieves a summary of all overdue invoices
func (s *ReminderService) GetOverdueInvoicesSummary(ctx context.Context, tenantID, schemaName string) (*OverdueInvoicesSummary, error) {
	asOfDate := time.Now()

	invoices, err := s.repo.GetOverdueInvoices(ctx, schemaName, tenantID, asOfDate)
	if err != nil {
		return nil, fmt.Errorf("get overdue invoices: %w", err)
	}

	// Enrich with reminder info
	for i := range invoices {
		count, lastAt, err := s.repo.GetReminderCount(ctx, schemaName, tenantID, invoices[i].ID)
		if err != nil {
			return nil, fmt.Errorf("get reminder count for %s: %w", invoices[i].ID, err)
		}
		invoices[i].ReminderCount = count
		invoices[i].LastReminderAt = lastAt
	}

	// Calculate summary statistics
	var totalOverdue decimal.Decimal
	var totalDaysOverdue int
	contactSet := make(map[string]bool)

	for _, inv := range invoices {
		totalOverdue = totalOverdue.Add(inv.OutstandingAmount)
		totalDaysOverdue += inv.DaysOverdue
		contactSet[inv.ContactID] = true
	}

	avgDaysOverdue := 0
	if len(invoices) > 0 {
		avgDaysOverdue = totalDaysOverdue / len(invoices)
	}

	return &OverdueInvoicesSummary{
		TotalOverdue:       totalOverdue,
		InvoiceCount:       len(invoices),
		ContactCount:       len(contactSet),
		AverageDaysOverdue: avgDaysOverdue,
		Invoices:           invoices,
		GeneratedAt:        time.Now(),
	}, nil
}

// SendReminder sends a payment reminder for a specific invoice
func (s *ReminderService) SendReminder(ctx context.Context, tenantID, schemaName string, req *SendReminderRequest, companyName string) (*ReminderResult, error) {
	// Get current overdue invoices to find the specific one
	asOfDate := time.Now()
	invoices, err := s.repo.GetOverdueInvoices(ctx, schemaName, tenantID, asOfDate)
	if err != nil {
		return nil, fmt.Errorf("get overdue invoices: %w", err)
	}

	var targetInvoice *OverdueInvoice
	for _, inv := range invoices {
		if inv.ID == req.InvoiceID {
			targetInvoice = &inv
			break
		}
	}

	if targetInvoice == nil {
		return &ReminderResult{
			InvoiceID: req.InvoiceID,
			Success:   false,
			Message:   "Invoice not found or not overdue",
		}, nil
	}

	// Check if contact has email
	if targetInvoice.ContactEmail == "" {
		return &ReminderResult{
			InvoiceID:     req.InvoiceID,
			InvoiceNumber: targetInvoice.InvoiceNumber,
			Success:       false,
			Message:       "Contact does not have an email address",
		}, nil
	}

	// Get reminder count
	reminderCount, _, err := s.repo.GetReminderCount(ctx, schemaName, tenantID, req.InvoiceID)
	if err != nil {
		return nil, fmt.Errorf("get reminder count: %w", err)
	}

	// Create reminder record
	reminder := &PaymentReminder{
		ID:             uuid.New().String(),
		TenantID:       tenantID,
		InvoiceID:      req.InvoiceID,
		InvoiceNumber:  targetInvoice.InvoiceNumber,
		ContactID:      targetInvoice.ContactID,
		ContactName:    targetInvoice.ContactName,
		ContactEmail:   targetInvoice.ContactEmail,
		ReminderNumber: reminderCount + 1,
		Status:         ReminderStatusPending,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.repo.CreateReminder(ctx, schemaName, reminder); err != nil {
		return nil, fmt.Errorf("create reminder: %w", err)
	}

	// Get email template
	template, err := s.emailService.GetTemplate(ctx, schemaName, tenantID, email.TemplateOverdueReminder)
	if err != nil {
		_ = s.repo.UpdateReminderStatus(ctx, schemaName, reminder.ID, ReminderStatusFailed, nil, err.Error())
		return &ReminderResult{
			InvoiceID:     req.InvoiceID,
			InvoiceNumber: targetInvoice.InvoiceNumber,
			Success:       false,
			Message:       fmt.Sprintf("Failed to get email template: %v", err),
		}, nil
	}

	// Prepare template data
	data := &email.TemplateData{
		CompanyName:   companyName,
		ContactName:   targetInvoice.ContactName,
		InvoiceNumber: targetInvoice.InvoiceNumber,
		TotalAmount:   targetInvoice.OutstandingAmount.String(),
		Currency:      targetInvoice.Currency,
		DueDate:       targetInvoice.DueDate,
		DaysOverdue:   targetInvoice.DaysOverdue,
		Message:       req.Message,
	}

	// Render template
	subject, bodyHTML, bodyText, err := s.emailService.RenderTemplate(template, data)
	if err != nil {
		_ = s.repo.UpdateReminderStatus(ctx, schemaName, reminder.ID, ReminderStatusFailed, nil, err.Error())
		return &ReminderResult{
			InvoiceID:     req.InvoiceID,
			InvoiceNumber: targetInvoice.InvoiceNumber,
			Success:       false,
			Message:       fmt.Sprintf("Failed to render email template: %v", err),
		}, nil
	}

	// Send email
	_, err = s.emailService.SendEmail(
		ctx,
		schemaName,
		tenantID,
		string(email.TemplateOverdueReminder),
		targetInvoice.ContactEmail,
		targetInvoice.ContactName,
		subject,
		bodyHTML,
		bodyText,
		nil,
		req.InvoiceID,
	)

	if err != nil {
		_ = s.repo.UpdateReminderStatus(ctx, schemaName, reminder.ID, ReminderStatusFailed, nil, err.Error())
		return &ReminderResult{
			InvoiceID:     req.InvoiceID,
			InvoiceNumber: targetInvoice.InvoiceNumber,
			Success:       false,
			Message:       fmt.Sprintf("Failed to send email: %v", err),
		}, nil
	}

	// Update reminder as sent
	now := time.Now()
	_ = s.repo.UpdateReminderStatus(ctx, schemaName, reminder.ID, ReminderStatusSent, &now, "")

	return &ReminderResult{
		InvoiceID:     req.InvoiceID,
		InvoiceNumber: targetInvoice.InvoiceNumber,
		Success:       true,
		Message:       fmt.Sprintf("Reminder #%d sent successfully", reminder.ReminderNumber),
		ReminderID:    reminder.ID,
	}, nil
}

// SendBulkReminders sends payment reminders for multiple invoices
func (s *ReminderService) SendBulkReminders(ctx context.Context, tenantID, schemaName string, req *SendBulkRemindersRequest, companyName string) (*BulkReminderResult, error) {
	result := &BulkReminderResult{
		TotalRequested: len(req.InvoiceIDs),
		Results:        make([]ReminderResult, 0, len(req.InvoiceIDs)),
	}

	for _, invoiceID := range req.InvoiceIDs {
		singleReq := &SendReminderRequest{
			InvoiceID: invoiceID,
			Message:   req.Message,
		}

		singleResult, err := s.SendReminder(ctx, tenantID, schemaName, singleReq, companyName)
		if err != nil {
			result.Results = append(result.Results, ReminderResult{
				InvoiceID: invoiceID,
				Success:   false,
				Message:   err.Error(),
			})
			result.Failed++
			continue
		}

		result.Results = append(result.Results, *singleResult)
		if singleResult.Success {
			result.Successful++
		} else {
			result.Failed++
		}
	}

	return result, nil
}

// GetReminderHistory gets the reminder history for an invoice
func (s *ReminderService) GetReminderHistory(ctx context.Context, tenantID, schemaName, invoiceID string) ([]PaymentReminder, error) {
	return s.repo.GetRemindersByInvoice(ctx, schemaName, tenantID, invoiceID)
}
