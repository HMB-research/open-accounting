package cutover

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
)

// CanonicalizeBundleFileCSV rewrites provider-specific CSV headers to the same
// canonical headers used by migration preflight validation.
func CanonicalizeBundleFileCSV(file BundleFile, providerPreset MigrationProviderPreset) (BundleFile, error) {
	preset, err := normalizeMigrationProviderPreset(providerPreset)
	if err != nil {
		return file, err
	}
	if preset == MigrationProviderPresetGeneric || file.Kind == KindEInvoices || strings.TrimSpace(file.CSVContent) == "" {
		return file, nil
	}
	if !isSupportedBundleKind(file.Kind) {
		return file, fmt.Errorf("unsupported migration file kind %q", file.Kind)
	}
	if preset == MigrationProviderPresetSmartAccounts {
		hintedKind, hasHint := smartAccountsFilenameKind(file.FileName)
		if !hasHint {
			hintedKind = file.Kind
			hasHint = true
		}
		headers, rows, err := readSmartAccountsCSVWithHint(file.CSVContent, hintedKind, hasHint)
		if err != nil {
			return file, err
		}
		spec := fileSpecForProviderPreset(file.Kind, preset)
		for i, header := range headers {
			headers[i] = canonicalHeader(spec.aliases, header)
		}
		headers, rows, _ = normalizeSmartAccountsSourceRows(file.Kind, file.FileName, headers, rows)
		file.CSVContent = writeCSVContent(headers, rows)
		return file, nil
	}

	canonicalContent, err := canonicalizeCSVHeaders(file.CSVContent, fileSpecForProviderPreset(file.Kind, preset))
	if err != nil {
		return file, err
	}
	file.CSVContent = canonicalContent
	return file, nil
}

func canonicalizeCSVHeaders(content string, spec fileSpec) (string, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(content), "\ufeff")
	if trimmed == "" {
		return "", fmt.Errorf("csv_content is required")
	}

	reader := csv.NewReader(strings.NewReader(trimmed))
	reader.Comma = detectDelimiter(trimmed)
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	headers, err := reader.Read()
	if err != nil {
		return "", fmt.Errorf("parse csv header: %w", err)
	}

	for i, header := range headers {
		headers[i] = canonicalHeader(spec.aliases, header)
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	_ = writer.Write(headers)
	for {
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", fmt.Errorf("parse csv row: %w", err)
		}
		_ = writer.Write(record)
	}
	writer.Flush()
	return buf.String(), nil
}
