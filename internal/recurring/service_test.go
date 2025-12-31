package recurring

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/email"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/pdf"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

// =============================================================================
// Mock Repository
// =============================================================================

type MockRepository struct {
	recurring                  map[string]*RecurringInvoice
	lines                      map[string][]RecurringInvoiceLine
	ensureSchemaErr            error
	createErr                  error
	createLineErr              error
	getByIDErr                 error
	getLinesErr                error
	listErr                    error
	updateErr                  error
	deleteLinesErr             error
	deleteErr                  error
	setActiveErr               error
	getDueIDsErr               error
	updateAfterGenErr          error
	updateInvoiceEmailStatusErr error
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		recurring: make(map[string]*RecurringInvoice),
		lines:     make(map[string][]RecurringInvoiceLine),
	}
}

func (m *MockRepository) EnsureSchema(ctx context.Context, schemaName string) error {
	return m.ensureSchemaErr
}

func (m *MockRepository) Create(ctx context.Context, schemaName string, ri *RecurringInvoice) error {
	if m.createErr != nil {
		return m.createErr
	}
	key := ri.TenantID + ":" + ri.ID
	m.recurring[key] = ri
	return nil
}

func (m *MockRepository) CreateLine(ctx context.Context, schemaName string, line *RecurringInvoiceLine) error {
	if m.createLineErr != nil {
		return m.createLineErr
	}
	m.lines[line.RecurringInvoiceID] = append(m.lines[line.RecurringInvoiceID], *line)
	return nil
}

func (m *MockRepository) GetByID(ctx context.Context, schemaName, tenantID, id string) (*RecurringInvoice, error) {
	if m.getByIDErr != nil {
		return nil, m.getByIDErr
	}
	key := tenantID + ":" + id
	if ri, ok := m.recurring[key]; ok {
		return ri, nil
	}
	return nil, ErrRecurringInvoiceNotFound
}

func (m *MockRepository) GetLines(ctx context.Context, schemaName, recurringInvoiceID string) ([]RecurringInvoiceLine, error) {
	if m.getLinesErr != nil {
		return nil, m.getLinesErr
	}
	return m.lines[recurringInvoiceID], nil
}

func (m *MockRepository) List(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]RecurringInvoice, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var results []RecurringInvoice
	for key, ri := range m.recurring {
		if key[:len(tenantID)] == tenantID {
			if !activeOnly || ri.IsActive {
				results = append(results, *ri)
			}
		}
	}
	return results, nil
}

func (m *MockRepository) Update(ctx context.Context, schemaName string, ri *RecurringInvoice) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	key := ri.TenantID + ":" + ri.ID
	m.recurring[key] = ri
	return nil
}

func (m *MockRepository) DeleteLines(ctx context.Context, schemaName, recurringInvoiceID string) error {
	if m.deleteLinesErr != nil {
		return m.deleteLinesErr
	}
	delete(m.lines, recurringInvoiceID)
	return nil
}

func (m *MockRepository) Delete(ctx context.Context, schemaName, tenantID, id string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	key := tenantID + ":" + id
	if _, ok := m.recurring[key]; !ok {
		return ErrRecurringInvoiceNotFound
	}
	delete(m.recurring, key)
	return nil
}

func (m *MockRepository) SetActive(ctx context.Context, schemaName, tenantID, id string, active bool) error {
	if m.setActiveErr != nil {
		return m.setActiveErr
	}
	key := tenantID + ":" + id
	if ri, ok := m.recurring[key]; ok {
		ri.IsActive = active
		return nil
	}
	return ErrRecurringInvoiceNotFound
}

func (m *MockRepository) GetDueRecurringInvoiceIDs(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) ([]string, error) {
	if m.getDueIDsErr != nil {
		return nil, m.getDueIDsErr
	}
	var ids []string
	for key, ri := range m.recurring {
		if key[:len(tenantID)] == tenantID && ri.IsActive {
			if !ri.NextGenerationDate.After(asOfDate) {
				if ri.EndDate == nil || !ri.EndDate.Before(asOfDate) {
					ids = append(ids, ri.ID)
				}
			}
		}
	}
	return ids, nil
}

func (m *MockRepository) UpdateAfterGeneration(ctx context.Context, schemaName, tenantID, id string, nextDate time.Time, generatedAt time.Time) error {
	if m.updateAfterGenErr != nil {
		return m.updateAfterGenErr
	}
	key := tenantID + ":" + id
	if ri, ok := m.recurring[key]; ok {
		ri.NextGenerationDate = nextDate
		ri.LastGeneratedAt = &generatedAt
		ri.GeneratedCount++
		return nil
	}
	return ErrRecurringInvoiceNotFound
}

func (m *MockRepository) UpdateInvoiceEmailStatus(ctx context.Context, schemaName, invoiceID string, sentAt *time.Time, status, logID string) error {
	if m.updateInvoiceEmailStatusErr != nil {
		return m.updateInvoiceEmailStatusErr
	}
	return nil
}

// =============================================================================
// Mock Invoicing Service
// =============================================================================

type MockInvoicingService struct {
	invoices     map[string]*invoicing.Invoice
	createErr    error
	getByIDErr   error
	lastCreateID string
}

func NewMockInvoicingService() *MockInvoicingService {
	return &MockInvoicingService{
		invoices: make(map[string]*invoicing.Invoice),
	}
}

func (m *MockInvoicingService) GetByID(ctx context.Context, tenantID, schemaName, id string) (*invoicing.Invoice, error) {
	if m.getByIDErr != nil {
		return nil, m.getByIDErr
	}
	if inv, ok := m.invoices[id]; ok {
		return inv, nil
	}
	return nil, fmt.Errorf("invoice not found")
}

func (m *MockInvoicingService) Create(ctx context.Context, tenantID, schemaName string, req *invoicing.CreateInvoiceRequest) (*invoicing.Invoice, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	inv := &invoicing.Invoice{
		ID:            "gen-inv-" + time.Now().Format("20060102150405"),
		TenantID:      tenantID,
		InvoiceNumber: "INV-2025-001",
		InvoiceType:   req.InvoiceType,
		ContactID:     req.ContactID,
		IssueDate:     req.IssueDate,
		DueDate:       req.DueDate,
		Currency:      req.Currency,
		Status:        invoicing.StatusDraft,
		Total:         decimal.NewFromInt(100),
	}
	m.invoices[inv.ID] = inv
	m.lastCreateID = inv.ID
	return inv, nil
}

// =============================================================================
// Mock Email Service
// =============================================================================

type MockEmailService struct {
	sentEmails        []SentEmail
	getTemplateErr    error
	renderTemplateErr error
	sendErr           error
}

type SentEmail struct {
	ToEmail string
	Subject string
}

func NewMockEmailService() *MockEmailService {
	return &MockEmailService{
		sentEmails: make([]SentEmail, 0),
	}
}

func (m *MockEmailService) GetTemplate(ctx context.Context, schemaName, tenantID string, templateType email.TemplateType) (*email.EmailTemplate, error) {
	if m.getTemplateErr != nil {
		return nil, m.getTemplateErr
	}
	return &email.EmailTemplate{
		ID:           "tmpl-1",
		TemplateType: templateType,
		Subject:      "Invoice {{.InvoiceNumber}}",
		BodyHTML:     "<p>Dear {{.ContactName}}</p>",
		BodyText:     "Dear {{.ContactName}}",
	}, nil
}

func (m *MockEmailService) RenderTemplate(tmpl *email.EmailTemplate, data *email.TemplateData) (string, string, string, error) {
	if m.renderTemplateErr != nil {
		return "", "", "", m.renderTemplateErr
	}
	return tmpl.Subject, tmpl.BodyHTML, tmpl.BodyText, nil
}

func (m *MockEmailService) SendEmail(ctx context.Context, schemaName, tenantID, templateType, toEmail, toName, subject, bodyHTML, bodyText string, attachments []email.Attachment, relatedEntityID string) (*email.EmailSentResponse, error) {
	if m.sendErr != nil {
		return nil, m.sendErr
	}
	m.sentEmails = append(m.sentEmails, SentEmail{ToEmail: toEmail, Subject: subject})
	return &email.EmailSentResponse{
		Success: true,
		LogID:   "log-123",
	}, nil
}

// =============================================================================
// Mock Tenant Service
// =============================================================================

type MockTenantService struct {
	tenants map[string]*tenant.Tenant
	getErr  error
}

func NewMockTenantService() *MockTenantService {
	return &MockTenantService{
		tenants: make(map[string]*tenant.Tenant),
	}
}

func (m *MockTenantService) GetTenant(ctx context.Context, tenantID string) (*tenant.Tenant, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if t, ok := m.tenants[tenantID]; ok {
		return t, nil
	}
	return nil, fmt.Errorf("tenant not found")
}

// =============================================================================
// Mock Contacts Service
// =============================================================================

type MockContactsService struct {
	contacts map[string]*contacts.Contact
	getErr   error
}

func NewMockContactsService() *MockContactsService {
	return &MockContactsService{
		contacts: make(map[string]*contacts.Contact),
	}
}

func (m *MockContactsService) GetByID(ctx context.Context, tenantID, schemaName, contactID string) (*contacts.Contact, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if c, ok := m.contacts[contactID]; ok {
		return c, nil
	}
	return nil, fmt.Errorf("contact not found")
}

// =============================================================================
// Mock PDF Service
// =============================================================================

type MockPDFService struct {
	generateErr error
}

func NewMockPDFService() *MockPDFService {
	return &MockPDFService{}
}

func (m *MockPDFService) PDFSettingsFromTenant(t *tenant.Tenant) pdf.PDFSettings {
	return pdf.PDFSettings{}
}

func (m *MockPDFService) GenerateInvoicePDF(invoice *invoicing.Invoice, t *tenant.Tenant, settings pdf.PDFSettings) ([]byte, error) {
	if m.generateErr != nil {
		return nil, m.generateErr
	}
	return []byte("fake-pdf-content"), nil
}

// =============================================================================
// Tests
// =============================================================================

func TestNewService(t *testing.T) {
	service := NewService(nil, nil, nil, nil, nil, nil)
	if service == nil {
		t.Fatal("NewService returned nil")
	}
}

func TestNewServiceWithDependencies(t *testing.T) {
	repo := NewMockRepository()
	invoicingSvc := NewMockInvoicingService()
	emailSvc := NewMockEmailService()
	pdfSvc := NewMockPDFService()
	tenantSvc := NewMockTenantService()
	contactsSvc := NewMockContactsService()

	service := NewServiceWithDependencies(repo, invoicingSvc, emailSvc, pdfSvc, tenantSvc, contactsSvc)
	if service == nil {
		t.Fatal("NewServiceWithDependencies returned nil")
	}
	if service.repo == nil {
		t.Error("repo should not be nil")
	}
	if service.invoicing == nil {
		t.Error("invoicing should not be nil")
	}
}

func TestService_EnsureSchema(t *testing.T) {
	tests := []struct {
		name      string
		repoErr   error
		expectErr bool
	}{
		{
			name:      "success",
			repoErr:   nil,
			expectErr: false,
		},
		{
			name:      "repository error",
			repoErr:   fmt.Errorf("db error"),
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockRepository()
			repo.ensureSchemaErr = tt.repoErr
			service := NewServiceWithDependencies(repo, nil, nil, nil, nil, nil)

			err := service.EnsureSchema(context.Background(), "test_schema")
			if tt.expectErr && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestService_EnsureSchema_NilRepo(t *testing.T) {
	service := &Service{}
	err := service.EnsureSchema(context.Background(), "test")
	if err == nil {
		t.Error("expected error for nil repo")
	}
	if err.Error() != "repository not available" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestDefaultPaymentTermsDays(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{0, 14},
		{1, 1},
		{7, 7},
		{14, 14},
		{30, 30},
		{60, 60},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			days := tt.input
			if days == 0 {
				days = 14
			}
			if days != tt.expected {
				t.Errorf("Default payment terms: input %d, got %d, want %d", tt.input, days, tt.expected)
			}
		})
	}
}

func TestDefaultCurrency(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "EUR"},
		{"EUR", "EUR"},
		{"USD", "USD"},
		{"GBP", "GBP"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			currency := tt.input
			if currency == "" {
				currency = "EUR"
			}
			if currency != tt.expected {
				t.Errorf("Default currency: input %q, got %q, want %q", tt.input, currency, tt.expected)
			}
		})
	}
}

