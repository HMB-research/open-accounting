package quotes

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockRepository is a mock implementation of the Repository interface
type MockRepository struct {
	quotes          map[string]*Quote
	nextNumber      int
	CreateErr       error
	GetByIDErr      error
	ListErr         error
	UpdateErr       error
	UpdateStatErr   error
	DeleteErr       error
	GenNumberErr    error
	SetConvOrderErr error
	SetConvInvErr   error
}

// NewMockRepository creates a new mock repository
func NewMockRepository() *MockRepository {
	return &MockRepository{
		quotes:     make(map[string]*Quote),
		nextNumber: 1,
	}
}

func (m *MockRepository) Create(ctx context.Context, schemaName string, quote *Quote) error {
	if m.CreateErr != nil {
		return m.CreateErr
	}
	m.quotes[quote.ID] = quote
	return nil
}

func (m *MockRepository) GetByID(ctx context.Context, schemaName, tenantID, quoteID string) (*Quote, error) {
	if m.GetByIDErr != nil {
		return nil, m.GetByIDErr
	}
	quote, ok := m.quotes[quoteID]
	if !ok {
		return nil, ErrQuoteNotFound
	}
	return quote, nil
}

func (m *MockRepository) List(ctx context.Context, schemaName, tenantID string, filter *QuoteFilter) ([]Quote, error) {
	if m.ListErr != nil {
		return nil, m.ListErr
	}
	var result []Quote
	for _, quote := range m.quotes {
		if quote.TenantID == tenantID {
			result = append(result, *quote)
		}
	}
	return result, nil
}

func (m *MockRepository) Update(ctx context.Context, schemaName string, quote *Quote) error {
	if m.UpdateErr != nil {
		return m.UpdateErr
	}
	m.quotes[quote.ID] = quote
	return nil
}

func (m *MockRepository) UpdateStatus(ctx context.Context, schemaName, tenantID, quoteID string, status QuoteStatus) error {
	if m.UpdateStatErr != nil {
		return m.UpdateStatErr
	}
	if quote, ok := m.quotes[quoteID]; ok {
		quote.Status = status
	}
	return nil
}

func (m *MockRepository) Delete(ctx context.Context, schemaName, tenantID, quoteID string) error {
	if m.DeleteErr != nil {
		return m.DeleteErr
	}
	delete(m.quotes, quoteID)
	return nil
}

func (m *MockRepository) GenerateNumber(ctx context.Context, schemaName, tenantID string) (string, error) {
	if m.GenNumberErr != nil {
		return "", m.GenNumberErr
	}
	num := m.nextNumber
	m.nextNumber++
	return "QUO-2024-" + string(rune('0'+num)), nil
}

func (m *MockRepository) SetConvertedToOrder(ctx context.Context, schemaName, tenantID, quoteID, orderID string) error {
	if m.SetConvOrderErr != nil {
		return m.SetConvOrderErr
	}
	if quote, ok := m.quotes[quoteID]; ok {
		quote.ConvertedToOrderID = &orderID
		quote.Status = QuoteStatusConverted
	}
	return nil
}

func (m *MockRepository) SetConvertedToInvoice(ctx context.Context, schemaName, tenantID, quoteID, invoiceID string) error {
	if m.SetConvInvErr != nil {
		return m.SetConvInvErr
	}
	if quote, ok := m.quotes[quoteID]; ok {
		quote.ConvertedToInvoiceID = &invoiceID
		quote.Status = QuoteStatusConverted
	}
	return nil
}

// AddQuote adds a quote to the mock repository for testing
func (m *MockRepository) AddQuote(quote *Quote) {
	m.quotes[quote.ID] = quote
}

func TestNewServiceWithRepository(t *testing.T) {
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)
	require.NotNil(t, svc)
	assert.NotNil(t, svc.repo)
}

func TestService_Create_Success(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	futureDate := time.Now().AddDate(0, 1, 0)
	req := &CreateQuoteRequest{
		ContactID:  "contact-1",
		QuoteDate:  time.Now(),
		ValidUntil: &futureDate,
		Currency:   "EUR",
		Lines: []CreateQuoteLineRequest{
			{
				Description: "Product A",
				Quantity:    decimal.NewFromInt(2),
				UnitPrice:   decimal.NewFromFloat(100.00),
				VATRate:     decimal.NewFromInt(22),
			},
		},
		UserID: "user-1",
	}

	quote, err := svc.Create(ctx, "tenant-1", "test_schema", req)
	require.NoError(t, err)
	require.NotNil(t, quote)

	assert.NotEmpty(t, quote.ID)
	assert.Equal(t, "tenant-1", quote.TenantID)
	assert.Equal(t, "contact-1", quote.ContactID)
	assert.Equal(t, QuoteStatusDraft, quote.Status)
	assert.NotEmpty(t, quote.QuoteNumber)
	assert.Len(t, quote.Lines, 1)
}

