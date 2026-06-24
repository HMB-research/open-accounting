package invoicing

import (
	"context"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInterestServiceConstructors(t *testing.T) {
	repository := NewInterestRepository(nil)
	require.NotNil(t, repository)
	assert.Nil(t, repository.db)

	gormRepository := NewInterestGORMRepository(nil)
	require.NotNil(t, gormRepository)
	assert.Nil(t, gormRepository.db)

	service := NewInterestService(nil)
	require.NotNil(t, service)
	require.NotNil(t, service.repo)

	fakeRepo := &fakeInterestRepository{}
	service = NewInterestServiceWithRepository(fakeRepo)
	require.NotNil(t, service)
	assert.Same(t, fakeRepo, service.repo)
}

func TestInterestServiceCalculatesAndDelegates(t *testing.T) {
	ctx := context.Background()
	dueDate := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	asOfDate := time.Date(2026, 6, 11, 0, 0, 0, 0, time.UTC)
	repository := &fakeInterestRepository{
		invoices: map[string]*interestInvoice{
			"inv-1": {
				ID:            "inv-1",
				InvoiceNumber: "INV-1",
				DueDate:       dueDate,
				Total:         decimal.NewFromInt(1000),
				AmountPaid:    decimal.NewFromInt(200),
				Currency:      "EUR",
			},
		},
		latest: &InvoiceInterest{ID: "interest-latest", InvoiceID: "inv-1"},
		history: []InvoiceInterest{
			{ID: "interest-1", InvoiceID: "inv-1"},
			{ID: "interest-2", InvoiceID: "inv-1"},
		},
		overdue: []interestInvoice{
			{
				ID:            "inv-1",
				InvoiceNumber: "INV-1",
				DueDate:       time.Now().AddDate(0, 0, -5),
				Total:         decimal.NewFromInt(1000),
				AmountPaid:    decimal.NewFromInt(200),
				Currency:      "EUR",
			},
			{
				ID:            "inv-paid",
				InvoiceNumber: "INV-PAID",
				DueDate:       time.Now().AddDate(0, 0, -5),
				Total:         decimal.NewFromInt(100),
				AmountPaid:    decimal.NewFromInt(100),
				Currency:      "EUR",
			},
		},
	}
	service := NewInterestServiceWithRepository(repository)

	result, err := service.CalculateInterest(ctx, "tenant_schema", "tenant-1", "inv-1", 0.001, asOfDate)
	require.NoError(t, err)
	assert.Equal(t, "inv-1", result.InvoiceID)
	assert.Equal(t, "INV-1", result.InvoiceNumber)
	assert.Equal(t, 10, result.DaysOverdue)
	assert.True(t, result.OutstandingAmount.Equal(decimal.NewFromInt(800)))
	assert.True(t, result.DailyInterest.Equal(decimal.RequireFromString("0.80")))
	assert.True(t, result.TotalInterest.Equal(decimal.RequireFromString("8.00")))
	assert.True(t, result.TotalWithInterest.Equal(decimal.RequireFromString("808.00")))
	assert.Equal(t, "EUR", result.Currency)

	saved, err := service.SaveInterestCalculation(ctx, "tenant_schema", result)
	require.NoError(t, err)
	assert.NotEmpty(t, saved.ID)
	assert.Equal(t, result.InvoiceID, saved.InvoiceID)
	assert.True(t, result.OutstandingAmount.Equal(saved.PrincipalAmount))
	require.Len(t, repository.created, 1)
	assert.Same(t, saved, repository.created[0])

	latest, err := service.GetLatestInterest(ctx, "tenant_schema", "inv-1")
	require.NoError(t, err)
	assert.Equal(t, "interest-latest", latest.ID)

	history, err := service.ListInterestHistory(ctx, "tenant_schema", "inv-1")
	require.NoError(t, err)
	assert.Len(t, history, 2)

	overdueResults, err := service.CalculateInterestForOverdueInvoices(ctx, "tenant_schema", "tenant-1", 0.001)
	require.NoError(t, err)
	require.Len(t, overdueResults, 2)
	assert.Equal(t, "inv-1", overdueResults[0].InvoiceID)
	assert.Equal(t, "inv-paid", overdueResults[1].InvoiceID)
	assert.True(t, overdueResults[1].OutstandingAmount.IsZero())
	assert.True(t, overdueResults[1].TotalInterest.IsZero())
	assert.True(t, repository.listOverdueCalled)
}

func TestInterestServicePropagatesRepositoryErrors(t *testing.T) {
	ctx := context.Background()
	service := NewInterestServiceWithRepository(&fakeInterestRepository{getErr: assert.AnError})
	_, err := service.CalculateInterest(ctx, "tenant_schema", "tenant-1", "inv-1", 0.001, time.Now())
	require.ErrorIs(t, err, assert.AnError)

	service = NewInterestServiceWithRepository(&fakeInterestRepository{createErr: assert.AnError})
	_, err = service.SaveInterestCalculation(ctx, "tenant_schema", &InterestCalculationResult{
		InvoiceID:         "inv-1",
		CalculatedAt:      time.Now(),
		OutstandingAmount: decimal.NewFromInt(100),
		InterestRate:      decimal.RequireFromString("0.001"),
		TotalInterest:     decimal.RequireFromString("1.00"),
		TotalWithInterest: decimal.RequireFromString("101.00"),
	})
	require.ErrorIs(t, err, assert.AnError)

	service = NewInterestServiceWithRepository(&fakeInterestRepository{latestErr: assert.AnError})
	_, err = service.GetLatestInterest(ctx, "tenant_schema", "inv-1")
	require.ErrorIs(t, err, assert.AnError)

	service = NewInterestServiceWithRepository(&fakeInterestRepository{historyErr: assert.AnError})
	_, err = service.ListInterestHistory(ctx, "tenant_schema", "inv-1")
	require.ErrorIs(t, err, assert.AnError)

	service = NewInterestServiceWithRepository(&fakeInterestRepository{listOverdueErr: assert.AnError})
	_, err = service.CalculateInterestForOverdueInvoices(ctx, "tenant_schema", "tenant-1", 0.001)
	require.ErrorIs(t, err, assert.AnError)
}

func TestCalculateInvoiceInterestEdgeCases(t *testing.T) {
	asOfDate := time.Date(2026, 6, 11, 0, 0, 0, 0, time.UTC)
	notOverdue := calculateInvoiceInterest(&interestInvoice{
		ID:            "inv-current",
		InvoiceNumber: "INV-CURRENT",
		DueDate:       asOfDate.AddDate(0, 0, 1),
		Total:         decimal.NewFromInt(1000),
		AmountPaid:    decimal.NewFromInt(100),
		Currency:      "EUR",
	}, 0.001, asOfDate)
	assert.Equal(t, 0, notOverdue.DaysOverdue)
	assert.True(t, notOverdue.OutstandingAmount.Equal(decimal.NewFromInt(900)))
	assert.True(t, notOverdue.DailyInterest.Equal(decimal.RequireFromString("0.90")))
	assert.True(t, notOverdue.TotalInterest.IsZero())
	assert.True(t, notOverdue.TotalWithInterest.Equal(decimal.NewFromInt(900)))

	fullyPaid := calculateInvoiceInterest(&interestInvoice{
		ID:            "inv-paid",
		InvoiceNumber: "INV-PAID",
		DueDate:       asOfDate.AddDate(0, 0, -10),
		Total:         decimal.NewFromInt(1000),
		AmountPaid:    decimal.NewFromInt(1000),
		Currency:      "EUR",
	}, 0.001, asOfDate)
	assert.Equal(t, 0, fullyPaid.DaysOverdue)
	assert.True(t, fullyPaid.OutstandingAmount.IsZero())
	assert.True(t, fullyPaid.DailyInterest.IsZero())
	assert.True(t, fullyPaid.TotalInterest.IsZero())
	assert.True(t, fullyPaid.TotalWithInterest.IsZero())
}

func TestInvoiceInterestModelMappings(t *testing.T) {
	calculatedAt := time.Date(2026, 6, 11, 0, 0, 0, 0, time.UTC)
	createdAt := calculatedAt.Add(time.Minute)
	interest := &InvoiceInterest{
		ID:                "interest-1",
		InvoiceID:         "inv-1",
		CalculatedAt:      calculatedAt,
		DaysOverdue:       10,
		PrincipalAmount:   decimal.NewFromInt(800),
		InterestRate:      decimal.RequireFromString("0.001"),
		InterestAmount:    decimal.RequireFromString("8.00"),
		TotalWithInterest: decimal.RequireFromString("808.00"),
		CreatedAt:         createdAt,
	}

	model := invoiceInterestToModel(interest)
	assert.Equal(t, interest.ID, model.ID)
	assert.Equal(t, interest.InvoiceID, model.InvoiceID)
	assert.Equal(t, interest.CalculatedAt, model.CalculatedAt)
	assert.Equal(t, interest.DaysOverdue, model.DaysOverdue)
	assert.True(t, model.PrincipalAmount.Decimal.Equal(interest.PrincipalAmount))
	assert.True(t, model.InterestRate.Decimal.Equal(interest.InterestRate))
	assert.True(t, model.InterestAmount.Decimal.Equal(interest.InterestAmount))
	assert.True(t, model.TotalWithInterest.Decimal.Equal(interest.TotalWithInterest))
	assert.Equal(t, interest.CreatedAt, model.CreatedAt)

	roundTrip := invoiceInterestFromModel(&models.InvoiceInterest{
		ID:                model.ID,
		InvoiceID:         model.InvoiceID,
		CalculatedAt:      model.CalculatedAt,
		DaysOverdue:       model.DaysOverdue,
		PrincipalAmount:   model.PrincipalAmount,
		InterestRate:      model.InterestRate,
		InterestAmount:    model.InterestAmount,
		TotalWithInterest: model.TotalWithInterest,
		CreatedAt:         model.CreatedAt,
	})
	assert.Equal(t, interest.ID, roundTrip.ID)
	assert.True(t, roundTrip.TotalWithInterest.Equal(interest.TotalWithInterest))
}

type fakeInterestRepository struct {
	invoices          map[string]*interestInvoice
	created           []*InvoiceInterest
	latest            *InvoiceInterest
	history           []InvoiceInterest
	overdue           []interestInvoice
	listOverdueCalled bool
	getErr            error
	createErr         error
	latestErr         error
	historyErr        error
	listOverdueErr    error
}

func (f *fakeInterestRepository) GetInvoiceForInterest(ctx context.Context, schemaName, tenantID, invoiceID string) (*interestInvoice, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.invoices[invoiceID], nil
}

func (f *fakeInterestRepository) CreateInterest(ctx context.Context, schemaName string, interest *InvoiceInterest) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.created = append(f.created, interest)
	return nil
}

func (f *fakeInterestRepository) GetLatestInterest(ctx context.Context, schemaName, invoiceID string) (*InvoiceInterest, error) {
	if f.latestErr != nil {
		return nil, f.latestErr
	}
	return f.latest, nil
}

func (f *fakeInterestRepository) ListInterestHistory(ctx context.Context, schemaName, invoiceID string) ([]InvoiceInterest, error) {
	if f.historyErr != nil {
		return nil, f.historyErr
	}
	return append([]InvoiceInterest(nil), f.history...), nil
}

func (f *fakeInterestRepository) ListOverdueInvoices(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) ([]interestInvoice, error) {
	f.listOverdueCalled = true
	if f.listOverdueErr != nil {
		return nil, f.listOverdueErr
	}
	return append([]interestInvoice(nil), f.overdue...), nil
}
