package main

import (
	"archive/zip"
	"encoding/csv"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/reports"
)

func TestWave7ReportResponseFormatDefaultAndAllowedValues(t *testing.T) {
	for _, tt := range []struct {
		target string
		want   string
	}{
		{target: "/reports", want: "json"},
		{target: "/reports?format=json", want: "json"},
		{target: "/reports?format=csv", want: "csv"},
		{target: "/reports?format=pdf", want: "pdf"},
	} {
		req := httptest.NewRequest(http.MethodGet, tt.target, nil)
		got, err := reportResponseFormat(req)
		require.NoError(t, err)
		assert.Equal(t, tt.want, got)
	}
}

func TestWave7ReportCSVSectionWritersReturnImmediateErrors(t *testing.T) {
	largeField := strings.Repeat("x", 1<<20)

	writer := csv.NewWriter(erroringWriter{})
	err := writeAccountSectionCSV(writer, "assets", []accounting.AccountBalance{{
		AccountCode:  "1000",
		AccountName:  largeField,
		AccountType:  accounting.AccountTypeAsset,
		DebitBalance: decimal.NewFromInt(10),
		NetBalance:   decimal.NewFromInt(10),
	}})
	if err == nil {
		writer.Flush()
		err = writer.Error()
	}
	require.Error(t, err)

	writer = csv.NewWriter(erroringWriter{})
	err = writeCashFlowSectionCSV(writer, "operating", []reports.CashFlowItem{{
		Code:        "cash_in",
		Description: largeField,
		Amount:      decimal.NewFromInt(25),
		IsSubtotal:  true,
	}})
	if err == nil {
		writer.Flush()
		err = writer.Error()
	}
	require.Error(t, err)
}

func TestWave7ReportXLSXPartAndCellBoundaries(t *testing.T) {
	writer := zip.NewWriter(erroringWriter{})
	require.NoError(t, writeXLSXPart(writer, "part.xml", strings.Repeat("payload", 1024)))
	require.Error(t, writer.Close())

	assert.Equal(t, "ZZ9", xlsxCellRef(9, 702))
	assert.Equal(t, `<c r="A1" t="b"><v>0</v></c>`, xlsxCellXML(1, 1, "is_over_budget", "false"))
	assert.Contains(t, xlsxCellXML(1, 2, "description", `<tag>&value`), "&lt;tag&gt;&amp;value")
	assert.Contains(t, xlsxWorkbookXML(strings.Repeat("a", 40)), strings.Repeat("a", 31))
}

func TestWave7ReportPDFColumnHelpers(t *testing.T) {
	rows := [][]string{
		{"amount"},
		{"123.45", "description", "extra"},
	}
	assert.Equal(t, 3, maxReportPDFColumns(rows))
	assert.Equal(t, []string{"x", "", ""}, normalizeReportPDFRow([]string{"x"}, 3))
	assert.Equal(t, "invoice number", reportPDFHeaderLabel("invoice_number"))

	cols := reportPDFColumns(
		[]string{"123.45", strings.Repeat("description ", 8)},
		[]string{"amount", "description"},
		false,
	)
	require.Len(t, cols, 2)

	wideRows := [][]string{{
		"c1", "c2", "c3", "c4", "c5",
		"c6", "c7", "c8", "c9", "c10",
		"c11", "c12", "c13", "c14", "c15",
	}, {
		"1", "2", "3", "4", "5",
		"6", "7", "8", "9", "10",
		"11", "12", "13", "14", "15",
	}}
	pdf, err := reportRowsPDF("Wide Report", "Subtitle", wideRows)
	require.NoError(t, err)
	requirePDF(t, pdf)
}
