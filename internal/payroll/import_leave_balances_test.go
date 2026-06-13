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

func TestImportLeaveBalancesCSV_RejectsNilOrEmptyRequest(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service := NewAbsenceService(NewMockAbsenceRepository(), &MockUUIDGenerator{prefix: "leave"})

	tests := []struct {
		name string
		req  *ImportLeaveBalancesRequest
	}{
		{name: "nil request"},
		{name: "empty content", req: &ImportLeaveBalancesRequest{CSVContent: " \n\t "}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.ImportLeaveBalancesCSV(ctx, "tenant_schema", "tenant-1", tt.req)

			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "csv_content is required")
		})
	}
}

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
		CSVContent: "year,employee_number,name,absence_type_code,entitled_days,carryover_days,used_days,pending_days,notes\n" +
			"2025,EMP-100,,ANNUAL_LEAVE,28,3,10,2,Opening balance\n" +
			"2025,,Juhan Tamm,ANNUAL_LEAVE,28,1,6,0,Updated balance\n",
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

func TestImportLeaveBalancesCSV_AcceptsMigrationHeaderAliases(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMockAbsenceRepository()
	repo.Employees["emp-400"] = &Employee{
		ID:             "emp-400",
		TenantID:       "tenant-1",
		EmployeeNumber: "EMP-400",
		FirstName:      "Marta",
		LastName:       "Mets",
		PersonalCode:   "49001010400",
		Email:          "marta@example.com",
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
		FileName: "leave-balances-aliases.csv",
		CSVContent: "period_year,employee_no,leave_type_code,entitlement,carried_forward_days,taken_days,reserved_days,notes\n" +
			"2025,EMP-400,ANNUAL_LEAVE,28,4,6,1,Alias opening balance\n",
	})
	require.NoError(t, err)

	assert.Equal(t, "leave-balances-aliases.csv", result.FileName)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Equal(t, 1, result.LeaveBalancesCreated)
	assert.Zero(t, result.LeaveBalancesUpdated)
	assert.Zero(t, result.RowsSkipped)
	assert.Nil(t, result.Errors)

	balance := repo.LeaveBalances["tenant-1-emp-400-type-annual-2025"]
	require.NotNil(t, balance)
	assert.Equal(t, "leave-1", balance.ID)
	assert.True(t, balance.EntitledDays.Equal(decimal.NewFromInt(28)))
	assert.True(t, balance.CarryoverDays.Equal(decimal.NewFromInt(4)))
	assert.True(t, balance.UsedDays.Equal(decimal.NewFromInt(6)))
	assert.True(t, balance.PendingDays.Equal(decimal.NewFromInt(1)))
	assert.True(t, balance.RemainingDays.Equal(decimal.NewFromInt(25)))
	assert.Equal(t, "Alias opening balance", balance.Notes)
}

func TestImportLeaveBalancesCSV_MatchesAbsenceTypesByIDAndNameWithDefaults(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMockAbsenceRepository()
	annualTypeID := "11111111-1111-4111-8111-111111111111"
	sickTypeID := "22222222-2222-4222-8222-222222222222"
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
	repo.AbsenceTypes[annualTypeID] = &AbsenceType{
		ID:                 annualTypeID,
		TenantID:           "tenant-1",
		Code:               "ANNUAL_LEAVE",
		Name:               "Annual leave",
		NameET:             "Pohipuhkus",
		DefaultDaysPerYear: decimal.NewFromInt(28),
		IsActive:           true,
	}
	repo.AbsenceTypes[sickTypeID] = &AbsenceType{
		ID:                 sickTypeID,
		TenantID:           "tenant-1",
		Code:               "SICK_LEAVE",
		Name:               "Sick leave",
		NameET:             "Haigusleht",
		DefaultDaysPerYear: decimal.NewFromInt(10),
		IsActive:           true,
	}
	service := NewAbsenceService(repo, &MockUUIDGenerator{prefix: "leave"})

	result, err := service.ImportLeaveBalancesCSV(ctx, "tenant_schema", "tenant-1", &ImportLeaveBalancesRequest{
		FileName: "leave-balances.csv",
		CSVContent: "year,personal_code,email,first_name,last_name,absence_type_id,absence_type,entitled_days,carryover_days,used_days,pending_days,notes\n" +
			"2025,49001010001,,,," + annualTypeID + ",,,2,4,1,Matched by type id\n" +
			"2025,,liis@example.com,,, ,Sick leave,12,1,2,0,Matched by English name\n" +
			"2025,,,Kati,Kask,,Haigusleht,,0,1,1,Matched by Estonian name\n",
	})
	require.NoError(t, err)

	assert.Equal(t, 3, result.RowsProcessed)
	assert.Equal(t, 3, result.LeaveBalancesCreated)
	assert.Zero(t, result.LeaveBalancesUpdated)
	assert.Zero(t, result.RowsSkipped)
	assert.Nil(t, result.Errors)

	annual := repo.LeaveBalances["tenant-1-emp-personal-"+annualTypeID+"-2025"]
	require.NotNil(t, annual)
	assert.True(t, annual.EntitledDays.Equal(decimal.NewFromInt(28)))
	assert.True(t, annual.RemainingDays.Equal(decimal.NewFromInt(25)))
	assert.Equal(t, "Matched by type id", annual.Notes)

	sickByName := repo.LeaveBalances["tenant-1-emp-email-"+sickTypeID+"-2025"]
	require.NotNil(t, sickByName)
	assert.True(t, sickByName.EntitledDays.Equal(decimal.NewFromInt(12)))
	assert.True(t, sickByName.RemainingDays.Equal(decimal.NewFromInt(11)))

	sickByNameET := repo.LeaveBalances["tenant-1-emp-name-"+sickTypeID+"-2025"]
	require.NotNil(t, sickByNameET)
	assert.True(t, sickByNameET.EntitledDays.Equal(decimal.NewFromInt(10)))
	assert.True(t, sickByNameET.RemainingDays.Equal(decimal.NewFromInt(8)))
}

