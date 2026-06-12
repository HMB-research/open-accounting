package accounting

import (
	"context"
	"errors"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newOpeningBalanceImportMockRepository(tenantID string) *MockRepository {
	repo := NewMockRepository()
	repo.accounts["acc-1000"] = &Account{
		ID:          "acc-1000",
		TenantID:    tenantID,
		Code:        "1000",
		Name:        "Cash",
		AccountType: AccountTypeAsset,
		IsActive:    true,
	}
	repo.accounts["acc-3000"] = &Account{
		ID:          "acc-3000",
		TenantID:    tenantID,
		Code:        "3000",
		Name:        "Owner Equity",
		AccountType: AccountTypeEquity,
		IsActive:    true,
	}
	return repo
}

type openingBalanceLoadErrorRepository struct {
	*MockRepository
	getJournalCalls int
}

func (r *openingBalanceLoadErrorRepository) GetJournalEntryByID(ctx context.Context, schemaName, tenantID, entryID string) (*JournalEntry, error) {
	r.getJournalCalls++
	if r.getJournalCalls > 1 {
		return nil, errors.New("load failed")
	}
	return r.MockRepository.GetJournalEntryByID(ctx, schemaName, tenantID, entryID)
}

func TestService_ImportOpeningBalancesCSV(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_test"
	tenantID := "tenant-1"

	t.Run("imports balanced opening balances into a posted journal entry", func(t *testing.T) {
		repo := NewMockRepository()
		repo.accounts["acc-1000"] = &Account{
			ID:          "acc-1000",
			TenantID:    tenantID,
			Code:        "1000",
			Name:        "Cash",
			AccountType: AccountTypeAsset,
			IsActive:    true,
		}
		repo.accounts["acc-3000"] = &Account{
			ID:          "acc-3000",
			TenantID:    tenantID,
			Code:        "3000",
			Name:        "Owner Equity",
			AccountType: AccountTypeEquity,
			IsActive:    true,
		}

		svc := NewServiceWithRepository(repo)

		result, err := svc.ImportOpeningBalancesCSV(ctx, schemaName, tenantID, &ImportOpeningBalancesRequest{
			FileName:    "opening-balances.csv",
			EntryDate:   "2026-01-01",
			Description: "Opening balances",
			Reference:   "OB-2026",
			UserID:      "user-1",
			CSVContent: "account_code,debit,credit,description\n" +
				"1000,1500.00,0,Cash opening balance\n" +
				"3000,0,1500.00,Equity opening balance\n",
		})
		require.NoError(t, err)
		assert.Equal(t, "opening-balances.csv", result.FileName)
		assert.Equal(t, 2, result.RowsProcessed)
		assert.Equal(t, 2, result.LinesImported)
		assert.True(t, result.TotalDebit.Equal(decimal.NewFromInt(1500)))
		assert.True(t, result.TotalCredit.Equal(decimal.NewFromInt(1500)))
		require.NotNil(t, result.JournalEntry)
		assert.Equal(t, StatusPosted, result.JournalEntry.Status)
		assert.Equal(t, "OPENING_BALANCE", result.JournalEntry.SourceType)
		assert.Len(t, result.JournalEntry.Lines, 2)
	})

	t.Run("rejects nil or incomplete requests", func(t *testing.T) {
		svc := NewServiceWithRepository(NewMockRepository())
		validCSV := "account_code,debit,credit\n1000,100.00,0\n3000,0,100.00\n"

		tests := []struct {
			name    string
			req     *ImportOpeningBalancesRequest
			wantErr string
		}{
			{
				name:    "nil request",
				req:     nil,
				wantErr: "csv_content is required",
			},
			{
				name: "missing entry date",
				req: &ImportOpeningBalancesRequest{
					UserID:     "user-1",
					CSVContent: validCSV,
				},
				wantErr: "entry_date is required",
			},
			{
				name: "missing csv content",
				req: &ImportOpeningBalancesRequest{
					EntryDate: "2026-01-01",
					UserID:    "user-1",
				},
				wantErr: "csv_content is required",
			},
			{
				name: "missing user id",
				req: &ImportOpeningBalancesRequest{
					EntryDate:  "2026-01-01",
					CSVContent: validCSV,
				},
				wantErr: "user_id is required",
			},
			{
				name: "invalid entry date",
				req: &ImportOpeningBalancesRequest{
					EntryDate:  "01-01-2026",
					UserID:     "user-1",
					CSVContent: validCSV,
				},
				wantErr: "entry_date must be in YYYY-MM-DD format",
			},
			{
				name: "parser error",
				req: &ImportOpeningBalancesRequest{
					EntryDate:  "2026-01-01",
					UserID:     "user-1",
					CSVContent: "account_code,debit,description\n1000,100.00,Cash\n",
				},
				wantErr: "missing required columns",
			},
			{
				name: "header only",
				req: &ImportOpeningBalancesRequest{
					EntryDate:  "2026-01-01",
					UserID:     "user-1",
					CSVContent: "account_code,debit,credit\n",
				},
				wantErr: "no opening balance rows found in CSV",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := svc.ImportOpeningBalancesCSV(ctx, schemaName, tenantID, tt.req)

				require.Error(t, err)
				assert.Nil(t, result)
				assert.Contains(t, err.Error(), tt.wantErr)
			})
		}
	})

	t.Run("uses default entry and line metadata", func(t *testing.T) {
		repo := newOpeningBalanceImportMockRepository(tenantID)
		svc := NewServiceWithRepository(repo)

		result, err := svc.ImportOpeningBalancesCSV(ctx, schemaName, tenantID, &ImportOpeningBalancesRequest{
			EntryDate:  "2026-01-01",
			UserID:     "user-1",
			CSVContent: "account_code,debit,credit\n1000,100.00,0\n3000,0,100.00\n",
		})

		require.NoError(t, err)
		require.NotNil(t, result.JournalEntry)
		assert.Equal(t, "Opening balances", result.JournalEntry.Description)
		assert.Equal(t, "OB-2026", result.JournalEntry.Reference)
		require.Len(t, result.JournalEntry.Lines, 2)
		assert.Equal(t, "Opening balance for Cash", result.JournalEntry.Lines[0].Description)
		assert.Equal(t, "Opening balance for Owner Equity", result.JournalEntry.Lines[1].Description)
	})

	t.Run("wraps list account errors", func(t *testing.T) {
		repo := newOpeningBalanceImportMockRepository(tenantID)
		repo.listAccountsErr = errors.New("database unavailable")
		svc := NewServiceWithRepository(repo)

		result, err := svc.ImportOpeningBalancesCSV(ctx, schemaName, tenantID, &ImportOpeningBalancesRequest{
			EntryDate:  "2026-01-01",
			UserID:     "user-1",
			CSVContent: "account_code,debit,credit\n1000,100.00,0\n3000,0,100.00\n",
		})

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "list accounts")
		assert.Contains(t, err.Error(), "database unavailable")
	})

	t.Run("reports row amount validation errors", func(t *testing.T) {
		repo := newOpeningBalanceImportMockRepository(tenantID)
		svc := NewServiceWithRepository(repo)

		result, err := svc.ImportOpeningBalancesCSV(ctx, schemaName, tenantID, &ImportOpeningBalancesRequest{
			EntryDate:  "2026-01-01",
			UserID:     "user-1",
			CSVContent: "account_code,debit,credit\n1000,100.00,100.00\n3000,0,100.00\n",
		})

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "row 2")
		assert.Contains(t, err.Error(), "row cannot contain both debit and credit amounts")
	})

	t.Run("requires both debit and credit totals", func(t *testing.T) {
		repo := newOpeningBalanceImportMockRepository(tenantID)
		svc := NewServiceWithRepository(repo)

		result, err := svc.ImportOpeningBalancesCSV(ctx, schemaName, tenantID, &ImportOpeningBalancesRequest{
			EntryDate:  "2026-01-01",
			UserID:     "user-1",
			CSVContent: "account_code,debit,credit\n1000,100.00,0\n",
		})

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "opening balances must include both debit and credit totals")
	})

	t.Run("wraps create post and load errors", func(t *testing.T) {
		validCSV := "account_code,debit,credit\n1000,100.00,0\n3000,0,100.00\n"

		tests := []struct {
			name    string
			repo    RepositoryInterface
			wantErr string
		}{
			{
				name: "create error",
				repo: func() RepositoryInterface {
					repo := newOpeningBalanceImportMockRepository(tenantID)
					repo.createJournalErr = errors.New("insert failed")
					return repo
				}(),
				wantErr: "create opening-balance journal entry",
			},
			{
				name: "post error",
				repo: func() RepositoryInterface {
					repo := newOpeningBalanceImportMockRepository(tenantID)
					repo.updateStatusErr = errors.New("status update failed")
					return repo
				}(),
				wantErr: "post opening-balance journal entry",
			},
			{
				name: "load error",
				repo: &openingBalanceLoadErrorRepository{
					MockRepository: newOpeningBalanceImportMockRepository(tenantID),
				},
				wantErr: "load opening-balance journal entry",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				svc := NewServiceWithRepository(tt.repo)

				result, err := svc.ImportOpeningBalancesCSV(ctx, schemaName, tenantID, &ImportOpeningBalancesRequest{
					EntryDate:  "2026-01-01",
					UserID:     "user-1",
					CSVContent: validCSV,
				})

				require.Error(t, err)
				assert.Nil(t, result)
				assert.Contains(t, err.Error(), tt.wantErr)
			})
		}
	})

	t.Run("rejects unbalanced opening balances", func(t *testing.T) {
		repo := NewMockRepository()
		repo.accounts["acc-1000"] = &Account{
			ID:          "acc-1000",
			TenantID:    tenantID,
			Code:        "1000",
			Name:        "Cash",
			AccountType: AccountTypeAsset,
			IsActive:    true,
		}
		repo.accounts["acc-3000"] = &Account{
			ID:          "acc-3000",
			TenantID:    tenantID,
			Code:        "3000",
			Name:        "Owner Equity",
			AccountType: AccountTypeEquity,
			IsActive:    true,
		}

		svc := NewServiceWithRepository(repo)

		_, err := svc.ImportOpeningBalancesCSV(ctx, schemaName, tenantID, &ImportOpeningBalancesRequest{
			EntryDate:  "2026-01-01",
			UserID:     "user-1",
			CSVContent: "account_code,debit,credit\n1000,100.00,0\n3000,0,90.00\n",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "opening balances do not balance")
	})

	t.Run("rejects unknown account codes", func(t *testing.T) {
		repo := NewMockRepository()
		repo.accounts["acc-1000"] = &Account{
			ID:          "acc-1000",
			TenantID:    tenantID,
			Code:        "1000",
			Name:        "Cash",
			AccountType: AccountTypeAsset,
			IsActive:    true,
		}

		svc := NewServiceWithRepository(repo)

		_, err := svc.ImportOpeningBalancesCSV(ctx, schemaName, tenantID, &ImportOpeningBalancesRequest{
			EntryDate:  "2026-01-01",
			UserID:     "user-1",
			CSVContent: "account_code,debit,credit\n9999,100.00,0\n3000,0,100.00\n",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "account_code")
		assert.Contains(t, err.Error(), "was not found")
	})
}

func TestParseOpeningBalanceAmounts(t *testing.T) {
	t.Run("accepts debit-only amount", func(t *testing.T) {
		debit, credit, err := parseOpeningBalanceAmounts(openingBalanceImportRow{
			values: map[string]string{"debit": "1,250.50", "credit": ""},
		})

		require.NoError(t, err)
		assert.True(t, decimal.RequireFromString("1250.50").Equal(debit))
		assert.True(t, credit.IsZero())
	})

	t.Run("accepts credit-only amount", func(t *testing.T) {
		debit, credit, err := parseOpeningBalanceAmounts(openingBalanceImportRow{
			values: map[string]string{"debit": "", "credit": "99,95"},
		})

		require.NoError(t, err)
		assert.True(t, debit.IsZero())
		assert.True(t, decimal.RequireFromString("99.95").Equal(credit))
	})

	t.Run("rejects invalid debit", func(t *testing.T) {
		debit, credit, err := parseOpeningBalanceAmounts(openingBalanceImportRow{
			values: map[string]string{"debit": "not-a-number", "credit": "0"},
		})

		require.Error(t, err)
		assert.True(t, debit.IsZero())
		assert.True(t, credit.IsZero())
		assert.Contains(t, err.Error(), "invalid debit")
	})

	t.Run("rejects invalid credit", func(t *testing.T) {
		debit, credit, err := parseOpeningBalanceAmounts(openingBalanceImportRow{
			values: map[string]string{"debit": "0", "credit": "not-a-number"},
		})

		require.Error(t, err)
		assert.True(t, debit.IsZero())
		assert.True(t, credit.IsZero())
		assert.Contains(t, err.Error(), "invalid credit")
	})

	t.Run("rejects negative amounts", func(t *testing.T) {
		debit, credit, err := parseOpeningBalanceAmounts(openingBalanceImportRow{
			values: map[string]string{"debit": "-10.00", "credit": "0"},
		})

		require.Error(t, err)
		assert.True(t, debit.IsZero())
		assert.True(t, credit.IsZero())
		assert.Contains(t, err.Error(), "amounts cannot be negative")
	})

	t.Run("requires one-sided amount", func(t *testing.T) {
		debit, credit, err := parseOpeningBalanceAmounts(openingBalanceImportRow{
			values: map[string]string{"debit": "0", "credit": "0"},
		})

		require.Error(t, err)
		assert.True(t, debit.IsZero())
		assert.True(t, credit.IsZero())
		assert.Contains(t, err.Error(), "either debit or credit is required")
	})

	t.Run("rejects two-sided amount", func(t *testing.T) {
		debit, credit, err := parseOpeningBalanceAmounts(openingBalanceImportRow{
			values: map[string]string{"debit": "10.00", "credit": "10.00"},
		})

		require.Error(t, err)
		assert.True(t, debit.IsZero())
		assert.True(t, credit.IsZero())
		assert.Contains(t, err.Error(), "row cannot contain both debit and credit amounts")
	})
}

func TestParseOpeningBalanceImportRows(t *testing.T) {
	t.Run("parses aliased semicolon headers and skips blank rows", func(t *testing.T) {
		rows, err := parseOpeningBalanceImportRows("\ufeff Account ; Debit Amount ; credit_amount ; line_description ; custom field\n" +
			" 1000 ; 1,250.50 ; ; Cash opening ; ignored\n" +
			" ; ; ; ; \n" +
			"3000; ; 1,250.50 ; Equity opening ; ignored\n")

		require.NoError(t, err)
		require.Len(t, rows, 2)

		assert.Equal(t, 2, rows[0].rowNumber)
		assert.Equal(t, "1000", rows[0].values["account_code"])
		assert.Equal(t, "1,250.50", rows[0].values["debit"])
		assert.Empty(t, rows[0].values["credit"])
		assert.Equal(t, "Cash opening", rows[0].values["description"])
		assert.Equal(t, "ignored", rows[0].values["custom_field"])

		assert.Equal(t, 4, rows[1].rowNumber)
		assert.Equal(t, "3000", rows[1].values["account_code"])
		assert.Empty(t, rows[1].values["debit"])
		assert.Equal(t, "1,250.50", rows[1].values["credit"])
		assert.Equal(t, "Equity opening", rows[1].values["description"])
	})

	t.Run("allows header-only csv", func(t *testing.T) {
		rows, err := parseOpeningBalanceImportRows("account_code,debit,credit\n")

		require.NoError(t, err)
		assert.Empty(t, rows)
	})

	t.Run("ignores blank header columns", func(t *testing.T) {
		rows, err := parseOpeningBalanceImportRows("account_code,debit,credit,\n1000,10.00,,ignored\n")

		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, "1000", rows[0].values["account_code"])
		assert.Equal(t, "10.00", rows[0].values["debit"])
		assert.Empty(t, rows[0].values["credit"])
		_, ok := rows[0].values[""]
		assert.False(t, ok)
	})

	t.Run("requires content", func(t *testing.T) {
		rows, err := parseOpeningBalanceImportRows(" \t\n ")

		require.Error(t, err)
		assert.Nil(t, rows)
		assert.Contains(t, err.Error(), "csv_content is required")
	})

	t.Run("requires account debit and credit columns", func(t *testing.T) {
		rows, err := parseOpeningBalanceImportRows("account_code,debit,description\n1000,10.00,Cash\n")

		require.Error(t, err)
		assert.Nil(t, rows)
		assert.Contains(t, err.Error(), "missing required columns")
	})

	t.Run("reports malformed csv headers", func(t *testing.T) {
		rows, err := parseOpeningBalanceImportRows("\"account_code,debit,credit\n")

		require.Error(t, err)
		assert.Nil(t, rows)
		assert.Contains(t, err.Error(), "parse csv header")
	})

	t.Run("reports malformed csv rows", func(t *testing.T) {
		rows, err := parseOpeningBalanceImportRows("account_code,debit,credit\n1000,\"10.00,0\n")

		require.Error(t, err)
		assert.Nil(t, rows)
		assert.Contains(t, err.Error(), "parse csv row 2")
	})
}

func TestOpeningBalanceImportHeaderAndDecimalNormalization(t *testing.T) {
	t.Run("canonicalizes opening balance header aliases", func(t *testing.T) {
		assert.Equal(t, "account_code", canonicalOpeningBalanceHeader(" Account "))
		assert.Equal(t, "debit", canonicalOpeningBalanceHeader("Debit Amount"))
		assert.Equal(t, "credit", canonicalOpeningBalanceHeader("credit_amount"))
		assert.Equal(t, "custom_field", canonicalOpeningBalanceHeader("Custom Field"))
	})

	t.Run("normalizes decimal formats", func(t *testing.T) {
		assert.Equal(t, "1250.50", normalizeOpeningBalanceDecimal("1,250.50"))
		assert.Equal(t, "1250.50", normalizeOpeningBalanceDecimal("1250,50"))
		assert.Equal(t, "1250.50", normalizeOpeningBalanceDecimal("1250.50"))
	})
}
