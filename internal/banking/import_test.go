package banking

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errorReader is a reader that returns an error after reading some data
type errorReader struct {
	data   string
	pos    int
	errAt  int
	errMsg string
}

func (e *errorReader) Read(p []byte) (n int, err error) {
	if e.pos >= e.errAt && e.errAt > 0 {
		return 0, errors.New(e.errMsg)
	}
	if e.pos >= len(e.data) {
		return 0, io.EOF
	}
	n = copy(p, e.data[e.pos:])
	e.pos += n
	return n, nil
}

func TestDefaultGenericMapping(t *testing.T) {
	mapping := DefaultGenericMapping()

	assert.Equal(t, 0, mapping.DateColumn)
	assert.Equal(t, 1, mapping.AmountColumn)
	assert.Equal(t, 2, mapping.DescriptionColumn)
	assert.Equal(t, -1, mapping.ValueDateColumn)
	assert.Equal(t, -1, mapping.ReferenceColumn)
	assert.Equal(t, "2006-01-02", mapping.DateFormat)
	assert.Equal(t, ".", mapping.DecimalSeparator)
	assert.Equal(t, ",", mapping.ThousandsSeparator)
	assert.True(t, mapping.HasHeader)
}

func TestSwedbankEEMapping(t *testing.T) {
	mapping := SwedbankEEMapping()

	assert.Equal(t, 0, mapping.DateColumn)
	assert.Equal(t, 1, mapping.ValueDateColumn)
	assert.Equal(t, 3, mapping.AmountColumn)
	assert.Equal(t, 6, mapping.DescriptionColumn)
	assert.Equal(t, 7, mapping.CounterpartyNameColumn)
	assert.Equal(t, "02.01.2006", mapping.DateFormat)
	assert.Equal(t, ",", mapping.DecimalSeparator)
	assert.Equal(t, " ", mapping.ThousandsSeparator)
}

