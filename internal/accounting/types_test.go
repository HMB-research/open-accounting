package accounting

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccountType_IsDebitNormal(t *testing.T) {
	tests := []struct {
		accountType AccountType
		expected    bool
	}{
		{AccountTypeAsset, true},
		{AccountTypeExpense, true},
		{AccountTypeLiability, false},
		{AccountTypeEquity, false},
		{AccountTypeRevenue, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.accountType), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.accountType.IsDebitNormal())
		})
	}
}

func TestJournalEntry_Validate(t *testing.T) {
	t.Run("valid balanced entry", func(t *testing.T) {
		entry := &JournalEntry{
			Lines: []JournalEntryLine{
				{
					AccountID:    "acc1",
					DebitAmount:  decimal.NewFromFloat(100),
					CreditAmount: decimal.Zero,
					BaseDebit:    decimal.NewFromFloat(100),
					BaseCredit:   decimal.Zero,
				},
				{
					AccountID:    "acc2",
					DebitAmount:  decimal.Zero,
					CreditAmount: decimal.NewFromFloat(100),
					BaseDebit:    decimal.Zero,
					BaseCredit:   decimal.NewFromFloat(100),
				},
			},
		}

		err := entry.Validate()
		assert.NoError(t, err)
	})

	t.Run("entry with no lines", func(t *testing.T) {
		entry := &JournalEntry{
			Lines: []JournalEntryLine{},
		}

		err := entry.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one line")
	})

	t.Run("unbalanced entry", func(t *testing.T) {
		entry := &JournalEntry{
			Lines: []JournalEntryLine{
				{
					AccountID:    "acc1",
					DebitAmount:  decimal.NewFromFloat(100),
					CreditAmount: decimal.Zero,
					BaseDebit:    decimal.NewFromFloat(100),
					BaseCredit:   decimal.Zero,
				},
				{
					AccountID:    "acc2",
					DebitAmount:  decimal.Zero,
					CreditAmount: decimal.NewFromFloat(50),
					BaseDebit:    decimal.Zero,
					BaseCredit:   decimal.NewFromFloat(50),
				},
			},
		}

		err := entry.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not balance")
	})

	t.Run("line with both debit and credit", func(t *testing.T) {
		entry := &JournalEntry{
			Lines: []JournalEntryLine{
				{
					AccountID:    "acc1",
					DebitAmount:  decimal.NewFromFloat(100),
					CreditAmount: decimal.NewFromFloat(50),
					BaseDebit:    decimal.NewFromFloat(100),
					BaseCredit:   decimal.NewFromFloat(50),
				},
			},
		}

		err := entry.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "both debit and credit")
	})

	t.Run("negative amount", func(t *testing.T) {
		entry := &JournalEntry{
			Lines: []JournalEntryLine{
				{
					AccountID:    "acc1",
					DebitAmount:  decimal.NewFromFloat(-100),
					CreditAmount: decimal.Zero,
					BaseDebit:    decimal.NewFromFloat(-100),
					BaseCredit:   decimal.Zero,
				},
			},
		}

		err := entry.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "negative")
	})

	t.Run("zero amount entry", func(t *testing.T) {
		entry := &JournalEntry{
			Lines: []JournalEntryLine{
				{
					AccountID:    "acc1",
					DebitAmount:  decimal.Zero,
					CreditAmount: decimal.Zero,
					BaseDebit:    decimal.Zero,
					BaseCredit:   decimal.Zero,
				},
			},
		}

		err := entry.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "zero amounts")
	})
}

func TestJournalEntry_TotalDebits(t *testing.T) {
	entry := &JournalEntry{
		Lines: []JournalEntryLine{
			{BaseDebit: decimal.NewFromFloat(100)},
			{BaseDebit: decimal.NewFromFloat(50.50)},
			{BaseDebit: decimal.NewFromFloat(25.25)},
		},
	}

	total := entry.TotalDebits()
	assert.True(t, total.Equal(decimal.NewFromFloat(175.75)))
}

