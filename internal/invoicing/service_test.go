package invoicing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// MockRepository is a mock implementation of Repository for testing
type MockRepository struct {
	invoices              map[string]*Invoice
	nextNumber            int
	CreateFn              func(ctx context.Context, schemaName string, invoice *Invoice) error
	GetByIDFn             func(ctx context.Context, schemaName, tenantID, invoiceID string) (*Invoice, error)
	ListFn                func(ctx context.Context, schemaName, tenantID string, filter *InvoiceFilter) ([]Invoice, error)
	UpdateStatusFn        func(ctx context.Context, schemaName, tenantID, invoiceID string, status InvoiceStatus) error
	UpdatePaymentFn       func(ctx context.Context, schemaName, tenantID, invoiceID string, amountPaid decimal.Decimal, status InvoiceStatus) error
	GenerateNumFn         func(ctx context.Context, schemaName, tenantID string, invoiceType InvoiceType) (string, error)
	UpdateOverdueStatusFn func(ctx context.Context, schemaName, tenantID string) (int, error)
	overdueUpdateErr      error
	overdueUpdateCount    int
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		invoices:   make(map[string]*Invoice),
		nextNumber: 1,
	}
}

func (m *MockRepository) Create(ctx context.Context, schemaName string, invoice *Invoice) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, schemaName, invoice)
	}
	m.invoices[invoice.ID] = invoice
	return nil
}

func (m *MockRepository) GetByID(ctx context.Context, schemaName, tenantID, invoiceID string) (*Invoice, error) {
	if m.GetByIDFn != nil {
		return m.GetByIDFn(ctx, schemaName, tenantID, invoiceID)
	}
	if inv, ok := m.invoices[invoiceID]; ok && inv.TenantID == tenantID {
		return inv, nil
	}
	return nil, ErrInvoiceNotFound
}

func (m *MockRepository) List(ctx context.Context, schemaName, tenantID string, filter *InvoiceFilter) ([]Invoice, error) {
	if m.ListFn != nil {
		return m.ListFn(ctx, schemaName, tenantID, filter)
	}
	var result []Invoice
	for _, inv := range m.invoices {
		if inv.TenantID == tenantID {
			result = append(result, *inv)
		}
	}
	return result, nil
}

func (m *MockRepository) UpdateStatus(ctx context.Context, schemaName, tenantID, invoiceID string, status InvoiceStatus) error {
	if m.UpdateStatusFn != nil {
		return m.UpdateStatusFn(ctx, schemaName, tenantID, invoiceID, status)
	}
	if inv, ok := m.invoices[invoiceID]; ok && inv.TenantID == tenantID {
		inv.Status = status
		return nil
	}
	return ErrInvoiceNotFound
}

func (m *MockRepository) UpdatePayment(ctx context.Context, schemaName, tenantID, invoiceID string, amountPaid decimal.Decimal, status InvoiceStatus) error {
	if m.UpdatePaymentFn != nil {
		return m.UpdatePaymentFn(ctx, schemaName, tenantID, invoiceID, amountPaid, status)
	}
	if inv, ok := m.invoices[invoiceID]; ok && inv.TenantID == tenantID {
		inv.AmountPaid = amountPaid
		inv.Status = status
		return nil
	}
	return ErrInvoiceNotFound
}

func (m *MockRepository) GenerateNumber(ctx context.Context, schemaName, tenantID string, invoiceType InvoiceType) (string, error) {
	if m.GenerateNumFn != nil {
		return m.GenerateNumFn(ctx, schemaName, tenantID, invoiceType)
	}
	prefix := "INV"
	if invoiceType == InvoiceTypePurchase {
		prefix = "BILL"
	} else if invoiceType == InvoiceTypeCreditNote {
		prefix = "CN"
	}
	m.nextNumber++
	return prefix + "-00001", nil
}

func (m *MockRepository) UpdateOverdueStatus(ctx context.Context, schemaName, tenantID string) (int, error) {
	if m.UpdateOverdueStatusFn != nil {
		return m.UpdateOverdueStatusFn(ctx, schemaName, tenantID)
	}
	if m.overdueUpdateErr != nil {
		return 0, m.overdueUpdateErr
	}
	// Default behavior: count and update overdue invoices in the mock
	count := 0
	now := time.Now()
	for _, inv := range m.invoices {
		if inv.TenantID == tenantID &&
			(inv.Status == StatusSent || inv.Status == StatusPartiallyPaid) &&
			inv.DueDate.Before(now) &&
			inv.AmountPaid.LessThan(inv.Total) {
			inv.Status = StatusOverdue
			count++
		}
	}
	if m.overdueUpdateCount > 0 {
		return m.overdueUpdateCount, nil
	}
	return count, nil
}

