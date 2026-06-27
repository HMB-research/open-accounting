package pdf

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/orders"
	"github.com/HMB-research/open-accounting/internal/payroll"
	"github.com/HMB-research/open-accounting/internal/quotes"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

func TestNewService(t *testing.T) {
	service := NewService()
	if service == nil {
		t.Fatal("NewService() returned nil")
	}
}

func TestDefaultPDFSettings(t *testing.T) {
	settings := DefaultPDFSettings()

	if settings.PrimaryColor == "" {
		t.Error("PrimaryColor should not be empty")
	}
	if settings.PrimaryColor != "#1d4ed8" {
		t.Errorf("PrimaryColor = %q, want %q", settings.PrimaryColor, "#1d4ed8")
	}

	if settings.FooterText == "" {
		t.Error("FooterText should not be empty")
	}
	if settings.FooterText != "Thank you for your business" {
		t.Errorf("FooterText = %q, want %q", settings.FooterText, "Thank you for your business")
	}

	if settings.InvoiceTerms == "" {
		t.Error("InvoiceTerms should not be empty")
	}

	// BankDetails should be empty by default
	if settings.BankDetails != "" {
		t.Errorf("BankDetails should be empty by default, got %q", settings.BankDetails)
	}
}

func TestFormatMoney(t *testing.T) {
	tests := []struct {
		name     string
		amount   decimal.Decimal
		currency string
		expected string
	}{
		{"Simple EUR", decimal.NewFromFloat(100.00), "EUR", "EUR 100.00"},
		{"With cents", decimal.NewFromFloat(1234.56), "EUR", "EUR 1234.56"},
		{"Zero", decimal.Zero, "USD", "USD 0.00"},
		{"Negative", decimal.NewFromFloat(-50.00), "GBP", "GBP -50.00"},
		{"Large amount", decimal.NewFromFloat(1000000.99), "EUR", "EUR 1000000.99"},
		{"Round down", decimal.NewFromFloat(99.999), "EUR", "EUR 100.00"},
		{"Many decimals", decimal.NewFromFloat(123.456789), "EUR", "EUR 123.46"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatMoney(tt.amount, tt.currency)
			if result != tt.expected {
				t.Errorf("formatMoney(%v, %q) = %q, want %q", tt.amount, tt.currency, result, tt.expected)
			}
		})
	}
}

func TestFormatDecimal(t *testing.T) {
	tests := []struct {
		name      string
		value     decimal.Decimal
		precision int32
		expected  string
	}{
		{"Two decimals", decimal.NewFromFloat(123.456), 2, "123.46"},
		{"Zero decimals", decimal.NewFromFloat(123.456), 0, "123"},
		{"Four decimals", decimal.NewFromFloat(1.23456789), 4, "1.2346"},
		{"Zero value", decimal.Zero, 2, "0.00"},
		{"Negative", decimal.NewFromFloat(-99.99), 2, "-99.99"},
		{"Integer as decimal", decimal.NewFromInt(100), 2, "100.00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDecimal(tt.value, tt.precision)
			if result != tt.expected {
				t.Errorf("formatDecimal(%v, %d) = %q, want %q", tt.value, tt.precision, result, tt.expected)
			}
		})
	}
}

func TestFormatDiscount(t *testing.T) {
	tests := []struct {
		name     string
		discount decimal.Decimal
		expected string
	}{
		{"Zero discount", decimal.Zero, "-"},
		{"10% discount", decimal.NewFromFloat(10), "10%"},
		{"5% discount", decimal.NewFromFloat(5), "5%"},
		{"25% discount", decimal.NewFromFloat(25), "25%"},
		{"100% discount", decimal.NewFromFloat(100), "100%"},
		{"Fractional discount", decimal.NewFromFloat(7.5), "8%"}, // Rounds to nearest integer
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDiscount(tt.discount)
			if result != tt.expected {
				t.Errorf("formatDiscount(%v) = %q, want %q", tt.discount, result, tt.expected)
			}
		})
	}
}