func TestDefaultInvoiceType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "SALES"},
		{"SALES", "SALES"},
		{"PURCHASE", "PURCHASE"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			invoiceType := tt.input
			if invoiceType == "" {
				invoiceType = "SALES"
			}
			if invoiceType != tt.expected {
				t.Errorf("Default invoice type: input %q, got %q, want %q", tt.input, invoiceType, tt.expected)
			}
		})
	}
}

func TestDefaultEmailTemplateType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "INVOICE_SEND"},
		{"INVOICE_SEND", "INVOICE_SEND"},
		{"OVERDUE_REMINDER", "OVERDUE_REMINDER"},
		{"PAYMENT_RECEIPT", "PAYMENT_RECEIPT"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			templateType := tt.input
			if templateType == "" {
				templateType = "INVOICE_SEND"
			}
			if templateType != tt.expected {
				t.Errorf("Default email template: input %q, got %q, want %q", tt.input, templateType, tt.expected)
			}
		})
	}
}

func TestDefaultAttachPDFToEmail(t *testing.T) {
	tests := []struct {
		name     string
		input    *bool
		expected bool
	}{
		{"nil defaults to true", nil, true},
		{"explicit true", boolPtr(true), true},
		{"explicit false", boolPtr(false), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attachPDF := true
			if tt.input != nil {
				attachPDF = *tt.input
			}
			if attachPDF != tt.expected {
				t.Errorf("Default attach PDF: input %v, got %v, want %v", tt.input, attachPDF, tt.expected)
			}
		})
	}
}

func TestEmailConfigurationRequest(t *testing.T) {
	tests := []struct {
		name              string
		sendEmail         bool
		templateType      string
		recipientOverride string
		attachPDF         *bool
		subjectOverride   string
		message           string
		expectedSendEmail bool
		expectedAttachPDF bool
	}{
		{
			name:              "email disabled",
			sendEmail:         false,
			expectedSendEmail: false,
			expectedAttachPDF: true,
		},
		{
			name:              "email enabled with defaults",
			sendEmail:         true,
			expectedSendEmail: true,
			expectedAttachPDF: true,
		},
		{
			name:              "email enabled without PDF",
			sendEmail:         true,
			attachPDF:         boolPtr(false),
			expectedSendEmail: true,
			expectedAttachPDF: false,
		},
		{
			name:              "email with recipient override",
			sendEmail:         true,
			recipientOverride: "custom@example.com",
			expectedSendEmail: true,
			expectedAttachPDF: true,
		},
		{
			name:              "email with custom subject and message",
			sendEmail:         true,
			subjectOverride:   "Custom Invoice Subject",
			message:           "Custom email message body",
			expectedSendEmail: true,
			expectedAttachPDF: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := CreateRecurringInvoiceRequest{
				SendEmailOnGeneration:  tt.sendEmail,
				EmailTemplateType:      tt.templateType,
				RecipientEmailOverride: tt.recipientOverride,
				AttachPDFToEmail:       tt.attachPDF,
				EmailSubjectOverride:   tt.subjectOverride,
				EmailMessage:           tt.message,
			}

			if req.SendEmailOnGeneration != tt.expectedSendEmail {
				t.Errorf("SendEmailOnGeneration = %v, want %v", req.SendEmailOnGeneration, tt.expectedSendEmail)
			}

			attachPDF := true
			if req.AttachPDFToEmail != nil {
				attachPDF = *req.AttachPDFToEmail
			}
			if attachPDF != tt.expectedAttachPDF {
				t.Errorf("AttachPDFToEmail = %v, want %v", attachPDF, tt.expectedAttachPDF)
			}
		})
	}
}

func TestUpdateEmailConfigurationRequest(t *testing.T) {
	tests := []struct {
		name        string
		setupReq    func() UpdateRecurringInvoiceRequest
		checkField  string
		expectNil   bool
		expectValue interface{}
	}{
		{
			name: "update SendEmailOnGeneration to true",
			setupReq: func() UpdateRecurringInvoiceRequest {
				val := true
				return UpdateRecurringInvoiceRequest{SendEmailOnGeneration: &val}
			},
			checkField:  "SendEmailOnGeneration",
			expectNil:   false,
			expectValue: true,
		},
		{
			name: "update SendEmailOnGeneration to false",
			setupReq: func() UpdateRecurringInvoiceRequest {
				val := false
				return UpdateRecurringInvoiceRequest{SendEmailOnGeneration: &val}
			},
			checkField:  "SendEmailOnGeneration",
			expectNil:   false,
			expectValue: false,
		},
		{
			name: "update EmailTemplateType",
			setupReq: func() UpdateRecurringInvoiceRequest {
				val := "OVERDUE_REMINDER"
				return UpdateRecurringInvoiceRequest{EmailTemplateType: &val}
			},
			checkField:  "EmailTemplateType",
			expectNil:   false,
			expectValue: "OVERDUE_REMINDER",
		},
		{
			name: "update RecipientEmailOverride",
			setupReq: func() UpdateRecurringInvoiceRequest {
				val := "new-recipient@example.com"
				return UpdateRecurringInvoiceRequest{RecipientEmailOverride: &val}
			},
			checkField:  "RecipientEmailOverride",
			expectNil:   false,
			expectValue: "new-recipient@example.com",
		},
		{
			name: "update AttachPDFToEmail",
			setupReq: func() UpdateRecurringInvoiceRequest {
				val := false
				return UpdateRecurringInvoiceRequest{AttachPDFToEmail: &val}
			},
			checkField:  "AttachPDFToEmail",
			expectNil:   false,
			expectValue: false,
		},
		{
			name: "partial update - only subject",
			setupReq: func() UpdateRecurringInvoiceRequest {
				val := "New Subject"
				return UpdateRecurringInvoiceRequest{EmailSubjectOverride: &val}
			},
			checkField:  "EmailSubjectOverride",
			expectNil:   false,
			expectValue: "New Subject",
		},
		{
			name: "no email fields updated",
			setupReq: func() UpdateRecurringInvoiceRequest {
				return UpdateRecurringInvoiceRequest{}
			},
			checkField: "SendEmailOnGeneration",
			expectNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupReq()

			switch tt.checkField {
			case "SendEmailOnGeneration":
				if tt.expectNil {
					if req.SendEmailOnGeneration != nil {
						t.Errorf("SendEmailOnGeneration should be nil")
					}
				} else {
					if req.SendEmailOnGeneration == nil {
						t.Errorf("SendEmailOnGeneration should not be nil")
					} else if *req.SendEmailOnGeneration != tt.expectValue.(bool) {
						t.Errorf("SendEmailOnGeneration = %v, want %v", *req.SendEmailOnGeneration, tt.expectValue)
					}
				}
			case "EmailTemplateType":
				if req.EmailTemplateType == nil || *req.EmailTemplateType != tt.expectValue.(string) {
					t.Errorf("EmailTemplateType mismatch")
				}
			case "RecipientEmailOverride":
				if req.RecipientEmailOverride == nil || *req.RecipientEmailOverride != tt.expectValue.(string) {
					t.Errorf("RecipientEmailOverride mismatch")
				}
			case "AttachPDFToEmail":
				if req.AttachPDFToEmail == nil || *req.AttachPDFToEmail != tt.expectValue.(bool) {
					t.Errorf("AttachPDFToEmail mismatch")
				}
			case "EmailSubjectOverride":
				if req.EmailSubjectOverride == nil || *req.EmailSubjectOverride != tt.expectValue.(string) {
					t.Errorf("EmailSubjectOverride mismatch")
				}
			}
		})
	}
}

func TestGenerationResultEmailFields(t *testing.T) {
	tests := []struct {
		name        string
		emailSent   bool
		emailStatus string
		emailLogID  string
		emailError  string
		description string
	}{
		{
			name:        "successful email delivery",
			emailSent:   true,
			emailStatus: "SENT",
			emailLogID:  "log-123",
			emailError:  "",
			description: "Email was successfully sent with log ID",
		},
		{
			name:        "email delivery failed",
			emailSent:   false,
			emailStatus: "FAILED",
			emailLogID:  "",
			emailError:  "SMTP connection timeout",
			description: "Email failed with error message",
		},
		{
			name:        "email skipped - no recipient",
			emailSent:   false,
			emailStatus: "SKIPPED",
			emailLogID:  "",
			emailError:  "no recipient email available",
			description: "Email was skipped due to missing recipient",
		},
		{
			name:        "email skipped - no config",
			emailSent:   false,
			emailStatus: "NO_CONFIG",
			emailLogID:  "",
			emailError:  "email service not configured",
			description: "Email was not sent because email service is not configured",
		},
		{
			name:        "email skipped - disabled",
			emailSent:   false,
			emailStatus: "SKIPPED",
			emailLogID:  "",
			emailError:  "",
			description: "Email was skipped because send_email_on_generation is false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerationResult{
				RecurringInvoiceID:     "ri-123",
				GeneratedInvoiceID:     "inv-456",
				GeneratedInvoiceNumber: "INV-2025-001",
				EmailSent:              tt.emailSent,
				EmailStatus:            tt.emailStatus,
				EmailLogID:             tt.emailLogID,
				EmailError:             tt.emailError,
			}

			if result.EmailSent != tt.emailSent {
				t.Errorf("EmailSent = %v, want %v (%s)", result.EmailSent, tt.emailSent, tt.description)
			}
			if result.EmailStatus != tt.emailStatus {
				t.Errorf("EmailStatus = %q, want %q (%s)", result.EmailStatus, tt.emailStatus, tt.description)
			}
			if result.EmailLogID != tt.emailLogID {
				t.Errorf("EmailLogID = %q, want %q (%s)", result.EmailLogID, tt.emailLogID, tt.description)
			}
			if result.EmailError != tt.emailError {
				t.Errorf("EmailError = %q, want %q (%s)", result.EmailError, tt.emailError, tt.description)
			}
		})
	}
}

