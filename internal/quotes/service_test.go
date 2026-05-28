package quotes

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/contacts"
)

// MockRepository implements Repository for testing
type MockRepository struct {
	Quotes        map[string]*Quote
	NextNumber    string
	GenerateErr   error
	CreateErr     error
	GetErr        error
	ListErr       error
	UpdateErr     error
	UpdateStatErr error
	DeleteErr     error
	ConvertOrdErr error
	ConvertInvErr error
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		Quotes:     make(map[string]*Quote),
		NextNumber: "Q-00001",
	}
}

func (m *MockRepository) Create(ctx context.Context, schemaName string, quote *Quote) error {
	if m.CreateErr != nil {
		return m.CreateErr
	}
	m.Quotes[quote.ID] = quote
	return nil
}

func (m *MockRepository) GetByID(ctx context.Context, schemaName, tenantID, quoteID string) (*Quote, error) {
	if m.GetErr != nil {
		return nil, m.GetErr
	}
	quote, ok := m.Quotes[quoteID]
	if !ok {
		return nil, ErrQuoteNotFound
	}
	return quote, nil
}

func (m *MockRepository) List(ctx context.Context, schemaName, tenantID string, filter *QuoteFilter) ([]Quote, error) {
	if m.ListErr != nil {
		return nil, m.ListErr
	}
	var quotes []Quote
	for _, q := range m.Quotes {
		if q.TenantID == tenantID {
			quotes = append(quotes, *q)
		}
	}
	return quotes, nil
}

func (m *MockRepository) Update(ctx context.Context, schemaName string, quote *Quote) error {
	if m.UpdateErr != nil {
		return m.UpdateErr
	}
	m.Quotes[quote.ID] = quote
	return nil
}

func (m *MockRepository) UpdateStatus(ctx context.Context, schemaName, tenantID, quoteID string, status QuoteStatus) error {
	if m.UpdateStatErr != nil {
		return m.UpdateStatErr
	}
	quote, ok := m.Quotes[quoteID]
	if !ok {
		return ErrQuoteNotFound
	}
	quote.Status = status
	return nil
}

func (m *MockRepository) Delete(ctx context.Context, schemaName, tenantID, quoteID string) error {
	if m.DeleteErr != nil {
		return m.DeleteErr
	}
	if _, ok := m.Quotes[quoteID]; !ok {
		return ErrQuoteNotFound
	}
	delete(m.Quotes, quoteID)
	return nil
}

func (m *MockRepository) GenerateNumber(ctx context.Context, schemaName, tenantID string) (string, error) {
	if m.GenerateErr != nil {
		return "", m.GenerateErr
	}
	return m.NextNumber, nil
}

func (m *MockRepository) SetConvertedToOrder(ctx context.Context, schemaName, tenantID, quoteID, orderID string) error {
	if m.ConvertOrdErr != nil {
		return m.ConvertOrdErr
	}
	quote, ok := m.Quotes[quoteID]
	if !ok {
		return ErrQuoteNotFound
	}
	quote.ConvertedToOrderID = &orderID
	quote.Status = QuoteStatusConverted
	return nil
}

func (m *MockRepository) SetConvertedToInvoice(ctx context.Context, schemaName, tenantID, quoteID, invoiceID string) error {
	if m.ConvertInvErr != nil {
		return m.ConvertInvErr
	}
	quote, ok := m.Quotes[quoteID]
	if !ok {
		return ErrQuoteNotFound
	}
	quote.ConvertedToInvoiceID = &invoiceID
	quote.Status = QuoteStatusConverted
	return nil
}

func TestNewServiceWithRepository(t *testing.T) {
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	assert.NotNil(t, svc)
	assert.Equal(t, repo, svc.repo)
}

