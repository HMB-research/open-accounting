package payments

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockRepository is a mock implementation of Repository for testing
type MockRepository struct {
	payments            map[string]*Payment
	allocations         map[string][]PaymentAllocation
	nextSeq             int
	createErr           error
	getErr              error
	listErr             error
	createAllocErr      error
	getNextNumErr       error
	getAllocErr         error
	getUnallocatedErr   error
	unallocatedPayments []Payment
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		payments:    make(map[string]*Payment),
		allocations: make(map[string][]PaymentAllocation),
		nextSeq:     1,
	}
}

func (m *MockRepository) Create(ctx context.Context, schemaName string, payment *Payment) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.payments[payment.ID] = payment
	return nil
}

func (m *MockRepository) GetByID(ctx context.Context, schemaName, tenantID, paymentID string) (*Payment, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	p, ok := m.payments[paymentID]
	if !ok {
		return nil, ErrPaymentNotFound
	}
	if p.TenantID != tenantID {
		return nil, ErrPaymentNotFound
	}
	// Attach allocations
	p.Allocations = m.allocations[paymentID]
	return p, nil
}

func (m *MockRepository) List(ctx context.Context, schemaName, tenantID string, filter *PaymentFilter) ([]Payment, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var result []Payment
	for _, p := range m.payments {
		if p.TenantID != tenantID {
			continue
		}
		if filter != nil {
			if filter.PaymentType != "" && p.PaymentType != filter.PaymentType {
				continue
			}
			if filter.ContactID != "" && (p.ContactID == nil || *p.ContactID != filter.ContactID) {
				continue
			}
		}
		result = append(result, *p)
	}
	return result, nil
}

func (m *MockRepository) CreateAllocation(ctx context.Context, schemaName string, allocation *PaymentAllocation) error {
	if m.createAllocErr != nil {
		return m.createAllocErr
	}
	m.allocations[allocation.PaymentID] = append(m.allocations[allocation.PaymentID], *allocation)
	return nil
}

func (m *MockRepository) GetAllocations(ctx context.Context, schemaName, tenantID, paymentID string) ([]PaymentAllocation, error) {
	if m.getAllocErr != nil {
		return nil, m.getAllocErr
	}
	return m.allocations[paymentID], nil
}

func (m *MockRepository) GetNextPaymentNumber(ctx context.Context, schemaName, tenantID string, paymentType PaymentType) (int, error) {
	if m.getNextNumErr != nil {
		return 0, m.getNextNumErr
	}
	seq := m.nextSeq
	m.nextSeq++
	return seq, nil
}

func (m *MockRepository) GetUnallocatedPayments(ctx context.Context, schemaName, tenantID string, paymentType PaymentType) ([]Payment, error) {
	if m.getUnallocatedErr != nil {
		return nil, m.getUnallocatedErr
	}
	if m.unallocatedPayments != nil {
		return m.unallocatedPayments, nil
	}
	// Default: filter from stored payments based on allocations
	var result []Payment
	for _, p := range m.payments {
		if p.TenantID != tenantID || p.PaymentType != paymentType {
			continue
		}
		// Calculate allocated amount
		allocatedAmount := decimal.Zero
		for _, alloc := range m.allocations[p.ID] {
			allocatedAmount = allocatedAmount.Add(alloc.Amount)
		}
		// Include if amount > allocated
		if p.Amount.GreaterThan(allocatedAmount) {
			result = append(result, *p)
		}
	}
	return result, nil
}

// TestableService wraps Service with repository injection for testing
type TestableService struct {
	repo Repository
}

func NewTestableService(repo Repository) *TestableService {
	return &TestableService{repo: repo}
}

// ValidatePaymentRequest validates the payment creation request
func ValidatePaymentRequest(req *CreatePaymentRequest) error {
	if req.Amount.LessThanOrEqual(decimal.Zero) {
		return errors.New("payment amount must be positive")
	}

	totalAllocated := decimal.Zero
	for _, alloc := range req.Allocations {
		if alloc.Amount.LessThanOrEqual(decimal.Zero) {
			return errors.New("allocation amount must be positive")
		}
		totalAllocated = totalAllocated.Add(alloc.Amount)
	}
	if totalAllocated.GreaterThan(req.Amount) {
		return errors.New("total allocations exceed payment amount")
	}

	return nil
}