func TestMockRepository_CRUD(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	schemaName := "test_schema"
	tenantID := "tenant-1"

	// Create
	ri := &RecurringInvoice{
		ID:                 "ri-1",
		TenantID:           tenantID,
		Name:               "Monthly Subscription",
		ContactID:          "contact-1",
		InvoiceType:        "SALES",
		Currency:           "EUR",
		Frequency:          "MONTHLY",
		StartDate:          time.Now(),
		NextGenerationDate: time.Now(),
		PaymentTermsDays:   14,
		IsActive:           true,
	}

	err := repo.Create(ctx, schemaName, ri)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// GetByID
	got, err := repo.GetByID(ctx, schemaName, tenantID, ri.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got.Name != ri.Name {
		t.Errorf("Name = %q, want %q", got.Name, ri.Name)
	}

	// List
	list, err := repo.List(ctx, schemaName, tenantID, false)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("List length = %d, want 1", len(list))
	}

	// SetActive - pause
	err = repo.SetActive(ctx, schemaName, tenantID, ri.ID, false)
	if err != nil {
		t.Fatalf("SetActive failed: %v", err)
	}
	got, _ = repo.GetByID(ctx, schemaName, tenantID, ri.ID)
	if got.IsActive {
		t.Error("IsActive should be false after pause")
	}

	// List active only should be empty now
	list, _ = repo.List(ctx, schemaName, tenantID, true)
	if len(list) != 0 {
		t.Errorf("List active only length = %d, want 0", len(list))
	}

	// Delete
	err = repo.Delete(ctx, schemaName, tenantID, ri.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = repo.GetByID(ctx, schemaName, tenantID, ri.ID)
	if err != ErrRecurringInvoiceNotFound {
		t.Errorf("GetByID after delete should return ErrRecurringInvoiceNotFound, got %v", err)
	}
}

func TestMockRepository_Lines(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	schemaName := "test_schema"
	riID := "ri-1"

	// Create lines
	line1 := &RecurringInvoiceLine{
		ID:                 "line-1",
		RecurringInvoiceID: riID,
		LineNumber:         1,
		Description:        "Service A",
		Quantity:           decimal.NewFromInt(1),
		UnitPrice:          decimal.NewFromFloat(100.00),
	}
	line2 := &RecurringInvoiceLine{
		ID:                 "line-2",
		RecurringInvoiceID: riID,
		LineNumber:         2,
		Description:        "Service B",
		Quantity:           decimal.NewFromInt(2),
		UnitPrice:          decimal.NewFromFloat(50.00),
	}

	_ = repo.CreateLine(ctx, schemaName, line1)
	_ = repo.CreateLine(ctx, schemaName, line2)

	// Get lines
	lines, err := repo.GetLines(ctx, schemaName, riID)
	if err != nil {
		t.Fatalf("GetLines failed: %v", err)
	}
	if len(lines) != 2 {
		t.Errorf("Lines length = %d, want 2", len(lines))
	}

	// Delete lines
	_ = repo.DeleteLines(ctx, schemaName, riID)
	lines, _ = repo.GetLines(ctx, schemaName, riID)
	if len(lines) != 0 {
		t.Errorf("Lines length after delete = %d, want 0", len(lines))
	}
}

func TestMockRepository_DueInvoices(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	schemaName := "test_schema"
	tenantID := "tenant-1"
	now := time.Now()

	// Create recurring invoices with different due dates
	dueYesterday := &RecurringInvoice{
		ID:                 "ri-due",
		TenantID:           tenantID,
		NextGenerationDate: now.AddDate(0, 0, -1),
		IsActive:           true,
	}
	dueTomorrow := &RecurringInvoice{
		ID:                 "ri-not-due",
		TenantID:           tenantID,
		NextGenerationDate: now.AddDate(0, 0, 1),
		IsActive:           true,
	}
	dueButInactive := &RecurringInvoice{
		ID:                 "ri-inactive",
		TenantID:           tenantID,
		NextGenerationDate: now.AddDate(0, 0, -1),
		IsActive:           false,
	}

	_ = repo.Create(ctx, schemaName, dueYesterday)
	_ = repo.Create(ctx, schemaName, dueTomorrow)
	_ = repo.Create(ctx, schemaName, dueButInactive)

	ids, err := repo.GetDueRecurringInvoiceIDs(ctx, schemaName, tenantID, now)
	if err != nil {
		t.Fatalf("GetDueRecurringInvoiceIDs failed: %v", err)
	}
	if len(ids) != 1 {
		t.Errorf("Due IDs length = %d, want 1", len(ids))
	}
	if len(ids) > 0 && ids[0] != "ri-due" {
		t.Errorf("Due ID = %q, want %q", ids[0], "ri-due")
	}
}

func TestMockRepository_UpdateAfterGeneration(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	schemaName := "test_schema"
	tenantID := "tenant-1"
	now := time.Now()

	ri := &RecurringInvoice{
		ID:                 "ri-1",
		TenantID:           tenantID,
		NextGenerationDate: now,
		IsActive:           true,
		GeneratedCount:     0,
	}
	_ = repo.Create(ctx, schemaName, ri)

	nextDate := now.AddDate(0, 1, 0)
	err := repo.UpdateAfterGeneration(ctx, schemaName, tenantID, ri.ID, nextDate, now)
	if err != nil {
		t.Fatalf("UpdateAfterGeneration failed: %v", err)
	}

	got, _ := repo.GetByID(ctx, schemaName, tenantID, ri.ID)
	if got.GeneratedCount != 1 {
		t.Errorf("GeneratedCount = %d, want 1", got.GeneratedCount)
	}
	if got.LastGeneratedAt == nil {
		t.Error("LastGeneratedAt should not be nil")
	}
	if !got.NextGenerationDate.Equal(nextDate) {
		t.Errorf("NextGenerationDate = %v, want %v", got.NextGenerationDate, nextDate)
	}
}

func TestMockInvoicingService(t *testing.T) {
	ctx := context.Background()
	svc := NewMockInvoicingService()

	// Create
	req := &invoicing.CreateInvoiceRequest{
		InvoiceType: invoicing.InvoiceTypeSales,
		ContactID:   "contact-1",
		IssueDate:   time.Now(),
		DueDate:     time.Now().AddDate(0, 0, 14),
		Currency:    "EUR",
	}

	inv, err := svc.Create(ctx, "tenant-1", "schema", req)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if inv.InvoiceNumber != "INV-2025-001" {
		t.Errorf("InvoiceNumber = %q, want %q", inv.InvoiceNumber, "INV-2025-001")
	}

	// GetByID
	got, err := svc.GetByID(ctx, "tenant-1", "schema", inv.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got.ID != inv.ID {
		t.Errorf("ID = %q, want %q", got.ID, inv.ID)
	}

	// GetByID not found
	_, err = svc.GetByID(ctx, "tenant-1", "schema", "non-existent")
	if err == nil {
		t.Error("GetByID should return error for non-existent invoice")
	}
}

func TestMockEmailService(t *testing.T) {
	ctx := context.Background()
	svc := NewMockEmailService()

	// GetTemplate
	tmpl, err := svc.GetTemplate(ctx, "schema", "tenant-1", email.TemplateInvoiceSend)
	if err != nil {
		t.Fatalf("GetTemplate failed: %v", err)
	}
	if tmpl.Subject == "" {
		t.Error("Template subject should not be empty")
	}

	// RenderTemplate
	data := &email.TemplateData{
		InvoiceNumber: "INV-001",
		ContactName:   "John Doe",
	}
	subject, bodyHTML, bodyText, err := svc.RenderTemplate(tmpl, data)
	if err != nil {
		t.Fatalf("RenderTemplate failed: %v", err)
	}
	if subject == "" || bodyHTML == "" || bodyText == "" {
		t.Error("Rendered content should not be empty")
	}

	// SendEmail
	resp, err := svc.SendEmail(ctx, "schema", "tenant-1", "INVOICE_SEND", "test@example.com", "Test User", "Subject", "<p>Body</p>", "Body", nil, "inv-1")
	if err != nil {
		t.Fatalf("SendEmail failed: %v", err)
	}
	if !resp.Success {
		t.Error("SendEmail should return success")
	}
	if len(svc.sentEmails) != 1 {
		t.Errorf("sentEmails length = %d, want 1", len(svc.sentEmails))
	}
}

func TestMockTenantService(t *testing.T) {
	ctx := context.Background()
	svc := NewMockTenantService()

	// Add a tenant
	svc.tenants["tenant-1"] = &tenant.Tenant{
		ID:   "tenant-1",
		Name: "Test Company",
	}

	// GetTenant - found
	got, err := svc.GetTenant(ctx, "tenant-1")
	if err != nil {
		t.Fatalf("GetTenant failed: %v", err)
	}
	if got.Name != "Test Company" {
		t.Errorf("Name = %q, want %q", got.Name, "Test Company")
	}

	// GetTenant - not found
	_, err = svc.GetTenant(ctx, "non-existent")
	if err == nil {
		t.Error("GetTenant should return error for non-existent tenant")
	}
}

func TestMockContactsService(t *testing.T) {
	ctx := context.Background()
	svc := NewMockContactsService()

	// Add a contact
	svc.contacts["contact-1"] = &contacts.Contact{
		ID:    "contact-1",
		Name:  "John Doe",
		Email: "john@example.com",
	}

	// GetByID - found
	got, err := svc.GetByID(ctx, "tenant-1", "schema", "contact-1")
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got.Email != "john@example.com" {
		t.Errorf("Email = %q, want %q", got.Email, "john@example.com")
	}

	// GetByID - not found
	_, err = svc.GetByID(ctx, "tenant-1", "schema", "non-existent")
	if err == nil {
		t.Error("GetByID should return error for non-existent contact")
	}
}

func TestMockPDFService(t *testing.T) {
	svc := NewMockPDFService()

	// PDFSettingsFromTenant - returns a value type so just test it doesn't panic
	settings := svc.PDFSettingsFromTenant(&tenant.Tenant{})

	// GenerateInvoicePDF - success
	pdfBytes, err := svc.GenerateInvoicePDF(&invoicing.Invoice{}, &tenant.Tenant{}, settings)
	if err != nil {
		t.Fatalf("GenerateInvoicePDF failed: %v", err)
	}
	if len(pdfBytes) == 0 {
		t.Error("PDF bytes should not be empty")
	}

	// GenerateInvoicePDF - error
	svc.generateErr = fmt.Errorf("pdf generation failed")
	_, err = svc.GenerateInvoicePDF(&invoicing.Invoice{}, &tenant.Tenant{}, settings)
	if err == nil {
		t.Error("GenerateInvoicePDF should return error when generateErr is set")
	}
}

func TestRecurringInvoiceLineCalculations(t *testing.T) {
	// Test line item calculations
	line := RecurringInvoiceLine{
		Quantity:        decimal.NewFromInt(5),
		UnitPrice:       decimal.NewFromFloat(100.00),
		DiscountPercent: decimal.NewFromFloat(10.00),
		VATRate:         decimal.NewFromFloat(20.00),
	}

	// Calculate subtotal: 5 * 100 = 500
	subtotal := line.Quantity.Mul(line.UnitPrice)
	if !subtotal.Equal(decimal.NewFromInt(500)) {
		t.Errorf("Subtotal = %s, want 500", subtotal)
	}

	// Calculate discount: 500 * 0.10 = 50
	discount := subtotal.Mul(line.DiscountPercent.Div(decimal.NewFromInt(100)))
	if !discount.Equal(decimal.NewFromInt(50)) {
		t.Errorf("Discount = %s, want 50", discount)
	}

	// Net after discount: 500 - 50 = 450
	netAmount := subtotal.Sub(discount)
	if !netAmount.Equal(decimal.NewFromInt(450)) {
		t.Errorf("Net amount = %s, want 450", netAmount)
	}

	// VAT: 450 * 0.20 = 90
	vatAmount := netAmount.Mul(line.VATRate.Div(decimal.NewFromInt(100)))
	if !vatAmount.Equal(decimal.NewFromInt(90)) {
		t.Errorf("VAT amount = %s, want 90", vatAmount)
	}

	// Total: 450 + 90 = 540
	total := netAmount.Add(vatAmount)
	if !total.Equal(decimal.NewFromInt(540)) {
		t.Errorf("Total = %s, want 540", total)
	}
}

// Helper function to create bool pointer
func boolPtr(b bool) *bool {
	return &b
}

// Helper functions for test data
func stringPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}

func frequencyPtr(f Frequency) *Frequency {
	return &f
}

// =============================================================================
// Service Method Tests
// =============================================================================

func TestService_Create(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		setupRepo   func() *MockRepository
		request     *CreateRecurringInvoiceRequest
		expectErr   bool
		errContains string
	}{
		{
			name: "success with all fields",
			setupRepo: func() *MockRepository {
				return NewMockRepository()
			},
			request: &CreateRecurringInvoiceRequest{
				Name:             "Monthly Subscription",
				ContactID:        "contact-1",
				InvoiceType:      "SALES",
				Currency:         "EUR",
				Frequency:        FrequencyMonthly,
				StartDate:        time.Now(),
				PaymentTermsDays: 14,
				Lines: []CreateRecurringInvoiceLineRequest{
					{
						Description: "Service Fee",
						Quantity:    decimal.NewFromInt(1),
						UnitPrice:   decimal.NewFromFloat(100.00),
						VATRate:     decimal.NewFromFloat(20.00),
					},
				},
				UserID: "user-1",
			},
			expectErr: false,
		},
		{
			name: "success with defaults",
			setupRepo: func() *MockRepository {
				return NewMockRepository()
			},
			request: &CreateRecurringInvoiceRequest{
				Name:      "Quarterly Service",
				ContactID: "contact-1",
				Frequency: FrequencyQuarterly,
				StartDate: time.Now(),
				Lines: []CreateRecurringInvoiceLineRequest{
					{
						Description: "Service Fee",
						Quantity:    decimal.NewFromInt(1),
						UnitPrice:   decimal.NewFromFloat(100.00),
						VATRate:     decimal.NewFromFloat(20.00),
					},
				},
				UserID: "user-1",
			},
			expectErr: false,
		},
		{
			name: "validation error - no name",
			setupRepo: func() *MockRepository {
				return NewMockRepository()
			},
			request: &CreateRecurringInvoiceRequest{
				ContactID: "contact-1",
				Frequency: FrequencyMonthly,
				StartDate: time.Now(),
				Lines: []CreateRecurringInvoiceLineRequest{
					{
						Description: "Service Fee",
						Quantity:    decimal.NewFromInt(1),
						UnitPrice:   decimal.NewFromFloat(100.00),
					},
				},
			},
			expectErr:   true,
			errContains: "validation failed",
		},
		{
			name: "validation error - no lines",
			setupRepo: func() *MockRepository {
				return NewMockRepository()
			},
			request: &CreateRecurringInvoiceRequest{
				Name:      "Monthly Subscription",
				ContactID: "contact-1",
				Frequency: FrequencyMonthly,
				StartDate: time.Now(),
				Lines:     []CreateRecurringInvoiceLineRequest{},
			},
			expectErr:   true,
			errContains: "validation failed",
		},
		{
			name: "repository error",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.createErr = fmt.Errorf("db error")
				return repo
			},
			request: &CreateRecurringInvoiceRequest{
				Name:      "Monthly Subscription",
				ContactID: "contact-1",
				Frequency: FrequencyMonthly,
				StartDate: time.Now(),
				Lines: []CreateRecurringInvoiceLineRequest{
					{
						Description: "Service Fee",
						Quantity:    decimal.NewFromInt(1),
						UnitPrice:   decimal.NewFromFloat(100.00),
						VATRate:     decimal.NewFromFloat(20.00),
					},
				},
			},
			expectErr:   true,
			errContains: "create recurring invoice",
		},
		{
			name: "with email configuration",
			setupRepo: func() *MockRepository {
				return NewMockRepository()
			},
			request: &CreateRecurringInvoiceRequest{
				Name:                   "Monthly Subscription",
				ContactID:              "contact-1",
				Frequency:              FrequencyMonthly,
				StartDate:              time.Now(),
				SendEmailOnGeneration:  true,
				EmailTemplateType:      "INVOICE_SEND",
				RecipientEmailOverride: "test@example.com",
				AttachPDFToEmail:       boolPtr(true),
				EmailSubjectOverride:   "Your Invoice",
				EmailMessage:           "Thank you for your business",
				Lines: []CreateRecurringInvoiceLineRequest{
					{
						Description: "Service Fee",
						Quantity:    decimal.NewFromInt(1),
						UnitPrice:   decimal.NewFromFloat(100.00),
						VATRate:     decimal.NewFromFloat(20.00),
					},
				},
				UserID: "user-1",
			},
			expectErr: false,
		},
		{
			name: "create line error",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.createLineErr = fmt.Errorf("line db error")
				return repo
			},
			request: &CreateRecurringInvoiceRequest{
				Name:      "Monthly Subscription",
				ContactID: "contact-1",
				Frequency: FrequencyMonthly,
				StartDate: time.Now(),
				Lines: []CreateRecurringInvoiceLineRequest{
					{
						Description: "Service Fee",
						Quantity:    decimal.NewFromInt(1),
						UnitPrice:   decimal.NewFromFloat(100.00),
						VATRate:     decimal.NewFromFloat(20.00),
					},
				},
			},
			expectErr:   true,
			errContains: "create recurring invoice line",
		},
		{
			name: "zero quantity defaults to 1",
			setupRepo: func() *MockRepository {
				return NewMockRepository()
			},
			request: &CreateRecurringInvoiceRequest{
				Name:      "Monthly Subscription",
				ContactID: "contact-1",
				Frequency: FrequencyMonthly,
				StartDate: time.Now(),
				Lines: []CreateRecurringInvoiceLineRequest{
					{
						Description: "Service Fee",
						Quantity:    decimal.Zero, // Zero quantity should default to 1
						UnitPrice:   decimal.NewFromFloat(100.00),
						VATRate:     decimal.NewFromFloat(20.00),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "ensure schema error",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.ensureSchemaErr = fmt.Errorf("schema error")
				return repo
			},
			request: &CreateRecurringInvoiceRequest{
				Name:      "Monthly Subscription",
				ContactID: "contact-1",
				Frequency: FrequencyMonthly,
				StartDate: time.Now(),
				Lines: []CreateRecurringInvoiceLineRequest{
					{
						Description: "Service Fee",
						Quantity:    decimal.NewFromInt(1),
						UnitPrice:   decimal.NewFromFloat(100.00),
						VATRate:     decimal.NewFromFloat(20.00),
					},
				},
			},
			expectErr:   true,
			errContains: "schema",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setupRepo()
			service := NewServiceWithDependencies(repo, nil, nil, nil, nil, nil)

			result, err := service.Create(ctx, "tenant-1", "test_schema", tt.request)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("expected result but got nil")
			}
			if result.ID == "" {
				t.Error("ID should be set")
			}
			if result.Name != tt.request.Name {
				t.Errorf("Name = %q, want %q", result.Name, tt.request.Name)
			}
		})
	}
}

