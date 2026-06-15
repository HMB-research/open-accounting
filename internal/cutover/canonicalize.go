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
		if err == io.EOF {
			return "", fmt.Errorf("csv file is empty")
		}
		return "", fmt.Errorf("parse csv header: %w", err)
	}

	for i, header := range headers {
		headers[i] = canonicalHeader(spec.aliases, header)
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	if err := writer.Write(headers); err != nil {
		return "", fmt.Errorf("write csv header: %w", err)
	}
	for {
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", fmt.Errorf("parse csv row: %w", err)
		}
		if err := writer.Write(record); err != nil {
			return "", fmt.Errorf("write csv row: %w", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", fmt.Errorf("write csv: %w", err)
	}
	return buf.String(), nil
}
