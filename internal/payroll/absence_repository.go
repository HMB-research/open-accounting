package payroll

import (
	"context"
	"fmt"
)

// AbsenceRepository defines the contract for absence/leave data access
type AbsenceRepository interface {
	// Absence type operations
	ListAbsenceTypes(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]AbsenceType, error)
	GetAbsenceType(ctx context.Context, schemaName, tenantID, typeID string) (*AbsenceType, error)
	GetAbsenceTypeByCode(ctx context.Context, schemaName, tenantID, code string) (*AbsenceType, error)

	// Leave balance operations
	GetLeaveBalance(ctx context.Context, schemaName, tenantID, employeeID, absenceTypeID string, year int) (*LeaveBalance, error)
	ListLeaveBalances(ctx context.Context, schemaName, tenantID, employeeID string, year int) ([]LeaveBalance, error)
	CreateLeaveBalance(ctx context.Context, schemaName string, balance *LeaveBalance) error
	UpdateLeaveBalance(ctx context.Context, schemaName string, balance *LeaveBalance) error

	// Leave record operations
	CreateLeaveRecord(ctx context.Context, schemaName string, record *LeaveRecord) error
	GetLeaveRecord(ctx context.Context, schemaName, tenantID, recordID string) (*LeaveRecord, error)
	ListLeaveRecords(ctx context.Context, schemaName, tenantID, employeeID string, year int) ([]LeaveRecord, error)
	UpdateLeaveRecord(ctx context.Context, schemaName string, record *LeaveRecord) error
}

// Error definitions for absence management
var (
	ErrAbsenceTypeNotFound      = fmt.Errorf("absence type not found")
	ErrLeaveBalanceNotFound     = fmt.Errorf("leave balance not found")
	ErrLeaveRecordNotFound      = fmt.Errorf("leave record not found")
	ErrInsufficientLeaveBalance = fmt.Errorf("insufficient leave balance")
	ErrLeaveRecordNotPending    = fmt.Errorf("leave record is not in pending status")
)

// AbsencePostgresRepository implements AbsenceRepository using PostgreSQL
type AbsencePostgresRepository struct {
	repo *PostgresRepository
}

// NewAbsencePostgresRepository creates a new PostgreSQL absence repository
func NewAbsencePostgresRepository(repo *PostgresRepository) *AbsencePostgresRepository {
	return &AbsencePostgresRepository{repo: repo}
}

// ListAbsenceTypes returns all absence types for a tenant
func (r *AbsencePostgresRepository) ListAbsenceTypes(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]AbsenceType, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, code, name, name_et, COALESCE(description, ''),
			is_paid, affects_salary, requires_document, COALESCE(document_type, ''),
			default_days_per_year, max_carryover_days, COALESCE(tsd_code, ''), COALESCE(emta_code, ''),
			is_system, is_active, sort_order, created_at, updated_at
		FROM %s.absence_types
		WHERE tenant_id = $1
	`, schemaName)

	if activeOnly {
		query += " AND is_active = true"
	}
	query += " ORDER BY sort_order, name"

	rows, err := r.repo.query(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var types []AbsenceType
	for rows.Next() {
		var t AbsenceType
		if err := rows.Scan(
			&t.ID, &t.TenantID, &t.Code, &t.Name, &t.NameET, &t.Description,
			&t.IsPaid, &t.AffectsSalary, &t.RequiresDocument, &t.DocumentType,
			&t.DefaultDaysPerYear, &t.MaxCarryoverDays, &t.TSDCode, &t.EMTACode,
			&t.IsSystem, &t.IsActive, &t.SortOrder, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, err
		}
		types = append(types, t)
	}

	return types, nil
}

// GetAbsenceType retrieves an absence type by ID
func (r *AbsencePostgresRepository) GetAbsenceType(ctx context.Context, schemaName, tenantID, typeID string) (*AbsenceType, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, code, name, name_et, COALESCE(description, ''),
			is_paid, affects_salary, requires_document, COALESCE(document_type, ''),
			default_days_per_year, max_carryover_days, COALESCE(tsd_code, ''), COALESCE(emta_code, ''),
			is_system, is_active, sort_order, created_at, updated_at
		FROM %s.absence_types
		WHERE tenant_id = $1 AND id = $2
	`, schemaName)

	var t AbsenceType
	err := r.repo.queryRow(ctx, query, tenantID, typeID).Scan(
		&t.ID, &t.TenantID, &t.Code, &t.Name, &t.NameET, &t.Description,
		&t.IsPaid, &t.AffectsSalary, &t.RequiresDocument, &t.DocumentType,
		&t.DefaultDaysPerYear, &t.MaxCarryoverDays, &t.TSDCode, &t.EMTACode,
		&t.IsSystem, &t.IsActive, &t.SortOrder, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, ErrAbsenceTypeNotFound
	}

	return &t, nil
}

