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

// ============================================================================
// ABSENCE SERVICE TESTS
// ============================================================================

func TestNewAbsenceService(t *testing.T) {
	repo := NewMockAbsenceRepository()
	uuidGen := &MockUUIDGenerator{prefix: "test"}

	service := NewAbsenceService(repo, uuidGen)

	assert.NotNil(t, service)
	assert.Equal(t, repo, service.repo)
	assert.Equal(t, uuidGen, service.uuid)
}

func TestListAbsenceTypes_Success(t *testing.T) {
	repo := NewMockAbsenceRepository()
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	service := NewAbsenceService(repo, uuidGen)
	ctx := context.Background()

	// Setup test data
	repo.AbsenceTypes["type-1"] = &AbsenceType{
		ID:       "type-1",
		TenantID: "tenant-1",
		Code:     "ANNUAL_LEAVE",
		Name:     "Annual Leave",
		NameET:   "Põhipuhkus",
		IsActive: true,
	}
	repo.AbsenceTypes["type-2"] = &AbsenceType{
		ID:       "type-2",
		TenantID: "tenant-1",
		Code:     "SICK_LEAVE",
		Name:     "Sick Leave",
		NameET:   "Haigusleht",
		IsActive: false,
	}

	types, err := service.ListAbsenceTypes(ctx, "test_schema", "tenant-1", false)

	require.NoError(t, err)
	assert.Len(t, types, 2)
}

func TestListAbsenceTypes_ActiveOnly(t *testing.T) {
	repo := NewMockAbsenceRepository()
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	service := NewAbsenceService(repo, uuidGen)
	ctx := context.Background()

	repo.AbsenceTypes["type-1"] = &AbsenceType{
		ID:       "type-1",
		TenantID: "tenant-1",
		Code:     "ANNUAL_LEAVE",
		IsActive: true,
	}
	repo.AbsenceTypes["type-2"] = &AbsenceType{
		ID:       "type-2",
		TenantID: "tenant-1",
		Code:     "SICK_LEAVE",
		IsActive: false,
	}

	types, err := service.ListAbsenceTypes(ctx, "test_schema", "tenant-1", true)

	require.NoError(t, err)
	assert.Len(t, types, 1)
	assert.Equal(t, "ANNUAL_LEAVE", types[0].Code)
}

func TestListAbsenceTypes_RepositoryError(t *testing.T) {
	repo := NewMockAbsenceRepository()
	repo.ListAbsenceTypesErr = errors.New("database error")
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	service := NewAbsenceService(repo, uuidGen)
	ctx := context.Background()

	_, err := service.ListAbsenceTypes(ctx, "test_schema", "tenant-1", false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "list absence types")
}

func TestGetAbsenceType_Success(t *testing.T) {
	repo := NewMockAbsenceRepository()
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	service := NewAbsenceService(repo, uuidGen)
	ctx := context.Background()

	repo.AbsenceTypes["type-1"] = &AbsenceType{
		ID:       "type-1",
		TenantID: "tenant-1",
		Code:     "ANNUAL_LEAVE",
		Name:     "Annual Leave",
	}

	at, err := service.GetAbsenceType(ctx, "test_schema", "tenant-1", "type-1")

	require.NoError(t, err)
	assert.Equal(t, "ANNUAL_LEAVE", at.Code)
}

func TestGetAbsenceType_NotFound(t *testing.T) {
	repo := NewMockAbsenceRepository()
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	service := NewAbsenceService(repo, uuidGen)
	ctx := context.Background()

	_, err := service.GetAbsenceType(ctx, "test_schema", "tenant-1", "nonexistent")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "absence type not found")
}

func TestGetLeaveBalances_Success(t *testing.T) {
	repo := NewMockAbsenceRepository()
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	service := NewAbsenceService(repo, uuidGen)
	ctx := context.Background()

	key := "tenant-1-emp-1-type-1-2025"
	repo.LeaveBalances[key] = &LeaveBalance{
		ID:            "bal-1",
		TenantID:      "tenant-1",
		EmployeeID:    "emp-1",
		AbsenceTypeID: "type-1",
		Year:          2025,
		EntitledDays:  decimal.NewFromInt(28),
		UsedDays:      decimal.NewFromInt(5),
		RemainingDays: decimal.NewFromInt(23),
	}

	balances, err := service.GetLeaveBalances(ctx, "test_schema", "tenant-1", "emp-1", 2025)

	require.NoError(t, err)
	assert.Len(t, balances, 1)
	assert.True(t, balances[0].EntitledDays.Equal(decimal.NewFromInt(28)))
}