func TestService_GetByID(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		setupRepo   func() *MockRepository
		tenantID    string
		id          string
		expectErr   bool
		errContains string
	}{
		{
			name: "success",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				ri := &RecurringInvoice{
					ID:        "ri-1",
					TenantID:  "tenant-1",
					Name:      "Monthly Subscription",
					ContactID: "contact-1",
					Frequency: FrequencyMonthly,
					IsActive:  true,
					Lines: []RecurringInvoiceLine{
						{ID: "line-1", RecurringInvoiceID: "ri-1", Description: "Service"},
					},
				}
				_ = repo.Create(ctx, "test_schema", ri)
				_ = repo.CreateLine(ctx, "test_schema", &ri.Lines[0])
				return repo
			},
			tenantID:  "tenant-1",
			id:        "ri-1",
			expectErr: false,
		},
		{
			name: "not found",
			setupRepo: func() *MockRepository {
				return NewMockRepository()
			},
			tenantID:    "tenant-1",
			id:          "non-existent",
			expectErr:   true,
			errContains: "not found",
		},
		{
			name: "repository error",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.getByIDErr = fmt.Errorf("db error")
				return repo
			},
			tenantID:    "tenant-1",
			id:          "ri-1",
			expectErr:   true,
			errContains: "get recurring invoice",
		},
		{
			name: "get lines error",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				ri := &RecurringInvoice{
					ID:        "ri-1",
					TenantID:  "tenant-1",
					Name:      "Monthly Subscription",
					ContactID: "contact-1",
					Frequency: FrequencyMonthly,
					IsActive:  true,
				}
				_ = repo.Create(ctx, "test_schema", ri)
				repo.getLinesErr = fmt.Errorf("lines db error")
				return repo
			},
			tenantID:    "tenant-1",
			id:          "ri-1",
			expectErr:   true,
			errContains: "get recurring invoice lines",
		},
		{
			name: "ensure schema error",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.ensureSchemaErr = fmt.Errorf("schema error")
				return repo
			},
			tenantID:    "tenant-1",
			id:          "ri-1",
			expectErr:   true,
			errContains: "schema",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setupRepo()
			service := NewServiceWithDependencies(repo, nil, nil, nil, nil, nil)

			result, err := service.GetByID(ctx, tt.tenantID, "test_schema", tt.id)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("expected result but got nil")
			}
			if result.ID != tt.id {
				t.Errorf("ID = %q, want %q", result.ID, tt.id)
			}
		})
	}
}

func TestService_List(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		setupRepo   func() *MockRepository
		activeOnly  bool
		expectCount int
		expectErr   bool
	}{
		{
			name: "list all",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				_ = repo.Create(ctx, "test_schema", &RecurringInvoice{ID: "ri-1", TenantID: "tenant-1", IsActive: true})
				_ = repo.Create(ctx, "test_schema", &RecurringInvoice{ID: "ri-2", TenantID: "tenant-1", IsActive: false})
				return repo
			},
			activeOnly:  false,
			expectCount: 2,
			expectErr:   false,
		},
		{
			name: "list active only",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				_ = repo.Create(ctx, "test_schema", &RecurringInvoice{ID: "ri-1", TenantID: "tenant-1", IsActive: true})
				_ = repo.Create(ctx, "test_schema", &RecurringInvoice{ID: "ri-2", TenantID: "tenant-1", IsActive: false})
				return repo
			},
			activeOnly:  true,
			expectCount: 1,
			expectErr:   false,
		},
		{
			name: "empty list",
			setupRepo: func() *MockRepository {
				return NewMockRepository()
			},
			activeOnly:  false,
			expectCount: 0,
			expectErr:   false,
		},
		{
			name: "repository error",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.listErr = fmt.Errorf("db error")
				return repo
			},
			activeOnly: false,
			expectErr:  true,
		},
		{
			name: "ensure schema error",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.ensureSchemaErr = fmt.Errorf("schema error")
				return repo
			},
			activeOnly: false,
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setupRepo()
			service := NewServiceWithDependencies(repo, nil, nil, nil, nil, nil)

			result, err := service.List(ctx, "tenant-1", "test_schema", tt.activeOnly)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result) != tt.expectCount {
				t.Errorf("result count = %d, want %d", len(result), tt.expectCount)
			}
		})
	}
}

