package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/expenses"
	"github.com/HMB-research/open-accounting/internal/inventory"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/payments"
)

func TestCLIJSONInputParserBranches(t *testing.T) {
	settings, err := parseTenantSettingsInput("", "")
	require.NoError(t, err)
	assert.Nil(t, settings)

	settings, err = parseTenantSettingsInput(`{"default_currency":"EUR","country_code":"EE","fiscal_year_start_month":1}`, "")
	require.NoError(t, err)
	require.NotNil(t, settings)
	assert.Equal(t, "EUR", settings.DefaultCurrency)

	settingsPath := filepath.Join(t.TempDir(), "tenant-settings.json")
	require.NoError(t, os.WriteFile(settingsPath, []byte(`{"default_currency":"USD","timezone":"UTC"}`), 0o600))
	settings, err = parseTenantSettingsInput("", settingsPath)
	require.NoError(t, err)
	require.NotNil(t, settings)
	assert.Equal(t, "USD", settings.DefaultCurrency)

	_, err = parseTenantSettingsInput(`{"default_currency":"EUR"}`, settingsPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "use either settings-json or settings-file")

	_, err = parseTenantSettingsInput("", filepath.Join(t.TempDir(), "missing.json"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read settings file")

	_, err = parseTenantSettingsInput("{bad json", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse tenant settings JSON")

	raw, err := parseRawJSONInput("", "", "")
	require.NoError(t, err)
	assert.Nil(t, raw)

	raw, err = parseRawJSONInput("", "", `{"enabled":true}`)
	require.NoError(t, err)
	assert.JSONEq(t, `{"enabled":true}`, string(raw))

	raw, err = parseRawJSONInput(`{"threshold":5}`, "", "")
	require.NoError(t, err)
	assert.JSONEq(t, `{"threshold":5}`, string(raw))

	rawPath := filepath.Join(t.TempDir(), "settings.json")
	require.NoError(t, os.WriteFile(rawPath, []byte(`{"mode":"strict"}`), 0o600))
	raw, err = parseRawJSONInput("", rawPath, "")
	require.NoError(t, err)
	assert.JSONEq(t, `{"mode":"strict"}`, string(raw))

	_, err = parseRawJSONInput(`{"mode":"inline"}`, rawPath, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "use either settings-json or settings-file")

	_, err = parseRawJSONInput("", filepath.Join(t.TempDir(), "missing.json"), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read settings file")

	_, err = parseRawJSONInput("{bad json", "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse JSON")
}

func TestCLIScalarParserBranches(t *testing.T) {
	date, err := parseRequiredDate("issued-date", "2026-06-01")
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), date)

	_, err = parseRequiredDate("issued-date", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "issued-date is required")

	_, err = parseRequiredDate("issued-date", "2026-99-99")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse issued-date")

	positiveInt, err := parseRequiredPositiveInt("limit", " 5 ")
	require.NoError(t, err)
	assert.Equal(t, 5, positiveInt)

	for _, value := range []string{"", "bad", "0"} {
		_, err = parseRequiredPositiveInt("limit", value)
		require.Error(t, err)
	}

	nonNegativeInt, err := parseRequiredNonNegativeInt("offset", "0")
	require.NoError(t, err)
	assert.Equal(t, 0, nonNegativeInt)

	for _, value := range []string{"", "bad", "-1"} {
		_, err = parseRequiredNonNegativeInt("offset", value)
		require.Error(t, err)
	}

	amount, err := parseRequiredPositiveDecimal("amount", "12.50")
	require.NoError(t, err)
	assert.True(t, amount.Equal(decimal.RequireFromString("12.50")))

	for _, value := range []string{"", "bad", "0"} {
		_, err = parseRequiredPositiveDecimal("amount", value)
		require.Error(t, err)
	}

	amount, err = parseRequiredNonNegativeDecimal("amount", "0")
	require.NoError(t, err)
	assert.True(t, amount.Equal(decimal.Zero))

	for _, value := range []string{"", "bad", "-0.01"} {
		_, err = parseRequiredNonNegativeDecimal("amount", value)
		require.Error(t, err)
	}

	confidence, err := parseBankMatchRuleConfidence("0.75")
	require.NoError(t, err)
	assert.Equal(t, 0.75, confidence)

	for _, value := range []string{"bad", "-0.1", "1.1"} {
		_, err = parseBankMatchRuleConfidence(value)
		require.Error(t, err)
	}
}

func TestCLIEnumParserOptionalBranches(t *testing.T) {
	invoiceType, err := parseOptionalInvoiceType("sales")
	require.NoError(t, err)
	assert.Equal(t, invoicing.InvoiceTypeSales, invoiceType)

	for _, value := range []string{"", "bad"} {
		_, err = parseOptionalInvoiceType(value)
		if value == "" {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
	}

	invoiceStatus, err := parseOptionalInvoiceStatus("paid")
	require.NoError(t, err)
	assert.Equal(t, invoicing.StatusPaid, invoiceStatus)

	for _, value := range []string{"", "bad"} {
		_, err = parseOptionalInvoiceStatus(value)
		if value == "" {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
	}

	expenseStatus, err := parseOptionalExpenseStatus("approved")
	require.NoError(t, err)
	assert.Equal(t, expenses.StatusApproved, expenseStatus)

	for _, value := range []string{"", "bad"} {
		_, err = parseOptionalExpenseStatus(value)
		if value == "" {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
	}

	productType, err := parseOptionalProductType("goods")
	require.NoError(t, err)
	assert.Equal(t, inventory.ProductTypeGoods, productType)

	for _, value := range []string{"", "bad"} {
		_, err = parseOptionalProductType(value)
		if value == "" {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
	}

	frequency, err := parseOptionalJournalTemplateFrequency("monthly")
	require.NoError(t, err)
	assert.Equal(t, accounting.JournalEntryTemplateFrequencyMonthly, frequency)

	for _, value := range []string{"", "bad"} {
		_, err = parseOptionalJournalTemplateFrequency(value)
		if value == "" {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
	}

	paymentType, err := parseOptionalPaymentType("received")
	require.NoError(t, err)
	assert.Equal(t, payments.PaymentTypeReceived, paymentType)

	for _, value := range []string{"", "bad"} {
		_, err = parseOptionalPaymentType(value)
		if value == "" {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
	}
}

func TestCLIFormattingHelperBranches(t *testing.T) {
	assert.Nil(t, optionalUpperStringPtr(""))
	upper := optionalUpperStringPtr(" eur ")
	require.NotNil(t, upper)
	assert.Equal(t, "EUR", *upper)

	assert.Equal(t, "Sent", quoteActionPastTense("send"))
	assert.Equal(t, "Accepted", quoteActionPastTense("accept"))
	assert.Equal(t, "Rejected", quoteActionPastTense("reject"))
	assert.Equal(t, "Archive", quoteActionPastTense("archive"))

	for action, expected := range map[string]string{
		"confirm": "Confirmed",
		"process": "Processed",
		"ship":    "Shipped",
		"deliver": "Delivered",
		"cancel":  "Canceled",
		"archive": "Archive",
	} {
		assert.Equal(t, expected, orderActionPastTense(action))
	}

	for action, expected := range map[string]string{
		"submit":  "Submitted",
		"approve": "Approved",
		"post":    "Posted",
		"archive": "Archive",
	} {
		assert.Equal(t, expected, expenseActionPastTense(action))
	}

	var invoiceLines *invoiceLineFlags
	assert.Equal(t, "", invoiceLines.String())

	var orderLines *orderLineFlags
	assert.Equal(t, "", orderLines.String())

	treatment, err := parseInvoiceLineVATTreatment("", "reverse charge", "false")
	require.NoError(t, err)
	assert.Equal(t, invoicing.VATTreatmentReverseCharge, treatment)

	treatment, err = parseInvoiceLineVATTreatment("standard", "", "")
	require.NoError(t, err)
	assert.Equal(t, invoicing.VATTreatmentStandard, treatment)

	_, err = parseInvoiceLineVATTreatment("", "", "not-bool")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse reverse_charge")

	_, err = parseInvoiceLineVATTreatment("bad-treatment", "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid line vat_treatment")
}
