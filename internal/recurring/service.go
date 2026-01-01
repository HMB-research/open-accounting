package recurring

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/email"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/pdf"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

// InvoicingService defines the interface for invoice operations needed by recurring
type InvoicingService interface {
	GetByID(ctx context.Context, tenantID, schemaName, id string) (*invoicing.Invoice, error)
	Create(ctx context.Context, tenantID, schemaName string, req *invoicing.CreateInvoiceRequest) (*invoicing.Invoice, error)
}

// EmailService defines the interface for email operations needed by recurring
type EmailService interface {
	GetTemplate(ctx context.Context, schemaName, tenantID string, templateType email.TemplateType) (*email.EmailTemplate, error)
	RenderTemplate(tmpl *email.EmailTemplate, data *email.TemplateData) (subject, bodyHTML, bodyText string, err error)
	SendEmail(ctx context.Context, schemaName, tenantID, templateType, toEmail, toName, subject, bodyHTML, bodyText string, attachments []email.Attachment, relatedEntityID string) (*email.EmailSentResponse, error)
}

// TenantService defines the interface for tenant operations needed by recurring
type TenantService interface {
	GetTenant(ctx context.Context, tenantID string) (*tenant.Tenant, error)
}

// ContactsService defines the interface for contact operations needed by recurring
type ContactsService interface {
	GetByID(ctx context.Context, tenantID, schemaName, contactID string) (*contacts.Contact, error)
}

// PDFService defines the interface for PDF operations needed by recurring
type PDFService interface {
	PDFSettingsFromTenant(t *tenant.Tenant) pdf.PDFSettings
	GenerateInvoicePDF(invoice *invoicing.Invoice, t *tenant.Tenant, settings pdf.PDFSettings) ([]byte, error)
}

// Service provides recurring invoice operations
type Service struct {
	db        *pgxpool.Pool
	repo      Repository
	invoicing InvoicingService
	email     EmailService
	pdfSvc    PDFService
	tenant    TenantService
	contacts  ContactsService
}

// NewService creates a new recurring invoice service
func NewService(db *pgxpool.Pool, invoicingService *invoicing.Service, emailService *email.Service, pdfService *pdf.Service, tenantService *tenant.Service, contactsService *contacts.Service) *Service {
	return &Service{
		db:        db,
		repo:      NewPostgresRepository(db),
		invoicing: invoicingService,
		email:     emailService,
		pdfSvc:    pdfService,
		tenant:    tenantService,
		contacts:  contactsService,
	}
}

// NewServiceWithDependencies creates a new recurring invoice service with injected dependencies
func NewServiceWithDependencies(repo Repository, invoicing InvoicingService, emailSvc EmailService, pdfSvc PDFService, tenantSvc TenantService, contactsSvc ContactsService) *Service {
	return &Service{
		repo:      repo,
		invoicing: invoicing,
		email:     emailSvc,
		pdfSvc:    pdfSvc,
		tenant:    tenantSvc,
		contacts:  contactsSvc,
	}
}

// EnsureSchema ensures the recurring invoice tables exist in the tenant schema
func (s *Service) EnsureSchema(ctx context.Context, schemaName string) error {
	if s.repo == nil {
		return fmt.Errorf("repository not available")
	}
	if err := s.repo.EnsureSchema(ctx, schemaName); err != nil {
		return fmt.Errorf("ensure recurring schema: %w", err)
	}
	return nil
}

