package accounting

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestGORMRepositoryWave9TemplateInvalidSchemaStopsBeforeWrites(t *testing.T) {
	repo := NewGORMRepository(newAccountingDryRunDB(t))

	err := repo.CreateJournalEntryTemplate(context.Background(), "tenant-schema", &JournalEntryTemplate{
		TenantID: "tenant-1",
		Name:     "Invalid schema template",
		Lines: []JournalEntryTemplateLine{{
			AccountID:    "account-1",
			DebitAmount:  decimal.NewFromInt(10),
			Currency:     "EUR",
			ExchangeRate: decimal.NewFromInt(1),
		}},
	})

	require.ErrorContains(t, err, "invalid SQL identifier")
}
