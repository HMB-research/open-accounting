package invoicing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type interestInvoice struct {
	ID            string
	InvoiceNumber string
	DueDate       time.Time
	Total         decimal.Decimal
	AmountPaid    decimal.Decimal
	Currency      string
	Status        string
}

type InterestRepository interface {
	GetInvoiceForInterest(ctx context.Context, schemaName, tenantID, invoiceID string) (*interestInvoice, error)
	CreateInterest(ctx context.Context, schemaName string, interest *InvoiceInterest) error
	GetLatestInterest(ctx context.Context, schemaName, invoiceID string) (*InvoiceInterest, error)
	ListInterestHistory(ctx context.Context, schemaName, invoiceID string) ([]InvoiceInterest, error)
	ListOverdueInvoices(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) ([]interestInvoice, error)
}

// InterestGORMRepository stores interest data through the shared ORM layer.
type InterestGORMRepository struct {
	db *gorm.DB
}

func NewInterestRepository(db *pgxpool.Pool) *InterestGORMRepository {
	if db == nil {
		return &InterestGORMRepository{}
	}
	gormDB, err := database.NewGormDBFromPool(context.Background(), db)
	if err != nil {
		panic(fmt.Errorf("create interest GORM repository: %w", err))
	}
	return NewInterestGORMRepository(gormDB)
}

func NewInterestGORMRepository(db *gorm.DB) *InterestGORMRepository {
	return &InterestGORMRepository{db: db}
}

func (r *InterestGORMRepository) tenantTable(ctx context.Context, schemaName, tableName string) (*gorm.DB, error) {
	return database.TenantTable(r.db.WithContext(ctx), schemaName, tableName)
}

func (r *InterestGORMRepository) GetInvoiceForInterest(ctx context.Context, schemaName, tenantID, invoiceID string) (*interestInvoice, error) {
	db, err := r.tenantTable(ctx, schemaName, "invoices")
	if err != nil {
		return nil, fmt.Errorf("qualify invoices table: %w", err)
	}

	var inv interestInvoice
	err = db.
		Select("id, invoice_number, due_date, total, amount_paid, currency, status").
		Where("id = ? AND tenant_id = ?", invoiceID, tenantID).
		First(&inv).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, &NotFoundError{Entity: "invoice"}
	}
	if err != nil {
		return nil, fmt.Errorf("get invoice: %w", err)
	}
	return &inv, nil
}

func (r *InterestGORMRepository) CreateInterest(ctx context.Context, schemaName string, interest *InvoiceInterest) error {
	db, err := r.tenantTable(ctx, schemaName, "invoice_interest")
	if err != nil {
		return fmt.Errorf("qualify invoice interest table: %w", err)
	}

	if err := db.Create(invoiceInterestToModel(interest)).Error; err != nil {
		return fmt.Errorf("save interest calculation: %w", err)
	}
	return nil
}

func (r *InterestGORMRepository) GetLatestInterest(ctx context.Context, schemaName, invoiceID string) (*InvoiceInterest, error) {
	db, err := r.tenantTable(ctx, schemaName, "invoice_interest")
	if err != nil {
		return nil, fmt.Errorf("qualify invoice interest table: %w", err)
	}

	var interestModel models.InvoiceInterest
	err = db.
		Where("invoice_id = ?", invoiceID).
		Order("calculated_at DESC").
		First(&interestModel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest interest: %w", err)
	}
	return invoiceInterestFromModel(&interestModel), nil
}

func (r *InterestGORMRepository) ListInterestHistory(ctx context.Context, schemaName, invoiceID string) ([]InvoiceInterest, error) {
	db, err := r.tenantTable(ctx, schemaName, "invoice_interest")
	if err != nil {
		return nil, fmt.Errorf("qualify invoice interest table: %w", err)
	}

	var interestModels []models.InvoiceInterest
	if err := db.
		Where("invoice_id = ?", invoiceID).
		Order("calculated_at DESC").
		Find(&interestModels).Error; err != nil {
		return nil, fmt.Errorf("list interest history: %w", err)
	}

	history := make([]InvoiceInterest, len(interestModels))
	for i := range interestModels {
		history[i] = *invoiceInterestFromModel(&interestModels[i])
	}
	return history, nil
}

func (r *InterestGORMRepository) ListOverdueInvoices(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) ([]interestInvoice, error) {
	db, err := r.tenantTable(ctx, schemaName, "invoices")
	if err != nil {
		return nil, fmt.Errorf("qualify invoices table: %w", err)
	}

	var invoices []interestInvoice
	if err := db.
		Select("id, invoice_number, due_date, total, amount_paid, currency").
		Where("tenant_id = ?", tenantID).
		Where("status IN ?", []string{"SENT", "PARTIALLY_PAID", "OVERDUE"}).
		Where("due_date < ?", asOfDate).
		Where("total > amount_paid").
		Order("due_date ASC").
		Find(&invoices).Error; err != nil {
		return nil, fmt.Errorf("list overdue invoices: %w", err)
	}
	return invoices, nil
}

// InterestService handles interest calculations for overdue invoices.
type InterestService struct {
	repo InterestRepository
}

// NewInterestService creates a new interest service.
func NewInterestService(db *pgxpool.Pool) *InterestService {
	return NewInterestServiceWithRepository(NewInterestRepository(db))
}

func NewInterestServiceWithRepository(repo InterestRepository) *InterestService {
	return &InterestService{repo: repo}
}