func TestGetLeaveBalance_Success(t *testing.T) {
	repo := NewMockAbsenceRepository()
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	service := NewAbsenceService(repo, uuidGen)
	ctx := context.Background()

	key := "tenant-1-emp-1-type-1-2025"
	repo.LeaveBalances[key] = &LeaveBalance{
		ID:            "bal-1",
		TenantID:      "tenant-1",
		EmployeeID:    "emp-1",
		AbsenceTypeID: "type-1",
		Year:          2025,
		EntitledDays:  decimal.NewFromInt(28),
		RemainingDays: decimal.NewFromInt(28),
	}

	balance, err := service.GetLeaveBalance(ctx, "test_schema", "tenant-1", "emp-1", "type-1", 2025)

	require.NoError(t, err)
	assert.True(t, balance.EntitledDays.Equal(decimal.NewFromInt(28)))
}

func TestCreateLeaveRecord_Success(t *testing.T) {
	repo := NewMockAbsenceRepository()
	uuidGen := &MockUUIDGenerator{prefix: "leave"}
	service := NewAbsenceService(repo, uuidGen)
	ctx := context.Background()

	// Setup absence type
	repo.AbsenceTypes["type-1"] = &AbsenceType{
		ID:       "type-1",
		TenantID: "tenant-1",
		Code:     "ANNUAL_LEAVE",
		IsActive: true,
	}

	// Setup leave balance
	key := "tenant-1-emp-1-type-1-2025"
	repo.LeaveBalances[key] = &LeaveBalance{
		ID:            "bal-1",
		TenantID:      "tenant-1",
		EmployeeID:    "emp-1",
		AbsenceTypeID: "type-1",
		Year:          2025,
		EntitledDays:  decimal.NewFromInt(28),
		RemainingDays: decimal.NewFromInt(28),
	}

	startDate := time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2025, 7, 5, 0, 0, 0, 0, time.UTC)

	req := &CreateLeaveRecordRequest{
		EmployeeID:    "emp-1",
		AbsenceTypeID: "type-1",
		StartDate:     startDate,
		EndDate:       endDate,
		TotalDays:     decimal.NewFromInt(5),
		WorkingDays:   decimal.NewFromInt(5),
		Notes:         "Summer vacation",
	}

	record, err := service.CreateLeaveRecord(ctx, "test_schema", "tenant-1", "user-1", req)

	require.NoError(t, err)
	assert.NotEmpty(t, record.ID)
	assert.Equal(t, LeavePending, record.Status)
	assert.Equal(t, "emp-1", record.EmployeeID)
	assert.Equal(t, "Summer vacation", record.Notes)
	assert.Equal(t, "user-1", record.RequestedBy)

	// Check balance was updated with pending days
	balance := repo.LeaveBalances[key]
	assert.True(t, balance.PendingDays.Equal(decimal.NewFromInt(5)))
}

