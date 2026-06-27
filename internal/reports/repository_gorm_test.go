package reports

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
	schemaName := "tenant_demo"
	tenantID := "tenant-1"
	contactID := "contact-1"
	startDate := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC)

	require.NotNil(t, repo)
	assert.Nil(t, repo.db)

	tests := []struct {
		name string
		run  func(t *testing.T) error
	}{
		{
			name: "dbWithContext",
			run: func(t *testing.T) error {
				db, err := repo.dbWithContext(ctx)
				assert.Nil(t, db)
				return err
			},
		},
		{
			name: "tenantTable",
			run: func(t *testing.T) error {
				db, err := repo.tenantTable(ctx, schemaName, "invoices", "i")
				assert.Nil(t, db)
				return err
			},
		},
		{
			name: "GetJournalEntriesForPeriod",
			run: func(t *testing.T) error {
				entries, err := repo.GetJournalEntriesForPeriod(ctx, schemaName, tenantID, startDate, endDate)
				assert.Nil(t, entries)
				return err
			},
		},
		{
			name: "GetCashAccountBalance",
			run: func(t *testing.T) error {
				balance, err := repo.GetCashAccountBalance(ctx, schemaName, tenantID, endDate)
				assert.True(t, balance.Equal(decimal.Zero))
				return err
			},
		},
		{
			name: "GetOutstandingInvoicesByContact",
			run: func(t *testing.T) error {
				balances, err := repo.GetOutstandingInvoicesByContact(ctx, schemaName, tenantID, "SALES", endDate)
				assert.Nil(t, balances)
				return err
			},
		},
		{
			name: "GetContactInvoices",
			run: func(t *testing.T) error {
				invoices, err := repo.GetContactInvoices(ctx, schemaName, tenantID, contactID, "SALES", endDate)
				assert.Nil(t, invoices)
				return err
			},
		},
		{
			name: "GetContact",
			run: func(t *testing.T) error {
				contact, err := repo.GetContact(ctx, schemaName, tenantID, contactID)
				assert.Empty(t, contact)
				return err
			},
		},
		{
			name: "GetContactStatementOpeningBalance",
			run: func(t *testing.T) error {
				balance, err := repo.GetContactStatementOpeningBalance(ctx, schemaName, tenantID, contactID, "SALES", "RECEIVED", startDate)
				assert.True(t, balance.Equal(decimal.Zero))
				return err
			},
		},
		{
			name: "sumInvoiceStatementAmountBefore",
			run: func(t *testing.T) error {
				balance, err := repo.sumInvoiceStatementAmountBefore(ctx, schemaName, tenantID, contactID, "SALES", startDate)
				assert.True(t, balance.Equal(decimal.Zero))
				return err
			},
		},
		{
			name: "sumPaymentStatementAmountBefore",
			run: func(t *testing.T) error {
				balance, err := repo.sumPaymentStatementAmountBefore(ctx, schemaName, tenantID, contactID, "RECEIVED", startDate)
				assert.True(t, balance.Equal(decimal.Zero))
				return err
			},
		},
		{
			name: "GetContactStatementEntries",
			run: func(t *testing.T) error {
				entries, err := repo.GetContactStatementEntries(ctx, schemaName, tenantID, contactID, "SALES", "RECEIVED", startDate, endDate)
				assert.Nil(t, entries)
				return err
			},
		},
		{
			name: "getContactStatementInvoiceEntries",
			run: func(t *testing.T) error {
				entries, err := repo.getContactStatementInvoiceEntries(ctx, schemaName, tenantID, contactID, "SALES", startDate, endDate)
				assert.Nil(t, entries)
				return err
			},
		},
		{
			name: "getContactStatementPaymentEntries",
			run: func(t *testing.T) error {
				entries, err := repo.getContactStatementPaymentEntries(ctx, schemaName, tenantID, contactID, "RECEIVED", startDate, endDate)
				assert.Nil(t, entries)
				return err
			},
		},
		{
			name: "GetSalesMarginLines",
			run: func(t *testing.T) error {
				lines, err := repo.GetSalesMarginLines(ctx, schemaName, tenantID, startDate, endDate)
				assert.Nil(t, lines)
				return err
			},
		},
		{
			name: "GetCashFlowMappingOverrides",
			run: func(t *testing.T) error {
				mapping, err := repo.GetCashFlowMappingOverrides(ctx, tenantID)
				assert.Empty(t, mapping)
				return err
			},
		},
		{
			name: "UpdateCashFlowMappingOverrides",
			run: func(t *testing.T) error {
				mapping, err := repo.UpdateCashFlowMappingOverrides(ctx, tenantID, CashFlowMappingOverrides{
					OperatingAccountCodes: []string{"4000"},
				})
				assert.Empty(t, mapping)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(t)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "reports repository database is not configured")
		})
	}
}

func TestGORMRepositoryNilReceiver(t *testing.T) {
	var repo *GORMRepository

	db, err := repo.dbWithContext(context.Background())
	require.ErrorContains(t, err, "reports repository database is not configured")
	assert.Nil(t, db)
}
