package payroll

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// ABSENCE SERVICE TESTS
// ============================================================================

type fakeLeaveEvidenceEvaluator struct {
	compliant bool
	err       error
	results   []documents.EvidencePolicyResult
	request   *documents.EvidencePolicyRequest
}

func (f *fakeLeaveEvidenceEvaluator) EvaluateEvidencePolicy(_ context.Context, _, _ string, req *documents.EvidencePolicyRequest) ([]documents.EvidencePolicyResult, error) {
	f.request = req
	if f.err != nil {
		return nil, f.err
	}
	if f.results != nil {
		return f.results, nil
	}
	return []documents.EvidencePolicyResult{{
		EntityType: req.EntityType,
		EntityID:   req.EntityIDs[0],
		Compliant:  f.compliant,
	}}, nil
}

func TestNewAbsenceService(t *testing.T) {
	repo := NewMockAbsenceRepository()
	uuidGen := &MockUUIDGenerator{prefix: "test"}

	service := NewAbsenceService(repo, uuidGen)

	assert.NotNil(t, service)
	assert.Equal(t, repo, service.repo)
	assert.Equal(t, uuidGen, service.uuid)
}

func TestNewAbsenceServiceWithPool_NilPool(t *testing.T) {
	service := NewAbsenceServiceWithPool(nil)

	require.NotNil(t, service)
	assert.Nil(t, service.repo)
	assert.IsType(t, &DefaultUUIDGenerator{}, service.uuid)
	assert.Nil(t, service.evidence)
}

func TestNewAbsenceServiceWithPoolAndEvidence_NilPool(t *testing.T) {
	evidence := &fakeLeaveEvidenceEvaluator{}

	service := NewAbsenceServiceWithPoolAndEvidence(nil, evidence)

	require.NotNil(t, service)
	assert.Nil(t, service.repo)
	assert.IsType(t, &DefaultUUIDGenerator{}, service.uuid)
	assert.Equal(t, evidence, service.evidence)
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

func TestGetLeaveBalances_RepositoryError(t *testing.T) {
	repo := NewMockAbsenceRepository()
	repo.ListLeaveBalancesErr = errors.New("database error")
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	service := NewAbsenceService(repo, uuidGen)
	ctx := context.Background()

	_, err := service.GetLeaveBalances(ctx, "test_schema", "tenant-1", "emp-1", 2025)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "list leave balances")
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

func TestGetLeaveBalance_RepositoryError(t *testing.T) {
	repo := NewMockAbsenceRepository()
	repo.GetLeaveBalanceErr = errors.New("database error")
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	service := NewAbsenceService(repo, uuidGen)
	ctx := context.Background()

	_, err := service.GetLeaveBalance(ctx, "test_schema", "tenant-1", "emp-1", "type-1", 2025)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "get leave balance")
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
		{
			name:    "negative working days",
			req:     &CreateLeaveRecordRequest{EmployeeID: "emp-1", AbsenceTypeID: "type-1", StartDate: startDate, EndDate: endDate, WorkingDays: decimal.NewFromInt(-1)},
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

func TestCreateLeaveRecord_ErrorBranches(t *testing.T) {
	ctx := context.Background()
	startDate := time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2025, 7, 5, 0, 0, 0, 0, time.UTC)
	validReq := func() *CreateLeaveRecordRequest {
		return &CreateLeaveRecordRequest{
			EmployeeID:    "emp-1",
			AbsenceTypeID: "type-1",
			StartDate:     startDate,
			EndDate:       endDate,
			TotalDays:     decimal.NewFromInt(5),
			WorkingDays:   decimal.NewFromInt(5),
		}
	}

	t.Run("absence type lookup error", func(t *testing.T) {
		repo := NewMockAbsenceRepository()
		repo.GetAbsenceTypeErr = errors.New("database error")
		service := NewAbsenceService(repo, &MockUUIDGenerator{prefix: "leave"})

		_, err := service.CreateLeaveRecord(ctx, "test_schema", "tenant-1", "user-1", validReq())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "get absence type")
	})

	t.Run("missing balance still creates record", func(t *testing.T) {
		repo := NewMockAbsenceRepository()
		repo.AbsenceTypes["type-1"] = &AbsenceType{ID: "type-1", TenantID: "tenant-1"}
		service := NewAbsenceService(repo, &MockUUIDGenerator{prefix: "leave"})

		record, err := service.CreateLeaveRecord(ctx, "test_schema", "tenant-1", "user-1", validReq())

		require.NoError(t, err)
		assert.Equal(t, LeavePending, record.Status)
		assert.Len(t, repo.LeaveRecords, 1)
	})

	t.Run("balance update error", func(t *testing.T) {
		repo := NewMockAbsenceRepository()
		repo.AbsenceTypes["type-1"] = &AbsenceType{ID: "type-1", TenantID: "tenant-1"}
		repo.LeaveBalances["tenant-1-emp-1-type-1-2025"] = &LeaveBalance{
			ID:            "bal-1",
			TenantID:      "tenant-1",
			EmployeeID:    "emp-1",
			AbsenceTypeID: "type-1",
			Year:          2025,
			EntitledDays:  decimal.NewFromInt(28),
			RemainingDays: decimal.NewFromInt(28),
		}
		repo.UpdateLeaveBalanceErr = errors.New("database error")
		service := NewAbsenceService(repo, &MockUUIDGenerator{prefix: "leave"})

		_, err := service.CreateLeaveRecord(ctx, "test_schema", "tenant-1", "user-1", validReq())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "update leave balance")
	})

	t.Run("create leave record error", func(t *testing.T) {
		repo := NewMockAbsenceRepository()
		repo.AbsenceTypes["type-1"] = &AbsenceType{ID: "type-1", TenantID: "tenant-1"}
		repo.CreateLeaveRecordErr = errors.New("database error")
		service := NewAbsenceService(repo, &MockUUIDGenerator{prefix: "leave"})

		_, err := service.CreateLeaveRecord(ctx, "test_schema", "tenant-1", "user-1", validReq())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "create leave record")
	})
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