// PreparePayment creates a Payment from CreatePaymentRequest with defaults
func PreparePayment(tenantID string, req *CreatePaymentRequest) *Payment {
	payment := &Payment{
		TenantID:      tenantID,
		PaymentType:   req.PaymentType,
		ContactID:     req.ContactID,
		PaymentDate:   req.PaymentDate,
		Amount:        req.Amount,
		Currency:      req.Currency,
		ExchangeRate:  req.ExchangeRate,
		PaymentMethod: req.PaymentMethod,
		BankAccount:   req.BankAccount,
		Reference:     req.Reference,
		Notes:         req.Notes,
		CreatedAt:     time.Now(),
		CreatedBy:     req.UserID,
	}

	if payment.Currency == "" {
		payment.Currency = "EUR"
	}
	if payment.ExchangeRate.IsZero() {
		payment.ExchangeRate = decimal.NewFromInt(1)
	}
	if payment.PaymentDate.IsZero() {
		payment.PaymentDate = time.Now()
	}

	payment.BaseAmount = payment.Amount.Mul(payment.ExchangeRate).Round(2)
	return payment
}

// GeneratePaymentNumber generates the payment number
func GeneratePaymentNumber(paymentType PaymentType, seq int) string {
	prefix := "PMT"
	if paymentType == PaymentTypeMade {
		prefix = "OUT"
	}
	return prefix + "-" + padNumber(seq, 5)
}

func padNumber(n, width int) string {
	s := ""
	for i := 0; i < width; i++ {
		s = "0" + s
	}
	numStr := ""
	if n == 0 {
		numStr = "0"
	} else {
		for n > 0 {
			numStr = string(rune('0'+n%10)) + numStr
			n /= 10
		}
	}
	result := s + numStr
	return result[len(result)-width:]
}

