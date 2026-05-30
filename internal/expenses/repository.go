package expenses

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/gorm"
)

type Repository interface {
	Create(ctx context.Context, schemaName string, expense *Expense) error
	GetByID(ctx context.Context, schemaName, tenantID, expenseID string) (*Expense, error)
	List(ctx context.Context, schemaName, tenantID string, filter ListExpensesFilter) ([]Expense, error)
	Update(ctx context.Context, schemaName string, expense *Expense) error
	GenerateNumber(ctx context.Context, schemaName, tenantID string) (string, error)
}

type GORMRepository struct {
	db *gorm.DB
}

func NewRepository(db *pgxpool.Pool) *GORMRepository {
	if db == nil {
		return &GORMRepository{}
	}
	gormDB, err := database.NewGormDBFromPool(context.Background(), db)
	if err != nil {
		panic(fmt.Errorf("create expenses GORM repository: %w", err))
	}
	return NewGORMRepository(gormDB)
}

func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

func (r *GORMRepository) tenantTable(ctx context.Context, schemaName string) (*gorm.DB, error) {
	return database.TenantTable(r.db.WithContext(ctx), schemaName, "expenses")
}

func (r *GORMRepository) Create(ctx context.Context, schemaName string, expense *Expense) error {
	db, err := r.tenantTable(ctx, schemaName)
	if err != nil {
		return err
	}

	if err := db.Create(expenseToModel(expense)).Error; err != nil {
		return fmt.Errorf("create expense: %w", err)
	}
	return nil
}

func (r *GORMRepository) GetByID(ctx context.Context, schemaName, tenantID, expenseID string) (*Expense, error) {
	db, err := r.tenantTable(ctx, schemaName)
	if err != nil {
		return nil, err
	}

	var expenseModel models.Expense
	err = db.Where("id = ? AND tenant_id = ?", expenseID, tenantID).First(&expenseModel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrExpenseNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get expense: %w", err)
	}
	return modelToExpense(&expenseModel), nil
}