func TestCreateLeaveRecord_ValidationErrors(t *testing.T) {
	repo := NewMockAbsenceRepository()
	uuidGen := &MockUUIDGenerator{prefix: "leave"}
	service := NewAbsenceService(repo, uuidGen)
	ctx := context.Background()

	startDate := time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2025, 7, 5, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		req     *CreateLeaveRecordRequest
		wantErr string
	}{
		{
			name:    "missing employee ID",
			req:     &CreateLeaveRecordRequest{AbsenceTypeID: "type-1", StartDate: startDate, EndDate: endDate, WorkingDays: decimal.NewFromInt(5)},
			wantErr: "employee ID is required",
		},
		{
			name:    "missing absence type ID",
			req:     &CreateLeaveRecordRequest{EmployeeID: "emp-1", StartDate: startDate, EndDate: endDate, WorkingDays: decimal.NewFromInt(5)},
			wantErr: "absence type ID is required",
		},
		{
			name:    "missing start date",
			req:     &CreateLeaveRecordRequest{EmployeeID: "emp-1", AbsenceTypeID: "type-1", EndDate: endDate, WorkingDays: decimal.NewFromInt(5)},
			wantErr: "start date is required",
		},
		{
			name:    "missing end date",
			req:     &CreateLeaveRecordRequest{EmployeeID: "emp-1", AbsenceTypeID: "type-1", StartDate: startDate, WorkingDays: decimal.NewFromInt(5)},
			wantErr: "end date is required",
		},
		{
			name:    "end before start",
			req:     &CreateLeaveRecordRequest{EmployeeID: "emp-1", AbsenceTypeID: "type-1", StartDate: endDate, EndDate: startDate, WorkingDays: decimal.NewFromInt(5)},
			wantErr: "end date must be after start date",
		},
		{
			name:    "zero working days",
			req:     &CreateLeaveRecordRequest{EmployeeID: "emp-1", AbsenceTypeID: "type-1", StartDate: startDate, EndDate: endDate, WorkingDays: decimal.Zero},
			wantErr: "working days must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.CreateLeaveRecord(ctx, "test_schema", "tenant-1", "user-1", tt.req)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestCreateLeaveRecord_InsufficientBalance(t *testing.T) {
	repo := NewMockAbsenceRepository()
	uuidGen := &MockUUIDGenerator{prefix: "leave"}
	service := NewAbsenceService(repo, uuidGen)
	ctx := context.Background()

	repo.AbsenceTypes["type-1"] = &AbsenceType{
		ID:       "type-1",
		TenantID: "tenant-1",
		Code:     "ANNUAL_LEAVE",
	}

	// Only 3 days remaining
	key := "tenant-1-emp-1-type-1-2025"
	repo.LeaveBalances[key] = &LeaveBalance{
		ID:            "bal-1",
		TenantID:      "tenant-1",
		EmployeeID:    "emp-1",
		AbsenceTypeID: "type-1",
		Year:          2025,
		EntitledDays:  decimal.NewFromInt(28),
		UsedDays:      decimal.NewFromInt(25),
		RemainingDays: decimal.NewFromInt(3),
	}

	req := &CreateLeaveRecordRequest{
		EmployeeID:    "emp-1",
		AbsenceTypeID: "type-1",
		StartDate:     time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC),
		EndDate:       time.Date(2025, 7, 5, 0, 0, 0, 0, time.UTC),
		TotalDays:     decimal.NewFromInt(5),
		WorkingDays:   decimal.NewFromInt(5), // Requesting 5 days but only 3 remaining
	}

	_, err := service.CreateLeaveRecord(ctx, "test_schema", "tenant-1", "user-1", req)

	require.Error(t, err)
	assert.Equal(t, ErrInsufficientLeaveBalance, err)
}

func TestGetLeaveRecord_Success(t *testing.T) {
	repo := NewMockAbsenceRepository()
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	service := NewAbsenceService(repo, uuidGen)
	ctx := context.Background()

	repo.LeaveRecords["rec-1"] = &LeaveRecord{
		ID:         "rec-1",
		TenantID:   "tenant-1",
		EmployeeID: "emp-1",
		Status:     LeavePending,
	}

	record, err := service.GetLeaveRecord(ctx, "test_schema", "tenant-1", "rec-1")

	require.NoError(t, err)
	assert.Equal(t, LeavePending, record.Status)
}

func TestGetLeaveRecord_NotFound(t *testing.T) {
	repo := NewMockAbsenceRepository()
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	service := NewAbsenceService(repo, uuidGen)
	ctx := context.Background()

	_, err := service.GetLeaveRecord(ctx, "test_schema", "tenant-1", "nonexistent")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "leave record not found")
}

func TestListLeaveRecords_Success(t *testing.T) {
	repo := NewMockAbsenceRepository()
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	service := NewAbsenceService(repo, uuidGen)
	ctx := context.Background()

	repo.LeaveRecords["rec-1"] = &LeaveRecord{
		ID:         "rec-1",
		TenantID:   "tenant-1",
		EmployeeID: "emp-1",
		StartDate:  time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC),
		Status:     LeavePending,
	}
	repo.LeaveRecords["rec-2"] = &LeaveRecord{
		ID:         "rec-2",
		TenantID:   "tenant-1",
		EmployeeID: "emp-1",
		StartDate:  time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC),
		Status:     LeaveApproved,
	}

	records, err := service.ListLeaveRecords(ctx, "test_schema", "tenant-1", "emp-1", 2025)

	require.NoError(t, err)
	assert.Len(t, records, 2)
}

