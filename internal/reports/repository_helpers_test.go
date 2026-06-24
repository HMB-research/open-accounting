package reports

import (
	"encoding/json"
	"testing"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReportRepositoryHelperStatusesAndTableQualification(t *testing.T) {
	assert.Equal(t, []string{
		string(models.InvoiceStatusSent),
		string(models.InvoiceStatusPartiallyPaid),
		string(models.InvoiceStatusOverdue),
	}, balanceConfirmationInvoiceStatuses())

	assert.Equal(t, []string{
		string(models.InvoiceStatusSent),
		string(models.InvoiceStatusPartiallyPaid),
		string(models.InvoiceStatusPaid),
		string(models.InvoiceStatusOverdue),
	}, contactStatementInvoiceStatuses())

	qualified, err := qualifiedTenantTable("tenant_demo", "invoices")
	require.NoError(t, err)
	assert.Equal(t, `"tenant_demo"."invoices"`, qualified)

	_, err = qualifiedTenantTable("tenant-demo", "invoices")
	require.ErrorContains(t, err, "invalid SQL identifier")
}

func TestCashFlowMappingSettingsHelpers(t *testing.T) {
	emptyMap, err := settingsMapFromRaw(nil)
	require.NoError(t, err)
	assert.Empty(t, emptyMap)

	emptyMap, err = settingsMapFromRaw(json.RawMessage("null"))
	require.NoError(t, err)
	assert.Empty(t, emptyMap)

	_, err = settingsMapFromRaw(json.RawMessage("{"))
	require.ErrorContains(t, err, "parse tenant settings")

	mapping, err := cashFlowMappingFromSettings(json.RawMessage(`{"company_name":"Demo","cash_flow_mapping":null}`))
	require.NoError(t, err)
	assert.Empty(t, mapping.OperatingAccountCodes)

	mapping, err = cashFlowMappingFromSettings(json.RawMessage(`{"company_name":"Demo"}`))
	require.NoError(t, err)
	assert.Empty(t, mapping.OperatingAccountCodes)

	mapping, err = cashFlowMappingFromSettings(json.RawMessage(`{
		"cash_flow_mapping": {
			"operating_account_codes": ["4000"],
			"investing_account_codes": ["1200"],
			"financing_account_codes": ["3000"]
		}
	}`))
	require.NoError(t, err)
	assert.Equal(t, []string{"4000"}, mapping.OperatingAccountCodes)
	assert.Equal(t, []string{"1200"}, mapping.InvestingAccountCodes)
	assert.Equal(t, []string{"3000"}, mapping.FinancingAccountCodes)

	_, err = cashFlowMappingFromSettings(json.RawMessage(`{"cash_flow_mapping": "not an object"}`))
	require.ErrorContains(t, err, "parse cash flow mapping")

	updated, err := settingsWithCashFlowMapping(json.RawMessage(`{"company_name":"Demo"}`), CashFlowMappingOverrides{
		OperatingAccountCodes: []string{"4000"},
	})
	require.NoError(t, err)

	var updatedMap map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(updated, &updatedMap))
	assert.JSONEq(t, `{"operating_account_codes":["4000"]}`, string(updatedMap["cash_flow_mapping"]))
	assert.JSONEq(t, `"Demo"`, string(updatedMap["company_name"]))

	_, err = settingsWithCashFlowMapping(json.RawMessage("{"), CashFlowMappingOverrides{})
	require.ErrorContains(t, err, "parse tenant settings")
}

func TestSortContactStatementEntries(t *testing.T) {
	entries := []ContactStatementEntry{
		{Date: "2026-01-10", DocumentType: "PAYMENT", DocumentNumber: "PMT-2", DocumentID: "pay-2"},
		{Date: "2026-01-10", DocumentType: "INVOICE", DocumentNumber: "INV-2", DocumentID: "inv-2"},
		{Date: "2026-01-09", DocumentType: "PAYMENT", DocumentNumber: "PMT-1", DocumentID: "pay-1"},
		{Date: "2026-01-10", DocumentType: "INVOICE", DocumentNumber: "INV-1", DocumentID: "inv-3"},
		{Date: "2026-01-10", DocumentType: "INVOICE", DocumentNumber: "INV-1", DocumentID: "inv-1"},
	}

	sortContactStatementEntries(entries)

	assert.Equal(t, []ContactStatementEntry{
		{Date: "2026-01-09", DocumentType: "PAYMENT", DocumentNumber: "PMT-1", DocumentID: "pay-1"},
		{Date: "2026-01-10", DocumentType: "INVOICE", DocumentNumber: "INV-1", DocumentID: "inv-1"},
		{Date: "2026-01-10", DocumentType: "INVOICE", DocumentNumber: "INV-1", DocumentID: "inv-3"},
		{Date: "2026-01-10", DocumentType: "INVOICE", DocumentNumber: "INV-2", DocumentID: "inv-2"},
		{Date: "2026-01-10", DocumentType: "PAYMENT", DocumentNumber: "PMT-2", DocumentID: "pay-2"},
	}, entries)

	assert.Equal(t, 1, documentSortOrder("INVOICE"))
	assert.Equal(t, 2, documentSortOrder("PAYMENT"))
	assert.Equal(t, 2, documentSortOrder("CREDIT_NOTE"))
}

func TestFirstNonEmpty(t *testing.T) {
	assert.Equal(t, "fallback", firstNonEmpty("", "fallback", "last"))
	assert.Equal(t, "", firstNonEmpty("", ""))
}
