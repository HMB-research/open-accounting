package expenses

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpenseImportWave8ParserBranches(t *testing.T) {
	_, err := parseExpenseImportRows(" ")
	require.ErrorContains(t, err, "csv_content is required")

	_, err = parseExpenseImportRows(`"unterminated`)
	require.ErrorContains(t, err, "parse csv header")

	rows, err := parseExpenseImportRows("expense_date,merchant,expense_account_id,payment_account_id,amount,\n,,,,,ignored\n")
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestExpenseImportWave8RowErrorBranches(t *testing.T) {
	expenseAccountID := "99999999-9999-4999-8999-999999999999"
	paymentAccountID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	service := NewServiceWithRepository(newMemoryRepository(), newFakeAccountingPoster(), &fakeEvidenceEvaluator{compliant: true})
	service.now = fixedExpenseNow

	result, err := service.ImportExpensesCSV(context.Background(), "tenant_acme", "tenant-1", &ImportExpensesRequest{
		UserID: "user-1",
		CSVContent: "expense_number,expense_date,merchant,expense_account_id,payment_account_id,amount,status,submitted_at,approved_at\n" +
			"EXP-BAD-DATE,2026/05/31,Bad Date," + expenseAccountID + "," + paymentAccountID + ",10,DRAFT,,\n" +
			"EXP-BAD-AMOUNT,2026-05-31,Bad Amount," + expenseAccountID + "," + paymentAccountID + ",many,DRAFT,,\n" +
			"EXP-BAD-SUBMITTED,2026-05-31,Bad Submitted," + expenseAccountID + "," + paymentAccountID + ",10,SUBMITTED,not-a-date,\n" +
			"EXP-BAD-APPROVED,2026-05-31,Bad Approved," + expenseAccountID + "," + paymentAccountID + ",10,APPROVED,,not-a-date\n",
	})

	require.NoError(t, err)
	assert.Zero(t, result.ExpensesCreated)
	assert.Equal(t, 4, result.RowsSkipped)
	require.Len(t, result.Errors, 4)
	assert.Contains(t, result.Errors[0].Message, "invalid expense_date")
	assert.Contains(t, result.Errors[1].Message, "invalid amount")
	assert.Contains(t, result.Errors[2].Message, "submitted_at")
	assert.Contains(t, result.Errors[3].Message, "approved_at")

	result, err = service.ImportExpensesCSV(context.Background(), "tenant_acme", "tenant-1", &ImportExpensesRequest{
		UserID: "user-1",
		CSVContent: "expense_number,expense_date,merchant,expense_account_id,payment_account_id,amount,status,submitted_at,rejection_reason\n" +
			"EXP-BAD-APPROVED-SUBMITTED,2026-05-31,Bad Approved Submitted," + expenseAccountID + "," + paymentAccountID + ",10,APPROVED,not-a-date,\n" +
			"EXP-BAD-REJECTED-SUBMITTED,2026-05-31,Bad Rejected Submitted," + expenseAccountID + "," + paymentAccountID + ",10,REJECTED,not-a-date,Missing receipt\n",
	})

	require.NoError(t, err)
	assert.Zero(t, result.ExpensesCreated)
	assert.Equal(t, 2, result.RowsSkipped)
	require.Len(t, result.Errors, 2)
	assert.Contains(t, result.Errors[0].Message, "submitted_at")
	assert.Contains(t, result.Errors[1].Message, "submitted_at")
}