func TestService_Create_DefaultValues(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	req := &CreateQuoteRequest{
		ContactID: "contact-1",
		// No currency, no exchange rate, no quote date
		Lines: []CreateQuoteLineRequest{
			{
				Description: "Product A",
				Quantity:    decimal.NewFromInt(1),
				UnitPrice:   decimal.NewFromFloat(50.00),
				VATRate:     decimal.NewFromInt(22),
			},
		},
		UserID: "user-1",
	}

	quote, err := svc.Create(ctx, "tenant-1", "test_schema", req)
	require.NoError(t, err)

	assert.Equal(t, "EUR", quote.Currency)
	assert.True(t, quote.ExchangeRate.Equal(decimal.NewFromInt(1)))
	assert.False(t, quote.QuoteDate.IsZero())
}

func TestService_Create_ValidationError(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	req := &CreateQuoteRequest{
		ContactID: "", // Missing contact
		Lines: []CreateQuoteLineRequest{
			{
				Description: "Product A",
				Quantity:    decimal.NewFromInt(1),
				UnitPrice:   decimal.NewFromFloat(50.00),
				VATRate:     decimal.NewFromInt(22),
			},
		},
	}

	quote, err := svc.Create(ctx, "tenant-1", "test_schema", req)
	require.Error(t, err)
	assert.Nil(t, quote)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestService_Create_GenerateNumberError(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.GenNumberErr = errors.New("database error")
	svc := NewServiceWithRepository(repo)

	req := &CreateQuoteRequest{
		ContactID: "contact-1",
		Lines: []CreateQuoteLineRequest{
			{
				Description: "Product A",
				Quantity:    decimal.NewFromInt(1),
				UnitPrice:   decimal.NewFromFloat(50.00),
				VATRate:     decimal.NewFromInt(22),
			},
		},
	}

	quote, err := svc.Create(ctx, "tenant-1", "test_schema", req)
	require.Error(t, err)
	assert.Nil(t, quote)
	assert.Contains(t, err.Error(), "generate quote number")
}

func TestService_Create_RepositoryError(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.CreateErr = errors.New("database error")
	svc := NewServiceWithRepository(repo)

	req := &CreateQuoteRequest{
		ContactID: "contact-1",
		Lines: []CreateQuoteLineRequest{
			{
				Description: "Product A",
				Quantity:    decimal.NewFromInt(1),
				UnitPrice:   decimal.NewFromFloat(50.00),
				VATRate:     decimal.NewFromInt(22),
			},
		},
	}

	quote, err := svc.Create(ctx, "tenant-1", "test_schema", req)
	require.Error(t, err)
	assert.Nil(t, quote)
	assert.Contains(t, err.Error(), "create quote")
}

func TestService_GetByID_Success(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	existingQuote := &Quote{
		ID:       "quote-1",
		TenantID: "tenant-1",
		Status:   QuoteStatusDraft,
	}
	repo.AddQuote(existingQuote)

	quote, err := svc.GetByID(ctx, "tenant-1", "test_schema", "quote-1")
	require.NoError(t, err)
	assert.Equal(t, "quote-1", quote.ID)
}

func TestService_GetByID_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	quote, err := svc.GetByID(ctx, "tenant-1", "test_schema", "nonexistent")
	require.Error(t, err)
	assert.Nil(t, quote)
}

func TestService_GetByID_Error(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.GetByIDErr = errors.New("database error")
	svc := NewServiceWithRepository(repo)

	quote, err := svc.GetByID(ctx, "tenant-1", "test_schema", "quote-1")
	require.Error(t, err)
	assert.Nil(t, quote)
	assert.Contains(t, err.Error(), "get quote")
}

func TestService_List_Success(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	repo.AddQuote(&Quote{ID: "quote-1", TenantID: "tenant-1"})
	repo.AddQuote(&Quote{ID: "quote-2", TenantID: "tenant-1"})
	repo.AddQuote(&Quote{ID: "quote-3", TenantID: "tenant-2"}) // Different tenant

	quotes, err := svc.List(ctx, "tenant-1", "test_schema", nil)
	require.NoError(t, err)
	assert.Len(t, quotes, 2)
}

