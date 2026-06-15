package accounting

import (
	"context"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGORMRepository_NilDatabase(t *testing.T) {
	repo := NewGORMRepository(nil)
	ctx := context.Background()
	schemaName := "tenant_schema"
	tenantID := "tenant-1"
	asOfDate := time.Date(2026, time.June, 30, 0, 0, 0, 0, time.UTC)

	require.NotNil(t, repo)
	assert.Nil(t, repo.db)

	tests := []struct {
		name string
		run  func(t *testing.T) error
	}{
		{
			name: "GetAccountByID",
			run: func(t *testing.T) error {
				account, err := repo.GetAccountByID(ctx, schemaName, tenantID, "account-1")
				assert.Nil(t, account)
				return err
			},
		},
		{
			name: "ListAccounts",
			run: func(t *testing.T) error {
				accounts, err := repo.ListAccounts(ctx, schemaName, tenantID, true)
				assert.Nil(t, accounts)
				return err
			},
		},
		{
			name: "CreateAccount",
			run: func(t *testing.T) error {
				return repo.CreateAccount(ctx, schemaName, &Account{TenantID: tenantID})
			},
		},
		{
			name: "UpdateAccount",
			run: func(t *testing.T) error {
				return repo.UpdateAccount(ctx, schemaName, &Account{ID: "account-1", TenantID: tenantID})
			},
		},
		{
			name: "GetJournalEntryByID",
			run: func(t *testing.T) error {
				entry, err := repo.GetJournalEntryByID(ctx, schemaName, tenantID, "entry-1")
				assert.Nil(t, entry)
				return err
			},
		},
		{
			name: "GetJournalEntryBySource",
			run: func(t *testing.T) error {
				entry, err := repo.GetJournalEntryBySource(ctx, schemaName, tenantID, "IMPORT", "source-1")
				assert.Nil(t, entry)
				return err
			},
		},
		{
			name: "ListJournalEntries",
			run: func(t *testing.T) error {
				entries, err := repo.ListJournalEntries(ctx, schemaName, tenantID, 25)
				assert.Nil(t, entries)
				return err
			},
		},
		{
			name: "CreateJournalEntryTemplate",
			run: func(t *testing.T) error {
				return repo.CreateJournalEntryTemplate(ctx, schemaName, &JournalEntryTemplate{TenantID: tenantID})
			},
		},
		{
			name: "ListJournalEntryTemplates",
			run: func(t *testing.T) error {
				templates, err := repo.ListJournalEntryTemplates(ctx, schemaName, tenantID, true)
				assert.Nil(t, templates)
				return err
			},
		},
		{
			name: "GetJournalEntryTemplateByID",
			run: func(t *testing.T) error {
				template, err := repo.GetJournalEntryTemplateByID(ctx, schemaName, tenantID, "template-1")
				assert.Nil(t, template)
				return err
			},
		},
		{
			name: "GetDueJournalEntryTemplateIDs",
			run: func(t *testing.T) error {
				ids, err := repo.GetDueJournalEntryTemplateIDs(ctx, schemaName, tenantID, asOfDate)
				assert.Nil(t, ids)
				return err
			},
		},
		{
			name: "UpdateJournalEntryTemplateAfterGeneration",
			run: func(t *testing.T) error {
				return repo.UpdateJournalEntryTemplateAfterGeneration(ctx, schemaName, tenantID, "template-1", asOfDate, asOfDate)
			},
		},
		{
			name: "countJournalEntryTemplateLines",
			run: func(t *testing.T) error {
				counts, err := repo.countJournalEntryTemplateLines(ctx, schemaName, []string{"template-1"})
				assert.Nil(t, counts)
				return err
			},
		},
		{
			name: "CreateJournalEntry",
			run: func(t *testing.T) error {
				return repo.CreateJournalEntry(ctx, schemaName, &JournalEntry{TenantID: tenantID})
			},
		},
		{
			name: "UpdateJournalEntryStatus",
			run: func(t *testing.T) error {
				return repo.UpdateJournalEntryStatus(ctx, schemaName, tenantID, "entry-1", StatusPosted, "user-1")
			},
		},
		{
			name: "GetAccountBalance",
			run: func(t *testing.T) error {
				balance, err := repo.GetAccountBalance(ctx, schemaName, tenantID, "account-1", asOfDate)
				assert.True(t, balance.IsZero())
				return err
			},
		},
		{
			name: "GetTrialBalance",
			run: func(t *testing.T) error {
				balances, err := repo.GetTrialBalance(ctx, schemaName, tenantID, asOfDate)
				assert.Nil(t, balances)
				return err
			},
		},
		{
			name: "GetPeriodBalances",
			run: func(t *testing.T) error {
				balances, err := repo.GetPeriodBalances(ctx, schemaName, tenantID, asOfDate.AddDate(0, -1, 0), asOfDate)
				assert.Nil(t, balances)
				return err
			},
		},
		{
			name: "VoidJournalEntry",
			run: func(t *testing.T) error {
				return repo.VoidJournalEntry(ctx, schemaName, tenantID, "entry-1", "user-1", "duplicate", &JournalEntry{})
			},
		},
		{
			name: "tenantTableAlias",
			run: func(t *testing.T) error {
				db, err := repo.tenantTableAlias(ctx, schemaName, "accounts", "a")
				assert.Nil(t, db)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(t)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "accounting repository database is not configured")
		})
	}
}

