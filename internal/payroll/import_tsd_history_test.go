package payroll

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImportTSDHistoryCSV_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMockRepository()
	repo.Employees["emp-existing-1"] = &Employee{
		ID:             "emp-existing-1",
		TenantID:       "tenant-1",
		EmployeeNumber: "EMP-100",
		FirstName:      "Mari",
		LastName:       "Maasikas",
		PersonalCode:   "49001010001",
		Email:          "mari@example.com",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
	repo.Employees["emp-existing-2"] = &Employee{
		ID:             "emp-existing-2",
		TenantID:       "tenant-1",
		EmployeeNumber: "EMP-101",
		FirstName:      "Juhan",
		LastName:       "Tamm",
		PersonalCode:   "49001010002",
		Email:          "juhan@example.com",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
	service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "tsd"})

	result, err := service.ImportTSDHistoryCSV(ctx, "tenant_schema", "tenant-1", &ImportTSDHistoryRequest{
		FileName: "tsd-history.csv",
		CSVContent: "year,month,status,submitted_at,emta_reference,employee_number,gross_payment,basic_exemption,taxable_amount,income_tax,social_tax,unemployment_insurance_employer,unemployment_insurance_employee,funded_pension\n" +
			"2025,12,ACCEPTED,2026-01-10,EMTA-2025-12,EMP-100,3200.00,50.00,3150.00,693.00,1056.00,25.60,51.20,64.00\n" +
			"2025,12,ACCEPTED,2026-01-10,EMTA-2025-12,EMP-101,2800.00,40.00,2760.00,607.20,924.00,22.40,44.80,56.00\n",
	})
	require.NoError(t, err)

	assert.Equal(t, "tsd-history.csv", result.FileName)
	assert.Equal(t, 2, result.RowsProcessed)
	assert.Equal(t, 1, result.DeclarationsCreated)
	assert.Equal(t, 2, result.RowsImported)
	assert.Zero(t, result.RowsSkipped)
	assert.Nil(t, result.Errors)

	require.Len(t, repo.TSDDeclarations, 1)
	declaration := repo.TSDDeclarations["tsd-1"]
	require.NotNil(t, declaration)
	assert.Equal(t, TSDAccepted, declaration.Status)
	assert.Equal(t, "EMTA-2025-12", declaration.EMTAReference)
	assert.True(t, declaration.TotalPayments.Equal(decimal.RequireFromString("6000.00")))
	assert.True(t, declaration.TotalIncomeTax.Equal(decimal.RequireFromString("1300.20")))
	assert.True(t, declaration.TotalSocialTax.Equal(decimal.RequireFromString("1980.00")))

	rows := repo.TSDRows["tsd-1"]
	require.Len(t, rows, 2)
	assert.Equal(t, "tsd-2", rows[0].ID)
	assert.Equal(t, "emp-existing-1", rows[0].EmployeeID)
	assert.Equal(t, "10", rows[0].PaymentType)
	assert.True(t, rows[1].GrossPayment.Equal(decimal.RequireFromString("2800.00")))
}

func TestImportTSDHistoryCSV_SkipsInvalidRowsAndExistingPeriods(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMockRepository()
	repo.Employees["emp-existing-1"] = &Employee{
		ID:             "emp-existing-1",
		TenantID:       "tenant-1",
		EmployeeNumber: "EMP-100",
		FirstName:      "Mari",
		LastName:       "Maasikas",
		PersonalCode:   "49001010001",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
	repo.TSDDeclarations["existing-tsd"] = &TSDDeclaration{
		ID:          "existing-tsd",
		TenantID:    "tenant-1",
		PeriodYear:  2025,
		PeriodMonth: 11,
		Status:      TSDAccepted,
	}
	service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "tsd"})

	result, err := service.ImportTSDHistoryCSV(ctx, "tenant_schema", "tenant-1", &ImportTSDHistoryRequest{
		FileName: "tsd-history.csv",
		CSVContent: "period_year,period_month,status,employee_number,gross_payment,income_tax\n" +
			"2025,11,ACCEPTED,EMP-100,3200.00,693.00\n" +
			"2025,12,ACCEPTED,EMP-404,2800.00,607.20\n" +
			"2025,12,BAD,EMP-100,2800.00,607.20\n" +
			"2025,12,ACCEPTED,EMP-100,2800.00,607.20\n",
	})
	require.NoError(t, err)

	assert.Equal(t, 4, result.RowsProcessed)
	assert.Equal(t, 1, result.DeclarationsCreated)
	assert.Equal(t, 1, result.RowsImported)
	assert.Equal(t, 3, result.RowsSkipped)
	require.Len(t, result.Errors, 3)
	assert.Contains(t, result.Errors[0].Message, "employee_number")
	assert.Contains(t, result.Errors[1].Message, "status must be DRAFT")
	assert.Contains(t, result.Errors[2].Message, "already exists")
}

func TestImportTSDHistoryCSV_RejectsMissingHeaders(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "tsd"})

	_, err := service.ImportTSDHistoryCSV(ctx, "tenant_schema", "tenant-1", &ImportTSDHistoryRequest{
		CSVContent: "year,month,employee_number\n2025,12,EMP-100\n",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required period_year, period_month, or gross_payment column")
}