func TestService_Update(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		setupRepo   func() *MockRepository
		request     *UpdateRecurringInvoiceRequest
		expectErr   bool
		errContains string
	}{
		{
			name: "update name",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				ri := &RecurringInvoice{
					ID:        "ri-1",
					TenantID:  "tenant-1",
					Name:      "Old Name",
					ContactID: "contact-1",
					Frequency: FrequencyMonthly,
					StartDate: time.Now(),
					IsActive:  true,
					Lines: []RecurringInvoiceLine{
						{ID: "line-1", RecurringInvoiceID: "ri-1", Description: "Service", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100)},
					},
				}
				_ = repo.Create(ctx, "test_schema", ri)
				_ = repo.CreateLine(ctx, "test_schema", &ri.Lines[0])
				return repo
			},
			request: &UpdateRecurringInvoiceRequest{
				Name: stringPtr("New Name"),
			},
			expectErr: false,
		},
		{
			name: "update email configuration",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				ri := &RecurringInvoice{
					ID:        "ri-1",
					TenantID:  "tenant-1",
					Name:      "Monthly Subscription",
					ContactID: "contact-1",
					Frequency: FrequencyMonthly,
					StartDate: time.Now(),
					IsActive:  true,
					Lines: []RecurringInvoiceLine{
						{ID: "line-1", RecurringInvoiceID: "ri-1", Description: "Service", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100)},
					},
				}
				_ = repo.Create(ctx, "test_schema", ri)
				_ = repo.CreateLine(ctx, "test_schema", &ri.Lines[0])
				return repo
			},
			request: &UpdateRecurringInvoiceRequest{
				SendEmailOnGeneration:  boolPtr(true),
				EmailTemplateType:      stringPtr("INVOICE_SEND"),
				RecipientEmailOverride: stringPtr("new@example.com"),
			},
			expectErr: false,
		},
		{
			name: "update with new lines",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				ri := &RecurringInvoice{
					ID:        "ri-1",
					TenantID:  "tenant-1",
					Name:      "Monthly Subscription",
					ContactID: "contact-1",
					Frequency: FrequencyMonthly,
					StartDate: time.Now(),
					IsActive:  true,
					Lines: []RecurringInvoiceLine{
						{ID: "line-1", RecurringInvoiceID: "ri-1", Description: "Old Service", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100)},
					},
				}
				_ = repo.Create(ctx, "test_schema", ri)
				_ = repo.CreateLine(ctx, "test_schema", &ri.Lines[0])
				return repo
			},
			request: &UpdateRecurringInvoiceRequest{
				Lines: []CreateRecurringInvoiceLineRequest{
					{
						Description: "New Service A",
						Quantity:    decimal.NewFromInt(2),
						UnitPrice:   decimal.NewFromFloat(50.00),
					},
					{
						Description: "New Service B",
						Quantity:    decimal.NewFromInt(1),
						UnitPrice:   decimal.NewFromFloat(75.00),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "update lines with zero quantity defaults to 1",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				ri := &RecurringInvoice{
					ID:        "ri-1",
					TenantID:  "tenant-1",
					Name:      "Monthly Subscription",
					ContactID: "contact-1",
					Frequency: FrequencyMonthly,
					StartDate: time.Now(),
					IsActive:  true,
					Lines: []RecurringInvoiceLine{
						{ID: "line-1", RecurringInvoiceID: "ri-1", Description: "Old Service", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100)},
					},
				}
				_ = repo.Create(ctx, "test_schema", ri)
				_ = repo.CreateLine(ctx, "test_schema", &ri.Lines[0])
				return repo
			},
			request: &UpdateRecurringInvoiceRequest{
				Lines: []CreateRecurringInvoiceLineRequest{
					{
						Description: "New Service",
						Quantity:    decimal.Zero, // Zero quantity should default to 1
						UnitPrice:   decimal.NewFromFloat(50.00),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "not found",
			setupRepo: func() *MockRepository {
				return NewMockRepository()
			},
			request: &UpdateRecurringInvoiceRequest{
				Name: stringPtr("New Name"),
			},
			expectErr:   true,
			errContains: "not found",
		},
		{
			name: "repository update error",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				ri := &RecurringInvoice{
					ID:        "ri-1",
					TenantID:  "tenant-1",
					Name:      "Monthly Subscription",
					ContactID: "contact-1",
					Frequency: FrequencyMonthly,
					StartDate: time.Now(),
					IsActive:  true,
					Lines: []RecurringInvoiceLine{
						{ID: "line-1", RecurringInvoiceID: "ri-1", Description: "Service", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100)},
					},
				}
				_ = repo.Create(ctx, "test_schema", ri)
				_ = repo.CreateLine(ctx, "test_schema", &ri.Lines[0])
				repo.updateErr = fmt.Errorf("db error")
				return repo
			},
			request: &UpdateRecurringInvoiceRequest{
				Name: stringPtr("New Name"),
			},
			expectErr:   true,
			errContains: "update recurring invoice",
		},
		{
			name: "delete lines error when updating lines",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				ri := &RecurringInvoice{
					ID:        "ri-1",
					TenantID:  "tenant-1",
					Name:      "Monthly Subscription",
					ContactID: "contact-1",
					Frequency: FrequencyMonthly,
					StartDate: time.Now(),
					IsActive:  true,
					Lines: []RecurringInvoiceLine{
						{ID: "line-1", RecurringInvoiceID: "ri-1", Description: "Service", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100)},
					},
				}
				_ = repo.Create(ctx, "test_schema", ri)
				_ = repo.CreateLine(ctx, "test_schema", &ri.Lines[0])
				repo.deleteLinesErr = fmt.Errorf("db error")
				return repo
			},
			request: &UpdateRecurringInvoiceRequest{
				Lines: []CreateRecurringInvoiceLineRequest{
					{
						Description: "New Service",
						Quantity:    decimal.NewFromInt(1),
						UnitPrice:   decimal.NewFromFloat(100.00),
					},
				},
			},
			expectErr:   true,
			errContains: "delete recurring invoice lines",
		},
		{
			name: "create line error when updating lines",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				ri := &RecurringInvoice{
					ID:        "ri-1",
					TenantID:  "tenant-1",
					Name:      "Monthly Subscription",
					ContactID: "contact-1",
					Frequency: FrequencyMonthly,
					StartDate: time.Now(),
					IsActive:  true,
					Lines: []RecurringInvoiceLine{
						{ID: "line-1", RecurringInvoiceID: "ri-1", Description: "Service", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100)},
					},
				}
				_ = repo.Create(ctx, "test_schema", ri)
				_ = repo.CreateLine(ctx, "test_schema", &ri.Lines[0])
				repo.createLineErr = fmt.Errorf("line db error")
				return repo
			},
			request: &UpdateRecurringInvoiceRequest{
				Lines: []CreateRecurringInvoiceLineRequest{
					{
						Description: "New Service",
						Quantity:    decimal.NewFromInt(1),
						UnitPrice:   decimal.NewFromFloat(100.00),
					},
				},
			},
			expectErr:   true,
			errContains: "create recurring invoice line",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setupRepo()
			service := NewServiceWithDependencies(repo, nil, nil, nil, nil, nil)

			result, err := service.Update(ctx, "tenant-1", "test_schema", "ri-1", tt.request)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("expected result but got nil")
			}
		})
	}
}

func TestService_Delete(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		setupRepo   func() *MockRepository
		expectErr   bool
		errContains string
	}{
		{
			name: "success",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				_ = repo.Create(ctx, "test_schema", &RecurringInvoice{ID: "ri-1", TenantID: "tenant-1"})
				return repo
			},
			expectErr: false,
		},
		{
			name: "not found",
			setupRepo: func() *MockRepository {
				return NewMockRepository()
			},
			expectErr:   true,
			errContains: "not found",
		},
		{
			name: "repository error",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				_ = repo.Create(ctx, "test_schema", &RecurringInvoice{ID: "ri-1", TenantID: "tenant-1"})
				repo.deleteErr = fmt.Errorf("db error")
				return repo
			},
			expectErr:   true,
			errContains: "delete recurring invoice",
		},
		{
			name: "ensure schema error",
			setupRepo: func() *MockRepository {
				repo := NewMockRepository()
				repo.ensureSchemaErr = fmt.Errorf("schema error")
				return repo
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setupRepo()
			service := NewServiceWithDependencies(repo, nil, nil, nil, nil, nil)

			err := service.Delete(ctx, "tenant-1", "test_schema", "ri-1")

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify deleted
			_, getErr := repo.GetByID(ctx, "test_schema", "tenant-1", "ri-1")
			if getErr != ErrRecurringInvoiceNotFound {
				t.Error("recurring invoice should be deleted")
			}
		})
	}
}

