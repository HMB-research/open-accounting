package expenses

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	Create(ctx context.Context, schemaName string, expense *Expense) error
	GetByID(ctx context.Context, schemaName, tenantID, expenseID string) (*Expense, error)
	List(ctx context.Context, schemaName, tenantID string, filter ListExpensesFilter) ([]Expense, error)
	Update(ctx context.Context, schemaName string, expense *Expense) error
	GenerateNumber(ctx context.Context, schemaName, tenantID string) (string, error)
}

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Create(ctx context.Context, schemaName string, expense *Expense) error {
	table, err := expensesTable(schemaName)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s (
			id, tenant_id, expense_number, expense_date, merchant, description, employee_id, contact_id,
			expense_account_id, payment_account_id, amount, currency, exchange_rate, base_amount, requires_receipt,
			status, journal_entry_id, submitted_at, submitted_by, approved_at, approved_by, rejected_at, rejected_by,
			rejection_reason, posted_at, posted_by, created_at, created_by, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13, $14, $15,
			$16, $17, $18, $19, $20, $21, $22, $23,
			NULLIF($24, ''), $25, $26, $27, $28, $29
		)
	`, table),
		expense.ID, expense.TenantID, expense.ExpenseNumber, expense.ExpenseDate, expense.Merchant, expense.Description,
		expense.EmployeeID, expense.ContactID, expense.ExpenseAccountID, expense.PaymentAccountID, expense.Amount,
		expense.Currency, expense.ExchangeRate, expense.BaseAmount, expense.RequiresReceipt, expense.Status,
		expense.JournalEntryID, expense.SubmittedAt, expense.SubmittedBy, expense.ApprovedAt, expense.ApprovedBy,
		expense.RejectedAt, expense.RejectedBy, expense.RejectionReason, expense.PostedAt, expense.PostedBy,
		expense.CreatedAt, expense.CreatedBy, expense.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create expense: %w", err)
	}

	return nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, schemaName, tenantID, expenseID string) (*Expense, error) {
	table, err := expensesTable(schemaName)
	if err != nil {
		return nil, err
	}

	row := r.db.QueryRow(ctx, expenseSelectSQL(table)+`
		WHERE id = $1 AND tenant_id = $2
	`, expenseID, tenantID)
	expense, err := scanExpense(row)
	if err == pgx.ErrNoRows {
		return nil, ErrExpenseNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get expense: %w", err)
	}
	return expense, nil
}

func (r *PostgresRepository) List(ctx context.Context, schemaName, tenantID string, filter ListExpensesFilter) ([]Expense, error) {
	table, err := expensesTable(schemaName)
	if err != nil {
		return nil, err
	}

	conditions := []string{"tenant_id = $1"}
	args := []any{tenantID}
	if filter.Status != "" {
		args = append(args, filter.Status)
		conditions = append(conditions, fmt.Sprintf("status = $%d", len(args)))
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	args = append(args, limit)
	limitPlaceholder := len(args)

	rows, err := r.db.Query(ctx, fmt.Sprintf(`%s
		WHERE %s
		ORDER BY expense_date DESC, created_at DESC
		LIMIT $%d
	`, expenseSelectSQL(table), strings.Join(conditions, " AND "), limitPlaceholder), args...)
	if err != nil {
		return nil, fmt.Errorf("list expenses: %w", err)
	}
	defer rows.Close()

	var result []Expense
	for rows.Next() {
		expense, err := scanExpense(rows)
		if err != nil {
			return nil, fmt.Errorf("scan expense: %w", err)
		}
		result = append(result, *expense)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate expenses: %w", err)
	}

	return result, nil
}

func (r *PostgresRepository) Update(ctx context.Context, schemaName string, expense *Expense) error {
	table, err := expensesTable(schemaName)
	if err != nil {
		return err
	}

	expense.UpdatedAt = time.Now().UTC()
	result, err := r.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s
		SET status = $1,
		    journal_entry_id = $2,
		    submitted_at = $3,
		    submitted_by = $4,
		    approved_at = $5,
		    approved_by = $6,
		    rejected_at = $7,
		    rejected_by = $8,
		    rejection_reason = NULLIF($9, ''),
		    posted_at = $10,
		    posted_by = $11,
		    updated_at = $12
		WHERE id = $13 AND tenant_id = $14
	`, table),
		expense.Status, expense.JournalEntryID, expense.SubmittedAt, expense.SubmittedBy, expense.ApprovedAt,
		expense.ApprovedBy, expense.RejectedAt, expense.RejectedBy, expense.RejectionReason, expense.PostedAt,
		expense.PostedBy, expense.UpdatedAt, expense.ID, expense.TenantID,
	)
	if err != nil {
		return fmt.Errorf("update expense: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrExpenseNotFound
	}

	return nil
}

func (r *PostgresRepository) GenerateNumber(ctx context.Context, schemaName, tenantID string) (string, error) {
	table, err := expensesTable(schemaName)
	if err != nil {
		return "", err
	}

	var seq int
	if err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(MAX(
			CASE
				WHEN expense_number ~ '[0-9]+$' THEN CAST(SUBSTRING(expense_number FROM '([0-9]+)$') AS INTEGER)
				ELSE 0
			END
		), 0) + 1
		FROM %s
		WHERE tenant_id = $1
	`, table), tenantID).Scan(&seq); err != nil {
		return "", fmt.Errorf("generate expense number: %w", err)
	}

	return fmt.Sprintf("EXP-%05d", seq), nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanExpense(row scanner) (*Expense, error) {
	var expense Expense
	err := row.Scan(
		&expense.ID, &expense.TenantID, &expense.ExpenseNumber, &expense.ExpenseDate, &expense.Merchant,
		&expense.Description, &expense.EmployeeID, &expense.ContactID, &expense.ExpenseAccountID,
		&expense.PaymentAccountID, &expense.Amount, &expense.Currency, &expense.ExchangeRate, &expense.BaseAmount,
		&expense.RequiresReceipt, &expense.Status, &expense.JournalEntryID, &expense.SubmittedAt, &expense.SubmittedBy,
		&expense.ApprovedAt, &expense.ApprovedBy, &expense.RejectedAt, &expense.RejectedBy, &expense.RejectionReason,
		&expense.PostedAt, &expense.PostedBy, &expense.CreatedAt, &expense.CreatedBy, &expense.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &expense, nil
}

func expenseSelectSQL(table string) string {
	return fmt.Sprintf(`
		SELECT id, tenant_id, expense_number, expense_date, merchant, COALESCE(description, ''), employee_id, contact_id,
		       expense_account_id, payment_account_id, amount, currency, exchange_rate, base_amount, requires_receipt,
		       status, journal_entry_id, submitted_at, submitted_by, approved_at, approved_by, rejected_at, rejected_by,
		       COALESCE(rejection_reason, ''), posted_at, posted_by, created_at, created_by, updated_at
		FROM %s
	`, table)
}

func expensesTable(schemaName string) (string, error) {
	table, err := database.QualifiedTable(schemaName, "expenses")
	if err != nil {
		return "", fmt.Errorf("qualify expenses table: %w", err)
	}
	return table, nil
}
