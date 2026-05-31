package payroll

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var (
	ErrEmployeeNotFound       = errors.New("employee not found")
	ErrPayrollRunNotFound     = errors.New("payroll run not found")
	ErrTSDDeclarationNotFound = errors.New("TSD declaration not found")
)

// UUIDGenerator provides IDs for payroll services.
type UUIDGenerator interface {
	New() string
}

// DefaultUUIDGenerator uses random UUID values.
type DefaultUUIDGenerator struct{}

func (g *DefaultUUIDGenerator) New() string {
	return uuid.New().String()
}

// Repository defines the contract for payroll data access.
type Repository interface {
	// Employee operations
	CreateEmployee(ctx context.Context, schemaName string, emp *Employee) error
	GetEmployee(ctx context.Context, schemaName, tenantID, employeeID string) (*Employee, error)
	ListEmployees(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]Employee, error)
	UpdateEmployee(ctx context.Context, schemaName string, emp *Employee) error

	// Salary component operations
	EndCurrentBaseSalary(ctx context.Context, schemaName, tenantID, employeeID string, effectiveTo time.Time) error
	CreateSalaryComponent(ctx context.Context, schemaName string, comp *SalaryComponent) error
	ListSalaryComponents(ctx context.Context, schemaName, tenantID, employeeID string, activeOn *time.Time) ([]SalaryComponent, error)
	GetCurrentSalary(ctx context.Context, schemaName, tenantID, employeeID string) (decimal.Decimal, error)

	// Payroll run operations
	CreatePayrollRun(ctx context.Context, schemaName string, run *PayrollRun) error
	GetPayrollRun(ctx context.Context, schemaName, tenantID, runID string) (*PayrollRun, error)
	ListPayrollRuns(ctx context.Context, schemaName, tenantID string, year int) ([]PayrollRun, error)
	UpdatePayrollRun(ctx context.Context, schemaName string, run *PayrollRun) error
	ApprovePayrollRun(ctx context.Context, schemaName, tenantID, runID, approverID string) error

	// Payslip operations
	DeletePayslipsByRunID(ctx context.Context, schemaName, runID string) error
	CreatePayslip(ctx context.Context, schemaName string, payslip *Payslip) error

	// TSD operations
	GetPayslipsWithEmployees(ctx context.Context, schemaName, tenantID, payrollRunID string) ([]Payslip, error)
	DeleteTSDByPeriod(ctx context.Context, schemaName, tenantID string, year, month int) error
	CreateTSDDeclaration(ctx context.Context, schemaName string, declaration *TSDDeclaration) error
	CreateTSDRows(ctx context.Context, schemaName string, rows []TSDRow) error
	GetTSD(ctx context.Context, schemaName, tenantID string, year, month int) (*TSDDeclaration, error)
	GetTSDRows(ctx context.Context, schemaName, tenantID, declarationID string) ([]TSDRow, error)
	ListTSD(ctx context.Context, schemaName, tenantID string) ([]TSDDeclaration, error)
	MarkTSDSubmitted(ctx context.Context, schemaName, tenantID, declarationID, emtaReference string, submittedAt time.Time) error
	UpdateTSDStatus(ctx context.Context, schemaName, tenantID, declarationID string, status TSDStatus, updatedAt time.Time) error

	// Transaction support
	WithTransaction(ctx context.Context, fn func(txRepo Repository) error) error
}
