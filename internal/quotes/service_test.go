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
	"github.com/HMB-research/open-accounting/internal/importrefs"
	"github.com/HMB-research/open-accounting/internal/inventory"
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

func TestNewService(t *testing.T) {
	svc := NewService(nil)

	require.NotNil(t, svc)
	assert.NotNil(t, svc.repo)
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

	t.Run("defaults quote date to now", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)

		req := &CreateQuoteRequest{
			ContactID: "contact-1",
			UserID:    "user-1",
			Lines: []CreateQuoteLineRequest{
				{Description: "Test", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(10)},
			},
		}

		quote, err := svc.Create(context.Background(), "tenant-1", "test_schema", req)

		require.NoError(t, err)
		assert.False(t, quote.QuoteDate.IsZero())
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
		legacyQuoteID := "11111111-1111-1111-1111-111111111111"

		csvContent := `id,quote_number,contact_code,quote_date,valid_until,status,currency,exchange_rate,notes,line_description,quantity,unit,unit_price,discount_percent,vat_rate,product_code
11111111-1111-1111-1111-111111111111,QT-LEGACY-1,CUST-1,2026-03-15,2026-04-15,sent,EUR,1,March offer,Consulting,2,hour,100,10,22,SERV-001
11111111-1111-1111-1111-111111111111,QT-LEGACY-1,CUST-1,2026-03-15,2026-04-15,sent,EUR,1,March offer,Support,1,hour,50,0,22,
`

		result, err := svc.ImportCSV(context.Background(), "tenant-1", "test_schema", []contacts.Contact{{
			ID:       "contact-1",
			TenantID: "tenant-1",
			Code:     "CUST-1",
			Name:     "Acme",
		}}, []inventory.Product{{
			ID:       "prod-1",
			TenantID: "tenant-1",
			Code:     "SERV-001",
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
		quote := repo.Quotes[legacyQuoteID]
		require.NotNil(t, quote)
		assert.Equal(t, legacyQuoteID, quote.ID)
		assert.Equal(t, "QT-LEGACY-1", quote.QuoteNumber)
		assert.Equal(t, "contact-1", quote.ContactID)
		assert.Equal(t, QuoteStatusSent, quote.Status)
		assert.True(t, quote.Subtotal.Equal(decimal.RequireFromString("230.00")))
		assert.True(t, quote.VATAmount.Equal(decimal.RequireFromString("50.60")))
		require.Len(t, quote.Lines, 2)
		assert.Equal(t, legacyQuoteID, quote.Lines[0].QuoteID)
		require.NotNil(t, quote.Lines[0].ProductID)
		assert.Equal(t, "prod-1", *quote.Lines[0].ProductID)
	})

	t.Run("resolves contact by VAT number", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)

		csvContent := `quote_number,contact_vat_number,quote_date,line_description,quantity,unit_price,vat_rate
QT-VAT-1,EE123456789,2026-03-15,Consulting,1,100,22
`

		result, err := svc.ImportCSV(context.Background(), "tenant-1", "test_schema", []contacts.Contact{{
			ID:        "contact-vat",
			TenantID:  "tenant-1",
			RegCode:   "12345678",
			VATNumber: "EE123456789",
			Name:      "VAT Customer",
		}}, nil, &ImportQuotesRequest{CSVContent: csvContent})

		require.NoError(t, err)
		assert.Equal(t, 1, result.RowsProcessed)
		assert.Equal(t, 1, result.QuotesCreated)
		assert.Zero(t, result.RowsSkipped)
		assert.Empty(t, result.Errors)
		require.Len(t, repo.Quotes, 1)
		for _, quote := range repo.Quotes {
			assert.Equal(t, "contact-vat", quote.ContactID)
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
QT-EXISTING,bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb,2026-03-15,Duplicate,1,10,22
QT-MISSING,cccccccc-cccc-4ccc-8ccc-cccccccccccc,2026-03-15,Unknown contact,1,10,22
QT-BAD,bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb,2026-03-15,Bad quantity,0,10,22
`

		result, err := svc.ImportCSV(context.Background(), "tenant-1", "test_schema", []contacts.Contact{{
			ID:       "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
			TenantID: "tenant-1",
			Name:     "Acme",
		}}, nil, &ImportQuotesRequest{CSVContent: csvContent})

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

	t.Run("skips invalid imported quote id", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)

		csvContent := `quote_id,quote_number,contact_id,quote_date,line_description,quantity,unit_price,vat_rate
legacy-id,QT-BAD-ID,bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb,2026-03-15,Consulting,1,10,22
`

		result, err := svc.ImportCSV(context.Background(), "tenant-1", "test_schema", []contacts.Contact{{
			ID:       "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
			TenantID: "tenant-1",
			Name:     "Acme",
		}}, nil, &ImportQuotesRequest{CSVContent: csvContent})

		require.NoError(t, err)
		assert.Zero(t, result.QuotesCreated)
		assert.Equal(t, 1, result.RowsSkipped)
		require.Len(t, result.Errors, 1)
		assert.Equal(t, 2, result.Errors[0].Row)
		assert.Contains(t, result.Errors[0].Message, "invalid id")
	})
}

func TestService_ImportCSVValidationFailures(t *testing.T) {
	t.Run("requires csv content", func(t *testing.T) {
		svc := NewServiceWithRepository(NewMockRepository())

		_, err := svc.ImportCSV(context.Background(), "tenant-1", "test_schema", nil, nil, nil)
		require.EqualError(t, err, "csv_content is required")

		_, err = svc.ImportCSV(context.Background(), "tenant-1", "test_schema", nil, nil, &ImportQuotesRequest{CSVContent: "  "})
		require.EqualError(t, err, "csv_content is required")
	})

	t.Run("returns parser errors before repository access", func(t *testing.T) {
		svc := NewServiceWithRepository(NewMockRepository())

		_, err := svc.ImportCSV(context.Background(), "tenant-1", "test_schema", nil, nil, &ImportQuotesRequest{
			CSVContent: "quote_number,contact_id\nQ-1,bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb\n",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing required")
	})

	t.Run("returns repository list failure", func(t *testing.T) {
		repo := NewMockRepository()
		repo.ListErr = errors.New("repository unavailable")
		svc := NewServiceWithRepository(repo)

		_, err := svc.ImportCSV(context.Background(), "tenant-1", "test_schema", nil, nil, &ImportQuotesRequest{
			CSVContent: `quote_number,contact_id,quote_date,line_description,quantity,unit_price,vat_rate
QT-1,bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb,2026-03-15,Consulting,1,10,22
`,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "list existing quotes")
	})
}

func TestService_ImportCSVParsesAliasesAndDerivedFields(t *testing.T) {
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	csvContent := `offer_number;email;date;valid_to;currency;exchange_rate;notes;description;qty;unit;price;discount;vat
QT-LEGACY-2;billing@example.com;2000-01-15;2000-02-15;eur;1,5;Old offer;Analysis;2;hour;100,50;0;22
`

	result, err := svc.ImportCSV(context.Background(), "tenant-1", "test_schema", []contacts.Contact{{
		ID:       "contact-email",
		TenantID: "tenant-1",
		Email:    "billing@example.com",
		Name:     "Acme Billing",
	}}, nil, &ImportQuotesRequest{
		CSVContent: csvContent,
		FileName:   "legacy-quotes.csv",
		UserID:     "user-1",
	})

	require.NoError(t, err)
	assert.Equal(t, "legacy-quotes.csv", result.FileName)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Equal(t, 1, result.QuotesCreated)
	assert.Equal(t, 1, result.LinesImported)
	assert.Zero(t, result.RowsSkipped)

	require.Len(t, repo.Quotes, 1)
	for _, quote := range repo.Quotes {
		assert.Equal(t, "QT-LEGACY-2", quote.QuoteNumber)
		assert.Equal(t, "contact-email", quote.ContactID)
		assert.Equal(t, "EUR", quote.Currency)
		assert.Equal(t, QuoteStatusExpired, quote.Status)
		assert.True(t, quote.ExchangeRate.Equal(decimal.RequireFromString("1.5")))
		assert.True(t, quote.Subtotal.Equal(decimal.RequireFromString("201.00")))
		assert.Equal(t, "Old offer", quote.Notes)
	}
}

func TestService_ImportCSVSkipsGroupConflictsAndCreateFailures(t *testing.T) {
	t.Run("skips grouped quote with inconsistent header", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)

		csvContent := `quote_number,contact_id,quote_date,currency,line_description,quantity,unit_price,vat_rate
QT-CONFLICT,bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb,2026-03-15,EUR,Consulting,1,10,22
QT-CONFLICT,bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb,2026-03-15,USD,Support,1,10,22
`

		result, err := svc.ImportCSV(context.Background(), "tenant-1", "test_schema", []contacts.Contact{{
			ID:       "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
			TenantID: "tenant-1",
		}}, nil, &ImportQuotesRequest{CSVContent: csvContent})

		require.NoError(t, err)
		assert.Zero(t, result.QuotesCreated)
		assert.Equal(t, 2, result.RowsSkipped)
		require.Len(t, result.Errors, 1)
		assert.Equal(t, 3, result.Errors[0].Row)
		assert.Contains(t, result.Errors[0].Message, "currency must be consistent")
	})

	t.Run("skips quote when repository create fails", func(t *testing.T) {
		repo := NewMockRepository()
		repo.CreateErr = errors.New("insert quote failed")
		svc := NewServiceWithRepository(repo)

		csvContent := `quote_number,contact_name,quote_date,line_description,quantity,unit_price,vat_rate
QT-FAIL,Acme,2026-03-15,Consulting,1,10,22
`

		result, err := svc.ImportCSV(context.Background(), "tenant-1", "test_schema", []contacts.Contact{{
			ID:       "contact-1",
			TenantID: "tenant-1",
			Name:     "Acme",
		}}, nil, &ImportQuotesRequest{CSVContent: csvContent})

		require.NoError(t, err)
		assert.Zero(t, result.QuotesCreated)
		assert.Equal(t, 1, result.RowsSkipped)
		require.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Message, "insert quote failed")
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

	t.Run("returns error when get fails", func(t *testing.T) {
		repo := NewMockRepository()
		repo.GetErr = errors.New("lookup failed")
		svc := NewServiceWithRepository(repo)

		_, err := svc.Update(context.Background(), "tenant-1", "test_schema", "quote-1", &UpdateQuoteRequest{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "get quote")
	})

	t.Run("returns error on validation failure", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Quotes["quote-1"] = &Quote{
			ID:       "quote-1",
			TenantID: "tenant-1",
			Status:   QuoteStatusDraft,
		}
		svc := NewServiceWithRepository(repo)

		_, err := svc.Update(context.Background(), "tenant-1", "test_schema", "quote-1", &UpdateQuoteRequest{
			ContactID:  "contact-1",
			QuoteDate:  time.Now(),
			ValidUntil: nil,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation failed")
	})

	t.Run("returns error when repository update fails", func(t *testing.T) {
		repo := NewMockRepository()
		repo.UpdateErr = errors.New("update failed")
		repo.Quotes["quote-1"] = &Quote{
			ID:       "quote-1",
			TenantID: "tenant-1",
			Status:   QuoteStatusDraft,
		}
		svc := NewServiceWithRepository(repo)

		_, err := svc.Update(context.Background(), "tenant-1", "test_schema", "quote-1", &UpdateQuoteRequest{
			ContactID: "contact-1",
			QuoteDate: time.Now(),
			Lines: []CreateQuoteLineRequest{
				{Description: "Updated", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(10)},
			},
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "update quote")
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

	t.Run("returns error when get fails", func(t *testing.T) {
		repo := NewMockRepository()
		repo.GetErr = errors.New("lookup failed")
		svc := NewServiceWithRepository(repo)

		err := svc.Send(context.Background(), "tenant-1", "test_schema", "quote-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "get quote")
	})

	t.Run("returns error when repository status update fails", func(t *testing.T) {
		repo := NewMockRepository()
		repo.UpdateStatErr = errors.New("status failed")
		repo.Quotes["quote-1"] = &Quote{ID: "quote-1", Status: QuoteStatusDraft}
		svc := NewServiceWithRepository(repo)

		err := svc.Send(context.Background(), "tenant-1", "test_schema", "quote-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "send quote")
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

	t.Run("returns error when get fails", func(t *testing.T) {
		repo := NewMockRepository()
		repo.GetErr = errors.New("lookup failed")
		svc := NewServiceWithRepository(repo)

		err := svc.Accept(context.Background(), "tenant-1", "test_schema", "quote-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "get quote")
	})

	t.Run("returns error when repository status update fails", func(t *testing.T) {
		repo := NewMockRepository()
		repo.UpdateStatErr = errors.New("status failed")
		repo.Quotes["quote-1"] = &Quote{ID: "quote-1", Status: QuoteStatusSent}
		svc := NewServiceWithRepository(repo)

		err := svc.Accept(context.Background(), "tenant-1", "test_schema", "quote-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "accept quote")
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

	t.Run("returns error when get fails", func(t *testing.T) {
		repo := NewMockRepository()
		repo.GetErr = errors.New("lookup failed")
		svc := NewServiceWithRepository(repo)

		err := svc.Reject(context.Background(), "tenant-1", "test_schema", "quote-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "get quote")
	})

	t.Run("returns error when repository status update fails", func(t *testing.T) {
		repo := NewMockRepository()
		repo.UpdateStatErr = errors.New("status failed")
		repo.Quotes["quote-1"] = &Quote{ID: "quote-1", Status: QuoteStatusSent}
		svc := NewServiceWithRepository(repo)

		err := svc.Reject(context.Background(), "tenant-1", "test_schema", "quote-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "reject quote")
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

func TestParseQuoteImportRows(t *testing.T) {
	t.Run("handles BOM semicolon aliases and blank rows", func(t *testing.T) {
		rows, err := parseQuoteImportRows("\ufeffoffer_number;contact_code;date;description;qty;price;vat\n\nQT-1;CUST-1;2026-03-15;Consulting;1;10;22\n")

		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, 2, rows[0].rowNumber)
		assert.Equal(t, "QT-1", rows[0].values["quote_number"])
		assert.Equal(t, "CUST-1", rows[0].values["contact_code"])
		assert.Equal(t, "Consulting", rows[0].values["line_description"])
		assert.Equal(t, "1", rows[0].values["quantity"])
	})

	t.Run("requires contact identifier column", func(t *testing.T) {
		_, err := parseQuoteImportRows("quote_number,quote_date,line_description,quantity,unit_price,vat_rate\nQT-1,2026-03-15,Consulting,1,10,22\n")

		require.EqualError(t, err, "missing contact identifier column")
	})
}

func TestParseQuoteImportDataRow(t *testing.T) {
	t.Run("parses valid row", func(t *testing.T) {
		productID := "22222222-2222-4222-8222-222222222222"
		row := quoteImportRowForTest(map[string]string{
			"status":     "approved",
			"currency":   "usd",
			"product_id": productID,
		})

		parsed, err := parseQuoteImportDataRow(row, importrefs.ProductLookup{})

		require.NoError(t, err)
		assert.Equal(t, "QT-1", parsed.header.quoteNumber)
		assert.Equal(t, "USD", parsed.header.currency)
		assert.Equal(t, QuoteStatusAccepted, parsed.header.explicitStatus)
		require.NotNil(t, parsed.line.productID)
		assert.Equal(t, productID, *parsed.line.productID)
	})

	tests := []struct {
		name      string
		overrides map[string]string
		want      string
	}{
		{name: "missing quote number", overrides: map[string]string{"quote_number": ""}, want: "quote_number is required"},
		{name: "missing contact identifier", overrides: map[string]string{"contact_id": ""}, want: "a contact identifier is required"},
		{name: "invalid quote date", overrides: map[string]string{"quote_date": "03/15/2026"}, want: "quote_date must use YYYY-MM-DD"},
		{name: "invalid valid until", overrides: map[string]string{"valid_until": "03/15/2026"}, want: "valid_until must use YYYY-MM-DD"},
		{name: "invalid exchange rate", overrides: map[string]string{"exchange_rate": "abc"}, want: "invalid exchange_rate"},
		{name: "zero exchange rate", overrides: map[string]string{"exchange_rate": "0"}, want: "exchange_rate must be greater than zero"},
		{name: "invalid status", overrides: map[string]string{"status": "unknown"}, want: `invalid status "unknown"`},
		{name: "missing description", overrides: map[string]string{"line_description": ""}, want: "line_description is required"},
		{name: "invalid quantity", overrides: map[string]string{"quantity": "abc"}, want: "invalid quantity"},
		{name: "zero quantity", overrides: map[string]string{"quantity": "0"}, want: "quantity must be greater than zero"},
		{name: "invalid unit price", overrides: map[string]string{"unit_price": "abc"}, want: "invalid unit_price"},
		{name: "negative unit price", overrides: map[string]string{"unit_price": "-1"}, want: "unit_price cannot be negative"},
		{name: "invalid discount", overrides: map[string]string{"discount_percent": "abc"}, want: "invalid discount_percent"},
		{name: "discount too high", overrides: map[string]string{"discount_percent": "101"}, want: "discount_percent must be between 0 and 100"},
		{name: "invalid vat rate", overrides: map[string]string{"vat_rate": "abc"}, want: "invalid vat_rate"},
		{name: "negative vat rate", overrides: map[string]string{"vat_rate": "-1"}, want: "vat_rate cannot be negative"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseQuoteImportDataRow(quoteImportRowForTest(tt.overrides), importrefs.ProductLookup{})

			require.EqualError(t, err, tt.want)
		})
	}
}

func TestMergeQuoteImportGroup(t *testing.T) {
	quoteDate := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	validUntil := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)

	t.Run("merges optional header values", func(t *testing.T) {
		group := &quoteImportGroup{
			header: quoteImportHeader{
				quoteNumber:  "QT-1",
				contactRef:   quoteImportContactRef{id: "contact-1"},
				quoteDate:    quoteDate,
				currency:     "EUR",
				exchangeRate: decimal.NewFromInt(1),
			},
		}

		conflict := mergeQuoteImportGroup(group, quoteImportHeader{
			quoteNumber:    "QT-1",
			contactRef:     quoteImportContactRef{code: "CUST-1"},
			quoteDate:      quoteDate,
			validUntil:     &validUntil,
			currency:       "EUR",
			exchangeRate:   decimal.NewFromInt(1),
			notes:          "Follow up",
			explicitStatus: QuoteStatusSent,
		}, 3)

		assert.Empty(t, conflict)
		require.NotNil(t, group.header.validUntil)
		assert.Equal(t, "CUST-1", group.header.contactRef.code)
		assert.Equal(t, "Follow up", group.header.notes)
		assert.Equal(t, QuoteStatusSent, group.header.explicitStatus)
	})

	tests := []struct {
		name      string
		next      quoteImportHeader
		want      string
		rowNumber int
	}{
		{
			name: "quote date conflict",
			next: quoteImportHeader{
				quoteDate:    quoteDate.AddDate(0, 0, 1),
				currency:     "EUR",
				exchangeRate: decimal.NewFromInt(1),
			},
			want: "quote_date must be consistent",
		},
		{
			name: "valid until conflict",
			next: quoteImportHeader{
				quoteDate:    quoteDate,
				validUntil:   ptrTime(validUntil.AddDate(0, 0, 1)),
				currency:     "EUR",
				exchangeRate: decimal.NewFromInt(1),
			},
			want: "valid_until must be consistent",
		},
		{
			name: "currency conflict",
			next: quoteImportHeader{
				quoteDate:    quoteDate,
				currency:     "USD",
				exchangeRate: decimal.NewFromInt(1),
			},
			want: "currency must be consistent",
		},
		{
			name: "exchange rate conflict",
			next: quoteImportHeader{
				quoteDate:    quoteDate,
				currency:     "EUR",
				exchangeRate: decimal.RequireFromString("1.2"),
			},
			want: "exchange_rate must be consistent",
		},
		{
			name: "contact conflict",
			next: quoteImportHeader{
				contactRef:   quoteImportContactRef{id: "contact-2"},
				quoteDate:    quoteDate,
				currency:     "EUR",
				exchangeRate: decimal.NewFromInt(1),
			},
			want: "contact_id must be consistent",
		},
		{
			name: "notes conflict",
			next: quoteImportHeader{
				quoteDate:    quoteDate,
				currency:     "EUR",
				exchangeRate: decimal.NewFromInt(1),
				notes:        "different",
			},
			want: "notes must be consistent",
		},
		{
			name: "status conflict",
			next: quoteImportHeader{
				quoteDate:      quoteDate,
				currency:       "EUR",
				exchangeRate:   decimal.NewFromInt(1),
				explicitStatus: QuoteStatusAccepted,
			},
			want:      "status must be consistent for each quote_number (row 7)",
			rowNumber: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group := &quoteImportGroup{
				header: quoteImportHeader{
					quoteNumber:    "QT-1",
					contactRef:     quoteImportContactRef{id: "contact-1"},
					quoteDate:      quoteDate,
					validUntil:     &validUntil,
					currency:       "EUR",
					exchangeRate:   decimal.NewFromInt(1),
					notes:          "same",
					explicitStatus: QuoteStatusSent,
				},
			}
			rowNumber := tt.rowNumber
			if rowNumber == 0 {
				rowNumber = 3
			}

			conflict := mergeQuoteImportGroup(group, tt.next, rowNumber)

			assert.Contains(t, conflict, tt.want)
		})
	}
}

func TestQuoteImportContactLookupFind(t *testing.T) {
	lookup := buildQuoteImportContactLookup([]contacts.Contact{{
		ID:        "contact-id",
		Code:      "CUST-1",
		RegCode:   "12345678",
		VATNumber: "EE12345678",
		Email:     "billing@example.com",
		Name:      "Acme OU",
	}})

	tests := []struct {
		name string
		ref  quoteImportContactRef
	}{
		{name: "id", ref: quoteImportContactRef{id: "contact-id"}},
		{name: "code", ref: quoteImportContactRef{code: "cust-1"}},
		{name: "registration code", ref: quoteImportContactRef{regCode: "12345678"}},
		{name: "VAT number", ref: quoteImportContactRef{vatNumber: "ee12345678"}},
		{name: "email", ref: quoteImportContactRef{email: "BILLING@example.com"}},
		{name: "name", ref: quoteImportContactRef{name: "acme ou"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contact, err := lookup.find(tt.ref)

			require.NoError(t, err)
			assert.Equal(t, "contact-id", contact.ID)
		})
	}

	_, err := lookup.find(quoteImportContactRef{email: "missing@example.com"})
	require.EqualError(t, err, `contact_email "missing@example.com" was not found`)

	_, err = lookup.find(quoteImportContactRef{})
	require.EqualError(t, err, "a contact identifier is required")
}

func TestQuoteImportHelpers(t *testing.T) {
	status, err := parseQuoteImportStatus("CONVERTED")
	require.NoError(t, err)
	assert.Equal(t, QuoteStatusConverted, status)

	status, err = parseQuoteImportStatus(" ")
	require.NoError(t, err)
	assert.Empty(t, status)

	assert.Equal(t, '\t', detectQuoteImportDelimiter("a\tb\n1\t2"))
	assert.Equal(t, ',', detectQuoteImportDelimiter("a,b\n1,2"))

	future := time.Now().AddDate(0, 0, 1)
	assert.Equal(t, QuoteStatusDraft, deriveQuoteImportStatus("", &future, normalizeQuoteImportDate(time.Now())))
	assert.Equal(t, QuoteStatusRejected, deriveQuoteImportStatus(QuoteStatusRejected, nil, time.Now()))
}

func quoteImportRowForTest(overrides map[string]string) quoteImportRow {
	values := map[string]string{
		"quote_number":     "QT-1",
		"contact_id":       "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
		"quote_date":       "2026-03-15",
		"valid_until":      "",
		"currency":         "EUR",
		"exchange_rate":    "1",
		"notes":            "",
		"status":           "",
		"line_description": "Consulting",
		"quantity":         "1",
		"unit":             "hour",
		"unit_price":       "10",
		"discount_percent": "0",
		"vat_rate":         "22",
		"product_id":       "",
	}
	for key, value := range overrides {
		values[key] = value
	}
	return quoteImportRow{rowNumber: 2, values: values}
}

func ptrTime(value time.Time) *time.Time {
	return &value
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