func TestNewServiceWithRepository(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil)
	if service == nil {
		t.Error("NewServiceWithRepository should return a non-nil service")
	}
}

func TestService_Create(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil)

	tests := []struct {
		name     string
		tenantID string
		req      *CreateInvoiceRequest
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "Valid sales invoice",
			tenantID: "tenant-1",
			req: &CreateInvoiceRequest{
				InvoiceType: InvoiceTypeSales,
				ContactID:   "contact-1",
				IssueDate:   time.Now(),
				DueDate:     time.Now().AddDate(0, 0, 14),
				Lines: []CreateInvoiceLineRequest{
					{
						Description: "Service",
						Quantity:    decimal.NewFromInt(1),
						UnitPrice:   decimal.NewFromFloat(100.00),
						VATRate:     decimal.NewFromInt(22),
					},
				},
			},
			wantErr: false,
		},
		{
			name:     "Valid purchase invoice",
			tenantID: "tenant-1",
			req: &CreateInvoiceRequest{
				InvoiceType: InvoiceTypePurchase,
				ContactID:   "contact-2",
				IssueDate:   time.Now(),
				DueDate:     time.Now().AddDate(0, 0, 30),
				Lines: []CreateInvoiceLineRequest{
					{
						Description: "Materials",
						Quantity:    decimal.NewFromFloat(5),
						UnitPrice:   decimal.NewFromFloat(50.00),
						VATRate:     decimal.NewFromInt(22),
					},
				},
			},
			wantErr: false,
		},
		{
			name:     "Missing contact",
			tenantID: "tenant-1",
			req: &CreateInvoiceRequest{
				InvoiceType: InvoiceTypeSales,
				ContactID:   "",
				IssueDate:   time.Now(),
				DueDate:     time.Now().AddDate(0, 0, 14),
				Lines: []CreateInvoiceLineRequest{
					{
						Description: "Service",
						Quantity:    decimal.NewFromInt(1),
						UnitPrice:   decimal.NewFromFloat(100.00),
						VATRate:     decimal.NewFromInt(22),
					},
				},
			},
			wantErr: true,
			errMsg:  "contact is required",
		},
		{
			name:     "Missing lines",
			tenantID: "tenant-1",
			req: &CreateInvoiceRequest{
				InvoiceType: InvoiceTypeSales,
				ContactID:   "contact-1",
				IssueDate:   time.Now(),
				DueDate:     time.Now().AddDate(0, 0, 14),
				Lines:       []CreateInvoiceLineRequest{},
			},
			wantErr: true,
			errMsg:  "at least one line",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoice, err := service.Create(ctx, tt.tenantID, "public", tt.req)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if invoice.ID == "" {
				t.Error("Invoice ID should not be empty")
			}
			if invoice.TenantID != tt.tenantID {
				t.Errorf("TenantID = %q, want %q", invoice.TenantID, tt.tenantID)
			}
			if invoice.InvoiceNumber == "" {
				t.Error("Invoice number should not be empty")
			}
			if invoice.Status != StatusDraft {
				t.Errorf("Status = %q, want %q", invoice.Status, StatusDraft)
			}
		})
	}
}

func TestService_Create_DefaultValues(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil)

	req := &CreateInvoiceRequest{
		InvoiceType: InvoiceTypeSales,
		ContactID:   "contact-1",
		Lines: []CreateInvoiceLineRequest{
			{
				Description: "Service",
				Quantity:    decimal.NewFromInt(1),
				UnitPrice:   decimal.NewFromFloat(100.00),
				VATRate:     decimal.NewFromInt(22),
			},
		},
	}

	invoice, err := service.Create(ctx, "tenant-1", "public", req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check defaults
	if invoice.Currency != "EUR" {
		t.Errorf("Default Currency = %q, want %q", invoice.Currency, "EUR")
	}
	if !invoice.ExchangeRate.Equal(decimal.NewFromInt(1)) {
		t.Errorf("Default ExchangeRate = %s, want 1", invoice.ExchangeRate)
	}
	if invoice.IssueDate.IsZero() {
		t.Error("IssueDate should not be zero")
	}
	if invoice.DueDate.IsZero() {
		t.Error("DueDate should not be zero")
	}
}