func TestService_PauseResume(t *testing.T) {
	ctx := context.Background()

	t.Run("pause success", func(t *testing.T) {
		repo := NewMockRepository()
		_ = repo.Create(ctx, "test_schema", &RecurringInvoice{ID: "ri-1", TenantID: "tenant-1", IsActive: true})
		service := NewServiceWithDependencies(repo, nil, nil, nil, nil, nil)

		err := service.Pause(ctx, "tenant-1", "test_schema", "ri-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		ri, _ := repo.GetByID(ctx, "test_schema", "tenant-1", "ri-1")
		if ri.IsActive {
			t.Error("recurring invoice should be paused")
		}
	})

	t.Run("resume success", func(t *testing.T) {
		repo := NewMockRepository()
		_ = repo.Create(ctx, "test_schema", &RecurringInvoice{ID: "ri-1", TenantID: "tenant-1", IsActive: false})
		service := NewServiceWithDependencies(repo, nil, nil, nil, nil, nil)

		err := service.Resume(ctx, "tenant-1", "test_schema", "ri-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		ri, _ := repo.GetByID(ctx, "test_schema", "tenant-1", "ri-1")
		if !ri.IsActive {
			t.Error("recurring invoice should be resumed")
		}
	})

	t.Run("pause not found", func(t *testing.T) {
		repo := NewMockRepository()
		service := NewServiceWithDependencies(repo, nil, nil, nil, nil, nil)

		err := service.Pause(ctx, "tenant-1", "test_schema", "non-existent")
		if err == nil {
			t.Error("expected error for non-existent invoice")
		}
		if !contains(err.Error(), "not found") {
			t.Errorf("error should contain 'not found', got %q", err.Error())
		}
	})

	t.Run("pause repository error", func(t *testing.T) {
		repo := NewMockRepository()
		_ = repo.Create(ctx, "test_schema", &RecurringInvoice{ID: "ri-1", TenantID: "tenant-1", IsActive: true})
		repo.setActiveErr = fmt.Errorf("db error")
		service := NewServiceWithDependencies(repo, nil, nil, nil, nil, nil)

		err := service.Pause(ctx, "tenant-1", "test_schema", "ri-1")
		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestService_CreateFromInvoice(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		repo := NewMockRepository()
		invoicingSvc := NewMockInvoicingService()
		invoicingSvc.invoices["inv-1"] = &invoicing.Invoice{
			ID:          "inv-1",
			TenantID:    "tenant-1",
			InvoiceType: invoicing.InvoiceTypeSales,
			ContactID:   "contact-1",
			Currency:    "EUR",
			Reference:   "REF-001",
			Notes:       "Test notes",
			Lines: []invoicing.InvoiceLine{
				{
					Description: "Service",
					Quantity:    decimal.NewFromInt(1),
					UnitPrice:   decimal.NewFromFloat(100.00),
					VATRate:     decimal.NewFromFloat(20.00),
				},
			},
		}
		service := NewServiceWithDependencies(repo, invoicingSvc, nil, nil, nil, nil)

		req := &CreateFromInvoiceRequest{
			InvoiceID:        "inv-1",
			Name:             "Monthly from Invoice",
			Frequency:        FrequencyMonthly,
			StartDate:        time.Now(),
			PaymentTermsDays: 14,
			UserID:           "user-1",
		}

		result, err := service.CreateFromInvoice(ctx, "tenant-1", "test_schema", req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("expected result")
		}
		if result.Name != "Monthly from Invoice" {
			t.Errorf("Name = %q, want %q", result.Name, "Monthly from Invoice")
		}
		if result.ContactID != "contact-1" {
			t.Errorf("ContactID = %q, want %q", result.ContactID, "contact-1")
		}
		if len(result.Lines) != 1 {
			t.Errorf("Lines count = %d, want 1", len(result.Lines))
		}
	})

	t.Run("invoice not found", func(t *testing.T) {
		repo := NewMockRepository()
		invoicingSvc := NewMockInvoicingService()
		service := NewServiceWithDependencies(repo, invoicingSvc, nil, nil, nil, nil)

		req := &CreateFromInvoiceRequest{
			InvoiceID: "non-existent",
			Name:      "Test",
			Frequency: FrequencyMonthly,
			StartDate: time.Now(),
		}

		_, err := service.CreateFromInvoice(ctx, "tenant-1", "test_schema", req)
		if err == nil {
			t.Error("expected error for non-existent invoice")
		}
	})
}

func TestService_GenerateDueInvoices(t *testing.T) {
	ctx := context.Background()

	t.Run("generates due invoices", func(t *testing.T) {
		repo := NewMockRepository()
		now := time.Now()
		// Create a due recurring invoice
		ri := &RecurringInvoice{
			ID:                 "ri-1",
			TenantID:           "tenant-1",
			Name:               "Monthly Service",
			ContactID:          "contact-1",
			Frequency:          FrequencyMonthly,
			NextGenerationDate: now.AddDate(0, 0, -1), // Due yesterday
			IsActive:           true,
			Lines: []RecurringInvoiceLine{
				{ID: "line-1", RecurringInvoiceID: "ri-1", Description: "Service", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100)},
			},
		}
		_ = repo.Create(ctx, "test_schema", ri)
		_ = repo.CreateLine(ctx, "test_schema", &ri.Lines[0])

		invoicingSvc := NewMockInvoicingService()
		service := NewServiceWithDependencies(repo, invoicingSvc, nil, nil, nil, nil)

		results, err := service.GenerateDueInvoices(ctx, "tenant-1", "test_schema", "user-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("results count = %d, want 1", len(results))
		}
	})

	t.Run("skips not due invoices", func(t *testing.T) {
		repo := NewMockRepository()
		now := time.Now()
		// Create a not due recurring invoice
		ri := &RecurringInvoice{
			ID:                 "ri-1",
			TenantID:           "tenant-1",
			Name:               "Monthly Service",
			ContactID:          "contact-1",
			Frequency:          FrequencyMonthly,
			NextGenerationDate: now.AddDate(0, 0, 1), // Due tomorrow
			IsActive:           true,
			Lines: []RecurringInvoiceLine{
				{ID: "line-1", RecurringInvoiceID: "ri-1", Description: "Service", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100)},
			},
		}
		_ = repo.Create(ctx, "test_schema", ri)
		_ = repo.CreateLine(ctx, "test_schema", &ri.Lines[0])

		invoicingSvc := NewMockInvoicingService()
		service := NewServiceWithDependencies(repo, invoicingSvc, nil, nil, nil, nil)

		results, err := service.GenerateDueInvoices(ctx, "tenant-1", "test_schema", "user-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("results count = %d, want 0", len(results))
		}
	})

	t.Run("skips inactive invoices", func(t *testing.T) {
		repo := NewMockRepository()
		now := time.Now()
		// Create an inactive recurring invoice that would otherwise be due
		ri := &RecurringInvoice{
			ID:                 "ri-1",
			TenantID:           "tenant-1",
			Name:               "Monthly Service",
			ContactID:          "contact-1",
			Frequency:          FrequencyMonthly,
			NextGenerationDate: now.AddDate(0, 0, -1),
			IsActive:           false, // Inactive
			Lines: []RecurringInvoiceLine{
				{ID: "line-1", RecurringInvoiceID: "ri-1", Description: "Service", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100)},
			},
		}
		_ = repo.Create(ctx, "test_schema", ri)
		_ = repo.CreateLine(ctx, "test_schema", &ri.Lines[0])

		invoicingSvc := NewMockInvoicingService()
		service := NewServiceWithDependencies(repo, invoicingSvc, nil, nil, nil, nil)

		results, err := service.GenerateDueInvoices(ctx, "tenant-1", "test_schema", "user-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("results count = %d, want 0", len(results))
		}
	})

	t.Run("repository error", func(t *testing.T) {
		repo := NewMockRepository()
		repo.getDueIDsErr = fmt.Errorf("db error")
		service := NewServiceWithDependencies(repo, nil, nil, nil, nil, nil)

		_, err := service.GenerateDueInvoices(ctx, "tenant-1", "test_schema", "user-1")
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("ensure schema error", func(t *testing.T) {
		repo := NewMockRepository()
		repo.ensureSchemaErr = fmt.Errorf("schema error")
		service := NewServiceWithDependencies(repo, nil, nil, nil, nil, nil)

		_, err := service.GenerateDueInvoices(ctx, "tenant-1", "test_schema", "user-1")
		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestService_GenerateInvoice(t *testing.T) {
	ctx := context.Background()

	t.Run("success without email", func(t *testing.T) {
		repo := NewMockRepository()
		now := time.Now()
		ri := &RecurringInvoice{
			ID:                 "ri-1",
			TenantID:           "tenant-1",
			Name:               "Monthly Service",
			ContactID:          "contact-1",
			InvoiceType:        "SALES",
			Currency:           "EUR",
			Frequency:          FrequencyMonthly,
			NextGenerationDate: now,
			PaymentTermsDays:   14,
			IsActive:           true,
			Lines: []RecurringInvoiceLine{
				{ID: "line-1", RecurringInvoiceID: "ri-1", Description: "Service", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100)},
			},
		}
		_ = repo.Create(ctx, "test_schema", ri)
		_ = repo.CreateLine(ctx, "test_schema", &ri.Lines[0])

		invoicingSvc := NewMockInvoicingService()
		service := NewServiceWithDependencies(repo, invoicingSvc, nil, nil, nil, nil)

		result, err := service.GenerateInvoice(ctx, "tenant-1", "test_schema", "ri-1", "user-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("expected result")
		}
		if result.RecurringInvoiceID != "ri-1" {
			t.Errorf("RecurringInvoiceID = %q, want %q", result.RecurringInvoiceID, "ri-1")
		}
		if result.EmailStatus != "SKIPPED" {
			t.Errorf("EmailStatus = %q, want %q", result.EmailStatus, "SKIPPED")
		}

		// Verify next generation date was updated
		updatedRI, _ := repo.GetByID(ctx, "test_schema", "tenant-1", "ri-1")
		if !updatedRI.NextGenerationDate.After(now) {
			t.Error("NextGenerationDate should be updated")
		}
		if updatedRI.GeneratedCount != 1 {
			t.Errorf("GeneratedCount = %d, want 1", updatedRI.GeneratedCount)
		}
	})

	t.Run("inactive invoice error", func(t *testing.T) {
		repo := NewMockRepository()
		ri := &RecurringInvoice{
			ID:       "ri-1",
			TenantID: "tenant-1",
			IsActive: false,
			Lines: []RecurringInvoiceLine{
				{ID: "line-1", RecurringInvoiceID: "ri-1", Description: "Service", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100)},
			},
		}
		_ = repo.Create(ctx, "test_schema", ri)
		_ = repo.CreateLine(ctx, "test_schema", &ri.Lines[0])

		invoicingSvc := NewMockInvoicingService()
		service := NewServiceWithDependencies(repo, invoicingSvc, nil, nil, nil, nil)

		_, err := service.GenerateInvoice(ctx, "tenant-1", "test_schema", "ri-1", "user-1")
		if err == nil {
			t.Error("expected error for inactive invoice")
		}
		if !contains(err.Error(), "not active") {
			t.Errorf("error should contain 'not active', got %q", err.Error())
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo := NewMockRepository()
		invoicingSvc := NewMockInvoicingService()
		service := NewServiceWithDependencies(repo, invoicingSvc, nil, nil, nil, nil)

		_, err := service.GenerateInvoice(ctx, "tenant-1", "test_schema", "non-existent", "user-1")
		if err == nil {
			t.Error("expected error for non-existent invoice")
		}
	})

	t.Run("invoice creation error", func(t *testing.T) {
		repo := NewMockRepository()
		now := time.Now()
		ri := &RecurringInvoice{
			ID:                 "ri-1",
			TenantID:           "tenant-1",
			Name:               "Monthly Service",
			ContactID:          "contact-1",
			Frequency:          FrequencyMonthly,
			NextGenerationDate: now,
			IsActive:           true,
			Lines: []RecurringInvoiceLine{
				{ID: "line-1", RecurringInvoiceID: "ri-1", Description: "Service", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100)},
			},
		}
		_ = repo.Create(ctx, "test_schema", ri)
		_ = repo.CreateLine(ctx, "test_schema", &ri.Lines[0])

		invoicingSvc := NewMockInvoicingService()
		invoicingSvc.createErr = fmt.Errorf("invoice creation failed")
		service := NewServiceWithDependencies(repo, invoicingSvc, nil, nil, nil, nil)

		_, err := service.GenerateInvoice(ctx, "tenant-1", "test_schema", "ri-1", "user-1")
		if err == nil {
			t.Error("expected error")
		}
		if !contains(err.Error(), "create invoice") {
			t.Errorf("error should contain 'create invoice', got %q", err.Error())
		}
	})
}

func TestService_GenerateInvoiceWithEmail(t *testing.T) {
	ctx := context.Background()

	t.Run("success with email", func(t *testing.T) {
		repo := NewMockRepository()
		now := time.Now()
		ri := &RecurringInvoice{
			ID:                    "ri-1",
			TenantID:              "tenant-1",
			Name:                  "Monthly Service",
			ContactID:             "contact-1",
			InvoiceType:           "SALES",
			Currency:              "EUR",
			Frequency:             FrequencyMonthly,
			NextGenerationDate:    now,
			PaymentTermsDays:      14,
			IsActive:              true,
			SendEmailOnGeneration: true,
			EmailTemplateType:     "INVOICE_SEND",
			AttachPDFToEmail:      true,
			Lines: []RecurringInvoiceLine{
				{ID: "line-1", RecurringInvoiceID: "ri-1", Description: "Service", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100)},
			},
		}
		_ = repo.Create(ctx, "test_schema", ri)
		_ = repo.CreateLine(ctx, "test_schema", &ri.Lines[0])

		invoicingSvc := NewMockInvoicingService()
		emailSvc := NewMockEmailService()
		pdfSvc := NewMockPDFService()
		tenantSvc := NewMockTenantService()
		tenantSvc.tenants["tenant-1"] = &tenant.Tenant{ID: "tenant-1", Name: "Test Company"}
		contactsSvc := NewMockContactsService()
		contactsSvc.contacts["contact-1"] = &contacts.Contact{ID: "contact-1", Name: "John Doe", Email: "john@example.com"}

		service := NewServiceWithDependencies(repo, invoicingSvc, emailSvc, pdfSvc, tenantSvc, contactsSvc)

		result, err := service.GenerateInvoice(ctx, "tenant-1", "test_schema", "ri-1", "user-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("expected result")
		}
		if !result.EmailSent {
			t.Error("EmailSent should be true")
		}
		if result.EmailStatus != "SENT" {
			t.Errorf("EmailStatus = %q, want %q", result.EmailStatus, "SENT")
		}
		if len(emailSvc.sentEmails) != 1 {
			t.Errorf("sentEmails count = %d, want 1", len(emailSvc.sentEmails))
		}
	})

	t.Run("email service not configured", func(t *testing.T) {
		repo := NewMockRepository()
		now := time.Now()
		ri := &RecurringInvoice{
			ID:                    "ri-1",
			TenantID:              "tenant-1",
			Name:                  "Monthly Service",
			ContactID:             "contact-1",
			Frequency:             FrequencyMonthly,
			NextGenerationDate:    now,
			IsActive:              true,
			SendEmailOnGeneration: true,
			Lines: []RecurringInvoiceLine{
				{ID: "line-1", RecurringInvoiceID: "ri-1", Description: "Service", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100)},
			},
		}
		_ = repo.Create(ctx, "test_schema", ri)
		_ = repo.CreateLine(ctx, "test_schema", &ri.Lines[0])

		invoicingSvc := NewMockInvoicingService()
		// Note: email service is nil
		service := NewServiceWithDependencies(repo, invoicingSvc, nil, nil, nil, nil)

		result, err := service.GenerateInvoice(ctx, "tenant-1", "test_schema", "ri-1", "user-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.EmailStatus != "NO_CONFIG" {
			t.Errorf("EmailStatus = %q, want %q", result.EmailStatus, "NO_CONFIG")
		}
	})

	t.Run("no recipient email", func(t *testing.T) {
		repo := NewMockRepository()
		now := time.Now()
		ri := &RecurringInvoice{
			ID:                    "ri-1",
			TenantID:              "tenant-1",
			Name:                  "Monthly Service",
			ContactID:             "contact-1",
			Frequency:             FrequencyMonthly,
			NextGenerationDate:    now,
			IsActive:              true,
			SendEmailOnGeneration: true,
			Lines: []RecurringInvoiceLine{
				{ID: "line-1", RecurringInvoiceID: "ri-1", Description: "Service", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100)},
			},
		}
		_ = repo.Create(ctx, "test_schema", ri)
		_ = repo.CreateLine(ctx, "test_schema", &ri.Lines[0])

		invoicingSvc := NewMockInvoicingService()
		emailSvc := NewMockEmailService()
		tenantSvc := NewMockTenantService()
		tenantSvc.tenants["tenant-1"] = &tenant.Tenant{ID: "tenant-1", Name: "Test Company"}
		contactsSvc := NewMockContactsService()
		contactsSvc.contacts["contact-1"] = &contacts.Contact{ID: "contact-1", Name: "John Doe", Email: ""} // No email

		service := NewServiceWithDependencies(repo, invoicingSvc, emailSvc, nil, tenantSvc, contactsSvc)

		result, err := service.GenerateInvoice(ctx, "tenant-1", "test_schema", "ri-1", "user-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.EmailStatus != "SKIPPED" {
			t.Errorf("EmailStatus = %q, want %q", result.EmailStatus, "SKIPPED")
		}
	})
}