func TestImportLeaveBalancesCSV_ResolvesDuplicateAndAmbiguousAbsenceTypeNames(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMockAbsenceRepository()
	repo.Employees["emp-study"] = &Employee{
		ID:             "emp-study",
		TenantID:       "tenant-1",
		EmployeeNumber: "EMP-300",
		FirstName:      "Marta",
		LastName:       "Mets",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
	repo.Employees["emp-annual"] = &Employee{
		ID:             "emp-annual",
		TenantID:       "tenant-1",
		EmployeeNumber: "EMP-301",
		FirstName:      "Peeter",
		LastName:       "Pihl",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
	repo.AbsenceTypes["type-study"] = &AbsenceType{
		ID:                 "type-study",
		TenantID:           "tenant-1",
		Code:               "STUDY_LEAVE",
		Name:               "Study leave",
		NameET:             "Study leave",
		DefaultDaysPerYear: decimal.NewFromInt(5),
		IsActive:           true,
	}
	repo.AbsenceTypes["type-annual"] = &AbsenceType{
		ID:                 "type-annual",
		TenantID:           "tenant-1",
		Code:               "ANNUAL_LEAVE",
		Name:               "Shared leave",
		DefaultDaysPerYear: decimal.NewFromInt(28),
		IsActive:           true,
	}
	repo.AbsenceTypes["type-sick"] = &AbsenceType{
		ID:                 "type-sick",
		TenantID:           "tenant-1",
		Code:               "SICK_LEAVE",
		Name:               "Shared leave",
		DefaultDaysPerYear: decimal.NewFromInt(10),
		IsActive:           true,
	}
	service := NewAbsenceService(repo, &MockUUIDGenerator{prefix: "leave"})

	result, err := service.ImportLeaveBalancesCSV(ctx, "tenant_schema", "tenant-1", &ImportLeaveBalancesRequest{
		CSVContent: "year,employee_number,absence_type_code,absence_type,entitled_days,notes\n" +
			"2025,EMP-300,,Study leave,,Localized duplicate name\n" +
			"2025,EMP-301,ANNUAL_LEAVE,Shared leave,28,Explicit code disambiguates name\n",
	})
	require.NoError(t, err)

	assert.Equal(t, 2, result.RowsProcessed)
	assert.Equal(t, 2, result.LeaveBalancesCreated)
	assert.Zero(t, result.RowsSkipped)
	assert.Nil(t, result.Errors)

	study := repo.LeaveBalances["tenant-1-emp-study-type-study-2025"]
	require.NotNil(t, study)
	assert.True(t, study.EntitledDays.Equal(decimal.NewFromInt(5)))
	assert.True(t, study.RemainingDays.Equal(decimal.NewFromInt(5)))
	assert.Equal(t, "Localized duplicate name", study.Notes)

	annual := repo.LeaveBalances["tenant-1-emp-annual-type-annual-2025"]
	require.NotNil(t, annual)
	assert.True(t, annual.EntitledDays.Equal(decimal.NewFromInt(28)))
	assert.True(t, annual.RemainingDays.Equal(decimal.NewFromInt(28)))
	assert.Equal(t, "Explicit code disambiguates name", annual.Notes)
}

func TestImportLeaveBalancesCSV_RejectsAmbiguousAndMismatchedAbsenceTypes(t *testing.T) {
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
		Name:               "Leave",
		DefaultDaysPerYear: decimal.NewFromInt(28),
		IsActive:           true,
	}
	repo.AbsenceTypes["type-sick"] = &AbsenceType{
		ID:                 "type-sick",
		TenantID:           "tenant-1",
		Code:               "SICK_LEAVE",
		Name:               "Leave",
		NameET:             "Sick Leave",
		DefaultDaysPerYear: decimal.NewFromInt(10),
		IsActive:           true,
	}
	service := NewAbsenceService(repo, &MockUUIDGenerator{prefix: "leave"})

	result, err := service.ImportLeaveBalancesCSV(ctx, "tenant_schema", "tenant-1", &ImportLeaveBalancesRequest{
		CSVContent: "year,employee_number,absence_type_code,absence_type,entitled_days\n" +
			"2025,EMP-100,,Leave,28\n" +
			"2025,EMP-100,ANNUAL_LEAVE,Sick Leave,28\n",
	})
	require.NoError(t, err)

	assert.Equal(t, 2, result.RowsProcessed)
	assert.Zero(t, result.LeaveBalancesCreated)
	assert.Zero(t, result.LeaveBalancesUpdated)
	assert.Equal(t, 2, result.RowsSkipped)
	require.Len(t, result.Errors, 2)
	assert.Contains(t, result.Errors[0].Message, "absence_type \"Leave\" matches multiple types")
	assert.Contains(t, result.Errors[1].Message, "absence type identifiers do not match the same type")
}

func TestImportLeaveBalancesCSV_ReportsRepositoryWriteErrors(t *testing.T) {
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
	repo.CreateLeaveBalanceErr = errors.New("write failed")
	service := NewAbsenceService(repo, &MockUUIDGenerator{prefix: "leave"})

	result, err := service.ImportLeaveBalancesCSV(ctx, "tenant_schema", "tenant-1", &ImportLeaveBalancesRequest{
		CSVContent: "year,employee_number,absence_type_code,entitled_days\n" +
			"2025,EMP-100,ANNUAL_LEAVE,28\n",
	})
	require.NoError(t, err)

	assert.Zero(t, result.LeaveBalancesCreated)
	assert.Zero(t, result.LeaveBalancesUpdated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "create leave balance: write failed")
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

func TestImportLeaveBalancesCSV_RejectsMalformedYearTypeAndAmountRows(t *testing.T) {
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
		CSVContent: "year,employee_number,absence_type_id,absence_type_code,absence_type,entitled_days,carryover_days,used_days,pending_days\n" +
			"2019,EMP-100,,ANNUAL_LEAVE,,28,0,0,0\n" +
			"2025,EMP-100,33333333-3333-4333-8333-333333333333,,,28,0,0,0\n" +
			"2025,EMP-100,,,,28,0,0,0\n" +
			"2025,EMP-100,,ANNUAL_LEAVE,,invalid,0,0,0\n" +
			"2025,EMP-100,,ANNUAL_LEAVE,,28,-1,0,0\n" +
			"2025,EMP-100,,ANNUAL_LEAVE,,28,0,used,0\n" +
			"2025,EMP-100,,ANNUAL_LEAVE,,28,0,0,-1\n",
	})
	require.NoError(t, err)

	assert.Equal(t, 7, result.RowsProcessed)
	assert.Zero(t, result.LeaveBalancesCreated)
	assert.Zero(t, result.LeaveBalancesUpdated)
	assert.Equal(t, 7, result.RowsSkipped)
	require.Len(t, result.Errors, 7)
	assert.Contains(t, result.Errors[0].Message, "period_year must be between 2020 and 2100")
	assert.Contains(t, result.Errors[1].Message, "absence_type_id \"33333333-3333-4333-8333-333333333333\" not found")
	assert.Contains(t, result.Errors[2].Message, "absence_type_code, absence_type, or absence_type_id is required")
	assert.Contains(t, result.Errors[3].Message, "invalid entitled_days")
	assert.Contains(t, result.Errors[4].Message, "carryover_days must be zero or greater")
	assert.Contains(t, result.Errors[5].Message, "invalid used_days")
	assert.Contains(t, result.Errors[6].Message, "pending_days must be zero or greater")
}