// GetAbsenceTypeByCode retrieves an absence type by code
func (r *AbsencePostgresRepository) GetAbsenceTypeByCode(ctx context.Context, schemaName, tenantID, code string) (*AbsenceType, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, code, name, name_et, COALESCE(description, ''),
			is_paid, affects_salary, requires_document, COALESCE(document_type, ''),
			default_days_per_year, max_carryover_days, COALESCE(tsd_code, ''), COALESCE(emta_code, ''),
			is_system, is_active, sort_order, created_at, updated_at
		FROM %s.absence_types
		WHERE tenant_id = $1 AND code = $2
	`, schemaName)

	var t AbsenceType
	err := r.repo.queryRow(ctx, query, tenantID, code).Scan(
		&t.ID, &t.TenantID, &t.Code, &t.Name, &t.NameET, &t.Description,
		&t.IsPaid, &t.AffectsSalary, &t.RequiresDocument, &t.DocumentType,
		&t.DefaultDaysPerYear, &t.MaxCarryoverDays, &t.TSDCode, &t.EMTACode,
		&t.IsSystem, &t.IsActive, &t.SortOrder, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, ErrAbsenceTypeNotFound
	}

	return &t, nil
}

// GetLeaveBalance retrieves a specific leave balance
func (r *AbsencePostgresRepository) GetLeaveBalance(ctx context.Context, schemaName, tenantID, employeeID, absenceTypeID string, year int) (*LeaveBalance, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, employee_id, absence_type_id, year,
			entitled_days, carryover_days, used_days, pending_days, remaining_days,
			COALESCE(notes, ''), created_at, updated_at
		FROM %s.leave_balances
		WHERE tenant_id = $1 AND employee_id = $2 AND absence_type_id = $3 AND year = $4
	`, schemaName)

	var b LeaveBalance
	err := r.repo.queryRow(ctx, query, tenantID, employeeID, absenceTypeID, year).Scan(
		&b.ID, &b.TenantID, &b.EmployeeID, &b.AbsenceTypeID, &b.Year,
		&b.EntitledDays, &b.CarryoverDays, &b.UsedDays, &b.PendingDays, &b.RemainingDays,
		&b.Notes, &b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		return nil, ErrLeaveBalanceNotFound
	}

	return &b, nil
}

