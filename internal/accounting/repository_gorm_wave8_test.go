package accounting

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGORMRepositoryWave8TemplateAndEntryWriteErrors(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_schema"
	expectedErr := errors.New("wave8 write failed")

	t.Run("template line create error", func(t *testing.T) {
		repo := NewGORMRepository(newAccountingDryRunDB(t, withAccountingWave8CreateErrorOnCall(2, expectedErr)))

		err := repo.CreateJournalEntryTemplate(ctx, schemaName, &JournalEntryTemplate{
			TenantID: "tenant-1",
			Name:     "Accrual",
			Lines: []JournalEntryTemplateLine{{
				AccountID:    "account-1",
				DebitAmount:  decimal.NewFromInt(10),
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
			}},
		})

		require.ErrorContains(t, err, "insert journal entry template line")
		assert.ErrorIs(t, err, expectedErr)
	})
}

func TestGORMRepositoryWave8BaseGormErrors(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_schema"
	tenantID := "tenant-1"
	templateID := "template-1"
	entryID := "entry-1"
	asOfDate := time.Date(2026, time.June, 30, 0, 0, 0, 0, time.UTC)
	expectedErr := errors.New("wave8 gorm failure")
	repo := newAccountingWave8ErrorRepository(t, expectedErr)

	templates, err := repo.ListJournalEntryTemplates(ctx, schemaName, tenantID, true)
	assert.Nil(t, templates)
	require.ErrorContains(t, err, "list journal entry templates")
	assert.ErrorIs(t, err, expectedErr)

	template, err := repo.GetJournalEntryTemplateByID(ctx, schemaName, tenantID, templateID)
	assert.Nil(t, template)
	require.ErrorContains(t, err, "get journal entry template")
	assert.ErrorIs(t, err, expectedErr)

	dueIDs, err := repo.GetDueJournalEntryTemplateIDs(ctx, schemaName, tenantID, asOfDate)
	assert.Nil(t, dueIDs)
	require.ErrorContains(t, err, "list due journal entry templates")
	assert.ErrorIs(t, err, expectedErr)

	err = repo.UpdateJournalEntryTemplateAfterGeneration(ctx, schemaName, tenantID, templateID, asOfDate, asOfDate)
	require.ErrorContains(t, err, "update journal entry template after generation")
	assert.ErrorIs(t, err, expectedErr)

	err = repo.UpdateJournalEntryStatus(ctx, schemaName, tenantID, entryID, StatusPosted, "user-1")
	require.ErrorContains(t, err, "update journal entry status")
	assert.ErrorIs(t, err, expectedErr)

	trialBalance, err := repo.GetTrialBalance(ctx, schemaName, tenantID, asOfDate)
	assert.Nil(t, trialBalance)
	require.ErrorContains(t, err, "get trial balance")
	assert.ErrorIs(t, err, expectedErr)

	periodBalances, err := repo.GetPeriodBalances(ctx, schemaName, tenantID, asOfDate.AddDate(0, -1, 0), asOfDate)
	assert.Nil(t, periodBalances)
	require.ErrorContains(t, err, "get period balances")
	assert.ErrorIs(t, err, expectedErr)
}

func TestGORMRepositoryWave8TemplateLineQueryError(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("template lines failed")
	templateModel := models.JournalEntryTemplate{
		ID:        "template-1",
		TenantID:  "tenant-1",
		Name:      "Template",
		IsActive:  true,
		CreatedAt: time.Date(2026, time.June, 25, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, time.June, 25, 0, 0, 0, 0, time.UTC),
	}
	repo := NewGORMRepository(newAccountingDryRunDB(t,
		withAccountingDryRunFixtures(accountingDryRunFixture{templates: []models.JournalEntryTemplate{templateModel}}),
		withAccountingWave8QueryErrors(nil, expectedErr),
	))

	template, err := repo.GetJournalEntryTemplateByID(ctx, "tenant_schema", "tenant-1", "template-1")

	assert.Nil(t, template)
	require.ErrorContains(t, err, "get journal entry template lines")
	assert.ErrorIs(t, err, expectedErr)
}

func newAccountingWave8ErrorRepository(t *testing.T, expectedErr error) *GORMRepository {
	t.Helper()

	db := newAccountingDryRunDB(t)
	db.AddError(expectedErr)
	return NewGORMRepository(db)
}

func withAccountingWave8CreateErrorOnCall(call int, expectedErr error) accountingDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var calls int
		err := db.Callback().Create().Before("gorm:create").Register(accountingDryRunCallbackName("create_error_wave8"), func(tx *gorm.DB) {
			calls++
			if calls == call {
				tx.AddError(expectedErr)
			}
		})
		require.NoError(t, err)
	}
}

func withAccountingWave8QueryErrors(queryErrors ...error) accountingDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Query().Before("gorm:query").Register(accountingDryRunCallbackName("query_error_wave8"), func(tx *gorm.DB) {
			if len(queryErrors) == 0 {
				return
			}
			errIndex := index
			if errIndex >= len(queryErrors) {
				errIndex = len(queryErrors) - 1
			}
			index++
			if queryErrors[errIndex] != nil {
				tx.AddError(queryErrors[errIndex])
			}
		})
		require.NoError(t, err)
	}
}
