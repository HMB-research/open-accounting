package payroll

import (
	"context"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAbsenceGORMRepositoryNilDatabase(t *testing.T) {
	repo := NewAbsenceGORMRepository(nil)
	ctx := context.Background()
	schemaName := "tenant_schema"
	tenantID := "tenant-1"
	balance := &LeaveBalance{
		ID:            "balance-1",
		TenantID:      tenantID,
		EmployeeID:    "employee-1",
		AbsenceTypeID: "absence-type-1",
		Year:          2026,
	}
	record := &LeaveRecord{
		ID:       "record-1",
		TenantID: tenantID,
	}

	require.NotNil(t, repo)
	assert.Nil(t, repo.db)

	tableName, err := repo.tenantTableName(schemaName, "leave_balances")
	require.NoError(t, err)
	assert.Equal(t, `"tenant_schema"."leave_balances"`, tableName)

	invalidTableName, err := repo.tenantTableName("tenant-schema", "leave_balances")
	require.Error(t, err)
	assert.Empty(t, invalidTableName)
	assert.Contains(t, err.Error(), "invalid SQL identifier")

	tests := []struct {
		name string
		run  func(t *testing.T) error
	}{
		{
			name: "dbWithContext",
			run: func(t *testing.T) error {
				db, err := repo.dbWithContext(ctx)
				assert.Nil(t, db)
				return err
			},
		},
		{
			name: "tenantTable",
			run: func(t *testing.T) error {
				db, err := repo.tenantTable(ctx, schemaName, "absence_types")
				assert.Nil(t, db)
				return err
			},
		},
		{
			name: "ListEmployees",
			run: func(t *testing.T) error {
				employees, err := repo.ListEmployees(ctx, schemaName, tenantID, true)
				assert.Nil(t, employees)
				return err
			},
		},
		{
			name: "ListAbsenceTypes",
			run: func(t *testing.T) error {
				types, err := repo.ListAbsenceTypes(ctx, schemaName, tenantID, true)
				assert.Nil(t, types)
				return err
			},
		},
		{
			name: "GetAbsenceType",
			run: func(t *testing.T) error {
				absenceType, err := repo.GetAbsenceType(ctx, schemaName, tenantID, "absence-type-1")
				assert.Nil(t, absenceType)
				return err
			},
		},
		{
			name: "GetAbsenceTypeByCode",
			run: func(t *testing.T) error {
				absenceType, err := repo.GetAbsenceTypeByCode(ctx, schemaName, tenantID, "ANNUAL")
				assert.Nil(t, absenceType)
				return err
			},
		},
		{
			name: "GetLeaveBalance",
			run: func(t *testing.T) error {
				gotBalance, err := repo.GetLeaveBalance(ctx, schemaName, tenantID, balance.EmployeeID, balance.AbsenceTypeID, balance.Year)
				assert.Nil(t, gotBalance)
				return err
			},
		},
		{
			name: "ListLeaveBalances",
			run: func(t *testing.T) error {
				balances, err := repo.ListLeaveBalances(ctx, schemaName, tenantID, balance.EmployeeID, balance.Year)
				assert.Nil(t, balances)
				return err
			},
		},
		{
			name: "CreateLeaveBalance",
			run: func(t *testing.T) error {
				return repo.CreateLeaveBalance(ctx, schemaName, balance)
			},
		},
		{
			name: "UpdateLeaveBalance",
			run: func(t *testing.T) error {
				return repo.UpdateLeaveBalance(ctx, schemaName, balance)
			},
		},
		{
			name: "CreateLeaveRecord",
			run: func(t *testing.T) error {
				return repo.CreateLeaveRecord(ctx, schemaName, record)
			},
		},
		{
			name: "GetLeaveRecord",
			run: func(t *testing.T) error {
				gotRecord, err := repo.GetLeaveRecord(ctx, schemaName, tenantID, record.ID)
				assert.Nil(t, gotRecord)
				return err
			},
		},
		{
			name: "ListLeaveRecords",
			run: func(t *testing.T) error {
				records, err := repo.ListLeaveRecords(ctx, schemaName, tenantID, record.EmployeeID, 2026)
				assert.Nil(t, records)
				return err
			},
		},
		{
			name: "UpdateLeaveRecord",
			run: func(t *testing.T) error {
				return repo.UpdateLeaveRecord(ctx, schemaName, record)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(t)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "absence repository database is not configured")
		})
	}
}

