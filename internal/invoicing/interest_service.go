package invoicing

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// InterestService handles interest calculations for overdue invoices
type InterestService struct {
	db *pgxpool.Pool
}

// NewInterestService creates a new interest service
func NewInterestService(db *pgxpool.Pool) *InterestService {
	return &InterestService{db: db}
}

// CalculateInterest calculates current interest for an invoice
func (s *InterestService) CalculateInterest(ctx context.Context, schemaName, tenantID, invoiceID string, interestRate float64, asOfDate time.Time) (*InterestCalculationResult, error) {
	// Get invoice details
	query := fmt.Sprintf(`
		SELECT id, invoice_number, due_date, total, amount_paid, currency, status
		FROM %s.invoices
		WHERE id = $1 AND tenant_id = $2
	`, schemaName)

	var inv struct {
		ID            string
		InvoiceNumber string
		DueDate       time.Time
		Total         decimal.Decimal
		AmountPaid    decimal.Decimal
		Currency      string
		Status        string
	}

	err := s.db.QueryRow(ctx, query, invoiceID, tenantID).Scan(
		&inv.ID, &inv.InvoiceNumber, &inv.DueDate, &inv.Total, &inv.AmountPaid, &inv.Currency, &inv.Status,
	)
	if err == pgx.ErrNoRows {
		return nil, &NotFoundError{Entity: "invoice"}
	}
	if err != nil {
		return nil, fmt.Errorf("get invoice: %w", err)
	}

	// Calculate outstanding amount
	outstanding := inv.Total.Sub(inv.AmountPaid)
	if outstanding.LessThanOrEqual(decimal.Zero) {
		// Invoice is fully paid, no interest
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
		}, nil
	}

	// Calculate days overdue
	daysOverdue := 0
	if asOfDate.After(inv.DueDate) {
		daysOverdue = int(asOfDate.Sub(inv.DueDate).Hours() / 24)
	}

	// Calculate interest: outstanding × rate × days
	rate := decimal.NewFromFloat(interestRate)
	dailyInterest := outstanding.Mul(rate).Round(2)
	totalInterest := dailyInterest.Mul(decimal.NewFromInt(int64(daysOverdue))).Round(2)
	totalWithInterest := outstanding.Add(totalInterest)

	return &InterestCalculationResult{
		InvoiceID:         inv.ID,
		InvoiceNumber:     inv.InvoiceNumber,
		DueDate:           inv.DueDate,
		DaysOverdue:       daysOverdue,
		OutstandingAmount: outstanding,
		InterestRate:      rate,
		DailyInterest:     dailyInterest,
		TotalInterest:     totalInterest,
		TotalWithInterest: totalWithInterest,
		CalculatedAt:      asOfDate,
		Currency:          inv.Currency,
	}, nil
}

// SaveInterestCalculation saves an interest calculation record
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

	query := fmt.Sprintf(`
		INSERT INTO %s.invoice_interest (
			id, invoice_id, calculated_at, days_overdue, principal_amount,
			interest_rate, interest_amount, total_with_interest, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, schemaName)

	_, err := s.db.Exec(ctx, query,
		interest.ID, interest.InvoiceID, interest.CalculatedAt, interest.DaysOverdue,
		interest.PrincipalAmount, interest.InterestRate, interest.InterestAmount,
		interest.TotalWithInterest, interest.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("save interest calculation: %w", err)
	}

	return interest, nil
}

// GetLatestInterest gets the most recent interest calculation for an invoice
func (s *InterestService) GetLatestInterest(ctx context.Context, schemaName, invoiceID string) (*InvoiceInterest, error) {
	query := fmt.Sprintf(`
		SELECT id, invoice_id, calculated_at, days_overdue, principal_amount,
			   interest_rate, interest_amount, total_with_interest, created_at
		FROM %s.invoice_interest
		WHERE invoice_id = $1
		ORDER BY calculated_at DESC
		LIMIT 1
	`, schemaName)

	var interest InvoiceInterest
	err := s.db.QueryRow(ctx, query, invoiceID).Scan(
		&interest.ID, &interest.InvoiceID, &interest.CalculatedAt, &interest.DaysOverdue,
		&interest.PrincipalAmount, &interest.InterestRate, &interest.InterestAmount,
		&interest.TotalWithInterest, &interest.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil // No interest calculated yet
	}
	if err != nil {
		return nil, fmt.Errorf("get latest interest: %w", err)
	}

	return &interest, nil
}

// ListInterestHistory gets all interest calculations for an invoice
func (s *InterestService) ListInterestHistory(ctx context.Context, schemaName, invoiceID string) ([]InvoiceInterest, error) {
	query := fmt.Sprintf(`
		SELECT id, invoice_id, calculated_at, days_overdue, principal_amount,
			   interest_rate, interest_amount, total_with_interest, created_at
		FROM %s.invoice_interest
		WHERE invoice_id = $1
		ORDER BY calculated_at DESC
	`, schemaName)

	rows, err := s.db.Query(ctx, query, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("list interest history: %w", err)
	}
	defer rows.Close()

	var history []InvoiceInterest
	for rows.Next() {
		var interest InvoiceInterest
		if err := rows.Scan(
			&interest.ID, &interest.InvoiceID, &interest.CalculatedAt, &interest.DaysOverdue,
			&interest.PrincipalAmount, &interest.InterestRate, &interest.InterestAmount,
			&interest.TotalWithInterest, &interest.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan interest: %w", err)
		}
		history = append(history, interest)
	}

	return history, nil
}

// CalculateInterestForOverdueInvoices calculates interest for all overdue invoices of a tenant
func (s *InterestService) CalculateInterestForOverdueInvoices(ctx context.Context, schemaName, tenantID string, interestRate float64) ([]InterestCalculationResult, error) {
	query := fmt.Sprintf(`
		SELECT id, invoice_number, due_date, total, amount_paid, currency
		FROM %s.invoices
		WHERE tenant_id = $1
		  AND status IN ('SENT', 'PARTIALLY_PAID', 'OVERDUE')
		  AND due_date < NOW()
		  AND total > amount_paid
		ORDER BY due_date ASC
	`, schemaName)

	rows, err := s.db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list overdue invoices: %w", err)
	}
	defer rows.Close()

	asOfDate := time.Now()
	var results []InterestCalculationResult

	for rows.Next() {
		var inv struct {
			ID            string
			InvoiceNumber string
			DueDate       time.Time
			Total         decimal.Decimal
			AmountPaid    decimal.Decimal
			Currency      string
		}
		if err := rows.Scan(&inv.ID, &inv.InvoiceNumber, &inv.DueDate, &inv.Total, &inv.AmountPaid, &inv.Currency); err != nil {
			return nil, fmt.Errorf("scan invoice: %w", err)
		}

		outstanding := inv.Total.Sub(inv.AmountPaid)
		daysOverdue := int(asOfDate.Sub(inv.DueDate).Hours() / 24)

		rate := decimal.NewFromFloat(interestRate)
		dailyInterest := outstanding.Mul(rate).Round(2)
		totalInterest := dailyInterest.Mul(decimal.NewFromInt(int64(daysOverdue))).Round(2)

		results = append(results, InterestCalculationResult{
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
		})
	}

	return results, nil
}