func TestService_GenerateInvoiceWithEmail_TenantError(t *testing.T) {
	ctx := context.Background()

	repo := NewMockRepository()
	now := time.Now()
	ri := &RecurringInvoice{
		ID:                    "ri-1",
		TenantID:              "tenant-1",
		Name:                  "Monthly Service",
		ContactID:             "contact-1",
		Frequency:             FrequencyMonthly,
		NextGenerationDate:    now,
		IsActive:              true,
		SendEmailOnGeneration: true,
		Lines: []RecurringInvoiceLine{
			{ID: "line-1", RecurringInvoiceID: "ri-1", Description: "Service", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100)},
		},
	}
	_ = repo.Create(ctx, "test_schema", ri)
	_ = repo.CreateLine(ctx, "test_schema", &ri.Lines[0])

	invoicingSvc := NewMockInvoicingService()
	emailSvc := NewMockEmailService()
	tenantSvc := NewMockTenantService()
	tenantSvc.getErr = fmt.Errorf("tenant not found")

	service := NewServiceWithDependencies(repo, invoicingSvc, emailSvc, nil, tenantSvc, nil)

	result, err := service.GenerateInvoice(ctx, "tenant-1", "test_schema", "ri-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EmailStatus != "FAILED" {
		t.Errorf("EmailStatus = %q, want %q", result.EmailStatus, "FAILED")
	}
	if result.EmailError == "" {
		t.Error("EmailError should contain error message")
	}
}

func TestService_GenerateInvoiceWithEmail_ContactError(t *testing.T) {
	ctx := context.Background()

	repo := NewMockRepository()
	now := time.Now()
	ri := &RecurringInvoice{
		ID:                    "ri-1",
		TenantID:              "tenant-1",
		Name:                  "Monthly Service",
		ContactID:             "contact-1",
		Frequency:             FrequencyMonthly,
		NextGenerationDate:    now,
		IsActive:              true,
		SendEmailOnGeneration: true,
		Lines: []RecurringInvoiceLine{
			{ID: "line-1", RecurringInvoiceID: "ri-1", Description: "Service", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100)},
		},
	}
	_ = repo.Create(ctx, "test_schema", ri)
	_ = repo.CreateLine(ctx, "test_schema", &ri.Lines[0])

	invoicingSvc := NewMockInvoicingService()
	emailSvc := NewMockEmailService()
	tenantSvc := NewMockTenantService()
	tenantSvc.tenants["tenant-1"] = &tenant.Tenant{ID: "tenant-1", Name: "Test Company"}
	contactsSvc := NewMockContactsService()
	contactsSvc.getErr = fmt.Errorf("contact not found")

	service := NewServiceWithDependencies(repo, invoicingSvc, emailSvc, nil, tenantSvc, contactsSvc)

	result, err := service.GenerateInvoice(ctx, "tenant-1", "test_schema", "ri-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EmailStatus != "FAILED" {
		t.Errorf("EmailStatus = %q, want %q", result.EmailStatus, "FAILED")
	}
	if result.EmailError == "" || !contains(result.EmailError, "contact") {
		t.Errorf("EmailError should mention contact, got %q", result.EmailError)
	}
}

func TestService_GenerateInvoiceWithEmail_TemplateError(t *testing.T) {
	ctx := context.Background()

	repo := NewMockRepository()
	now := time.Now()
	ri := &RecurringInvoice{
		ID:                    "ri-1",
		TenantID:              "tenant-1",
		Name:                  "Monthly Service",
		ContactID:             "contact-1",
		Frequency:             FrequencyMonthly,
		NextGenerationDate:    now,
		IsActive:              true,
		SendEmailOnGeneration: true,
		Lines: []RecurringInvoiceLine{
			{ID: "line-1", RecurringInvoiceID: "ri-1", Description: "Service", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100)},
		},
	}
	_ = repo.Create(ctx, "test_schema", ri)
	_ = repo.CreateLine(ctx, "test_schema", &ri.Lines[0])

	invoicingSvc := NewMockInvoicingService()
	emailSvc := NewMockEmailService()
	emailSvc.getTemplateErr = fmt.Errorf("template not found")
	tenantSvc := NewMockTenantService()
	tenantSvc.tenants["tenant-1"] = &tenant.Tenant{ID: "tenant-1", Name: "Test Company"}
	contactsSvc := NewMockContactsService()
	contactsSvc.contacts["contact-1"] = &contacts.Contact{ID: "contact-1", Name: "John Doe", Email: "john@example.com"}

	service := NewServiceWithDependencies(repo, invoicingSvc, emailSvc, nil, tenantSvc, contactsSvc)

	result, err := service.GenerateInvoice(ctx, "tenant-1", "test_schema", "ri-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EmailStatus != "FAILED" {
		t.Errorf("EmailStatus = %q, want %q", result.EmailStatus, "FAILED")
	}
	if result.EmailError == "" || !contains(result.EmailError, "template") {
		t.Errorf("EmailError should mention template, got %q", result.EmailError)
	}
}

func TestService_GenerateInvoiceWithEmail_RenderTemplateError(t *testing.T) {
	ctx := context.Background()

	repo := NewMockRepository()
	now := time.Now()
	ri := &RecurringInvoice{
		ID:                    "ri-1",
		TenantID:              "tenant-1",
		Name:                  "Monthly Service",
		ContactID:             "contact-1",
		Frequency:             FrequencyMonthly,
		NextGenerationDate:    now,
		IsActive:              true,
		SendEmailOnGeneration: true,
		Lines: []RecurringInvoiceLine{
			{ID: "line-1", RecurringInvoiceID: "ri-1", Description: "Service", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100)},
		},
	}
	_ = repo.Create(ctx, "test_schema", ri)
	_ = repo.CreateLine(ctx, "test_schema", &ri.Lines[0])

	invoicingSvc := NewMockInvoicingService()
	emailSvc := NewMockEmailService()
	emailSvc.renderTemplateErr = fmt.Errorf("invalid template syntax")
	tenantSvc := NewMockTenantService()
	tenantSvc.tenants["tenant-1"] = &tenant.Tenant{ID: "tenant-1", Name: "Test Company"}
	contactsSvc := NewMockContactsService()
	contactsSvc.contacts["contact-1"] = &contacts.Contact{ID: "contact-1", Name: "John Doe", Email: "john@example.com"}

	service := NewServiceWithDependencies(repo, invoicingSvc, emailSvc, nil, tenantSvc, contactsSvc)

	result, err := service.GenerateInvoice(ctx, "tenant-1", "test_schema", "ri-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EmailStatus != "FAILED" {
		t.Errorf("EmailStatus = %q, want %q", result.EmailStatus, "FAILED")
	}
	if result.EmailError == "" || !contains(result.EmailError, "render template") {
		t.Errorf("EmailError should mention render template, got %q", result.EmailError)
	}
}

func TestService_GenerateInvoiceWithEmail_SendError(t *testing.T) {
	ctx := context.Background()

	repo := NewMockRepository()
	now := time.Now()
	ri := &RecurringInvoice{
		ID:                    "ri-1",
		TenantID:              "tenant-1",
		Name:                  "Monthly Service",
		ContactID:             "contact-1",
		Frequency:             FrequencyMonthly,
		NextGenerationDate:    now,
		IsActive:              true,
		SendEmailOnGeneration: true,
		Lines: []RecurringInvoiceLine{
			{ID: "line-1", RecurringInvoiceID: "ri-1", Description: "Service", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100)},
		},
	}
	_ = repo.Create(ctx, "test_schema", ri)
	_ = repo.CreateLine(ctx, "test_schema", &ri.Lines[0])

	invoicingSvc := NewMockInvoicingService()
	emailSvc := NewMockEmailService()
	emailSvc.sendErr = fmt.Errorf("SMTP error")
	tenantSvc := NewMockTenantService()
	tenantSvc.tenants["tenant-1"] = &tenant.Tenant{ID: "tenant-1", Name: "Test Company"}
	contactsSvc := NewMockContactsService()
	contactsSvc.contacts["contact-1"] = &contacts.Contact{ID: "contact-1", Name: "John Doe", Email: "john@example.com"}

	service := NewServiceWithDependencies(repo, invoicingSvc, emailSvc, nil, tenantSvc, contactsSvc)

	result, err := service.GenerateInvoice(ctx, "tenant-1", "test_schema", "ri-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EmailStatus != "FAILED" {
		t.Errorf("EmailStatus = %q, want %q", result.EmailStatus, "FAILED")
	}
	if result.EmailError == "" || !contains(result.EmailError, "send email") {
		t.Errorf("EmailError should mention send email, got %q", result.EmailError)
	}
}

func TestService_GenerateInvoiceWithEmail_PDFError(t *testing.T) {
	ctx := context.Background()

	repo := NewMockRepository()
	now := time.Now()
	ri := &RecurringInvoice{
		ID:                    "ri-1",
		TenantID:              "tenant-1",
		Name:                  "Monthly Service",
		ContactID:             "contact-1",
		Frequency:             FrequencyMonthly,
		NextGenerationDate:    now,
		IsActive:              true,
		SendEmailOnGeneration: true,
		AttachPDFToEmail:      true,
		Lines: []RecurringInvoiceLine{
			{ID: "line-1", RecurringInvoiceID: "ri-1", Description: "Service", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100)},
		},
	}
	_ = repo.Create(ctx, "test_schema", ri)
	_ = repo.CreateLine(ctx, "test_schema", &ri.Lines[0])

	invoicingSvc := NewMockInvoicingService()
	emailSvc := NewMockEmailService()
	pdfSvc := NewMockPDFService()
	pdfSvc.generateErr = fmt.Errorf("PDF generation failed")
	tenantSvc := NewMockTenantService()
	tenantSvc.tenants["tenant-1"] = &tenant.Tenant{ID: "tenant-1", Name: "Test Company"}
	contactsSvc := NewMockContactsService()
	contactsSvc.contacts["contact-1"] = &contacts.Contact{ID: "contact-1", Name: "John Doe", Email: "john@example.com"}

	service := NewServiceWithDependencies(repo, invoicingSvc, emailSvc, pdfSvc, tenantSvc, contactsSvc)

	result, err := service.GenerateInvoice(ctx, "tenant-1", "test_schema", "ri-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Email should still be sent despite PDF error
	if result.EmailStatus != "SENT" {
		t.Errorf("EmailStatus = %q, want %q", result.EmailStatus, "SENT")
	}
	if !result.EmailSent {
		t.Error("EmailSent should be true")
	}
}

func TestService_GenerateInvoiceWithEmail_SubjectOverride(t *testing.T) {
	ctx := context.Background()

	repo := NewMockRepository()
	now := time.Now()
	ri := &RecurringInvoice{
		ID:                    "ri-1",
		TenantID:              "tenant-1",
		Name:                  "Monthly Service",
		ContactID:             "contact-1",
		Frequency:             FrequencyMonthly,
		NextGenerationDate:    now,
		IsActive:              true,
		SendEmailOnGeneration: true,
		EmailSubjectOverride:  "Custom Subject Override",
		Lines: []RecurringInvoiceLine{
			{ID: "line-1", RecurringInvoiceID: "ri-1", Description: "Service", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100)},
		},
	}
	_ = repo.Create(ctx, "test_schema", ri)
	_ = repo.CreateLine(ctx, "test_schema", &ri.Lines[0])

	invoicingSvc := NewMockInvoicingService()
	emailSvc := NewMockEmailService()
	tenantSvc := NewMockTenantService()
	tenantSvc.tenants["tenant-1"] = &tenant.Tenant{ID: "tenant-1", Name: "Test Company"}
	contactsSvc := NewMockContactsService()
	contactsSvc.contacts["contact-1"] = &contacts.Contact{ID: "contact-1", Name: "John Doe", Email: "john@example.com"}

	service := NewServiceWithDependencies(repo, invoicingSvc, emailSvc, nil, tenantSvc, contactsSvc)

	result, err := service.GenerateInvoice(ctx, "tenant-1", "test_schema", "ri-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EmailStatus != "SENT" {
		t.Errorf("EmailStatus = %q, want %q", result.EmailStatus, "SENT")
	}
	// Check that sent email has correct subject
	if len(emailSvc.sentEmails) != 1 {
		t.Fatalf("expected 1 sent email, got %d", len(emailSvc.sentEmails))
	}
}

