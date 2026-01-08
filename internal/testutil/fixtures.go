package testutil

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestAccounts holds account IDs from the default chart of accounts
// for use in integration tests that require valid GL account references.
type TestAccounts struct {
	AssetAccountID                string // Code 1500 - Fixed Assets
	DepreciationExpenseAccountID  string // Code 5600 - Depreciation Expense
	AccumulatedDepreciationAcctID string // Code 1600 - Accumulated Depreciation
}

// GetTestAccounts retrieves account IDs from the default chart of accounts.
// These accounts are created automatically when a tenant schema is initialized
// via create_default_chart_of_accounts in the migration.
func GetTestAccounts(t *testing.T, pool *pgxpool.Pool, schemaName string) *TestAccounts {
	t.Helper()
	ctx := context.Background()
	accounts := &TestAccounts{}

	// Query Fixed Assets account (code 1500)
	err := pool.QueryRow(ctx,
		fmt.Sprintf(`SELECT id FROM %s.accounts WHERE code = '1500'`, schemaName),
	).Scan(&accounts.AssetAccountID)
	if err != nil {
		t.Fatalf("Failed to get Fixed Assets account (1500): %v", err)
	}

	// Query Depreciation Expense account (code 5600)
	err = pool.QueryRow(ctx,
		fmt.Sprintf(`SELECT id FROM %s.accounts WHERE code = '5600'`, schemaName),
	).Scan(&accounts.DepreciationExpenseAccountID)
	if err != nil {
		t.Fatalf("Failed to get Depreciation Expense account (5600): %v", err)
	}

	// Query Accumulated Depreciation account (code 1600)
	err = pool.QueryRow(ctx,
		fmt.Sprintf(`SELECT id FROM %s.accounts WHERE code = '1600'`, schemaName),
	).Scan(&accounts.AccumulatedDepreciationAcctID)
	if err != nil {
		t.Fatalf("Failed to get Accumulated Depreciation account (1600): %v", err)
	}

	return accounts
}
