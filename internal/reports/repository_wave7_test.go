package reports

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGORMRepositoryWave7PanicsOnGormPoolError(t *testing.T) {
	pool := stubNewGormDBFromPoolError(t, errors.New("pool unavailable"))

	require.PanicsWithError(t, "create reports GORM repository: pool unavailable", func() {
		_ = NewGORMRepository(pool)
	})
}

func TestGORMRepositoryWave7NilDatabaseGuards(t *testing.T) {
	ctx := context.Background()
	repo := NewGORMRepository(nil)
	startDate := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 0, 30)
	tenantID := "tenant-1"
	contactID := "contact-1"

	tests := []struct {
		name string
		run  func(t *testing.T) error
	}{
		{
			name: "GetJournalEntriesForPeriod",
			run: func(t *testing.T) error {
				got, err := repo.GetJournalEntriesForPeriod(ctx, "tenant_reports", tenantID, startDate, endDate)
				assert.Nil(t, got)
				return err
			},
		},
		{
			name: "GetCashAccountBalance",
			run: func(t *testing.T) error {
				got, err := repo.GetCashAccountBalance(ctx, "tenant_reports", tenantID, endDate)
				assert.True(t, got.Equal(decimal.Zero))
				return err
			},
		},
		{
			name: "GetOutstandingInvoicesByContact",
			run: func(t *testing.T) error {
				got, err := repo.GetOutstandingInvoicesByContact(ctx, "tenant_reports", tenantID, "SALES", endDate)
				assert.Nil(t, got)
				return err
			},
		},
		{
			name: "GetContactStatementEntries",
			run: func(t *testing.T) error {
				got, err := repo.GetContactStatementEntries(ctx, "tenant_reports", tenantID, contactID, "SALES", "RECEIVED", startDate, endDate)
				assert.Nil(t, got)
				return err
			},
		},
		{
			name: "GetSalesMarginLines",
			run: func(t *testing.T) error {
				got, err := repo.GetSalesMarginLines(ctx, "tenant_reports", tenantID, startDate, endDate)
				assert.Nil(t, got)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(t)
			require.ErrorContains(t, err, "reports repository database is not configured")
		})
	}
}

func TestMockRepositoryWave7ContactStatementOpeningError(t *testing.T) {
	expectedErr := errors.New("opening failed")
	repo := &MockRepository{GetContactStatementOpeningErr: expectedErr}

	got, err := repo.GetContactStatementOpeningBalance(context.Background(), "tenant_reports", "tenant-1", "contact-1", "SALES", "RECEIVED", time.Now())

	assert.True(t, got.Equal(decimal.Zero))
	assert.ErrorIs(t, err, expectedErr)
}