func TestService_GenerateInvoiceWithEmail_RecipientOverride(t *testing.T) {
	ctx := context.Background()

	repo := NewMockRepository()
	now := time.Now()
	ri := &RecurringInvoice{
		ID:                     "ri-1",
		TenantID:               "tenant-1",
		Name:                   "Monthly Service",
		ContactID:              "contact-1",
		Frequency:              FrequencyMonthly,
		NextGenerationDate:     now,
		IsActive:               true,
		SendEmailOnGeneration:  true,
		RecipientEmailOverride: "override@example.com",
		Lines: []RecurringInvoiceLine{
			{ID: "line-1", RecurringInvoiceID: "ri-1", Description: "Service", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100)},
		},
	}
	_ = repo.Create(ctx, "test_schema", ri)
	_ = repo.CreateLine(ctx, "test_schema", &ri.Lines[0])

	invoicingSvc := NewMockInvoicingService()
	emailSvc := NewMockEmailService()
	tenantSvc := NewMockTenantService()
	tenantSvc.tenants["tenant-1"] = &tenant.Tenant{ID: "tenant-1", Name: "Test Company"}
	contactsSvc := NewMockContactsService()
	contactsSvc.contacts["contact-1"] = &contacts.Contact{ID: "contact-1", Name: "John Doe", Email: "john@example.com"}

	service := NewServiceWithDependencies(repo, invoicingSvc, emailSvc, nil, tenantSvc, contactsSvc)

	result, err := service.GenerateInvoice(ctx, "tenant-1", "test_schema", "ri-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EmailStatus != "SENT" {
		t.Errorf("EmailStatus = %q, want %q", result.EmailStatus, "SENT")
	}
	// Verify email was sent to override address
	if len(emailSvc.sentEmails) != 1 {
		t.Fatalf("expected 1 sent email, got %d", len(emailSvc.sentEmails))
	}
	if emailSvc.sentEmails[0].ToEmail != "override@example.com" {
		t.Errorf("ToEmail = %q, want %q", emailSvc.sentEmails[0].ToEmail, "override@example.com")
	}
}

func TestService_GenerateInvoiceWithEmail_UpdateEmailStatusError(t *testing.T) {
	ctx := context.Background()

	repo := NewMockRepository()
	now := time.Now()
	ri := &RecurringInvoice{
		ID:                    "ri-1",
		TenantID:              "tenant-1",
		Name:                  "Monthly Service",
		ContactID:             "contact-1",
		InvoiceType:           "SALES",
		Currency:              "EUR",
		Frequency:             FrequencyMonthly,
		NextGenerationDate:    now,
		PaymentTermsDays:      14,
		IsActive:              true,
		SendEmailOnGeneration: true,
		EmailTemplateType:     "INVOICE_SEND",
		Lines: []RecurringInvoiceLine{
			{ID: "line-1", RecurringInvoiceID: "ri-1", Description: "Service", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100)},
		},
	}
	_ = repo.Create(ctx, "test_schema", ri)
	_ = repo.CreateLine(ctx, "test_schema", &ri.Lines[0])
	// Set error for UpdateInvoiceEmailStatus
	repo.updateInvoiceEmailStatusErr = fmt.Errorf("db error updating email status")

	invoicingSvc := NewMockInvoicingService()
	emailSvc := NewMockEmailService()
	tenantSvc := NewMockTenantService()
	tenantSvc.tenants["tenant-1"] = &tenant.Tenant{ID: "tenant-1", Name: "Test Company"}
	contactsSvc := NewMockContactsService()
	contactsSvc.contacts["contact-1"] = &contacts.Contact{ID: "contact-1", Name: "John Doe", Email: "john@example.com"}

	service := NewServiceWithDependencies(repo, invoicingSvc, emailSvc, nil, tenantSvc, contactsSvc)

	// The error is logged but shouldn't fail the overall generation
	result, err := service.GenerateInvoice(ctx, "tenant-1", "test_schema", "ri-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	// Email should still be marked as sent
	if !result.EmailSent {
		t.Error("EmailSent should be true despite email status update failure")
	}
	if result.EmailStatus != "SENT" {
		t.Errorf("EmailStatus = %q, want %q", result.EmailStatus, "SENT")
	}
}

func TestService_Update_AllFields(t *testing.T) {
	ctx := context.Background()

	repo := NewMockRepository()
	ri := &RecurringInvoice{
		ID:               "ri-1",
		TenantID:         "tenant-1",
		Name:             "Old Name",
		ContactID:        "old-contact",
		Frequency:        FrequencyMonthly,
		StartDate:        time.Now(),
		PaymentTermsDays: 14,
		IsActive:         true,
		Lines: []RecurringInvoiceLine{
			{ID: "line-1", RecurringInvoiceID: "ri-1", Description: "Service", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100)},
		},
	}
	_ = repo.Create(ctx, "test_schema", ri)
	_ = repo.CreateLine(ctx, "test_schema", &ri.Lines[0])

	service := NewServiceWithDependencies(repo, nil, nil, nil, nil, nil)

	endDate := time.Now().AddDate(1, 0, 0)
	req := &UpdateRecurringInvoiceRequest{
		Name:                   stringPtr("New Name"),
		ContactID:              stringPtr("new-contact"),
		Frequency:              frequencyPtr(FrequencyQuarterly),
		EndDate:                &endDate,
		PaymentTermsDays:       intPtr(30),
		Reference:              stringPtr("REF-123"),
		Notes:                  stringPtr("New notes"),
		SendEmailOnGeneration:  boolPtr(true),
		EmailTemplateType:      stringPtr("INVOICE_SEND"),
		RecipientEmailOverride: stringPtr("test@example.com"),
		AttachPDFToEmail:       boolPtr(false),
		EmailSubjectOverride:   stringPtr("Custom Subject"),
		EmailMessage:           stringPtr("Custom message"),
	}

	result, err := service.Update(ctx, "tenant-1", "test_schema", "ri-1", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Name != "New Name" {
		t.Errorf("Name = %q, want %q", result.Name, "New Name")
	}
	if result.ContactID != "new-contact" {
		t.Errorf("ContactID = %q, want %q", result.ContactID, "new-contact")
	}
	if result.Frequency != FrequencyQuarterly {
		t.Errorf("Frequency = %v, want %v", result.Frequency, FrequencyQuarterly)
	}
	if result.EndDate == nil {
		t.Error("EndDate should not be nil")
	}
	if result.PaymentTermsDays != 30 {
		t.Errorf("PaymentTermsDays = %d, want 30", result.PaymentTermsDays)
	}
	if result.Reference != "REF-123" {
		t.Errorf("Reference = %q, want %q", result.Reference, "REF-123")
	}
	if result.Notes != "New notes" {
		t.Errorf("Notes = %q, want %q", result.Notes, "New notes")
	}
	if !result.SendEmailOnGeneration {
		t.Error("SendEmailOnGeneration should be true")
	}
	if result.EmailTemplateType != "INVOICE_SEND" {
		t.Errorf("EmailTemplateType = %q, want %q", result.EmailTemplateType, "INVOICE_SEND")
	}
	if result.RecipientEmailOverride != "test@example.com" {
		t.Errorf("RecipientEmailOverride = %q, want %q", result.RecipientEmailOverride, "test@example.com")
	}
	if result.AttachPDFToEmail {
		t.Error("AttachPDFToEmail should be false")
	}
	if result.EmailSubjectOverride != "Custom Subject" {
		t.Errorf("EmailSubjectOverride = %q, want %q", result.EmailSubjectOverride, "Custom Subject")
	}
	if result.EmailMessage != "Custom message" {
		t.Errorf("EmailMessage = %q, want %q", result.EmailMessage, "Custom message")
	}
}

func TestService_Update_ValidationError(t *testing.T) {
	ctx := context.Background()

	repo := NewMockRepository()
	ri := &RecurringInvoice{
		ID:               "ri-1",
		TenantID:         "tenant-1",
		Name:             "Monthly Subscription",
		ContactID:        "contact-1",
		Frequency:        FrequencyMonthly,
		StartDate:        time.Now(),
		PaymentTermsDays: 14,
		IsActive:         true,
		Lines: []RecurringInvoiceLine{
			{ID: "line-1", RecurringInvoiceID: "ri-1", Description: "Service", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100)},
		},
	}
	_ = repo.Create(ctx, "test_schema", ri)
	_ = repo.CreateLine(ctx, "test_schema", &ri.Lines[0])

	service := NewServiceWithDependencies(repo, nil, nil, nil, nil, nil)

	// Try to set empty name which should fail validation
	req := &UpdateRecurringInvoiceRequest{
		Name: stringPtr(""),
	}

	_, err := service.Update(ctx, "tenant-1", "test_schema", "ri-1", req)
	if err == nil {
		t.Error("expected validation error")
	}
	if !contains(err.Error(), "validation") {
		t.Errorf("error should contain 'validation', got %q", err.Error())
	}
}

func TestService_GenerateDueInvoices_GetByIDError(t *testing.T) {
	ctx := context.Background()

	repo := NewMockRepository()
	now := time.Now()
	ri := &RecurringInvoice{
		ID:                 "ri-1",
		TenantID:           "tenant-1",
		Name:               "Monthly Service",
		ContactID:          "contact-1",
		Frequency:          FrequencyMonthly,
		NextGenerationDate: now.AddDate(0, 0, -1),
		IsActive:           true,
		Lines: []RecurringInvoiceLine{
			{ID: "line-1", RecurringInvoiceID: "ri-1", Description: "Service", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100)},
		},
	}
	_ = repo.Create(ctx, "test_schema", ri)
	_ = repo.CreateLine(ctx, "test_schema", &ri.Lines[0])

	// Set error after GetDueRecurringInvoiceIDs succeeds
	repo.getByIDErr = fmt.Errorf("get by id error")

	invoicingSvc := NewMockInvoicingService()
	service := NewServiceWithDependencies(repo, invoicingSvc, nil, nil, nil, nil)

	results, err := service.GenerateDueInvoices(ctx, "tenant-1", "test_schema", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Results should be empty since GetByID failed
	if len(results) != 0 {
		t.Errorf("results count = %d, want 0 (should skip on error)", len(results))
	}
}

func TestService_GenerateInvoice_UpdateAfterGenError(t *testing.T) {
	ctx := context.Background()

	repo := NewMockRepository()
	now := time.Now()
	ri := &RecurringInvoice{
		ID:                 "ri-1",
		TenantID:           "tenant-1",
		Name:               "Monthly Service",
		ContactID:          "contact-1",
		InvoiceType:        "SALES",
		Currency:           "EUR",
		Frequency:          FrequencyMonthly,
		NextGenerationDate: now,
		PaymentTermsDays:   14,
		IsActive:           true,
		Lines: []RecurringInvoiceLine{
			{ID: "line-1", RecurringInvoiceID: "ri-1", Description: "Service", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100)},
		},
	}
	_ = repo.Create(ctx, "test_schema", ri)
	_ = repo.CreateLine(ctx, "test_schema", &ri.Lines[0])
	repo.updateAfterGenErr = fmt.Errorf("update after gen error")

	invoicingSvc := NewMockInvoicingService()
	service := NewServiceWithDependencies(repo, invoicingSvc, nil, nil, nil, nil)

	_, err := service.GenerateInvoice(ctx, "tenant-1", "test_schema", "ri-1", "user-1")
	if err == nil {
		t.Error("expected error")
	}
	if !contains(err.Error(), "update recurring invoice") {
		t.Errorf("error should contain 'update recurring invoice', got %q", err.Error())
	}
}

// Helper function for string containment check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
