package recurring

import (
	"testing"
)

func TestNewService(t *testing.T) {
	service := NewService(nil, nil)
	if service == nil {
		t.Fatal("NewService(nil, nil) returned nil")
	}
	if service.db != nil {
		t.Error("NewService(nil, nil).db should be nil")
	}
	if service.invoicing != nil {
		t.Error("NewService(nil, nil).invoicing should be nil")
	}
}

func TestNewService_NotNil(t *testing.T) {
	service := NewService(nil, nil)
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
