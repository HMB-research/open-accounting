package payroll

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImportLeaveBalancesCSV_CreatesAndUpdatesBalances(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMockAbsenceRepository()
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
		FirstName:      "Juhan",
		LastName:       "Tamm",
		Email:          "juhan@example.com",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
	repo.AbsenceTypes["type-annual"] = &AbsenceType{
		ID:                 "type-annual",
		TenantID:           "tenant-1",
		Code:               "ANNUAL_LEAVE",
		Name:               "Annual leave",
		NameET:             "Pohipuhkus",
		DefaultDaysPerYear: decimal.NewFromInt(28),
		IsActive:           true,
	}
	repo.LeaveBalances["tenant-1-emp-2-type-annual-2025"] = &LeaveBalance{
		ID:            "balance-existing",
		TenantID:      "tenant-1",
		EmployeeID:    "emp-2",
		AbsenceTypeID: "type-annual",
		Year:          2025,
		EntitledDays:  decimal.NewFromInt(28),
		CarryoverDays: decimal.Zero,
		UsedDays:      decimal.Zero,
		PendingDays:   decimal.Zero,
		RemainingDays: decimal.NewFromInt(28),
	}
	service := NewAbsenceService(repo, &MockUUIDGenerator{prefix: "leave"})

	result, err := service.ImportLeaveBalancesCSV(ctx, "tenant_schema", "tenant-1", &ImportLeaveBalancesRequest{
		FileName: "leave-balances.csv",
		CSVContent: "year,employee_number,absence_type_code,entitled_days,carryover_days,used_days,pending_days,notes\n" +
			"2025,EMP-100,ANNUAL_LEAVE,28,3,10,2,Opening balance\n" +
			"2025,EMP-101,ANNUAL_LEAVE,28,1,6,0,Updated balance\n",
	})
	require.NoError(t, err)

	assert.Equal(t, "leave-balances.csv", result.FileName)
	assert.Equal(t, 2, result.RowsProcessed)
	assert.Equal(t, 1, result.LeaveBalancesCreated)
	assert.Equal(t, 1, result.LeaveBalancesUpdated)
	assert.Zero(t, result.RowsSkipped)
	assert.Nil(t, result.Errors)

	created := repo.LeaveBalances["tenant-1-emp-1-type-annual-2025"]
	require.NotNil(t, created)
	assert.Equal(t, "leave-1", created.ID)
	assert.True(t, created.RemainingDays.Equal(decimal.NewFromInt(19)))
	assert.Equal(t, "Opening balance", created.Notes)

	updated := repo.LeaveBalances["tenant-1-emp-2-type-annual-2025"]
	require.NotNil(t, updated)
	assert.Equal(t, "balance-existing", updated.ID)
	assert.True(t, updated.RemainingDays.Equal(decimal.NewFromInt(23)))
	assert.Equal(t, "Updated balance", updated.Notes)
}

func TestImportLeaveBalancesCSV_SkipsInvalidRows(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMockAbsenceRepository()
	repo.Employees["emp-1"] = &Employee{
		ID:             "emp-1",
		TenantID:       "tenant-1",
		EmployeeNumber: "EMP-100",
		FirstName:      "Mari",
		LastName:       "Maasikas",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
	repo.AbsenceTypes["type-annual"] = &AbsenceType{
		ID:                 "type-annual",
		TenantID:           "tenant-1",
		Code:               "ANNUAL_LEAVE",
		Name:               "Annual leave",
		DefaultDaysPerYear: decimal.NewFromInt(28),
		IsActive:           true,
	}
	service := NewAbsenceService(repo, &MockUUIDGenerator{prefix: "leave"})

	result, err := service.ImportLeaveBalancesCSV(ctx, "tenant_schema", "tenant-1", &ImportLeaveBalancesRequest{
		CSVContent: "year,employee_number,absence_type_code,entitled_days\n" +
			"2025,EMP-999,ANNUAL_LEAVE,28\n" +
			"2025,EMP-100,SICK_LEAVE,10\n" +
			"2025,EMP-100,ANNUAL_LEAVE,-1\n",
	})
	require.NoError(t, err)

	assert.Equal(t, 3, result.RowsProcessed)
	assert.Zero(t, result.LeaveBalancesCreated)
	assert.Zero(t, result.LeaveBalancesUpdated)
	assert.Equal(t, 3, result.RowsSkipped)
	require.Len(t, result.Errors, 3)
	assert.Contains(t, result.Errors[0].Message, "employee_number \"EMP-999\" not found")
	assert.Contains(t, result.Errors[1].Message, "absence_type_code \"SICK_LEAVE\" not found")
	assert.Contains(t, result.Errors[2].Message, "entitled_days must be zero or greater")
}

func TestImportLeaveBalancesCSV_RejectsMissingHeaders(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMockAbsenceRepository()
	service := NewAbsenceService(repo, &MockUUIDGenerator{prefix: "leave"})

	_, err := service.ImportLeaveBalancesCSV(ctx, "tenant_schema", "tenant-1", &ImportLeaveBalancesRequest{
		CSVContent: "employee_number,entitled_days\nEMP-100,28\n",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required year column")
}