func TestAbsenceTypeModelMapping(t *testing.T) {
	now := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	description := "Annual leave entitlement"
	documentType := "VACATION_REQUEST"
	tsdCode := "10"
	emtaCode := "PUHKUS"
	model := &models.AbsenceType{
		ID:                 uuid.NewString(),
		TenantID:           uuid.NewString(),
		Code:               "ANNUAL",
		Name:               "Annual leave",
		NameET:             "Puhkus",
		Description:        &description,
		IsPaid:             true,
		AffectsSalary:      false,
		RequiresDocument:   true,
		DocumentType:       &documentType,
		DefaultDaysPerYear: models.Decimal{Decimal: decimal.NewFromFloat(28.5)},
		MaxCarryoverDays:   models.Decimal{Decimal: decimal.NewFromFloat(7.5)},
		TSDCode:            &tsdCode,
		EMTACode:           &emtaCode,
		IsSystem:           true,
		IsActive:           true,
		SortOrder:          20,
		CreatedAt:          now,
		UpdatedAt:          now.Add(time.Hour),
	}

	got := modelToAbsenceType(model)

	assert.Equal(t, model.ID, got.ID)
	assert.Equal(t, model.TenantID, got.TenantID)
	assert.Equal(t, model.Code, got.Code)
	assert.Equal(t, model.Name, got.Name)
	assert.Equal(t, model.NameET, got.NameET)
	assert.Equal(t, description, got.Description)
	assert.Equal(t, model.IsPaid, got.IsPaid)
	assert.Equal(t, model.AffectsSalary, got.AffectsSalary)
	assert.Equal(t, model.RequiresDocument, got.RequiresDocument)
	assert.Equal(t, documentType, got.DocumentType)
	requireDecimalEqual(t, got.DefaultDaysPerYear, model.DefaultDaysPerYear.Decimal)
	requireDecimalEqual(t, got.MaxCarryoverDays, model.MaxCarryoverDays.Decimal)
	assert.Equal(t, tsdCode, got.TSDCode)
	assert.Equal(t, emtaCode, got.EMTACode)
	assert.Equal(t, model.IsSystem, got.IsSystem)
	assert.Equal(t, model.IsActive, got.IsActive)
	assert.Equal(t, model.SortOrder, got.SortOrder)
	assert.Equal(t, model.CreatedAt, got.CreatedAt)
	assert.Equal(t, model.UpdatedAt, got.UpdatedAt)

	withoutOptionalFields := modelToAbsenceType(&models.AbsenceType{})
	assert.Empty(t, withoutOptionalFields.Description)
	assert.Empty(t, withoutOptionalFields.DocumentType)
	assert.Empty(t, withoutOptionalFields.TSDCode)
	assert.Empty(t, withoutOptionalFields.EMTACode)
}

