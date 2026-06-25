package recurring

import (
	"context"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestRecurringWave9ConstructorPanicsForUnreachablePool(t *testing.T) {
	pool := recurringWave9UnreachablePool(t)
	defer pool.Close()

	require.Panics(t, func() {
		_ = NewService(pool, nil, nil, nil, nil, nil)
	})
}

func TestRecurringWave9ImportParserEdges(t *testing.T) {
	t.Run("header parse error", func(t *testing.T) {
		_, err := parseRecurringImportRows(`"unterminated`)
		require.ErrorContains(t, err, "parse csv header")
	})

	t.Run("uppercase frequency candidate", func(t *testing.T) {
		frequency, err := parseRecurringImportFrequency("QUARTERLY")
		require.NoError(t, err)
		require.Equal(t, FrequencyQuarterly, frequency)
	})
}

func TestRecurringWave9ImportRecordsTemplateValidationErrors(t *testing.T) {
	service := NewServiceWithDependencies(NewMockRepository(), nil, nil, nil, nil, nil)

	result, err := service.ImportCSV(context.Background(), "tenant-1", "tenant_demo", []contacts.Contact{{
		ID:   "contact-1",
		Code: "CUST-1",
	}}, nil, &ImportRecurringInvoicesRequest{
		CSVContent: "name,contact_code,frequency,start_date,end_date,line_description,quantity,unit_price,vat_rate\n" +
			"Backwards Retainer,CUST-1,MONTHLY,2026-03-01,2026-02-01,Consulting,1,100,22\n",
	})

	require.NoError(t, err)
	require.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	require.Contains(t, result.Errors[0].Message, "end date cannot be before start date")
}

func recurringWave9UnreachablePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	config, err := pgxpool.ParseConfig("postgres://open_accounting:open_accounting@127.0.0.1:1/open_accounting?sslmode=disable")
	require.NoError(t, err)
	config.ConnConfig.ConnectTimeout = 10 * time.Millisecond
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	require.NoError(t, err)
	return pool
}