func TestValidatePaymentRequest(t *testing.T) {
	tests := []struct {
		name        string
		req         *CreatePaymentRequest
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid request",
			req: &CreatePaymentRequest{
				PaymentType: PaymentTypeReceived,
				Amount:      decimal.NewFromFloat(100),
			},
			expectError: false,
		},
		{
			name: "zero amount",
			req: &CreatePaymentRequest{
				PaymentType: PaymentTypeReceived,
				Amount:      decimal.Zero,
			},
			expectError: true,
			errorMsg:    "payment amount must be positive",
		},
		{
			name: "negative amount",
			req: &CreatePaymentRequest{
				PaymentType: PaymentTypeReceived,
				Amount:      decimal.NewFromFloat(-100),
			},
			expectError: true,
			errorMsg:    "payment amount must be positive",
		},
		{
			name: "valid with allocations",
			req: &CreatePaymentRequest{
				PaymentType: PaymentTypeReceived,
				Amount:      decimal.NewFromFloat(100),
				Allocations: []AllocationRequest{
					{InvoiceID: "inv-1", Amount: decimal.NewFromFloat(50)},
					{InvoiceID: "inv-2", Amount: decimal.NewFromFloat(50)},
				},
			},
			expectError: false,
		},
		{
			name: "allocations exceed amount",
			req: &CreatePaymentRequest{
				PaymentType: PaymentTypeReceived,
				Amount:      decimal.NewFromFloat(100),
				Allocations: []AllocationRequest{
					{InvoiceID: "inv-1", Amount: decimal.NewFromFloat(60)},
					{InvoiceID: "inv-2", Amount: decimal.NewFromFloat(60)},
				},
			},
			expectError: true,
			errorMsg:    "total allocations exceed payment amount",
		},
		{
			name: "zero allocation amount",
			req: &CreatePaymentRequest{
				PaymentType: PaymentTypeReceived,
				Amount:      decimal.NewFromFloat(100),
				Allocations: []AllocationRequest{
					{InvoiceID: "inv-1", Amount: decimal.Zero},
				},
			},
			expectError: true,
			errorMsg:    "allocation amount must be positive",
		},
		{
			name: "negative allocation amount",
			req: &CreatePaymentRequest{
				PaymentType: PaymentTypeReceived,
				Amount:      decimal.NewFromFloat(100),
				Allocations: []AllocationRequest{
					{InvoiceID: "inv-1", Amount: decimal.NewFromFloat(-10)},
				},
			},
			expectError: true,
			errorMsg:    "allocation amount must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePaymentRequest(tt.req)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPreparePayment(t *testing.T) {
	t.Run("sets defaults for empty fields", func(t *testing.T) {
		req := &CreatePaymentRequest{
			PaymentType: PaymentTypeReceived,
			Amount:      decimal.NewFromFloat(100),
		}

		payment := PreparePayment("tenant-1", req)

		assert.Equal(t, "tenant-1", payment.TenantID)
		assert.Equal(t, PaymentTypeReceived, payment.PaymentType)
		assert.Equal(t, "EUR", payment.Currency)
		assert.True(t, payment.ExchangeRate.Equal(decimal.NewFromInt(1)))
		assert.True(t, payment.BaseAmount.Equal(decimal.NewFromFloat(100)))
		assert.False(t, payment.PaymentDate.IsZero())
	})

	t.Run("uses provided values", func(t *testing.T) {
		contactID := "contact-1"
		paymentDate := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)

		req := &CreatePaymentRequest{
			PaymentType:   PaymentTypeMade,
			ContactID:     &contactID,
			Amount:        decimal.NewFromFloat(200),
			Currency:      "USD",
			ExchangeRate:  decimal.NewFromFloat(0.92),
			PaymentDate:   paymentDate,
			PaymentMethod: "BANK_TRANSFER",
			BankAccount:   "EE123456789",
			Reference:     "REF-001",
			Notes:         "Test payment",
			UserID:        "user-1",
		}

		payment := PreparePayment("tenant-1", req)

		assert.Equal(t, PaymentTypeMade, payment.PaymentType)
		assert.Equal(t, "contact-1", *payment.ContactID)
		assert.Equal(t, "USD", payment.Currency)
		assert.True(t, payment.ExchangeRate.Equal(decimal.NewFromFloat(0.92)))
		assert.Equal(t, paymentDate, payment.PaymentDate)
		assert.Equal(t, "BANK_TRANSFER", payment.PaymentMethod)
		assert.Equal(t, "EE123456789", payment.BankAccount)
		assert.Equal(t, "REF-001", payment.Reference)
		assert.Equal(t, "Test payment", payment.Notes)
		assert.Equal(t, "user-1", payment.CreatedBy)
		// BaseAmount = 200 * 0.92 = 184.00
		assert.True(t, payment.BaseAmount.Equal(decimal.NewFromFloat(184)), "BaseAmount = %s", payment.BaseAmount)
	})
}