func (r *GORMRepository) List(ctx context.Context, schemaName, tenantID string, filter ListExpensesFilter) ([]Expense, error) {
	db, err := r.tenantTable(ctx, schemaName)
	if err != nil {
		return nil, err
	}

	query := db.Where("tenant_id = ?", tenantID)
	if filter.Status != "" {
		query = query.Where("status = ?", string(filter.Status))
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	var expenseModels []models.Expense
	if err := query.
		Order("expense_date DESC, created_at DESC").
		Limit(limit).
		Find(&expenseModels).Error; err != nil {
		return nil, fmt.Errorf("list expenses: %w", err)
	}

	result := make([]Expense, len(expenseModels))
	for i := range expenseModels {
		result[i] = *modelToExpense(&expenseModels[i])
	}
	return result, nil
}

func (r *GORMRepository) Update(ctx context.Context, schemaName string, expense *Expense) error {
	db, err := r.tenantTable(ctx, schemaName)
	if err != nil {
		return err
	}

	expense.UpdatedAt = time.Now().UTC()
	result := db.Where("id = ? AND tenant_id = ?", expense.ID, expense.TenantID).
		Updates(map[string]interface{}{
			"status":           string(expense.Status),
			"journal_entry_id": expense.JournalEntryID,
			"submitted_at":     expense.SubmittedAt,
			"submitted_by":     expense.SubmittedBy,
			"approved_at":      expense.ApprovedAt,
			"approved_by":      expense.ApprovedBy,
			"rejected_at":      expense.RejectedAt,
			"rejected_by":      expense.RejectedBy,
			"rejection_reason": expense.RejectionReason,
			"posted_at":        expense.PostedAt,
			"posted_by":        expense.PostedBy,
			"updated_at":       expense.UpdatedAt,
		})
	if result.Error != nil {
		return fmt.Errorf("update expense: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrExpenseNotFound
	}
	return nil
}

func (r *GORMRepository) GenerateNumber(ctx context.Context, schemaName, tenantID string) (string, error) {
	db, err := r.tenantTable(ctx, schemaName)
	if err != nil {
		return "", err
	}

	var seq int
	if err := db.
		Select(`
			COALESCE(MAX(
				CASE
					WHEN expense_number ~ ? THEN CAST(SUBSTRING(expense_number FROM ?) AS INTEGER)
					ELSE 0
				END
			), 0) + 1
		`, "[0-9]+$", "([0-9]+)$").
		Where("tenant_id = ?", tenantID).
		Scan(&seq).Error; err != nil {
		return "", fmt.Errorf("generate expense number: %w", err)
	}

	return fmt.Sprintf("EXP-%05d", seq), nil
}

func expenseToModel(expense *Expense) *models.Expense {
	return &models.Expense{
		ID:               expense.ID,
		TenantID:         expense.TenantID,
		ExpenseNumber:    expense.ExpenseNumber,
		ExpenseDate:      expense.ExpenseDate,
		Merchant:         expense.Merchant,
		Description:      expense.Description,
		EmployeeID:       expense.EmployeeID,
		ContactID:        expense.ContactID,
		ExpenseAccountID: expense.ExpenseAccountID,
		PaymentAccountID: expense.PaymentAccountID,
		Amount:           models.Decimal{Decimal: expense.Amount},
		Currency:         expense.Currency,
		ExchangeRate:     models.Decimal{Decimal: expense.ExchangeRate},
		BaseAmount:       models.Decimal{Decimal: expense.BaseAmount},
		RequiresReceipt:  expense.RequiresReceipt,
		Status:           string(expense.Status),
		JournalEntryID:   expense.JournalEntryID,
		SubmittedAt:      expense.SubmittedAt,
		SubmittedBy:      expense.SubmittedBy,
		ApprovedAt:       expense.ApprovedAt,
		ApprovedBy:       expense.ApprovedBy,
		RejectedAt:       expense.RejectedAt,
		RejectedBy:       expense.RejectedBy,
		RejectionReason:  expense.RejectionReason,
		PostedAt:         expense.PostedAt,
		PostedBy:         expense.PostedBy,
		CreatedAt:        expense.CreatedAt,
		CreatedBy:        expense.CreatedBy,
		UpdatedAt:        expense.UpdatedAt,
	}
}

func modelToExpense(expense *models.Expense) *Expense {
	return &Expense{
		ID:               expense.ID,
		TenantID:         expense.TenantID,
		ExpenseNumber:    expense.ExpenseNumber,
		ExpenseDate:      expense.ExpenseDate,
		Merchant:         expense.Merchant,
		Description:      expense.Description,
		EmployeeID:       expense.EmployeeID,
		ContactID:        expense.ContactID,
		ExpenseAccountID: expense.ExpenseAccountID,
		PaymentAccountID: expense.PaymentAccountID,
		Amount:           expense.Amount.Decimal,
		Currency:         expense.Currency,
		ExchangeRate:     expense.ExchangeRate.Decimal,
		BaseAmount:       expense.BaseAmount.Decimal,
		RequiresReceipt:  expense.RequiresReceipt,
		Status:           ExpenseStatus(expense.Status),
		JournalEntryID:   expense.JournalEntryID,
		SubmittedAt:      expense.SubmittedAt,
		SubmittedBy:      expense.SubmittedBy,
		ApprovedAt:       expense.ApprovedAt,
		ApprovedBy:       expense.ApprovedBy,
		RejectedAt:       expense.RejectedAt,
		RejectedBy:       expense.RejectedBy,
		RejectionReason:  expense.RejectionReason,
		PostedAt:         expense.PostedAt,
		PostedBy:         expense.PostedBy,
		CreatedAt:        expense.CreatedAt,
		CreatedBy:        expense.CreatedBy,
		UpdatedAt:        expense.UpdatedAt,
	}
}
