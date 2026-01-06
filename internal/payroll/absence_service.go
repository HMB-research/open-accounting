package payroll

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// AbsenceService handles leave/absence management business logic
type AbsenceService struct {
	repo AbsenceRepository
	uuid UUIDGenerator
}

// NewAbsenceService creates a new absence service
func NewAbsenceService(repo AbsenceRepository, uuid UUIDGenerator) *AbsenceService {
	return &AbsenceService{
		repo: repo,
		uuid: uuid,
	}
}

// NewAbsenceServiceWithPool creates a new absence service using a PostgreSQL connection pool
func NewAbsenceServiceWithPool(pool *pgxpool.Pool) *AbsenceService {
	pgRepo := NewPostgresRepository(pool)
	absenceRepo := NewAbsencePostgresRepository(pgRepo)
	return &AbsenceService{
		repo: absenceRepo,
		uuid: &DefaultUUIDGenerator{},
	}
}

// ListAbsenceTypes returns all absence types for a tenant
func (s *AbsenceService) ListAbsenceTypes(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]AbsenceType, error) {
	types, err := s.repo.ListAbsenceTypes(ctx, schemaName, tenantID, activeOnly)
	if err != nil {
		return nil, fmt.Errorf("list absence types: %w", err)
	}
	return types, nil
}

// GetAbsenceType returns a specific absence type
func (s *AbsenceService) GetAbsenceType(ctx context.Context, schemaName, tenantID, typeID string) (*AbsenceType, error) {
	t, err := s.repo.GetAbsenceType(ctx, schemaName, tenantID, typeID)
	if err != nil {
		return nil, fmt.Errorf("get absence type: %w", err)
	}
	return t, nil
}

// GetLeaveBalances returns leave balances for an employee for a specific year
func (s *AbsenceService) GetLeaveBalances(ctx context.Context, schemaName, tenantID, employeeID string, year int) ([]LeaveBalance, error) {
	balances, err := s.repo.ListLeaveBalances(ctx, schemaName, tenantID, employeeID, year)
	if err != nil {
		return nil, fmt.Errorf("list leave balances: %w", err)
	}
	return balances, nil
}

// GetLeaveBalance returns a specific leave balance
func (s *AbsenceService) GetLeaveBalance(ctx context.Context, schemaName, tenantID, employeeID, absenceTypeID string, year int) (*LeaveBalance, error) {
	balance, err := s.repo.GetLeaveBalance(ctx, schemaName, tenantID, employeeID, absenceTypeID, year)
	if err != nil {
		return nil, fmt.Errorf("get leave balance: %w", err)
	}
	return balance, nil
}

// UpdateLeaveBalance updates a leave balance
func (s *AbsenceService) UpdateLeaveBalance(ctx context.Context, schemaName, tenantID, employeeID, absenceTypeID string, year int, req *UpdateLeaveBalanceRequest) (*LeaveBalance, error) {
	// Get existing balance
	balance, err := s.repo.GetLeaveBalance(ctx, schemaName, tenantID, employeeID, absenceTypeID, year)
	if err != nil {
		return nil, fmt.Errorf("get leave balance: %w", err)
	}

	// Update fields
	if req.EntitledDays != nil {
		balance.EntitledDays = *req.EntitledDays
	}
	if req.CarryoverDays != nil {
		balance.CarryoverDays = *req.CarryoverDays
	}
	if req.Notes != "" {
		balance.Notes = req.Notes
	}
	balance.UpdatedAt = time.Now()

	// Recalculate remaining days
	balance.RemainingDays = balance.EntitledDays.Add(balance.CarryoverDays).Sub(balance.UsedDays).Sub(balance.PendingDays)

	if err := s.repo.UpdateLeaveBalance(ctx, schemaName, balance); err != nil {
		return nil, fmt.Errorf("update leave balance: %w", err)
	}

	return balance, nil
}

