package email

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTemplateTypeConstants(t *testing.T) {
	tests := []struct {
		templateType TemplateType
		expected     string
	}{
		{TemplateInvoiceSend, "INVOICE_SEND"},
		{TemplatePaymentReceipt, "PAYMENT_RECEIPT"},
		{TemplateOverdueReminder, "OVERDUE_REMINDER"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if string(tt.templateType) != tt.expected {
				t.Errorf("TemplateType = %q, want %q", tt.templateType, tt.expected)
			}
		})
	}
}

func TestEmailStatusConstants(t *testing.T) {
	tests := []struct {
		status   EmailStatus
		expected string
	}{
		{StatusPending, "PENDING"},
		{StatusSent, "SENT"},
		{StatusFailed, "FAILED"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("EmailStatus = %q, want %q", tt.status, tt.expected)
			}
		})
	}
}

func TestSMTPConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      SMTPConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid config",
			config: SMTPConfig{
				Host:      "smtp.example.com",
				Port:      587,
				FromEmail: "noreply@example.com",
			},
			expectError: false,
		},
		{
			name: "Missing host",
			config: SMTPConfig{
				Host:      "",
				Port:      587,
				FromEmail: "noreply@example.com",
			},
			expectError: true,
			errorMsg:    "SMTP host is required",
		},
		{
			name: "Zero port",
			config: SMTPConfig{
				Host:      "smtp.example.com",
				Port:      0,
				FromEmail: "noreply@example.com",
			},
			expectError: true,
			errorMsg:    "invalid SMTP port",
		},
		{
			name: "Negative port",
			config: SMTPConfig{
				Host:      "smtp.example.com",
				Port:      -1,
				FromEmail: "noreply@example.com",
			},
			expectError: true,
			errorMsg:    "invalid SMTP port",
		},
		{
			name: "Port too high",
			config: SMTPConfig{
				Host:      "smtp.example.com",
				Port:      65536,
				FromEmail: "noreply@example.com",
			},
			expectError: true,
			errorMsg:    "invalid SMTP port",
		},
		{
			name: "Missing from email",
			config: SMTPConfig{
				Host:      "smtp.example.com",
				Port:      587,
				FromEmail: "",
			},
			expectError: true,
			errorMsg:    "from email is required",
		},
		{
			name: "Valid port 25",
			config: SMTPConfig{
				Host:      "smtp.example.com",
				Port:      25,
				FromEmail: "noreply@example.com",
			},
			expectError: false,
		},
		{
			name: "Valid port 465",
			config: SMTPConfig{
				Host:      "smtp.example.com",
				Port:      465,
				FromEmail: "noreply@example.com",
			},
			expectError: false,
		},
		{
			name: "Valid max port",
			config: SMTPConfig{
				Host:      "smtp.example.com",
				Port:      65535,
				FromEmail: "noreply@example.com",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.errorMsg)
				} else if err.Error() != tt.errorMsg {
					t.Errorf("Validate() error = %q, want %q", err.Error(), tt.errorMsg)
				}
			} else if err != nil {
				t.Errorf("Validate() unexpected error: %v", err)
			}
		})
	}
}

