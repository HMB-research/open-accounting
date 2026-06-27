package demo

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var dryRunCallbackID uint64

func TestGormStatusReaderRejectsMissingConfiguration(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name   string
		reader *gormStatusReader
	}{
		{name: "nil receiver"},
		{name: "nil db", reader: &gormStatusReader{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, err := tt.reader.ReadDemoStatus(ctx, "tenant_demo1", 1)

			require.ErrorContains(t, err, "demo status reader is not configured")
			assert.Equal(t, StatusResponse{User: 1}, status)
		})
	}
}

func TestGormStatusReaderRejectsInvalidIdentifiersBeforeQuery(t *testing.T) {
	reader := &gormStatusReader{db: newDryRunGormDB(t)}

	status, err := reader.entityStatus(context.Background(), "tenant-demo-invalid", "accounts", "name")

	require.ErrorContains(t, err, `invalid SQL identifier "tenant-demo-invalid"`)
	assert.Equal(t, EntityStatus{}, status)

	status, err = reader.employeeStatus(context.Background(), "tenant-demo-invalid")
	require.ErrorContains(t, err, `invalid SQL identifier "tenant-demo-invalid"`)
	assert.Equal(t, EntityStatus{}, status)

	status, err = reader.periodStatus(context.Background(), "tenant-demo-invalid", "payroll_runs")
	require.ErrorContains(t, err, `invalid SQL identifier "tenant-demo-invalid"`)
	assert.Equal(t, EntityStatus{}, status)
}

func TestGormStatusReaderTenantTableRejectsMissingConfiguration(t *testing.T) {
	var reader *gormStatusReader

	db, err := reader.tenantTable(context.Background(), "tenant_demo1", "accounts")

	require.ErrorContains(t, err, "demo status reader is not configured")
	assert.Nil(t, db)
}

func TestGormStatusReaderReadDemoStatusWrapsFirstEntityError(t *testing.T) {
	reader := &gormStatusReader{db: newDryRunGormDB(t)}

	status, err := reader.ReadDemoStatus(context.Background(), "tenant-demo-invalid", 2)

	require.ErrorContains(t, err, "read accounts status")
	require.ErrorContains(t, err, `invalid SQL identifier "tenant-demo-invalid"`)
	assert.Equal(t, StatusResponse{User: 2}, status)
}

func TestGormStatusReaderReadDemoStatusWithDryRunQueries(t *testing.T) {
	reader := newDryRunStatusReader(t, demoStatusDryRunFixture{
		count: 2,
		keys:  []string{"A-key", "B-key"},
		employees: []employeeStatusRow{
			{FirstName: "Mari", LastName: "Maasikas"},
			{FirstName: "Jaan", LastName: ""},
		},
		periods: []periodStatusRow{
			{PeriodYear: 2026, PeriodMonth: 1},
			{PeriodYear: 2026, PeriodMonth: 11},
		},
	})

	status, err := reader.ReadDemoStatus(context.Background(), "tenant_demo1", 4)

	require.NoError(t, err)
	assert.Equal(t, 4, status.User)
	assert.Equal(t, EntityStatus{Count: 2, Keys: []string{"A-key", "B-key"}}, status.Accounts)
	assert.Equal(t, EntityStatus{Count: 2, Keys: []string{"A-key", "B-key"}}, status.Contacts)
	assert.Equal(t, EntityStatus{Count: 2, Keys: []string{"Mari Maasikas", "Jaan"}}, status.Employees)
	assert.Equal(t, EntityStatus{Count: 2, Keys: []string{"2026-01", "2026-11"}}, status.PayrollRuns)
	assert.Equal(t, EntityStatus{Count: 2, Keys: []string{"2026-01", "2026-11"}}, status.TsdDeclarations)
}

func TestGormStatusReaderEntityStatusToleratesQueryErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("count error returns empty status", func(t *testing.T) {
		reader := newDryRunStatusReader(t, demoStatusDryRunFixture{
			countErr: errors.New("count failed"),
		})

		status, err := reader.entityStatus(ctx, "tenant_demo1", "accounts", "name")

		require.NoError(t, err)
		assert.Equal(t, EntityStatus{}, status)
	})

	t.Run("key query error keeps count", func(t *testing.T) {
		reader := newDryRunStatusReader(t, demoStatusDryRunFixture{
			count:   7,
			rowsErr: errors.New("pluck failed"),
		})

		status, err := reader.entityStatus(ctx, "tenant_demo1", "accounts", "name")

		require.NoError(t, err)
		assert.Equal(t, EntityStatus{Count: 7}, status)
	})
}

