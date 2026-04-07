package payroll

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/testutil"
)

func TestAbsencePostgresRepository_Integration(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	baseRepo := NewPostgresRepository(pool)
	repo := NewAbsencePostgresRepository(baseRepo)
	ctx := context.Background()

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
	if _, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.absence_types
			(id, tenant_id, code, name, name_et, description, is_paid, affects_salary, requires_document,
			 document_type, default_days_per_year, max_carryover_days, tsd_code, emta_code, is_system, is_active, sort_order, created_at, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
	`, absenceType.ID, absenceType.TenantID, absenceType.Code, absenceType.Name, absenceType.NameET, absenceType.Description,
		absenceType.IsPaid, absenceType.AffectsSalary, absenceType.RequiresDocument, absenceType.DocumentType,
		absenceType.DefaultDaysPerYear, absenceType.MaxCarryoverDays, absenceType.TSDCode, absenceType.EMTACode,
		absenceType.IsSystem, absenceType.IsActive, absenceType.SortOrder, absenceType.CreatedAt, absenceType.UpdatedAt); err != nil {
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

func datePtr(t time.Time) *time.Time {
	return &t
}
