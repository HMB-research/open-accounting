package payroll

import (
	"context"
	"errors"
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
		CSVContent: "year,month,status,submitted_at,emta_reference,employee_number,name,gross_payment,basic_exemption,taxable_amount,income_tax,social_tax,unemployment_insurance_employer,unemployment_insurance_employee,funded_pension\n" +
			"2025,12,ACCEPTED,2026-01-10,EMTA-2025-12,EMP-100,,3200.00,50.00,3150.00,693.00,1056.00,25.60,51.20,64.00\n" +
			"2025,12,ACCEPTED,2026-01-10,EMTA-2025-12,,Juhan Tamm,2800.00,40.00,2760.00,607.20,924.00,22.40,44.80,56.00\n",
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

func TestImportTSDHistoryCSV_MatchesAlternateEmployeeIdentifiersAndDerivesAmounts(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMockRepository()
	repo.Employees["emp-personal"] = &Employee{
		ID:             "emp-personal",
		TenantID:       "tenant-1",
		EmployeeNumber: "EMP-200",
		FirstName:      "Mari",
		LastName:       "Maasikas",
		PersonalCode:   "49001010001",
		Email:          "mari@example.com",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
	repo.Employees["emp-email"] = &Employee{
		ID:             "emp-email",
		TenantID:       "tenant-1",
		EmployeeNumber: "EMP-201",
		FirstName:      "Liis",
		LastName:       "Lepp",
		PersonalCode:   "49001010002",
		Email:          "liis@example.com",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
	repo.Employees["emp-name"] = &Employee{
		ID:             "emp-name",
		TenantID:       "tenant-1",
		EmployeeNumber: "EMP-202",
		FirstName:      "Kati",
		LastName:       "Kask",
		PersonalCode:   "49001010003",
		Email:          "kati@example.com",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
	service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "tsd"})

	result, err := service.ImportTSDHistoryCSV(ctx, "tenant_schema", "tenant-1", &ImportTSDHistoryRequest{
		FileName: "tsd-history.csv",
		CSVContent: "year,month,status,personal_code,email,first_name,last_name,payment_code,gross_salary,basic_exemption,income_tax,social_tax,unemployment_insurance_employer,unemployment_insurance_employee,funded_pension\n" +
			"2025,8,DRAFT,49001010001,,,,13,3000.00,50.00,600.00,990.00,24.00,48.00,60.00\n" +
			"2025,8,DRAFT,,liis@example.com,,,10,2500.00,40.00,500.00,825.00,20.00,40.00,50.00\n" +
			"2025,8,DRAFT,,,Kati,Kask,,2000.00,30.00,400.00,660.00,16.00,32.00,40.00\n",
	})
	require.NoError(t, err)

	assert.Equal(t, 3, result.RowsProcessed)
	assert.Equal(t, 1, result.DeclarationsCreated)
	assert.Equal(t, 3, result.RowsImported)
	assert.Zero(t, result.RowsSkipped)
	assert.Nil(t, result.Errors)

	declaration := repo.TSDDeclarations["tsd-1"]
	require.NotNil(t, declaration)
	assert.Equal(t, TSDDraft, declaration.Status)
	assert.Nil(t, declaration.SubmittedAt)
	assert.True(t, declaration.TotalPayments.Equal(decimal.RequireFromString("7500.00")))
	assert.True(t, declaration.TotalIncomeTax.Equal(decimal.RequireFromString("1500.00")))
	assert.True(t, declaration.TotalSocialTax.Equal(decimal.RequireFromString("2475.00")))
	assert.True(t, declaration.TotalUnemploymentER.Equal(decimal.RequireFromString("60.00")))
	assert.True(t, declaration.TotalUnemploymentEE.Equal(decimal.RequireFromString("120.00")))
	assert.True(t, declaration.TotalFundedPension.Equal(decimal.RequireFromString("150.00")))

	rows := repo.TSDRows["tsd-1"]
	require.Len(t, rows, 3)
	assert.Equal(t, "emp-personal", rows[0].EmployeeID)
	assert.Equal(t, "13", rows[0].PaymentType)
	assert.True(t, rows[0].TaxableAmount.Equal(decimal.RequireFromString("2950.00")))
	assert.Equal(t, "emp-email", rows[1].EmployeeID)
	assert.Equal(t, "emp-name", rows[2].EmployeeID)
	assert.Equal(t, "10", rows[2].PaymentType)
}

func TestImportTSDHistoryCSV_RejectsMismatchedIdentifiersAndGroupInconsistency(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMockRepository()
	repo.Employees["emp-1"] = &Employee{
		ID:             "emp-1",
		TenantID:       "tenant-1",
		EmployeeNumber: "EMP-100",
		FirstName:      "Mari",
		LastName:       "Maasikas",
		PersonalCode:   "49001010001",
		Email:          "mari@example.com",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
	repo.Employees["emp-2"] = &Employee{
		ID:             "emp-2",
		TenantID:       "tenant-1",
		EmployeeNumber: "EMP-101",
		FirstName:      "Liis",
		LastName:       "Lepp",
		PersonalCode:   "49001010002",
		Email:          "liis@example.com",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
	service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "tsd"})

	result, err := service.ImportTSDHistoryCSV(ctx, "tenant_schema", "tenant-1", &ImportTSDHistoryRequest{
		CSVContent: "year,month,status,submitted_at,emta_reference,employee_number,email,gross_payment\n" +
			"2025,7,SUBMITTED,2025-08-10,EMTA-1,EMP-100,,1000.00\n" +
			"2025,7,SUBMITTED,2025-08-10,EMTA-1,EMP-100,,900.00\n" +
			"2025,7,SUBMITTED,2025-08-10,EMTA-1,EMP-100,liis@example.com,800.00\n" +
			"2025,7,ACCEPTED,2025-08-10,EMTA-1,EMP-101,,700.00\n" +
			"2025,7,SUBMITTED,2025-08-11,EMTA-1,EMP-101,,600.00\n" +
			"2025,7,SUBMITTED,2025-08-10,EMTA-2,EMP-101,,500.00\n",
	})
	require.NoError(t, err)

	assert.Equal(t, 6, result.RowsProcessed)
	assert.Equal(t, 1, result.DeclarationsCreated)
	assert.Equal(t, 1, result.RowsImported)
	assert.Equal(t, 5, result.RowsSkipped)
	require.Len(t, result.Errors, 5)
	assert.Contains(t, result.Errors[0].Message, "employee already has a TSD row in this period")
	assert.Contains(t, result.Errors[1].Message, "employee identifiers do not match the same employee")
	assert.Contains(t, result.Errors[2].Message, "status must be consistent for each TSD period")
	assert.Contains(t, result.Errors[3].Message, "submitted_at must be consistent for each TSD period")
	assert.Contains(t, result.Errors[4].Message, "emta_reference must be consistent for each TSD period")
}

func TestImportTSDHistoryCSV_ReportsTransactionErrorsPerGroup(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMockRepository()
	repo.Employees["emp-1"] = &Employee{
		ID:             "emp-1",
		TenantID:       "tenant-1",
		EmployeeNumber: "EMP-100",
		FirstName:      "Mari",
		LastName:       "Maasikas",
		PersonalCode:   "49001010001",
		Email:          "mari@example.com",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
	repo.Employees["emp-2"] = &Employee{
		ID:             "emp-2",
		TenantID:       "tenant-1",
		EmployeeNumber: "EMP-101",
		FirstName:      "Liis",
		LastName:       "Lepp",
		PersonalCode:   "49001010002",
		Email:          "liis@example.com",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
	repo.CreateTSDRowsErr = errors.New("write failed")
	service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "tsd"})

	result, err := service.ImportTSDHistoryCSV(ctx, "tenant_schema", "tenant-1", &ImportTSDHistoryRequest{
		CSVContent: "year,month,status,employee_number,gross_payment\n" +
			"2025,6,DRAFT,EMP-100,1000.00\n" +
			"2025,6,DRAFT,EMP-101,900.00\n",
	})
	require.NoError(t, err)

	assert.Zero(t, result.DeclarationsCreated)
	assert.Zero(t, result.RowsImported)
	assert.Equal(t, 2, result.RowsSkipped)
	require.Len(t, result.Errors, 2)
	assert.Contains(t, result.Errors[0].Message, "create TSD rows: write failed")
	assert.Contains(t, result.Errors[1].Message, "create TSD rows: write failed")
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