// CreateLeaveRecord creates a new leave request
func (s *AbsenceService) CreateLeaveRecord(ctx context.Context, schemaName, tenantID, requestedBy string, req *CreateLeaveRecordRequest) (*LeaveRecord, error) {
	// Validate request
	if req.EmployeeID == "" {
		return nil, fmt.Errorf("employee ID is required")
	}
	if req.AbsenceTypeID == "" {
		return nil, fmt.Errorf("absence type ID is required")
	}
	if req.StartDate.IsZero() {
		return nil, fmt.Errorf("start date is required")
	}
	if req.EndDate.IsZero() {
		return nil, fmt.Errorf("end date is required")
	}
	if req.EndDate.Before(req.StartDate) {
		return nil, fmt.Errorf("end date must be after start date")
	}
	if req.WorkingDays.IsNegative() || req.WorkingDays.IsZero() {
		return nil, fmt.Errorf("working days must be positive")
	}

	// Verify absence type exists
	_, err := s.repo.GetAbsenceType(ctx, schemaName, tenantID, req.AbsenceTypeID)
	if err != nil {
		return nil, fmt.Errorf("get absence type: %w", err)
	}

	// Check leave balance (if balance tracking is enabled)
	year := req.StartDate.Year()
	balance, err := s.repo.GetLeaveBalance(ctx, schemaName, tenantID, req.EmployeeID, req.AbsenceTypeID, year)
	if err == nil && balance != nil {
		// Balance exists - check if there's enough remaining
		if balance.RemainingDays.LessThan(req.WorkingDays) {
			return nil, ErrInsufficientLeaveBalance
		}
		// Update pending days
		balance.PendingDays = balance.PendingDays.Add(req.WorkingDays)
		balance.RemainingDays = balance.EntitledDays.Add(balance.CarryoverDays).Sub(balance.UsedDays).Sub(balance.PendingDays)
		balance.UpdatedAt = time.Now()
		if err := s.repo.UpdateLeaveBalance(ctx, schemaName, balance); err != nil {
			return nil, fmt.Errorf("update leave balance: %w", err)
		}
	}

	now := time.Now()
	record := &LeaveRecord{
		ID:             s.uuid.New(),
		TenantID:       tenantID,
		EmployeeID:     req.EmployeeID,
		AbsenceTypeID:  req.AbsenceTypeID,
		StartDate:      req.StartDate,
		EndDate:        req.EndDate,
		TotalDays:      req.TotalDays,
		WorkingDays:    req.WorkingDays,
		Status:         LeavePending,
		DocumentNumber: req.DocumentNumber,
		DocumentDate:   req.DocumentDate,
		Notes:          req.Notes,
		RequestedAt:    now,
		RequestedBy:    requestedBy,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.repo.CreateLeaveRecord(ctx, schemaName, record); err != nil {
		return nil, fmt.Errorf("create leave record: %w", err)
	}

	return record, nil
}

// GetLeaveRecord returns a specific leave record
func (s *AbsenceService) GetLeaveRecord(ctx context.Context, schemaName, tenantID, recordID string) (*LeaveRecord, error) {
	record, err := s.repo.GetLeaveRecord(ctx, schemaName, tenantID, recordID)
	if err != nil {
		return nil, fmt.Errorf("get leave record: %w", err)
	}
	return record, nil
}

// ListLeaveRecords returns leave records for a tenant/employee
func (s *AbsenceService) ListLeaveRecords(ctx context.Context, schemaName, tenantID, employeeID string, year int) ([]LeaveRecord, error) {
	records, err := s.repo.ListLeaveRecords(ctx, schemaName, tenantID, employeeID, year)
	if err != nil {
		return nil, fmt.Errorf("list leave records: %w", err)
	}
	return records, nil
}

// ApproveLeaveRecord approves a pending leave request
func (s *AbsenceService) ApproveLeaveRecord(ctx context.Context, schemaName, tenantID, recordID, approvedBy string) (*LeaveRecord, error) {
	record, err := s.repo.GetLeaveRecord(ctx, schemaName, tenantID, recordID)
	if err != nil {
		return nil, fmt.Errorf("get leave record: %w", err)
	}

	if record.Status != LeavePending {
		return nil, ErrLeaveRecordNotPending
	}

	now := time.Now()
	record.Status = LeaveApproved
	record.ApprovedAt = &now
	record.ApprovedBy = approvedBy
	record.UpdatedAt = now

	if err := s.repo.UpdateLeaveRecord(ctx, schemaName, record); err != nil {
		return nil, fmt.Errorf("update leave record: %w", err)
	}

	// Update balance: move from pending to used
	year := record.StartDate.Year()
	balance, err := s.repo.GetLeaveBalance(ctx, schemaName, tenantID, record.EmployeeID, record.AbsenceTypeID, year)
	if err == nil && balance != nil {
		balance.PendingDays = balance.PendingDays.Sub(record.WorkingDays)
		if balance.PendingDays.IsNegative() {
			balance.PendingDays = decimal.Zero
		}
		balance.UsedDays = balance.UsedDays.Add(record.WorkingDays)
		balance.RemainingDays = balance.EntitledDays.Add(balance.CarryoverDays).Sub(balance.UsedDays).Sub(balance.PendingDays)
		balance.UpdatedAt = now
		if err := s.repo.UpdateLeaveBalance(ctx, schemaName, balance); err != nil {
			return nil, fmt.Errorf("update leave balance: %w", err)
		}
	}

	return record, nil
}

// RejectLeaveRecord rejects a pending leave request
func (s *AbsenceService) RejectLeaveRecord(ctx context.Context, schemaName, tenantID, recordID, rejectedBy, reason string) (*LeaveRecord, error) {
	record, err := s.repo.GetLeaveRecord(ctx, schemaName, tenantID, recordID)
	if err != nil {
		return nil, fmt.Errorf("get leave record: %w", err)
	}

	if record.Status != LeavePending {
		return nil, ErrLeaveRecordNotPending
	}

	now := time.Now()
	record.Status = LeaveRejected
	record.RejectedAt = &now
	record.RejectedBy = rejectedBy
	record.RejectionReason = reason
	record.UpdatedAt = now

	if err := s.repo.UpdateLeaveRecord(ctx, schemaName, record); err != nil {
		return nil, fmt.Errorf("update leave record: %w", err)
	}

	// Update balance: remove from pending
	year := record.StartDate.Year()
	balance, err := s.repo.GetLeaveBalance(ctx, schemaName, tenantID, record.EmployeeID, record.AbsenceTypeID, year)
	if err == nil && balance != nil {
		balance.PendingDays = balance.PendingDays.Sub(record.WorkingDays)
		if balance.PendingDays.IsNegative() {
			balance.PendingDays = decimal.Zero
		}
		balance.RemainingDays = balance.EntitledDays.Add(balance.CarryoverDays).Sub(balance.UsedDays).Sub(balance.PendingDays)
		balance.UpdatedAt = now
		if err := s.repo.UpdateLeaveBalance(ctx, schemaName, balance); err != nil {
			return nil, fmt.Errorf("update leave balance: %w", err)
		}
	}

	return record, nil
}

// CancelLeaveRecord cancels a leave request (by the employee)
func (s *AbsenceService) CancelLeaveRecord(ctx context.Context, schemaName, tenantID, recordID, cancelledBy string) (*LeaveRecord, error) {
	record, err := s.repo.GetLeaveRecord(ctx, schemaName, tenantID, recordID)
	if err != nil {
		return nil, fmt.Errorf("get leave record: %w", err)
	}

	// Can only cancel pending or approved leaves
	if record.Status != LeavePending && record.Status != LeaveApproved {
		return nil, fmt.Errorf("can only cancel pending or approved leave requests")
	}

	wasPending := record.Status == LeavePending

	now := time.Now()
	record.Status = LeaveCanceled
	record.UpdatedAt = now

	if err := s.repo.UpdateLeaveRecord(ctx, schemaName, record); err != nil {
		return nil, fmt.Errorf("update leave record: %w", err)
	}

	// Update balance
	year := record.StartDate.Year()
	balance, err := s.repo.GetLeaveBalance(ctx, schemaName, tenantID, record.EmployeeID, record.AbsenceTypeID, year)
	if err == nil && balance != nil {
		if wasPending {
			balance.PendingDays = balance.PendingDays.Sub(record.WorkingDays)
		} else {
			// Was approved, so reduce used days
			balance.UsedDays = balance.UsedDays.Sub(record.WorkingDays)
		}
		if balance.PendingDays.IsNegative() {
			balance.PendingDays = decimal.Zero
		}
		if balance.UsedDays.IsNegative() {
			balance.UsedDays = decimal.Zero
		}
		balance.RemainingDays = balance.EntitledDays.Add(balance.CarryoverDays).Sub(balance.UsedDays).Sub(balance.PendingDays)
		balance.UpdatedAt = now
		if err := s.repo.UpdateLeaveBalance(ctx, schemaName, balance); err != nil {
			return nil, fmt.Errorf("update leave balance: %w", err)
		}
	}

	return record, nil
}

// InitializeEmployeeLeaveBalances creates leave balances for an employee for the current year
func (s *AbsenceService) InitializeEmployeeLeaveBalances(ctx context.Context, schemaName, tenantID, employeeID string, year int) ([]LeaveBalance, error) {
	// Get all active absence types
	absenceTypes, err := s.repo.ListAbsenceTypes(ctx, schemaName, tenantID, true)
	if err != nil {
		return nil, fmt.Errorf("list absence types: %w", err)
	}

	var balances []LeaveBalance
	now := time.Now()

	for _, at := range absenceTypes {
		// Check if balance already exists
		existing, err := s.repo.GetLeaveBalance(ctx, schemaName, tenantID, employeeID, at.ID, year)
		if err == nil && existing != nil {
			balances = append(balances, *existing)
			continue
		}

		// Create new balance
		balance := &LeaveBalance{
			ID:            s.uuid.New(),
			TenantID:      tenantID,
			EmployeeID:    employeeID,
			AbsenceTypeID: at.ID,
			Year:          year,
			EntitledDays:  at.DefaultDaysPerYear,
			CarryoverDays: decimal.Zero,
			UsedDays:      decimal.Zero,
			PendingDays:   decimal.Zero,
			RemainingDays: at.DefaultDaysPerYear,
			CreatedAt:     now,
			UpdatedAt:     now,
		}

		if err := s.repo.CreateLeaveBalance(ctx, schemaName, balance); err != nil {
			return nil, fmt.Errorf("create leave balance: %w", err)
		}

		balance.AbsenceType = &at
		balances = append(balances, *balance)
	}

	return balances, nil
}