func TestGeneratePaymentNumber(t *testing.T) {
	tests := []struct {
		paymentType PaymentType
		seq         int
		expected    string
	}{
		{PaymentTypeReceived, 1, "PMT-00001"},
		{PaymentTypeReceived, 999, "PMT-00999"},
		{PaymentTypeReceived, 12345, "PMT-12345"},
		{PaymentTypeMade, 1, "OUT-00001"},
		{PaymentTypeMade, 42, "OUT-00042"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := GeneratePaymentNumber(tt.paymentType, tt.seq)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMockRepository_Create(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	payment := &Payment{
		ID:            "pay-1",
		TenantID:      "tenant-1",
		PaymentNumber: "PMT-00001",
		PaymentType:   PaymentTypeReceived,
		Amount:        decimal.NewFromFloat(100),
	}

	err := repo.Create(ctx, "test_schema", payment)
	require.NoError(t, err)

	// Verify it was stored
	stored, err := repo.GetByID(ctx, "test_schema", "tenant-1", "pay-1")
	require.NoError(t, err)
	assert.Equal(t, "pay-1", stored.ID)
}

func TestMockRepository_GetByID_NotFound(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	_, err := repo.GetByID(ctx, "test_schema", "tenant-1", "nonexistent")
	assert.ErrorIs(t, err, ErrPaymentNotFound)
}

func TestMockRepository_GetByID_WrongTenant(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	payment := &Payment{
		ID:       "pay-1",
		TenantID: "tenant-1",
	}
	_ = repo.Create(ctx, "test_schema", payment)

	_, err := repo.GetByID(ctx, "test_schema", "tenant-2", "pay-1")
	assert.ErrorIs(t, err, ErrPaymentNotFound)
}

func TestMockRepository_List(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	contactID := "contact-1"

	payments := []*Payment{
		{ID: "pay-1", TenantID: "tenant-1", PaymentType: PaymentTypeReceived, ContactID: &contactID},
		{ID: "pay-2", TenantID: "tenant-1", PaymentType: PaymentTypeMade, ContactID: &contactID},
		{ID: "pay-3", TenantID: "tenant-2", PaymentType: PaymentTypeReceived},
	}

	for _, p := range payments {
		_ = repo.Create(ctx, "test_schema", p)
	}

	t.Run("list all for tenant", func(t *testing.T) {
		result, err := repo.List(ctx, "test_schema", "tenant-1", nil)
		require.NoError(t, err)
		assert.Len(t, result, 2)
	})

	t.Run("filter by payment type", func(t *testing.T) {
		result, err := repo.List(ctx, "test_schema", "tenant-1", &PaymentFilter{
			PaymentType: PaymentTypeReceived,
		})
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, "pay-1", result[0].ID)
	})

	t.Run("filter by contact", func(t *testing.T) {
		result, err := repo.List(ctx, "test_schema", "tenant-1", &PaymentFilter{
			ContactID: "contact-1",
		})
		require.NoError(t, err)
		assert.Len(t, result, 2)
	})
}

func TestMockRepository_Allocations(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	alloc := &PaymentAllocation{
		ID:        "alloc-1",
		TenantID:  "tenant-1",
		PaymentID: "pay-1",
		InvoiceID: "inv-1",
		Amount:    decimal.NewFromFloat(50),
	}

	err := repo.CreateAllocation(ctx, "test_schema", alloc)
	require.NoError(t, err)

	allocations, err := repo.GetAllocations(ctx, "test_schema", "tenant-1", "pay-1")
	require.NoError(t, err)
	assert.Len(t, allocations, 1)
	assert.Equal(t, "inv-1", allocations[0].InvoiceID)
}

func TestMockRepository_GetNextPaymentNumber(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	seq1, err := repo.GetNextPaymentNumber(ctx, "test_schema", "tenant-1", PaymentTypeReceived)
	require.NoError(t, err)
	assert.Equal(t, 1, seq1)

	seq2, err := repo.GetNextPaymentNumber(ctx, "test_schema", "tenant-1", PaymentTypeReceived)
	require.NoError(t, err)
	assert.Equal(t, 2, seq2)
}

func TestPayment_FullyAllocated(t *testing.T) {
	payment := &Payment{
		Amount: decimal.NewFromFloat(100),
		Allocations: []PaymentAllocation{
			{Amount: decimal.NewFromFloat(100)},
		},
	}

	assert.True(t, payment.UnallocatedAmount().IsZero())
}

func TestPadNumber(t *testing.T) {
	tests := []struct {
		n        int
		width    int
		expected string
	}{
		{0, 5, "00000"},
		{1, 5, "00001"},
		{42, 5, "00042"},
		{12345, 5, "12345"},
		{123456, 5, "23456"}, // Truncates from left
	}

	for _, tt := range tests {
		result := padNumber(tt.n, tt.width)
		assert.Equal(t, tt.expected, result)
	}
}

func TestPaymentTypeConstants(t *testing.T) {
	assert.Equal(t, "RECEIVED", string(PaymentTypeReceived))
	assert.Equal(t, "MADE", string(PaymentTypeMade))
}

func TestErrPaymentNotFound(t *testing.T) {
	err := ErrPaymentNotFound
	assert.Equal(t, "payment not found", err.Error())
}

// MockInvoiceService implements InvoiceService for testing
type MockInvoiceService struct {
	recordPaymentCalls []struct {
		tenantID   string
		schemaName string
		invoiceID  string
		amount     decimal.Decimal
	}
	recordPaymentErr error
}

func (m *MockInvoiceService) RecordPayment(ctx context.Context, tenantID, schemaName, invoiceID string, amount decimal.Decimal) error {
	m.recordPaymentCalls = append(m.recordPaymentCalls, struct {
		tenantID   string
		schemaName string
		invoiceID  string
		amount     decimal.Decimal
	}{tenantID, schemaName, invoiceID, amount})
	return m.recordPaymentErr
}

func TestNewServiceWithRepository(t *testing.T) {
	repo := NewMockRepository()
	invoiceSvc := &MockInvoiceService{}
	service := NewServiceWithRepository(repo, invoiceSvc)

	assert.NotNil(t, service)
	assert.NotNil(t, service.repo)
}

func TestService_Create(t *testing.T) {
	tests := []struct {
		name      string
		req       *CreatePaymentRequest
		wantErr   bool
		errMsg    string
		checkFunc func(t *testing.T, p *Payment)
	}{
		{
			name: "valid payment received",
			req: &CreatePaymentRequest{
				PaymentType: PaymentTypeReceived,
				Amount:      decimal.NewFromFloat(100),
			},
			wantErr: false,
			checkFunc: func(t *testing.T, p *Payment) {
				assert.Equal(t, PaymentTypeReceived, p.PaymentType)
				assert.Equal(t, "PMT-00001", p.PaymentNumber)
				assert.Equal(t, "EUR", p.Currency)
				assert.True(t, p.Amount.Equal(decimal.NewFromFloat(100)))
			},
		},
		{
			name: "valid payment made",
			req: &CreatePaymentRequest{
				PaymentType: PaymentTypeMade,
				Amount:      decimal.NewFromFloat(200),
				Currency:    "USD",
			},
			wantErr: false,
			checkFunc: func(t *testing.T, p *Payment) {
				assert.Equal(t, PaymentTypeMade, p.PaymentType)
				assert.Equal(t, "OUT-00001", p.PaymentNumber)
				assert.Equal(t, "USD", p.Currency)
			},
		},
		{
			name: "zero amount",
			req: &CreatePaymentRequest{
				PaymentType: PaymentTypeReceived,
				Amount:      decimal.Zero,
			},
			wantErr: true,
			errMsg:  "payment amount must be positive",
		},
		{
			name: "negative amount",
			req: &CreatePaymentRequest{
				PaymentType: PaymentTypeReceived,
				Amount:      decimal.NewFromFloat(-100),
			},
			wantErr: true,
			errMsg:  "payment amount must be positive",
		},
		{
			name: "allocations exceed amount",
			req: &CreatePaymentRequest{
				PaymentType: PaymentTypeReceived,
				Amount:      decimal.NewFromFloat(100),
				Allocations: []AllocationRequest{
					{InvoiceID: "inv-1", Amount: decimal.NewFromFloat(60)},
					{InvoiceID: "inv-2", Amount: decimal.NewFromFloat(60)},
				},
			},
			wantErr: true,
			errMsg:  "total allocations exceed payment amount",
		},
		{
			name: "zero allocation amount",
			req: &CreatePaymentRequest{
				PaymentType: PaymentTypeReceived,
				Amount:      decimal.NewFromFloat(100),
				Allocations: []AllocationRequest{
					{InvoiceID: "inv-1", Amount: decimal.Zero},
				},
			},
			wantErr: true,
			errMsg:  "allocation amount must be positive",
		},
		{
			name: "valid with allocations",
			req: &CreatePaymentRequest{
				PaymentType: PaymentTypeReceived,
				Amount:      decimal.NewFromFloat(100),
				Allocations: []AllocationRequest{
					{InvoiceID: "inv-1", Amount: decimal.NewFromFloat(50)},
					{InvoiceID: "inv-2", Amount: decimal.NewFromFloat(50)},
				},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, p *Payment) {
				assert.Len(t, p.Allocations, 2)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockRepository()
			invoiceSvc := &MockInvoiceService{}
			service := NewServiceWithRepository(repo, invoiceSvc)
			ctx := context.Background()

			payment, err := service.Create(ctx, "tenant-1", "test_schema", tt.req)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}
			require.NoError(t, err)
			if tt.checkFunc != nil {
				tt.checkFunc(t, payment)
			}
		})
	}
}

func TestService_Create_WithExchangeRate(t *testing.T) {
	repo := NewMockRepository()
	invoiceSvc := &MockInvoiceService{}
	service := NewServiceWithRepository(repo, invoiceSvc)
	ctx := context.Background()

	req := &CreatePaymentRequest{
		PaymentType:  PaymentTypeReceived,
		Amount:       decimal.NewFromFloat(100),
		Currency:     "USD",
		ExchangeRate: decimal.NewFromFloat(0.92),
	}

	payment, err := service.Create(ctx, "tenant-1", "test_schema", req)
	require.NoError(t, err)

	// BaseAmount = 100 * 0.92 = 92.00
	assert.True(t, payment.BaseAmount.Equal(decimal.NewFromFloat(92)), "BaseAmount = %s", payment.BaseAmount)
}

func TestService_Create_CallsInvoicing(t *testing.T) {
	repo := NewMockRepository()
	invoiceSvc := &MockInvoiceService{}
	service := NewServiceWithRepository(repo, invoiceSvc)
	ctx := context.Background()

	req := &CreatePaymentRequest{
		PaymentType: PaymentTypeReceived,
		Amount:      decimal.NewFromFloat(100),
		Allocations: []AllocationRequest{
			{InvoiceID: "inv-1", Amount: decimal.NewFromFloat(50)},
			{InvoiceID: "inv-2", Amount: decimal.NewFromFloat(30)},
		},
	}

	_, err := service.Create(ctx, "tenant-1", "test_schema", req)
	require.NoError(t, err)

	// Verify invoicing was called for each allocation
	assert.Len(t, invoiceSvc.recordPaymentCalls, 2)
	assert.Equal(t, "inv-1", invoiceSvc.recordPaymentCalls[0].invoiceID)
	assert.True(t, invoiceSvc.recordPaymentCalls[0].amount.Equal(decimal.NewFromFloat(50)))
	assert.Equal(t, "inv-2", invoiceSvc.recordPaymentCalls[1].invoiceID)
	assert.True(t, invoiceSvc.recordPaymentCalls[1].amount.Equal(decimal.NewFromFloat(30)))
}

func TestService_Create_RepositoryError(t *testing.T) {
	repo := NewMockRepository()
	repo.createErr = errors.New("database error")
	service := NewServiceWithRepository(repo, nil)
	ctx := context.Background()

	_, err := service.Create(ctx, "tenant-1", "test_schema", &CreatePaymentRequest{
		PaymentType: PaymentTypeReceived,
		Amount:      decimal.NewFromFloat(100),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "insert payment")
}

func TestService_GetByID(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil)
	ctx := context.Background()

	// Create a payment first
	payment := &Payment{
		ID:            "pay-123",
		TenantID:      "tenant-1",
		PaymentNumber: "PMT-00001",
		PaymentType:   PaymentTypeReceived,
		Amount:        decimal.NewFromFloat(100),
		Currency:      "EUR",
	}
	repo.payments[payment.ID] = payment

	// Add an allocation
	alloc := &PaymentAllocation{
		ID:        "alloc-1",
		TenantID:  "tenant-1",
		PaymentID: "pay-123",
		InvoiceID: "inv-1",
		Amount:    decimal.NewFromFloat(50),
	}
	_ = repo.CreateAllocation(ctx, "test_schema", alloc)

	result, err := service.GetByID(ctx, "tenant-1", "test_schema", "pay-123")
	require.NoError(t, err)
	assert.Equal(t, "pay-123", result.ID)
	assert.Len(t, result.Allocations, 1)
}

func TestService_GetByID_NotFound(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil)
	ctx := context.Background()

	_, err := service.GetByID(ctx, "tenant-1", "test_schema", "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get payment")
}