func TestSMTPConfig_IsConfigured(t *testing.T) {
	tests := []struct {
		name     string
		config   SMTPConfig
		expected bool
	}{
		{
			name: "Fully configured",
			config: SMTPConfig{
				Host:      "smtp.example.com",
				Port:      587,
				FromEmail: "noreply@example.com",
			},
			expected: true,
		},
		{
			name: "Missing host",
			config: SMTPConfig{
				Host:      "",
				Port:      587,
				FromEmail: "noreply@example.com",
			},
			expected: false,
		},
		{
			name: "Zero port",
			config: SMTPConfig{
				Host:      "smtp.example.com",
				Port:      0,
				FromEmail: "noreply@example.com",
			},
			expected: false,
		},
		{
			name: "Missing from email",
			config: SMTPConfig{
				Host:      "smtp.example.com",
				Port:      587,
				FromEmail: "",
			},
			expected: false,
		},
		{
			name:     "Empty config",
			config:   SMTPConfig{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.IsConfigured()
			if result != tt.expected {
				t.Errorf("IsConfigured() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSendInvoiceRequest_Validate(t *testing.T) {
	tests := []struct {
		name        string
		request     SendInvoiceRequest
		expectError bool
	}{
		{
			name: "Valid request",
			request: SendInvoiceRequest{
				RecipientEmail: "customer@example.com",
			},
			expectError: false,
		},
		{
			name: "Valid request with all fields",
			request: SendInvoiceRequest{
				RecipientEmail: "customer@example.com",
				RecipientName:  "John Doe",
				Subject:        "Your Invoice",
				Message:        "Please find attached",
				AttachPDF:      true,
			},
			expectError: false,
		},
		{
			name: "Missing recipient email",
			request: SendInvoiceRequest{
				RecipientEmail: "",
				RecipientName:  "John Doe",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()
			if tt.expectError && err == nil {
				t.Error("Validate() expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Validate() unexpected error: %v", err)
			}
		})
	}
}

func TestSendPaymentReceiptRequest_Validate(t *testing.T) {
	tests := []struct {
		name        string
		request     SendPaymentReceiptRequest
		expectError bool
	}{
		{
			name: "Valid request",
			request: SendPaymentReceiptRequest{
				RecipientEmail: "customer@example.com",
			},
			expectError: false,
		},
		{
			name: "Missing recipient email",
			request: SendPaymentReceiptRequest{
				RecipientEmail: "",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()
			if tt.expectError && err == nil {
				t.Error("Validate() expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Validate() unexpected error: %v", err)
			}
		})
	}
}

func TestDefaultTemplates(t *testing.T) {
	templates := DefaultTemplates()

	expectedTypes := []TemplateType{
		TemplateInvoiceSend,
		TemplatePaymentReceipt,
		TemplateOverdueReminder,
	}

	for _, tt := range expectedTypes {
		t.Run(string(tt), func(t *testing.T) {
			template, exists := templates[tt]
			if !exists {
				t.Fatalf("Default template for %q not found", tt)
			}
			if template.TemplateType != tt {
				t.Errorf("Template type = %q, want %q", template.TemplateType, tt)
			}
			if template.Subject == "" {
				t.Error("Template subject is empty")
			}
			if template.BodyHTML == "" {
				t.Error("Template body HTML is empty")
			}
			if !template.IsActive {
				t.Error("Default template should be active")
			}
		})
	}

	if len(templates) != len(expectedTypes) {
		t.Errorf("DefaultTemplates() returned %d templates, want %d", len(templates), len(expectedTypes))
	}
}

func TestDefaultTemplates_ContainPlaceholders(t *testing.T) {
	templates := DefaultTemplates()

	// Invoice template should have invoice-specific placeholders
	invoiceTemplate := templates[TemplateInvoiceSend]
	invoicePlaceholders := []string{"{{.InvoiceNumber}}", "{{.ContactName}}", "{{.TotalAmount}}", "{{.DueDate}}"}
	for _, ph := range invoicePlaceholders {
		if !containsString(invoiceTemplate.BodyHTML, ph) && !containsString(invoiceTemplate.Subject, ph) {
			t.Errorf("Invoice template missing placeholder: %s", ph)
		}
	}

	// Payment receipt should have payment-specific placeholders
	paymentTemplate := templates[TemplatePaymentReceipt]
	paymentPlaceholders := []string{"{{.Amount}}", "{{.PaymentDate}}", "{{.Reference}}"}
	for _, ph := range paymentPlaceholders {
		if !containsString(paymentTemplate.BodyHTML, ph) {
			t.Errorf("Payment template missing placeholder: %s", ph)
		}
	}

	// Overdue template should have overdue-specific placeholders
	overdueTemplate := templates[TemplateOverdueReminder]
	overduePlaceholders := []string{"{{.InvoiceNumber}}", "{{.DaysOverdue}}"}
	for _, ph := range overduePlaceholders {
		if !containsString(overdueTemplate.BodyHTML, ph) && !containsString(overdueTemplate.Subject, ph) {
			t.Errorf("Overdue template missing placeholder: %s", ph)
		}
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestSMTPConfig_JSONSerialization(t *testing.T) {
	config := SMTPConfig{
		Host:      "smtp.example.com",
		Port:      587,
		Username:  "user@example.com",
		Password:  "secret",
		FromEmail: "noreply@example.com",
		FromName:  "Open Accounting",
		UseTLS:    true,
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal SMTPConfig: %v", err)
	}

	var parsed SMTPConfig
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal SMTPConfig: %v", err)
	}

	if parsed.Host != config.Host {
		t.Errorf("Host = %q, want %q", parsed.Host, config.Host)
	}
	if parsed.Port != config.Port {
		t.Errorf("Port = %d, want %d", parsed.Port, config.Port)
	}
	if parsed.UseTLS != config.UseTLS {
		t.Errorf("UseTLS = %v, want %v", parsed.UseTLS, config.UseTLS)
	}
}

func TestEmailTemplate_JSONSerialization(t *testing.T) {
	template := EmailTemplate{
		ID:           "template-123",
		TenantID:     "tenant-1",
		TemplateType: TemplateInvoiceSend,
		Subject:      "Invoice {{.InvoiceNumber}}",
		BodyHTML:     "<h1>Invoice</h1>",
		BodyText:     "Invoice",
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	data, err := json.Marshal(template)
	if err != nil {
		t.Fatalf("Failed to marshal EmailTemplate: %v", err)
	}

	var parsed EmailTemplate
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal EmailTemplate: %v", err)
	}

	if parsed.ID != template.ID {
		t.Errorf("ID = %q, want %q", parsed.ID, template.ID)
	}
	if parsed.TemplateType != template.TemplateType {
		t.Errorf("TemplateType = %q, want %q", parsed.TemplateType, template.TemplateType)
	}
}

func TestEmailLog_JSONSerialization(t *testing.T) {
	sentAt := time.Now()
	log := EmailLog{
		ID:             "log-123",
		TenantID:       "tenant-1",
		EmailType:      "INVOICE_SEND",
		RecipientEmail: "customer@example.com",
		RecipientName:  "John Doe",
		Subject:        "Invoice INV-001",
		Status:         StatusSent,
		SentAt:         &sentAt,
		RelatedID:      "invoice-123",
		CreatedAt:      time.Now(),
	}

	data, err := json.Marshal(log)
	if err != nil {
		t.Fatalf("Failed to marshal EmailLog: %v", err)
	}

	var parsed EmailLog
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal EmailLog: %v", err)
	}

	if parsed.ID != log.ID {
		t.Errorf("ID = %q, want %q", parsed.ID, log.ID)
	}
	if parsed.Status != log.Status {
		t.Errorf("Status = %q, want %q", parsed.Status, log.Status)
	}
}

func TestEmailSentResponse_JSONSerialization(t *testing.T) {
	response := EmailSentResponse{
		Success: true,
		LogID:   "log-123",
		Message: "Email sent successfully",
	}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal EmailSentResponse: %v", err)
	}

	var parsed EmailSentResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal EmailSentResponse: %v", err)
	}

	if parsed.Success != response.Success {
		t.Errorf("Success = %v, want %v", parsed.Success, response.Success)
	}
	if parsed.LogID != response.LogID {
		t.Errorf("LogID = %q, want %q", parsed.LogID, response.LogID)
	}
}

func TestTestSMTPResponse_JSONSerialization(t *testing.T) {
	response := TestSMTPResponse{
		Success: false,
		Message: "Connection failed",
	}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal TestSMTPResponse: %v", err)
	}

	var parsed TestSMTPResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal TestSMTPResponse: %v", err)
	}

	if parsed.Success != response.Success {
		t.Errorf("Success = %v, want %v", parsed.Success, response.Success)
	}
	if parsed.Message != response.Message {
		t.Errorf("Message = %q, want %q", parsed.Message, response.Message)
	}
}

func TestTemplateData_Fields(t *testing.T) {
	data := TemplateData{
		CompanyName:   "Acme Corp",
		ContactName:   "John Doe",
		Message:       "Custom message",
		InvoiceNumber: "INV-2025-001",
		TotalAmount:   "1000.00",
		Currency:      "EUR",
		DueDate:       "2025-02-01",
		IssueDate:     "2025-01-01",
		DaysOverdue:   15,
		Amount:        "500.00",
		PaymentDate:   "2025-01-15",
		Reference:     "PAY-001",
	}

	if data.CompanyName != "Acme Corp" {
		t.Errorf("CompanyName = %q, want %q", data.CompanyName, "Acme Corp")
	}
	if data.DaysOverdue != 15 {
		t.Errorf("DaysOverdue = %d, want %d", data.DaysOverdue, 15)
	}
}

func TestUpdateTemplateRequest_JSONSerialization(t *testing.T) {
	req := UpdateTemplateRequest{
		Subject:  "New Subject",
		BodyHTML: "<h1>Updated</h1>",
		BodyText: "Updated",
		IsActive: true,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal UpdateTemplateRequest: %v", err)
	}

	var parsed UpdateTemplateRequest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal UpdateTemplateRequest: %v", err)
	}

	if parsed.Subject != req.Subject {
		t.Errorf("Subject = %q, want %q", parsed.Subject, req.Subject)
	}
}

