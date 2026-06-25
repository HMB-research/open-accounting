package accounting

import (
	"context"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGORMRepositoryWave5ListJournalEntryTemplatesAppliesLineCounts(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.June, 25, 10, 0, 0, 0, time.UTC)
	repo := NewGORMRepository(newAccountingDryRunDB(t,
		withAccountingDryRunFixtures(accountingDryRunFixture{
			templates: []models.JournalEntryTemplate{
				{
					ID:        "template-2",
					TenantID:  "tenant-1",
					Name:      "Utilities",
					IsActive:  true,
					CreatedAt: now,
					UpdatedAt: now,
				},
				{
					ID:        "template-1",
					TenantID:  "tenant-1",
					Name:      "Rent",
					IsActive:  true,
					CreatedAt: now,
					UpdatedAt: now,
				},
			},
		}),
		withAccountingDryRunScanRowsWave4(accountingDryRunRowSetWave4{
			columns: []string{"template_id", "line_count"},
			values: [][]driver.Value{
				{"template-1", int64(2)},
				{"template-2", int64(4)},
			},
		}),
	))

	templates, err := repo.ListJournalEntryTemplates(ctx, "tenant_schema", "tenant-1", true)

	require.NoError(t, err)
	require.Len(t, templates, 2)
	assert.Equal(t, "template-2", templates[0].ID)
	assert.Equal(t, 4, templates[0].LineCount)
	assert.Equal(t, "template-1", templates[1].ID)
	assert.Equal(t, 2, templates[1].LineCount)
}

func TestGORMRepositoryWave5CountJournalEntryTemplateLinesEmptyInput(t *testing.T) {
	repo := NewGORMRepository(newAccountingDryRunDB(t))

	counts, err := repo.countJournalEntryTemplateLines(context.Background(), "tenant_schema", nil)

	require.NoError(t, err)
	assert.Empty(t, counts)
}