func TestLeaveBalanceModelMappings(t *testing.T) {
	now := time.Date(2026, 4, 5, 6, 7, 8, 0, time.UTC)
	balance := &LeaveBalance{
		ID:            uuid.NewString(),
		TenantID:      uuid.NewString(),
		EmployeeID:    uuid.NewString(),
		AbsenceTypeID: uuid.NewString(),
		Year:          2026,
		EntitledDays:  decimal.NewFromFloat(28.5),
		CarryoverDays: decimal.NewFromFloat(3.25),
		UsedDays:      decimal.NewFromFloat(4.5),
		PendingDays:   decimal.NewFromFloat(2.25),
		RemainingDays: decimal.NewFromInt(25),
		Notes:         "Initial migration balance",
		CreatedAt:     now,
		UpdatedAt:     now.Add(time.Hour),
	}

	model := leaveBalanceToModel(balance)

	assert.Equal(t, balance.ID, model.ID)
	assert.Equal(t, balance.TenantID, model.TenantID)
	assert.Equal(t, balance.EmployeeID, model.EmployeeID)
	assert.Equal(t, balance.AbsenceTypeID, model.AbsenceTypeID)
	assert.Equal(t, balance.Year, model.Year)
	requireDecimalEqual(t, model.EntitledDays.Decimal, balance.EntitledDays)
	requireDecimalEqual(t, model.CarryoverDays.Decimal, balance.CarryoverDays)
	requireDecimalEqual(t, model.UsedDays.Decimal, balance.UsedDays)
	requireDecimalEqual(t, model.PendingDays.Decimal, balance.PendingDays)
	requireDecimalEqual(t, model.RemainingDays.Decimal, decimal.Zero)
	require.NotNil(t, model.Notes)
	assert.Equal(t, balance.Notes, *model.Notes)
	assert.Equal(t, balance.CreatedAt, model.CreatedAt)
	assert.Equal(t, balance.UpdatedAt, model.UpdatedAt)

	model.RemainingDays = models.Decimal{Decimal: balance.RemainingDays}
	roundTrip := modelToLeaveBalance(model)
	assert.Equal(t, balance.ID, roundTrip.ID)
	assert.Equal(t, balance.TenantID, roundTrip.TenantID)
	assert.Equal(t, balance.EmployeeID, roundTrip.EmployeeID)
	assert.Equal(t, balance.AbsenceTypeID, roundTrip.AbsenceTypeID)
	assert.Equal(t, balance.Year, roundTrip.Year)
	requireDecimalEqual(t, roundTrip.EntitledDays, balance.EntitledDays)
	requireDecimalEqual(t, roundTrip.CarryoverDays, balance.CarryoverDays)
	requireDecimalEqual(t, roundTrip.UsedDays, balance.UsedDays)
	requireDecimalEqual(t, roundTrip.PendingDays, balance.PendingDays)
	requireDecimalEqual(t, roundTrip.RemainingDays, balance.RemainingDays)
	assert.Equal(t, balance.Notes, roundTrip.Notes)
	assert.Equal(t, balance.CreatedAt, roundTrip.CreatedAt)
	assert.Equal(t, balance.UpdatedAt, roundTrip.UpdatedAt)

	assert.Nil(t, leaveBalanceToModel(&LeaveBalance{}).Notes)
	assert.Empty(t, modelToLeaveBalance(&models.LeaveBalance{}).Notes)
}