func TestUpdateSMTPConfigRequest_JSONSerialization(t *testing.T) {
	req := UpdateSMTPConfigRequest{
		Host:      "smtp.example.com",
		Port:      587,
		Username:  "user",
		Password:  "pass",
		FromEmail: "from@example.com",
		FromName:  "Sender",
		UseTLS:    true,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal UpdateSMTPConfigRequest: %v", err)
	}

	var parsed UpdateSMTPConfigRequest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal UpdateSMTPConfigRequest: %v", err)
	}

	if parsed.Host != req.Host {
		t.Errorf("Host = %q, want %q", parsed.Host, req.Host)
	}
	if parsed.Port != req.Port {
		t.Errorf("Port = %d, want %d", parsed.Port, req.Port)
	}
}

func TestTestSMTPRequest_JSONSerialization(t *testing.T) {
	req := TestSMTPRequest{
		RecipientEmail: "test@example.com",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal TestSMTPRequest: %v", err)
	}

	var parsed TestSMTPRequest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal TestSMTPRequest: %v", err)
	}

	if parsed.RecipientEmail != req.RecipientEmail {
		t.Errorf("RecipientEmail = %q, want %q", parsed.RecipientEmail, req.RecipientEmail)
	}
}

func TestNewService(t *testing.T) {
	service := NewService(nil)
	if service == nil {
		t.Error("NewService() returned nil")
	}
}