func TestService_Create(t *testing.T) {
	t.Run("creates quote successfully", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)

		req := &CreateQuoteRequest{
			ContactID: "contact-1",
			QuoteDate: time.Now(),
			Currency:  "EUR",
			UserID:    "user-1",
			Lines: []CreateQuoteLineRequest{
				{
					Description: "Test product",
					Quantity:    decimal.NewFromInt(2),
					UnitPrice:   decimal.NewFromFloat(100.00),
					VATRate:     decimal.NewFromInt(20),
				},
			},
		}

		quote, err := svc.Create(context.Background(), "tenant-1", "test_schema", req)

		require.NoError(t, err)
		assert.NotEmpty(t, quote.ID)
		assert.Equal(t, "Q-00001", quote.QuoteNumber)
		assert.Equal(t, "tenant-1", quote.TenantID)
		assert.Equal(t, "contact-1", quote.ContactID)
		assert.Equal(t, "EUR", quote.Currency)
		assert.Equal(t, QuoteStatusDraft, quote.Status)
		assert.Len(t, quote.Lines, 1)
		assert.True(t, quote.Subtotal.Equal(decimal.NewFromFloat(200.00)))
	})

	t.Run("defaults currency to EUR", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)

		req := &CreateQuoteRequest{
			ContactID: "contact-1",
			QuoteDate: time.Now(),
			Currency:  "",
			UserID:    "user-1",
			Lines: []CreateQuoteLineRequest{
				{Description: "Test", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(10)},
			},
		}

		quote, err := svc.Create(context.Background(), "tenant-1", "test_schema", req)

		require.NoError(t, err)
		assert.Equal(t, "EUR", quote.Currency)
	})

	t.Run("defaults exchange rate to 1", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)

		req := &CreateQuoteRequest{
			ContactID: "contact-1",
			QuoteDate: time.Now(),
			UserID:    "user-1",
			Lines: []CreateQuoteLineRequest{
				{Description: "Test", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(10)},
			},
		}

		quote, err := svc.Create(context.Background(), "tenant-1", "test_schema", req)

		require.NoError(t, err)
		assert.True(t, quote.ExchangeRate.Equal(decimal.NewFromInt(1)))
	})

	t.Run("returns error on validation failure", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)

		req := &CreateQuoteRequest{
			ContactID: "",
			QuoteDate: time.Now(),
			UserID:    "user-1",
			Lines: []CreateQuoteLineRequest{
				{Description: "Test", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(10)},
			},
		}

		_, err := svc.Create(context.Background(), "tenant-1", "test_schema", req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation failed")
	})

	t.Run("returns error when generate number fails", func(t *testing.T) {
		repo := NewMockRepository()
		repo.GenerateErr = errors.New("generate error")
		svc := NewServiceWithRepository(repo)

		req := &CreateQuoteRequest{
			ContactID: "contact-1",
			QuoteDate: time.Now(),
			UserID:    "user-1",
			Lines: []CreateQuoteLineRequest{
				{Description: "Test", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(10)},
			},
		}

		_, err := svc.Create(context.Background(), "tenant-1", "test_schema", req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "generate quote number")
	})

	t.Run("returns error when repository create fails", func(t *testing.T) {
		repo := NewMockRepository()
		repo.CreateErr = errors.New("db error")
		svc := NewServiceWithRepository(repo)

		req := &CreateQuoteRequest{
			ContactID: "contact-1",
			QuoteDate: time.Now(),
			UserID:    "user-1",
			Lines: []CreateQuoteLineRequest{
				{Description: "Test", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(10)},
			},
		}

		_, err := svc.Create(context.Background(), "tenant-1", "test_schema", req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "create quote")
	})
}

