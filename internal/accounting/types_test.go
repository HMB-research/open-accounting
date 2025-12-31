package accounting

import (
	"testing"
	"time"

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

func TestAccountType_Constants(t *testing.T) {
	assert.Equal(t, AccountType("ASSET"), AccountTypeAsset)
	assert.Equal(t, AccountType("LIABILITY"), AccountTypeLiability)
	assert.Equal(t, AccountType("EQUITY"), AccountTypeEquity)
	assert.Equal(t, AccountType("REVENUE"), AccountTypeRevenue)
	assert.Equal(t, AccountType("EXPENSE"), AccountTypeExpense)
}

func TestJournalEntryStatus_Constants(t *testing.T) {
	assert.Equal(t, JournalEntryStatus("DRAFT"), StatusDraft)
	assert.Equal(t, JournalEntryStatus("POSTED"), StatusPosted)
	assert.Equal(t, JournalEntryStatus("VOIDED"), StatusVoided)
}

func TestAccount_Fields(t *testing.T) {
	account := Account{
		ID:          "acc-123",
		TenantID:    "tenant-456",
		Code:        "1000",
		Name:        "Cash",
		AccountType: AccountTypeAsset,
		IsActive:    true,
		IsSystem:    true,
		Description: "Main cash account",
	}

	assert.Equal(t, "acc-123", account.ID)
	assert.Equal(t, "1000", account.Code)
	assert.Equal(t, "Cash", account.Name)
	assert.Equal(t, AccountTypeAsset, account.AccountType)
	assert.True(t, account.IsActive)
	assert.True(t, account.IsSystem)
}

func TestAccount_WithParent(t *testing.T) {
	parentID := "parent-123"
	account := Account{
		ID:          "acc-456",
		TenantID:    "tenant-789",
		Code:        "1001",
		Name:        "Petty Cash",
		AccountType: AccountTypeAsset,
		ParentID:    &parentID,
		IsActive:    true,
	}

	require.NotNil(t, account.ParentID)
	assert.Equal(t, "parent-123", *account.ParentID)
}

func TestJournalEntry_Fields(t *testing.T) {
	now := time.Now()
	sourceID := "inv-123"

	entry := JournalEntry{
		ID:          "je-123",
		TenantID:    "tenant-456",
		EntryNumber: "JE-00001",
		EntryDate:   now,
		Description: "Test entry",
		Reference:   "REF-001",
		SourceType:  "INVOICE",
		SourceID:    &sourceID,
		Status:      StatusDraft,
		CreatedAt:   now,
		CreatedBy:   "user-123",
	}

	assert.Equal(t, "JE-00001", entry.EntryNumber)
	assert.Equal(t, "INVOICE", entry.SourceType)
	require.NotNil(t, entry.SourceID)
	assert.Equal(t, "inv-123", *entry.SourceID)
	assert.Equal(t, StatusDraft, entry.Status)
}

func TestJournalEntryLine_Fields(t *testing.T) {
	line := JournalEntryLine{
		ID:             "line-123",
		TenantID:       "tenant-456",
		JournalEntryID: "je-789",
		AccountID:      "acc-001",
		Description:    "Line description",
		DebitAmount:    decimal.NewFromFloat(100.50),
		CreditAmount:   decimal.Zero,
		Currency:       "EUR",
		ExchangeRate:   decimal.NewFromInt(1),
		BaseDebit:      decimal.NewFromFloat(100.50),
		BaseCredit:     decimal.Zero,
	}

	assert.Equal(t, "line-123", line.ID)
	assert.True(t, line.DebitAmount.Equal(decimal.NewFromFloat(100.50)))
	assert.Equal(t, "EUR", line.Currency)
}

func TestCreateJournalEntryRequest_Fields(t *testing.T) {
	now := time.Now()
	sourceID := "source-123"

	req := CreateJournalEntryRequest{
		EntryDate:   now,
		Description: "New entry",
		Reference:   "REF-002",
		SourceType:  "MANUAL",
		SourceID:    &sourceID,
		UserID:      "user-123",
		Lines: []CreateJournalEntryLineReq{
			{
				AccountID:    "acc-001",
				Description:  "Debit line",
				DebitAmount:  decimal.NewFromFloat(500),
				CreditAmount: decimal.Zero,
			},
			{
				AccountID:    "acc-002",
				Description:  "Credit line",
				DebitAmount:  decimal.Zero,
				CreditAmount: decimal.NewFromFloat(500),
			},
		},
	}

	assert.Equal(t, "New entry", req.Description)
	assert.Len(t, req.Lines, 2)
	assert.Equal(t, "Debit line", req.Lines[0].Description)
}

func TestAccountBalance_Fields(t *testing.T) {
	balance := AccountBalance{
		AccountID:     "acc-123",
		AccountCode:   "1000",
		AccountName:   "Cash",
		AccountType:   AccountTypeAsset,
		DebitBalance:  decimal.NewFromFloat(10000),
		CreditBalance: decimal.NewFromFloat(3000),
		NetBalance:    decimal.NewFromFloat(7000),
	}

	assert.Equal(t, "1000", balance.AccountCode)
	assert.True(t, balance.NetBalance.Equal(decimal.NewFromFloat(7000)))
}

func TestAccountGroup_Fields(t *testing.T) {
	group := AccountGroup{
		Code:    "10",
		Name:    "Current Assets",
		Balance: decimal.NewFromFloat(50000),
		Children: []AccountBalance{
			{AccountCode: "1000", AccountName: "Cash", NetBalance: decimal.NewFromFloat(25000)},
			{AccountCode: "1100", AccountName: "Accounts Receivable", NetBalance: decimal.NewFromFloat(25000)},
		},
	}

	assert.Equal(t, "Current Assets", group.Name)
	assert.Len(t, group.Children, 2)
}

func TestBalanceSheet_Fields(t *testing.T) {
	now := time.Now()
	bs := BalanceSheet{
		TenantID:         "tenant-123",
		AsOfDate:         now,
		GeneratedAt:      now,
		TotalAssets:      decimal.NewFromFloat(100000),
		TotalLiabilities: decimal.NewFromFloat(40000),
		TotalEquity:      decimal.NewFromFloat(50000),
		RetainedEarnings: decimal.NewFromFloat(10000),
		IsBalanced:       true,
	}

	assert.True(t, bs.TotalAssets.Equal(decimal.NewFromFloat(100000)))
	assert.True(t, bs.IsBalanced)
}

func TestIncomeStatement_Fields(t *testing.T) {
	now := time.Now()
	lastMonth := now.AddDate(0, -1, 0)

	is := IncomeStatement{
		TenantID:      "tenant-123",
		StartDate:     lastMonth,
		EndDate:       now,
		GeneratedAt:   now,
		TotalRevenue:  decimal.NewFromFloat(50000),
		TotalExpenses: decimal.NewFromFloat(35000),
		NetIncome:     decimal.NewFromFloat(15000),
	}

	assert.True(t, is.NetIncome.Equal(decimal.NewFromFloat(15000)))
}

func TestJournalEntry_ComplexValidation(t *testing.T) {
	t.Run("multiple lines balanced", func(t *testing.T) {
		entry := &JournalEntry{
			Lines: []JournalEntryLine{
				{BaseDebit: decimal.NewFromFloat(100), BaseCredit: decimal.Zero},
				{BaseDebit: decimal.NewFromFloat(50), BaseCredit: decimal.Zero},
				{BaseDebit: decimal.Zero, BaseCredit: decimal.NewFromFloat(75)},
				{BaseDebit: decimal.Zero, BaseCredit: decimal.NewFromFloat(75)},
			},
		}

		err := entry.Validate()
		assert.NoError(t, err)
	})

	t.Run("negative credit amount", func(t *testing.T) {
		entry := &JournalEntry{
			Lines: []JournalEntryLine{
				{DebitAmount: decimal.Zero, CreditAmount: decimal.NewFromFloat(-100)},
			},
		}

		err := entry.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "negative")
	})
}

func TestJournalEntry_EdgeCases(t *testing.T) {
	t.Run("empty lines slice", func(t *testing.T) {
		entry := &JournalEntry{}
		assert.True(t, entry.TotalDebits().IsZero())
		assert.True(t, entry.TotalCredits().IsZero())
		assert.True(t, entry.IsBalanced())
	})

	t.Run("high precision amounts", func(t *testing.T) {
		amount := decimal.RequireFromString("0.00000001")
		entry := &JournalEntry{
			Lines: []JournalEntryLine{
				{BaseDebit: amount, BaseCredit: decimal.Zero},
				{BaseDebit: decimal.Zero, BaseCredit: amount},
			},
		}

		assert.True(t, entry.IsBalanced())
		assert.True(t, entry.TotalDebits().Equal(amount))
	})
}
