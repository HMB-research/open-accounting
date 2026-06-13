package payroll

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockUUIDGenerator returns predictable UUIDs for testing
type MockUUIDGenerator struct {
	counter int
	prefix  string
}

func (m *MockUUIDGenerator) New() string {
	m.counter++
	return m.prefix + "-" + string(rune('0'+m.counter))
}

// MockRepository implements Repository for testing
type MockRepository struct {
	// Employee data
	Employees         map[string]*Employee
	CreateEmployeeErr error
	GetEmployeeErr    error
	ListEmployeesErr  error
	UpdateEmployeeErr error

	// Salary component data
	Salaries                 map[string]decimal.Decimal // employeeID -> salary
	SalaryComponents         map[string][]SalaryComponent
	EndCurrentBaseSalaryErr  error
	CreateSalaryComponentErr error
	ListSalaryComponentsErr  error
	GetCurrentSalaryErr      error

	// Payroll run data
	PayrollRuns          map[string]*PayrollRun
	CreatePayrollRunErr  error
	GetPayrollRunErr     error
	ListPayrollRunsErr   error
	UpdatePayrollRunErr  error
	ApprovePayrollRunErr error

	// Payslip data
	Payslips          []Payslip
	DeletePayslipsErr error
	CreatePayslipErr  error

	// TSD data
	TSDDeclarations     map[string]*TSDDeclaration
	TSDRows             map[string][]TSDRow
	GetPayslipsErr      error
	DeleteTSDErr        error
	CreateTSDErr        error
	CreateTSDRowsErr    error
	GetTSDErr           error
	GetTSDRowsErr       error
	ListTSDErr          error
	MarkTSDSubmittedErr error
	UpdateTSDStatusErr  error

	// Transaction handling
	BeginTxErr         error
	WithTransactionErr error
	mockTx             *MockTx
}

type MockTx struct {
	CommitCalled   bool
	RollbackCalled bool
	CommitErr      error
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		Employees:        make(map[string]*Employee),
		PayrollRuns:      make(map[string]*PayrollRun),
		Salaries:         make(map[string]decimal.Decimal),
		SalaryComponents: make(map[string][]SalaryComponent),
		TSDDeclarations:  make(map[string]*TSDDeclaration),
		TSDRows:          make(map[string][]TSDRow),
		mockTx:           &MockTx{},
	}
}

func (m *MockRepository) CreateEmployee(ctx context.Context, schemaName string, emp *Employee) error {
	if m.CreateEmployeeErr != nil {
		return m.CreateEmployeeErr
	}
	m.Employees[emp.ID] = emp
	return nil
}

func (m *MockRepository) GetEmployee(ctx context.Context, schemaName, tenantID, employeeID string) (*Employee, error) {
	if m.GetEmployeeErr != nil {
		return nil, m.GetEmployeeErr
	}
	emp, ok := m.Employees[employeeID]
	if !ok {
		return nil, ErrEmployeeNotFound
	}
	return emp, nil
}

func (m *MockRepository) ListEmployees(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]Employee, error) {
	if m.ListEmployeesErr != nil {
		return nil, m.ListEmployeesErr
	}
	var employees []Employee
	for _, emp := range m.Employees {
		if emp.TenantID == tenantID {
			if !activeOnly || emp.IsActive {
				employees = append(employees, *emp)
			}
		}
	}
	return employees, nil
}

func (m *MockRepository) UpdateEmployee(ctx context.Context, schemaName string, emp *Employee) error {
	if m.UpdateEmployeeErr != nil {
		return m.UpdateEmployeeErr
	}
	m.Employees[emp.ID] = emp
	return nil
}

func (m *MockRepository) EndCurrentBaseSalary(ctx context.Context, schemaName, tenantID, employeeID string, effectiveTo time.Time) error {
	if m.EndCurrentBaseSalaryErr == nil {
		for i := range m.SalaryComponents[employeeID] {
			comp := &m.SalaryComponents[employeeID][i]
			if comp.TenantID == tenantID && comp.ComponentType == SalaryComponentBaseSalary && comp.EffectiveTo == nil {
				comp.EffectiveTo = &effectiveTo
			}
		}
		m.recalculateSalary(employeeID, time.Now())
	}
	return m.EndCurrentBaseSalaryErr
}

func (m *MockRepository) CreateSalaryComponent(ctx context.Context, schemaName string, comp *SalaryComponent) error {
	if m.CreateSalaryComponentErr != nil {
		return m.CreateSalaryComponentErr
	}
	m.SalaryComponents[comp.EmployeeID] = append(m.SalaryComponents[comp.EmployeeID], *comp)
	m.recalculateSalary(comp.EmployeeID, time.Now())
	return nil
}

func (m *MockRepository) ListSalaryComponents(ctx context.Context, schemaName, tenantID, employeeID string, activeOn *time.Time) ([]SalaryComponent, error) {
	if m.ListSalaryComponentsErr != nil {
		return nil, m.ListSalaryComponentsErr
	}
	components := []SalaryComponent{}
	for _, comp := range m.SalaryComponents[employeeID] {
		if comp.TenantID != tenantID {
			continue
		}
		if activeOn != nil && !salaryComponentActiveOn(comp, *activeOn) {
			continue
		}
		components = append(components, comp)
	}
	return components, nil
}

func (m *MockRepository) GetCurrentSalary(ctx context.Context, schemaName, tenantID, employeeID string) (decimal.Decimal, error) {
	if m.GetCurrentSalaryErr != nil {
		return decimal.Zero, m.GetCurrentSalaryErr
	}
	salary, ok := m.Salaries[employeeID]
	if !ok {
		return decimal.Zero, nil
	}
	return salary, nil
}

func (m *MockRepository) recalculateSalary(employeeID string, activeOn time.Time) {
	if len(m.SalaryComponents[employeeID]) == 0 {
		return
	}
	total := decimal.Zero
	for _, comp := range m.SalaryComponents[employeeID] {
		if comp.IsRecurring && salaryComponentActiveOn(comp, activeOn) {
			total = total.Add(comp.Amount)
		}
	}
	m.Salaries[employeeID] = total
}

func salaryComponentActiveOn(comp SalaryComponent, activeOn time.Time) bool {
	if comp.EffectiveFrom.After(activeOn) {
		return false
	}
	return comp.EffectiveTo == nil || !comp.EffectiveTo.Before(activeOn)
}

func (m *MockRepository) CreatePayrollRun(ctx context.Context, schemaName string, run *PayrollRun) error {
	if m.CreatePayrollRunErr != nil {
		return m.CreatePayrollRunErr
	}
	m.PayrollRuns[run.ID] = run
	return nil
}

func (m *MockRepository) GetPayrollRun(ctx context.Context, schemaName, tenantID, runID string) (*PayrollRun, error) {
	if m.GetPayrollRunErr != nil {
		return nil, m.GetPayrollRunErr
	}
	run, ok := m.PayrollRuns[runID]
	if !ok {
		return nil, ErrPayrollRunNotFound
	}
	return run, nil
}

func (m *MockRepository) ListPayrollRuns(ctx context.Context, schemaName, tenantID string, year int) ([]PayrollRun, error) {
	if m.ListPayrollRunsErr != nil {
		return nil, m.ListPayrollRunsErr
	}
	var runs []PayrollRun
	for _, run := range m.PayrollRuns {
		if run.TenantID == tenantID {
			if year == 0 || run.PeriodYear == year {
				runs = append(runs, *run)
			}
		}
	}
	return runs, nil
}

func (m *MockRepository) UpdatePayrollRun(ctx context.Context, schemaName string, run *PayrollRun) error {
	if m.UpdatePayrollRunErr != nil {
		return m.UpdatePayrollRunErr
	}
	m.PayrollRuns[run.ID] = run
	return nil
}