// Create creates a new recurring invoice
func (s *Service) Create(ctx context.Context, tenantID, schemaName string, req *CreateRecurringInvoiceRequest) (*RecurringInvoice, error) {
	if err := s.EnsureSchema(ctx, schemaName); err != nil {
		return nil, err
	}

	// Default AttachPDFToEmail to true if not specified
	attachPDF := true
	if req.AttachPDFToEmail != nil {
		attachPDF = *req.AttachPDFToEmail
	}

	// Default email template type
	emailTemplateType := req.EmailTemplateType
	if emailTemplateType == "" {
		emailTemplateType = "INVOICE_SEND"
	}

	ri := &RecurringInvoice{
		ID:                 uuid.New().String(),
		TenantID:           tenantID,
		Name:               req.Name,
		ContactID:          req.ContactID,
		InvoiceType:        req.InvoiceType,
		Currency:           req.Currency,
		Frequency:          req.Frequency,
		StartDate:          req.StartDate,
		EndDate:            req.EndDate,
		NextGenerationDate: req.StartDate,
		PaymentTermsDays:   req.PaymentTermsDays,
		Reference:          req.Reference,
		Notes:              req.Notes,
		IsActive:           true,
		GeneratedCount:     0,
		CreatedAt:          time.Now(),
		CreatedBy:          req.UserID,
		UpdatedAt:          time.Now(),

		// Email configuration
		SendEmailOnGeneration:  req.SendEmailOnGeneration,
		EmailTemplateType:      emailTemplateType,
		RecipientEmailOverride: req.RecipientEmailOverride,
		AttachPDFToEmail:       attachPDF,
		EmailSubjectOverride:   req.EmailSubjectOverride,
		EmailMessage:           req.EmailMessage,
	}

	if ri.Currency == "" {
		ri.Currency = "EUR"
	}
	if ri.InvoiceType == "" {
		ri.InvoiceType = "SALES"
	}
	if ri.PaymentTermsDays == 0 {
		ri.PaymentTermsDays = 14
	}

	// Convert lines
	for i, reqLine := range req.Lines {
		line := RecurringInvoiceLine{
			ID:                 uuid.New().String(),
			RecurringInvoiceID: ri.ID,
			LineNumber:         i + 1,
			Description:        reqLine.Description,
			Quantity:           reqLine.Quantity,
			Unit:               reqLine.Unit,
			UnitPrice:          reqLine.UnitPrice,
			DiscountPercent:    reqLine.DiscountPercent,
			VATRate:            reqLine.VATRate,
			AccountID:          reqLine.AccountID,
			ProductID:          reqLine.ProductID,
		}
		if line.Quantity.IsZero() {
			line.Quantity = decimal.NewFromInt(1)
		}
		ri.Lines = append(ri.Lines, line)
	}

	if err := ri.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Use repository for creation
	if err := s.repo.Create(ctx, schemaName, ri); err != nil {
		return nil, fmt.Errorf("create recurring invoice: %w", err)
	}

	// Create lines via repository
	for _, line := range ri.Lines {
		lineCopy := line
		if err := s.repo.CreateLine(ctx, schemaName, &lineCopy); err != nil {
			return nil, fmt.Errorf("create recurring invoice line: %w", err)
		}
	}

	return ri, nil
}

// CreateFromInvoice creates a recurring invoice from an existing invoice
func (s *Service) CreateFromInvoice(ctx context.Context, tenantID, schemaName string, req *CreateFromInvoiceRequest) (*RecurringInvoice, error) {
	invoice, err := s.invoicing.GetByID(ctx, tenantID, schemaName, req.InvoiceID)
	if err != nil {
		return nil, fmt.Errorf("get invoice: %w", err)
	}

	// Convert invoice lines to recurring invoice lines
	lines := make([]CreateRecurringInvoiceLineRequest, 0, len(invoice.Lines))
	for _, invLine := range invoice.Lines {
		lines = append(lines, CreateRecurringInvoiceLineRequest{
			Description:     invLine.Description,
			Quantity:        invLine.Quantity,
			Unit:            invLine.Unit,
			UnitPrice:       invLine.UnitPrice,
			DiscountPercent: invLine.DiscountPercent,
			VATRate:         invLine.VATRate,
			AccountID:       invLine.AccountID,
			ProductID:       invLine.ProductID,
		})
	}

	createReq := &CreateRecurringInvoiceRequest{
		Name:             req.Name,
		ContactID:        invoice.ContactID,
		InvoiceType:      string(invoice.InvoiceType),
		Currency:         invoice.Currency,
		Frequency:        req.Frequency,
		StartDate:        req.StartDate,
		EndDate:          req.EndDate,
		PaymentTermsDays: req.PaymentTermsDays,
		Reference:        invoice.Reference,
		Notes:            invoice.Notes,
		Lines:            lines,
		UserID:           req.UserID,
	}

	return s.Create(ctx, tenantID, schemaName, createReq)
}

// GetByID retrieves a recurring invoice by ID
func (s *Service) GetByID(ctx context.Context, tenantID, schemaName, id string) (*RecurringInvoice, error) {
	if err := s.EnsureSchema(ctx, schemaName); err != nil {
		return nil, err
	}

	ri, err := s.repo.GetByID(ctx, schemaName, tenantID, id)
	if err == ErrRecurringInvoiceNotFound {
		return nil, fmt.Errorf("recurring invoice not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get recurring invoice: %w", err)
	}

	// Load lines
	lines, err := s.repo.GetLines(ctx, schemaName, id)
	if err != nil {
		return nil, fmt.Errorf("get recurring invoice lines: %w", err)
	}
	ri.Lines = lines

	return ri, nil
}