func TestService_ImportCSV(t *testing.T) {
	t.Run("imports grouped quotes and preserves status", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)

		csvContent := `quote_number,contact_code,quote_date,valid_until,status,currency,exchange_rate,notes,line_description,quantity,unit,unit_price,discount_percent,vat_rate,product_id
QT-LEGACY-1,CUST-1,2026-03-15,2026-04-15,sent,EUR,1,March offer,Consulting,2,hour,100,10,22,prod-1
QT-LEGACY-1,CUST-1,2026-03-15,2026-04-15,sent,EUR,1,March offer,Support,1,hour,50,0,22,
`

		result, err := svc.ImportCSV(context.Background(), "tenant-1", "test_schema", []contacts.Contact{{
			ID:       "contact-1",
			TenantID: "tenant-1",
			Code:     "CUST-1",
			Name:     "Acme",
		}}, &ImportQuotesRequest{
			CSVContent: csvContent,
			FileName:   "quotes.csv",
			UserID:     "user-1",
		})

		require.NoError(t, err)
		assert.Equal(t, "quotes.csv", result.FileName)
		assert.Equal(t, 2, result.RowsProcessed)
		assert.Equal(t, 1, result.QuotesCreated)
		assert.Equal(t, 2, result.LinesImported)
		assert.Zero(t, result.RowsSkipped)
		assert.Nil(t, result.Errors)

		require.Len(t, repo.Quotes, 1)
		for _, quote := range repo.Quotes {
			assert.Equal(t, "QT-LEGACY-1", quote.QuoteNumber)
			assert.Equal(t, "contact-1", quote.ContactID)
			assert.Equal(t, QuoteStatusSent, quote.Status)
			assert.True(t, quote.Subtotal.Equal(decimal.RequireFromString("230.00")))
			assert.True(t, quote.VATAmount.Equal(decimal.RequireFromString("50.60")))
			require.Len(t, quote.Lines, 2)
			require.NotNil(t, quote.Lines[0].ProductID)
			assert.Equal(t, "prod-1", *quote.Lines[0].ProductID)
		}
	})

	t.Run("skips duplicate and invalid groups", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Quotes["existing"] = &Quote{
			ID:          "existing",
			TenantID:    "tenant-1",
			QuoteNumber: "QT-EXISTING",
		}
		svc := NewServiceWithRepository(repo)

		csvContent := `quote_number,contact_id,quote_date,line_description,quantity,unit_price,vat_rate
QT-EXISTING,contact-1,2026-03-15,Duplicate,1,10,22
QT-MISSING,missing-contact,2026-03-15,Unknown contact,1,10,22
QT-BAD,contact-1,2026-03-15,Bad quantity,0,10,22
`

		result, err := svc.ImportCSV(context.Background(), "tenant-1", "test_schema", []contacts.Contact{{
			ID:       "contact-1",
			TenantID: "tenant-1",
			Name:     "Acme",
		}}, &ImportQuotesRequest{CSVContent: csvContent})

		require.NoError(t, err)
		assert.Equal(t, 3, result.RowsProcessed)
		assert.Zero(t, result.QuotesCreated)
		assert.Equal(t, 3, result.RowsSkipped)
		require.Len(t, result.Errors, 3)
		messages := make([]string, 0, len(result.Errors))
		for _, rowErr := range result.Errors {
			messages = append(messages, rowErr.Message)
		}
		joinedMessages := strings.Join(messages, "\n")
		assert.Contains(t, joinedMessages, "already exists")
		assert.Contains(t, joinedMessages, "contact_id")
		assert.Contains(t, joinedMessages, "quantity must be greater than zero")
	})
}

func TestService_GetByID(t *testing.T) {
	t.Run("returns quote when found", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Quotes["quote-1"] = &Quote{ID: "quote-1", TenantID: "tenant-1", QuoteNumber: "Q-00001"}
		svc := NewServiceWithRepository(repo)

		quote, err := svc.GetByID(context.Background(), "tenant-1", "test_schema", "quote-1")

		require.NoError(t, err)
		assert.Equal(t, "quote-1", quote.ID)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)

		_, err := svc.GetByID(context.Background(), "tenant-1", "test_schema", "not-found")

		require.Error(t, err)
	})
}