func TestTruncateText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxLen   int
		expected string
	}{
		{"Short text", "Hello", 10, "Hello"},
		{"Exact length", "Hello", 5, "Hello"},
		{"Needs truncation", "Hello World", 8, "Hello..."},
		{"Very long text", "This is a very long description that needs to be truncated", 20, "This is a very lo..."},
		{"Empty text", "", 10, ""},
		{"Max length 3", "Hello", 3, "..."},
		{"Max length 4", "Hello", 4, "H..."},
		// Note: truncateText works on bytes, not runes, so multi-byte chars may be split
		{"Unicode text short", "Tere", 10, "Tere"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateText(tt.text, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncateText(%q, %d) = %q, want %q", tt.text, tt.maxLen, result, tt.expected)
			}
		})
	}
}

func TestTruncateText_LengthConstraint(t *testing.T) {
	tests := []struct {
		text   string
		maxLen int
	}{
		{"Short", 10},
		{"This is a longer text", 15},
		{"Very very very long text that definitely needs truncation", 25},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			result := truncateText(tt.text, tt.maxLen)
			if len(result) > tt.maxLen {
				t.Errorf("truncateText(%q, %d) returned %q with length %d, exceeds max %d",
					tt.text, tt.maxLen, result, len(result), tt.maxLen)
			}
		})
	}
}

func TestPDFSettings_Fields(t *testing.T) {
	settings := PDFSettings{
		PrimaryColor: "#ff0000",
		FooterText:   "Custom footer",
		BankDetails:  "IBAN: EE123456789",
		InvoiceTerms: "Net 30 days",
	}

	if settings.PrimaryColor != "#ff0000" {
		t.Errorf("PrimaryColor = %q, want %q", settings.PrimaryColor, "#ff0000")
	}
	if settings.FooterText != "Custom footer" {
		t.Errorf("FooterText = %q, want %q", settings.FooterText, "Custom footer")
	}
	if settings.BankDetails != "IBAN: EE123456789" {
		t.Errorf("BankDetails = %q, want %q", settings.BankDetails, "IBAN: EE123456789")
	}
	if settings.InvoiceTerms != "Net 30 days" {
		t.Errorf("InvoiceTerms = %q, want %q", settings.InvoiceTerms, "Net 30 days")
	}
}

func TestFormatMoney_Currencies(t *testing.T) {
	amount := decimal.NewFromFloat(1000.00)
	currencies := []string{"EUR", "USD", "GBP", "SEK", "NOK", "DKK", "CHF", "JPY"}

	for _, currency := range currencies {
		t.Run(currency, func(t *testing.T) {
			result := formatMoney(amount, currency)
			if len(result) < len(currency)+1 {
				t.Errorf("formatMoney result %q too short for currency %q", result, currency)
			}
			// Should start with currency code
			if result[:len(currency)] != currency {
				t.Errorf("formatMoney result %q should start with %q", result, currency)
			}
		})
	}
}

func TestFormatDecimal_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		value     decimal.Decimal
		precision int32
	}{
		{"Very small", decimal.NewFromFloat(0.0001), 4},
		{"Very large", decimal.NewFromFloat(999999999.99), 2},
		{"Negative small", decimal.NewFromFloat(-0.01), 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDecimal(tt.value, tt.precision)
			if result == "" {
				t.Error("formatDecimal should not return empty string")
			}
		})
	}
}

func TestPDFSettingsFromTenant(t *testing.T) {
	svc := NewService()

	t.Run("returns defaults when tenant has no settings", func(t *testing.T) {
		tenant := &tenant.Tenant{
			Name:     "Test Company",
			Settings: tenant.TenantSettings{},
		}

		settings := svc.PDFSettingsFromTenant(tenant)

		assert.Equal(t, "#1d4ed8", settings.PrimaryColor)
		assert.Equal(t, "Thank you for your business", settings.FooterText)
		assert.Empty(t, settings.BankDetails)
		assert.Equal(t, "Payment due within 14 days of invoice date.", settings.InvoiceTerms)
	})

	t.Run("uses tenant settings when provided", func(t *testing.T) {
		tenant := &tenant.Tenant{
			Name: "Custom Company",
			Settings: tenant.TenantSettings{
				PDFPrimaryColor: "#ff5500",
				PDFFooterText:   "Custom footer text",
				BankDetails:     "IBAN: EE123456\nSWIFT: ABCD",
				InvoiceTerms:    "Net 30 days",
			},
		}

		settings := svc.PDFSettingsFromTenant(tenant)

		assert.Equal(t, "#ff5500", settings.PrimaryColor)
		assert.Equal(t, "Custom footer text", settings.FooterText)
		assert.Equal(t, "IBAN: EE123456\nSWIFT: ABCD", settings.BankDetails)
		assert.Equal(t, "Net 30 days", settings.InvoiceTerms)
	})

	t.Run("partial tenant settings uses defaults for missing", func(t *testing.T) {
		tenant := &tenant.Tenant{
			Name: "Partial Company",
			Settings: tenant.TenantSettings{
				PDFPrimaryColor: "#00ff00",
				// Others are empty
			},
		}

		settings := svc.PDFSettingsFromTenant(tenant)

		assert.Equal(t, "#00ff00", settings.PrimaryColor)
		assert.Equal(t, "Thank you for your business", settings.FooterText)
		assert.Empty(t, settings.BankDetails)
		assert.Equal(t, "Payment due within 14 days of invoice date.", settings.InvoiceTerms)
	})
}