func TestService_Create_CalculatesTotals(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil)

	req := &CreateInvoiceRequest{
		InvoiceType: InvoiceTypeSales,
		ContactID:   "contact-1",
		IssueDate:   time.Now(),
		DueDate:     time.Now().AddDate(0, 0, 14),
		Lines: []CreateInvoiceLineRequest{
			{
				Description: "Service A",
				Quantity:    decimal.NewFromInt(2),
				UnitPrice:   decimal.NewFromFloat(100.00),
				VATRate:     decimal.NewFromInt(22),
			},
			{
				Description: "Service B",
				Quantity:    decimal.NewFromInt(1),
				UnitPrice:   decimal.NewFromFloat(50.00),
				VATRate:     decimal.NewFromInt(22),
			},
		},
	}

	invoice, err := service.Create(ctx, "tenant-1", "public", req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Subtotal = (2*100) + (1*50) = 250
	expectedSubtotal := decimal.NewFromFloat(250.00)
	if !invoice.Subtotal.Equal(expectedSubtotal) {
		t.Errorf("Subtotal = %s, want %s", invoice.Subtotal, expectedSubtotal)
	}

	// VAT = 250 * 0.22 = 55
	expectedVAT := decimal.NewFromFloat(55.00)
	if !invoice.VATAmount.Equal(expectedVAT) {
		t.Errorf("VATAmount = %s, want %s", invoice.VATAmount, expectedVAT)
	}

	// Total = 250 + 55 = 305
	expectedTotal := decimal.NewFromFloat(305.00)
	if !invoice.Total.Equal(expectedTotal) {
		t.Errorf("Total = %s, want %s", invoice.Total, expectedTotal)
	}
}

func TestService_Create_RepositoryError(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.CreateFn = func(ctx context.Context, schemaName string, invoice *Invoice) error {
		return errors.New("database error")
	}
	service := NewServiceWithRepository(repo, nil)

	req := &CreateInvoiceRequest{
		InvoiceType: InvoiceTypeSales,
		ContactID:   "contact-1",
		IssueDate:   time.Now(),
		DueDate:     time.Now().AddDate(0, 0, 14),
		Lines: []CreateInvoiceLineRequest{
			{
				Description: "Service",
				Quantity:    decimal.NewFromInt(1),
				UnitPrice:   decimal.NewFromFloat(100.00),
				VATRate:     decimal.NewFromInt(22),
			},
		},
	}

	_, err := service.Create(ctx, "tenant-1", "public", req)
	if err == nil {
		t.Error("Expected error but got nil")
	}
}

func TestService_GetByID(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil)

	// Create an invoice first
	created, _ := service.Create(ctx, "tenant-1", "public", &CreateInvoiceRequest{
		InvoiceType: InvoiceTypeSales,
		ContactID:   "contact-1",
		IssueDate:   time.Now(),
		DueDate:     time.Now().AddDate(0, 0, 14),
		Lines: []CreateInvoiceLineRequest{
			{Description: "Test", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100), VATRate: decimal.NewFromInt(22)},
		},
	})

	tests := []struct {
		name      string
		tenantID  string
		invoiceID string
		wantErr   bool
	}{
		{
			name:      "Existing invoice",
			tenantID:  "tenant-1",
			invoiceID: created.ID,
			wantErr:   false,
		},
		{
			name:      "Non-existing invoice",
			tenantID:  "tenant-1",
			invoiceID: "non-existent",
			wantErr:   true,
		},
		{
			name:      "Wrong tenant",
			tenantID:  "other-tenant",
			invoiceID: created.ID,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoice, err := service.GetByID(ctx, tt.tenantID, "public", tt.invoiceID)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if invoice.ID != tt.invoiceID {
				t.Errorf("Invoice ID = %q, want %q", invoice.ID, tt.invoiceID)
			}
		})
	}
}