// List retrieves all recurring invoices for a tenant
func (s *Service) List(ctx context.Context, tenantID, schemaName string, activeOnly bool) ([]RecurringInvoice, error) {
	if err := s.EnsureSchema(ctx, schemaName); err != nil {
		return nil, err
	}

	results, err := s.repo.List(ctx, schemaName, tenantID, activeOnly)
	if err != nil {
		return nil, fmt.Errorf("list recurring invoices: %w", err)
	}

	return results, nil
}

// Update updates a recurring invoice
func (s *Service) Update(ctx context.Context, tenantID, schemaName, id string, req *UpdateRecurringInvoiceRequest) (*RecurringInvoice, error) {
	ri, err := s.GetByID(ctx, tenantID, schemaName, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		ri.Name = *req.Name
	}
	if req.ContactID != nil {
		ri.ContactID = *req.ContactID
	}
	if req.Frequency != nil {
		ri.Frequency = *req.Frequency
	}
	if req.EndDate != nil {
		ri.EndDate = req.EndDate
	}
	if req.PaymentTermsDays != nil {
		ri.PaymentTermsDays = *req.PaymentTermsDays
	}
	if req.Reference != nil {
		ri.Reference = *req.Reference
	}
	if req.Notes != nil {
		ri.Notes = *req.Notes
	}
	// Email configuration updates
	if req.SendEmailOnGeneration != nil {
		ri.SendEmailOnGeneration = *req.SendEmailOnGeneration
	}
	if req.EmailTemplateType != nil {
		ri.EmailTemplateType = *req.EmailTemplateType
	}
	if req.RecipientEmailOverride != nil {
		ri.RecipientEmailOverride = *req.RecipientEmailOverride
	}
	if req.AttachPDFToEmail != nil {
		ri.AttachPDFToEmail = *req.AttachPDFToEmail
	}
	if req.EmailSubjectOverride != nil {
		ri.EmailSubjectOverride = *req.EmailSubjectOverride
	}
	if req.EmailMessage != nil {
		ri.EmailMessage = *req.EmailMessage
	}
	ri.UpdatedAt = time.Now()

	if err := ri.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Update via repository
	if err := s.repo.Update(ctx, schemaName, ri); err != nil {
		return nil, fmt.Errorf("update recurring invoice: %w", err)
	}

	// Update lines if provided
	if len(req.Lines) > 0 {
		// Delete existing lines
		if err := s.repo.DeleteLines(ctx, schemaName, id); err != nil {
			return nil, fmt.Errorf("delete recurring invoice lines: %w", err)
		}

		// Insert new lines
		ri.Lines = nil
		for i, reqLine := range req.Lines {
			line := RecurringInvoiceLine{
				ID:                 uuid.New().String(),
				RecurringInvoiceID: ri.ID,
				LineNumber:         i + 1,
				Description:        reqLine.Description,
				Quantity:           reqLine.Quantity,
				Unit:               reqLine.Unit,
				UnitPrice:          reqLine.UnitPrice,
				DiscountPercent:    reqLine.DiscountPercent,
				VATRate:            reqLine.VATRate,
				AccountID:          reqLine.AccountID,
				ProductID:          reqLine.ProductID,
			}
			if line.Quantity.IsZero() {
				line.Quantity = decimal.NewFromInt(1)
			}

			if err := s.repo.CreateLine(ctx, schemaName, &line); err != nil {
				return nil, fmt.Errorf("create recurring invoice line: %w", err)
			}
			ri.Lines = append(ri.Lines, line)
		}
	}

	return ri, nil
}

// Delete deletes a recurring invoice
func (s *Service) Delete(ctx context.Context, tenantID, schemaName, id string) error {
	if err := s.EnsureSchema(ctx, schemaName); err != nil {
		return err
	}

	if err := s.repo.Delete(ctx, schemaName, tenantID, id); err != nil {
		if err == ErrRecurringInvoiceNotFound {
			return fmt.Errorf("recurring invoice not found: %s", id)
		}
		return fmt.Errorf("delete recurring invoice: %w", err)
	}
	return nil
}

// Pause pauses a recurring invoice
func (s *Service) Pause(ctx context.Context, tenantID, schemaName, id string) error {
	return s.setActive(ctx, tenantID, schemaName, id, false)
}

