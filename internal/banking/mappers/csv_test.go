package mappers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCSVNormalizesHeadersAndSkipsEmptyRows(t *testing.T) {
	parsed, err := ParseCSV(" Date ;Debit/Credit (D/C);Beneficiary's account\n2026-03-15;D;EE123\n ; ; \n", "statement")
	require.NoError(t, err)

	assert.Equal(t, []string{"Date ", "Debit/Credit (D/C)", "Beneficiary's account"}, parsed.Headers)
	require.Len(t, parsed.Rows, 1)
	assert.Equal(t, "2026-03-15", Field(parsed.Rows[0], parsed.Index, "date"))
	assert.Equal(t, "D", Field(parsed.Rows[0], parsed.Index, "debit_credit_d_c"))
	assert.Equal(t, "EE123", Field(parsed.Rows[0], parsed.Index, "beneficiarys_account"))
	assert.True(t, HasAnyHeader(parsed.Index, "Beneficiary's account", "missing"))
	assert.False(t, HasAnyHeader(parsed.Index, "missing"))
}

func TestParseCSVDetectsCommonDelimiters(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		delimiter rune
	}{
		{name: "comma", content: "date,amount,description\n2026-03-15,10,Payment", delimiter: ','},
		{name: "semicolon", content: "date;amount;description\n2026-03-15;10;Payment", delimiter: ';'},
		{name: "tab", content: "date\tamount\tdescription\n2026-03-15\t10\tPayment", delimiter: '\t'},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.delimiter, DetectDelimiter(tt.content))
			parsed, err := ParseCSV(tt.content, "statement")
			require.NoError(t, err)
			require.Len(t, parsed.Rows, 1)
			assert.Equal(t, "Payment", Field(parsed.Rows[0], parsed.Index, "description"))
		})
	}
}

func TestParseCSVRejectsEmptyAndMalformedContent(t *testing.T) {
	_, err := ParseCSV("  \n\t", "statement")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "statement CSV is empty")

	_, err = ParseCSV("date,amount\n2026-03-15,\"10", "statement")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read statement CSV row 2")
}

func TestNormalizeHeaderRemovesPunctuationAndRepeatedSeparators(t *testing.T) {
	assert.Equal(t, "beneficiarys_remitters_account", NormalizeHeader(" Beneficiary’s/remitter's account "))
	assert.Equal(t, "debit_credit_d_c", NormalizeHeader("Debit/Credit (D/C)"))
	assert.Equal(t, "konto_teenusepakkuja_viide", NormalizeHeader("Konto teenusepakkuja viide"))
}