func TestService_List(t *testing.T) {
	t.Run("returns quotes for tenant", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Quotes["quote-1"] = &Quote{ID: "quote-1", TenantID: "tenant-1"}
		repo.Quotes["quote-2"] = &Quote{ID: "quote-2", TenantID: "tenant-1"}
		svc := NewServiceWithRepository(repo)

		quotes, err := svc.List(context.Background(), "tenant-1", "test_schema", nil)

		require.NoError(t, err)
		assert.Len(t, quotes, 2)
	})

	t.Run("returns error on repository failure", func(t *testing.T) {
		repo := NewMockRepository()
		repo.ListErr = errors.New("db error")
		svc := NewServiceWithRepository(repo)

		_, err := svc.List(context.Background(), "tenant-1", "test_schema", nil)

		require.Error(t, err)
	})
}

func TestService_Update(t *testing.T) {
	t.Run("updates draft quote", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Quotes["quote-1"] = &Quote{
			ID:       "quote-1",
			TenantID: "tenant-1",
			Status:   QuoteStatusDraft,
		}
		svc := NewServiceWithRepository(repo)

		req := &UpdateQuoteRequest{
			ContactID: "contact-2",
			QuoteDate: time.Now(),
			Lines: []CreateQuoteLineRequest{
				{Description: "Updated", Quantity: decimal.NewFromInt(3), UnitPrice: decimal.NewFromFloat(50)},
			},
		}

		quote, err := svc.Update(context.Background(), "tenant-1", "test_schema", "quote-1", req)

		require.NoError(t, err)
		assert.Equal(t, "contact-2", quote.ContactID)
		assert.Len(t, quote.Lines, 1)
	})

	t.Run("returns error when updating sent quote", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Quotes["quote-1"] = &Quote{
			ID:       "quote-1",
			TenantID: "tenant-1",
			Status:   QuoteStatusSent,
		}
		svc := NewServiceWithRepository(repo)

		req := &UpdateQuoteRequest{
			ContactID: "contact-2",
			QuoteDate: time.Now(),
			Lines: []CreateQuoteLineRequest{
				{Description: "Updated", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(10)},
			},
		}

		_, err := svc.Update(context.Background(), "tenant-1", "test_schema", "quote-1", req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "only draft quotes can be updated")
	})
}

func TestService_Send(t *testing.T) {
	t.Run("sends draft quote", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Quotes["quote-1"] = &Quote{ID: "quote-1", Status: QuoteStatusDraft}
		svc := NewServiceWithRepository(repo)

		err := svc.Send(context.Background(), "tenant-1", "test_schema", "quote-1")

		require.NoError(t, err)
		assert.Equal(t, QuoteStatusSent, repo.Quotes["quote-1"].Status)
	})

	t.Run("returns error when not draft", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Quotes["quote-1"] = &Quote{ID: "quote-1", Status: QuoteStatusSent}
		svc := NewServiceWithRepository(repo)

		err := svc.Send(context.Background(), "tenant-1", "test_schema", "quote-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not in draft status")
	})
}

func TestService_Accept(t *testing.T) {
	t.Run("accepts sent quote", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Quotes["quote-1"] = &Quote{ID: "quote-1", Status: QuoteStatusSent}
		svc := NewServiceWithRepository(repo)

		err := svc.Accept(context.Background(), "tenant-1", "test_schema", "quote-1")

		require.NoError(t, err)
		assert.Equal(t, QuoteStatusAccepted, repo.Quotes["quote-1"].Status)
	})

	t.Run("accepts draft quote", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Quotes["quote-1"] = &Quote{ID: "quote-1", Status: QuoteStatusDraft}
		svc := NewServiceWithRepository(repo)

		err := svc.Accept(context.Background(), "tenant-1", "test_schema", "quote-1")

		require.NoError(t, err)
		assert.Equal(t, QuoteStatusAccepted, repo.Quotes["quote-1"].Status)
	})

	t.Run("returns error when already converted", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Quotes["quote-1"] = &Quote{ID: "quote-1", Status: QuoteStatusConverted}
		svc := NewServiceWithRepository(repo)

		err := svc.Accept(context.Background(), "tenant-1", "test_schema", "quote-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be accepted")
	})
}