func TestListLeaveRecords_RepositoryError(t *testing.T) {
	repo := NewMockAbsenceRepository()
	repo.ListLeaveRecordsErr = errors.New("database error")
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	service := NewAbsenceService(repo, uuidGen)
	ctx := context.Background()

	_, err := service.ListLeaveRecords(ctx, "test_schema", "tenant-1", "emp-1", 2025)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "list leave records")
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
	repo.AbsenceTypes["type-1"] = &AbsenceType{
		ID:               "type-1",
		TenantID:         "tenant-1",
		Code:             "ANNUAL_LEAVE",
		RequiresDocument: false,
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

func TestApproveLeaveRecord_RequiresApprovedDocument(t *testing.T) {
	repo := NewMockAbsenceRepository()
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	evidence := &fakeLeaveEvidenceEvaluator{compliant: false}
	service := NewAbsenceServiceWithEvidence(repo, uuidGen, evidence)
	ctx := context.Background()

	repo.AbsenceTypes["type-1"] = &AbsenceType{
		ID:               "type-1",
		TenantID:         "tenant-1",
		Code:             "SICK_LEAVE",
		RequiresDocument: true,
		DocumentType:     "medical_certificate",
	}
	repo.LeaveRecords["rec-1"] = &LeaveRecord{
		ID:            "rec-1",
		TenantID:      "tenant-1",
		EmployeeID:    "emp-1",
		AbsenceTypeID: "type-1",
		StartDate:     time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC),
		WorkingDays:   decimal.NewFromInt(2),
		Status:        LeavePending,
	}

	_, err := service.ApproveLeaveRecord(ctx, "test_schema", "tenant-1", "rec-1", "approver-1")

	require.ErrorIs(t, err, ErrApprovedLeaveDocumentRequired)
	var conflict *LeaveEvidencePolicyConflictError
	require.ErrorAs(t, err, &conflict)
	require.Len(t, conflict.Results, 1)
	assert.Equal(t, "rec-1", conflict.Results[0].EntityID)
	require.NotNil(t, evidence.request)
	assert.Equal(t, documents.EntityTypeLeaveRecord, evidence.request.EntityType)
	assert.Equal(t, []string{"rec-1"}, evidence.request.EntityIDs)
	assert.Equal(t, []string{documents.DocumentTypeSupportingDocument, documents.DocumentTypeTaxSupport}, evidence.request.Rules[0].DocumentTypes)
	assert.True(t, evidence.request.Rules[0].RequireApproved)
	assert.Equal(t, LeavePending, repo.LeaveRecords["rec-1"].Status)
}

