package tax

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTaxWave6QueryVATDataWrapsReverseChargeScanError(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("reverse charge scan failed")
	repo := NewGORMRepository(newTaxDryRunDB(t, withTaxWave6ScanRowsThenError(taxDryRunRowSetWave5{
		columns: []string{"vat_rate", "is_output", "tax_base", "tax_amount"},
	}, expectedErr)))

	rows, err := repo.QueryVATData(
		ctx,
		"tenant_tax",
		"tenant-1",
		time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.June, 30, 0, 0, 0, 0, time.UTC),
	)

	require.Nil(t, rows)
	require.ErrorContains(t, err, "query reverse charge VAT data")
	assert.ErrorIs(t, err, expectedErr)
}

func withTaxWave6ScanRowsThenError(first taxDryRunRowSetWave5, secondErr error) taxDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Row().After("gorm:row").Register(taxDryRunCallbackName("wave6_scan_rows_then_error"), func(tx *gorm.DB) {
			switch index {
			case 0:
				tx.Statement.Dest = newTaxDryRunSQLRowsWave5(t, first)
				tx.RowsAffected = int64(len(first.values))
			case 1:
				tx.AddError(secondErr)
			default:
				tx.AddError(errors.New("unexpected extra tax wave6 scan"))
			}
			index++
		})
		require.NoError(t, err)
	}
}