// Resume resumes a paused recurring invoice
func (s *Service) Resume(ctx context.Context, tenantID, schemaName, id string) error {
	return s.setActive(ctx, tenantID, schemaName, id, true)
}

func (s *Service) setActive(ctx context.Context, tenantID, schemaName, id string, active bool) error {
	if err := s.repo.SetActive(ctx, schemaName, tenantID, id, active); err != nil {
		if err == ErrRecurringInvoiceNotFound {
			return fmt.Errorf("recurring invoice not found: %s", id)
		}
		return fmt.Errorf("update recurring invoice: %w", err)
	}
	return nil
}

// GenerateDueInvoices generates invoices for all due recurring invoices
func (s *Service) GenerateDueInvoices(ctx context.Context, tenantID, schemaName, userID string) ([]GenerationResult, error) {
	if err := s.EnsureSchema(ctx, schemaName); err != nil {
		return nil, err
	}

	// Find all due recurring invoices via repository
	ids, err := s.repo.GetDueRecurringInvoiceIDs(ctx, schemaName, tenantID, time.Now())
	if err != nil {
		return nil, fmt.Errorf("list due recurring invoices: %w", err)
	}

	results := make([]GenerationResult, 0, len(ids))
	for _, id := range ids {
		result, err := s.GenerateInvoice(ctx, tenantID, schemaName, id, userID)
		if err != nil {
			// Log error but continue with other recurring invoices
			continue
		}
		results = append(results, *result)
	}

	return results, nil
}

// GenerateInvoice generates a single invoice from a recurring invoice
func (s *Service) GenerateInvoice(ctx context.Context, tenantID, schemaName, recurringID, userID string) (*GenerationResult, error) {
	ri, err := s.GetByID(ctx, tenantID, schemaName, recurringID)
	if err != nil {
		return nil, err
	}

	if !ri.IsActive {
		return nil, fmt.Errorf("recurring invoice is not active")
	}

	// Create invoice request from recurring invoice
	issueDate := time.Now()
	dueDate := issueDate.AddDate(0, 0, ri.PaymentTermsDays)

	lines := make([]invoicing.CreateInvoiceLineRequest, 0, len(ri.Lines))
	for _, riLine := range ri.Lines {
		lines = append(lines, invoicing.CreateInvoiceLineRequest{
			Description:     riLine.Description,
			Quantity:        riLine.Quantity,
			Unit:            riLine.Unit,
			UnitPrice:       riLine.UnitPrice,
			DiscountPercent: riLine.DiscountPercent,
			VATRate:         riLine.VATRate,
			AccountID:       riLine.AccountID,
			ProductID:       riLine.ProductID,
		})
	}

	invoiceReq := &invoicing.CreateInvoiceRequest{
		InvoiceType:  invoicing.InvoiceType(ri.InvoiceType),
		ContactID:    ri.ContactID,
		IssueDate:    issueDate,
		DueDate:      dueDate,
		Currency:     ri.Currency,
		ExchangeRate: decimal.NewFromInt(1),
		Reference:    ri.Reference,
		Notes:        ri.Notes,
		Lines:        lines,
		UserID:       userID,
	}

	// Create the invoice
	invoice, err := s.invoicing.Create(ctx, tenantID, schemaName, invoiceReq)
	if err != nil {
		return nil, fmt.Errorf("create invoice: %w", err)
	}

	// Update recurring invoice via repository
	nextDate := ri.CalculateNextDate(ri.NextGenerationDate)
	now := time.Now()
	if err := s.repo.UpdateAfterGeneration(ctx, schemaName, tenantID, recurringID, nextDate, now); err != nil {
		return nil, fmt.Errorf("update recurring invoice: %w", err)
	}

	// Prepare result
	result := &GenerationResult{
		RecurringInvoiceID:     recurringID,
		GeneratedInvoiceID:     invoice.ID,
		GeneratedInvoiceNumber: invoice.InvoiceNumber,
		EmailSent:              false,
		EmailStatus:            "SKIPPED",
	}

	// Send email if configured
	if ri.SendEmailOnGeneration {
		emailResult := s.sendGeneratedInvoiceEmail(ctx, tenantID, schemaName, ri, invoice)
		result.EmailSent = emailResult.EmailSent
		result.EmailStatus = emailResult.EmailStatus
		result.EmailLogID = emailResult.EmailLogID
		result.EmailError = emailResult.EmailError

		// Update invoice with email status
		s.updateInvoiceEmailStatus(ctx, schemaName, invoice.ID, emailResult)
	}

	return result, nil
}