func TestService_List(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil)

	// Create some invoices
	_, _ = service.Create(ctx, "tenant-1", "public", &CreateInvoiceRequest{
		InvoiceType: InvoiceTypeSales,
		ContactID:   "contact-1",
		IssueDate:   time.Now(),
		DueDate:     time.Now().AddDate(0, 0, 14),
		Lines:       []CreateInvoiceLineRequest{{Description: "A", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100), VATRate: decimal.NewFromInt(22)}},
	})
	_, _ = service.Create(ctx, "tenant-1", "public", &CreateInvoiceRequest{
		InvoiceType: InvoiceTypePurchase,
		ContactID:   "contact-2",
		IssueDate:   time.Now(),
		DueDate:     time.Now().AddDate(0, 0, 30),
		Lines:       []CreateInvoiceLineRequest{{Description: "B", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(200), VATRate: decimal.NewFromInt(22)}},
	})
	_, _ = service.Create(ctx, "tenant-2", "public", &CreateInvoiceRequest{
		InvoiceType: InvoiceTypeSales,
		ContactID:   "contact-3",
		IssueDate:   time.Now(),
		DueDate:     time.Now().AddDate(0, 0, 14),
		Lines:       []CreateInvoiceLineRequest{{Description: "C", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(300), VATRate: decimal.NewFromInt(22)}},
	})

	invoices, err := service.List(ctx, "tenant-1", "public", nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(invoices) != 2 {
		t.Errorf("Expected 2 invoices, got %d", len(invoices))
	}
}

func TestService_List_WithFilter(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.ListFn = func(ctx context.Context, schemaName, tenantID string, filter *InvoiceFilter) ([]Invoice, error) {
		invoices := []Invoice{
			{ID: "1", TenantID: tenantID, InvoiceType: InvoiceTypeSales, Status: StatusDraft},
			{ID: "2", TenantID: tenantID, InvoiceType: InvoiceTypePurchase, Status: StatusSent},
		}

		if filter != nil && filter.InvoiceType != "" {
			var filtered []Invoice
			for _, inv := range invoices {
				if inv.InvoiceType == filter.InvoiceType {
					filtered = append(filtered, inv)
				}
			}
			return filtered, nil
		}
		return invoices, nil
	}
	service := NewServiceWithRepository(repo, nil)

	invoices, err := service.List(ctx, "tenant-1", "public", &InvoiceFilter{InvoiceType: InvoiceTypeSales})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(invoices) != 1 {
		t.Errorf("Expected 1 sales invoice, got %d", len(invoices))
	}
}

func TestService_List_Error(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.ListFn = func(ctx context.Context, schemaName, tenantID string, filter *InvoiceFilter) ([]Invoice, error) {
		return nil, errors.New("database error")
	}
	service := NewServiceWithRepository(repo, nil)

	_, err := service.List(ctx, "tenant-1", "public", nil)
	if err == nil {
		t.Error("Expected error but got nil")
	}
}

func TestService_Send(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil)

	// Create a draft invoice
	created, _ := service.Create(ctx, "tenant-1", "public", &CreateInvoiceRequest{
		InvoiceType: InvoiceTypeSales,
		ContactID:   "contact-1",
		IssueDate:   time.Now(),
		DueDate:     time.Now().AddDate(0, 0, 14),
		Lines:       []CreateInvoiceLineRequest{{Description: "Test", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100), VATRate: decimal.NewFromInt(22)}},
	})

	err := service.Send(ctx, "tenant-1", "public", created.ID)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify status changed
	updated, _ := service.GetByID(ctx, "tenant-1", "public", created.ID)
	if updated.Status != StatusSent {
		t.Errorf("Status = %q, want %q", updated.Status, StatusSent)
	}
}

func TestService_Send_NotDraft(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil)

	// Create and send an invoice
	created, _ := service.Create(ctx, "tenant-1", "public", &CreateInvoiceRequest{
		InvoiceType: InvoiceTypeSales,
		ContactID:   "contact-1",
		IssueDate:   time.Now(),
		DueDate:     time.Now().AddDate(0, 0, 14),
		Lines:       []CreateInvoiceLineRequest{{Description: "Test", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100), VATRate: decimal.NewFromInt(22)}},
	})
	_ = service.Send(ctx, "tenant-1", "public", created.ID)

	// Try to send again
	err := service.Send(ctx, "tenant-1", "public", created.ID)
	if err == nil {
		t.Error("Expected error when sending non-draft invoice")
	}
}

func TestService_Send_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil)

	err := service.Send(ctx, "tenant-1", "public", "non-existent")
	if err == nil {
		t.Error("Expected error for non-existent invoice")
	}
}

