package expenses

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGORMRepositoryNilDatabase(t *testing.T) {
	repo := NewGORMRepository(nil)
	ctx := context.Background()
	schemaName := "tenant_schema"
	tenantID := "tenant-1"

	require.NotNil(t, repo)
	assert.Nil(t, repo.db)
	assert.Nil(t, NewRepository(nil).db)

	tests := []struct {
		name string
		run  func(t *testing.T) error
	}{
		{
			name: "Create",
			run: func(t *testing.T) error {
				return repo.Create(ctx, schemaName, &Expense{TenantID: tenantID})
			},
		},
		{
			name: "GetByID",
			run: func(t *testing.T) error {
				expense, err := repo.GetByID(ctx, schemaName, tenantID, "expense-1")
				assert.Nil(t, expense)
				return err
			},
		},
		{
			name: "List",
			run: func(t *testing.T) error {
				expenses, err := repo.List(ctx, schemaName, tenantID, ListExpensesFilter{
					Status: StatusSubmitted,
					Limit:  25,
				})
				assert.Nil(t, expenses)
				return err
			},
		},
		{
			name: "Update",
			run: func(t *testing.T) error {
				return repo.Update(ctx, schemaName, &Expense{ID: "expense-1", TenantID: tenantID})
			},
		},
		{
			name: "GenerateNumber",
			run: func(t *testing.T) error {
				number, err := repo.GenerateNumber(ctx, schemaName, tenantID)
				assert.Empty(t, number)
				return err
			},
		},
		{
			name: "tenantTable",
			run: func(t *testing.T) error {
				table, err := repo.tenantTable(ctx, schemaName)
				assert.Nil(t, table)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(t)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "expenses repository database is not configured")
		})
	}
}

func TestExpenseModelMappingRoundTrip(t *testing.T) {
	employeeID := "employee-id"
	contactID := "contact-id"
	journalEntryID := "journal-entry-id"
	submittedBy := "submitter-id"
	approvedBy := "approver-id"
	rejectedBy := "rejecter-id"
	postedBy := "poster-id"
	expenseDate := time.Date(2026, time.May, 30, 0, 0, 0, 0, time.UTC)
	submittedAt := time.Date(2026, time.May, 31, 9, 0, 0, 0, time.UTC)
	approvedAt := time.Date(2026, time.May, 31, 10, 0, 0, 0, time.UTC)
	rejectedAt := time.Date(2026, time.May, 31, 11, 0, 0, 0, time.UTC)
	postedAt := time.Date(2026, time.May, 31, 12, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, time.May, 30, 8, 30, 0, 0, time.UTC)
	updatedAt := time.Date(2026, time.May, 31, 12, 15, 0, 0, time.UTC)
	expense := &Expense{
		ID:               "expense-id",
		TenantID:         "tenant-id",
		ExpenseNumber:    "EXP-00042",
		ExpenseDate:      expenseDate,
		Merchant:         "Integration Cafe",
		Description:      "Team lunch",
		EmployeeID:       &employeeID,
		ContactID:        &contactID,
		ExpenseAccountID: "expense-account-id",
		PaymentAccountID: "payment-account-id",
		Amount:           decimal.RequireFromString("42.50"),
		Currency:         "EUR",
		ExchangeRate:     decimal.RequireFromString("1.2000000000"),
		BaseAmount:       decimal.RequireFromString("51.00"),
		RequiresReceipt:  true,
		Status:           StatusRejected,
		JournalEntryID:   &journalEntryID,
		SubmittedAt:      &submittedAt,
		SubmittedBy:      &submittedBy,
		ApprovedAt:       &approvedAt,
		ApprovedBy:       &approvedBy,
		RejectedAt:       &rejectedAt,
		RejectedBy:       &rejectedBy,
		RejectionReason:  "Missing receipt",
		PostedAt:         &postedAt,
		PostedBy:         &postedBy,
		CreatedAt:        createdAt,
		CreatedBy:        "creator-id",
		UpdatedAt:        updatedAt,
	}

	model := expenseToModel(expense)
	assert.Equal(t, string(StatusRejected), model.Status)
	assert.True(t, model.Amount.Decimal.Equal(expense.Amount))
	assert.True(t, model.ExchangeRate.Decimal.Equal(expense.ExchangeRate))
	assert.True(t, model.BaseAmount.Decimal.Equal(expense.BaseAmount))

	roundTrip := modelToExpense(model)
	assert.Equal(t, expense, roundTrip)
}
