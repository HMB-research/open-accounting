package inventory

import (
	"context"
	"testing"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/stretchr/testify/require"
)

func TestInventoryWave12IssueStockRequiresLedgerAfterAccountingLines(t *testing.T) {
	repo := inventoryWave9StockFixture()
	service := NewServiceWithRepositoryAndAccounting(repo, fakeInventoryAccountLister{accounts: []accounting.Account{
		{ID: "11111111-1111-4111-8111-111111111111", AccountType: accounting.AccountTypeExpense},
		{ID: "22222222-2222-4222-8222-222222222222", AccountType: accounting.AccountTypeAsset},
	}})

	_, err := service.issueStock(context.Background(), "tenant-1", "tenant_demo", inventoryWave9IssueRequest(true))

	require.ErrorContains(t, err, "accounting service is unavailable")
}