func TestService_RecordPayment(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil)

	// Create and send an invoice with total 122.00
	created, _ := service.Create(ctx, "tenant-1", "public", &CreateInvoiceRequest{
		InvoiceType: InvoiceTypeSales,
		ContactID:   "contact-1",
		IssueDate:   time.Now(),
		DueDate:     time.Now().AddDate(0, 0, 14),
		Lines:       []CreateInvoiceLineRequest{{Description: "Test", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100), VATRate: decimal.NewFromInt(22)}},
	})
	_ = service.Send(ctx, "tenant-1", "public", created.ID)

	// Record partial payment
	err := service.RecordPayment(ctx, "tenant-1", "public", created.ID, decimal.NewFromFloat(50.00))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	updated, _ := service.GetByID(ctx, "tenant-1", "public", created.ID)
	if !updated.AmountPaid.Equal(decimal.NewFromFloat(50.00)) {
		t.Errorf("AmountPaid = %s, want 50.00", updated.AmountPaid)
	}
	if updated.Status != StatusPartiallyPaid {
		t.Errorf("Status = %q, want %q", updated.Status, StatusPartiallyPaid)
	}
}

func TestService_RecordPayment_FullPayment(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil)

	created, _ := service.Create(ctx, "tenant-1", "public", &CreateInvoiceRequest{
		InvoiceType: InvoiceTypeSales,
		ContactID:   "contact-1",
		IssueDate:   time.Now(),
		DueDate:     time.Now().AddDate(0, 0, 14),
		Lines:       []CreateInvoiceLineRequest{{Description: "Test", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100), VATRate: decimal.NewFromInt(22)}},
	})
	_ = service.Send(ctx, "tenant-1", "public", created.ID)

	// Pay full amount (122.00)
	err := service.RecordPayment(ctx, "tenant-1", "public", created.ID, decimal.NewFromFloat(122.00))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	updated, _ := service.GetByID(ctx, "tenant-1", "public", created.ID)
	if updated.Status != StatusPaid {
		t.Errorf("Status = %q, want %q", updated.Status, StatusPaid)
	}
}

func TestService_RecordPayment_Voided(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil)

	created, _ := service.Create(ctx, "tenant-1", "public", &CreateInvoiceRequest{
		InvoiceType: InvoiceTypeSales,
		ContactID:   "contact-1",
		IssueDate:   time.Now(),
		DueDate:     time.Now().AddDate(0, 0, 14),
		Lines:       []CreateInvoiceLineRequest{{Description: "Test", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100), VATRate: decimal.NewFromInt(22)}},
	})

	// Void the invoice
	_ = service.Void(ctx, "tenant-1", "public", created.ID)

	// Try to record payment on voided invoice
	err := service.RecordPayment(ctx, "tenant-1", "public", created.ID, decimal.NewFromFloat(50.00))
	if err == nil {
		t.Error("Expected error when recording payment on voided invoice")
	}
}

func TestService_RecordPayment_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil)

	err := service.RecordPayment(ctx, "tenant-1", "public", "non-existent", decimal.NewFromFloat(50.00))
	if err == nil {
		t.Error("Expected error for non-existent invoice")
	}
}

func TestService_Void(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil)

	created, _ := service.Create(ctx, "tenant-1", "public", &CreateInvoiceRequest{
		InvoiceType: InvoiceTypeSales,
		ContactID:   "contact-1",
		IssueDate:   time.Now(),
		DueDate:     time.Now().AddDate(0, 0, 14),
		Lines:       []CreateInvoiceLineRequest{{Description: "Test", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100), VATRate: decimal.NewFromInt(22)}},
	})

	err := service.Void(ctx, "tenant-1", "public", created.ID)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	updated, _ := service.GetByID(ctx, "tenant-1", "public", created.ID)
	if updated.Status != StatusVoided {
		t.Errorf("Status = %q, want %q", updated.Status, StatusVoided)
	}
}

func TestService_Void_AlreadyVoided(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil)

	created, _ := service.Create(ctx, "tenant-1", "public", &CreateInvoiceRequest{
		InvoiceType: InvoiceTypeSales,
		ContactID:   "contact-1",
		IssueDate:   time.Now(),
		DueDate:     time.Now().AddDate(0, 0, 14),
		Lines:       []CreateInvoiceLineRequest{{Description: "Test", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100), VATRate: decimal.NewFromInt(22)}},
	})
	_ = service.Void(ctx, "tenant-1", "public", created.ID)

	// Try to void again
	err := service.Void(ctx, "tenant-1", "public", created.ID)
	if err == nil {
		t.Error("Expected error when voiding already voided invoice")
	}
}