func TestService_List(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil)
	ctx := context.Background()

	contactID := "contact-1"
	payments := []*Payment{
		{ID: "pay-1", TenantID: "tenant-1", PaymentType: PaymentTypeReceived, ContactID: &contactID},
		{ID: "pay-2", TenantID: "tenant-1", PaymentType: PaymentTypeMade, ContactID: &contactID},
		{ID: "pay-3", TenantID: "tenant-2", PaymentType: PaymentTypeReceived},
	}
	for _, p := range payments {
		repo.payments[p.ID] = p
	}

	t.Run("list all for tenant", func(t *testing.T) {
		result, err := service.List(ctx, "tenant-1", "test_schema", nil)
		require.NoError(t, err)
		assert.Len(t, result, 2)
	})

	t.Run("filter by payment type", func(t *testing.T) {
		result, err := service.List(ctx, "tenant-1", "test_schema", &PaymentFilter{
			PaymentType: PaymentTypeReceived,
		})
		require.NoError(t, err)
		assert.Len(t, result, 1)
	})
}

func TestService_List_Error(t *testing.T) {
	repo := NewMockRepository()
	repo.listErr = errors.New("database error")
	service := NewServiceWithRepository(repo, nil)
	ctx := context.Background()

	_, err := service.List(ctx, "tenant-1", "test_schema", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list payments")
}

func TestService_AllocateToInvoice(t *testing.T) {
	repo := NewMockRepository()
	invoiceSvc := &MockInvoiceService{}
	service := NewServiceWithRepository(repo, invoiceSvc)
	ctx := context.Background()

	// Create a payment with some unallocated amount
	payment := &Payment{
		ID:            "pay-123",
		TenantID:      "tenant-1",
		PaymentNumber: "PMT-00001",
		PaymentType:   PaymentTypeReceived,
		Amount:        decimal.NewFromFloat(100),
		Currency:      "EUR",
	}
	repo.payments[payment.ID] = payment

	// Allocate 50 to an invoice
	err := service.AllocateToInvoice(ctx, "tenant-1", "test_schema", "pay-123", "inv-1", decimal.NewFromFloat(50))
	require.NoError(t, err)

	// Verify allocation was created
	allocations, _ := repo.GetAllocations(ctx, "test_schema", "tenant-1", "pay-123")
	assert.Len(t, allocations, 1)
	assert.Equal(t, "inv-1", allocations[0].InvoiceID)

	// Verify invoicing was called
	assert.Len(t, invoiceSvc.recordPaymentCalls, 1)
}

func TestService_AllocateToInvoice_ExceedsBalance(t *testing.T) {
	repo := NewMockRepository()
	invoiceSvc := &MockInvoiceService{}
	service := NewServiceWithRepository(repo, invoiceSvc)
	ctx := context.Background()

	// Create a payment with 100 and allocate 80
	payment := &Payment{
		ID:       "pay-123",
		TenantID: "tenant-1",
		Amount:   decimal.NewFromFloat(100),
	}
	repo.payments[payment.ID] = payment

	// Allocate 80
	alloc := &PaymentAllocation{
		ID:        "alloc-1",
		PaymentID: "pay-123",
		Amount:    decimal.NewFromFloat(80),
	}
	_ = repo.CreateAllocation(ctx, "test_schema", alloc)

	// Try to allocate 30 more (exceeds 20 remaining)
	err := service.AllocateToInvoice(ctx, "tenant-1", "test_schema", "pay-123", "inv-2", decimal.NewFromFloat(30))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "amount exceeds unallocated balance")
}