func TestService_Reject(t *testing.T) {
	t.Run("rejects sent quote", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Quotes["quote-1"] = &Quote{ID: "quote-1", Status: QuoteStatusSent}
		svc := NewServiceWithRepository(repo)

		err := svc.Reject(context.Background(), "tenant-1", "test_schema", "quote-1")

		require.NoError(t, err)
		assert.Equal(t, QuoteStatusRejected, repo.Quotes["quote-1"].Status)
	})

	t.Run("rejects draft quote", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Quotes["quote-1"] = &Quote{ID: "quote-1", Status: QuoteStatusDraft}
		svc := NewServiceWithRepository(repo)

		err := svc.Reject(context.Background(), "tenant-1", "test_schema", "quote-1")

		require.NoError(t, err)
		assert.Equal(t, QuoteStatusRejected, repo.Quotes["quote-1"].Status)
	})

	t.Run("returns error when accepted", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Quotes["quote-1"] = &Quote{ID: "quote-1", Status: QuoteStatusAccepted}
		svc := NewServiceWithRepository(repo)

		err := svc.Reject(context.Background(), "tenant-1", "test_schema", "quote-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be rejected")
	})
}

func TestService_Delete(t *testing.T) {
	t.Run("deletes quote", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Quotes["quote-1"] = &Quote{ID: "quote-1"}
		svc := NewServiceWithRepository(repo)

		err := svc.Delete(context.Background(), "tenant-1", "test_schema", "quote-1")

		require.NoError(t, err)
		assert.Empty(t, repo.Quotes)
	})

	t.Run("returns error on failure", func(t *testing.T) {
		repo := NewMockRepository()
		repo.DeleteErr = errors.New("db error")
		svc := NewServiceWithRepository(repo)

		err := svc.Delete(context.Background(), "tenant-1", "test_schema", "quote-1")

		require.Error(t, err)
	})
}

func TestService_ConvertToOrder(t *testing.T) {
	t.Run("marks quote as converted to order", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Quotes["quote-1"] = &Quote{ID: "quote-1"}
		svc := NewServiceWithRepository(repo)

		err := svc.ConvertToOrder(context.Background(), "tenant-1", "test_schema", "quote-1", "order-1")

		require.NoError(t, err)
		assert.Equal(t, "order-1", *repo.Quotes["quote-1"].ConvertedToOrderID)
		assert.Equal(t, QuoteStatusConverted, repo.Quotes["quote-1"].Status)
	})

	t.Run("returns error on failure", func(t *testing.T) {
		repo := NewMockRepository()
		repo.ConvertOrdErr = errors.New("db error")
		svc := NewServiceWithRepository(repo)

		err := svc.ConvertToOrder(context.Background(), "tenant-1", "test_schema", "quote-1", "order-1")

		require.Error(t, err)
	})
}

func TestService_ConvertToInvoice(t *testing.T) {
	t.Run("marks quote as converted to invoice", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Quotes["quote-1"] = &Quote{ID: "quote-1"}
		svc := NewServiceWithRepository(repo)

		err := svc.ConvertToInvoice(context.Background(), "tenant-1", "test_schema", "quote-1", "invoice-1")

		require.NoError(t, err)
		assert.Equal(t, "invoice-1", *repo.Quotes["quote-1"].ConvertedToInvoiceID)
		assert.Equal(t, QuoteStatusConverted, repo.Quotes["quote-1"].Status)
	})

	t.Run("returns error on failure", func(t *testing.T) {
		repo := NewMockRepository()
		repo.ConvertInvErr = errors.New("db error")
		svc := NewServiceWithRepository(repo)

		err := svc.ConvertToInvoice(context.Background(), "tenant-1", "test_schema", "quote-1", "invoice-1")

		require.Error(t, err)
	})
}