// ListLeaveBalances returns leave balances for an employee
func (r *AbsencePostgresRepository) ListLeaveBalances(ctx context.Context, schemaName, tenantID, employeeID string, year int) ([]LeaveBalance, error) {
	query := fmt.Sprintf(`
		SELECT lb.id, lb.tenant_id, lb.employee_id, lb.absence_type_id, lb.year,
			lb.entitled_days, lb.carryover_days, lb.used_days, lb.pending_days, lb.remaining_days,
			COALESCE(lb.notes, ''), lb.created_at, lb.updated_at,
			at.code, at.name, at.name_et
		FROM %s.leave_balances lb
		JOIN %s.absence_types at ON lb.absence_type_id = at.id
		WHERE lb.tenant_id = $1 AND lb.employee_id = $2 AND lb.year = $3
		ORDER BY at.sort_order, at.name
	`, schemaName, schemaName)

	rows, err := r.repo.query(ctx, query, tenantID, employeeID, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var balances []LeaveBalance
	for rows.Next() {
		var b LeaveBalance
		var atCode, atName, atNameET string
		if err := rows.Scan(
			&b.ID, &b.TenantID, &b.EmployeeID, &b.AbsenceTypeID, &b.Year,
			&b.EntitledDays, &b.CarryoverDays, &b.UsedDays, &b.PendingDays, &b.RemainingDays,
			&b.Notes, &b.CreatedAt, &b.UpdatedAt,
			&atCode, &atName, &atNameET,
		); err != nil {
			return nil, err
		}
		b.AbsenceType = &AbsenceType{
			ID:     b.AbsenceTypeID,
			Code:   atCode,
			Name:   atName,
			NameET: atNameET,
		}
		balances = append(balances, b)
	}

	return balances, nil
}

// CreateLeaveBalance inserts a new leave balance
func (r *AbsencePostgresRepository) CreateLeaveBalance(ctx context.Context, schemaName string, balance *LeaveBalance) error {
	query := fmt.Sprintf(`
		INSERT INTO %s.leave_balances (
			id, tenant_id, employee_id, absence_type_id, year,
			entitled_days, carryover_days, used_days, pending_days, notes, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, schemaName)

	return r.repo.exec(ctx, query,
		balance.ID, balance.TenantID, balance.EmployeeID, balance.AbsenceTypeID, balance.Year,
		balance.EntitledDays, balance.CarryoverDays, balance.UsedDays, balance.PendingDays,
		balance.Notes, balance.CreatedAt, balance.UpdatedAt,
	)
}

// UpdateLeaveBalance updates a leave balance
func (r *AbsencePostgresRepository) UpdateLeaveBalance(ctx context.Context, schemaName string, balance *LeaveBalance) error {
	query := fmt.Sprintf(`
		UPDATE %s.leave_balances
		SET entitled_days = $1, carryover_days = $2, used_days = $3, pending_days = $4,
			notes = $5, updated_at = $6
		WHERE id = $7
	`, schemaName)

	return r.repo.exec(ctx, query,
		balance.EntitledDays, balance.CarryoverDays, balance.UsedDays, balance.PendingDays,
		balance.Notes, balance.UpdatedAt, balance.ID,
	)
}

// CreateLeaveRecord inserts a new leave record
func (r *AbsencePostgresRepository) CreateLeaveRecord(ctx context.Context, schemaName string, record *LeaveRecord) error {
	query := fmt.Sprintf(`
		INSERT INTO %s.leave_records (
			id, tenant_id, employee_id, absence_type_id, start_date, end_date,
			total_days, working_days, status, document_number, document_date, document_url,
			requested_at, requested_by, notes, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
	`, schemaName)

	return r.repo.exec(ctx, query,
		record.ID, record.TenantID, record.EmployeeID, record.AbsenceTypeID,
		record.StartDate, record.EndDate, record.TotalDays, record.WorkingDays,
		record.Status, record.DocumentNumber, record.DocumentDate, record.DocumentURL,
		record.RequestedAt, record.RequestedBy, record.Notes, record.CreatedAt, record.UpdatedAt,
	)
}

// GetLeaveRecord retrieves a leave record by ID
func (r *AbsencePostgresRepository) GetLeaveRecord(ctx context.Context, schemaName, tenantID, recordID string) (*LeaveRecord, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, employee_id, absence_type_id, start_date, end_date,
			total_days, working_days, status, COALESCE(document_number, ''), document_date, COALESCE(document_url, ''),
			requested_at, COALESCE(requested_by, ''), approved_at, COALESCE(approved_by, ''),
			rejected_at, COALESCE(rejected_by, ''), COALESCE(rejection_reason, ''),
			COALESCE(payroll_run_id, ''), COALESCE(notes, ''), created_at, updated_at
		FROM %s.leave_records
		WHERE tenant_id = $1 AND id = $2
	`, schemaName)

	var r2 LeaveRecord
	err := r.repo.queryRow(ctx, query, tenantID, recordID).Scan(
		&r2.ID, &r2.TenantID, &r2.EmployeeID, &r2.AbsenceTypeID, &r2.StartDate, &r2.EndDate,
		&r2.TotalDays, &r2.WorkingDays, &r2.Status, &r2.DocumentNumber, &r2.DocumentDate, &r2.DocumentURL,
		&r2.RequestedAt, &r2.RequestedBy, &r2.ApprovedAt, &r2.ApprovedBy,
		&r2.RejectedAt, &r2.RejectedBy, &r2.RejectionReason,
		&r2.PayrollRunID, &r2.Notes, &r2.CreatedAt, &r2.UpdatedAt,
	)
	if err != nil {
		return nil, ErrLeaveRecordNotFound
	}

	return &r2, nil
}

// ListLeaveRecords returns leave records for an employee
func (r *AbsencePostgresRepository) ListLeaveRecords(ctx context.Context, schemaName, tenantID, employeeID string, year int) ([]LeaveRecord, error) {
	query := fmt.Sprintf(`
		SELECT lr.id, lr.tenant_id, lr.employee_id, lr.absence_type_id, lr.start_date, lr.end_date,
			lr.total_days, lr.working_days, lr.status, COALESCE(lr.document_number, ''), lr.document_date, COALESCE(lr.document_url, ''),
			lr.requested_at, COALESCE(lr.requested_by, ''), lr.approved_at, COALESCE(lr.approved_by, ''),
			lr.rejected_at, COALESCE(lr.rejected_by, ''), COALESCE(lr.rejection_reason, ''),
			COALESCE(lr.payroll_run_id, ''), COALESCE(lr.notes, ''), lr.created_at, lr.updated_at,
			at.code, at.name, at.name_et
		FROM %s.leave_records lr
		JOIN %s.absence_types at ON lr.absence_type_id = at.id
		WHERE lr.tenant_id = $1
	`, schemaName, schemaName)

	args := []interface{}{tenantID}
	argIdx := 2

	if employeeID != "" {
		query += fmt.Sprintf(" AND lr.employee_id = $%d", argIdx)
		args = append(args, employeeID)
		argIdx++
	}

	if year > 0 {
		query += fmt.Sprintf(" AND EXTRACT(YEAR FROM lr.start_date) = $%d", argIdx)
		args = append(args, year)
	}

	query += " ORDER BY lr.start_date DESC"

	rows, err := r.repo.query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []LeaveRecord
	for rows.Next() {
		var lr LeaveRecord
		var atCode, atName, atNameET string
		if err := rows.Scan(
			&lr.ID, &lr.TenantID, &lr.EmployeeID, &lr.AbsenceTypeID, &lr.StartDate, &lr.EndDate,
			&lr.TotalDays, &lr.WorkingDays, &lr.Status, &lr.DocumentNumber, &lr.DocumentDate, &lr.DocumentURL,
			&lr.RequestedAt, &lr.RequestedBy, &lr.ApprovedAt, &lr.ApprovedBy,
			&lr.RejectedAt, &lr.RejectedBy, &lr.RejectionReason,
			&lr.PayrollRunID, &lr.Notes, &lr.CreatedAt, &lr.UpdatedAt,
			&atCode, &atName, &atNameET,
		); err != nil {
			return nil, err
		}
		lr.AbsenceType = &AbsenceType{
			ID:     lr.AbsenceTypeID,
			Code:   atCode,
			Name:   atName,
			NameET: atNameET,
		}
		records = append(records, lr)
	}

	return records, nil
}

// UpdateLeaveRecord updates a leave record
func (r *AbsencePostgresRepository) UpdateLeaveRecord(ctx context.Context, schemaName string, record *LeaveRecord) error {
	query := fmt.Sprintf(`
		UPDATE %s.leave_records
		SET status = $1, approved_at = $2, approved_by = $3, rejected_at = $4, rejected_by = $5,
			rejection_reason = $6, updated_at = $7
		WHERE id = $8
	`, schemaName)

	return r.repo.exec(ctx, query,
		record.Status, record.ApprovedAt, record.ApprovedBy,
		record.RejectedAt, record.RejectedBy, record.RejectionReason,
		record.UpdatedAt, record.ID,
	)
}

// MockAbsenceRepository implements AbsenceRepository for testing
type MockAbsenceRepository struct {
	AbsenceTypes  map[string]*AbsenceType
	LeaveBalances map[string]*LeaveBalance
	LeaveRecords  map[string]*LeaveRecord

	ListAbsenceTypesErr     error
	GetAbsenceTypeErr       error
	GetAbsenceTypeByCodeErr error
	GetLeaveBalanceErr      error
	ListLeaveBalancesErr    error
	CreateLeaveBalanceErr   error
	UpdateLeaveBalanceErr   error
	CreateLeaveRecordErr    error
	GetLeaveRecordErr       error
	ListLeaveRecordsErr     error
	UpdateLeaveRecordErr    error
}

// NewMockAbsenceRepository creates a new mock absence repository
func NewMockAbsenceRepository() *MockAbsenceRepository {
	return &MockAbsenceRepository{
		AbsenceTypes:  make(map[string]*AbsenceType),
		LeaveBalances: make(map[string]*LeaveBalance),
		LeaveRecords:  make(map[string]*LeaveRecord),
	}
}

func (m *MockAbsenceRepository) ListAbsenceTypes(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]AbsenceType, error) {
	if m.ListAbsenceTypesErr != nil {
		return nil, m.ListAbsenceTypesErr
	}
	var types []AbsenceType
	for _, t := range m.AbsenceTypes {
		if t.TenantID == tenantID {
			if !activeOnly || t.IsActive {
				types = append(types, *t)
			}
		}
	}
	return types, nil
}

func (m *MockAbsenceRepository) GetAbsenceType(ctx context.Context, schemaName, tenantID, typeID string) (*AbsenceType, error) {
	if m.GetAbsenceTypeErr != nil {
		return nil, m.GetAbsenceTypeErr
	}
	t, ok := m.AbsenceTypes[typeID]
	if !ok {
		return nil, ErrAbsenceTypeNotFound
	}
	return t, nil
}

func (m *MockAbsenceRepository) GetAbsenceTypeByCode(ctx context.Context, schemaName, tenantID, code string) (*AbsenceType, error) {
	if m.GetAbsenceTypeByCodeErr != nil {
		return nil, m.GetAbsenceTypeByCodeErr
	}
	for _, t := range m.AbsenceTypes {
		if t.TenantID == tenantID && t.Code == code {
			return t, nil
		}
	}
	return nil, ErrAbsenceTypeNotFound
}

func (m *MockAbsenceRepository) GetLeaveBalance(ctx context.Context, schemaName, tenantID, employeeID, absenceTypeID string, year int) (*LeaveBalance, error) {
	if m.GetLeaveBalanceErr != nil {
		return nil, m.GetLeaveBalanceErr
	}
	key := fmt.Sprintf("%s-%s-%s-%d", tenantID, employeeID, absenceTypeID, year)
	b, ok := m.LeaveBalances[key]
	if !ok {
		return nil, ErrLeaveBalanceNotFound
	}
	return b, nil
}

func (m *MockAbsenceRepository) ListLeaveBalances(ctx context.Context, schemaName, tenantID, employeeID string, year int) ([]LeaveBalance, error) {
	if m.ListLeaveBalancesErr != nil {
		return nil, m.ListLeaveBalancesErr
	}
	var balances []LeaveBalance
	for _, b := range m.LeaveBalances {
		if b.TenantID == tenantID && b.EmployeeID == employeeID && b.Year == year {
			balances = append(balances, *b)
		}
	}
	return balances, nil
}

func (m *MockAbsenceRepository) CreateLeaveBalance(ctx context.Context, schemaName string, balance *LeaveBalance) error {
	if m.CreateLeaveBalanceErr != nil {
		return m.CreateLeaveBalanceErr
	}
	key := fmt.Sprintf("%s-%s-%s-%d", balance.TenantID, balance.EmployeeID, balance.AbsenceTypeID, balance.Year)
	m.LeaveBalances[key] = balance
	return nil
}

func (m *MockAbsenceRepository) UpdateLeaveBalance(ctx context.Context, schemaName string, balance *LeaveBalance) error {
	if m.UpdateLeaveBalanceErr != nil {
		return m.UpdateLeaveBalanceErr
	}
	key := fmt.Sprintf("%s-%s-%s-%d", balance.TenantID, balance.EmployeeID, balance.AbsenceTypeID, balance.Year)
	m.LeaveBalances[key] = balance
	return nil
}

func (m *MockAbsenceRepository) CreateLeaveRecord(ctx context.Context, schemaName string, record *LeaveRecord) error {
	if m.CreateLeaveRecordErr != nil {
		return m.CreateLeaveRecordErr
	}
	m.LeaveRecords[record.ID] = record
	return nil
}

func (m *MockAbsenceRepository) GetLeaveRecord(ctx context.Context, schemaName, tenantID, recordID string) (*LeaveRecord, error) {
	if m.GetLeaveRecordErr != nil {
		return nil, m.GetLeaveRecordErr
	}
	r, ok := m.LeaveRecords[recordID]
	if !ok {
		return nil, ErrLeaveRecordNotFound
	}
	return r, nil
}

func (m *MockAbsenceRepository) ListLeaveRecords(ctx context.Context, schemaName, tenantID, employeeID string, year int) ([]LeaveRecord, error) {
	if m.ListLeaveRecordsErr != nil {
		return nil, m.ListLeaveRecordsErr
	}
	var records []LeaveRecord
	for _, r := range m.LeaveRecords {
		if r.TenantID == tenantID {
			if employeeID == "" || r.EmployeeID == employeeID {
				if year == 0 || r.StartDate.Year() == year {
					records = append(records, *r)
				}
			}
		}
	}
	return records, nil
}

func (m *MockAbsenceRepository) UpdateLeaveRecord(ctx context.Context, schemaName string, record *LeaveRecord) error {
	if m.UpdateLeaveRecordErr != nil {
		return m.UpdateLeaveRecordErr
	}
	m.LeaveRecords[record.ID] = record
	return nil
}
