package expenses

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpensesWave4ImportNilRequestAndCreateError(t *testing.T) {
	service := NewServiceWithRepository(newMemoryRepository(), nil, nil)

	_, err := service.ImportExpensesCSV(context.Background(), "tenant_schema", "tenant-1", nil)
	require.ErrorContains(t, err, "csv_content is required")

	repo := newMemoryRepository()
	repo.createErr = errors.New("insert failed")
	service = NewServiceWithRepository(repo, nil, nil)
	service.now = fixedExpenseNow

	result, err := service.ImportExpensesCSV(context.Background(), "tenant_schema", "tenant-1", &ImportExpensesRequest{
		UserID: "user-1",
		CSVContent: "expense_number,expense_date,merchant,expense_account_id,payment_account_id,amount\n" +
			"EXP-1,2026-06-01,Vendor,99999999-9999-4999-8999-999999999999,aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa,12.50\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Zero(t, result.ExpensesCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "insert failed")
}

func TestExpensesWave4PostExpenseAccountLookupErrors(t *testing.T) {
	t.Run("expense account lookup", func(t *testing.T) {
		repo := newMemoryRepository()
		accountingSvc := newFakeAccountingPoster()
		delete(accountingSvc.accounts, "expense-account")
		service := NewServiceWithRepository(repo, accountingSvc, &fakeEvidenceEvaluator{compliant: true})
		service.now = fixedExpenseNow
		expense := approvedTestExpense(t, service)

		_, err := service.PostExpense(context.Background(), "tenant_schema", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-3"})

		require.ErrorIs(t, err, ErrExpenseAccountingInvalid)
		assert.Contains(t, err.Error(), "load expense account")
	})

	t.Run("payment account lookup", func(t *testing.T) {
		repo := newMemoryRepository()
		accountingSvc := newFakeAccountingPoster()
		delete(accountingSvc.accounts, "cash-account")
		service := NewServiceWithRepository(repo, accountingSvc, &fakeEvidenceEvaluator{compliant: true})
		service.now = fixedExpenseNow
		expense := approvedTestExpense(t, service)

		_, err := service.PostExpense(context.Background(), "tenant_schema", "tenant-1", expense.ID, &ExpenseActionRequest{UserID: "user-3"})

		require.ErrorIs(t, err, ErrExpenseAccountingInvalid)
		assert.Contains(t, err.Error(), "load payment account")
	})
}

func TestExpensesWave4HydrationAndActionUserHelpers(t *testing.T) {
	hydrateExpenseDerivedFields(nil)

	expenses := []Expense{{Status: StatusDraft, RequiresReceipt: true, ExpenseNumber: "EXP-1"}}
	hydrateExpenseSlice(expenses)
	require.NotEmpty(t, expenses[0].RemediationActions)
	assert.Equal(t, "expense_receipt_required", expenses[0].RemediationActions[0].Code)

	userID, err := actionUserID(&ExpenseActionRequest{UserID: " user-1 "})
	require.NoError(t, err)
	assert.Equal(t, "user-1", userID)
}
