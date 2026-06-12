package mappers

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Format identifies a bank statement import layout.
type Format string

const (
	FormatAuto    Format = "auto"
	FormatGeneric Format = "generic"
	FormatLHV     Format = "lhv"
	FormatCAMT053 Format = "camt053"
	FormatLHVCAMT Format = "lhv-camt"
)

// ParsedCSV contains a normalized CSV header and data rows.
type ParsedCSV struct {
	Headers []string
	Rows    [][]string
	Index   map[string]int
}

// ParseCSV reads delimited CSV content with a header row.
func ParseCSV(content, label string) (*ParsedCSV, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil, fmt.Errorf("%s CSV is empty", label)
	}

	reader := csv.NewReader(strings.NewReader(trimmed))
	reader.Comma = DetectDelimiter(trimmed)
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1

	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read %s CSV header: %w", label, err)
	}

	index := BuildHeaderIndex(headers)
	var rows [][]string
	for rowNum := 2; ; rowNum++ {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read %s CSV row %d: %w", label, rowNum, err)
		}
		if IsEmptyRecord(record) {
			continue
		}
		rows = append(rows, record)
	}

	return &ParsedCSV{Headers: headers, Rows: rows, Index: index}, nil
}

// DetectDelimiter chooses comma, semicolon, or tab from the header line.
func DetectDelimiter(content string) rune {
	firstLine := content
	if idx := strings.IndexAny(content, "\r\n"); idx >= 0 {
		firstLine = content[:idx]
	}
	delimiters := []rune{',', ';', '\t'}
	bestDelimiter := ','
	bestCount := -1
	for _, delimiter := range delimiters {
		count := strings.Count(firstLine, string(delimiter))
		if count > bestCount {
			bestCount = count
			bestDelimiter = delimiter
		}
	}
	return bestDelimiter
}

// BuildHeaderIndex returns a lookup map using normalized header labels.
func BuildHeaderIndex(headers []string) map[string]int {
	index := make(map[string]int, len(headers))
	for i, header := range headers {
		index[NormalizeHeader(header)] = i
	}
	return index
}

// NormalizeHeader makes bank-export headers comparable across languages.
func NormalizeHeader(header string) string {
	key := strings.ToLower(strings.TrimSpace(header))
	replacements := map[string]string{
		"’": "",
		"'": "",
		"`": "",
		"´": "",
		"(": "",
		")": "",
		"/": "_",
		"-": "_",
		" ": "_",
		".": "",
	}
	for old, replacement := range replacements {
		key = strings.ReplaceAll(key, old, replacement)
	}
	for strings.Contains(key, "__") {
		key = strings.ReplaceAll(key, "__", "_")
	}
	return strings.Trim(key, "_")
}

// Field returns the first matching field from a record by normalized header name.
func Field(record []string, index map[string]int, names ...string) string {
	for _, name := range names {
		if i, ok := index[NormalizeHeader(name)]; ok && i < len(record) {
			return strings.TrimSpace(record[i])
		}
	}
	return ""
}

// HasAnyHeader reports whether any alias exists in the parsed header.
func HasAnyHeader(index map[string]int, names ...string) bool {
	for _, name := range names {
		if _, ok := index[NormalizeHeader(name)]; ok {
			return true
		}
	}
	return false
}

// IsEmptyRecord reports whether a CSV record contains only blank fields.
func IsEmptyRecord(record []string) bool {
	for _, field := range record {
		if strings.TrimSpace(field) != "" {
			return false
		}
	}
	return true
}