func TestGenerateInvoicePDF(t *testing.T) {
	svc := NewService()

	t.Run("generates PDF for basic invoice", func(t *testing.T) {
		invoice := createTestInvoice()
		tnant := createTestTenant()
		settings := DefaultPDFSettings()

		pdfBytes, err := svc.GenerateInvoicePDF(invoice, tnant, settings)

		require.NoError(t, err)
		require.NotEmpty(t, pdfBytes)
		// PDF files start with %PDF
		assert.True(t, len(pdfBytes) > 4)
		assert.Equal(t, "%PDF", string(pdfBytes[:4]))
	})

	t.Run("generates PDF for credit note", func(t *testing.T) {
		invoice := createTestInvoice()
		invoice.InvoiceType = invoicing.InvoiceTypeCreditNote
		tnant := createTestTenant()
		settings := DefaultPDFSettings()

		pdfBytes, err := svc.GenerateInvoicePDF(invoice, tnant, settings)

		require.NoError(t, err)
		require.NotEmpty(t, pdfBytes)
		assert.Equal(t, "%PDF", string(pdfBytes[:4]))
	})

	t.Run("generates PDF for purchase invoice", func(t *testing.T) {
		invoice := createTestInvoice()
		invoice.InvoiceType = invoicing.InvoiceTypePurchase
		tnant := createTestTenant()
		settings := DefaultPDFSettings()

		pdfBytes, err := svc.GenerateInvoicePDF(invoice, tnant, settings)

		require.NoError(t, err)
		require.NotEmpty(t, pdfBytes)
		assert.Equal(t, "%PDF", string(pdfBytes[:4]))
	})

	t.Run("generates PDF with contact details", func(t *testing.T) {
		invoice := createTestInvoice()
		invoice.Contact = &contacts.Contact{
			Name:         "Customer Inc",
			Email:        "customer@example.com",
			AddressLine1: "123 Main St",
			AddressLine2: "Suite 100",
			City:         "Tallinn",
			PostalCode:   "10115",
			CountryCode:  "EE",
			VATNumber:    "EE123456789",
		}
		tnant := createTestTenant()
		settings := DefaultPDFSettings()

		pdfBytes, err := svc.GenerateInvoicePDF(invoice, tnant, settings)

		require.NoError(t, err)
		require.NotEmpty(t, pdfBytes)
	})

	t.Run("generates PDF with partial payment", func(t *testing.T) {
		invoice := createTestInvoice()
		invoice.AmountPaid = decimal.NewFromInt(50)
		tnant := createTestTenant()
		settings := DefaultPDFSettings()

		pdfBytes, err := svc.GenerateInvoicePDF(invoice, tnant, settings)

		require.NoError(t, err)
		require.NotEmpty(t, pdfBytes)
	})

	t.Run("generates PDF with reference", func(t *testing.T) {
		invoice := createTestInvoice()
		invoice.Reference = "PO-12345"
		tnant := createTestTenant()
		settings := DefaultPDFSettings()

		pdfBytes, err := svc.GenerateInvoicePDF(invoice, tnant, settings)

		require.NoError(t, err)
		require.NotEmpty(t, pdfBytes)
	})

	t.Run("generates PDF with notes", func(t *testing.T) {
		invoice := createTestInvoice()
		invoice.Notes = "Special delivery instructions"
		tnant := createTestTenant()
		settings := DefaultPDFSettings()

		pdfBytes, err := svc.GenerateInvoicePDF(invoice, tnant, settings)

		require.NoError(t, err)
		require.NotEmpty(t, pdfBytes)
	})

	t.Run("generates PDF with bank details", func(t *testing.T) {
		invoice := createTestInvoice()
		tnant := createTestTenant()
		settings := DefaultPDFSettings()
		settings.BankDetails = "IBAN: EE123456789\nSWIFT: HABAEE2X\nBank: Swedbank"

		pdfBytes, err := svc.GenerateInvoicePDF(invoice, tnant, settings)

		require.NoError(t, err)
		require.NotEmpty(t, pdfBytes)
	})

	t.Run("generates PDF with multiple line items", func(t *testing.T) {
		invoice := createTestInvoice()
		invoice.Lines = []invoicing.InvoiceLine{
			{
				LineNumber:      1,
				Description:     "Product A",
				Quantity:        decimal.NewFromInt(2),
				UnitPrice:       decimal.NewFromFloat(25.00),
				VATRate:         decimal.NewFromInt(20),
				DiscountPercent: decimal.Zero,
				LineTotal:       decimal.NewFromFloat(50.00),
			},
			{
				LineNumber:      2,
				Description:     "Product B with longer description that might need truncation",
				Quantity:        decimal.NewFromInt(5),
				UnitPrice:       decimal.NewFromFloat(10.00),
				VATRate:         decimal.NewFromInt(20),
				DiscountPercent: decimal.NewFromInt(10),
				LineTotal:       decimal.NewFromFloat(45.00),
			},
			{
				LineNumber:      3,
				Description:     "Service C",
				Quantity:        decimal.NewFromFloat(1.5),
				UnitPrice:       decimal.NewFromFloat(100.00),
				VATRate:         decimal.NewFromInt(0),
				DiscountPercent: decimal.Zero,
				LineTotal:       decimal.NewFromFloat(150.00),
			},
		}
		tnant := createTestTenant()
		settings := DefaultPDFSettings()

		pdfBytes, err := svc.GenerateInvoicePDF(invoice, tnant, settings)

		require.NoError(t, err)
		require.NotEmpty(t, pdfBytes)
	})

	t.Run("generates PDF with full tenant settings", func(t *testing.T) {
		invoice := createTestInvoice()
		tnant := createTestTenant()
		tnant.Settings = tenant.TenantSettings{
			Address:   "123 Business St\nTallinn",
			Email:     "info@company.ee",
			Phone:     "+372 5551234",
			VATNumber: "EE123456789",
			RegCode:   "12345678",
		}
		settings := DefaultPDFSettings()

		pdfBytes, err := svc.GenerateInvoicePDF(invoice, tnant, settings)

		require.NoError(t, err)
		require.NotEmpty(t, pdfBytes)
	})
}