func TestGormStatusReaderEmployeeAndPeriodStatusTolerateQueryErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("employee count error returns empty status", func(t *testing.T) {
		reader := newDryRunStatusReader(t, demoStatusDryRunFixture{
			countErr: errors.New("employee count failed"),
		})

		status, err := reader.employeeStatus(ctx, "tenant_demo1")

		require.NoError(t, err)
		assert.Equal(t, EntityStatus{}, status)
	})

	t.Run("employee rows error keeps count", func(t *testing.T) {
		reader := newDryRunStatusReader(t, demoStatusDryRunFixture{
			count:   4,
			rowsErr: errors.New("employee rows failed"),
		})

		status, err := reader.employeeStatus(ctx, "tenant_demo1")

		require.NoError(t, err)
		assert.Equal(t, EntityStatus{Count: 4}, status)
	})

	t.Run("period count error returns empty status", func(t *testing.T) {
		reader := newDryRunStatusReader(t, demoStatusDryRunFixture{
			countErr: errors.New("period count failed"),
		})

		status, err := reader.periodStatus(ctx, "tenant_demo1", "payroll_runs")

		require.NoError(t, err)
		assert.Equal(t, EntityStatus{}, status)
	})

	t.Run("period rows error keeps count", func(t *testing.T) {
		reader := newDryRunStatusReader(t, demoStatusDryRunFixture{
			count:   6,
			rowsErr: errors.New("period rows failed"),
		})

		status, err := reader.periodStatus(ctx, "tenant_demo1", "payroll_runs")

		require.NoError(t, err)
		assert.Equal(t, EntityStatus{Count: 6}, status)
	})
}

func TestGormStatusReaderFormatsEmployeeAndPeriodKeys(t *testing.T) {
	reader := newDryRunStatusReader(t, demoStatusDryRunFixture{
		count: 3,
		employees: []employeeStatusRow{
			{FirstName: "  Mari", LastName: "Maasikas  "},
			{FirstName: "Jaan", LastName: ""},
			{FirstName: "", LastName: "Tamm"},
		},
		periods: []periodStatusRow{
			{PeriodYear: 2025, PeriodMonth: 12},
			{PeriodYear: 2026, PeriodMonth: 3},
		},
	})

	employees, err := reader.employeeStatus(context.Background(), "tenant_demo1")
	require.NoError(t, err)
	assert.Equal(t, EntityStatus{
		Count: 3,
		Keys:  []string{"Mari Maasikas", "Jaan", "Tamm"},
	}, employees)

	periods, err := reader.periodStatus(context.Background(), "tenant_demo1", "payroll_runs")
	require.NoError(t, err)
	assert.Equal(t, EntityStatus{
		Count: 3,
		Keys:  []string{"2025-12", "2026-03"},
	}, periods)
}

type demoStatusDryRunFixture struct {
	count     int64
	keys      []string
	employees []employeeStatusRow
	periods   []periodStatusRow
	countErr  error
	rowsErr   error
}

func newDryRunStatusReader(t *testing.T, fixture demoStatusDryRunFixture) *gormStatusReader {
	t.Helper()

	db := newDryRunGormDB(t)
	callbackName := fmt.Sprintf("demo_status_unit:populate_%d", atomic.AddUint64(&dryRunCallbackID, 1))
	err := db.Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		switch dest := tx.Statement.Dest.(type) {
		case *int64:
			if fixture.countErr != nil {
				tx.AddError(fixture.countErr)
				return
			}
			*dest = fixture.count
			tx.RowsAffected = 1
		case *[]string:
			if fixture.rowsErr != nil {
				tx.AddError(fixture.rowsErr)
				return
			}
			*dest = append((*dest)[:0], fixture.keys...)
		case *[]employeeStatusRow:
			if fixture.rowsErr != nil {
				tx.AddError(fixture.rowsErr)
				return
			}
			*dest = append((*dest)[:0], fixture.employees...)
		case *[]periodStatusRow:
			if fixture.rowsErr != nil {
				tx.AddError(fixture.rowsErr)
				return
			}
			*dest = append((*dest)[:0], fixture.periods...)
		}
	})
	require.NoError(t, err)

	return &gormStatusReader{db: db}
}

func newDryRunGormDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  "host=127.0.0.1 user=open_accounting_test dbname=open_accounting_test sslmode=disable",
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		DryRun:                 true,
		DisableAutomaticPing:   true,
		SkipDefaultTransaction: true,
	})
	require.NoError(t, err)
	return db
}