func TestApproveLeaveRecord_Success(t *testing.T) {
	repo := NewMockAbsenceRepository()
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	service := NewAbsenceService(repo, uuidGen)
	ctx := context.Background()

	repo.LeaveRecords["rec-1"] = &LeaveRecord{
		ID:            "rec-1",
		TenantID:      "tenant-1",
		EmployeeID:    "emp-1",
		AbsenceTypeID: "type-1",
		StartDate:     time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC),
		WorkingDays:   decimal.NewFromInt(5),
		Status:        LeavePending,
	}

	// Setup balance with pending days
	key := "tenant-1-emp-1-type-1-2025"
	repo.LeaveBalances[key] = &LeaveBalance{
		ID:            "bal-1",
		TenantID:      "tenant-1",
		EmployeeID:    "emp-1",
		AbsenceTypeID: "type-1",
		Year:          2025,
		EntitledDays:  decimal.NewFromInt(28),
		PendingDays:   decimal.NewFromInt(5),
		UsedDays:      decimal.Zero,
		RemainingDays: decimal.NewFromInt(23),
	}

	record, err := service.ApproveLeaveRecord(ctx, "test_schema", "tenant-1", "rec-1", "approver-1")

	require.NoError(t, err)
	assert.Equal(t, LeaveApproved, record.Status)
	assert.Equal(t, "approver-1", record.ApprovedBy)
	assert.NotNil(t, record.ApprovedAt)

	// Check balance was updated
	balance := repo.LeaveBalances[key]
	assert.True(t, balance.PendingDays.IsZero())
	assert.True(t, balance.UsedDays.Equal(decimal.NewFromInt(5)))
}

func TestApproveLeaveRecord_NotPending(t *testing.T) {
	repo := NewMockAbsenceRepository()
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	service := NewAbsenceService(repo, uuidGen)
	ctx := context.Background()

	repo.LeaveRecords["rec-1"] = &LeaveRecord{
		ID:       "rec-1",
		TenantID: "tenant-1",
		Status:   LeaveApproved, // Already approved
	}

	_, err := service.ApproveLeaveRecord(ctx, "test_schema", "tenant-1", "rec-1", "approver-1")

	require.Error(t, err)
	assert.Equal(t, ErrLeaveRecordNotPending, err)
}

func TestRejectLeaveRecord_Success(t *testing.T) {
	repo := NewMockAbsenceRepository()
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	service := NewAbsenceService(repo, uuidGen)
	ctx := context.Background()

	repo.LeaveRecords["rec-1"] = &LeaveRecord{
		ID:            "rec-1",
		TenantID:      "tenant-1",
		EmployeeID:    "emp-1",
		AbsenceTypeID: "type-1",
		StartDate:     time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC),
		WorkingDays:   decimal.NewFromInt(5),
		Status:        LeavePending,
	}

	key := "tenant-1-emp-1-type-1-2025"
	repo.LeaveBalances[key] = &LeaveBalance{
		ID:            "bal-1",
		TenantID:      "tenant-1",
		EmployeeID:    "emp-1",
		AbsenceTypeID: "type-1",
		Year:          2025,
		EntitledDays:  decimal.NewFromInt(28),
		PendingDays:   decimal.NewFromInt(5),
		RemainingDays: decimal.NewFromInt(23),
	}

	record, err := service.RejectLeaveRecord(ctx, "test_schema", "tenant-1", "rec-1", "manager-1", "Staffing shortage")

	require.NoError(t, err)
	assert.Equal(t, LeaveRejected, record.Status)
	assert.Equal(t, "manager-1", record.RejectedBy)
	assert.Equal(t, "Staffing shortage", record.RejectionReason)
	assert.NotNil(t, record.RejectedAt)

	// Check balance was updated - pending should be removed
	balance := repo.LeaveBalances[key]
	assert.True(t, balance.PendingDays.IsZero())
	assert.True(t, balance.RemainingDays.Equal(decimal.NewFromInt(28)))
}

