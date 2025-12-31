package payroll

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
	EndCurrentBaseSalaryErr  error
	CreateSalaryComponentErr error
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

	// Transaction handling
	BeginTxErr error
	mockTx     *MockTx
}

type MockTx struct {
	CommitCalled   bool
	RollbackCalled bool
	CommitErr      error
}

func (t *MockTx) Commit(ctx context.Context) error {
	t.CommitCalled = true
	return t.CommitErr
}

func (t *MockTx) Rollback(ctx context.Context) error {
	t.RollbackCalled = true
	return nil
}

func (t *MockTx) Begin(ctx context.Context) (pgx.Tx, error) { return nil, nil }
func (t *MockTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (t *MockTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults { return nil }
func (t *MockTx) LargeObjects() pgx.LargeObjects                               { return pgx.LargeObjects{} }
func (t *MockTx) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (t *MockTx) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (t *MockTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}
func (t *MockTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row { return nil }
func (t *MockTx) Conn() *pgx.Conn                                               { return nil }

func NewMockRepository() *MockRepository {
	return &MockRepository{
		Employees:   make(map[string]*Employee),
		PayrollRuns: make(map[string]*PayrollRun),
		Salaries:    make(map[string]decimal.Decimal),
		mockTx:      &MockTx{},
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
	return m.EndCurrentBaseSalaryErr
}

func (m *MockRepository) CreateSalaryComponent(ctx context.Context, schemaName string, comp *SalaryComponent) error {
	if m.CreateSalaryComponentErr != nil {
		return m.CreateSalaryComponentErr
	}
	m.Salaries[comp.EmployeeID] = comp.Amount
	return nil
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

func (m *MockRepository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	if m.BeginTxErr != nil {
		return nil, m.BeginTxErr
	}
	return m.mockTx, nil
}

func (m *MockRepository) WithTx(tx pgx.Tx) Repository {
	return m // Return the same mock for simplicity in tests
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
	assert.Equal(t, amount, repo.Salaries["emp-1"])
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