// sendGeneratedInvoiceEmail sends email for a generated invoice
func (s *Service) sendGeneratedInvoiceEmail(ctx context.Context, tenantID, schemaName string, ri *RecurringInvoice, invoice *invoicing.Invoice) *GenerationResult {
	result := &GenerationResult{
		EmailSent:   false,
		EmailStatus: "FAILED",
	}

	// Check if email service is available
	if s.email == nil {
		result.EmailStatus = "NO_CONFIG"
		result.EmailError = "email service not configured"
		return result
	}

	// Get tenant for company name and PDF settings
	t, err := s.tenant.GetTenant(ctx, tenantID)
	if err != nil {
		result.EmailError = fmt.Sprintf("failed to get tenant: %v", err)
		return result
	}

	// Get contact for email address
	contact, err := s.contacts.GetByID(ctx, tenantID, schemaName, ri.ContactID)
	if err != nil {
		result.EmailError = fmt.Sprintf("failed to get contact: %v", err)
		return result
	}

	// Determine recipient email
	recipientEmail := ri.RecipientEmailOverride
	if recipientEmail == "" {
		recipientEmail = contact.Email
	}
	if recipientEmail == "" {
		result.EmailStatus = "SKIPPED"
		result.EmailError = "no recipient email available"
		return result
	}

	// Get email template
	templateType := email.TemplateType(ri.EmailTemplateType)
	if templateType == "" {
		templateType = email.TemplateInvoiceSend
	}
	tmpl, err := s.email.GetTemplate(ctx, schemaName, tenantID, templateType)
	if err != nil {
		result.EmailError = fmt.Sprintf("failed to get template: %v", err)
		return result
	}

	// Prepare template data
	templateData := &email.TemplateData{
		CompanyName:   t.Name,
		ContactName:   contact.Name,
		InvoiceNumber: invoice.InvoiceNumber,
		TotalAmount:   invoice.Total.StringFixed(2),
		Currency:      invoice.Currency,
		DueDate:       invoice.DueDate.Format("02.01.2006"),
		IssueDate:     invoice.IssueDate.Format("02.01.2006"),
		Message:       ri.EmailMessage,
	}

	// Render template
	subject, bodyHTML, bodyText, err := s.email.RenderTemplate(tmpl, templateData)
	if err != nil {
		result.EmailError = fmt.Sprintf("failed to render template: %v", err)
		return result
	}

	// Override subject if specified
	if ri.EmailSubjectOverride != "" {
		subject = ri.EmailSubjectOverride
	}

	// Prepare attachments
	var attachments []email.Attachment
	if ri.AttachPDFToEmail && s.pdfSvc != nil {
		pdfSettings := s.pdfSvc.PDFSettingsFromTenant(t)
		pdfBytes, err := s.pdfSvc.GenerateInvoicePDF(invoice, t, pdfSettings)
		if err != nil {
			// Log PDF error but continue without attachment
			result.EmailError = fmt.Sprintf("PDF generation failed: %v", err)
		} else {
			attachments = append(attachments, email.Attachment{
				Filename:    fmt.Sprintf("invoice_%s.pdf", invoice.InvoiceNumber),
				Content:     pdfBytes,
				ContentType: "application/pdf",
			})
		}
	}

	// Send email
	emailResp, err := s.email.SendEmail(
		ctx,
		schemaName,
		tenantID,
		string(templateType),
		recipientEmail,
		contact.Name,
		subject,
		bodyHTML,
		bodyText,
		attachments,
		invoice.ID,
	)
	if err != nil {
		result.EmailError = fmt.Sprintf("failed to send email: %v", err)
		return result
	}

	result.EmailSent = true
	result.EmailStatus = "SENT"
	result.EmailLogID = emailResp.LogID
	result.EmailError = ""
	return result
}

// updateInvoiceEmailStatus updates the invoice with email delivery status
func (s *Service) updateInvoiceEmailStatus(ctx context.Context, schemaName, invoiceID string, emailResult *GenerationResult) {
	var sentAt *time.Time
	if emailResult.EmailSent {
		now := time.Now()
		sentAt = &now
	}

	err := s.repo.UpdateInvoiceEmailStatus(ctx, schemaName, invoiceID, sentAt, emailResult.EmailStatus, emailResult.EmailLogID)
	if err != nil {
		// Log error but don't fail - invoice was already created
		fmt.Printf("failed to update invoice email status: %v\n", err)
	}
}