func TestJournalEntry_TotalCredits(t *testing.T) {
	entry := &JournalEntry{
		Lines: []JournalEntryLine{
			{BaseCredit: decimal.NewFromFloat(100)},
			{BaseCredit: decimal.NewFromFloat(75.75)},
		},
	}

	total := entry.TotalCredits()
	assert.True(t, total.Equal(decimal.NewFromFloat(175.75)))
}

func TestJournalEntry_IsBalanced(t *testing.T) {
	t.Run("balanced", func(t *testing.T) {
		entry := &JournalEntry{
			Lines: []JournalEntryLine{
				{BaseDebit: decimal.NewFromFloat(100), BaseCredit: decimal.Zero},
				{BaseDebit: decimal.Zero, BaseCredit: decimal.NewFromFloat(100)},
			},
		}
		assert.True(t, entry.IsBalanced())
	})

	t.Run("unbalanced", func(t *testing.T) {
		entry := &JournalEntry{
			Lines: []JournalEntryLine{
				{BaseDebit: decimal.NewFromFloat(100), BaseCredit: decimal.Zero},
				{BaseDebit: decimal.Zero, BaseCredit: decimal.NewFromFloat(99)},
			},
		}
		assert.False(t, entry.IsBalanced())
	})
}

func TestJournalEntry_MultiCurrency(t *testing.T) {
	// Test with exchange rate
	entry := &JournalEntry{
		Lines: []JournalEntryLine{
			{
				DebitAmount:  decimal.NewFromFloat(100),
				CreditAmount: decimal.Zero,
				Currency:     "USD",
				ExchangeRate: decimal.NewFromFloat(0.92),
				BaseDebit:    decimal.NewFromFloat(92),
				BaseCredit:   decimal.Zero,
			},
			{
				DebitAmount:  decimal.Zero,
				CreditAmount: decimal.NewFromFloat(100),
				Currency:     "USD",
				ExchangeRate: decimal.NewFromFloat(0.92),
				BaseDebit:    decimal.Zero,
				BaseCredit:   decimal.NewFromFloat(92),
			},
		},
	}

	assert.True(t, entry.IsBalanced())
	assert.True(t, entry.TotalDebits().Equal(decimal.NewFromFloat(92)))
	assert.True(t, entry.TotalCredits().Equal(decimal.NewFromFloat(92)))
}

func TestDecimalPrecision(t *testing.T) {
	// Test that we can handle large numbers with 8 decimal precision
	// as per NUMERIC(28,8) in the database

	t.Run("large amounts", func(t *testing.T) {
		largeAmount := decimal.RequireFromString("12345678901234567890.12345678")

		entry := &JournalEntry{
			Lines: []JournalEntryLine{
				{
					DebitAmount:  largeAmount,
					CreditAmount: decimal.Zero,
					BaseDebit:    largeAmount,
					BaseCredit:   decimal.Zero,
				},
				{
					DebitAmount:  decimal.Zero,
					CreditAmount: largeAmount,
					BaseDebit:    decimal.Zero,
					BaseCredit:   largeAmount,
				},
			},
		}

		assert.True(t, entry.IsBalanced())
		assert.True(t, entry.TotalDebits().Equal(largeAmount))
	})

	t.Run("precise calculations", func(t *testing.T) {
		// Test rounding behavior
		a := decimal.RequireFromString("1.00000001")
		b := decimal.RequireFromString("2.00000002")
		expected := decimal.RequireFromString("3.00000003")

		result := a.Add(b)
		assert.True(t, result.Equal(expected))
	})

	t.Run("division precision", func(t *testing.T) {
		amount := decimal.NewFromFloat(100)
		rate := decimal.RequireFromString("3")

		// 100 / 3 should give us high precision
		result := amount.Div(rate)

		// Should be approximately 33.33333333
		expected := decimal.RequireFromString("33.33333333")
		assert.True(t, result.Round(8).Equal(expected))
	})
}

func TestNewService(t *testing.T) {
	service := NewService(nil)
	assert.NotNil(t, service)
}

func TestNewService_WithNilPool(t *testing.T) {
	// Service can be created without a pool for testing
	service := NewService(nil)
	assert.NotNil(t, service, "NewService should return a non-nil service even with nil pool")
}