func TestLeaveRecordModelMappings(t *testing.T) {
	now := time.Date(2026, 5, 6, 7, 8, 9, 0, time.UTC)
	documentDate := now.AddDate(0, 0, -2)
	approvedAt := now.Add(time.Hour)
	rejectedAt := now.Add(2 * time.Hour)
	record := &LeaveRecord{
		ID:              uuid.NewString(),
		TenantID:        uuid.NewString(),
		EmployeeID:      uuid.NewString(),
		AbsenceTypeID:   uuid.NewString(),
		StartDate:       now.AddDate(0, 1, 0),
		EndDate:         now.AddDate(0, 1, 4),
		TotalDays:       decimal.NewFromInt(5),
		WorkingDays:     decimal.NewFromInt(3),
		Status:          LeaveApproved,
		DocumentNumber:  "CERT-2026-001",
		DocumentDate:    &documentDate,
		DocumentURL:     "https://example.com/cert-2026-001",
		RequestedAt:     now,
		RequestedBy:     uuid.NewString(),
		ApprovedAt:      &approvedAt,
		ApprovedBy:      uuid.NewString(),
		RejectedAt:      &rejectedAt,
		RejectedBy:      uuid.NewString(),
		RejectionReason: "replacement request",
		PayrollRunID:    uuid.NewString(),
		Notes:           "linked to imported payroll history",
		CreatedAt:       now.Add(-time.Hour),
		UpdatedAt:       now.Add(3 * time.Hour),
	}

	model := leaveRecordToModel(record)

	assert.Equal(t, record.ID, model.ID)
	assert.Equal(t, record.TenantID, model.TenantID)
	assert.Equal(t, record.EmployeeID, model.EmployeeID)
	assert.Equal(t, record.AbsenceTypeID, model.AbsenceTypeID)
	assert.Equal(t, record.StartDate, model.StartDate)
	assert.Equal(t, record.EndDate, model.EndDate)
	requireDecimalEqual(t, model.TotalDays.Decimal, record.TotalDays)
	requireDecimalEqual(t, model.WorkingDays.Decimal, record.WorkingDays)
	assert.Equal(t, string(record.Status), model.Status)
	requireStringPointerValue(t, model.DocumentNumber, record.DocumentNumber)
	assert.Equal(t, record.DocumentDate, model.DocumentDate)
	requireStringPointerValue(t, model.DocumentURL, record.DocumentURL)
	assert.Equal(t, record.RequestedAt, model.RequestedAt)
	requireStringPointerValue(t, model.RequestedBy, record.RequestedBy)
	assert.Equal(t, record.ApprovedAt, model.ApprovedAt)
	requireStringPointerValue(t, model.ApprovedBy, record.ApprovedBy)
	assert.Equal(t, record.RejectedAt, model.RejectedAt)
	requireStringPointerValue(t, model.RejectedBy, record.RejectedBy)
	requireStringPointerValue(t, model.RejectionReason, record.RejectionReason)
	requireStringPointerValue(t, model.PayrollRunID, record.PayrollRunID)
	requireStringPointerValue(t, model.Notes, record.Notes)
	assert.Equal(t, record.CreatedAt, model.CreatedAt)
	assert.Equal(t, record.UpdatedAt, model.UpdatedAt)

	roundTrip := modelToLeaveRecord(model)
	assert.Equal(t, record.ID, roundTrip.ID)
	assert.Equal(t, record.TenantID, roundTrip.TenantID)
	assert.Equal(t, record.EmployeeID, roundTrip.EmployeeID)
	assert.Equal(t, record.AbsenceTypeID, roundTrip.AbsenceTypeID)
	assert.Equal(t, record.StartDate, roundTrip.StartDate)
	assert.Equal(t, record.EndDate, roundTrip.EndDate)
	requireDecimalEqual(t, roundTrip.TotalDays, record.TotalDays)
	requireDecimalEqual(t, roundTrip.WorkingDays, record.WorkingDays)
	assert.Equal(t, record.Status, roundTrip.Status)
	assert.Equal(t, record.DocumentNumber, roundTrip.DocumentNumber)
	assert.Equal(t, record.DocumentDate, roundTrip.DocumentDate)
	assert.Equal(t, record.DocumentURL, roundTrip.DocumentURL)
	assert.Equal(t, record.RequestedAt, roundTrip.RequestedAt)
	assert.Equal(t, record.RequestedBy, roundTrip.RequestedBy)
	assert.Equal(t, record.ApprovedAt, roundTrip.ApprovedAt)
	assert.Equal(t, record.ApprovedBy, roundTrip.ApprovedBy)
	assert.Equal(t, record.RejectedAt, roundTrip.RejectedAt)
	assert.Equal(t, record.RejectedBy, roundTrip.RejectedBy)
	assert.Equal(t, record.RejectionReason, roundTrip.RejectionReason)
	assert.Equal(t, record.PayrollRunID, roundTrip.PayrollRunID)
	assert.Equal(t, record.Notes, roundTrip.Notes)
	assert.Equal(t, record.CreatedAt, roundTrip.CreatedAt)
	assert.Equal(t, record.UpdatedAt, roundTrip.UpdatedAt)

	withoutOptionalFields := leaveRecordToModel(&LeaveRecord{})
	assert.Nil(t, withoutOptionalFields.DocumentNumber)
	assert.Nil(t, withoutOptionalFields.DocumentURL)
	assert.Nil(t, withoutOptionalFields.RequestedBy)
	assert.Nil(t, withoutOptionalFields.ApprovedBy)
	assert.Nil(t, withoutOptionalFields.RejectedBy)
	assert.Nil(t, withoutOptionalFields.RejectionReason)
	assert.Nil(t, withoutOptionalFields.PayrollRunID)
	assert.Nil(t, withoutOptionalFields.Notes)
	assert.Empty(t, modelToLeaveRecord(&models.LeaveRecord{}).DocumentNumber)
	assert.Empty(t, modelToLeaveRecord(&models.LeaveRecord{}).RequestedBy)
}

func requireStringPointerValue(t *testing.T, got *string, want string) {
	t.Helper()
	require.NotNil(t, got)
	assert.Equal(t, want, *got)
}
