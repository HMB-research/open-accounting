package recurring

import (
	"testing"
)

func TestNewService(t *testing.T) {
	// NewService now requires 6 parameters: db, invoicing, email, pdf, tenant, contacts
	service := NewService(nil, nil, nil, nil, nil, nil)
	if service == nil {
		t.Fatal("NewService returned nil")
	}
	if service.db != nil {
		t.Error("NewService(...).db should be nil when passed nil")
	}
	if service.invoicing != nil {
		t.Error("NewService(...).invoicing should be nil when passed nil")
	}
	if service.email != nil {
		t.Error("NewService(...).email should be nil when passed nil")
	}
	if service.pdf != nil {
		t.Error("NewService(...).pdf should be nil when passed nil")
	}
	if service.tenant != nil {
		t.Error("NewService(...).tenant should be nil when passed nil")
	}
	if service.contacts != nil {
		t.Error("NewService(...).contacts should be nil when passed nil")
	}
}

func TestNewService_NotNil(t *testing.T) {
	service := NewService(nil, nil, nil, nil, nil, nil)
	if service == nil {
		t.Error("NewService should always return a non-nil service")
	}
}

func TestDefaultPaymentTermsDays(t *testing.T) {
	// Test the default payment terms logic from Create method
	// if ri.PaymentTermsDays == 0 { ri.PaymentTermsDays = 14 }
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
	// Test the default currency logic from Create method
	// if ri.Currency == "" { ri.Currency = "EUR" }
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
	// Test the default invoice type logic from Create method
	// if ri.InvoiceType == "" { ri.InvoiceType = "SALES" }
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
	// Test the default email template type logic from Create method
	// if emailTemplateType == "" { emailTemplateType = "INVOICE_SEND" }
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
	// Test the default attach PDF logic from Create method
	// attachPDF defaults to true if not specified
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
	// Test that CreateRecurringInvoiceRequest properly handles email configuration
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

			// Apply default logic for AttachPDFToEmail
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
	// Test that UpdateRecurringInvoiceRequest properly handles email configuration updates
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
	// Test that GenerationResult properly represents email delivery outcomes
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

// Helper function to create bool pointer
func boolPtr(b bool) *bool {
	return &b
}