func (m *MockRepository) ApprovePayrollRun(ctx context.Context, schemaName, tenantID, runID, approverID string) error {
	if m.ApprovePayrollRunErr != nil {
		return m.ApprovePayrollRunErr
	}
	run, ok := m.PayrollRuns[runID]
	if !ok || run.Status != PayrollCalculated {
		return ErrPayrollRunNotFound
	}
	run.Status = PayrollApproved
	run.ApprovedBy = approverID
	now := time.Now()
	run.ApprovedAt = &now
	return nil
}

func (m *MockRepository) DeletePayslipsByRunID(ctx context.Context, schemaName, runID string) error {
	if m.DeletePayslipsErr != nil {
		return m.DeletePayslipsErr
	}
	m.Payslips = nil
	return nil
}

func (m *MockRepository) CreatePayslip(ctx context.Context, schemaName string, payslip *Payslip) error {
	if m.CreatePayslipErr != nil {
		return m.CreatePayslipErr
	}
	m.Payslips = append(m.Payslips, *payslip)
	return nil
}

func (m *MockRepository) GetPayslipsWithEmployees(ctx context.Context, schemaName, tenantID, payrollRunID string) ([]Payslip, error) {
	if m.GetPayslipsErr != nil {
		return nil, m.GetPayslipsErr
	}
	payslips := []Payslip{}
	for _, payslip := range m.Payslips {
		if payslip.TenantID == tenantID && payslip.PayrollRunID == payrollRunID {
			if payslip.Employee == nil {
				if employee, ok := m.Employees[payslip.EmployeeID]; ok {
					payslip.Employee = employee
				}
			}
			payslips = append(payslips, payslip)
		}
	}
	return payslips, nil
}

func (m *MockRepository) DeleteTSDByPeriod(ctx context.Context, schemaName, tenantID string, year, month int) error {
	if m.DeleteTSDErr != nil {
		return m.DeleteTSDErr
	}
	for id, declaration := range m.TSDDeclarations {
		if declaration.TenantID == tenantID && declaration.PeriodYear == year && declaration.PeriodMonth == month {
			delete(m.TSDDeclarations, id)
			delete(m.TSDRows, id)
		}
	}
	return nil
}

func (m *MockRepository) CreateTSDDeclaration(ctx context.Context, schemaName string, declaration *TSDDeclaration) error {
	if m.CreateTSDErr != nil {
		return m.CreateTSDErr
	}
	copy := *declaration
	m.TSDDeclarations[declaration.ID] = &copy
	return nil
}

func (m *MockRepository) CreateTSDRows(ctx context.Context, schemaName string, rows []TSDRow) error {
	if m.CreateTSDRowsErr != nil {
		return m.CreateTSDRowsErr
	}
	for _, row := range rows {
		m.TSDRows[row.DeclarationID] = append(m.TSDRows[row.DeclarationID], row)
	}
	return nil
}

func (m *MockRepository) GetTSD(ctx context.Context, schemaName, tenantID string, year, month int) (*TSDDeclaration, error) {
	if m.GetTSDErr != nil {
		return nil, m.GetTSDErr
	}
	for _, declaration := range m.TSDDeclarations {
		if declaration.TenantID == tenantID && declaration.PeriodYear == year && declaration.PeriodMonth == month {
			copy := *declaration
			copy.Rows = append([]TSDRow(nil), m.TSDRows[declaration.ID]...)
			return &copy, nil
		}
	}
	return nil, ErrTSDDeclarationNotFound
}

func (m *MockRepository) GetTSDRows(ctx context.Context, schemaName, tenantID, declarationID string) ([]TSDRow, error) {
	if m.GetTSDRowsErr != nil {
		return nil, m.GetTSDRowsErr
	}
	return append([]TSDRow(nil), m.TSDRows[declarationID]...), nil
}

func (m *MockRepository) ListTSD(ctx context.Context, schemaName, tenantID string, filter TSDListFilter) ([]TSDDeclaration, error) {
	if m.ListTSDErr != nil {
		return nil, m.ListTSDErr
	}
	declarations := []TSDDeclaration{}
	for _, declaration := range m.TSDDeclarations {
		if declaration.TenantID == tenantID &&
			(filter.Year == 0 || declaration.PeriodYear == filter.Year) &&
			(filter.Month == 0 || declaration.PeriodMonth == filter.Month) {
			copy := *declaration
			declarations = append(declarations, copy)
		}
	}
	return declarations, nil
}

func (m *MockRepository) MarkTSDSubmitted(ctx context.Context, schemaName, tenantID, declarationID, emtaReference string, submittedAt time.Time) error {
	if m.MarkTSDSubmittedErr != nil {
		return m.MarkTSDSubmittedErr
	}
	declaration, ok := m.TSDDeclarations[declarationID]
	if !ok || declaration.TenantID != tenantID {
		return ErrTSDDeclarationNotFound
	}
	declaration.Status = TSDSubmitted
	declaration.SubmittedAt = &submittedAt
	declaration.EMTAReference = emtaReference
	declaration.UpdatedAt = submittedAt
	return nil
}

func (m *MockRepository) UpdateTSDStatus(ctx context.Context, schemaName, tenantID, declarationID string, status TSDStatus, updatedAt time.Time) error {
	if m.UpdateTSDStatusErr != nil {
		return m.UpdateTSDStatusErr
	}
	declaration, ok := m.TSDDeclarations[declarationID]
	if !ok || declaration.TenantID != tenantID {
		return ErrTSDDeclarationNotFound
	}
	declaration.Status = status
	declaration.UpdatedAt = updatedAt
	return nil
}

func (m *MockRepository) WithTransaction(ctx context.Context, fn func(txRepo Repository) error) error {
	if m.WithTransactionErr != nil {
		return m.WithTransactionErr
	}
	if m.BeginTxErr != nil {
		return fmt.Errorf("begin transaction: %w", m.BeginTxErr)
	}
	if err := fn(m); err != nil {
		m.mockTx.RollbackCalled = true
		return err
	}
	if m.mockTx.CommitErr != nil {
		m.mockTx.RollbackCalled = true
		return fmt.Errorf("commit transaction: %w", m.mockTx.CommitErr)
	}
	m.mockTx.CommitCalled = true
	return nil
}

// ============================================================================
// SERVICE TESTS
// ============================================================================

func TestNewServiceWithRepository(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "test"}

	service := NewServiceWithRepository(repo, uuidGen)

	assert.NotNil(t, service)
	assert.Equal(t, repo, service.repo)
	assert.Equal(t, uuidGen, service.uuid)
}

func TestCreateEmployee_Success(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "emp"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	req := &CreateEmployeeRequest{
		FirstName:           "Mari",
		LastName:            "Maasikas",
		PersonalCode:        "38501234567",
		Email:               "mari@example.com",
		StartDate:           time.Now(),
		Position:            "Developer",
		Department:          "Engineering",
		ApplyBasicExemption: true,
	}

	emp, err := service.CreateEmployee(ctx, "test_schema", "tenant-1", req)

	require.NoError(t, err)
	assert.NotEmpty(t, emp.ID)
	assert.Equal(t, "Mari", emp.FirstName)
	assert.Equal(t, "Maasikas", emp.LastName)
	assert.Equal(t, EmploymentFullTime, emp.EmploymentType) // default
	assert.True(t, emp.ApplyBasicExemption)
	assert.Equal(t, DefaultBasicExemption, emp.BasicExemptionAmount)
	assert.True(t, emp.IsActive)
	assert.Equal(t, "EE", emp.TaxResidency)
}

