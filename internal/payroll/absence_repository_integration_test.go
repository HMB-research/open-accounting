//go:build integration

package payroll

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/HMB-research/open-accounting/internal/testutil"
)

func TestAbsenceGORMRepository_Integration(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	ctx := context.Background()
	gormDB, err := database.NewGormDBFromPool(ctx, pool)
	if err != nil {
		t.Fatalf("failed to create GORM DB: %v", err)
	}
	baseRepo := NewGORMRepository(gormDB)
	repo := NewAbsenceGORMRepository(gormDB)

	if _, err := pool.Exec(ctx, "SELECT add_payroll_tables($1)", tenant.SchemaName); err != nil {
		t.Fatalf("failed to add payroll tables: %v", err)
	}

	absenceType := &AbsenceType{
		ID:                 uuid.New().String(),
		TenantID:           tenant.ID,
		Code:               "TEST_LEAVE",
		Name:               "Test Leave",
		NameET:             "Testpuhkus",
		Description:        "Used for integration coverage",
		IsPaid:             true,
		AffectsSalary:      false,
		RequiresDocument:   true,
		DocumentType:       "CERTIFICATE",
		DefaultDaysPerYear: decimal.NewFromInt(10),
		MaxCarryoverDays:   decimal.NewFromInt(2),
		TSDCode:            "42",
		EMTACode:           "T42",
		IsSystem:           false,
		IsActive:           true,
		SortOrder:          99,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	absenceTypesTable, err := database.TenantTable(gormDB.WithContext(ctx), tenant.SchemaName, "absence_types")
	if err != nil {
		t.Fatalf("failed to qualify absence types table: %v", err)
	}
	if err := absenceTypesTable.Create(absenceTypeToModel(absenceType)).Error; err != nil {
		t.Fatalf("failed to create absence type: %v", err)
	}

	employee := &Employee{
		ID:                   uuid.New().String(),
		TenantID:             tenant.ID,
		EmployeeNumber:       "ABS-001",
		FirstName:            "Anu",
		LastName:             "Absence",
		PersonalCode:         "48901234567",
		Email:                "anu.absence@example.com",
		StartDate:            time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EmploymentType:       EmploymentFullTime,
		TaxResidency:         "EE",
		ApplyBasicExemption:  true,
		BasicExemptionAmount: DefaultBasicExemption,
		FundedPensionRate:    FundedPensionRateDefault,
		IsActive:             true,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}
	if err := baseRepo.CreateEmployee(ctx, tenant.SchemaName, employee); err != nil {
		t.Fatalf("failed to create employee: %v", err)
	}

	t.Run("absence types", func(t *testing.T) {
		types, err := repo.ListAbsenceTypes(ctx, tenant.SchemaName, tenant.ID, false)
		if err != nil {
			t.Fatalf("ListAbsenceTypes failed: %v", err)
		}
		if len(types) != 1 {
			t.Fatalf("expected 1 tenant-specific absence type, got %d", len(types))
		}

		gotByID, err := repo.GetAbsenceType(ctx, tenant.SchemaName, tenant.ID, absenceType.ID)
		if err != nil {
			t.Fatalf("GetAbsenceType failed: %v", err)
		}
		if gotByID.Code != absenceType.Code {
			t.Fatalf("expected code %s, got %s", absenceType.Code, gotByID.Code)
		}

		gotByCode, err := repo.GetAbsenceTypeByCode(ctx, tenant.SchemaName, tenant.ID, absenceType.Code)
		if err != nil {
			t.Fatalf("GetAbsenceTypeByCode failed: %v", err)
		}
		if gotByCode.ID != absenceType.ID {
			t.Fatalf("expected absence type id %s, got %s", absenceType.ID, gotByCode.ID)
		}
	})

	t.Run("leave balances", func(t *testing.T) {
		balance := &LeaveBalance{
			ID:            uuid.New().String(),
			TenantID:      tenant.ID,
			EmployeeID:    employee.ID,
			AbsenceTypeID: absenceType.ID,
			Year:          2025,
			EntitledDays:  decimal.NewFromInt(10),
			CarryoverDays: decimal.NewFromInt(2),
			UsedDays:      decimal.NewFromInt(1),
			PendingDays:   decimal.NewFromInt(1),
			Notes:         "initial balance",
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		if err := repo.CreateLeaveBalance(ctx, tenant.SchemaName, balance); err != nil {
			t.Fatalf("CreateLeaveBalance failed: %v", err)
		}

		gotBalance, err := repo.GetLeaveBalance(ctx, tenant.SchemaName, tenant.ID, employee.ID, absenceType.ID, 2025)
		if err != nil {
			t.Fatalf("GetLeaveBalance failed: %v", err)
		}
		if !gotBalance.RemainingDays.Equal(decimal.NewFromInt(10)) {
			t.Fatalf("expected remaining days 10, got %s", gotBalance.RemainingDays)
		}

		balance.UsedDays = decimal.NewFromInt(3)
		balance.PendingDays = decimal.Zero
		balance.Notes = "updated balance"
		balance.UpdatedAt = time.Now()
		if err := repo.UpdateLeaveBalance(ctx, tenant.SchemaName, balance); err != nil {
			t.Fatalf("UpdateLeaveBalance failed: %v", err)
		}

		balances, err := repo.ListLeaveBalances(ctx, tenant.SchemaName, tenant.ID, employee.ID, 2025)
		if err != nil {
			t.Fatalf("ListLeaveBalances failed: %v", err)
		}
		if len(balances) != 1 {
			t.Fatalf("expected 1 leave balance, got %d", len(balances))
		}
		if balances[0].AbsenceType == nil || balances[0].AbsenceType.Code != absenceType.Code {
			t.Fatalf("expected joined absence type details, got %+v", balances[0].AbsenceType)
		}
	})

	t.Run("leave records", func(t *testing.T) {
		record := &LeaveRecord{
			ID:             uuid.New().String(),
			TenantID:       tenant.ID,
			EmployeeID:     employee.ID,
			AbsenceTypeID:  absenceType.ID,
			StartDate:      time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC),
			EndDate:        time.Date(2025, 7, 3, 0, 0, 0, 0, time.UTC),
			TotalDays:      decimal.NewFromInt(3),
			WorkingDays:    decimal.NewFromInt(3),
			Status:         LeavePending,
			DocumentNumber: "CERT-123",
			DocumentDate:   datePtr(time.Date(2025, 6, 30, 0, 0, 0, 0, time.UTC)),
			DocumentURL:    "https://example.com/cert",
			RequestedAt:    time.Now(),
			RequestedBy:    uuid.New().String(),
			Notes:          "summer leave",
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		if err := repo.CreateLeaveRecord(ctx, tenant.SchemaName, record); err != nil {
			t.Fatalf("CreateLeaveRecord failed: %v", err)
		}

		gotRecord, err := repo.GetLeaveRecord(ctx, tenant.SchemaName, tenant.ID, record.ID)
		if err != nil {
			t.Fatalf("GetLeaveRecord failed: %v", err)
		}
		if gotRecord.DocumentNumber != record.DocumentNumber {
			t.Fatalf("expected document number %s, got %s", record.DocumentNumber, gotRecord.DocumentNumber)
		}

		approvedAt := time.Now()
		record.Status = LeaveApproved
		record.ApprovedAt = &approvedAt
		record.ApprovedBy = uuid.New().String()
		record.RejectedBy = uuid.New().String()
		record.UpdatedAt = time.Now()
		if err := repo.UpdateLeaveRecord(ctx, tenant.SchemaName, record); err != nil {
			t.Fatalf("UpdateLeaveRecord failed: %v", err)
		}

		records, err := repo.ListLeaveRecords(ctx, tenant.SchemaName, tenant.ID, employee.ID, 2025)
		if err != nil {
			t.Fatalf("ListLeaveRecords failed: %v", err)
		}
		if len(records) != 1 {
			t.Fatalf("expected 1 leave record, got %d", len(records))
		}
		if records[0].AbsenceType == nil || records[0].AbsenceType.Code != absenceType.Code {
			t.Fatalf("expected joined absence type on leave record, got %+v", records[0].AbsenceType)
		}

		allRecords, err := repo.ListLeaveRecords(ctx, tenant.SchemaName, tenant.ID, "", 0)
		if err != nil {
			t.Fatalf("ListLeaveRecords without filters failed: %v", err)
		}
		if len(allRecords) != 1 {
			t.Fatalf("expected unfiltered list to return 1 record, got %d", len(allRecords))
		}
	})
}

func TestAbsenceGORMRepository_FiltersAndNotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	ctx := context.Background()
	gormDB, err := database.NewGormDBFromPool(ctx, pool)
	if err != nil {
		t.Fatalf("failed to create GORM DB: %v", err)
	}
	baseRepo := NewGORMRepository(gormDB)
	repo := NewAbsenceGORMRepository(gormDB)

	if _, err := pool.Exec(ctx, "SELECT add_payroll_tables($1)", tenant.SchemaName); err != nil {
		t.Fatalf("failed to add payroll tables: %v", err)
	}

	activeEmployee := testEmployee(tenant.ID, "ABS-FILTER-ACTIVE")
	activeEmployee.FirstName = "Active"
	activeEmployee.LastName = "Employee"
	inactiveEmployee := testEmployee(tenant.ID, "ABS-FILTER-INACTIVE")
	inactiveEmployee.FirstName = "Inactive"
	inactiveEmployee.LastName = "Employee"
	inactiveEmployee.IsActive = false
	for _, employee := range []*Employee{activeEmployee, inactiveEmployee} {
		if err := baseRepo.CreateEmployee(ctx, tenant.SchemaName, employee); err != nil {
			t.Fatalf("failed to create employee %s: %v", employee.EmployeeNumber, err)
		}
	}

	activeType := &AbsenceType{
		ID:                 uuid.New().String(),
		TenantID:           tenant.ID,
		Code:               "ACTIVE_LEAVE",
		Name:               "Active Leave",
		DefaultDaysPerYear: decimal.NewFromInt(20),
		IsActive:           true,
		SortOrder:          1,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	inactiveType := &AbsenceType{
		ID:                 uuid.New().String(),
		TenantID:           tenant.ID,
		Code:               "INACTIVE_LEAVE",
		Name:               "Inactive Leave",
		DefaultDaysPerYear: decimal.NewFromInt(5),
		IsActive:           false,
		SortOrder:          2,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	absenceTypesTable, err := database.TenantTable(gormDB.WithContext(ctx), tenant.SchemaName, "absence_types")
	if err != nil {
		t.Fatalf("failed to qualify absence types table: %v", err)
	}
	for _, absenceType := range []*AbsenceType{activeType, inactiveType} {
		if err := absenceTypesTable.Create(absenceTypeToModel(absenceType)).Error; err != nil {
			t.Fatalf("failed to create absence type %s: %v", absenceType.Code, err)
		}
	}

	allEmployees, err := repo.ListEmployees(ctx, tenant.SchemaName, tenant.ID, false)
	if err != nil {
		t.Fatalf("ListEmployees failed: %v", err)
	}
	if len(allEmployees) != 2 {
		t.Fatalf("expected 2 employees, got %d", len(allEmployees))
	}
	activeEmployees, err := repo.ListEmployees(ctx, tenant.SchemaName, tenant.ID, true)
	if err != nil {
		t.Fatalf("ListEmployees active failed: %v", err)
	}
	if len(activeEmployees) != 1 || activeEmployees[0].ID != activeEmployee.ID {
		t.Fatalf("expected active employee %s, got %+v", activeEmployee.ID, activeEmployees)
	}

	allTypes, err := repo.ListAbsenceTypes(ctx, tenant.SchemaName, tenant.ID, false)
	if err != nil {
		t.Fatalf("ListAbsenceTypes failed: %v", err)
	}
	if len(allTypes) != 2 {
		t.Fatalf("expected 2 absence types, got %d", len(allTypes))
	}
	activeTypes, err := repo.ListAbsenceTypes(ctx, tenant.SchemaName, tenant.ID, true)
	if err != nil {
		t.Fatalf("ListAbsenceTypes active failed: %v", err)
	}
	if len(activeTypes) != 1 || activeTypes[0].ID != activeType.ID {
		t.Fatalf("expected active absence type %s, got %+v", activeType.ID, activeTypes)
	}

	emptyBalances, err := repo.ListLeaveBalances(ctx, tenant.SchemaName, tenant.ID, activeEmployee.ID, 2099)
	if err != nil {
		t.Fatalf("ListLeaveBalances empty failed: %v", err)
	}
	if len(emptyBalances) != 0 {
		t.Fatalf("expected no leave balances, got %d", len(emptyBalances))
	}
	emptyRecords, err := repo.ListLeaveRecords(ctx, tenant.SchemaName, tenant.ID, activeEmployee.ID, 2099)
	if err != nil {
		t.Fatalf("ListLeaveRecords empty failed: %v", err)
	}
	if len(emptyRecords) != 0 {
		t.Fatalf("expected no leave records, got %d", len(emptyRecords))
	}

	if _, err := repo.GetAbsenceType(ctx, tenant.SchemaName, tenant.ID, uuid.New().String()); !errors.Is(err, ErrAbsenceTypeNotFound) {
		t.Fatalf("expected ErrAbsenceTypeNotFound from GetAbsenceType, got %v", err)
	}
	if _, err := repo.GetAbsenceTypeByCode(ctx, tenant.SchemaName, tenant.ID, "MISSING_LEAVE"); !errors.Is(err, ErrAbsenceTypeNotFound) {
		t.Fatalf("expected ErrAbsenceTypeNotFound from GetAbsenceTypeByCode, got %v", err)
	}
	if _, err := repo.GetLeaveBalance(ctx, tenant.SchemaName, tenant.ID, activeEmployee.ID, activeType.ID, 2099); !errors.Is(err, ErrLeaveBalanceNotFound) {
		t.Fatalf("expected ErrLeaveBalanceNotFound from GetLeaveBalance, got %v", err)
	}
	if err := repo.UpdateLeaveBalance(ctx, tenant.SchemaName, &LeaveBalance{
		ID:            uuid.New().String(),
		TenantID:      tenant.ID,
		EmployeeID:    activeEmployee.ID,
		AbsenceTypeID: activeType.ID,
		Year:          2099,
		UpdatedAt:     time.Now(),
	}); !errors.Is(err, ErrLeaveBalanceNotFound) {
		t.Fatalf("expected ErrLeaveBalanceNotFound from UpdateLeaveBalance, got %v", err)
	}
	if _, err := repo.GetLeaveRecord(ctx, tenant.SchemaName, tenant.ID, uuid.New().String()); !errors.Is(err, ErrLeaveRecordNotFound) {
		t.Fatalf("expected ErrLeaveRecordNotFound from GetLeaveRecord, got %v", err)
	}
	if err := repo.UpdateLeaveRecord(ctx, tenant.SchemaName, &LeaveRecord{
		ID:        uuid.New().String(),
		TenantID:  tenant.ID,
		Status:    LeaveApproved,
		UpdatedAt: time.Now(),
	}); !errors.Is(err, ErrLeaveRecordNotFound) {
		t.Fatalf("expected ErrLeaveRecordNotFound from UpdateLeaveRecord, got %v", err)
	}
}

func datePtr(t time.Time) *time.Time {
	return &t
}

func absenceTypeToModel(t *AbsenceType) *models.AbsenceType {
	return &models.AbsenceType{
		ID:                 t.ID,
		TenantID:           t.TenantID,
		Code:               t.Code,
		Name:               t.Name,
		NameET:             t.NameET,
		Description:        stringPtrIfNotBlank(t.Description),
		IsPaid:             t.IsPaid,
		AffectsSalary:      t.AffectsSalary,
		RequiresDocument:   t.RequiresDocument,
		DocumentType:       stringPtrIfNotBlank(t.DocumentType),
		DefaultDaysPerYear: models.Decimal{Decimal: t.DefaultDaysPerYear},
		MaxCarryoverDays:   models.Decimal{Decimal: t.MaxCarryoverDays},
		TSDCode:            stringPtrIfNotBlank(t.TSDCode),
		EMTACode:           stringPtrIfNotBlank(t.EMTACode),
		IsSystem:           t.IsSystem,
		IsActive:           t.IsActive,
		SortOrder:          t.SortOrder,
		CreatedAt:          t.CreatedAt,
		UpdatedAt:          t.UpdatedAt,
	}
}
