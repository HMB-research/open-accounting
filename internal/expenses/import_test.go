package expenses

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceImportExpensesCSV(t *testing.T) {
	repo := newMemoryRepository()
	service := NewServiceWithRepository(repo, newFakeAccountingPoster(), &fakeEvidenceEvaluator{compliant: true})
	service.now = fixedExpenseNow

	result, err := service.ImportExpensesCSV(context.Background(), "tenant_acme", "tenant-1", &ImportExpensesRequest{
		FileName: "expenses.csv",
		UserID:   "user-1",
		CSVContent: "expense_number,expense_date,merchant,description,expense_account_code,payment_account_code,amount,currency,exchange_rate,requires_receipt,status\n" +
			"EXP-LEG-1,2026-05-30,Office Store,Toner,5500,1000,120.50,EUR,1,true,DRAFT\n" +
			"EXP-LEG-2,2026-05-31,Taxi,Ride,5500,1000,25.00,EUR,1,false,APPROVED\n",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "expenses.csv", result.FileName)
	assert.Equal(t, 2, result.RowsProcessed)
	assert.Equal(t, 2, result.ExpensesCreated)
	assert.Equal(t, 0, result.RowsSkipped)
	assert.Len(t, repo.expenses, 2)
	assert.Equal(t, StatusDraft, repo.expensesByNumber(t, "EXP-LEG-1").Status)
	approved := repo.expensesByNumber(t, "EXP-LEG-2")
	assert.Equal(t, StatusApproved, approved.Status)
	assert.Equal(t, "expense-account", approved.ExpenseAccountID)
	assert.Equal(t, "cash-account", approved.PaymentAccountID)
	assert.False(t, approved.RequiresReceipt)
	require.NotNil(t, approved.ApprovedAt)
}

func TestServiceImportExpensesCSVSkipsInvalidRows(t *testing.T) {
	repo := newMemoryRepository()
	service := NewServiceWithRepository(repo, newFakeAccountingPoster(), &fakeEvidenceEvaluator{compliant: true})
	service.now = fixedExpenseNow
	lockDate := time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC)

	result, err := service.ImportExpensesCSV(context.Background(), "tenant_acme", "tenant-1", &ImportExpensesRequest{
		UserID:   "user-1",
		LockDate: &lockDate,
		CSVContent: "expense_number,expense_date,merchant,expense_account_id,payment_account_id,amount,status,rejection_reason\n" +
			"EXP-LOCKED,2026-05-30,Locked,expense-account,cash-account,10,DRAFT,\n" +
			"EXP-POSTED,2026-05-31,Posted,expense-account,cash-account,20,POSTED,\n" +
			"EXP-REJECTED,2026-05-31,Rejected,expense-account,cash-account,30,REJECTED,Missing receipt\n",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 3, result.RowsProcessed)
	assert.Equal(t, 1, result.ExpensesCreated)
	assert.Equal(t, 2, result.RowsSkipped)
	require.Len(t, result.Errors, 2)
	assert.Contains(t, result.Errors[0].Message, "period locked through 2026-05-30")
	assert.Contains(t, result.Errors[1].Message, "posted expenses must be imported")
	assert.Equal(t, StatusRejected, repo.expensesByNumber(t, "EXP-REJECTED").Status)
}

func (r *memoryRepository) expensesByNumber(t *testing.T, expenseNumber string) *Expense {
	t.Helper()
	for _, expense := range r.expenses {
		if expense.ExpenseNumber == expenseNumber {
			return expense
		}
	}
	t.Fatalf("expense %s not found", expenseNumber)
	return nil
}