func TestCreateEmployee_ValidationErrors(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "emp"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	tests := []struct {
		name    string
		req     *CreateEmployeeRequest
		wantErr string
	}{
		{
			name:    "missing first name",
			req:     &CreateEmployeeRequest{LastName: "Maasikas", StartDate: time.Now()},
			wantErr: "first name and last name are required",
		},
		{
			name:    "missing last name",
			req:     &CreateEmployeeRequest{FirstName: "Mari", StartDate: time.Now()},
			wantErr: "first name and last name are required",
		},
		{
			name:    "missing start date",
			req:     &CreateEmployeeRequest{FirstName: "Mari", LastName: "Maasikas"},
			wantErr: "start date is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.CreateEmployee(ctx, "test_schema", "tenant-1", tt.req)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestCreateEmployee_RepositoryError(t *testing.T) {
	repo := NewMockRepository()
	repo.CreateEmployeeErr = errors.New("database error")
	uuidGen := &MockUUIDGenerator{prefix: "emp"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	req := &CreateEmployeeRequest{
		FirstName: "Mari",
		LastName:  "Maasikas",
		StartDate: time.Now(),
	}

	_, err := service.CreateEmployee(ctx, "test_schema", "tenant-1", req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create employee")
}

func TestGetEmployee_Success(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "emp"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	// Setup test data
	repo.Employees["emp-1"] = &Employee{
		ID:        "emp-1",
		TenantID:  "tenant-1",
		FirstName: "Mari",
		LastName:  "Maasikas",
	}

	emp, err := service.GetEmployee(ctx, "test_schema", "tenant-1", "emp-1")

	require.NoError(t, err)
	assert.Equal(t, "emp-1", emp.ID)
	assert.Equal(t, "Mari", emp.FirstName)
}

func TestGetEmployee_NotFound(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "emp"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	_, err := service.GetEmployee(ctx, "test_schema", "tenant-1", "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "employee not found")
}

func TestListEmployees_Success(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "emp"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	// Setup test data
	repo.Employees["emp-1"] = &Employee{ID: "emp-1", TenantID: "tenant-1", FirstName: "Mari", IsActive: true}
	repo.Employees["emp-2"] = &Employee{ID: "emp-2", TenantID: "tenant-1", FirstName: "Jaan", IsActive: false}
	repo.Employees["emp-3"] = &Employee{ID: "emp-3", TenantID: "tenant-2", FirstName: "Peeter", IsActive: true}

	employees, err := service.ListEmployees(ctx, "test_schema", "tenant-1", false)

	require.NoError(t, err)
	assert.Len(t, employees, 2) // Only tenant-1 employees
}

func TestListEmployees_ActiveOnly(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "emp"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	// Setup test data
	repo.Employees["emp-1"] = &Employee{ID: "emp-1", TenantID: "tenant-1", FirstName: "Mari", IsActive: true}
	repo.Employees["emp-2"] = &Employee{ID: "emp-2", TenantID: "tenant-1", FirstName: "Jaan", IsActive: false}

	employees, err := service.ListEmployees(ctx, "test_schema", "tenant-1", true)

	require.NoError(t, err)
	assert.Len(t, employees, 1)
	assert.Equal(t, "emp-1", employees[0].ID)
}

func TestUpdateEmployee_Success(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "emp"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	// Setup test data
	repo.Employees["emp-1"] = &Employee{
		ID:        "emp-1",
		TenantID:  "tenant-1",
		FirstName: "Mari",
		LastName:  "Maasikas",
		Position:  "Developer",
	}

	req := &UpdateEmployeeRequest{
		Position: "Senior Developer",
	}

	emp, err := service.UpdateEmployee(ctx, "test_schema", "tenant-1", "emp-1", req)

	require.NoError(t, err)
	assert.Equal(t, "Senior Developer", emp.Position)
	assert.Equal(t, "Mari", emp.FirstName) // Unchanged
}

func TestUpdateEmployee_AllFields(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "emp"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	// Setup test data
	repo.Employees["emp-1"] = &Employee{
		ID:       "emp-1",
		TenantID: "tenant-1",
	}

	isActive := false
	applyExemption := true
	basicExemption := decimal.NewFromFloat(800)
	pensionRate := decimal.NewFromFloat(0.04)
	endDate := time.Now()

	req := &UpdateEmployeeRequest{
		EmployeeNumber:       "E001",
		FirstName:            "Updated",
		LastName:             "Name",
		PersonalCode:         "12345678901",
		Email:                "updated@example.com",
		Phone:                "+372 5551234",
		Address:              "New Address",
		BankAccount:          "EE123456789",
		EndDate:              &endDate,
		Position:             "Manager",
		Department:           "Sales",
		EmploymentType:       EmploymentPartTime,
		ApplyBasicExemption:  &applyExemption,
		BasicExemptionAmount: &basicExemption,
		FundedPensionRate:    &pensionRate,
		IsActive:             &isActive,
	}

	emp, err := service.UpdateEmployee(ctx, "test_schema", "tenant-1", "emp-1", req)

	require.NoError(t, err)
	assert.Equal(t, "E001", emp.EmployeeNumber)
	assert.Equal(t, "Updated", emp.FirstName)
	assert.Equal(t, "Name", emp.LastName)
	assert.Equal(t, "updated@example.com", emp.Email)
	assert.Equal(t, EmploymentPartTime, emp.EmploymentType)
	assert.Equal(t, basicExemption, emp.BasicExemptionAmount)
	assert.Equal(t, pensionRate, emp.FundedPensionRate)
	assert.False(t, emp.IsActive)
}

func TestSetBaseSalary_Success(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "comp"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	amount := decimal.NewFromFloat(2000)
	effectiveFrom := time.Now()

	err := service.SetBaseSalary(ctx, "test_schema", "tenant-1", "emp-1", amount, effectiveFrom)

	require.NoError(t, err)
	assert.True(t, repo.Salaries["emp-1"].Equal(amount))
}

func TestSetBaseSalary_Error(t *testing.T) {
	repo := NewMockRepository()
	repo.CreateSalaryComponentErr = errors.New("database error")
	uuidGen := &MockUUIDGenerator{prefix: "comp"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	err := service.SetBaseSalary(ctx, "test_schema", "tenant-1", "emp-1", decimal.NewFromFloat(2000), time.Now())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "set base salary")
}

func TestSetBaseSalary_ValidationErrors(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "comp"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()
	effectiveFrom := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name          string
		amount        decimal.Decimal
		effectiveFrom time.Time
		wantError     string
	}{
		{
			name:          "zero amount",
			amount:        decimal.Zero,
			effectiveFrom: effectiveFrom,
			wantError:     "amount must be positive",
		},
		{
			name:          "negative amount",
			amount:        decimal.NewFromInt(-1),
			effectiveFrom: effectiveFrom,
			wantError:     "amount must be positive",
		},
		{
			name:      "missing effective from",
			amount:    decimal.NewFromInt(2000),
			wantError: "effective from date is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.SetBaseSalary(ctx, "test_schema", "tenant-1", "emp-1", tt.amount, tt.effectiveFrom)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantError)
			assert.Empty(t, repo.SalaryComponents["emp-1"])
		})
	}
}

func TestSetBaseSalary_IgnoresEndCurrentBaseSalaryError(t *testing.T) {
	repo := NewMockRepository()
	repo.EndCurrentBaseSalaryErr = errors.New("current salary not found")
	uuidGen := &MockUUIDGenerator{prefix: "comp"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()
	effectiveFrom := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	err := service.SetBaseSalary(ctx, "test_schema", "tenant-1", "emp-1", decimal.NewFromInt(2400), effectiveFrom)

	require.NoError(t, err)
	require.Len(t, repo.SalaryComponents["emp-1"], 1)
	component := repo.SalaryComponents["emp-1"][0]
	assert.Equal(t, SalaryComponentBaseSalary, component.ComponentType)
	assert.True(t, component.Amount.Equal(decimal.NewFromInt(2400)))
	assert.Equal(t, effectiveFrom, component.EffectiveFrom)
	assert.Nil(t, component.EffectiveTo)
}

func TestAddSalaryComponent_DefaultsAndSumsCurrentSalary(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "comp"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()
	effectiveFrom := time.Now().AddDate(0, 0, -1)
	repo.Employees["emp-1"] = &Employee{
		ID:       "emp-1",
		TenantID: "tenant-1",
	}

	err := service.SetBaseSalary(ctx, "test_schema", "tenant-1", "emp-1", decimal.NewFromInt(2000), effectiveFrom)
	require.NoError(t, err)

	component, err := service.AddSalaryComponent(ctx, "test_schema", "tenant-1", "emp-1", &CreateSalaryComponentRequest{
		Amount:        decimal.NewFromInt(600),
		EffectiveFrom: effectiveFrom,
	})

	require.NoError(t, err)
	assert.Equal(t, SalaryComponentSecondaryEmployment, component.ComponentType)
	assert.Equal(t, "Secondary employment", component.Name)
	assert.True(t, component.IsTaxable)
	assert.True(t, component.IsRecurring)

	salary, err := service.GetCurrentSalary(ctx, "test_schema", "tenant-1", "emp-1")
	require.NoError(t, err)
	assert.True(t, salary.Equal(decimal.NewFromInt(2600)))
}

func TestAddSalaryComponent_DefaultNamesForSupportedTypes(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "comp"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()
	effectiveFrom := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	repo.Employees["emp-1"] = &Employee{
		ID:       "emp-1",
		TenantID: "tenant-1",
	}

	tests := []struct {
		name          string
		componentType string
		wantType      string
		wantName      string
	}{
		{
			name:          "base salary",
			componentType: " base_salary ",
			wantType:      SalaryComponentBaseSalary,
			wantName:      "Base Salary",
		},
		{
			name:          "bonus",
			componentType: SalaryComponentBonus,
			wantType:      SalaryComponentBonus,
			wantName:      "Bonus",
		},
		{
			name:          "commission",
			componentType: SalaryComponentCommission,
			wantType:      SalaryComponentCommission,
			wantName:      "Commission",
		},
		{
			name:          "benefit",
			componentType: SalaryComponentBenefit,
			wantType:      SalaryComponentBenefit,
			wantName:      "Benefit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			component, err := service.AddSalaryComponent(ctx, "test_schema", "tenant-1", "emp-1", &CreateSalaryComponentRequest{
				ComponentType: tt.componentType,
				Amount:        decimal.NewFromInt(100),
				EffectiveFrom: effectiveFrom,
			})
			require.NoError(t, err)
			assert.Equal(t, tt.wantType, component.ComponentType)
			assert.Equal(t, tt.wantName, component.Name)
			assert.True(t, component.IsTaxable)
			assert.True(t, component.IsRecurring)
		})
	}
}

func TestAddSalaryComponent_UsesProvidedNameAndFlags(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "comp"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()
	effectiveFrom := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	effectiveTo := effectiveFrom.AddDate(0, 6, 0)
	isTaxable := false
	isRecurring := false
	repo.Employees["emp-1"] = &Employee{
		ID:       "emp-1",
		TenantID: "tenant-1",
	}

	component, err := service.AddSalaryComponent(ctx, "test_schema", "tenant-1", "emp-1", &CreateSalaryComponentRequest{
		ComponentType: SalaryComponentBonus,
		Name:          "  Launch bonus  ",
		Amount:        decimal.NewFromInt(500),
		IsTaxable:     &isTaxable,
		IsRecurring:   &isRecurring,
		EffectiveFrom: effectiveFrom,
		EffectiveTo:   &effectiveTo,
	})

	require.NoError(t, err)
	assert.Equal(t, "Launch bonus", component.Name)
	assert.False(t, component.IsTaxable)
	assert.False(t, component.IsRecurring)
	assert.Equal(t, &effectiveTo, component.EffectiveTo)
}

func TestAddSalaryComponent_ValidationErrors(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "comp"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()
	effectiveFrom := time.Now()
	repo.Employees["emp-1"] = &Employee{
		ID:       "emp-1",
		TenantID: "tenant-1",
	}
	effectiveToBefore := effectiveFrom.AddDate(0, 0, -1)

	tests := []struct {
		name       string
		employeeID string
		req        *CreateSalaryComponentRequest
		wantError  string
	}{
		{
			name:       "nil request",
			employeeID: "emp-1",
			req:        nil,
			wantError:  "salary component request is required",
		},
		{
			name:       "employee missing",
			employeeID: "missing",
			req: &CreateSalaryComponentRequest{
				Amount:        decimal.NewFromInt(100),
				EffectiveFrom: effectiveFrom,
			},
			wantError: "employee not found",
		},
		{
			name:       "unsupported type",
			employeeID: "emp-1",
			req: &CreateSalaryComponentRequest{
				ComponentType: "allowance",
				Amount:        decimal.NewFromInt(100),
				EffectiveFrom: effectiveFrom,
			},
			wantError: "unsupported salary component type",
		},
		{
			name:       "zero amount",
			employeeID: "emp-1",
			req: &CreateSalaryComponentRequest{
				Amount:        decimal.Zero,
				EffectiveFrom: effectiveFrom,
			},
			wantError: "amount must be positive",
		},
		{
			name:       "missing effective from",
			employeeID: "emp-1",
			req: &CreateSalaryComponentRequest{
				Amount: decimal.NewFromInt(100),
			},
			wantError: "effective from date is required",
		},
		{
			name:       "effective to before from",
			employeeID: "emp-1",
			req: &CreateSalaryComponentRequest{
				Amount:        decimal.NewFromInt(100),
				EffectiveFrom: effectiveFrom,
				EffectiveTo:   &effectiveToBefore,
			},
			wantError: "effective to date must be on or after effective from date",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			component, err := service.AddSalaryComponent(ctx, "test_schema", "tenant-1", tt.employeeID, tt.req)
			require.Error(t, err)
			assert.Nil(t, component)
			assert.Contains(t, err.Error(), tt.wantError)
		})
	}
}