// CalculateInterest calculates current interest for an invoice.
func (s *InterestService) CalculateInterest(ctx context.Context, schemaName, tenantID, invoiceID string, interestRate float64, asOfDate time.Time) (*InterestCalculationResult, error) {
	inv, err := s.repo.GetInvoiceForInterest(ctx, schemaName, tenantID, invoiceID)
	if err != nil {
		return nil, err
	}

	return calculateInvoiceInterest(inv, interestRate, asOfDate), nil
}

// SaveInterestCalculation saves an interest calculation record.
func (s *InterestService) SaveInterestCalculation(ctx context.Context, schemaName string, result *InterestCalculationResult) (*InvoiceInterest, error) {
	interest := &InvoiceInterest{
		ID:                uuid.New().String(),
		InvoiceID:         result.InvoiceID,
		CalculatedAt:      result.CalculatedAt,
		DaysOverdue:       result.DaysOverdue,
		PrincipalAmount:   result.OutstandingAmount,
		InterestRate:      result.InterestRate,
		InterestAmount:    result.TotalInterest,
		TotalWithInterest: result.TotalWithInterest,
		CreatedAt:         time.Now(),
	}

	if err := s.repo.CreateInterest(ctx, schemaName, interest); err != nil {
		return nil, err
	}
	return interest, nil
}

// GetLatestInterest gets the most recent interest calculation for an invoice.
func (s *InterestService) GetLatestInterest(ctx context.Context, schemaName, invoiceID string) (*InvoiceInterest, error) {
	return s.repo.GetLatestInterest(ctx, schemaName, invoiceID)
}

// ListInterestHistory gets all interest calculations for an invoice.
func (s *InterestService) ListInterestHistory(ctx context.Context, schemaName, invoiceID string) ([]InvoiceInterest, error) {
	return s.repo.ListInterestHistory(ctx, schemaName, invoiceID)
}

// CalculateInterestForOverdueInvoices calculates interest for all overdue invoices of a tenant.
func (s *InterestService) CalculateInterestForOverdueInvoices(ctx context.Context, schemaName, tenantID string, interestRate float64) ([]InterestCalculationResult, error) {
	asOfDate := time.Now()
	invoices, err := s.repo.ListOverdueInvoices(ctx, schemaName, tenantID, asOfDate)
	if err != nil {
		return nil, err
	}

	results := make([]InterestCalculationResult, 0, len(invoices))
	for i := range invoices {
		results = append(results, *calculateInvoiceInterest(&invoices[i], interestRate, asOfDate))
	}
	return results, nil
}

func calculateInvoiceInterest(inv *interestInvoice, interestRate float64, asOfDate time.Time) *InterestCalculationResult {
	outstanding := inv.Total.Sub(inv.AmountPaid)
	if outstanding.LessThanOrEqual(decimal.Zero) {
		return &InterestCalculationResult{
			InvoiceID:         inv.ID,
			InvoiceNumber:     inv.InvoiceNumber,
			DueDate:           inv.DueDate,
			DaysOverdue:       0,
			OutstandingAmount: decimal.Zero,
			InterestRate:      decimal.NewFromFloat(interestRate),
			DailyInterest:     decimal.Zero,
			TotalInterest:     decimal.Zero,
			TotalWithInterest: decimal.Zero,
			CalculatedAt:      asOfDate,
			Currency:          inv.Currency,
		}
	}

	daysOverdue := 0
	if asOfDate.After(inv.DueDate) {
		daysOverdue = int(asOfDate.Sub(inv.DueDate).Hours() / 24)
	}

	rate := decimal.NewFromFloat(interestRate)
	dailyInterest := outstanding.Mul(rate).Round(2)
	totalInterest := dailyInterest.Mul(decimal.NewFromInt(int64(daysOverdue))).Round(2)

	return &InterestCalculationResult{
		InvoiceID:         inv.ID,
		InvoiceNumber:     inv.InvoiceNumber,
		DueDate:           inv.DueDate,
		DaysOverdue:       daysOverdue,
		OutstandingAmount: outstanding,
		InterestRate:      rate,
		DailyInterest:     dailyInterest,
		TotalInterest:     totalInterest,
		TotalWithInterest: outstanding.Add(totalInterest),
		CalculatedAt:      asOfDate,
		Currency:          inv.Currency,
	}
}

func invoiceInterestToModel(interest *InvoiceInterest) *models.InvoiceInterest {
	return &models.InvoiceInterest{
		ID:                interest.ID,
		InvoiceID:         interest.InvoiceID,
		CalculatedAt:      interest.CalculatedAt,
		DaysOverdue:       interest.DaysOverdue,
		PrincipalAmount:   models.Decimal{Decimal: interest.PrincipalAmount},
		InterestRate:      models.Decimal{Decimal: interest.InterestRate},
		InterestAmount:    models.Decimal{Decimal: interest.InterestAmount},
		TotalWithInterest: models.Decimal{Decimal: interest.TotalWithInterest},
		CreatedAt:         interest.CreatedAt,
	}
}

func invoiceInterestFromModel(interest *models.InvoiceInterest) *InvoiceInterest {
	return &InvoiceInterest{
		ID:                interest.ID,
		InvoiceID:         interest.InvoiceID,
		CalculatedAt:      interest.CalculatedAt,
		DaysOverdue:       interest.DaysOverdue,
		PrincipalAmount:   interest.PrincipalAmount.Decimal,
		InterestRate:      interest.InterestRate.Decimal,
		InterestAmount:    interest.InterestAmount.Decimal,
		TotalWithInterest: interest.TotalWithInterest.Decimal,
		CreatedAt:         interest.CreatedAt,
	}
}
