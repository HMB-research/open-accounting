package reports

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGORMRepositoryWave9StatementSequentialErrors(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_reports"
	tenantID := "tenant-1"
	contactID := "contact-1"
	startDate := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, time.June, 30, 0, 0, 0, 0, time.UTC)

	t.Run("opening balance returns payment total error after invoice total succeeds", func(t *testing.T) {
		expectedErr := errors.New("payment opening scan failed")
		repo := &GORMRepository{db: newReportsDryRunDB(t,
			withReportsDryRunScanRows(
				reportsDryRunRowSet{columns: []string{"total"}, values: [][]driver.Value{{"125.00"}}},
				reportsDryRunRowSet{columns: []string{"total"}, values: [][]driver.Value{{"25.00"}}},
			),
			withReportsDryRunRowErrors(nil, expectedErr),
		)}

		balance, err := repo.GetContactStatementOpeningBalance(ctx, schemaName, tenantID, contactID, "SALES", "RECEIVED", startDate)

		assert.True(t, balance.Equal(decimal.Zero))
		require.ErrorContains(t, err, "query contact statement payment opening balance")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("statement entries returns payment query error after invoice entries succeed", func(t *testing.T) {
		expectedErr := errors.New("payment entries scan failed")
		invoiceColumns := []string{"document_id", "document_number", "document_date", "due_date", "reference", "notes", "currency", "document_amount", "statement_amount"}
		paymentColumns := []string{"document_id", "document_number", "document_date", "reference", "notes", "currency", "document_amount", "statement_amount"}
		repo := &GORMRepository{db: newReportsDryRunDB(t,
			withReportsDryRunScanRows(
				reportsDryRunRowSet{columns: invoiceColumns},
				reportsDryRunRowSet{columns: paymentColumns},
			),
			withReportsDryRunRowErrors(nil, expectedErr),
		)}

		entries, err := repo.GetContactStatementEntries(ctx, schemaName, tenantID, contactID, "SALES", "RECEIVED", startDate, endDate)

		assert.Nil(t, entries)
		require.ErrorContains(t, err, "query contact statement payments")
		assert.ErrorIs(t, err, expectedErr)
	})
}

func TestCashFlowMappingWave9InvalidTenantSettings(t *testing.T) {
	mapping, err := cashFlowMappingFromSettings(json.RawMessage(`{"cash_flow_mapping":`))

	assert.Empty(t, mapping)
	require.ErrorContains(t, err, "parse tenant settings")

	updated, err := settingsWithCashFlowMapping(json.RawMessage(`null`), CashFlowMappingOverrides{
		InvestingAccountCodes: []string{"1500"},
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{"cash_flow_mapping":{"investing_account_codes":["1500"]}}`, string(updated))
}