func TestService_RenderTemplate(t *testing.T) {
	service := NewService(nil)

	tests := []struct {
		name            string
		template        EmailTemplate
		data            TemplateData
		expectError     bool
		expectedSubject string
		checkBodyHTML   func(string) bool
	}{
		{
			name: "Invoice template",
			template: EmailTemplate{
				TemplateType: TemplateInvoiceSend,
				Subject:      "Invoice {{.InvoiceNumber}} from {{.CompanyName}}",
				BodyHTML:     "<p>Dear {{.ContactName}}, please find invoice {{.InvoiceNumber}} for {{.TotalAmount}} {{.Currency}}.</p>",
				BodyText:     "Dear {{.ContactName}}, please find invoice {{.InvoiceNumber}}.",
			},
			data: TemplateData{
				CompanyName:   "Acme Corp",
				ContactName:   "John Doe",
				InvoiceNumber: "INV-001",
				TotalAmount:   "1000.00",
				Currency:      "EUR",
			},
			expectError:     false,
			expectedSubject: "Invoice INV-001 from Acme Corp",
			checkBodyHTML: func(body string) bool {
				return containsString(body, "John Doe") && containsString(body, "INV-001") && containsString(body, "1000.00")
			},
		},
		{
			name: "Payment receipt template",
			template: EmailTemplate{
				TemplateType: TemplatePaymentReceipt,
				Subject:      "Payment Receipt from {{.CompanyName}}",
				BodyHTML:     "<p>Payment of {{.Amount}} {{.Currency}} received on {{.PaymentDate}}.</p>",
			},
			data: TemplateData{
				CompanyName: "Acme Corp",
				Amount:      "500.00",
				Currency:    "EUR",
				PaymentDate: "2025-01-15",
			},
			expectError:     false,
			expectedSubject: "Payment Receipt from Acme Corp",
			checkBodyHTML: func(body string) bool {
				return containsString(body, "500.00") && containsString(body, "2025-01-15")
			},
		},
		{
			name: "Overdue reminder template",
			template: EmailTemplate{
				TemplateType: TemplateOverdueReminder,
				Subject:      "Overdue: {{.InvoiceNumber}}",
				BodyHTML:     "<p>Invoice {{.InvoiceNumber}} is {{.DaysOverdue}} days overdue.</p>",
			},
			data: TemplateData{
				InvoiceNumber: "INV-002",
				DaysOverdue:   15,
			},
			expectError:     false,
			expectedSubject: "Overdue: INV-002",
			checkBodyHTML: func(body string) bool {
				return containsString(body, "INV-002") && containsString(body, "15")
			},
		},
		{
			name: "Template with custom message",
			template: EmailTemplate{
				Subject:  "Subject",
				BodyHTML: "<p>{{.Message}}</p>",
			},
			data: TemplateData{
				Message: "This is a custom message.",
			},
			expectError:     false,
			expectedSubject: "Subject",
			checkBodyHTML: func(body string) bool {
				return containsString(body, "This is a custom message.")
			},
		},
		{
			name: "Invalid subject template",
			template: EmailTemplate{
				Subject:  "{{.InvalidField",
				BodyHTML: "<p>Body</p>",
			},
			data:        TemplateData{},
			expectError: true,
		},
		{
			name: "Invalid HTML template",
			template: EmailTemplate{
				Subject:  "Subject",
				BodyHTML: "<p>{{.InvalidField</p>",
			},
			data:        TemplateData{},
			expectError: true,
		},
		{
			name: "Invalid text template",
			template: EmailTemplate{
				Subject:  "Subject",
				BodyHTML: "<p>Valid</p>",
				BodyText: "{{.InvalidField",
			},
			data:        TemplateData{},
			expectError: true,
		},
		{
			name: "Empty body text is valid",
			template: EmailTemplate{
				Subject:  "Subject",
				BodyHTML: "<p>HTML only</p>",
				BodyText: "",
			},
			data:            TemplateData{},
			expectError:     false,
			expectedSubject: "Subject",
			checkBodyHTML: func(body string) bool {
				return containsString(body, "HTML only")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subject, bodyHTML, bodyText, err := service.RenderTemplate(&tt.template, &tt.data)

			if tt.expectError {
				if err == nil {
					t.Error("RenderTemplate() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("RenderTemplate() unexpected error: %v", err)
				return
			}

			if subject != tt.expectedSubject {
				t.Errorf("Subject = %q, want %q", subject, tt.expectedSubject)
			}

			if tt.checkBodyHTML != nil && !tt.checkBodyHTML(bodyHTML) {
				t.Errorf("BodyHTML validation failed: %s", bodyHTML)
			}

			// If template has body text, verify it was rendered
			if tt.template.BodyText != "" && bodyText == "" {
				t.Error("BodyText should not be empty when template has body_text")
			}
		})
	}
}

func TestService_RenderTemplate_DefaultTemplates(t *testing.T) {
	service := NewService(nil)
	templates := DefaultTemplates()

	// Test that all default templates can be rendered
	data := TemplateData{
		CompanyName:   "Test Company",
		ContactName:   "Test Contact",
		InvoiceNumber: "INV-TEST-001",
		TotalAmount:   "1234.56",
		Currency:      "EUR",
		DueDate:       "2025-02-28",
		IssueDate:     "2025-01-28",
		DaysOverdue:   30,
		Amount:        "999.99",
		PaymentDate:   "2025-01-20",
		Reference:     "PAY-REF-001",
		Message:       "Thank you for your business",
	}

	for templateType, template := range templates {
		t.Run(string(templateType), func(t *testing.T) {
			subject, bodyHTML, _, err := service.RenderTemplate(&template, &data)
			if err != nil {
				t.Errorf("Failed to render %s template: %v", templateType, err)
			}
			if subject == "" {
				t.Error("Rendered subject is empty")
			}
			if bodyHTML == "" {
				t.Error("Rendered bodyHTML is empty")
			}
		})
	}
}

func TestAttachment_Fields(t *testing.T) {
	attachment := Attachment{
		Filename:    "invoice.pdf",
		Content:     []byte("PDF content"),
		ContentType: "application/pdf",
	}

	if attachment.Filename != "invoice.pdf" {
		t.Errorf("Filename = %q, want %q", attachment.Filename, "invoice.pdf")
	}
	if len(attachment.Content) != 11 {
		t.Errorf("Content length = %d, want %d", len(attachment.Content), 11)
	}
	if attachment.ContentType != "application/pdf" {
		t.Errorf("ContentType = %q, want %q", attachment.ContentType, "application/pdf")
	}
}

func TestInvoiceEmailData_Fields(t *testing.T) {
	now := time.Now()
	data := InvoiceEmailData{
		InvoiceID:     "inv-123",
		InvoiceNumber: "INV-2025-001",
		ContactName:   "John Doe",
		ContactEmail:  "john@example.com",
		TotalAmount:   "1000.00",
		Currency:      "EUR",
		IssueDate:     now,
		DueDate:       now.AddDate(0, 0, 30),
		CompanyName:   "Acme Corp",
	}

	if data.InvoiceID != "inv-123" {
		t.Errorf("InvoiceID = %q, want %q", data.InvoiceID, "inv-123")
	}
	if data.InvoiceNumber != "INV-2025-001" {
		t.Errorf("InvoiceNumber = %q, want %q", data.InvoiceNumber, "INV-2025-001")
	}
	if data.ContactEmail != "john@example.com" {
		t.Errorf("ContactEmail = %q, want %q", data.ContactEmail, "john@example.com")
	}
}

func TestPaymentEmailData_Fields(t *testing.T) {
	now := time.Now()
	data := PaymentEmailData{
		PaymentID:    "pay-123",
		ContactName:  "Jane Doe",
		ContactEmail: "jane@example.com",
		Amount:       "500.00",
		Currency:     "EUR",
		PaymentDate:  now,
		Reference:    "PAY-REF-001",
		CompanyName:  "Acme Corp",
	}

	if data.PaymentID != "pay-123" {
		t.Errorf("PaymentID = %q, want %q", data.PaymentID, "pay-123")
	}
	if data.Reference != "PAY-REF-001" {
		t.Errorf("Reference = %q, want %q", data.Reference, "PAY-REF-001")
	}
}