func TestService_AllocateToInvoice_PaymentNotFound(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil)
	ctx := context.Background()

	err := service.AllocateToInvoice(ctx, "tenant-1", "test_schema", "nonexistent", "inv-1", decimal.NewFromFloat(50))
	require.Error(t, err)
}

func TestService_GetUnallocatedPayments_Success(t *testing.T) {
	repo := NewMockRepository()
	// Add some payments
	payment1 := &Payment{
		ID:          "pay-1",
		TenantID:    "tenant-1",
		PaymentType: PaymentTypeReceived,
		Amount:      decimal.NewFromInt(100),
	}
	payment2 := &Payment{
		ID:          "pay-2",
		TenantID:    "tenant-1",
		PaymentType: PaymentTypeReceived,
		Amount:      decimal.NewFromInt(50),
	}
	_ = repo.Create(context.Background(), "test_schema", payment1)
	_ = repo.Create(context.Background(), "test_schema", payment2)
	// Fully allocate payment1
	_ = repo.CreateAllocation(context.Background(), "test_schema", &PaymentAllocation{
		ID:        "alloc-1",
		TenantID:  "tenant-1",
		PaymentID: "pay-1",
		InvoiceID: "inv-1",
		Amount:    decimal.NewFromInt(100),
	})

	service := NewServiceWithRepository(repo, nil)
	ctx := context.Background()

	result, err := service.GetUnallocatedPayments(ctx, "tenant-1", "test_schema", PaymentTypeReceived)
	require.NoError(t, err)
	assert.Len(t, result, 1) // Only payment2 should be unallocated
	assert.Equal(t, "pay-2", result[0].ID)
}