func TestService_Void_WithPayments(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil)

	created, _ := service.Create(ctx, "tenant-1", "public", &CreateInvoiceRequest{
		InvoiceType: InvoiceTypeSales,
		ContactID:   "contact-1",
		IssueDate:   time.Now(),
		DueDate:     time.Now().AddDate(0, 0, 14),
		Lines:       []CreateInvoiceLineRequest{{Description: "Test", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100), VATRate: decimal.NewFromInt(22)}},
	})
	_ = service.Send(ctx, "tenant-1", "public", created.ID)
	_ = service.RecordPayment(ctx, "tenant-1", "public", created.ID, decimal.NewFromFloat(50.00))

	// Try to void invoice with payments
	err := service.Void(ctx, "tenant-1", "public", created.ID)
	if err == nil {
		t.Error("Expected error when voiding invoice with payments")
	}
}

func TestService_Void_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil)

	err := service.Void(ctx, "tenant-1", "public", "non-existent")
	if err == nil {
		t.Error("Expected error for non-existent invoice")
	}
}

func TestInvoiceTypeConstants(t *testing.T) {
	if InvoiceTypeSales != "SALES" {
		t.Errorf("InvoiceTypeSales = %q, want SALES", InvoiceTypeSales)
	}
	if InvoiceTypePurchase != "PURCHASE" {
		t.Errorf("InvoiceTypePurchase = %q, want PURCHASE", InvoiceTypePurchase)
	}
	if InvoiceTypeCreditNote != "CREDIT_NOTE" {
		t.Errorf("InvoiceTypeCreditNote = %q, want CREDIT_NOTE", InvoiceTypeCreditNote)
	}
}

func TestInvoiceStatusConstants(t *testing.T) {
	statuses := []struct {
		status   InvoiceStatus
		expected string
	}{
		{StatusDraft, "DRAFT"},
		{StatusSent, "SENT"},
		{StatusPartiallyPaid, "PARTIALLY_PAID"},
		{StatusPaid, "PAID"},
		{StatusOverdue, "OVERDUE"},
		{StatusVoided, "VOIDED"},
	}

	for _, tt := range statuses {
		if string(tt.status) != tt.expected {
			t.Errorf("Status = %q, want %q", tt.status, tt.expected)
		}
	}
}

// Test error cases for improved coverage

func TestService_Create_GenerateNumberError(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.GenerateNumFn = func(ctx context.Context, schemaName, tenantID string, invoiceType InvoiceType) (string, error) {
		return "", errors.New("sequence error")
	}
	service := NewServiceWithRepository(repo, nil)

	_, err := service.Create(ctx, "tenant-1", "public", &CreateInvoiceRequest{
		InvoiceType: InvoiceTypeSales,
		ContactID:   "contact-1",
		IssueDate:   time.Now(),
		DueDate:     time.Now().AddDate(0, 0, 14),
		Lines:       []CreateInvoiceLineRequest{{Description: "Test", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100), VATRate: decimal.NewFromInt(22)}},
	})
	if err == nil {
		t.Error("Expected error when GenerateNumber fails")
	}
}

func TestService_Send_UpdateStatusError(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.UpdateStatusFn = func(ctx context.Context, schemaName, tenantID, invoiceID string, status InvoiceStatus) error {
		return errors.New("database error")
	}
	service := NewServiceWithRepository(repo, nil)

	created, _ := service.Create(ctx, "tenant-1", "public", &CreateInvoiceRequest{
		InvoiceType: InvoiceTypeSales,
		ContactID:   "contact-1",
		Lines:       []CreateInvoiceLineRequest{{Description: "Test", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100), VATRate: decimal.NewFromInt(22)}},
	})

	err := service.Send(ctx, "tenant-1", "public", created.ID)
	if err == nil {
		t.Error("Expected error when UpdateStatus fails")
	}
}

func TestService_RecordPayment_UpdatePaymentError(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.UpdatePaymentFn = func(ctx context.Context, schemaName, tenantID, invoiceID string, amountPaid decimal.Decimal, status InvoiceStatus) error {
		return errors.New("database error")
	}
	service := NewServiceWithRepository(repo, nil)

	created, _ := service.Create(ctx, "tenant-1", "public", &CreateInvoiceRequest{
		InvoiceType: InvoiceTypeSales,
		ContactID:   "contact-1",
		Lines:       []CreateInvoiceLineRequest{{Description: "Test", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100), VATRate: decimal.NewFromInt(22)}},
	})

	err := service.RecordPayment(ctx, "tenant-1", "public", created.ID, decimal.NewFromFloat(50.00))
	if err == nil {
		t.Error("Expected error when UpdatePayment fails")
	}
}