func TestRejectLeaveRecord_NotPending(t *testing.T) {
	repo := NewMockAbsenceRepository()
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	service := NewAbsenceService(repo, uuidGen)
	ctx := context.Background()

	repo.LeaveRecords["rec-1"] = &LeaveRecord{
		ID:       "rec-1",
		TenantID: "tenant-1",
		Status:   LeaveApproved,
	}

	_, err := service.RejectLeaveRecord(ctx, "test_schema", "tenant-1", "rec-1", "manager-1", "reason")

	require.Error(t, err)
	assert.Equal(t, ErrLeaveRecordNotPending, err)
}

func TestCancelLeaveRecord_Pending(t *testing.T) {
	repo := NewMockAbsenceRepository()
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	service := NewAbsenceService(repo, uuidGen)
	ctx := context.Background()

	repo.LeaveRecords["rec-1"] = &LeaveRecord{
		ID:            "rec-1",
		TenantID:      "tenant-1",
		EmployeeID:    "emp-1",
		AbsenceTypeID: "type-1",
		StartDate:     time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC),
		WorkingDays:   decimal.NewFromInt(5),
		Status:        LeavePending,
	}

	key := "tenant-1-emp-1-type-1-2025"
	repo.LeaveBalances[key] = &LeaveBalance{
		ID:            "bal-1",
		TenantID:      "tenant-1",
		EmployeeID:    "emp-1",
		AbsenceTypeID: "type-1",
		Year:          2025,
		EntitledDays:  decimal.NewFromInt(28),
		PendingDays:   decimal.NewFromInt(5),
		RemainingDays: decimal.NewFromInt(23),
	}

	record, err := service.CancelLeaveRecord(ctx, "test_schema", "tenant-1", "rec-1", "emp-1")

	require.NoError(t, err)
	assert.Equal(t, LeaveCanceled, record.Status)

	// Pending days should be returned
	balance := repo.LeaveBalances[key]
	assert.True(t, balance.PendingDays.IsZero())
	assert.True(t, balance.RemainingDays.Equal(decimal.NewFromInt(28)))
}

func TestCancelLeaveRecord_Approved(t *testing.T) {
	repo := NewMockAbsenceRepository()
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	service := NewAbsenceService(repo, uuidGen)
	ctx := context.Background()

	repo.LeaveRecords["rec-1"] = &LeaveRecord{
		ID:            "rec-1",
		TenantID:      "tenant-1",
		EmployeeID:    "emp-1",
		AbsenceTypeID: "type-1",
		StartDate:     time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC),
		WorkingDays:   decimal.NewFromInt(5),
		Status:        LeaveApproved,
	}

	key := "tenant-1-emp-1-type-1-2025"
	repo.LeaveBalances[key] = &LeaveBalance{
		ID:            "bal-1",
		TenantID:      "tenant-1",
		EmployeeID:    "emp-1",
		AbsenceTypeID: "type-1",
		Year:          2025,
		EntitledDays:  decimal.NewFromInt(28),
		UsedDays:      decimal.NewFromInt(5),
		RemainingDays: decimal.NewFromInt(23),
	}

	record, err := service.CancelLeaveRecord(ctx, "test_schema", "tenant-1", "rec-1", "emp-1")

	require.NoError(t, err)
	assert.Equal(t, LeaveCanceled, record.Status)

	// Used days should be returned
	balance := repo.LeaveBalances[key]
	assert.True(t, balance.UsedDays.IsZero())
	assert.True(t, balance.RemainingDays.Equal(decimal.NewFromInt(28)))
}