func TestAddSalaryComponent_RepositoryError(t *testing.T) {
	repo := NewMockRepository()
	repo.CreateSalaryComponentErr = errors.New("database error")
	uuidGen := &MockUUIDGenerator{prefix: "comp"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()
	repo.Employees["emp-1"] = &Employee{
		ID:       "emp-1",
		TenantID: "tenant-1",
	}

	component, err := service.AddSalaryComponent(ctx, "test_schema", "tenant-1", "emp-1", &CreateSalaryComponentRequest{
		Amount:        decimal.NewFromInt(100),
		EffectiveFrom: time.Now(),
	})

	require.Error(t, err)
	assert.Nil(t, component)
	assert.Contains(t, err.Error(), "create salary component")
}

func TestListSalaryComponents_ActiveOn(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "comp"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()
	activeOn := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	ended := activeOn.AddDate(0, 0, -1)
	repo.Employees["emp-1"] = &Employee{
		ID:       "emp-1",
		TenantID: "tenant-1",
	}
	repo.SalaryComponents["emp-1"] = []SalaryComponent{
		{
			ID:            "comp-active",
			TenantID:      "tenant-1",
			EmployeeID:    "emp-1",
			ComponentType: SalaryComponentSecondaryEmployment,
			Name:          "Evening contract",
			Amount:        decimal.NewFromInt(600),
			IsRecurring:   true,
			EffectiveFrom: activeOn.AddDate(0, -1, 0),
		},
		{
			ID:            "comp-ended",
			TenantID:      "tenant-1",
			EmployeeID:    "emp-1",
			ComponentType: SalaryComponentBonus,
			Name:          "Ended bonus",
			Amount:        decimal.NewFromInt(100),
			IsRecurring:   true,
			EffectiveFrom: activeOn.AddDate(0, -2, 0),
			EffectiveTo:   &ended,
		},
		{
			ID:            "comp-future",
			TenantID:      "tenant-1",
			EmployeeID:    "emp-1",
			ComponentType: SalaryComponentBenefit,
			Name:          "Future benefit",
			Amount:        decimal.NewFromInt(100),
			IsRecurring:   true,
			EffectiveFrom: activeOn.AddDate(0, 0, 1),
		},
	}

	components, err := service.ListSalaryComponents(ctx, "test_schema", "tenant-1", "emp-1", &activeOn)

	require.NoError(t, err)
	require.Len(t, components, 1)
	assert.Equal(t, "comp-active", components[0].ID)
}

func TestListSalaryComponents_Errors(t *testing.T) {
	ctx := context.Background()

	t.Run("employee missing", func(t *testing.T) {
		repo := NewMockRepository()
		service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "comp"})

		components, err := service.ListSalaryComponents(ctx, "test_schema", "tenant-1", "missing", nil)

		require.Error(t, err)
		assert.Nil(t, components)
		assert.Contains(t, err.Error(), "employee not found")
	})

	t.Run("repository error", func(t *testing.T) {
		repo := NewMockRepository()
		repo.ListSalaryComponentsErr = errors.New("database error")
		service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "comp"})
		repo.Employees["emp-1"] = &Employee{
			ID:       "emp-1",
			TenantID: "tenant-1",
		}

		components, err := service.ListSalaryComponents(ctx, "test_schema", "tenant-1", "emp-1", nil)

		require.Error(t, err)
		assert.Nil(t, components)
		assert.Contains(t, err.Error(), "list salary components")
	})
}

