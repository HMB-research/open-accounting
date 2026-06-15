package payroll

import (
	"context"
	"fmt"
)

// AbsenceRepository defines the contract for absence/leave data access
type AbsenceRepository interface {
	// Employee lookup operations
	ListEmployees(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]Employee, error)

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
	ErrAbsenceTypeNotFound           = fmt.Errorf("absence type not found")
	ErrLeaveBalanceNotFound          = fmt.Errorf("leave balance not found")
	ErrLeaveRecordNotFound           = fmt.Errorf("leave record not found")
	ErrInsufficientLeaveBalance      = fmt.Errorf("insufficient leave balance")
	ErrLeaveRecordNotPending         = fmt.Errorf("leave record is not in pending status")
	ErrApprovedLeaveDocumentRequired = fmt.Errorf("approved leave document is required")
)

// MockAbsenceRepository implements AbsenceRepository for testing
type MockAbsenceRepository struct {
	Employees     map[string]*Employee
	AbsenceTypes  map[string]*AbsenceType
	LeaveBalances map[string]*LeaveBalance
	LeaveRecords  map[string]*LeaveRecord

	ListEmployeesErr        error
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
		Employees:     make(map[string]*Employee),
		AbsenceTypes:  make(map[string]*AbsenceType),
		LeaveBalances: make(map[string]*LeaveBalance),
		LeaveRecords:  make(map[string]*LeaveRecord),
	}
}

func (m *MockAbsenceRepository) ListEmployees(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]Employee, error) {
	if m.ListEmployeesErr != nil {
		return nil, m.ListEmployeesErr
	}
	employees := []Employee{}
	for _, emp := range m.Employees {
		if emp.TenantID == tenantID {
			if !activeOnly || emp.IsActive {
				employees = append(employees, *emp)
			}
		}
	}
	return employees, nil
}

func (m *MockAbsenceRepository) ListAbsenceTypes(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]AbsenceType, error) {
	if m.ListAbsenceTypesErr != nil {
		return nil, m.ListAbsenceTypesErr
	}
	types := []AbsenceType{}
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
	balances := []LeaveBalance{}
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
	records := []LeaveRecord{}
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