func TestService_Void_UpdateStatusError(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.UpdateStatusFn = func(ctx context.Context, schemaName, tenantID, invoiceID string, status InvoiceStatus) error {
		return errors.New("database error")
	}
	service := NewServiceWithRepository(repo, nil)

	created, _ := service.Create(ctx, "tenant-1", "public", &CreateInvoiceRequest{
		InvoiceType: InvoiceTypeSales,
		ContactID:   "contact-1",
		Lines:       []CreateInvoiceLineRequest{{Description: "Test", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100), VATRate: decimal.NewFromInt(22)}},
	})

	err := service.Void(ctx, "tenant-1", "public", created.ID)
	if err == nil {
		t.Error("Expected error when UpdateStatus fails")
	}
}

func TestService_UpdateOverdueStatus_Success(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	// Add an overdue invoice
	overdueInvoice := &Invoice{
		ID:         "inv-overdue",
		TenantID:   "tenant-1",
		Status:     StatusSent,
		DueDate:    time.Now().AddDate(0, 0, -7), // 7 days ago
		Total:      decimal.NewFromFloat(100),
		AmountPaid: decimal.Zero,
	}
	repo.invoices["inv-overdue"] = overdueInvoice

	// Add a non-overdue invoice
	futureInvoice := &Invoice{
		ID:         "inv-future",
		TenantID:   "tenant-1",
		Status:     StatusSent,
		DueDate:    time.Now().AddDate(0, 0, 7), // 7 days in future
		Total:      decimal.NewFromFloat(100),
		AmountPaid: decimal.Zero,
	}
	repo.invoices["inv-future"] = futureInvoice

	service := NewServiceWithRepository(repo, nil)

	count, err := service.UpdateOverdueStatus(ctx, "tenant-1", "public")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 updated invoice, got %d", count)
	}
}

func TestService_UpdateOverdueStatus_Error(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.overdueUpdateErr = errors.New("repository error")
	service := NewServiceWithRepository(repo, nil)

	_, err := service.UpdateOverdueStatus(ctx, "tenant-1", "public")
	if err == nil {
		t.Error("Expected error from repository")
	}
}

func TestService_RecordPayment_OverpaymentCapped(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil)

	created, _ := service.Create(ctx, "tenant-1", "public", &CreateInvoiceRequest{
		InvoiceType: InvoiceTypeSales,
		ContactID:   "contact-1",
		Lines:       []CreateInvoiceLineRequest{{Description: "Test", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100), VATRate: decimal.NewFromInt(22)}},
	})
	_ = service.Send(ctx, "tenant-1", "public", created.ID)

	// Pay more than total - should be capped to total
	err := service.RecordPayment(ctx, "tenant-1", "public", created.ID, decimal.NewFromFloat(200.00))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	updated, _ := service.GetByID(ctx, "tenant-1", "public", created.ID)
	if !updated.AmountPaid.Equal(created.Total) {
		t.Errorf("AmountPaid = %s, want %s (capped at total)", updated.AmountPaid, created.Total)
	}
	if updated.Status != StatusPaid {
		t.Errorf("Status = %q, want %q", updated.Status, StatusPaid)
	}
}

func TestService_RecordPayment_ZeroAmount(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil)

	created, _ := service.Create(ctx, "tenant-1", "public", &CreateInvoiceRequest{
		InvoiceType: InvoiceTypeSales,
		ContactID:   "contact-1",
		Lines:       []CreateInvoiceLineRequest{{Description: "Test", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(100), VATRate: decimal.NewFromInt(22)}},
	})
	_ = service.Send(ctx, "tenant-1", "public", created.ID)

	// Record zero payment - status should remain unchanged
	err := service.RecordPayment(ctx, "tenant-1", "public", created.ID, decimal.Zero)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	updated, _ := service.GetByID(ctx, "tenant-1", "public", created.ID)
	if !updated.AmountPaid.IsZero() {
		t.Errorf("AmountPaid = %s, want 0", updated.AmountPaid)
	}
	if updated.Status != StatusSent {
		t.Errorf("Status = %q, want %q (unchanged)", updated.Status, StatusSent)
	}
}