func TestService_GetUnallocatedPayments_Error(t *testing.T) {
	repo := NewMockRepository()
	repo.getUnallocatedErr = errors.New("repository error")
	service := NewServiceWithRepository(repo, nil)
	ctx := context.Background()

	_, err := service.GetUnallocatedPayments(ctx, "tenant-1", "test_schema", PaymentTypeReceived)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "repository error")
}

func TestService_GetByID_RepositoryError(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil)
	ctx := context.Background()

	// Create a payment first
	payment := &Payment{
		ID:       "pay-123",
		TenantID: "tenant-1",
	}
	repo.payments[payment.ID] = payment

	// Force GetByID error
	repo.getErr = errors.New("database error")

	_, err := service.GetByID(ctx, "tenant-1", "test_schema", "pay-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get payment")
}

func TestService_GetByID_GetAllocationsError(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil)
	ctx := context.Background()

	// Create a payment first
	payment := &Payment{
		ID:       "pay-123",
		TenantID: "tenant-1",
	}
	repo.payments[payment.ID] = payment

	// Force GetAllocations error
	repo.getAllocErr = errors.New("allocations db error")

	_, err := service.GetByID(ctx, "tenant-1", "test_schema", "pay-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get allocations")
}

func TestService_AllocateToInvoice_CreateAllocationError(t *testing.T) {
	repo := NewMockRepository()
	invoiceSvc := &MockInvoiceService{}
	service := NewServiceWithRepository(repo, invoiceSvc)
	ctx := context.Background()

	payment := &Payment{
		ID:       "pay-123",
		TenantID: "tenant-1",
		Amount:   decimal.NewFromFloat(100),
	}
	repo.payments[payment.ID] = payment
	repo.createAllocErr = errors.New("database error")

	err := service.AllocateToInvoice(ctx, "tenant-1", "test_schema", "pay-123", "inv-1", decimal.NewFromFloat(50))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insert allocation")
}