func TestGetCurrentSalary_Success(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "emp"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	repo.Salaries["emp-1"] = decimal.NewFromFloat(2500)

	salary, err := service.GetCurrentSalary(ctx, "test_schema", "tenant-1", "emp-1")

	require.NoError(t, err)
	assert.True(t, salary.Equal(decimal.NewFromFloat(2500)))
}

func TestGetCurrentSalary_NoSalary(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "emp"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	salary, err := service.GetCurrentSalary(ctx, "test_schema", "tenant-1", "emp-1")

	require.NoError(t, err)
	assert.True(t, salary.IsZero())
}

func TestCreatePayrollRun_Success(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "run"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	paymentDate := time.Now()
	req := &CreatePayrollRunRequest{
		PeriodYear:  2025,
		PeriodMonth: 1,
		PaymentDate: &paymentDate,
		Notes:       "January payroll",
	}

	run, err := service.CreatePayrollRun(ctx, "test_schema", "tenant-1", "user-1", req)

	require.NoError(t, err)
	assert.NotEmpty(t, run.ID)
	assert.Equal(t, 2025, run.PeriodYear)
	assert.Equal(t, 1, run.PeriodMonth)
	assert.Equal(t, PayrollDraft, run.Status)
	assert.Equal(t, "user-1", run.CreatedBy)
	assert.Equal(t, []string{"payroll_run_calculate"}, payrollRunRemediationCodes(run.RemediationActions))
}