func TestService_List_Error(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.ListErr = errors.New("database error")
	svc := NewServiceWithRepository(repo)

	quotes, err := svc.List(ctx, "tenant-1", "test_schema", nil)
	require.Error(t, err)
	assert.Nil(t, quotes)
	assert.Contains(t, err.Error(), "list quotes")
}

func TestService_Update_Success(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	existingQuote := &Quote{
		ID:        "quote-1",
		TenantID:  "tenant-1",
		ContactID: "contact-1",
		Status:    QuoteStatusDraft,
		Lines: []QuoteLine{
			{
				ID:          "line-1",
				Description: "Old Product",
				Quantity:    decimal.NewFromInt(1),
				UnitPrice:   decimal.NewFromFloat(100.00),
			},
		},
	}
	repo.AddQuote(existingQuote)

	futureDate := time.Now().AddDate(0, 2, 0)
	req := &UpdateQuoteRequest{
		ContactID:  "contact-2",
		QuoteDate:  time.Now(),
		ValidUntil: &futureDate,
		Currency:   "USD",
		Lines: []CreateQuoteLineRequest{
			{
				Description: "New Product",
				Quantity:    decimal.NewFromInt(3),
				UnitPrice:   decimal.NewFromFloat(150.00),
				VATRate:     decimal.NewFromInt(22),
			},
		},
	}

	quote, err := svc.Update(ctx, "tenant-1", "test_schema", "quote-1", req)
	require.NoError(t, err)
	assert.Equal(t, "contact-2", quote.ContactID)
	assert.Equal(t, "USD", quote.Currency)
	assert.Len(t, quote.Lines, 1)
	assert.Equal(t, "New Product", quote.Lines[0].Description)
}

func TestService_Update_OnlyDrafts(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		status  QuoteStatus
		wantErr bool
	}{
		{"draft - can update", QuoteStatusDraft, false},
		{"sent - cannot update", QuoteStatusSent, true},
		{"accepted - cannot update", QuoteStatusAccepted, true},
		{"rejected - cannot update", QuoteStatusRejected, true},
		{"expired - cannot update", QuoteStatusExpired, true},
		{"converted - cannot update", QuoteStatusConverted, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockRepository()
			svc := NewServiceWithRepository(repo)

			existingQuote := &Quote{
				ID:        "quote-1",
				TenantID:  "tenant-1",
				ContactID: "contact-1",
				Status:    tt.status,
			}
			repo.AddQuote(existingQuote)

			req := &UpdateQuoteRequest{
				ContactID: "contact-2",
				QuoteDate: time.Now(),
				Lines: []CreateQuoteLineRequest{
					{
						Description: "Product",
						Quantity:    decimal.NewFromInt(1),
						UnitPrice:   decimal.NewFromFloat(100.00),
						VATRate:     decimal.NewFromInt(22),
					},
				},
			}

			_, err := svc.Update(ctx, "tenant-1", "test_schema", "quote-1", req)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "only draft quotes")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestService_Update_DefaultValues(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	existingQuote := &Quote{
		ID:       "quote-1",
		TenantID: "tenant-1",
		Status:   QuoteStatusDraft,
	}
	repo.AddQuote(existingQuote)

	req := &UpdateQuoteRequest{
		ContactID: "contact-1",
		QuoteDate: time.Now(),
		Currency:  "", // Should default to EUR
		// ExchangeRate is zero - should default to 1
		Lines: []CreateQuoteLineRequest{
			{
				Description: "Product",
				Quantity:    decimal.NewFromInt(1),
				UnitPrice:   decimal.NewFromFloat(100.00),
				VATRate:     decimal.NewFromInt(22),
			},
		},
	}

	quote, err := svc.Update(ctx, "tenant-1", "test_schema", "quote-1", req)
	require.NoError(t, err)
	assert.Equal(t, "EUR", quote.Currency)
	assert.True(t, quote.ExchangeRate.Equal(decimal.NewFromInt(1)))
}

func TestService_Send_Success(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	repo.AddQuote(&Quote{
		ID:       "quote-1",
		TenantID: "tenant-1",
		Status:   QuoteStatusDraft,
	})

	err := svc.Send(ctx, "tenant-1", "test_schema", "quote-1")
	require.NoError(t, err)
	assert.Equal(t, QuoteStatusSent, repo.quotes["quote-1"].Status)
}

func TestService_Send_NotDraft(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	repo.AddQuote(&Quote{
		ID:       "quote-1",
		TenantID: "tenant-1",
		Status:   QuoteStatusSent, // Already sent
	})

	err := svc.Send(ctx, "tenant-1", "test_schema", "quote-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in draft status")
}

func TestService_Accept_Success(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name   string
		status QuoteStatus
	}{
		{"from sent", QuoteStatusSent},
		{"from draft", QuoteStatusDraft},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockRepository()
			svc := NewServiceWithRepository(repo)

			repo.AddQuote(&Quote{
				ID:       "quote-1",
				TenantID: "tenant-1",
				Status:   tt.status,
			})

			err := svc.Accept(ctx, "tenant-1", "test_schema", "quote-1")
			require.NoError(t, err)
			assert.Equal(t, QuoteStatusAccepted, repo.quotes["quote-1"].Status)
		})
	}
}