func TestGenerateCommercialDocumentPDFs(t *testing.T) {
	svc := NewService()
	tnant := createTestTenant()
	settings := DefaultPDFSettings()
	settings.InvoiceTerms = "Payment due within 10 days."
	settings.FooterText = "Thank you for choosing Test Company."

	t.Run("generates quote PDF", func(t *testing.T) {
		quote := createTestQuote()

		pdfBytes, err := svc.GenerateQuotePDF(quote, tnant, settings)

		require.NoError(t, err)
		require.NotEmpty(t, pdfBytes)
		assert.Equal(t, "%PDF", string(pdfBytes[:4]))
	})

	t.Run("generates order PDF", func(t *testing.T) {
		order := createTestOrder()

		pdfBytes, err := svc.GenerateOrderPDF(order, tnant, settings)

		require.NoError(t, err)
		require.NotEmpty(t, pdfBytes)
		assert.Equal(t, "%PDF", string(pdfBytes[:4]))
	})

	t.Run("generates document with default reference label and sparse contact", func(t *testing.T) {
		doc := commercialDocumentPDF{
			Title:            "CUSTOM DOCUMENT",
			NumberLabel:      "Document No.",
			Number:           "DOC-001",
			Status:           "READY",
			PrimaryDateLabel: "Document Date",
			PrimaryDate:      "15.03.2026",
			RecipientLabel:   "Recipient:",
			Reference:        "EXT-REF-001",
			Currency:         "EUR",
			Subtotal:         decimal.NewFromInt(80),
			VATAmount:        decimal.NewFromInt(16),
			Total:            decimal.NewFromInt(96),
			Lines: []commercialDocumentLine{{
				Description:     "Standalone service line",
				Quantity:        decimal.NewFromInt(1),
				UnitPrice:       decimal.NewFromInt(80),
				VATRate:         decimal.NewFromInt(20),
				DiscountPercent: decimal.Zero,
				LineTotal:       decimal.NewFromInt(96),
			}},
		}

		pdfBytes, err := svc.generateCommercialDocumentPDF(doc, tnant, PDFSettings{})

		require.NoError(t, err)
		require.NotEmpty(t, pdfBytes)
		assert.Equal(t, "%PDF", string(pdfBytes[:4]))
	})
}