func TestService_AllocateToInvoice_InvoicingError(t *testing.T) {
	repo := NewMockRepository()
	invoiceSvc := &MockInvoiceService{recordPaymentErr: errors.New("invoice error")}
	service := NewServiceWithRepository(repo, invoiceSvc)
	ctx := context.Background()

	payment := &Payment{
		ID:       "pay-123",
		TenantID: "tenant-1",
		Amount:   decimal.NewFromFloat(100),
	}
	repo.payments[payment.ID] = payment

	err := service.AllocateToInvoice(ctx, "tenant-1", "test_schema", "pay-123", "inv-1", decimal.NewFromFloat(50))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update invoice payment")
}

func TestService_Create_GenerateNumberError(t *testing.T) {
	repo := NewMockRepository()
	repo.getNextNumErr = errors.New("sequence error")
	service := NewServiceWithRepository(repo, nil)
	ctx := context.Background()

	_, err := service.Create(ctx, "tenant-1", "test_schema", &CreatePaymentRequest{
		PaymentType: PaymentTypeReceived,
		Amount:      decimal.NewFromFloat(100),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "generate payment number")
}

func TestService_Create_AllocationError(t *testing.T) {
	repo := NewMockRepository()
	repo.createAllocErr = errors.New("allocation db error")
	service := NewServiceWithRepository(repo, nil)
	ctx := context.Background()
	contactID := "contact-1"

	_, err := service.Create(ctx, "tenant-1", "test_schema", &CreatePaymentRequest{
		PaymentType: PaymentTypeReceived,
		ContactID:   &contactID,
		Amount:      decimal.NewFromFloat(100),
		Allocations: []AllocationRequest{
			{InvoiceID: "inv-1", Amount: decimal.NewFromFloat(50)},
		},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "insert allocation")
}

func TestService_Create_InvoicingError(t *testing.T) {
	repo := NewMockRepository()
	invoiceSvc := &MockInvoiceService{
		recordPaymentErr: errors.New("invoicing error"),
	}
	service := NewServiceWithRepository(repo, invoiceSvc)
	ctx := context.Background()
	contactID := "contact-1"

	// This should succeed even though invoicing fails (it's logged but not fatal)
	result, err := service.Create(ctx, "tenant-1", "test_schema", &CreatePaymentRequest{
		PaymentType: PaymentTypeReceived,
		ContactID:   &contactID,
		Amount:      decimal.NewFromFloat(100),
		Allocations: []AllocationRequest{
			{InvoiceID: "inv-1", Amount: decimal.NewFromFloat(50)},
		},
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
}