func TestService_Accept_InvalidStatus(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	repo.AddQuote(&Quote{
		ID:       "quote-1",
		TenantID: "tenant-1",
		Status:   QuoteStatusRejected, // Cannot accept rejected
	})

	err := svc.Accept(ctx, "tenant-1", "test_schema", "quote-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be accepted")
}

func TestService_Reject_Success(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name   string
		status QuoteStatus
	}{
		{"from sent", QuoteStatusSent},
		{"from draft", QuoteStatusDraft},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockRepository()
			svc := NewServiceWithRepository(repo)

			repo.AddQuote(&Quote{
				ID:       "quote-1",
				TenantID: "tenant-1",
				Status:   tt.status,
			})

			err := svc.Reject(ctx, "tenant-1", "test_schema", "quote-1")
			require.NoError(t, err)
			assert.Equal(t, QuoteStatusRejected, repo.quotes["quote-1"].Status)
		})
	}
}

func TestService_Reject_InvalidStatus(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	repo.AddQuote(&Quote{
		ID:       "quote-1",
		TenantID: "tenant-1",
		Status:   QuoteStatusAccepted, // Cannot reject accepted
	})

	err := svc.Reject(ctx, "tenant-1", "test_schema", "quote-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be rejected")
}

func TestService_Delete_Success(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	repo.AddQuote(&Quote{
		ID:       "quote-1",
		TenantID: "tenant-1",
		Status:   QuoteStatusDraft,
	})

	err := svc.Delete(ctx, "tenant-1", "test_schema", "quote-1")
	require.NoError(t, err)
	_, exists := repo.quotes["quote-1"]
	assert.False(t, exists)
}

func TestService_Delete_Error(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.DeleteErr = errors.New("database error")
	svc := NewServiceWithRepository(repo)

	err := svc.Delete(ctx, "tenant-1", "test_schema", "quote-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete quote")
}

func TestService_ConvertToOrder_Success(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	repo.AddQuote(&Quote{
		ID:       "quote-1",
		TenantID: "tenant-1",
		Status:   QuoteStatusAccepted,
	})

	err := svc.ConvertToOrder(ctx, "tenant-1", "test_schema", "quote-1", "order-1")
	require.NoError(t, err)
	assert.NotNil(t, repo.quotes["quote-1"].ConvertedToOrderID)
	assert.Equal(t, "order-1", *repo.quotes["quote-1"].ConvertedToOrderID)
	assert.Equal(t, QuoteStatusConverted, repo.quotes["quote-1"].Status)
}