func TestGeneratePayslipPDF(t *testing.T) {
	svc := NewService()
	tnant := createTestTenant()
	run := &payroll.PayrollRun{
		ID:          "run-1",
		TenantID:    "tenant-1",
		PeriodYear:  2026,
		PeriodMonth: 3,
		Status:      payroll.PayrollCalculated,
	}
	payslip := &payroll.Payslip{
		ID:                      "payslip-1",
		TenantID:                "tenant-1",
		PayrollRunID:            "run-1",
		EmployeeID:              "employee-1",
		GrossSalary:             decimal.RequireFromString("3200.00"),
		TaxableIncome:           decimal.RequireFromString("2500.00"),
		IncomeTax:               decimal.RequireFromString("550.00"),
		UnemploymentInsuranceEE: decimal.RequireFromString("51.20"),
		FundedPension:           decimal.RequireFromString("64.00"),
		NetSalary:               decimal.RequireFromString("2534.80"),
		SocialTax:               decimal.RequireFromString("1056.00"),
		UnemploymentInsuranceER: decimal.RequireFromString("25.60"),
		TotalEmployerCost:       decimal.RequireFromString("4281.60"),
		BasicExemptionApplied:   decimal.RequireFromString("700.00"),
		PaymentStatus:           "PENDING",
		Employee:                &payroll.Employee{FirstName: "Mari", LastName: "Maasikas", PersonalCode: "49001010001", Email: "mari@example.com"},
	}

	pdfBytes, err := svc.GeneratePayslipPDF(payslip, run, tnant)

	require.NoError(t, err)
	require.NotEmpty(t, pdfBytes)
	assert.Equal(t, "%PDF", string(pdfBytes[:4]))

	t.Run("generates paid payslip without run or employee", func(t *testing.T) {
		paidAt := time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC)
		edgePayslip := *payslip
		edgePayslip.Employee = nil
		edgePayslip.PaidAt = &paidAt

		pdfBytes, err := svc.GeneratePayslipPDF(&edgePayslip, nil, tnant)

		require.NoError(t, err)
		require.NotEmpty(t, pdfBytes)
		assert.Equal(t, "%PDF", string(pdfBytes[:4]))
	})

	t.Run("falls back to employee id when employee name is blank", func(t *testing.T) {
		edgePayslip := *payslip
		edgePayslip.Employee = &payroll.Employee{Email: "blank-name@example.com"}

		pdfBytes, err := svc.GeneratePayslipPDF(&edgePayslip, run, tnant)

		require.NoError(t, err)
		require.NotEmpty(t, pdfBytes)
		assert.Equal(t, "%PDF", string(pdfBytes[:4]))
	})
}