func TestParseAmount(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		decimalSep   string
		thousandsSep string
		expected     decimal.Decimal
		expectError  bool
	}{
		{
			name:         "Simple positive",
			input:        "100.50",
			decimalSep:   ".",
			thousandsSep: ",",
			expected:     decimal.NewFromFloat(100.50),
		},
		{
			name:         "Simple negative",
			input:        "-100.50",
			decimalSep:   ".",
			thousandsSep: ",",
			expected:     decimal.NewFromFloat(-100.50),
		},
		{
			name:         "With thousands separator",
			input:        "1,234.56",
			decimalSep:   ".",
			thousandsSep: ",",
			expected:     decimal.NewFromFloat(1234.56),
		},
		{
			name:         "European format",
			input:        "1 234,56",
			decimalSep:   ",",
			thousandsSep: " ",
			expected:     decimal.NewFromFloat(1234.56),
		},
		{
			name:         "With currency symbol EUR",
			input:        "€100.50",
			decimalSep:   ".",
			thousandsSep: ",",
			expected:     decimal.NewFromFloat(100.50),
		},
		{
			name:         "With currency symbol USD",
			input:        "$1,000.00",
			decimalSep:   ".",
			thousandsSep: ",",
			expected:     decimal.NewFromFloat(1000.00),
		},
		{
			name:         "With currency symbol GBP",
			input:        "£500.00",
			decimalSep:   ".",
			thousandsSep: ",",
			expected:     decimal.NewFromFloat(500.00),
		},
		{
			name:         "Negative in parentheses",
			input:        "(100.50)",
			decimalSep:   ".",
			thousandsSep: ",",
			expected:     decimal.NewFromFloat(-100.50),
		},
		{
			name:         "With whitespace",
			input:        "  100.50  ",
			decimalSep:   ".",
			thousandsSep: ",",
			expected:     decimal.NewFromFloat(100.50),
		},
		{
			name:         "Zero",
			input:        "0.00",
			decimalSep:   ".",
			thousandsSep: ",",
			expected:     decimal.Zero,
		},
		{
			name:         "Large number",
			input:        "1,234,567.89",
			decimalSep:   ".",
			thousandsSep: ",",
			expected:     decimal.NewFromFloat(1234567.89),
		},
		{
			name:         "Invalid format",
			input:        "abc",
			decimalSep:   ".",
			thousandsSep: ",",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseAmount(tt.input, tt.decimalSep, tt.thousandsSep)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.True(t, result.Equal(tt.expected), "got %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestDetectCSVFormat(t *testing.T) {
	tests := []struct {
		name     string
		headers  []string
		expected CSVFormat
	}{
		{
			name:     "Generic headers",
			headers:  []string{"Date", "Amount", "Description"},
			expected: FormatGeneric,
		},
		{
			name:     "Swedbank EE headers",
			headers:  []string{"Kuupäev", "Value Date", "Amount", "Summa", "Reference", "Description", "Saaja/maksja nimi"},
			expected: FormatSwedbankEE,
		},
		{
			name:     "Empty headers",
			headers:  []string{},
			expected: FormatGeneric,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectCSVFormat(tt.headers)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetMappingForFormat(t *testing.T) {
	tests := []struct {
		format   CSVFormat
		expected CSVColumnMapping
	}{
		{
			format:   FormatGeneric,
			expected: DefaultGenericMapping(),
		},
		{
			format:   FormatSwedbankEE,
			expected: SwedbankEEMapping(),
		},
		{
			format:   FormatSEBEE, // Fallback to generic
			expected: DefaultGenericMapping(),
		},
		{
			format:   FormatLHVEE, // Fallback to generic
			expected: DefaultGenericMapping(),
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			result := GetMappingForFormat(tt.format)
			assert.Equal(t, tt.expected.DateColumn, result.DateColumn)
			assert.Equal(t, tt.expected.AmountColumn, result.AmountColumn)
			assert.Equal(t, tt.expected.DateFormat, result.DateFormat)
		})
	}
}

func TestParseCSVPreview(t *testing.T) {
	t.Run("Basic CSV", func(t *testing.T) {
		csvData := "Date,Amount,Description\n2025-01-01,100.00,Test payment\n2025-01-02,200.00,Another payment"
		reader := strings.NewReader(csvData)

		rows, err := ParseCSVPreview(reader, 10)
		require.NoError(t, err)
		assert.Len(t, rows, 3)
		assert.Equal(t, []string{"Date", "Amount", "Description"}, rows[0])
	})

	t.Run("Limited rows", func(t *testing.T) {
		csvData := "A,B\n1,2\n3,4\n5,6\n7,8"
		reader := strings.NewReader(csvData)

		rows, err := ParseCSVPreview(reader, 2)
		require.NoError(t, err)
		assert.Len(t, rows, 2)
	})

	t.Run("Empty CSV", func(t *testing.T) {
		reader := strings.NewReader("")

		rows, err := ParseCSVPreview(reader, 10)
		require.NoError(t, err)
		assert.Len(t, rows, 0)
	})

	t.Run("CSV with varying field counts parses due to FieldsPerRecord=-1", func(t *testing.T) {
		// With FieldsPerRecord=-1, varying field counts are allowed
		csvData := "A,B,C\n1,2\n3,4,5,6"
		reader := strings.NewReader(csvData)

		rows, err := ParseCSVPreview(reader, 10)
		require.NoError(t, err)
		assert.Len(t, rows, 3)
	})

	t.Run("Reader error during parsing", func(t *testing.T) {
		reader := &errorReader{
			data:   "Date,Amount\n2025-01-01,100.00",
			errAt:  15,
			errMsg: "simulated read error",
		}

		_, err := ParseCSVPreview(reader, 10)
		assert.Error(t, err)
	})
}

func TestValidateCSVRow(t *testing.T) {
	mapping := CSVColumnMapping{
		DateColumn:         0,
		AmountColumn:       1,
		DateFormat:         "2006-01-02",
		DecimalSeparator:   ".",
		ThousandsSeparator: ",",
	}

	tests := []struct {
		name        string
		record      []string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid row",
			record:      []string{"2025-01-15", "100.50", "Description"},
			expectError: false,
		},
		{
			name:        "Date column out of range",
			record:      []string{},
			expectError: true,
			errorMsg:    "date column",
		},
		{
			name:        "Amount column out of range",
			record:      []string{"2025-01-15"},
			expectError: true,
			errorMsg:    "amount column",
		},
		{
			name:        "Invalid date format",
			record:      []string{"15-01-2025", "100.50"},
			expectError: true,
			errorMsg:    "invalid date",
		},
		{
			name:        "Invalid amount",
			record:      []string{"2025-01-15", "not-a-number"},
			expectError: true,
			errorMsg:    "invalid amount",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCSVRow(tt.record, mapping)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFormatAmount(t *testing.T) {
	tests := []struct {
		name     string
		amount   decimal.Decimal
		decimals int32
		expected string
	}{
		{
			name:     "Simple amount",
			amount:   decimal.NewFromFloat(100.50),
			decimals: 2,
			expected: "100.50",
		},
		{
			name:     "With thousands",
			amount:   decimal.NewFromFloat(1234.56),
			decimals: 2,
			expected: "1,234.56",
		},
		{
			name:     "Large number",
			amount:   decimal.NewFromFloat(1234567.89),
			decimals: 2,
			expected: "1,234,567.89",
		},
		{
			name:     "Negative amount",
			amount:   decimal.NewFromFloat(-1234.56),
			decimals: 2,
			expected: "-1,234.56",
		},
		{
			name:     "Zero",
			amount:   decimal.Zero,
			decimals: 2,
			expected: "0.00",
		},
		{
			name:     "No decimals",
			amount:   decimal.NewFromInt(1000),
			decimals: 0,
			expected: "1,000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatAmount(tt.amount, tt.decimals)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseDateFormats(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    time.Time
		expectError bool
	}{
		{
			name:     "ISO format",
			input:    "2025-01-15",
			expected: time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "European format",
			input:    "15.01.2025",
			expected: time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "US format MM/DD/YYYY",
			input:    "01/15/2025",
			expected: time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "European slash format",
			input:    "15/01/2025",
			expected: time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "With whitespace",
			input:    "  2025-01-15  ",
			expected: time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:        "Invalid format",
			input:       "not-a-date",
			expectError: true,
		},
		{
			name:        "Empty string",
			input:       "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseDateFormats(tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected.Year(), result.Year())
				assert.Equal(t, tt.expected.Month(), result.Month())
				assert.Equal(t, tt.expected.Day(), result.Day())
			}
		})
	}
}

func TestParseIntOrDefault(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		defaultVal int
		expected   int
	}{
		{
			name:       "Valid integer",
			input:      "42",
			defaultVal: 0,
			expected:   42,
		},
		{
			name:       "Empty string",
			input:      "",
			defaultVal: 10,
			expected:   10,
		},
		{
			name:       "Invalid string",
			input:      "abc",
			defaultVal: 5,
			expected:   5,
		},
		{
			name:       "Negative integer",
			input:      "-10",
			defaultVal: 0,
			expected:   -10,
		},
		{
			name:       "Zero",
			input:      "0",
			defaultVal: 99,
			expected:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseIntOrDefault(tt.input, tt.defaultVal)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCSVFormatConstants(t *testing.T) {
	assert.Equal(t, CSVFormat("GENERIC"), FormatGeneric)
	assert.Equal(t, CSVFormat("SWEDBANK_EE"), FormatSwedbankEE)
	assert.Equal(t, CSVFormat("SEB_EE"), FormatSEBEE)
	assert.Equal(t, CSVFormat("LHV_EE"), FormatLHVEE)
}

func TestCSVColumnMapping_Fields(t *testing.T) {
	mapping := CSVColumnMapping{
		DateColumn:                0,
		ValueDateColumn:           1,
		AmountColumn:              2,
		DescriptionColumn:         3,
		ReferenceColumn:           4,
		CounterpartyNameColumn:    5,
		CounterpartyAccountColumn: 6,
		ExternalIDColumn:          7,
		DateFormat:                "2006-01-02",
		DecimalSeparator:          ".",
		ThousandsSeparator:        ",",
		SkipRows:                  1,
		HasHeader:                 true,
	}

	assert.Equal(t, 0, mapping.DateColumn)
	assert.Equal(t, 1, mapping.ValueDateColumn)
	assert.Equal(t, 2, mapping.AmountColumn)
	assert.Equal(t, 3, mapping.DescriptionColumn)
	assert.Equal(t, 4, mapping.ReferenceColumn)
	assert.Equal(t, 5, mapping.CounterpartyNameColumn)
	assert.Equal(t, 6, mapping.CounterpartyAccountColumn)
	assert.Equal(t, 7, mapping.ExternalIDColumn)
	assert.Equal(t, "2006-01-02", mapping.DateFormat)
	assert.Equal(t, ".", mapping.DecimalSeparator)
	assert.Equal(t, ",", mapping.ThousandsSeparator)
	assert.Equal(t, 1, mapping.SkipRows)
	assert.True(t, mapping.HasHeader)
}
