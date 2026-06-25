package cutover

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCutoverWave11SortsIssueSeverityFileAndRow(t *testing.T) {
	issues := []ValidationIssue{
		{Severity: SeverityWarning, FileName: "b.csv", Row: 1},
		{Severity: SeverityError, FileName: "b.csv", Row: 2},
		{Severity: SeverityError, FileName: "a.csv", Row: 3},
		{Severity: SeverityError, FileName: "a.csv", Row: 1},
	}

	sortMigrationIssues(issues)

	assert.Equal(t, SeverityError, issues[0].Severity)
	assert.Equal(t, "a.csv", issues[0].FileName)
	assert.Equal(t, 1, issues[0].Row)
	assert.Equal(t, SeverityWarning, issues[3].Severity)
}

func TestCutoverWave11ExecutionRunRepositoryUsesInjectedPoolDB(t *testing.T) {
	expectedDB := newCutoverDryRunDB(t)
	pool := new(pgxpool.Pool)
	original := newMigrationExecutionRunGormDBFromPool
	t.Cleanup(func() {
		newMigrationExecutionRunGormDBFromPool = original
	})
	var called bool
	newMigrationExecutionRunGormDBFromPool = func(ctx context.Context, got *pgxpool.Pool) (*gorm.DB, error) {
		require.NotNil(t, ctx)
		require.Same(t, pool, got)
		called = true
		return expectedDB, nil
	}

	repo := NewMigrationExecutionRunRepository(pool)

	require.True(t, called)
	require.NotNil(t, repo)
	require.Same(t, expectedDB, repo.db)
}

func TestCutoverWave11CrossFileSkipAndInvoiceBranches(t *testing.T) {
	report := &BundleValidationReport{}
	validatePayrollTSDHistoryConsistency(report, []parsedFile{
		wave6ParsedFile(KindPayrollHistory, "payroll.csv", []string{"period_year", "period_month", "employee_number", "gross_salary"},
			map[string]string{"period_year": "2026", "period_month": "5", "employee_number": "EMP-1", "gross_salary": ""},
			map[string]string{"period_year": "2026", "period_month": "5", "employee_number": "EMP-2", "gross_salary": "100"},
		),
		wave6ParsedFile(KindTSDHistory, "tsd.csv", []string{"period_year", "period_month", "employee_number", "gross_payment"},
			map[string]string{"period_year": "2026", "period_month": "5", "employee_number": "", "gross_payment": "100"},
		),
	})
	assert.Empty(t, report.Issues)

	report = &BundleValidationReport{}
	validateImportedInvoiceAmountPaidConsistency(report, []parsedFile{
		wave6ParsedFile(KindInvoices, "invoices.csv", []string{"invoice_number", "invoice_type", "amount_paid", "quantity", "unit_price", "vat_rate"},
			map[string]string{"invoice_number": "INV-1", "invoice_type": "SALES", "amount_paid": "10", "quantity": "1", "unit_price": "10", "vat_rate": "0"},
		),
	})
	assert.Empty(t, report.Issues)

	accountType, ok := cutoverNormalizedAccountType("not-a-type")
	assert.False(t, ok)
	assert.Empty(t, accountType)

	accountType, ok = cutoverNormalizedAccountType("LIABILITY")
	assert.True(t, ok)
	assert.Equal(t, "LIABILITY", accountType)
}