func TestApproveLeaveRecord_WithApprovedDocument(t *testing.T) {
	repo := NewMockAbsenceRepository()
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	evidence := &fakeLeaveEvidenceEvaluator{compliant: true}
	service := NewAbsenceServiceWithEvidence(repo, uuidGen, evidence)
	ctx := context.Background()

	repo.AbsenceTypes["type-1"] = &AbsenceType{
		ID:               "type-1",
		TenantID:         "tenant-1",
		Code:             "SICK_LEAVE",
		RequiresDocument: true,
	}
	repo.LeaveRecords["rec-1"] = &LeaveRecord{
		ID:            "rec-1",
		TenantID:      "tenant-1",
		EmployeeID:    "emp-1",
		AbsenceTypeID: "type-1",
		StartDate:     time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC),
		WorkingDays:   decimal.NewFromInt(2),
		Status:        LeavePending,
	}

	record, err := service.ApproveLeaveRecord(ctx, "test_schema", "tenant-1", "rec-1", "approver-1")

	require.NoError(t, err)
	assert.Equal(t, LeaveApproved, record.Status)
	assert.Equal(t, "approver-1", record.ApprovedBy)
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

func TestApproveLeaveRecord_ErrorBranches(t *testing.T) {
	ctx := context.Background()
	pendingRecord := func() *LeaveRecord {
		return &LeaveRecord{
			ID:            "rec-1",
			TenantID:      "tenant-1",
			EmployeeID:    "emp-1",
			AbsenceTypeID: "type-1",
			StartDate:     time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC),
			WorkingDays:   decimal.NewFromInt(5),
			Status:        LeavePending,
		}
	}

	t.Run("record lookup error", func(t *testing.T) {
		repo := NewMockAbsenceRepository()
		repo.GetLeaveRecordErr = errors.New("database error")
		service := NewAbsenceService(repo, &MockUUIDGenerator{prefix: "test"})

		_, err := service.ApproveLeaveRecord(ctx, "test_schema", "tenant-1", "rec-1", "approver-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "get leave record")
	})

	t.Run("absence type lookup error", func(t *testing.T) {
		repo := NewMockAbsenceRepository()
		repo.LeaveRecords["rec-1"] = pendingRecord()
		repo.GetAbsenceTypeErr = errors.New("database error")
		service := NewAbsenceService(repo, &MockUUIDGenerator{prefix: "test"})

		_, err := service.ApproveLeaveRecord(ctx, "test_schema", "tenant-1", "rec-1", "approver-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "get absence type")
	})

	t.Run("required document without evidence service", func(t *testing.T) {
		repo := NewMockAbsenceRepository()
		repo.LeaveRecords["rec-1"] = pendingRecord()
		repo.AbsenceTypes["type-1"] = &AbsenceType{ID: "type-1", TenantID: "tenant-1", RequiresDocument: true}
		service := NewAbsenceService(repo, &MockUUIDGenerator{prefix: "test"})

		_, err := service.ApproveLeaveRecord(ctx, "test_schema", "tenant-1", "rec-1", "approver-1")

		require.ErrorIs(t, err, ErrApprovedLeaveDocumentRequired)
		assert.Contains(t, err.Error(), "document evidence service is unavailable")
	})

	t.Run("evidence evaluation error", func(t *testing.T) {
		repo := NewMockAbsenceRepository()
		repo.LeaveRecords["rec-1"] = pendingRecord()
		repo.AbsenceTypes["type-1"] = &AbsenceType{ID: "type-1", TenantID: "tenant-1", RequiresDocument: true}
		evidence := &fakeLeaveEvidenceEvaluator{err: errors.New("service unavailable")}
		service := NewAbsenceServiceWithEvidence(repo, &MockUUIDGenerator{prefix: "test"}, evidence)

		_, err := service.ApproveLeaveRecord(ctx, "test_schema", "tenant-1", "rec-1", "approver-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "evaluate leave record evidence")
	})

	t.Run("evidence no results", func(t *testing.T) {
		repo := NewMockAbsenceRepository()
		repo.LeaveRecords["rec-1"] = pendingRecord()
		repo.AbsenceTypes["type-1"] = &AbsenceType{ID: "type-1", TenantID: "tenant-1", RequiresDocument: true}
		evidence := &fakeLeaveEvidenceEvaluator{results: []documents.EvidencePolicyResult{}}
		service := NewAbsenceServiceWithEvidence(repo, &MockUUIDGenerator{prefix: "test"}, evidence)

		_, err := service.ApproveLeaveRecord(ctx, "test_schema", "tenant-1", "rec-1", "approver-1")

		require.ErrorIs(t, err, ErrApprovedLeaveDocumentRequired)
		var conflict *LeaveEvidencePolicyConflictError
		require.ErrorAs(t, err, &conflict)
		assert.Empty(t, conflict.Results)
		assert.Contains(t, err.Error(), "before approving leave record rec-1")
	})

	t.Run("update record error", func(t *testing.T) {
		repo := NewMockAbsenceRepository()
		repo.LeaveRecords["rec-1"] = pendingRecord()
		repo.AbsenceTypes["type-1"] = &AbsenceType{ID: "type-1", TenantID: "tenant-1"}
		repo.UpdateLeaveRecordErr = errors.New("database error")
		service := NewAbsenceService(repo, &MockUUIDGenerator{prefix: "test"})

		_, err := service.ApproveLeaveRecord(ctx, "test_schema", "tenant-1", "rec-1", "approver-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "update leave record")
	})

	t.Run("missing balance still approves record", func(t *testing.T) {
		repo := NewMockAbsenceRepository()
		repo.LeaveRecords["rec-1"] = pendingRecord()
		repo.AbsenceTypes["type-1"] = &AbsenceType{ID: "type-1", TenantID: "tenant-1"}
		service := NewAbsenceService(repo, &MockUUIDGenerator{prefix: "test"})

		record, err := service.ApproveLeaveRecord(ctx, "test_schema", "tenant-1", "rec-1", "approver-1")

		require.NoError(t, err)
		assert.Equal(t, LeaveApproved, record.Status)
	})

	t.Run("balance update error", func(t *testing.T) {
		repo := NewMockAbsenceRepository()
		repo.LeaveRecords["rec-1"] = pendingRecord()
		repo.AbsenceTypes["type-1"] = &AbsenceType{ID: "type-1", TenantID: "tenant-1"}
		repo.LeaveBalances["tenant-1-emp-1-type-1-2025"] = &LeaveBalance{
			ID:            "bal-1",
			TenantID:      "tenant-1",
			EmployeeID:    "emp-1",
			AbsenceTypeID: "type-1",
			Year:          2025,
			EntitledDays:  decimal.NewFromInt(28),
			PendingDays:   decimal.NewFromInt(2),
			RemainingDays: decimal.NewFromInt(26),
		}
		repo.UpdateLeaveBalanceErr = errors.New("database error")
		service := NewAbsenceService(repo, &MockUUIDGenerator{prefix: "test"})

		_, err := service.ApproveLeaveRecord(ctx, "test_schema", "tenant-1", "rec-1", "approver-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "update leave balance")
		assert.True(t, repo.LeaveBalances["tenant-1-emp-1-type-1-2025"].PendingDays.IsZero())
	})
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

func TestRejectLeaveRecord_ErrorBranches(t *testing.T) {
	ctx := context.Background()
	pendingRecord := func() *LeaveRecord {
		return &LeaveRecord{
			ID:            "rec-1",
			TenantID:      "tenant-1",
			EmployeeID:    "emp-1",
			AbsenceTypeID: "type-1",
			StartDate:     time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC),
			WorkingDays:   decimal.NewFromInt(5),
			Status:        LeavePending,
		}
	}

	t.Run("record lookup error", func(t *testing.T) {
		repo := NewMockAbsenceRepository()
		repo.GetLeaveRecordErr = errors.New("database error")
		service := NewAbsenceService(repo, &MockUUIDGenerator{prefix: "test"})

		_, err := service.RejectLeaveRecord(ctx, "test_schema", "tenant-1", "rec-1", "manager-1", "reason")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "get leave record")
	})

	t.Run("update record error", func(t *testing.T) {
		repo := NewMockAbsenceRepository()
		repo.LeaveRecords["rec-1"] = pendingRecord()
		repo.UpdateLeaveRecordErr = errors.New("database error")
		service := NewAbsenceService(repo, &MockUUIDGenerator{prefix: "test"})

		_, err := service.RejectLeaveRecord(ctx, "test_schema", "tenant-1", "rec-1", "manager-1", "reason")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "update leave record")
	})

	t.Run("missing balance still rejects record", func(t *testing.T) {
		repo := NewMockAbsenceRepository()
		repo.LeaveRecords["rec-1"] = pendingRecord()
		service := NewAbsenceService(repo, &MockUUIDGenerator{prefix: "test"})

		record, err := service.RejectLeaveRecord(ctx, "test_schema", "tenant-1", "rec-1", "manager-1", "reason")

		require.NoError(t, err)
		assert.Equal(t, LeaveRejected, record.Status)
	})

	t.Run("balance update error", func(t *testing.T) {
		repo := NewMockAbsenceRepository()
		repo.LeaveRecords["rec-1"] = pendingRecord()
		repo.LeaveBalances["tenant-1-emp-1-type-1-2025"] = &LeaveBalance{
			ID:            "bal-1",
			TenantID:      "tenant-1",
			EmployeeID:    "emp-1",
			AbsenceTypeID: "type-1",
			Year:          2025,
			EntitledDays:  decimal.NewFromInt(28),
			PendingDays:   decimal.NewFromInt(2),
			RemainingDays: decimal.NewFromInt(26),
		}
		repo.UpdateLeaveBalanceErr = errors.New("database error")
		service := NewAbsenceService(repo, &MockUUIDGenerator{prefix: "test"})

		_, err := service.RejectLeaveRecord(ctx, "test_schema", "tenant-1", "rec-1", "manager-1", "reason")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "update leave balance")
		assert.True(t, repo.LeaveBalances["tenant-1-emp-1-type-1-2025"].PendingDays.IsZero())
	})
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

func TestCancelLeaveRecord_ErrorBranches(t *testing.T) {
	ctx := context.Background()
	pendingRecord := func() *LeaveRecord {
		return &LeaveRecord{
			ID:            "rec-1",
			TenantID:      "tenant-1",
			EmployeeID:    "emp-1",
			AbsenceTypeID: "type-1",
			StartDate:     time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC),
			WorkingDays:   decimal.NewFromInt(5),
			Status:        LeavePending,
		}
	}

	t.Run("record lookup error", func(t *testing.T) {
		repo := NewMockAbsenceRepository()
		repo.GetLeaveRecordErr = errors.New("database error")
		service := NewAbsenceService(repo, &MockUUIDGenerator{prefix: "test"})

		_, err := service.CancelLeaveRecord(ctx, "test_schema", "tenant-1", "rec-1", "emp-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "get leave record")
	})

	t.Run("update record error", func(t *testing.T) {
		repo := NewMockAbsenceRepository()
		repo.LeaveRecords["rec-1"] = pendingRecord()
		repo.UpdateLeaveRecordErr = errors.New("database error")
		service := NewAbsenceService(repo, &MockUUIDGenerator{prefix: "test"})

		_, err := service.CancelLeaveRecord(ctx, "test_schema", "tenant-1", "rec-1", "emp-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "update leave record")
	})

	t.Run("missing balance still cancels record", func(t *testing.T) {
		repo := NewMockAbsenceRepository()
		repo.LeaveRecords["rec-1"] = pendingRecord()
		service := NewAbsenceService(repo, &MockUUIDGenerator{prefix: "test"})

		record, err := service.CancelLeaveRecord(ctx, "test_schema", "tenant-1", "rec-1", "emp-1")

		require.NoError(t, err)
		assert.Equal(t, LeaveCanceled, record.Status)
	})

	t.Run("pending balance update error", func(t *testing.T) {
		repo := NewMockAbsenceRepository()
		repo.LeaveRecords["rec-1"] = pendingRecord()
		repo.LeaveBalances["tenant-1-emp-1-type-1-2025"] = &LeaveBalance{
			ID:            "bal-1",
			TenantID:      "tenant-1",
			EmployeeID:    "emp-1",
			AbsenceTypeID: "type-1",
			Year:          2025,
			EntitledDays:  decimal.NewFromInt(28),
			PendingDays:   decimal.NewFromInt(2),
			UsedDays:      decimal.Zero,
			RemainingDays: decimal.NewFromInt(26),
		}
		repo.UpdateLeaveBalanceErr = errors.New("database error")
		service := NewAbsenceService(repo, &MockUUIDGenerator{prefix: "test"})

		_, err := service.CancelLeaveRecord(ctx, "test_schema", "tenant-1", "rec-1", "emp-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "update leave balance")
		assert.True(t, repo.LeaveBalances["tenant-1-emp-1-type-1-2025"].PendingDays.IsZero())
	})

	t.Run("approved balance update error", func(t *testing.T) {
		repo := NewMockAbsenceRepository()
		record := pendingRecord()
		record.Status = LeaveApproved
		repo.LeaveRecords["rec-1"] = record
		repo.LeaveBalances["tenant-1-emp-1-type-1-2025"] = &LeaveBalance{
			ID:            "bal-1",
			TenantID:      "tenant-1",
			EmployeeID:    "emp-1",
			AbsenceTypeID: "type-1",
			Year:          2025,
			EntitledDays:  decimal.NewFromInt(28),
			PendingDays:   decimal.Zero,
			UsedDays:      decimal.NewFromInt(2),
			RemainingDays: decimal.NewFromInt(26),
		}
		repo.UpdateLeaveBalanceErr = errors.New("database error")
		service := NewAbsenceService(repo, &MockUUIDGenerator{prefix: "test"})

		_, err := service.CancelLeaveRecord(ctx, "test_schema", "tenant-1", "rec-1", "emp-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "update leave balance")
		assert.True(t, repo.LeaveBalances["tenant-1-emp-1-type-1-2025"].UsedDays.IsZero())
	})
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

func TestInitializeEmployeeLeaveBalances_ErrorBranches(t *testing.T) {
	ctx := context.Background()

	t.Run("list absence types error", func(t *testing.T) {
		repo := NewMockAbsenceRepository()
		repo.ListAbsenceTypesErr = errors.New("database error")
		service := NewAbsenceService(repo, &MockUUIDGenerator{prefix: "bal"})

		_, err := service.InitializeEmployeeLeaveBalances(ctx, "test_schema", "tenant-1", "emp-1", 2025)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "list absence types")
	})

	t.Run("create leave balance error", func(t *testing.T) {
		repo := NewMockAbsenceRepository()
		repo.AbsenceTypes["type-1"] = &AbsenceType{
			ID:                 "type-1",
			TenantID:           "tenant-1",
			DefaultDaysPerYear: decimal.NewFromInt(28),
			IsActive:           true,
		}
		repo.CreateLeaveBalanceErr = errors.New("database error")
		service := NewAbsenceService(repo, &MockUUIDGenerator{prefix: "bal"})

		_, err := service.InitializeEmployeeLeaveBalances(ctx, "test_schema", "tenant-1", "emp-1", 2025)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "create leave balance")
	})
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

func TestUpdateLeaveBalance_ErrorBranches(t *testing.T) {
	ctx := context.Background()

	t.Run("lookup error", func(t *testing.T) {
		repo := NewMockAbsenceRepository()
		repo.GetLeaveBalanceErr = errors.New("database error")
		service := NewAbsenceService(repo, &MockUUIDGenerator{prefix: "test"})

		_, err := service.UpdateLeaveBalance(ctx, "test_schema", "tenant-1", "emp-1", "type-1", 2025, &UpdateLeaveBalanceRequest{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "get leave balance")
	})

	t.Run("update error", func(t *testing.T) {
		repo := NewMockAbsenceRepository()
		repo.LeaveBalances["tenant-1-emp-1-type-1-2025"] = &LeaveBalance{
			ID:            "bal-1",
			TenantID:      "tenant-1",
			EmployeeID:    "emp-1",
			AbsenceTypeID: "type-1",
			Year:          2025,
			EntitledDays:  decimal.NewFromInt(28),
			CarryoverDays: decimal.NewFromInt(2),
			UsedDays:      decimal.NewFromInt(4),
			PendingDays:   decimal.NewFromInt(1),
			RemainingDays: decimal.NewFromInt(25),
			Notes:         "existing note",
		}
		repo.UpdateLeaveBalanceErr = errors.New("database error")
		service := NewAbsenceService(repo, &MockUUIDGenerator{prefix: "test"})

		_, err := service.UpdateLeaveBalance(ctx, "test_schema", "tenant-1", "emp-1", "type-1", 2025, &UpdateLeaveBalanceRequest{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "update leave balance")
		assert.Equal(t, "existing note", repo.LeaveBalances["tenant-1-emp-1-type-1-2025"].Notes)
	})
}