func TestImportLeaveBalancesCSV_RejectsInvalidAbsenceTypeID(t *testing.T) {
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
	service := NewAbsenceService(repo, &MockUUIDGenerator{prefix: "leave"})

	result, err := service.ImportLeaveBalancesCSV(ctx, "tenant_schema", "tenant-1", &ImportLeaveBalancesRequest{
		CSVContent: "year,employee_number,absence_type_id,entitled_days\n" +
			"2025,EMP-100,legacy-type,28\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Zero(t, result.LeaveBalancesCreated)
	assert.Zero(t, result.LeaveBalancesUpdated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "absence_type_id must be a valid UUID")
}

func TestImportLeaveBalancesCSV_RejectsMissingHeaders(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMockAbsenceRepository()
	service := NewAbsenceService(repo, &MockUUIDGenerator{prefix: "leave"})

	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name:    "missing year",
			content: "employee_number,absence_type_code,entitled_days\nEMP-100,ANNUAL_LEAVE,28\n",
			wantErr: "missing required year column",
		},
		{
			name:    "missing absence type",
			content: "year,employee_number,entitled_days\n2025,EMP-100,28\n",
			wantErr: "missing required absence_type_code, absence_type, or absence_type_id column",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.ImportLeaveBalancesCSV(ctx, "tenant_schema", "tenant-1", &ImportLeaveBalancesRequest{
				CSVContent: tt.content,
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