func TestCancelLeaveRecord_AlreadyRejected(t *testing.T) {
	repo := NewMockAbsenceRepository()
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	service := NewAbsenceService(repo, uuidGen)
	ctx := context.Background()

	repo.LeaveRecords["rec-1"] = &LeaveRecord{
		ID:       "rec-1",
		TenantID: "tenant-1",
		Status:   LeaveRejected,
	}

	_, err := service.CancelLeaveRecord(ctx, "test_schema", "tenant-1", "rec-1", "emp-1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "can only cancel pending or approved")
}

func TestInitializeEmployeeLeaveBalances_Success(t *testing.T) {
	repo := NewMockAbsenceRepository()
	uuidGen := &MockUUIDGenerator{prefix: "bal"}
	service := NewAbsenceService(repo, uuidGen)
	ctx := context.Background()

	// Setup absence types
	repo.AbsenceTypes["type-1"] = &AbsenceType{
		ID:                 "type-1",
		TenantID:           "tenant-1",
		Code:               "ANNUAL_LEAVE",
		DefaultDaysPerYear: decimal.NewFromInt(28),
		IsActive:           true,
	}
	repo.AbsenceTypes["type-2"] = &AbsenceType{
		ID:                 "type-2",
		TenantID:           "tenant-1",
		Code:               "SICK_LEAVE",
		DefaultDaysPerYear: decimal.Zero,
		IsActive:           true,
	}

	balances, err := service.InitializeEmployeeLeaveBalances(ctx, "test_schema", "tenant-1", "emp-1", 2025)

	require.NoError(t, err)
	assert.Len(t, balances, 2)

	// Verify balances were created
	assert.Len(t, repo.LeaveBalances, 2)
}

func TestInitializeEmployeeLeaveBalances_ExistingBalance(t *testing.T) {
	repo := NewMockAbsenceRepository()
	uuidGen := &MockUUIDGenerator{prefix: "bal"}
	service := NewAbsenceService(repo, uuidGen)
	ctx := context.Background()

	// Setup absence type
	repo.AbsenceTypes["type-1"] = &AbsenceType{
		ID:                 "type-1",
		TenantID:           "tenant-1",
		Code:               "ANNUAL_LEAVE",
		DefaultDaysPerYear: decimal.NewFromInt(28),
		IsActive:           true,
	}

	// Pre-existing balance
	key := "tenant-1-emp-1-type-1-2025"
	repo.LeaveBalances[key] = &LeaveBalance{
		ID:            "existing-bal",
		TenantID:      "tenant-1",
		EmployeeID:    "emp-1",
		AbsenceTypeID: "type-1",
		Year:          2025,
		EntitledDays:  decimal.NewFromInt(30), // Custom entitlement
		RemainingDays: decimal.NewFromInt(30),
	}

	balances, err := service.InitializeEmployeeLeaveBalances(ctx, "test_schema", "tenant-1", "emp-1", 2025)

	require.NoError(t, err)
	assert.Len(t, balances, 1)
	assert.Equal(t, "existing-bal", balances[0].ID)                        // Should return existing, not create new
	assert.True(t, balances[0].EntitledDays.Equal(decimal.NewFromInt(30))) // Keep custom value
}

func TestUpdateLeaveBalance_Success(t *testing.T) {
	repo := NewMockAbsenceRepository()
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	service := NewAbsenceService(repo, uuidGen)
	ctx := context.Background()

	key := "tenant-1-emp-1-type-1-2025"
	repo.LeaveBalances[key] = &LeaveBalance{
		ID:            "bal-1",
		TenantID:      "tenant-1",
		EmployeeID:    "emp-1",
		AbsenceTypeID: "type-1",
		Year:          2025,
		EntitledDays:  decimal.NewFromInt(28),
		CarryoverDays: decimal.Zero,
		UsedDays:      decimal.Zero,
		PendingDays:   decimal.Zero,
		RemainingDays: decimal.NewFromInt(28),
	}

	newEntitled := decimal.NewFromInt(30)
	newCarryover := decimal.NewFromInt(5)
	req := &UpdateLeaveBalanceRequest{
		EntitledDays:  &newEntitled,
		CarryoverDays: &newCarryover,
		Notes:         "Adjusted for seniority",
	}

	balance, err := service.UpdateLeaveBalance(ctx, "test_schema", "tenant-1", "emp-1", "type-1", 2025, req)

	require.NoError(t, err)
	assert.True(t, balance.EntitledDays.Equal(decimal.NewFromInt(30)))
	assert.True(t, balance.CarryoverDays.Equal(decimal.NewFromInt(5)))
	assert.True(t, balance.RemainingDays.Equal(decimal.NewFromInt(35))) // 30 + 5 - 0 - 0
	assert.Equal(t, "Adjusted for seniority", balance.Notes)
}