func TestGORMRepository_CreateJournalEntryInTxRequiresTransaction(t *testing.T) {
	repo := NewGORMRepository(nil)

	err := repo.createJournalEntryInTx(context.Background(), nil, "tenant_schema", &JournalEntry{TenantID: "tenant-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil gorm DB")
}

func TestGORMAccountModelMappingRoundTrip(t *testing.T) {
	createdAt := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	parentID := "parent-account-id"
	account := &Account{
		ID:          "account-id",
		TenantID:    "tenant-id",
		Code:        "4000",
		Name:        "Sales",
		AccountType: AccountTypeRevenue,
		ParentID:    &parentID,
		IsActive:    true,
		IsSystem:    true,
		Description: "Revenue from product sales",
		CreatedAt:   createdAt,
	}

	model := accountToModel(account)
	assert.Equal(t, models.AccountTypeRevenue, model.AccountType)

	roundTrip := modelToAccount(model)
	assert.Equal(t, account, roundTrip)
}

func TestGORMJournalEntryModelMappingRoundTrip(t *testing.T) {
	sourceID := "source-id"
	postedAt := time.Date(2026, time.February, 3, 4, 5, 6, 0, time.UTC)
	postedBy := "poster-id"
	voidedAt := time.Date(2026, time.February, 4, 5, 6, 7, 0, time.UTC)
	voidedBy := "voider-id"
	createdAt := time.Date(2026, time.February, 1, 2, 3, 4, 0, time.UTC)
	entry := &JournalEntry{
		ID:               "journal-id",
		TenantID:         "tenant-id",
		EntryNumber:      "JE-2026-0001",
		EntryDate:        time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC),
		Description:      "Payroll accrual",
		Reference:        "PAY-2026-02",
		SourceType:       "PAYROLL_IMPORT",
		SourceID:         &sourceID,
		RequiresEvidence: true,
		Status:           StatusVoided,
		PostedAt:         &postedAt,
		PostedBy:         &postedBy,
		VoidedAt:         &voidedAt,
		VoidedBy:         &voidedBy,
		VoidReason:       "Replaced by corrected import",
		CreatedAt:        createdAt,
		CreatedBy:        "creator-id",
	}

	model := journalEntryToModel(entry)
	assert.Equal(t, models.JournalStatusVoided, model.Status)

	roundTrip := modelToJournalEntry(model)
	assert.Equal(t, entry, roundTrip)
}

func TestGORMJournalEntryLineModelMappingRoundTrip(t *testing.T) {
	line := &JournalEntryLine{
		ID:             "line-id",
		TenantID:       "tenant-id",
		JournalEntryID: "journal-id",
		AccountID:      "expense-account-id",
		Description:    "Gross wages",
		DebitAmount:    decimal.RequireFromString("1234.56"),
		CreditAmount:   decimal.Zero,
		Currency:       "USD",
		ExchangeRate:   decimal.RequireFromString("0.9200000000"),
		BaseDebit:      decimal.RequireFromString("1135.7952"),
		BaseCredit:     decimal.Zero,
	}

	model := journalEntryLineToModel(line)
	assert.True(t, model.DebitAmount.Decimal.Equal(line.DebitAmount))
	assert.True(t, model.ExchangeRate.Decimal.Equal(line.ExchangeRate))

	roundTrip := modelToJournalEntryLine(model)
	assert.Equal(t, line, roundTrip)
}

func TestGORMJournalEntryTemplateModelMappingRoundTrip(t *testing.T) {
	startDate := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, time.December, 31, 0, 0, 0, 0, time.UTC)
	nextDate := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	lastGeneratedAt := time.Date(2026, time.March, 1, 8, 30, 0, 0, time.UTC)
	createdAt := time.Date(2026, time.February, 20, 10, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, time.February, 21, 11, 0, 0, 0, time.UTC)
	template := &JournalEntryTemplate{
		ID:                 "template-id",
		TenantID:           "tenant-id",
		Name:               "Monthly payroll accrual",
		Description:        "Recurring payroll accrual",
		Reference:          "PAYROLL-ACCRUAL",
		RequiresEvidence:   true,
		IsActive:           true,
		Frequency:          JournalEntryTemplateFrequencyMonthly,
		StartDate:          &startDate,
		EndDate:            &endDate,
		NextGenerationDate: &nextDate,
		LastGeneratedAt:    &lastGeneratedAt,
		GeneratedCount:     3,
		CreatedAt:          createdAt,
		CreatedBy:          "creator-id",
		UpdatedAt:          updatedAt,
	}

	model := journalEntryTemplateToModel(template)
	assert.Equal(t, string(JournalEntryTemplateFrequencyMonthly), model.Frequency)

	roundTrip := modelToJournalEntryTemplate(model)
	assert.Equal(t, template, roundTrip)
}

func TestGORMJournalEntryTemplateLineModelMappingRoundTrip(t *testing.T) {
	line := &JournalEntryTemplateLine{
		ID:           "template-line-id",
		TemplateID:   "template-id",
		LineNumber:   2,
		AccountID:    "liability-account-id",
		Description:  "Accrued payroll liability",
		DebitAmount:  decimal.Zero,
		CreditAmount: decimal.RequireFromString("1234.56"),
		Currency:     "EUR",
		ExchangeRate: decimal.NewFromInt(1),
	}

	model := journalEntryTemplateLineToModel(line)
	assert.True(t, model.CreditAmount.Decimal.Equal(line.CreditAmount))

	roundTrip := modelToJournalEntryTemplateLine(model)
	assert.Equal(t, line, roundTrip)
}