func createTestInvoice() *invoicing.Invoice {
	now := time.Now()
	return &invoicing.Invoice{
		ID:            "inv-123",
		InvoiceNumber: "INV-2024-001",
		InvoiceType:   invoicing.InvoiceTypeSales,
		Status:        invoicing.StatusDraft,
		IssueDate:     now,
		DueDate:       now.AddDate(0, 0, 14),
		Currency:      "EUR",
		Subtotal:      decimal.NewFromFloat(100.00),
		VATAmount:     decimal.NewFromFloat(20.00),
		Total:         decimal.NewFromFloat(120.00),
		AmountPaid:    decimal.Zero,
		Lines: []invoicing.InvoiceLine{
			{
				LineNumber:      1,
				Description:     "Test Product",
				Quantity:        decimal.NewFromInt(1),
				UnitPrice:       decimal.NewFromFloat(100.00),
				VATRate:         decimal.NewFromInt(20),
				DiscountPercent: decimal.Zero,
				LineTotal:       decimal.NewFromFloat(100.00),
			},
		},
	}
}

func createTestQuote() *quotes.Quote {
	quoteDate := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	validUntil := quoteDate.AddDate(0, 0, 30)
	return &quotes.Quote{
		ID:          "quote-123",
		TenantID:    "tenant-123",
		QuoteNumber: "QUO-2026-001",
		ContactID:   "contact-123",
		Contact:     createPDFTestContact(),
		QuoteDate:   quoteDate,
		ValidUntil:  &validUntil,
		Status:      quotes.QuoteStatusSent,
		Currency:    "EUR",
		Subtotal:    decimal.NewFromInt(150),
		VATAmount:   decimal.NewFromInt(30),
		Total:       decimal.NewFromInt(180),
		Notes:       "Quote notes for customer review.",
		Lines: []quotes.QuoteLine{
			{
				Description:     "Commercial consulting package with a long description that exercises truncation in the PDF table",
				Quantity:        decimal.NewFromInt(2),
				UnitPrice:       decimal.NewFromInt(50),
				VATRate:         decimal.NewFromInt(20),
				DiscountPercent: decimal.NewFromInt(10),
				LineTotal:       decimal.NewFromInt(108),
			},
			{
				LineNumber:      2,
				Description:     "Implementation support",
				Quantity:        decimal.NewFromInt(1),
				UnitPrice:       decimal.NewFromInt(60),
				VATRate:         decimal.NewFromInt(20),
				DiscountPercent: decimal.Zero,
				LineTotal:       decimal.NewFromInt(72),
			},
		},
	}
}

func createTestOrder() *orders.Order {
	orderDate := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	expectedDelivery := orderDate.AddDate(0, 0, 14)
	quoteID := "quote-123"
	return &orders.Order{
		ID:               "order-123",
		TenantID:         "tenant-123",
		OrderNumber:      "ORD-2026-001",
		ContactID:        "contact-123",
		Contact:          createPDFTestContact(),
		OrderDate:        orderDate,
		ExpectedDelivery: &expectedDelivery,
		Status:           orders.OrderStatusConfirmed,
		Currency:         "EUR",
		Subtotal:         decimal.NewFromInt(200),
		VATAmount:        decimal.NewFromInt(40),
		Total:            decimal.NewFromInt(240),
		Notes:            "Order confirmation notes.",
		QuoteID:          &quoteID,
		Lines: []orders.OrderLine{
			{
				LineNumber:      1,
				Description:     "Hardware bundle",
				Quantity:        decimal.NewFromInt(1),
				UnitPrice:       decimal.NewFromInt(120),
				VATRate:         decimal.NewFromInt(20),
				DiscountPercent: decimal.Zero,
				LineTotal:       decimal.NewFromInt(144),
			},
			{
				Description:     "Installation service",
				Quantity:        decimal.NewFromInt(2),
				UnitPrice:       decimal.NewFromInt(40),
				VATRate:         decimal.NewFromInt(20),
				DiscountPercent: decimal.Zero,
				LineTotal:       decimal.NewFromInt(96),
			},
		},
	}
}

func createPDFTestContact() *contacts.Contact {
	return &contacts.Contact{
		ID:           "contact-123",
		Name:         "Customer Inc",
		Email:        "customer@example.com",
		AddressLine1: "123 Main St",
		AddressLine2: "Suite 100",
		City:         "Tallinn",
		PostalCode:   "10115",
		CountryCode:  "EE",
		VATNumber:    "EE123456789",
	}
}

func createTestTenant() *tenant.Tenant {
	return &tenant.Tenant{
		ID:       "tenant-123",
		Name:     "Test Company OÜ",
		Slug:     "test-company",
		Settings: tenant.TenantSettings{},
	}
}