func TestCreatePayrollRun_ValidationErrors(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "run"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	tests := []struct {
		name    string
		req     *CreatePayrollRunRequest
		wantErr string
	}{
		{
			name:    "invalid year - too low",
			req:     &CreatePayrollRunRequest{PeriodYear: 2019, PeriodMonth: 1},
			wantErr: "invalid period year",
		},
		{
			name:    "invalid year - too high",
			req:     &CreatePayrollRunRequest{PeriodYear: 2101, PeriodMonth: 1},
			wantErr: "invalid period year",
		},
		{
			name:    "invalid month - zero",
			req:     &CreatePayrollRunRequest{PeriodYear: 2025, PeriodMonth: 0},
			wantErr: "invalid period month",
		},
		{
			name:    "invalid month - 13",
			req:     &CreatePayrollRunRequest{PeriodYear: 2025, PeriodMonth: 13},
			wantErr: "invalid period month",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.CreatePayrollRun(ctx, "test_schema", "tenant-1", "user-1", tt.req)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestGetPayrollRun_Success(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "run"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	repo.PayrollRuns["run-1"] = &PayrollRun{
		ID:          "run-1",
		TenantID:    "tenant-1",
		PeriodYear:  2025,
		PeriodMonth: 1,
		Status:      PayrollDraft,
	}

	run, err := service.GetPayrollRun(ctx, "test_schema", "tenant-1", "run-1")

	require.NoError(t, err)
	assert.Equal(t, "run-1", run.ID)
	assert.Equal(t, PayrollDraft, run.Status)
	assert.NotEmpty(t, run.RemediationActions)
}

func TestGetPayrollRun_NotFound(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "run"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	_, err := service.GetPayrollRun(ctx, "test_schema", "tenant-1", "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "payroll run not found")
}

func TestListPayrollRuns_Success(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "run"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	repo.PayrollRuns["run-1"] = &PayrollRun{ID: "run-1", TenantID: "tenant-1", PeriodYear: 2025}
	repo.PayrollRuns["run-2"] = &PayrollRun{ID: "run-2", TenantID: "tenant-1", PeriodYear: 2024}
	repo.PayrollRuns["run-3"] = &PayrollRun{ID: "run-3", TenantID: "tenant-2", PeriodYear: 2025}

	runs, err := service.ListPayrollRuns(ctx, "test_schema", "tenant-1", 0)

	require.NoError(t, err)
	assert.Len(t, runs, 2)
	assert.NotEmpty(t, runs[0].RemediationActions)
	assert.NotEmpty(t, runs[1].RemediationActions)
}

func TestListPayrollRuns_FilterByYear(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "run"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	repo.PayrollRuns["run-1"] = &PayrollRun{ID: "run-1", TenantID: "tenant-1", PeriodYear: 2025}
	repo.PayrollRuns["run-2"] = &PayrollRun{ID: "run-2", TenantID: "tenant-1", PeriodYear: 2024}

	runs, err := service.ListPayrollRuns(ctx, "test_schema", "tenant-1", 2025)

	require.NoError(t, err)
	assert.Len(t, runs, 1)
	assert.Equal(t, 2025, runs[0].PeriodYear)
	assert.NotEmpty(t, runs[0].RemediationActions)
}

func TestApprovePayrollRun_Success(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "run"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	repo.PayrollRuns["run-1"] = &PayrollRun{
		ID:       "run-1",
		TenantID: "tenant-1",
		Status:   PayrollCalculated,
	}

	err := service.ApprovePayrollRun(ctx, "test_schema", "tenant-1", "run-1", "approver-1")

	require.NoError(t, err)
	assert.Equal(t, PayrollApproved, repo.PayrollRuns["run-1"].Status)
}

func TestApprovePayrollRun_NotCalculated(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "run"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	repo.PayrollRuns["run-1"] = &PayrollRun{
		ID:       "run-1",
		TenantID: "tenant-1",
		Status:   PayrollDraft, // Not in CALCULATED status
	}

	err := service.ApprovePayrollRun(ctx, "test_schema", "tenant-1", "run-1", "approver-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found or not in CALCULATED status")
}

func TestCalculatePayroll_Success(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	// Setup payroll run
	repo.PayrollRuns["run-1"] = &PayrollRun{
		ID:          "run-1",
		TenantID:    "tenant-1",
		PeriodYear:  2025,
		PeriodMonth: 1,
		Status:      PayrollDraft,
	}

	// Setup employees
	repo.Employees["emp-1"] = &Employee{
		ID:                   "emp-1",
		TenantID:             "tenant-1",
		FirstName:            "Mari",
		LastName:             "Maasikas",
		IsActive:             true,
		ApplyBasicExemption:  true,
		BasicExemptionAmount: decimal.NewFromFloat(700),
		FundedPensionRate:    decimal.NewFromFloat(0.02),
	}

	// Setup salary
	repo.Salaries["emp-1"] = decimal.NewFromFloat(2000)

	run, err := service.CalculatePayroll(ctx, "test_schema", "tenant-1", "run-1")

	require.NoError(t, err)
	assert.Equal(t, PayrollCalculated, run.Status)
	assert.Len(t, run.Payslips, 1)
	assert.True(t, run.TotalGross.Equal(decimal.NewFromFloat(2000)))
	assert.Equal(t, []string{"payroll_payment_date_missing", "payroll_run_approve"}, payrollRunRemediationCodes(run.RemediationActions))
}

func TestProcessPayrollRun_CalculateOnly(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "process"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	repo.PayrollRuns["run-1"] = &PayrollRun{
		ID:          "run-1",
		TenantID:    "tenant-1",
		PeriodYear:  2026,
		PeriodMonth: 3,
		Status:      PayrollDraft,
	}
	repo.Employees["emp-1"] = &Employee{
		ID:                   "emp-1",
		TenantID:             "tenant-1",
		FirstName:            "Mari",
		LastName:             "Maasikas",
		IsActive:             true,
		ApplyBasicExemption:  true,
		BasicExemptionAmount: DefaultBasicExemption,
		FundedPensionRate:    FundedPensionRateDefault,
	}
	repo.Salaries["emp-1"] = decimal.NewFromFloat(3200)

	result, err := service.ProcessPayrollRun(ctx, "test_schema", "tenant-1", "run-1", "approver-1", nil)

	require.NoError(t, err)
	require.NotNil(t, result.PayrollRun)
	assert.Equal(t, 1, result.PayslipCount)
	assert.False(t, result.Approved)
	assert.Equal(t, PayrollCalculated, result.PayrollRun.Status)
	assert.True(t, result.PayrollRun.TotalGross.Equal(decimal.NewFromFloat(3200)))
	assert.Equal(t, []string{"payroll_payment_date_missing", "payroll_run_approve"}, payrollRunRemediationCodes(result.PayrollRun.RemediationActions))
	assert.True(t, repo.mockTx.CommitCalled)
}

func TestProcessPayrollRun_CalculatesAndApproves(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "process"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	repo.PayrollRuns["run-1"] = &PayrollRun{
		ID:          "run-1",
		TenantID:    "tenant-1",
		PeriodYear:  2026,
		PeriodMonth: 3,
		Status:      PayrollDraft,
	}
	repo.Employees["emp-1"] = &Employee{
		ID:                   "emp-1",
		TenantID:             "tenant-1",
		FirstName:            "Mari",
		LastName:             "Maasikas",
		IsActive:             true,
		ApplyBasicExemption:  true,
		BasicExemptionAmount: DefaultBasicExemption,
		FundedPensionRate:    FundedPensionRateDefault,
	}
	repo.Salaries["emp-1"] = decimal.NewFromFloat(3200)

	result, err := service.ProcessPayrollRun(ctx, "test_schema", "tenant-1", "run-1", "approver-1", &ProcessPayrollRunRequest{Approve: true})

	require.NoError(t, err)
	require.NotNil(t, result.PayrollRun)
	assert.Equal(t, 1, result.PayslipCount)
	assert.True(t, result.Approved)
	assert.Equal(t, PayrollApproved, result.PayrollRun.Status)
	assert.Equal(t, "approver-1", result.PayrollRun.ApprovedBy)
	assert.NotNil(t, result.PayrollRun.ApprovedAt)
	assert.True(t, result.PayrollRun.TotalNet.GreaterThan(decimal.Zero))
	assert.Equal(t, []string{"payroll_payment_date_missing", "payroll_generate_tsd"}, payrollRunRemediationCodes(result.PayrollRun.RemediationActions))
	assert.True(t, repo.mockTx.CommitCalled)
}

func TestProcessPayrollRun_CalculateError(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "process"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	result, err := service.ProcessPayrollRun(ctx, "test_schema", "tenant-1", "missing", "approver-1", nil)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "payroll run not found")
}

func TestProcessPayrollRun_ApproveError(t *testing.T) {
	repo := NewMockRepository()
	repo.ApprovePayrollRunErr = errors.New("database error")
	uuidGen := &MockUUIDGenerator{prefix: "process"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	repo.PayrollRuns["run-1"] = &PayrollRun{
		ID:          "run-1",
		TenantID:    "tenant-1",
		PeriodYear:  2026,
		PeriodMonth: 3,
		Status:      PayrollDraft,
	}
	repo.Employees["emp-1"] = &Employee{
		ID:                   "emp-1",
		TenantID:             "tenant-1",
		FirstName:            "Mari",
		LastName:             "Maasikas",
		IsActive:             true,
		ApplyBasicExemption:  true,
		BasicExemptionAmount: DefaultBasicExemption,
		FundedPensionRate:    FundedPensionRateDefault,
	}
	repo.Salaries["emp-1"] = decimal.NewFromFloat(3200)

	result, err := service.ProcessPayrollRun(ctx, "test_schema", "tenant-1", "run-1", "approver-1", &ProcessPayrollRunRequest{Approve: true})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "approve payroll run")
	assert.Equal(t, PayrollCalculated, repo.PayrollRuns["run-1"].Status)
	assert.True(t, repo.mockTx.CommitCalled)
}

func TestCalculatePayroll_NotDraftStatus(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	repo.PayrollRuns["run-1"] = &PayrollRun{
		ID:       "run-1",
		TenantID: "tenant-1",
		Status:   PayrollCalculated, // Already calculated
	}

	_, err := service.CalculatePayroll(ctx, "test_schema", "tenant-1", "run-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be in DRAFT status")
}

func TestCalculatePayroll_SkipsEmployeesWithoutSalary(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	repo.PayrollRuns["run-1"] = &PayrollRun{
		ID:       "run-1",
		TenantID: "tenant-1",
		Status:   PayrollDraft,
	}

	repo.Employees["emp-1"] = &Employee{
		ID:       "emp-1",
		TenantID: "tenant-1",
		IsActive: true,
	}
	// No salary set for emp-1

	run, err := service.CalculatePayroll(ctx, "test_schema", "tenant-1", "run-1")

	require.NoError(t, err)
	assert.Len(t, run.Payslips, 0) // No payslips created
	assert.Equal(t, []string{"payroll_payment_date_missing", "payroll_no_payslips", "payroll_run_approve"}, payrollRunRemediationCodes(run.RemediationActions))
}

func TestBuildPayrollRunRemediationActions(t *testing.T) {
	paymentDate := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		run       *PayrollRun
		wantCodes []string
	}{
		{
			name: "draft with payment date",
			run: &PayrollRun{
				ID:          "run-draft",
				PeriodYear:  2026,
				PeriodMonth: 3,
				Status:      PayrollDraft,
				PaymentDate: &paymentDate,
			},
			wantCodes: []string{"payroll_run_calculate"},
		},
		{
			name: "draft missing payment date",
			run: &PayrollRun{
				ID:          "run-draft-no-date",
				PeriodYear:  2026,
				PeriodMonth: 3,
				Status:      PayrollDraft,
			},
			wantCodes: []string{"payroll_payment_date_missing", "payroll_run_calculate"},
		},
		{
			name: "calculated with payslips",
			run: &PayrollRun{
				ID:          "run-calculated",
				PeriodYear:  2026,
				PeriodMonth: 4,
				Status:      PayrollCalculated,
				PaymentDate: &paymentDate,
				TotalGross:  decimal.NewFromInt(3200),
				Payslips:    []Payslip{{ID: "payslip-1"}},
			},
			wantCodes: []string{"payroll_run_approve"},
		},
		{
			name: "calculated empty",
			run: &PayrollRun{
				ID:          "run-empty",
				PeriodYear:  2026,
				PeriodMonth: 5,
				Status:      PayrollCalculated,
			},
			wantCodes: []string{"payroll_payment_date_missing", "payroll_no_payslips", "payroll_run_approve"},
		},
		{
			name: "approved",
			run: &PayrollRun{
				ID:          "run-approved",
				PeriodYear:  2026,
				PeriodMonth: 6,
				Status:      PayrollApproved,
				PaymentDate: &paymentDate,
			},
			wantCodes: []string{"payroll_generate_tsd"},
		},
		{
			name: "paid",
			run: &PayrollRun{
				ID:          "run-paid",
				PeriodYear:  2026,
				PeriodMonth: 7,
				Status:      PayrollPaid,
				PaymentDate: &paymentDate,
			},
			wantCodes: []string{"payroll_paid_tsd_followup"},
		},
		{
			name: "declared",
			run: &PayrollRun{
				ID:          "run-declared",
				PeriodYear:  2026,
				PeriodMonth: 8,
				Status:      PayrollDeclared,
			},
			wantCodes: []string{"payroll_declared_archive"},
		},
		{
			name: "unknown",
			run: &PayrollRun{
				ID:          "run-unknown",
				PeriodYear:  2026,
				PeriodMonth: 9,
				Status:      PayrollStatus("stale"),
			},
			wantCodes: []string{"payroll_payment_date_missing", "payroll_status_review"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actions := BuildPayrollRunRemediationActions(tt.run)
			assert.Equal(t, tt.wantCodes, payrollRunRemediationCodes(actions))
			for _, action := range actions {
				assert.Equal(t, "payroll", action.Scope)
				assert.Equal(t, "accountant", action.OwnerRole)
				assert.NotEmpty(t, action.Period)
				assert.NotEmpty(t, action.Action)
				assert.Equal(t, "payroll_runs", action.WorkspaceQueue)
				assert.NotEmpty(t, action.AssignmentKey)
				assert.NotEmpty(t, action.Priority)
				if action.Severity == "ACTION" {
					assert.Equal(t, "high", action.Priority)
					assert.Equal(t, 1, action.DueInDays)
				}
			}
		})
	}

	assert.Nil(t, BuildPayrollRunRemediationActions(nil))
}

func TestCalculatePayroll_MultipleEmployees(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	repo.PayrollRuns["run-1"] = &PayrollRun{
		ID:       "run-1",
		TenantID: "tenant-1",
		Status:   PayrollDraft,
	}

	repo.Employees["emp-1"] = &Employee{
		ID:                   "emp-1",
		TenantID:             "tenant-1",
		IsActive:             true,
		ApplyBasicExemption:  true,
		BasicExemptionAmount: DefaultBasicExemption,
		FundedPensionRate:    FundedPensionRateDefault,
	}
	repo.Employees["emp-2"] = &Employee{
		ID:                  "emp-2",
		TenantID:            "tenant-1",
		IsActive:            true,
		ApplyBasicExemption: false,
		FundedPensionRate:   FundedPensionRateDefault,
	}

	repo.Salaries["emp-1"] = decimal.NewFromFloat(2000)
	repo.Salaries["emp-2"] = decimal.NewFromFloat(3000)

	run, err := service.CalculatePayroll(ctx, "test_schema", "tenant-1", "run-1")

	require.NoError(t, err)
	assert.Len(t, run.Payslips, 2)
	assert.True(t, run.TotalGross.Equal(decimal.NewFromFloat(5000)))
}

func TestCalculatePayroll_TransactionError(t *testing.T) {
	repo := NewMockRepository()
	repo.BeginTxErr = errors.New("transaction error")
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	repo.PayrollRuns["run-1"] = &PayrollRun{
		ID:       "run-1",
		TenantID: "tenant-1",
		Status:   PayrollDraft,
	}

	_, err := service.CalculatePayroll(ctx, "test_schema", "tenant-1", "run-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "begin transaction")
}

func TestErrorDefinitions(t *testing.T) {
	assert.NotNil(t, ErrEmployeeNotFound)
	assert.NotNil(t, ErrPayrollRunNotFound)
	assert.Equal(t, "employee not found", ErrEmployeeNotFound.Error())
	assert.Equal(t, "payroll run not found", ErrPayrollRunNotFound.Error())
}

func TestDefaultUUIDGenerator(t *testing.T) {
	gen := &DefaultUUIDGenerator{}
	uuid1 := gen.New()
	uuid2 := gen.New()

	assert.NotEmpty(t, uuid1)
	assert.NotEmpty(t, uuid2)
	assert.NotEqual(t, uuid1, uuid2)
}

func TestGetEmployee_RepositoryError(t *testing.T) {
	repo := NewMockRepository()
	repo.GetEmployeeErr = errors.New("database error")
	uuidGen := &MockUUIDGenerator{prefix: "emp"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	_, err := service.GetEmployee(ctx, "test_schema", "tenant-1", "emp-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get employee")
}

func TestListEmployees_RepositoryError(t *testing.T) {
	repo := NewMockRepository()
	repo.ListEmployeesErr = errors.New("database error")
	uuidGen := &MockUUIDGenerator{prefix: "emp"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	_, err := service.ListEmployees(ctx, "test_schema", "tenant-1", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list employees")
}

func TestUpdateEmployee_RepositoryError(t *testing.T) {
	repo := NewMockRepository()
	repo.UpdateEmployeeErr = errors.New("database error")
	uuidGen := &MockUUIDGenerator{prefix: "emp"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	// First add an employee to update
	repo.Employees["emp-1"] = &Employee{
		ID:       "emp-1",
		TenantID: "tenant-1",
	}

	req := &UpdateEmployeeRequest{
		Position: "Manager",
	}

	_, err := service.UpdateEmployee(ctx, "test_schema", "tenant-1", "emp-1", req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update employee")
}

func TestUpdateEmployee_NotFound(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "emp"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	req := &UpdateEmployeeRequest{
		Position: "Manager",
	}

	_, err := service.UpdateEmployee(ctx, "test_schema", "tenant-1", "nonexistent", req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "employee not found")
}

func TestGetCurrentSalary_RepositoryError(t *testing.T) {
	repo := NewMockRepository()
	repo.GetCurrentSalaryErr = errors.New("database error")
	uuidGen := &MockUUIDGenerator{prefix: "emp"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	_, err := service.GetCurrentSalary(ctx, "test_schema", "tenant-1", "emp-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get current salary")
}

func TestCreatePayrollRun_RepositoryError(t *testing.T) {
	repo := NewMockRepository()
	repo.CreatePayrollRunErr = errors.New("database error")
	uuidGen := &MockUUIDGenerator{prefix: "run"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	req := &CreatePayrollRunRequest{
		PeriodYear:  2025,
		PeriodMonth: 1,
	}

	_, err := service.CreatePayrollRun(ctx, "test_schema", "tenant-1", "user-1", req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create payroll run")
}

func TestGetPayrollRun_RepositoryError(t *testing.T) {
	repo := NewMockRepository()
	repo.GetPayrollRunErr = errors.New("database error")
	uuidGen := &MockUUIDGenerator{prefix: "run"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	_, err := service.GetPayrollRun(ctx, "test_schema", "tenant-1", "run-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get payroll run")
}

func TestListPayrollRuns_RepositoryError(t *testing.T) {
	repo := NewMockRepository()
	repo.ListPayrollRunsErr = errors.New("database error")
	uuidGen := &MockUUIDGenerator{prefix: "run"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	_, err := service.ListPayrollRuns(ctx, "test_schema", "tenant-1", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list payroll runs")
}

func TestApprovePayrollRun_RepositoryError(t *testing.T) {
	repo := NewMockRepository()
	repo.ApprovePayrollRunErr = errors.New("database error")
	uuidGen := &MockUUIDGenerator{prefix: "run"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	// Setup a payroll run in calculated status
	repo.PayrollRuns["run-1"] = &PayrollRun{
		ID:       "run-1",
		TenantID: "tenant-1",
		Status:   PayrollCalculated,
	}

	err := service.ApprovePayrollRun(ctx, "test_schema", "tenant-1", "run-1", "approver-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "approve payroll run")
}

func TestCalculatePayroll_PayrollRunNotFound(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	_, err := service.CalculatePayroll(ctx, "test_schema", "tenant-1", "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "payroll run not found")
}

func TestCalculatePayroll_ListEmployeesError(t *testing.T) {
	repo := NewMockRepository()
	repo.ListEmployeesErr = errors.New("database error")
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	repo.PayrollRuns["run-1"] = &PayrollRun{
		ID:       "run-1",
		TenantID: "tenant-1",
		Status:   PayrollDraft,
	}

	_, err := service.CalculatePayroll(ctx, "test_schema", "tenant-1", "run-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list employees")
}

func TestCalculatePayroll_CreatePayslipError(t *testing.T) {
	repo := NewMockRepository()
	repo.CreatePayslipErr = errors.New("database error")
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	repo.PayrollRuns["run-1"] = &PayrollRun{
		ID:       "run-1",
		TenantID: "tenant-1",
		Status:   PayrollDraft,
	}

	repo.Employees["emp-1"] = &Employee{
		ID:                   "emp-1",
		TenantID:             "tenant-1",
		IsActive:             true,
		ApplyBasicExemption:  true,
		BasicExemptionAmount: DefaultBasicExemption,
		FundedPensionRate:    FundedPensionRateDefault,
	}
	repo.Salaries["emp-1"] = decimal.NewFromFloat(2000)

	_, err := service.CalculatePayroll(ctx, "test_schema", "tenant-1", "run-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insert payslip")
}

func TestCalculatePayroll_UpdatePayrollRunError(t *testing.T) {
	repo := NewMockRepository()
	repo.UpdatePayrollRunErr = errors.New("database error")
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	repo.PayrollRuns["run-1"] = &PayrollRun{
		ID:       "run-1",
		TenantID: "tenant-1",
		Status:   PayrollDraft,
	}

	repo.Employees["emp-1"] = &Employee{
		ID:                   "emp-1",
		TenantID:             "tenant-1",
		IsActive:             true,
		ApplyBasicExemption:  true,
		BasicExemptionAmount: DefaultBasicExemption,
		FundedPensionRate:    FundedPensionRateDefault,
	}
	repo.Salaries["emp-1"] = decimal.NewFromFloat(2000)

	_, err := service.CalculatePayroll(ctx, "test_schema", "tenant-1", "run-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update payroll run")
}

func TestCalculatePayroll_CommitError(t *testing.T) {
	repo := NewMockRepository()
	repo.mockTx.CommitErr = errors.New("commit failed")
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	repo.PayrollRuns["run-1"] = &PayrollRun{
		ID:       "run-1",
		TenantID: "tenant-1",
		Status:   PayrollDraft,
	}

	repo.Employees["emp-1"] = &Employee{
		ID:                   "emp-1",
		TenantID:             "tenant-1",
		IsActive:             true,
		ApplyBasicExemption:  true,
		BasicExemptionAmount: DefaultBasicExemption,
		FundedPensionRate:    FundedPensionRateDefault,
	}
	repo.Salaries["emp-1"] = decimal.NewFromFloat(2000)

	_, err := service.CalculatePayroll(ctx, "test_schema", "tenant-1", "run-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "commit")
}

func TestCalculatePayroll_SkipInactiveEmployees(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	repo.PayrollRuns["run-1"] = &PayrollRun{
		ID:       "run-1",
		TenantID: "tenant-1",
		Status:   PayrollDraft,
	}

	// Only inactive employee with salary
	repo.Employees["emp-1"] = &Employee{
		ID:       "emp-1",
		TenantID: "tenant-1",
		IsActive: false, // Inactive
	}
	repo.Salaries["emp-1"] = decimal.NewFromFloat(2000)

	run, err := service.CalculatePayroll(ctx, "test_schema", "tenant-1", "run-1")
	require.NoError(t, err)
	assert.Len(t, run.Payslips, 0) // No payslips for inactive
}

func TestCalculatePayroll_GetCurrentSalaryError(t *testing.T) {
	repo := NewMockRepository()
	repo.GetCurrentSalaryErr = errors.New("database error")
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	repo.PayrollRuns["run-1"] = &PayrollRun{
		ID:       "run-1",
		TenantID: "tenant-1",
		Status:   PayrollDraft,
	}

	repo.Employees["emp-1"] = &Employee{
		ID:       "emp-1",
		TenantID: "tenant-1",
		IsActive: true,
	}

	// When salary fetch errors, the employee should be skipped
	run, err := service.CalculatePayroll(ctx, "test_schema", "tenant-1", "run-1")
	require.NoError(t, err)
	assert.Len(t, run.Payslips, 0) // Skipped due to error
}

func TestCreateEmployee_DefaultValues(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "emp"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	req := &CreateEmployeeRequest{
		FirstName:           "Mari",
		LastName:            "Maasikas",
		StartDate:           time.Now(),
		ApplyBasicExemption: true, // Should set default exemption amount
	}

	emp, err := service.CreateEmployee(ctx, "test_schema", "tenant-1", req)

	require.NoError(t, err)
	assert.Equal(t, EmploymentFullTime, emp.EmploymentType) // Default
	assert.True(t, emp.BasicExemptionAmount.Equal(DefaultBasicExemption))
	assert.Equal(t, "EE", emp.TaxResidency)
	assert.True(t, emp.IsActive)
}

func TestCreatePayrollRun_DefaultPaymentDate(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "run"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	req := &CreatePayrollRunRequest{
		PeriodYear:  2025,
		PeriodMonth: 1,
		Notes:       "Test notes",
		// PaymentDate is nil
	}

	run, err := service.CreatePayrollRun(ctx, "test_schema", "tenant-1", "user-1", req)

	require.NoError(t, err)
	assert.Nil(t, run.PaymentDate)
	assert.Equal(t, "Test notes", run.Notes)
}

func TestCalculatePayroll_EmployeeWithoutBasicExemption(t *testing.T) {
	repo := NewMockRepository()
	uuidGen := &MockUUIDGenerator{prefix: "test"}
	service := NewServiceWithRepository(repo, uuidGen)
	ctx := context.Background()

	repo.PayrollRuns["run-1"] = &PayrollRun{
		ID:       "run-1",
		TenantID: "tenant-1",
		Status:   PayrollDraft,
	}

	repo.Employees["emp-1"] = &Employee{
		ID:                  "emp-1",
		TenantID:            "tenant-1",
		IsActive:            true,
		ApplyBasicExemption: false, // No exemption
		FundedPensionRate:   FundedPensionRateDefault,
	}
	repo.Salaries["emp-1"] = decimal.NewFromFloat(2000)

	run, err := service.CalculatePayroll(ctx, "test_schema", "tenant-1", "run-1")

	require.NoError(t, err)
	assert.Len(t, run.Payslips, 1)
	// Without exemption, taxable income equals gross
	assert.True(t, run.Payslips[0].TaxableIncome.Equal(decimal.NewFromFloat(2000)))
}

func payrollRunRemediationCodes(actions []PayrollRunRemediationAction) []string {
	codes := make([]string, 0, len(actions))
	for _, action := range actions {
		codes = append(codes, action.Code)
	}
	return codes
}
