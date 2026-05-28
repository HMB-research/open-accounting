package main

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/reports"
)

func trialBalanceXLSX(report *accounting.TrialBalance) ([]byte, error) {
	content, err := trialBalanceCSV(report)
	if err != nil {
		return nil, err
	}
	return reportCSVBytesToXLSX("Trial Balance", content)
}

func balanceSheetXLSX(report *accounting.BalanceSheet) ([]byte, error) {
	content, err := balanceSheetCSV(report)
	if err != nil {
		return nil, err
	}
	return reportCSVBytesToXLSX("Balance Sheet", content)
}

func incomeStatementXLSX(report *accounting.IncomeStatement) ([]byte, error) {
	content, err := incomeStatementCSV(report)
	if err != nil {
		return nil, err
	}
	return reportCSVBytesToXLSX("Income Statement", content)
}

func cashFlowStatementXLSX(report *reports.CashFlowStatement) ([]byte, error) {
	content, err := cashFlowStatementCSV(report)
	if err != nil {
		return nil, err
	}
	return reportCSVBytesToXLSX("Cash Flow", content)
}

func accountBalanceXLSX(accountID, asOfDate, balance string) ([]byte, error) {
	content, err := accountBalanceCSV(accountID, asOfDate, balance)
	if err != nil {
		return nil, err
	}
	return reportCSVBytesToXLSX("Account Balance", content)
}

func reportCSVBytesToXLSX(sheetName string, content []byte) ([]byte, error) {
	rows, err := csv.NewReader(bytes.NewReader(content)).ReadAll()
	if err != nil {
		return nil, err
	}
	return reportRowsXLSX(sheetName, rows)
}

func reportRowsXLSX(sheetName string, rows [][]string) ([]byte, error) {
	buffer := &bytes.Buffer{}
	writer := zip.NewWriter(buffer)

	files := map[string]string{
		"[Content_Types].xml":        xlsxContentTypesXML(),
		"_rels/.rels":                xlsxRootRelsXML(),
		"docProps/app.xml":           xlsxAppXML(),
		"docProps/core.xml":          xlsxCoreXML(),
		"xl/_rels/workbook.xml.rels": xlsxWorkbookRelsXML(),
		"xl/styles.xml":              xlsxStylesXML(),
		"xl/workbook.xml":            xlsxWorkbookXML(sheetName),
		"xl/worksheets/sheet1.xml":   xlsxWorksheetXML(rows),
	}

	for _, name := range []string{
		"[Content_Types].xml",
		"_rels/.rels",
		"docProps/app.xml",
		"docProps/core.xml",
		"xl/_rels/workbook.xml.rels",
		"xl/styles.xml",
		"xl/workbook.xml",
		"xl/worksheets/sheet1.xml",
	} {
		if err := writeXLSXPart(writer, name, files[name]); err != nil {
			return nil, err
		}
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func writeXLSXPart(writer *zip.Writer, name, content string) error {
	part, err := writer.Create(name)
	if err != nil {
		return err
	}
	_, err = part.Write([]byte(content))
	return err
}

func xlsxWorksheetXML(rows [][]string) string {
	var builder strings.Builder
	builder.WriteString(xmlHeader())
	builder.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)

	var headers []string
	if len(rows) > 0 {
		headers = rows[0]
	}
	for rowIndex, row := range rows {
		excelRow := rowIndex + 1
		fmt.Fprintf(&builder, `<row r="%d">`, excelRow)
		for colIndex, value := range row {
			header := ""
			if colIndex < len(headers) {
				header = headers[colIndex]
			}
			builder.WriteString(xlsxCellXML(excelRow, colIndex+1, header, value))
		}
		builder.WriteString(`</row>`)
	}

	builder.WriteString(`</sheetData></worksheet>`)
	return builder.String()
}

func xlsxCellXML(row, column int, header, value string) string {
	ref := xlsxCellRef(row, column)
	if value == "" {
		return fmt.Sprintf(`<c r="%s" t="inlineStr"><is><t></t></is></c>`, ref)
	}
	if isBoolReportColumn(header) && (value == "true" || value == "false") {
		boolValue := "0"
		if value == "true" {
			boolValue = "1"
		}
		return fmt.Sprintf(`<c r="%s" t="b"><v>%s</v></c>`, ref, boolValue)
	}
	if isNumericReportColumn(header) {
		if parsed, err := decimal.NewFromString(value); err == nil {
			return fmt.Sprintf(`<c r="%s"><v>%s</v></c>`, ref, parsed.String())
		}
	}
	return fmt.Sprintf(`<c r="%s" t="inlineStr"><is><t>%s</t></is></c>`, ref, xlsxEscape(value))
}

func xlsxCellRef(row, column int) string {
	letters := ""
	for column > 0 {
		column--
		letters = string(rune('A'+column%26)) + letters
		column /= 26
	}
	return fmt.Sprintf("%s%d", letters, row)
}

func isNumericReportColumn(header string) bool {
	switch strings.ToLower(strings.TrimSpace(header)) {
	case "amount", "amount_paid", "balance", "budget_amount", "budget_used_percentage", "contact_count", "contact_invoice_count", "count", "credit_balance", "current", "days_1_30", "days_31_60", "days_61_90", "days_90_plus", "days_overdue", "debit_balance", "invoice_count", "net_balance", "outstanding_amount", "total", "total_amount", "total_balance", "total_expenses":
		return true
	default:
		return false
	}
}

func isBoolReportColumn(header string) bool {
	switch strings.ToLower(strings.TrimSpace(header)) {
	case "is_over_budget", "is_subtotal":
		return true
	default:
		return false
	}
}

func xlsxEscape(value string) string {
	var buffer bytes.Buffer
	_ = xml.EscapeText(&buffer, []byte(value))
	return buffer.String()
}

func safeSheetName(sheetName string) string {
	value := strings.TrimSpace(sheetName)
	if value == "" {
		return "Report"
	}
	replacer := strings.NewReplacer(`"`, "", "*", "", "?", "", ":", "", "/", "", "\\", "", "[", "", "]", "")
	value = replacer.Replace(value)
	if len(value) > 31 {
		value = value[:31]
	}
	return value
}

func xmlHeader() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`
}

func xlsxContentTypesXML() string {
	return xmlHeader() + `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/><Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`
}

func xlsxRootRelsXML() string {
	return xmlHeader() + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/><Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/></Relationships>`
}

func xlsxWorkbookXML(sheetName string) string {
	return xmlHeader() + fmt.Sprintf(`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="%s" sheetId="1" r:id="rId1"/></sheets></workbook>`, xlsxEscape(safeSheetName(sheetName)))
}

func xlsxWorkbookRelsXML() string {
	return xmlHeader() + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`
}

func xlsxStylesXML() string {
	return xmlHeader() + `<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><fonts count="1"><font><sz val="11"/><name val="Calibri"/></font></fonts><fills count="1"><fill><patternFill patternType="none"/></fill></fills><borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders><cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs><cellXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/></cellXfs></styleSheet>`
}

func xlsxAppXML() string {
	return xmlHeader() + `<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties" xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes"><Application>Open Accounting</Application></Properties>`
}

func xlsxCoreXML() string {
	return xmlHeader() + `<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:dcmitype="http://purl.org/dc/dcmitype/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><dc:creator>Open Accounting</dc:creator><cp:lastModifiedBy>Open Accounting</cp:lastModifiedBy></cp:coreProperties>`
}