func TestService_ConvertToOrder_Error(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.SetConvOrderErr = errors.New("database error")
	svc := NewServiceWithRepository(repo)

	err := svc.ConvertToOrder(ctx, "tenant-1", "test_schema", "quote-1", "order-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "convert to order")
}

func TestService_ConvertToInvoice_Success(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	repo.AddQuote(&Quote{
		ID:       "quote-1",
		TenantID: "tenant-1",
		Status:   QuoteStatusAccepted,
	})

	err := svc.ConvertToInvoice(ctx, "tenant-1", "test_schema", "quote-1", "invoice-1")
	require.NoError(t, err)
	assert.NotNil(t, repo.quotes["quote-1"].ConvertedToInvoiceID)
	assert.Equal(t, "invoice-1", *repo.quotes["quote-1"].ConvertedToInvoiceID)
	assert.Equal(t, QuoteStatusConverted, repo.quotes["quote-1"].Status)
}

func TestService_ConvertToInvoice_Error(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.SetConvInvErr = errors.New("database error")
	svc := NewServiceWithRepository(repo)

	err := svc.ConvertToInvoice(ctx, "tenant-1", "test_schema", "quote-1", "invoice-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "convert to invoice")
}

func TestService_StatusTransitions_GetErrors(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name   string
		method func(*Service, context.Context, string, string, string) error
	}{
		{"send not found", func(s *Service, ctx context.Context, t, sc, id string) error {
			return s.Send(ctx, t, sc, id)
		}},
		{"accept not found", func(s *Service, ctx context.Context, t, sc, id string) error {
			return s.Accept(ctx, t, sc, id)
		}},
		{"reject not found", func(s *Service, ctx context.Context, t, sc, id string) error {
			return s.Reject(ctx, t, sc, id)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockRepository()
			repo.GetByIDErr = errors.New("not found")
			svc := NewServiceWithRepository(repo)

			err := tt.method(svc, ctx, "tenant-1", "test_schema", "quote-1")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "get quote")
		})
	}
}

func TestService_StatusTransitions_UpdateErrors(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name   string
		method func(*Service, context.Context, string, string, string) error
		status QuoteStatus
	}{
		{"send update error", func(s *Service, ctx context.Context, t, sc, id string) error {
			return s.Send(ctx, t, sc, id)
		}, QuoteStatusDraft},
		{"accept update error", func(s *Service, ctx context.Context, t, sc, id string) error {
			return s.Accept(ctx, t, sc, id)
		}, QuoteStatusSent},
		{"reject update error", func(s *Service, ctx context.Context, t, sc, id string) error {
			return s.Reject(ctx, t, sc, id)
		}, QuoteStatusSent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockRepository()
			repo.UpdateStatErr = errors.New("database error")
			repo.AddQuote(&Quote{
				ID:       "quote-1",
				TenantID: "tenant-1",
				Status:   tt.status,
			})
			svc := NewServiceWithRepository(repo)

			err := tt.method(svc, ctx, "tenant-1", "test_schema", "quote-1")
			require.Error(t, err)
		})
	}
}

func TestService_Update_GetError(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.GetByIDErr = errors.New("database error")
	svc := NewServiceWithRepository(repo)

	req := &UpdateQuoteRequest{
		ContactID: "contact-1",
		Lines: []CreateQuoteLineRequest{
			{
				Description: "Product",
				Quantity:    decimal.NewFromInt(1),
				UnitPrice:   decimal.NewFromFloat(100.00),
				VATRate:     decimal.NewFromInt(22),
			},
		},
	}

	quote, err := svc.Update(ctx, "tenant-1", "test_schema", "quote-1", req)
	require.Error(t, err)
	assert.Nil(t, quote)
	assert.Contains(t, err.Error(), "get quote")
}

func TestService_Update_ValidationError(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	existingQuote := &Quote{
		ID:       "quote-1",
		TenantID: "tenant-1",
		Status:   QuoteStatusDraft,
	}
	repo.AddQuote(existingQuote)

	req := &UpdateQuoteRequest{
		ContactID: "contact-1",
		Lines: []CreateQuoteLineRequest{
			{
				Description: "", // Missing description
				Quantity:    decimal.NewFromInt(1),
				UnitPrice:   decimal.NewFromFloat(100.00),
				VATRate:     decimal.NewFromInt(22),
			},
		},
	}

	quote, err := svc.Update(ctx, "tenant-1", "test_schema", "quote-1", req)
	require.Error(t, err)
	assert.Nil(t, quote)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestService_Update_RepositoryError(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.UpdateErr = errors.New("database error")
	svc := NewServiceWithRepository(repo)

	existingQuote := &Quote{
		ID:       "quote-1",
		TenantID: "tenant-1",
		Status:   QuoteStatusDraft,
	}
	repo.AddQuote(existingQuote)

	req := &UpdateQuoteRequest{
		ContactID: "contact-1",
		QuoteDate: time.Now(),
		Lines: []CreateQuoteLineRequest{
			{
				Description: "Product",
				Quantity:    decimal.NewFromInt(1),
				UnitPrice:   decimal.NewFromFloat(100.00),
				VATRate:     decimal.NewFromInt(22),
			},
		},
	}

	quote, err := svc.Update(ctx, "tenant-1", "test_schema", "quote-1", req)
	require.Error(t, err)
	assert.Nil(t, quote)
	assert.Contains(t, err.Error(), "update quote")
}
