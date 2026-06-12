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

	require.NotNil(t, repo)
	assert.Nil(t, repo.db)

	account, err := repo.GetAccountByID(context.Background(), "tenant_schema", "tenant-1", "account-1")
	require.Error(t, err)
	assert.Nil(t, account)
	assert.Contains(t, err.Error(), "accounting repository database is not configured")
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
