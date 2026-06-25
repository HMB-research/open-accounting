package expenses

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type expensesWave11BlankNumberRepo struct {
	*memoryRepository
}

func (r *expensesWave11BlankNumberRepo) GenerateNumber(context.Context, string, string) (string, error) {
	return "\t\n", nil
}

func TestExpensesWave11ImportReportsBlankGeneratedNumber(t *testing.T) {
	repo := &expensesWave11BlankNumberRepo{memoryRepository: newMemoryRepository()}
	service := NewServiceWithRepository(repo, newFakeAccountingPoster(), nil)

	result, err := service.ImportExpensesCSV(context.Background(), "tenant_acme", "tenant-1", &ImportExpensesRequest{
		UserID: "user-1",
		CSVContent: "expense_date,merchant,expense_account_code,payment_account_code,amount\n" +
			"2026-06-01,Vendor,5500,1000,12.50\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Zero(t, result.ExpensesCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "expense_number is required")
}
